package consumers

import (
	"context"
	"log/slog"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/db"
	"github.com/spacesedan/sentiflow/internal/models"
	"github.com/spacesedan/sentiflow/internal/utils"
)

var insertBuffer = utils.NewBatchBuffer[models.SentimentAnalysisResult]()

func StartResultsConsumer(ctx context.Context, consumer *kafka.Consumer) {
	iterator := kafka_client.NewKafkaMessageIterator(ctx, consumer)
	committer := kafka_client.NewCommitHandler(ctx, consumer)

	ticker := time.NewTicker(utils.BATCH_TIMEOUT)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			processResults(ctx, committer)
		default:
			msg, err := iterator.Next()
			if err != nil {
				utils.HandleConsumerError(err)
				continue
			}

			var results []models.SentimentAnalysisResult

			err = utils.DeserializeFromJSON(msg.Value, &results)
			if err != nil {
				utils.HandleConsumerError(err)
				continue
			}

			for _, result := range results {
				utils.TrackMessage(result.ContentID, msg)
				insertBuffer.Add(result)
				if insertBuffer.Size() >= utils.DYNAMODB_BATCH_SIZE {
					processResults(ctx, committer)
				}
			}
		}
	}
}

func processResults(ctx context.Context, committer *kafka_client.KafkaCommitHandler) {
	var insertErr error
	batch := insertBuffer.GetAndClear()
	if len(batch) == 0 {
		return
	}

	for i := 0; i < 3; i++ {
		insertErr = db.BatchInsertSentimentResults(ctx, batch)
		if insertErr == nil {
			break
		}
		slog.Error("[ResultConsumer] Failed to write results to DB",
			slog.String("error", insertErr.Error()),
			slog.Int("attempt", i+1))
	}

	for _, result := range batch {
		msg, found := utils.GetMessageForContent(result.ContentID)
		if found {
			if err := committer.Commit(msg); err != nil {
				slog.Warn("[ResultConsumer] Failed to commit offset",
					slog.String("error", err.Error()))
			}
		}

	}
}
