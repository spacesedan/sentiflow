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
	// storedBytes, _ := json.Marshal(storedHeadlines)
	// os.WriteFile("./test_data/storedHeadlines.json", storedBytes, 0644)
	// os.Exit(1)

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

	for i := 0; i < 5; i++ {
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
		slog.Warn("failed to get a response from OpenAI after 5 tries",
			slog.String("error", completionErr.Error()))
		return completionErr
	}

	cleanedResponse := cleanOpenAIResponse(resp.Choices[0].Message.Content)

	var generatedHeadlines *models.OpenAIHeadlineResponse
	if err := json.Unmarshal([]byte(cleanedResponse), &generatedHeadlines); err != nil {
		slog.Error("Failed to unmarshal generated headlines",
			slog.String("error", err.Error()))
		return err
	}

	// Process OpenAI responses: separate valid ones from those to be re-queued.
	var validOpenAIResponses []models.OpenAIHeadline
	// Create a map of the original batch items for quick lookup when re-queueing.
	originalBatchMap := make(map[string]models.Headline, len(batch))
	for _, bh := range batch {
		originalBatchMap[bh.ID] = bh
	}

	for _, ogh := range generatedHeadlines.Headlines { // ogh is models.OpenAIHeadline
		if ogh.Query == "" || ogh.Category == "" {
			slog.Warn("[TopicGenerator] OpenAI response missing query or category, re-queueing",
				slog.String("ID", ogh.ID),
				slog.String("query", ogh.Query),
				slog.String("category", ogh.Category))
			// Find the original headline from the input batch to re-queue it.
			if originalHeadline, ok := originalBatchMap[ogh.ID]; ok {
				headlineBuffer.Add(originalHeadline) // Add original models.Headline back to buffer
			} else {
				// This should ideally not happen if IDs are consistent.
				slog.Warn("[TopicGenerator] Could not find original headline in current batch to re-queue", slog.String("ID", ogh.ID))
			}
		} else {
			validOpenAIResponses = append(validOpenAIResponses, ogh)
		}
	}

	// Proceed with only the valid OpenAI responses.
	// return the unique headlines from the valid generated ones
	uniqueHeadlines := removeLocalDuplicates(validOpenAIResponses) // uniqueHeadlines is []models.OpenAIHeadline
	// uniqueBytes, _ := json.Marshal(uniqueHeadlines)
	// os.WriteFile("./test_data/uniqueHeadlines.json", uniqueBytes, 0644)

	// match the unique generated headlines to the original using the IDs
	// batch is the original []models.Headline
	matchedHeadlines := matchLocalHeadlines(uniqueHeadlines, batch) // matchedHeadlines is []models.Headline
	// matchedBytes, _ := json.Marshal(matchedHeadlines)
	// os.WriteFile("./test_data/matchedHeadlines.json", matchedBytes, 0644)

	// filter the generated headlines from the stored and keep only new headlines
	filteredHeadline := filterAgainstStored(matchedHeadlines, storedHeadlines)
	// filteredBytes, _ := json.Marshal(filteredHeadline)
	// os.WriteFile("./test_data/filteredHeadlines.json", filteredBytes, 0644)

	if err := db.StoreBatchedHeadlines(ctx, filteredHeadline); err != nil {
		slog.Error("Failed to store generated headlines in db",
			slog.String("error", err.Error()))
		return err
	}

	return nil
}

