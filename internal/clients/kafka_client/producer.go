package kafka_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spacesedan/sentiflow/internal/models"
)

var (
	producer *kafka.Producer
	initOnce sync.Once
)

func generateTransactionalID(baseID string) string {
	envTransactionalID := os.Getenv("KAFKA_PRODUCER_ID")
	if envTransactionalID != "" {
		return envTransactionalID
	}

	now := time.Now().Unix()
	pid := os.Getpid()
	return fmt.Sprintf("%s-%d-%d-producer", baseID, now, pid)
}

func InitProducer(cfg KafkaConfig) error {
	var initErr error

	initOnce.Do(func() {
		transactionalID := generateTransactionalID(cfg.Topic)
		slog.Info("[KafkaClient] Initializing Kafka Producer...",
			slog.String("transactional_id", transactionalID))

		p, err := kafka.NewProducer(&kafka.ConfigMap{
			"bootstrap.servers":                     cfg.Broker,
			"security.protocol":                     "PLAINTEXT", // Force PLAINTEXT
			"api.version.request":                   "true",      // Ensure correct API version request
			"enable.idempotence":                    true,
			"acks":                                  "all",
			"max.in.flight.requests.per.connection": 1,
			"transactional.id":                      transactionalID,
		})
		if err != nil {
			initErr = fmt.Errorf("[KafkaClient] Failed to create producer: %w", err)
			return
		}

		if err := p.InitTransactions(context.Background()); err != nil {
			initErr = fmt.Errorf("[KafkaClient] Failed to init transactions: %w", err)
			return
		}

		producer = p
		slog.Info("[KafkaClient] Kafka Producer initialized successfully",
			slog.String("transactional_id", transactionalID))
	})
	return initErr
}

func CloseProducer() {
	slog.Info("[KafkaClient] Shutting down Kafka producer...")
	if producer != nil {
		slog.Info("[KafkaClient] Flushing Kafka producer before shutdown...")
		if remaining := producer.Flush(5000); remaining > 0 {
			slog.Warn("[KafkaClient] Not all messages were delivered before shutdown",
				slog.Int("remaining", remaining))
		}
		producer.Close()
		slog.Info("[KafkaClient] Kafka producer shut down")
	}
}

// PublishToKafka sends a Reddit post to Kafka
func PublishToKafka(topic string, message interface{}) error {
	// BEGIN the transaction for this batch (in this case, just 1 message).
	if err := producer.BeginTransaction(); err != nil {
		return fmt.Errorf("[KafkaClient] failed to begin transaction: %v", err)
	}

	// Extract the PostID dynamically
	var postID string

	switch msg := message.(type) {
	case models.RedditPost:
		postID = msg.PostID
		// TODO: add the logic for the Result object
	default:
		postID = "unknown"
	}

	// Serialize the message.
	jsonData, err := json.Marshal(message)
	if err != nil {
		// If serialization fails, abort the transaction.
		abortErr := producer.AbortTransaction(context.Background())
		if abortErr != nil {
			return fmt.Errorf("[KafkaClient] failed to abort transaction after marshal error: %v", abortErr)
		}
		return err
	}

	// Construct the Kafka message with the Reddit Post ID as the key.
	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(postID),
		Value:          jsonData,
	}

	// Produce the message within this transaction.
	for i := 0; i < 3; i++ {
		err = producer.Produce(msg, nil)
		if err == nil {
			break
		}
		slog.Warn("[KafkaClient] Failed to produce message, retrying...",
			slog.Int("attempt", i+1))
	}
	if err != nil {
		abortTransaction()
		return err
	}

	// COMMIT the transaction. If this fails, you should decide how to handle it
	// (log, retry, or propagate the error).
	if err := commitTransaction(); err != nil {
		return err
	}

	slog.Info("[KafkaClient] Published Reddit post to Kafka transactionally",
		slog.String("topic", topic),
		slog.String("post_id", postID))

	return nil
}

func extractPostID(message interface{}) string {
	switch msg := message.(type) {
	case models.RedditPost:
		return msg.PostID
	default:
		return "unknown"
	}
}

func abortTransaction() {
	if err := producer.AbortTransaction(context.Background()); err != nil {
		slog.Warn("[KafkaClient] Failed to abort transaction",
			slog.String("error", err.Error()))
	}
}

func commitTransaction() error {
	for i := 0; i < 3; i++ {
		if err := producer.CommitTransaction(context.Background()); err == nil {
			return nil
		}
		slog.Warn("[KafkaClient] Failed to commit transaction, retrying...",
			slog.Int("attempt", i+1))
	}

	return errors.New("[KafkaClient] Failed to commit transaction after 3 retries")
}
