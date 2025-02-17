package clients

import (
	"encoding/json"
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
// InitKafka initializes the Kafka producer
func InitKafka() error {
	var broker string

	// 🔥 Check if running in Docker (KAFKA_BROKER set)
	if os.Getenv("KAFKA_BROKER") != "" {
		broker = os.Getenv("KAFKA_BROKER")
	} else {
		broker = "localhost:29092"
	}

	slog.Info("🔄 Connecting to Kafka", slog.String("broker", broker))

	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":   broker,
		"security.protocol":   "PLAINTEXT", // ✅ Force PLAINTEXT
		"api.version.request": "true",      // ✅ Ensure correct API version request
	})
	if err != nil {
		return err
	}
	producer = p
	slog.Info("✅ Kafka Producer initialized")
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
	jsonData, err := json.Marshal(post)
	if err != nil {
		return err
	}

	topic := KafkaTopic

	// Send message to Kafka
	err = producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Value:          jsonData,
	}, nil)
	if err != nil {
		return err
	}

	slog.Info("📨 Published Reddit post to Kafka",
		slog.String("topic", post.Topic),
		slog.String("subreddit", post.Subreddit))
	return nil
}
