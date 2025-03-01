package processing

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/db"
	"github.com/spacesedan/sentiflow/internal/models"
)

const openAIPrompt = `Shorten each headline into a concise, **search-friendly phrase**. 
**Important**: 
- Preserve any **names or key entities** (e.g., people, companies, places) mentioned in the original text.
- Assign **exactly one** of these categories:
  - Technology
  - Business & Finance
  - Politics & World Affairs
  - Entertainment & Pop Culture
  - Health & Science
  - Sports
  - Lifestyle & Society
  - Memes & Internet Trends
  - Crime & Law
- **Deduplicate** by URL: if the same URL appears multiple times, only **one** entry should be returned for that URL.

### **STRICT OUTPUT FORMAT**  
You MUST return only **valid JSON**, formatted exactly as follows:  
{
  "topics": [
    {"topic": "XXX", "category": "XXX", "url": "XXX"}
  ]
}

### **REQUIREMENTS**
- **No Markdown formatting** (no triple backticks, no explanations).
- **No extra text before or after the JSON output**.
- **No trailing commas** in JSON objects or arrays.
- **Ensure correct escaping of special characters** in JSON strings.
- **Do NOT modify the URLs**—return them exactly as given.

If you are unable to generate a valid response, return an **empty JSON object**:
{
    "topics": [
        { "topic": "XXX", "category": "XXX", "url": "XXX" }
    ]
}
`

