package kafka_client

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spacesedan/sentiflow/internal/models"
)

var producer *kafka.Producer

func InitKafkaProducer(cfg KafkaConfig) error {
	slog.Info("[KafkaClient] Initializing Kafka Producer...")

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":                     cfg.Broker,
		"security.protocol":                     "PLAINTEXT", // Force PLAINTEXT
		"api.version.request":                   "true",      // Ensure correct API version request
		"enable.idempotence":                    true,
		"acks":                                  "all",
		"max.in.flight.requests.per.connection": 1,
		"transactional.id":                      "sentiflow-producer-1",
	})
	if err != nil {
		return fmt.Errorf("[KafkaClient] Failed to create producer: %w", err)
	}

	if err := p.InitTransactions(context.Background()); err != nil {
		return fmt.Errorf("[KafkaClient] Failed to init transactions: %w", err)
	}

	producer = p
	slog.Info("[KafkaClient] Kafka Producer initialized successfully")
	return nil
}

func CloseKafkaProducer() {
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
func PublishToKafka(topic string, post models.RedditPost) error {
	// BEGIN the transaction for this batch (in this case, just 1 message).
	if err := producer.BeginTransaction(); err != nil {
		return fmt.Errorf("[KafkaClient] failed to begin transaction: %v", err)
	}

	// Serialize the Reddit post.
	jsonData, err := json.Marshal(post)
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
		Key:            []byte(post.PostID),
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
		// If producing fails, abort the transaction.
		abortErr := producer.AbortTransaction(context.Background())
		if abortErr != nil {
			return fmt.Errorf("[KafkaClient] failed to abort transaction after produce error: %v", abortErr)
		}
		return err
	}

	// COMMIT the transaction. If this fails, you should decide how to handle it
	// (log, retry, or propagate the error).
	var commitErr error
	for i := 0; i < 3; i++ {
		commitErr := producer.CommitTransaction(context.Background())
		if commitErr == nil {
			break
		}
		slog.Warn("[KafkaClient] Failed to commit transaction, retruing...",
			slog.Int("attempt", i+1))
	}
	if commitErr != nil {
		return fmt.Errorf("[KafkaClient] failed to commit transaction after 3 retries: %w", commitErr)
	}

	slog.Info("[KafkaClient] Published Reddit post to Kafka transactionally",
		slog.String("topic", post.Topic),
		slog.String("post_id", post.PostID))

	return nil
}
