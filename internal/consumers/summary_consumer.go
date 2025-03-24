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

func StartSummaryConsumer(ctx context.Context, consumer *kafka.Consumer) {
	iterator := kafka_client.NewKafkaMessageIterator(ctx, consumer)
	committer := kafka_client.NewCommitHandler(ctx, consumer)

	for {
		select {
		case <-ctx.Done():
			slog.Warn("[SummaryConsumer] Stopping consumer...")
			return
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

			// Track Message to commit after processing
			utils.TrackMessage(summaryRequest.ContentID, msg)

			hfRequest := models.SummaryRequest{
				Inputs: summaryRequest.Text,
			}

			// get the summary from our summarization service
			summary, err := clients.GetHuggingFaceClient().GetSummary(hfRequest)
			if err != nil {
				slog.Error("[SummaryConsumer] Failed to get summary",
					slog.String("content_id", summaryRequest.ContentID),
					slog.String("error", err.Error()))
				continue
			}

			summaryText := summary.Summary
			// check the quality of our summary
			if summaryText == "" || summaryText == summaryRequest.Text {
				slog.Warn("[SummaryConsumer] Skipping low-value summary",
					slog.String("content_id", summaryRequest.ContentID))
				continue
			}

			// build a new sentiment analysis input with our summary
			saInput := buildSummarizedSentimentInput(summaryRequest, summary.Summary)

			// send the summarized request for analysis
			sendForAnalysis(committer, saInput)

		}
	}
}

func sendForAnalysis(commiter *kafka_client.KafkaCommitHandler, content models.SentimentAnalysisInput) {
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