// GenerateTopicsFromHeadlines processes new headlines in batches, dedupes results, and merges them
func GenerateTopicsFromHeadlines(headlines []models.NewsAPIArticles) (*models.OpenAITopicResponse, error) {
	slog.Info("[TopicGenerator] Starting topic generation")

	// Fetch stored topics from DB for deduplication
	storedTopics, err := db.GetAllTopics()
	if err != nil {
		slog.Error("[TopicGenerator] Failed to fetch stored topics", slog.String("error", err.Error()))
		storedTopics = []models.Topic{} // fallback to empty
	}

	// Chunk articles into batches of 75
	batches := chunkArticles(headlines, 75)
	slog.Info("[TopicGenerator] Headlines chunked", slog.Int("num_chunks", len(batches)))

	var allTopics []models.Topic

	for i, batch := range batches {
		if i > 0 {
			time.Sleep(3 * time.Second) // Rate limit
		}

		// Marshal batch to JSON
		batchBytes, err := json.Marshal(batch)
		if err != nil {
			slog.Error("[TopicGenerator] JSON marshal failed", slog.String("error", err.Error()))
			continue
		}

		// OpenAI API Call with retry logic
		var topicsRaw string
		var chatCompletion *openai.ChatCompletion
		maxRetries := 3

		for attempt := 1; attempt <= maxRetries; attempt++ {
			chatCompletion, err = clients.GetAIClient().Client.Chat.Completions.New(context.TODO(),
				openai.ChatCompletionNewParams{
					Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
						openai.SystemMessage(openAIPrompt),
						openai.UserMessage(string(batchBytes)),
					}),
					Model:       openai.F(openai.ChatModelGPT3_5Turbo),
					Temperature: openai.Float(0.5),
				})
			// Handle API failure
			if err != nil {
				slog.Warn("[TopicGenerator] OpenAI API call failed, retrying",
					slog.Int("attempt", attempt),
					slog.String("error", err.Error()))
				time.Sleep(2 * time.Second)
				continue
			}

			// Extract response
			if len(chatCompletion.Choices) == 0 || strings.TrimSpace(chatCompletion.Choices[0].Message.Content) == "" {
				slog.Warn("[TopicGenerator] OpenAI returned empty response, retrying",
					slog.Int("attempt", attempt))
				time.Sleep(2 * time.Second)
				continue
			}

			// Clean OpenAI response formatting issues
			topicsRaw = strings.TrimSpace(topicsRaw)
			topicsRaw = strings.TrimPrefix(topicsRaw, "```json")
			topicsRaw = strings.TrimSuffix(topicsRaw, "```")
			topicsRaw = strings.ReplaceAll(topicsRaw, "\u0022", `"`) // Replace Unicode quotes if present
			topicsRaw = strings.ReplaceAll(topicsRaw, "\u201C", `"`) // Replace curly left quote
			topicsRaw = strings.ReplaceAll(topicsRaw, "\u201D", `"`) // Replace curly right quote

			// Step 1: Validate JSON format
			if !json.Valid([]byte(topicsRaw)) {
				slog.Warn("[TopicGenerator] OpenAI returned invalid JSON, retrying",
					slog.Int("attempt", attempt),
					slog.String("raw_response", topicsRaw))
				time.Sleep(2 * time.Second)
				continue
			}

			err = os.WriteFile("openai.json", []byte(topicsRaw), 0644)
			if err != nil {
				panic(err)
			}
			os.Exit(1)

			// Step 2: Log raw JSON before parsing
			slog.Info("[TopicGenerator] Valid JSON response received", slog.String("json_preview", topicsRaw[:min(len(topicsRaw), 500)]))

			// Step 3: Parse as a generic map for inspection
			var tempMap map[string]interface{}
			if err := json.Unmarshal([]byte(topicsRaw), &tempMap); err != nil {
				slog.Warn("[TopicGenerator] Failed to parse JSON into map, retrying",
					slog.Int("attempt", attempt),
					slog.String("error", err.Error()))
				time.Sleep(2 * time.Second)
				continue
			}

			// Step 4: Check if "topics" key exists
			topicsData, exists := tempMap["topics"]
			if !exists {
				slog.Warn("[TopicGenerator] Missing 'topics' key in response, retrying",
					slog.Int("attempt", attempt))
				time.Sleep(2 * time.Second)
				continue
			}

			// Step 5: Marshal back to JSON to check structural correctness
			_, _ = json.Marshal(topicsData)
			slog.Info("[TopicGenerator] Extracted 'topics' key successfully")

			// Step 6: Parse JSON into expected struct
			var batchResp models.OpenAITopicResponse
			err := json.Unmarshal([]byte(topicsRaw), &batchResp)
			if err != nil {
				slog.Warn("[TopicGenerator] Failed to parse JSON into struct, retrying",
					slog.Int("attempt", attempt),
					slog.String("error", err.Error()))
				time.Sleep(2 * time.Second)
				continue
			}

			// Step 5: Deduplicate new topics
			batchResp.Topics = removeLocalDuplicates(batchResp.Topics)
			batchResp.Topics = filterAgainstStored(batchResp.Topics, storedTopics)
			allTopics = append(allTopics, batchResp.Topics...)

			// Successful parse, exit retry loop
			break
		}

		// If all retries failed, log the final bad response
		if err != nil {
			slog.Error("[TopicGenerator] OpenAI failed after retries", slog.String("error", err.Error()))
			continue
		}

		if topicsRaw != "" {
			slog.Error("[TopicGenerator] OpenAI returned unparseable response after retries",
				slog.String("raw_response", topicsRaw[:500]))
		}
	}

	// Handle case where no topics were generated
	if len(allTopics) == 0 {
		return nil, errors.New("[TopicGenerator] No new topics were generated or all were duplicates")
	}

	slog.Info("[TopicGenerator] Successfully generated topics",
		slog.Int("total_new_topics", len(allTopics)))

	return &models.OpenAITopicResponse{Topics: allTopics}, nil
}

// chunkArticles splits a slice of NewsAPIArticles into groups of `size`.
func chunkArticles(articles []models.NewsAPIArticles, size int) [][]models.NewsAPIArticles {
	var batches [][]models.NewsAPIArticles
	for i := 0; i < len(articles); i += size {
		end := i + size
		if end > len(articles) {
			end = len(articles)
		}
		batches = append(batches, articles[i:end])
	}
	return batches
}

// removeLocalDuplicates ensures the newly generated batch from OpenAI
// doesn't contain duplicates among itself (by URL).
func removeLocalDuplicates(topics []models.Topic) []models.Topic {
	seen := make(map[string]struct{})
	var unique []models.Topic
	for _, t := range topics {
		if _, exists := seen[t.URL]; !exists && t.URL != "" {
			seen[t.URL] = struct{}{}
			unique = append(unique, t)
		}
	}
	return unique
}

// filterAgainstStored removes any topics whose URL is already in storedTopics
func filterAgainstStored(newTopics []models.Topic, storedTopics []models.Topic) []models.Topic {
	storedSet := make(map[string]struct{}, len(storedTopics))
	for _, st := range storedTopics {
		storedSet[st.URL] = struct{}{}
	}

	var final []models.Topic
	for _, t := range newTopics {
		if _, exists := storedSet[t.URL]; !exists {
			final = append(final, t)
		}
	}
	return final
}
