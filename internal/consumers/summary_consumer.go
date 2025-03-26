package consumers

import (
	"context"
	"log/slog"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/models"
	"github.com/spacesedan/sentiflow/internal/utils"
)

var summaryBuffer = utils.NewBatchBuffer[models.SentimentAnalysisInput]()

func StartSummaryConsumer(ctx context.Context, consumer *kafka.Consumer) {
	iterator := kafka_client.NewKafkaMessageIterator(ctx, consumer)
	committer := kafka_client.NewCommitHandler(ctx, consumer)

	ticker := time.NewTicker(time.Second * 5)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Warn("[SummaryConsumer] Stopping consumer...")
			return
		case <-ticker.C:
			processSummaryBatch(committer)
		default:
			msg, err := iterator.Next()
			if err != nil {
				utils.HandleConsumerError(err)
				continue
			}

			var summaryRequest models.SentimentAnalysisInput
			if err := utils.DeserializeFromJSON(msg.Value, &summaryRequest); err != nil {
				utils.HandleConsumerError(err)
				continue
			}

			summaryBuffer.Add(summaryRequest)
			// Track Message to commit after processing
			utils.TrackMessage(summaryRequest.ContentID, msg)

			if summaryBuffer.Size() > utils.BATCH_SIZE {
				processSummaryBatch(committer)
			}

		}
	}
}

func processSummaryBatch(commiter *kafka_client.KafkaCommitHandler) {
	var summaryErr error
	var summaries models.SummaryBatchResponse
	batch := summaryBuffer.GetAndClear()
	if len(batch) == 0 {
		return
	}

	var summarizedSAInputs []models.SentimentAnalysisInput
	for attempt := 1; attempt <= 3; attempt++ {
		start := time.Now()
		summaries, summaryErr = sendBatchForSummary(batch)
		if summaryErr == nil {
			break
		}
		slog.Warn("[SummaryConsumer] Failed to get summaries, retrying...",
			slog.Int("attempt", attempt),
			slog.Duration("elapsed", time.Since(start)),
			slog.String("error", summaryErr.Error()))

	}
	if summaryErr != nil {
		slog.Error("[SummaryConsumer] Failed to get summaries after 3 tries")
		return
	}
	mappedSummaries := mapSummariesToContentID(summaries)

	for _, req := range batch {
		summaryText := mappedSummaries[req.ContentID].Summary
		// check the quality of our summary
		if summaryText == "" || summaryText == req.Text {
			slog.Warn("[SummaryConsumer] Skipping low-value summary",
				slog.String("content_id", req.ContentID))
			continue
		}
		saInput := buildSummarizedSentimentInput(req, mappedSummaries[req.ContentID].Summary)
		summarizedSAInputs = append(summarizedSAInputs, saInput)
	}

	sendForAnalysis(commiter, summarizedSAInputs)
}

func mapSummariesToContentID(summaries models.SummaryBatchResponse) map[string]models.SummaryResponse {
	mappedSummaries := make(map[string]models.SummaryResponse, len(summaries.Summaries))
	for _, summary := range summaries.Summaries {
		mappedSummaries[summary.ContentID] = summary
	}

	return mappedSummaries
}

func sendBatchForSummary(batch []models.SentimentAnalysisInput) (models.SummaryBatchResponse, error) {
	var hfRequest models.SummaryBatchRequest

	for _, post := range batch {
		hfRequest.Inputs = append(hfRequest.Inputs, models.SummaryRequest{
			ContentID: post.ContentID,
			Text:      post.Text,
		})
	}

	return clients.GetHuggingFaceClient().GetSummaries(hfRequest)
}

func sendForAnalysis(commiter *kafka_client.KafkaCommitHandler, summarizedContent []models.SentimentAnalysisInput) {
	for _, content := range summarizedContent {
		// Publish the summarized content to be analyzed
		for i := 0; i < 3; i++ {
			err := kafka_client.PublishToKafka(
				kafka_client.KAFKA_TOPIC_SENTIMENT_REQUEST,
				[]models.SentimentAnalysisInput{content},
			)
			if err == nil {
				break
			}
			slog.Warn("[SummaryConsumer] summary request publishing failed",
				slog.Int("attempt", i+1),
				slog.String("error", err.Error()))

			time.Sleep(2 * time.Second)
		}

		// commit the message to prevent reprocessing
		trackedMsg, found := utils.
			GetMessageForContent(content.ContentID)
		if found {
			err := commiter.Commit(trackedMsg)
			if err != nil {
				slog.Warn("[SummaryConsumer] Failed to commit offset",
					slog.String("error", err.Error()))
			}
		}
	}
}

// buildSummarizedSentimentInput Builds a new sentiment analysis request using summarized text
func buildSummarizedSentimentInput(request models.SentimentAnalysisInput, summary string) models.SentimentAnalysisInput {
	return models.SentimentAnalysisInput{
		RawContent: models.RawContent{
			ContentID: request.ContentID,
			Source:    request.Source,
			Topic:     request.Topic,
			Metadata:  request.Metadata,
		},
		Text:          summary,
		OriginalText:  request.Text,
		WasSummarized: true,
	}
}
