package processing

import (
	"context"
	"encoding/json"
	"log/slog"
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

	var err error
	// Fetch stored topics from DB for deduplication
	storedTopics, err := db.GetAllTopics()
	if err != nil {
		slog.Error("[TopicGenerator] Failed to fetch stored topics", slog.String("error", err.Error()))
		storedTopics = []models.Topic{} // Fallback to empty
	}

	// Chunk articles into batches of 100
	batches := chunkArticles(headlines, 100)
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
		maxRetries := 3
		success := false // Track success

		for attempt := 1; attempt <= maxRetries; attempt++ {
			err = nil

			chatCompletion, err := clients.GetAIClient().Client.Chat.Completions.New(context.TODO(),
				openai.ChatCompletionNewParams{
					Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
						openai.SystemMessage(openAIPrompt),
						openai.UserMessage(string(batchBytes)),
					}),
					Model:       openai.F(openai.ChatModelGPT3_5Turbo),
					Temperature: openai.Float(0.5),
				})
			if err != nil {
				slog.Warn("[TopicGenerator] OpenAI API call failed, retrying",
					slog.Int("attempt", attempt),
					slog.String("error", err.Error()))
				time.Sleep(2 * time.Second)
				continue
			}

			if len(chatCompletion.Choices) == 0 || strings.TrimSpace(chatCompletion.Choices[0].Message.Content) == "" {
				slog.Warn("[TopicGenerator] OpenAI returned empty response, retrying",
					slog.Int("attempt", attempt))
				time.Sleep(2 * time.Second)
				continue
			}

			topicsRaw = cleanOpenAIResponse(chatCompletion.Choices[0].Message.Content)

			// Parse JSON
			var batchResp models.OpenAITopicResponse
			err = json.Unmarshal([]byte(topicsRaw), &batchResp)
			if err != nil {
				slog.Warn("[TopicGenerator] Failed to parse JSON into struct, retrying",
					slog.Int("attempt", attempt),
					slog.String("error", err.Error()))
				time.Sleep(2 * time.Second)
				continue
			}

			// Deduplicate new topics before appending
			batchResp.Topics = removeLocalDuplicates(batchResp.Topics)
			batchResp.Topics = filterAgainstStored(batchResp.Topics, storedTopics)
			allTopics = append(allTopics, batchResp.Topics...)

			// Success: Exit retry loop
			success = true
			break
		}

		// If all retries failed, log final error
		if !success {
			slog.Error("[TopicGenerator] OpenAI failed after retries", slog.String("error", err.Error()))
			continue
		}
	}

	// Handle case where no topics were generated
	if len(allTopics) == 0 {
		slog.Info("[TopicGenerator] No new topics were generated or all were duplicates")
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

func cleanOpenAIResponse(response string) string {
	// Trim unnecessary whitespace
	response = strings.TrimSpace(response)

	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimSuffix(response, "```")

	// Standardize quotes in case OpenAI outputs them incorrectly
	response = strings.ReplaceAll(response, "\u0022", `"`) // Replace Unicode quotes
	response = strings.ReplaceAll(response, "\u201C", `"`) // Left curly quote
	response = strings.ReplaceAll(response, "\u201D", `"`) // Right curly quote

	return strings.TrimSpace(response) // Final trim
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
