package kafka_client

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

var consumerRegistry = make(map[string]func(context.Context, *kafka.Consumer))

func RegisterConsumer(topic string, consumerFunc func(context.Context, *kafka.Consumer)) {
	consumerRegistry[topic] = consumerFunc
}

func StartConsumer(ctx context.Context) error {
	cfg := GetKafkaConfig()
	consumerFunc, exists := consumerRegistry[cfg.Topic]
	if !exists {
		return fmt.Errorf("[ConsumerFactory] No consumer found for topic: %s", cfg.Topic)
	}

	consumer, err := NewConsumer()
	if err != nil {
		return fmt.Errorf("[ConsumerFactory] Failed to initialize Kafka consumer: %w", err)
	}
	defer consumer.Close()

	slog.Info("[ConsumerFactory] Starting consumer for topic...", slog.String("topic", cfg.Topic))
	consumerFunc(ctx, consumer)

	return nil
}
