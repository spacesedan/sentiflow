package kafka_client

import (
	"fmt"
	"log/slog"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

func NewConsumer() (*kafka.Consumer, error) {
	cfg := GetKafkaConfig()
	slog.Info("[KafkaClient] Initializing Kafka Consumer...",
		slog.String("broker", cfg.Broker),
		slog.String("topic", cfg.Topic),
		slog.String("group_id", cfg.GroupID))

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":        cfg.Broker,
		"group.id":                 cfg.GroupID,
		"auto.offset.reset":        "earliest",
		"enable.auto.commit":       false,
		"isolation.level":          "read_committed",
		"session.timeout.ms":       10000,
		"max.poll.interval.ms":     300000,
		"reconnect.backoff.ms":     500,
		"reconnect.backoff.max.ms": 10000,
		"socket.keepalive.enable":  true,
	})
	if err != nil {
		return nil, fmt.Errorf("[KafkaClient] Failed to create consumer: %w", err)
	}

	err = c.SubscribeTopics([]string{cfg.Topic}, nil)
	if err != nil {
		return nil, fmt.Errorf("[KafkaClient] Failed to subscribe to topics: %w", err)
	}

	slog.Info("[KafkaClient] Kafka Consumer initialized successfully")
	return c, err
}