func buildChatMessage(headlines []models.Headline) []openai.ChatCompletionMessage {
	systemMessage := `
You will receive several news headlines formatted as JSON objects.

Your task is to transform each headline into a queryable format and assign it to one of the predefined categories.

Instructions:

Respond only with a valid JSON object. Do not include any additional text or commentary.

For each headline object, include the following fields:

- headline: The original headline as it was provided.

- query: A concise, clear, and searchable version of the headline.
    - **IMPORTANT** This field must not be empty or null. If no relevant query can be formed, return a simplified version of the headline instead.

- category: One of the following categories:

    Technology

    Business & Finance

    Politics & World Affairs

    Entertainment & Pop Culture

    Health & Science

    Sports

    Lifestyle & Society

    Memes & Internet Trends

    Crime & Law

- id: Return the exact same ID that was received in the input.

Expected JSON response format:
{
  "headlines": [
    {
      "headline": "Original headline here",
      "query": "Queryable version of the headline",
      "category": "One of the predefined categories",
      "id": "Same ID as provided"
    }
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
		slog.Error("Could not extract valid JSON from OpenAI response")
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
func removeLocalDuplicates(headlines []models.OpenAIHeadline) []models.OpenAIHeadline {
	slog.Info("[TopicGenerator] Removing duplicate generated topics",
		slog.Int("starting", len(headlines)))
	seen := make(map[string]struct{})
	var unique []models.OpenAIHeadline
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

func matchLocalHeadlines(uniqueHeadlines []models.OpenAIHeadline, headlines []models.Headline) []models.Headline {
	slog.Info("[TopicGenerator] Matching generated headlines to their original headlines")

	seen := make(map[string]struct{}, len(uniqueHeadlines))
	var matched []models.Headline

	// map the unique headlines into a map for easier searching
	uniqueMap := make(map[string]models.OpenAIHeadline, len(uniqueHeadlines))
	for _, uh := range uniqueHeadlines {
		if uh.ID != "" {
			uniqueMap[uh.ID] = uh
		}
	}

	// add the generate query and category while ignoring any duplicates
	for _, h := range headlines { // h is an original models.Headline from the input batch
		if _, exists := seen[h.ID]; !exists && h.ID != "" {
			uniqueOpenAIResp, foundInUniqueMap := uniqueMap[h.ID] // uniqueOpenAIResp is models.OpenAIHeadline

			// If not foundInUniqueMap, it means its corresponding OpenAI response was either:
			// 1. Invalid (missing query/category) and thus re-queued (so not in uniqueOpenAIHeadlines).
			// 2. A duplicate OpenAI response removed by removeLocalDuplicates.
			// In such cases, we skip creating a matched headline for storage in this cycle.
			// Also, as a safeguard, check if Query or Category is empty even if found.
			if !foundInUniqueMap || uniqueOpenAIResp.Query == "" || uniqueOpenAIResp.Category == "" {
				if !foundInUniqueMap {
					slog.Debug("[TopicGenerator] Original headline ID not found in unique OpenAI responses; likely re-queued or was a duplicate OpenAI response.", slog.String("ID", h.ID))
				} else {
					// This state (found in map but query/category empty) should ideally not be reached
					// if validOpenAIResponses is correctly filtered before removeLocalDuplicates.
					slog.Warn("[TopicGenerator] Matched OpenAI response has empty query/category despite pre-filtering, skipping.",
						slog.String("ID", h.ID),
						slog.String("query", uniqueOpenAIResp.Query),
						slog.String("category", uniqueOpenAIResp.Category))
				}
				continue
			}

			headline := models.Headline{
				ID:       h.ID,
				Query:    uniqueOpenAIResp.Query,
				Category: uniqueOpenAIResp.Category,
				HeadlineMeta: models.HeadlineMeta{
					Source:      h.HeadlineMeta.Source,
					Title:       h.HeadlineMeta.Title,
					Author:      h.HeadlineMeta.Author,
					Description: h.HeadlineMeta.Description,
					PublishedAt: h.HeadlineMeta.PublishedAt,
					Url:         h.HeadlineMeta.Url,
					UrlToImage:  h.HeadlineMeta.UrlToImage,
				},
			}
			matched = append(matched, headline)
			seen[h.ID] = struct{}{}
		}
	}

	slog.Info("[TopicGenerator] Successfully matched generated headlines with the original request")
	return matched
}

// filterAgainstStored removes any topics whose URL is already in storedTopics
func filterAgainstStored(newHeadlines []models.Headline, storedHeadlines []models.Headline) []models.Headline {
	slog.Info("[TopicGenerator] Removing topics that have been previously stored", slog.Int("starting", len(newHeadlines)))
	// there are no stored headlines to filter against
	if len(storedHeadlines) == 0 {
		slog.Info("[TopicGenerator] Nothing stored, skipping...")
		return newHeadlines
	}
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
