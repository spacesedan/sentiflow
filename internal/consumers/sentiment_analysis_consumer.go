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

var resultBuffer = utils.NewBatchBuffer[models.SentimentAnalysisResult]()

func StartSentimentAnalysisConsumer(ctx context.Context, consumer *kafka.Consumer) {
	iterator := kafka_client.NewKafkaMessageIterator(ctx, consumer)
	committer := kafka_client.NewCommitHandler(ctx, consumer)

	for {
		select {
		case <-ctx.Done():
			slog.Warn("[SentimentAnalysisConsumer] Consumer shutting down...")
			return
		default:
			msg, err := iterator.Next()
			if err != nil {
				utils.HandleConsumerError(err)
				continue
			}

			var requests []models.SentimentAnalysisInput

			if err := utils.DeserializeFromJSON(msg.Value, &requests); err != nil {
				utils.HandleConsumerError(err)
				continue
			}

			utils.TrackMessage(requests[0].ContentID, msg)

			var hfRequest models.SentimentAnalysisBatchRequest

			for _, request := range requests {
				hfRequest.Posts = append(hfRequest.Posts, models.SentimentAnalysisRequest{
					ContentID: request.ContentID,
					Text:      request.Text,
				})
			}

			sentimentScores, err := clients.GetHuggingFaceClient().GetBatchedSentimentAnalysis(hfRequest)
			if err != nil {
				slog.Error("[SentimentAnalysisConsumer] Failed to get sentiment scores",
					slog.String("error", err.Error()))
				continue
			}

			mappedScores := mapSentimentScoreToContentID(sentimentScores)

			for _, request := range requests {
				score, ok := mappedScores[request.ContentID]
				if !ok {
					slog.Warn("[SentimentAnalysisConsumer] No sentiment results for content ID",
						slog.String("content_id", request.ContentID))
				}
				resultBuffer.Add(models.SentimentAnalysisResult{
					SentimentAnalysisInput: request,
					SentimentScore:         score.SentimentScore,
					SentimentLabel:         score.SentimentLabel,
					Confidence:             score.Confidence,
				})

			}
			sendResultsForStorage(committer)

		}
	}
}

func sendResultsForStorage(committer *kafka_client.KafkaCommitHandler) {
	batch := resultBuffer.GetAndClear()
	if len(batch) == 0 {
		return
	}

	for i := 0; i < 3; i++ {

		err := kafka_client.PublishToKafka(kafka_client.KAFKA_TOPIC_SENTIMENT_RESULTS, batch)
		if err == nil {
			break
		}
		slog.Warn("[SentimentAnalysisConsumer] Batch publishing Failed",
			slog.Int("attempt", i+1),
			slog.String("error", err.Error()))
		time.Sleep(2 * time.Second)
	}

	for _, content := range batch {
		trackedMsg, found := utils.GetMessageForContent(content.ContentID)
		if found {
			err := committer.Commit(trackedMsg)
			if err != nil {
				slog.Warn("[SentimentAnalysisConsumer] Failed to commit offset",
					slog.String("error", err.Error()))
			}
		}
	}
}

// mapSentimentScoreToContentID Creates a map to sentiment scores to avoid nested loops
func mapSentimentScoreToContentID(scores models.SentimentAnalysisBatchResponse) map[string]models.SentimentAnalysisResponse {
	scoreMap := make(map[string]models.SentimentAnalysisResponse, len(scores))

	for _, score := range scores {
		scoreMap[score.ContentID] = score
	}

	return scoreMap
}
