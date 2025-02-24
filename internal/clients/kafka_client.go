package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spacesedan/sentiflow/internal/models"
)

// Kafka topic for Reddit posts
const KafkaTopic = "reddit-content"

// Kafka producer instance
var producer *kafka.Producer

// InitKafka initializes the Kafka producer
func InitKafka() error {
	var broker string

	// Check if running in Docker (KAFKA_BROKER set)
	if os.Getenv("KAFKA_BROKER") != "" {
		broker = os.Getenv("KAFKA_BROKER")
	} else {
		broker = "localhost:29092"
	}

	slog.Info("Connecting to Kafka", slog.String("broker", broker))

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":                     broker,
		"security.protocol":                     "PLAINTEXT", // Force PLAINTEXT
		"api.version.request":                   "true",      // Ensure correct API version request
		"enable.idempotence":                    "true",
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
	slog.Info("Kafka Producer initialized")
	return nil
}

func CloseKafka() {
	if producer != nil {
		producer.Flush(1)
		producer.Close()
		slog.Info("Kafka producer shut down")
	}
}

// PublishToKafka sends a Reddit post to Kafka
func PublishToKafka(post models.RedditPost) error {
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
	topic := KafkaTopic
	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(post.PostID),
		Value:          jsonData,
	}

	// Produce the message within this transaction.
	if err := producer.Produce(msg, nil); err != nil {
		// If producing fails, abort the transaction.
		abortErr := producer.AbortTransaction(context.Background())
		if abortErr != nil {
			return fmt.Errorf("[KafkaClient] failed to abort transaction after produce error: %v", abortErr)
		}
		return err
	}

	// COMMIT the transaction. If this fails, you should decide how to handle it
	// (log, retry, or propagate the error).
	if err := producer.CommitTransaction(context.Background()); err != nil {
		return fmt.Errorf("[KafkaClient] failed to commit transaction: %v", err)
	}

	slog.Info("[KafkaClient] Published Reddit post to Kafka transactionally",
		slog.String("topic", post.Topic),
		slog.String("subreddit", post.Subreddit))

	return nil
}
