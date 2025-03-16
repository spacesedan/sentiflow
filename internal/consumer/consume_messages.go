package consumer

import (
	"context"
	"encoding/json"
	"log/slog"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/models"
)

const (
	BATCH_SIZE    = 50
	BATCH_TIMEOUT = 5 * time.Second
)

var (
	messageBuffer []models.RedditPost
	messageMap    sync.Map
	bufferLock    sync.Mutex
)

func ConsumeAndBatchMessages(ctx context.Context) {
	slog.Info("[Consumer] Starting consumer with batching...")

	ticker := time.NewTicker(BATCH_TIMEOUT)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Warn("[Consumer] Consumer shutting down...")
			return
		case <-ticker.C:
			publishBatchedMessages()
		default:
			msg, err := kafka_client.KafkaMessageIterator(ctx)
			if err != nil {
				slog.Error("[Consumer] Failed to fetch message", slog.String("error", err.Error()))
			}

			var post models.RedditPost
			if err := json.Unmarshal(msg.Value, &post); err != nil {
				slog.Warn("[Consumer] Failed to deserialize message, skipping...",
					slog.String("error", err.Error()))
				continue
			}

			messageMap.Store(post.PostID, msg)

			bufferLock.Lock()
			messageBuffer = append(messageBuffer, post)
			bufferLock.Unlock()

			if len(messageBuffer) >= BATCH_SIZE {
				go publishBatchedMessages()
			}

		}
	}
}

func publishBatchedMessages() {
	bufferLock.Lock()
	defer bufferLock.Unlock()

	// make sure there are messages to send
	if len(messageBuffer) == 0 {
		return
	}

	slog.Info("[Consumer] Sending batch to Sentiment Service",
		slog.Int("batch_size", len(messageBuffer)))

	// Publish messages to Kafka
	err := kafka_client.PublishToKafka(kafka_client.KAFKA_TOPIC_SENTIMENT_BATCHES, messageBuffer)
	if err != nil {
		slog.Warn("[Consumer] Batch publishing failed",
			slog.String("error", err.Error()))
	}

	// Commit messaged after sending to kafka
	for _, post := range messageBuffer {
		msg, found := getMessageForPost(post.PostID)
		if found {
			err := kafka_client.CommitMessage(msg)
			if err != nil {
				slog.Warn("[Consumer] failed to commit message",
					slog.String("post_id", post.PostID),
					slog.String("error", err.Error()))
			} else {
				slog.Info("[Consumer] Successfully committed message",
					slog.String("post_id", post.PostID))
			}
		} else {
			slog.Warn("[Consumer] no Kafka message found for PostID",
				slog.String("post_id", post.PostID))
		}
	}

	messageBuffer = nil
}

func getMessageForPost(postID string) (*kafka.Message, bool) {
	msg, ok := messageMap.Load(postID)
	if !ok {
		return nil, false
	}

	messageMap.Delete(postID)
	return msg.(*kafka.Message), true
}
