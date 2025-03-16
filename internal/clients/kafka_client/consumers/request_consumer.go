package consumers

import (
	"context"
	"log/slog"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client/utils"
	"github.com/spacesedan/sentiflow/internal/models"
)

var postBatchBuffer = utils.NewBatchBuffer[models.RedditPost]()

func StartRequestConsumer(ctx context.Context, consumer *kafka.Consumer) {
	iterator := kafka_client.NewKafkaMessageIterator(ctx, consumer)
	committer := kafka_client.NewCommitHandler(ctx, consumer)

	slog.Info("[RequestConsumer] Listening for messages...")

	ticker := time.NewTicker(utils.BATCH_TIMEOUT)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Warn("[RequestConsumer] Stopping consumer...")
			return
		case <-ticker.C:
			go sendBatchToKafka(committer)
		default:
			msg, err := iterator.Next()
			if err != nil {
				utils.HandleConsumerError(err)
				continue
			}

			var post models.RedditPost
			if err := utils.DeserializeFromJSON(msg.Value, &post); err != nil {
				continue
			}

			utils.TrackMessage(post.PostID, msg)
			postBatchBuffer.Add(post)

			if postBatchBuffer.Size() >= utils.BATCH_SIZE {
				go sendBatchToKafka(committer)
			}

		}
	}
}

func sendBatchToKafka(commiter *kafka_client.KafkaCommitHandler) {
	batch := postBatchBuffer.GetAndClear()
	if batch == nil || len(batch) == 0 {
		return
	}

	for i := 0; i < 3; i++ {

		err := kafka_client.PublishToKafka(kafka_client.KAFKA_TOPIC_SENTIMENT_BATCHES, batch)
		if err == nil {
			break
		}
		slog.Warn("[RequestConsumer] Batch publishing Failed",
			slog.Int("attempt", i+1),
			slog.String("error", err.Error()))
		time.Sleep(2 * time.Second)
	}

	for _, post := range batch {
		trackedMsg, found := utils.GetMessageForPost(post.PostID)
		if found {
			err := commiter.Commit(trackedMsg)
			if err != nil {
				slog.Warn("[RequestConsumer] Failed to commit offset",
					slog.String("error", err.Error()))
			}
		}
	}
}
