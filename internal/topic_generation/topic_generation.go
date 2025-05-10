package topicgeneration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/sashabaranov/go-openai"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/db"
	"github.com/spacesedan/sentiflow/internal/models"
	"github.com/spacesedan/sentiflow/internal/utils"
)

var headlineBuffer = utils.NewBatchBuffer[models.Headline]()

// generateHeadlineID generates a unique ID for a headline using the source, headline, and url
func generateHeadlineID(headline, source, url string) string {
	raw := fmt.Sprintf("%s:%s:%s", headline, source, url)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

// GenerateTopicsFromHeadlines processes new headlines in batches, dedupes results, and merges them
func GenerateTopicsFromHeadlines(ctx context.Context, articles []models.NewsAPIArticle) {
	slog.Info("[TopicGenerator] Starting headline generation")

	var err error
	var storedHeadlines []models.Headline

	headlines := normalizeHeadlines(articles)

	// Get the stored headlines in dynamoDB
	storedHeadlines, err = db.GetAllHeadlines()
	if err != nil {
		// fallback to en empty array if no headlines are stored
		slog.Error("[TopicGenerator] Failed to fetch stored topics", slog.String("error", err.Error()))
		storedHeadlines = []models.Headline{} // Fallback to empty
	}

	for _, headline := range headlines {
		select {
		case <-ctx.Done():
			slog.Warn("[TopicGenerator] context canceled, flushing remaining buffer")
			if err := processHeadlineBatch(ctx, storedHeadlines); err != nil {
				slog.Error("[TopicGenerator] context canceled; flushing remaining buffer",
					slog.String("error", err.Error()))
			}
			return
		default:
			// add headlines to the batch
			headlineBuffer.Add(headline)
			// once the batch gets to a certain size process the batch
			if headlineBuffer.Size() >= 100 {
				if err := processHeadlineBatch(ctx, storedHeadlines); err != nil {
					slog.Error("[TopicGenerator] Error flushing buffer",
						slog.String("error", err.Error()))
				}
			}
		}
	}

	// at the end of the loop process any remaining healines in the batch
	if headlineBuffer.Size() > 0 {
		if err := processHeadlineBatch(ctx, storedHeadlines); err != nil {
			slog.Error("[TopicGenerator] Error processing final batch",
				slog.String("error", err.Error()))
		}
	}
}

// normalizeHeadline - updates the shape of the NewsAPI article to work with stored headlines
func normalizeHeadlines(articles []models.NewsAPIArticle) []models.Headline {
	var headlines []models.Headline
	source := "NewsAPI"

	for _, article := range articles {
		headline := models.Headline{
			ID: generateHeadlineID(article.Title, source, article.URL),
			HeadlineMeta: models.HeadlineMeta{
				Source:      source,
				Author:      article.Author,
				Title:       article.Title,
				Description: article.Description,
				Url:         article.URL,
				UrlToImage:  article.UrlToImage,
				PublishedAt: article.PublishedAt,
			},
		}
		headlines = append(headlines, headline)
	}

	return headlines
}

// processHeadlineBatch - makes the request to OpenAI to generate queryable headlines
func processHeadlineBatch(ctx context.Context, storedHeadlines []models.Headline) error {
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

	var generatedHeadlines *models.OpenAITopicResponse
	if err := json.Unmarshal([]byte(cleanedResponse), &generatedHeadlines); err != nil {
		slog.Error("Failed to unmarshal generated topics",
			slog.String("error", err.Error()))
		return err
	}

	uniqueHeadlines := removeLocalDuplicates(generatedHeadlines.Headlines)
	filteredHeadline := filterAgainstStored(uniqueHeadlines, storedHeadlines)

	if err := db.StoreBatchedHeadlines(ctx, filteredHeadline); err != nil {
		slog.Error("Failed to store generated topics in db",
			slog.String("error", err.Error()))
		return err
	}

	return nil
}

func buildChatMessage(headlines []models.Headline) []openai.ChatCompletionMessage {
	systemMessage := `
You will receive several news headlines as JSON objects.
Respond ONLY with a valid JSON object, without any additional commentary.
Each topic must have:

- "headline": The original "headline" as it was sent.
- "queryable_headline": Concise, clear, and easily searchable (queryable).
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
- "id": return the same ID that was sent in the message

JSON response structure:
{
  "headines": [
    {
      "headline": "the original headline",
      "queryable_headline": "queryable version of the headline",
      "category": "Predefined Category",
      "id": "return the exact value that was sent in the orginal request"
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
		req := models.OpenAIRequest{
			Headline: headline.HeadlineMeta.Title,
			ID:       headline.ID,
		}
		bytes, err := json.Marshal(req)
		if err != nil {
			slog.Warn("Failed to marshal headline",
				slog.String("headline", headline.HeadlineMeta.Title),
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
func removeLocalDuplicates(headlines []models.Headline) []models.Headline {
	slog.Info("[TopicGenerator] Removing duplicate generated topics",
		slog.Int("starting", len(headlines)))
	seen := make(map[string]struct{})
	var unique []models.Headline
	for _, t := range headlines {
		if _, exists := seen[t.ID]; !exists && t.ID != "" {
			seen[t.ID] = struct{}{}
			unique = append(unique, t)
		}
	}
	slog.Info("[TopicGenerator] Successfully removed generated topics",
		slog.Int("ending", len(unique)))
	return unique
}

// filterAgainstStored removes any topics whose URL is already in storedTopics
func filterAgainstStored(newHeadlines []models.Headline, storedHeadlines []models.Headline) []models.Headline {
	slog.Info("[TopicGenerator] Removing topics that have been previously stored", slog.Int("starting", len(newHeadlines)))
	storedSet := make(map[string]struct{}, len(storedHeadlines))
	for _, st := range storedHeadlines {
		storedSet[st.ID] = struct{}{}
	}

	var final []models.Headline
	for _, t := range newHeadlines {
		if _, exists := storedSet[t.ID]; !exists {
			final = append(final, t)
		}
	}

	slog.Info("[TopicGenerator] Successfully filtered out generated topics",
		slog.Int("ending", len(final)))
	return final
}
