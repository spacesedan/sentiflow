package consumers

import (
	"context"
	"log/slog"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/models"
	"github.com/spacesedan/sentiflow/internal/utils"
)

const SUMMARY_THRESHOLD = 1024

var postBatchBuffer = utils.NewBatchBuffer[models.SentimentAnalysisInput]()

func StartRawContentConsumer(ctx context.Context, consumer *kafka.Consumer) {
	iterator := kafka_client.NewKafkaMessageIterator(ctx, consumer)
	committer := kafka_client.NewCommitHandler(ctx, consumer)

	slog.Info("[RawContentConsumer] Listening for messages...")

	ticker := time.NewTicker(utils.BATCH_TIMEOUT)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Warn("[RawContentConsumer] Stopping consumer...")
			return
		case <-ticker.C:
			go sendBatchToKafka(ctx, committer)
		default:
			msg, err := iterator.Next()
			if err != nil {
				utils.HandleConsumerError(err)
				continue
			}

			// Get the raw content from the message
			var content models.RawContent
			if err := utils.DeserializeFromJSON(msg.Value, &content); err != nil {
				continue
			}

			// convert the raw content to a sentiment analysis input
			saInput := utils.RawToSentimentAnalysisInput(content)

			// track the message for downstream use
			utils.TrackMessage(saInput.ContentID, msg)

			// if incoming message is longer than 1024 characters it needs to
			// be summarized
			if len(saInput.Text) > SUMMARY_THRESHOLD {
				sendForSummary(ctx, committer, saInput)
				continue
			}

			// if the message is under our summary threshold add to our buffer
			postBatchBuffer.Add(saInput)

			if postBatchBuffer.Size() >= utils.BATCH_SIZE {
				go sendBatchToKafka(ctx, committer)
			}

		}
	}
}

func sendForSummary(ctx context.Context, commiter *kafka_client.KafkaCommitHandler, content models.SentimentAnalysisInput) {
	// Publish the long message to be summarized
	for i := 0; i < 3; i++ {
		err := kafka_client.PublishToKafka(ctx, kafka_client.KAFKA_TOPIC_SUMMARY_REQUEST, content)
		if err == nil {
			break
		}
		slog.Warn("[RawContentConsumer] Summary request publishing failed",
			slog.Int("attempt", i+1),
			slog.String("error", err.Error()))
		time.Sleep(2 * time.Second)
	}

	trackedMsg, found := utils.GetMessageForContent(content.ContentID)
	if found {
		err := commiter.Commit(trackedMsg)
		if err != nil {
			slog.Warn("[RawContentConsumer] Failed to commit offset",
				slog.String("error", err.Error()))
		}
	}
}

func sendBatchToKafka(ctx context.Context, commiter *kafka_client.KafkaCommitHandler) {
	batch := postBatchBuffer.GetAndClear()
	if len(batch) == 0 {
		return
	}

	for i := 0; i < 3; i++ {

		err := kafka_client.PublishToKafka(ctx, kafka_client.KAFKA_TOPIC_SENTIMENT_REQUEST, batch)
		if err == nil {
			break
		}
		slog.Warn("[RawContentConsumer] Batch publishing Failed",
			slog.Int("attempt", i+1),
			slog.String("error", err.Error()))
		time.Sleep(2 * time.Second)
	}

	for _, content := range batch {
		trackedMsg, found := utils.GetMessageForContent(content.ContentID)
		if found {
			err := commiter.Commit(trackedMsg)
			if err != nil {
				slog.Warn("[RawContentConsumer] Failed to commit offset",
					slog.String("error", err.Error()))
			}
		}
	}
}
