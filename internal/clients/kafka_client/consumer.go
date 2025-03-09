package kafka_client

import (
	"fmt"
	"log/slog"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

var consumer *kafka.Consumer

func InitKafkaConsumer(cfg KafkaConfig) error {
	slog.Info("[KafkaClient] Initializing Kafka Consumer...")

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": cfg.Broker,
		"group.id":          cfg.GroupID,
		"auto.offset.reset": "earliest",
		"isolation.level":   "read_committed",
	})
	if err != nil {
		return fmt.Errorf("[KafkaClient] Failed to create consumer: %w", err)
	}

	err = c.SubscribeTopics([]string{KAFKA_TOPIC}, nil)
	if err != nil {
		return fmt.Errorf("[KafkaClient] Failed to subscribe to topics: %w", err)
	}

	consumer = c
	slog.Info("[KafkaClient] Kafka Consumer initialized successfully")
	return nil
}

func CloseKafkaConsumer() {
	slog.Info("[KafkaClient] Shutting down Kafka consumer...")
	if consumer != nil {
		consumer.Close()
		slog.Info("[KafkaClient] Kafka consumer shut down")
	}
}
