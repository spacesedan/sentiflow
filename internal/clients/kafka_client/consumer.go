package kafka_client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

var consumer *kafka.Consumer

func InitKafkaConsumer(cfg KafkaConfig) error {
	slog.Info("[KafkaClient] Initializing Kafka Consumer...",
		slog.String("broker", cfg.Broker),
		slog.String("group_id", cfg.GroupID),
		slog.String("topics", fmt.Sprintf("%s, %s", KAFKA_TOPIC_REDDIT_CONTENT, KAFKA_TOPIC_UNCERTAIN_SENTIMENT)))

	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers":  cfg.Broker,
		"group.id":           cfg.GroupID,
		"auto.offset.reset":  "earliest",
		"enable.auto.commit": false,
		"isolation.level":    "read_committed",
	})
	if err != nil {
		return fmt.Errorf("[KafkaClient] Failed to create consumer: %w", err)
	}

	err = c.SubscribeTopics([]string{KAFKA_TOPIC_REDDIT_CONTENT, KAFKA_TOPIC_UNCERTAIN_SENTIMENT}, nil)
	if err != nil {
		return fmt.Errorf("[KafkaClient] Failed to subscribe to topics: %w", err)
	}

	consumer = c
	slog.Info("[KafkaClient] Kafka Consumer initialized successfully")
	return nil
}

const (
	maxRetries = 5
	retryDelay = 2 * time.Second
)

func KafkaMessageIterator(ctx context.Context) (*kafka.Message, error) {
	if consumer == nil {
		return nil, errors.New("[KafkaIterator] Kafka consumer has not been initialized")
	}

	for i := 0; i < maxRetries; i++ {
		select {
		case <-ctx.Done():
			slog.Warn("[KafkaIterator] Context cancelled, stopping iterator")
			return nil, ctx.Err()
		default:
			msg, err := consumer.ReadMessage(-1)
			if err != nil {
				if kafkaErr, ok := err.(kafka.Error); ok && kafkaErr.Code() == kafka.ErrAllBrokersDown {
					slog.Error("[KafkaIterator] All Kafka brokers are down. Aborting")
					return nil, err
				}

				slog.Warn("[KafkaIterator] Failed to read message, retrying...",
					slog.Int("attempt", i+1),
					slog.Int("max_retries", maxRetries),
					slog.String("error", err.Error()))

				time.Sleep(retryDelay)
				continue
			}
			return msg, nil

		}
	}
	return nil, errors.New("[KafkaIterator] Failed to read message after retries")
}

func CommitMessage(msg *kafka.Message) error {
	if consumer == nil {
		return errors.New("[KafkaCommitter] Kafka consumer has not been initialized")
	}

	_, err := consumer.CommitMessage(msg)
	if err != nil {
		slog.Warn("[KafkaCommitter] Failed to commit offset",
			slog.String("error", err.Error()),
			slog.String("partition", fmt.Sprintf("%d", msg.TopicPartition.Partition)),
			slog.String("offset", fmt.Sprintf("%d", msg.TopicPartition.Offset)))
		return err
	}

	slog.Info("[KafkaCommitter] Successfully committed offset",
		slog.String("partition", fmt.Sprintf("%d", msg.TopicPartition.Partition)),
		slog.String("offset", fmt.Sprintf("%d", msg.TopicPartition.Offset)))

	return nil
}
