package processing

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"strings"
	"time"

	"github.com/openai/openai-go"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/db"
	"github.com/spacesedan/sentiflow/internal/models"
)

// GenerateTopicsFromHeadlines processes new headlines in batches, dedupes results, and merges them
func GenerateTopicsFromHeadlines(headlines []models.NewsAPIArticles) (*models.OpenAITopicResponse, error) {
	slog.Info("[TopicGenerator] Starting to generate topics from headlines")

	// 1️⃣ Fetch all stored topics from DB for external dedup
	storedTopics, err := db.GetAllTopics()
	if err != nil {
		slog.Error("[TopicGenerator] Failed to fetch stored topics from DB", slog.String("error", err.Error()))
		storedTopics = []models.Topic{} // fallback to empty if DB is unreachable
	}

	// 2️⃣ Break the incoming headlines into chunks of 100
	batches := chunkArticles(headlines, 100)
	slog.Info("[TopicGenerator] Headlines chunked", slog.Int("num_chunks", len(batches)))

	var allTopics []models.Topic // final aggregate of unique topics

	for i, batch := range batches {
		if i > 0 {
			// Sleep a bit to avoid hitting any rate limits
			time.Sleep(3 * time.Second)
		}

		// 3️⃣ Marshal only the chunk
		batchBytes, err := json.Marshal(batch)
		if err != nil {
			slog.Error("[TopicGenerator] JSON marshal batch failed", slog.String("error", err.Error()))
			continue
		}

		// 4️⃣ Build the minimal prompt
		prompt := `
Shorten every topic into a short, precise phrase that can be a search query.
Categorize them into:
- Technology, Business & Finance, Politics & World Affairs, Entertainment & Pop Culture,
  Health & Science, Sports, Lifestyle & Society, Memes & Internet Trends, Crime & Law.
Include "url" in the final output. Deduplicate if you produce the same URL multiple times.

Output JSON Format:
{
  "topics": [
    {"topic": "XXX", "category": "XXX", "url": "XXX"}
  ]
}
`

		// 5️⃣ Call OpenAI
		start := time.Now()
		chatCompletion, err := clients.GetAIClient().Client.Chat.Completions.New(context.TODO(),
			openai.ChatCompletionNewParams{
				Messages: openai.F([]openai.ChatCompletionMessageParamUnion{
					openai.SystemMessage(prompt),
					openai.UserMessage(string(batchBytes)),
				}),
				Model:       openai.F(openai.ChatModelGPT3_5Turbo),
				Temperature: openai.Float(0.5),
				// Optionally set a max token limit for the response, e.g.,
				// MaxTokens: openai.Int(500),
			})
		elapsed := time.Since(start)

		if err != nil {
			slog.Error("[TopicGenerator] OpenAI API call failed",
				slog.String("error", err.Error()),
				slog.Duration("duration", elapsed))
			continue
		}

		slog.Info("[TopicGenerator] OpenAI call for batch completed",
			slog.Int("batch_index", i),
			slog.Duration("duration", elapsed))

		if len(chatCompletion.Choices) == 0 || strings.TrimSpace(chatCompletion.Choices[0].Message.Content) == "" {
			slog.Error("[TopicGenerator] OpenAI returned an empty response for this batch")
			continue
		}

		topicsRaw := strings.TrimSpace(chatCompletion.Choices[0].Message.Content)
		// remove optional triple-backticks if present
		topicsRaw = strings.TrimPrefix(topicsRaw, "```json")
		topicsRaw = strings.TrimSuffix(topicsRaw, "```")
		topicsRaw = strings.TrimSpace(topicsRaw)

		var batchResp models.OpenAITopicResponse
		if err := json.Unmarshal([]byte(topicsRaw), &batchResp); err != nil {
			slog.Error("[TopicGenerator] Failed to parse OpenAI response JSON", slog.String("error", err.Error()))
			continue
		}

		// 6️⃣ Remove duplicates within the new batch itself
		batchResp.Topics = removeLocalDuplicates(batchResp.Topics)

		// 7️⃣ Remove duplicates that are already in storedTopics
		batchResp.Topics = filterAgainstStored(batchResp.Topics, storedTopics)

		// 9️⃣ Accumulate these new topics in allTopics for the final return
		allTopics = append(allTopics, batchResp.Topics...)
	}

	if len(allTopics) == 0 {
		return nil, errors.New("[TopicGenerator] No new topics were generated or all were duplicates")
	}

	slog.Info("[TopicGenerator] Successfully generated topics from headlines",
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
