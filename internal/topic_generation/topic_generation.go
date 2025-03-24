package topicgeneration

import (
	"context"
	"encoding/json"
	"log/slog"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/db"
	"github.com/spacesedan/sentiflow/internal/models"
	"github.com/spacesedan/sentiflow/internal/utils"
)

var headlineBuffer = utils.NewBatchBuffer[models.NewsAPIArticles]()

// GenerateTopicsFromHeadlines processes new headlines in batches, dedupes results, and merges them
func GenerateTopicsFromHeadlines(ctx context.Context, headlines []models.NewsAPIArticles) {
	slog.Info("[TopicGenerator] Starting topic generation")

	var err error
	var storedTopics []models.Topic

	storedTopics, err = db.GetAllTopics()
	if err != nil {
		slog.Error("[TopicGenerator] Failed to fetch stored topics", slog.String("error", err.Error()))
		storedTopics = []models.Topic{} // Fallback to empty
	}

	for _, headline := range headlines {
		select {
		case <-ctx.Done():
			slog.Warn("[TopicGenerator] context canceled, flushing remaining buffer")
			if err := processHeadlineBatch(ctx, storedTopics); err != nil {
				slog.Error("[TopicGenerator] context canceled; flushing remaining buffer",
					slog.String("error", err.Error()))
			}
			return
		default:
			headlineBuffer.Add(headline)
			if headlineBuffer.Size() >= 100 {
				if err := processHeadlineBatch(ctx, storedTopics); err != nil {
					slog.Error("[TopicGenerator] Error flushing buffer",
						slog.String("error", err.Error()))
				}
			}
		}
	}

	if headlineBuffer.Size() > 0 {
		if err := processHeadlineBatch(ctx, storedTopics); err != nil {
			slog.Error("[TopicGenerator] Error processing final batch",
				slog.String("error", err.Error()))
		}
	}
}

func processHeadlineBatch(ctx context.Context, storedTopics []models.Topic) error {
	batch := headlineBuffer.GetAndClear()
	if len(batch) == 0 {
		return nil
	}

	var completionErr error
	var resp openai.ChatCompletionResponse

	messages := buildChatMessage(batch)

	for i := 0; i < 3; i++ {
		start := time.Now()
		resp, completionErr = clients.GetOpenAIClient().Client.CreateChatCompletion(ctx, openai.ChatCompletionRequest{
			Model:    openai.GPT3Dot5Turbo1106,
			Messages: messages,
			ResponseFormat: &openai.ChatCompletionResponseFormat{
				Type: openai.ChatCompletionResponseFormatTypeJSONObject,
			},
		})
		if completionErr == nil {
			break
		}
		slog.Warn("Failed to get a response from OpenAI, retrying...",
			slog.String("error", completionErr.Error()),
			slog.Int("attempt", i+1),
			slog.Duration("elapsed", time.Since(start)))
	}
	if completionErr != nil {
		slog.Warn("failed to get a response from OpenAI after 3 tries",
			slog.String("error", completionErr.Error()))
		return completionErr
	}

	cleanedResponse := cleanOpenAIResponse(resp.Choices[0].Message.Content)

	var generatedTopics *models.OpenAITopicResponse
	if err := json.Unmarshal([]byte(cleanedResponse), &generatedTopics); err != nil {
		slog.Error("Failed to unmarshal generated topics",
			slog.String("error", err.Error()))
		return err
	}

	uniqueTopics := removeLocalDuplicates(generatedTopics.Topics)
	filteredTopics := filterAgainstStored(uniqueTopics, storedTopics)

	if err := db.StoreBatchedTopics(ctx, filteredTopics); err != nil {
		slog.Error("Failed to store generated topics in db",
			slog.String("error", err.Error()))
		return err
	}

	return nil
}

func buildChatMessage(headlines []models.NewsAPIArticles) []openai.ChatCompletionMessage {
	systemMessage := `
You will receive several news headlines as JSON objects.
Respond ONLY with a valid JSON object, without any additional commentary.
Each topic must have:

- "title": Concise, clear, and easily searchable (queryable).
- "category": One of the following predefined categories:
  - Technology
  - Business & Finance
  - Politics & World Affairs
  - Entertainment & Pop Culture
  - Health & Science
  - Sports
  - Lifestyle & Society
  - Memes & Internet Trends
  - Crime & Law
- "url": Original URL from the headline.

JSON response structure:
{
  "topics": [
    {
      "title": "queryable title",
      "category": "Predefined Category",
      "url": "original URL"
    },
    ...
  ]
}
`

	messages := []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: systemMessage,
		},
	}

	for _, headline := range headlines {
		bytes, err := json.Marshal(headline)
		if err != nil {
			slog.Warn("Failed to marshal headline",
				slog.String("headline", headline.Title),
				slog.String("error", err.Error()))
			continue
		}

		messages = append(messages, openai.ChatCompletionMessage{
			Role:    openai.ChatMessageRoleUser,
			Content: string(bytes),
		})
	}

	return messages
}

func cleanOpenAIResponse(response string) string {
	response = strings.TrimSpace(response)

	// Find the first '{' and the last '}' to extract only JSON.
	startIdx := strings.Index(response, "{")
	endIdx := strings.LastIndex(response, "}")

	if startIdx == -1 || endIdx == -1 || endIdx <= startIdx {
		slog.Error("Could not extract valid JSON from OpenAI response", slog.String("raw_response", response))
		return "" // or handle the error appropriately
	}

	response = response[startIdx : endIdx+1]

	// Remove Markdown code block if present.
	response = strings.TrimPrefix(response, "```json")
	response = strings.TrimSuffix(response, "```")

	// Standardize quotes
	response = strings.ReplaceAll(response, "\u0022", `"`)
	response = strings.ReplaceAll(response, "\u201C", `"`)
	response = strings.ReplaceAll(response, "\u201D", `"`)

	return strings.TrimSpace(response)
}

// removeLocalDuplicates ensures the newly generated batch from OpenAI
// doesn't contain duplicates among itself (by URL).
func removeLocalDuplicates(topics []models.Topic) []models.Topic {
	slog.Info("[TopicGenerator] Removing duplicate generated topics",
		slog.Int("starting", len(topics)))
	seen := make(map[string]struct{})
	var unique []models.Topic
	for _, t := range topics {
		if _, exists := seen[t.URL]; !exists && t.URL != "" {
			seen[t.URL] = struct{}{}
			unique = append(unique, t)
		}
	}
	slog.Info("[TopicGenerator] Successfully removed generated topics",
		slog.Int("ending", len(unique)))
	return unique
}

// filterAgainstStored removes any topics whose URL is already in storedTopics
func filterAgainstStored(newTopics []models.Topic, storedTopics []models.Topic) []models.Topic {
	slog.Info("[TopicGenerator] Removing topics that have been previously stored", slog.Int("starting", len(newTopics)))
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

	slog.Info("[TopicGenerator] Successfully filtered out generated topics",
		slog.Int("ending", len(final)))
	return final
}
