package kafka_client

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type KafkaMessageIterator struct {
	consumer *kafka.Consumer
	ctx      context.Context
}

func NewKafkaMessageIterator(ctx context.Context, consumer *kafka.Consumer) *KafkaMessageIterator {
	return &KafkaMessageIterator{
		consumer: consumer,
		ctx:      ctx,
	}
}

func (it *KafkaMessageIterator) Next() (*kafka.Message, error) {
	if it.consumer == nil {
		return nil, errors.New("[KafkaIterator] Kafka consumer has not been initialized")
	}

	for i := 0; i < MAX_RETRIES; i++ {
		select {
		case <-it.ctx.Done():
			slog.Warn("[KafkaIterator] Context cancelled, stopping iterator")
			return nil, it.ctx.Err()
		default:
			msg, err := it.consumer.ReadMessage(-1)
			if err != nil {
				if kafkaErr, ok := err.(kafka.Error); ok && kafkaErr.Code() == kafka.ErrAllBrokersDown {
					slog.Error("[KafkaIterator] All Kafka brokers are down. Aborting")
					return nil, err
				}

				slog.Warn("[KafkaIterator] Failed to read message, retrying...",
					slog.Int("attempt", i+1),
					slog.Int("max_retries", MAX_RETRIES),
					slog.String("error", err.Error()))

				time.Sleep(RETRY_DELAY)
				continue
			}
			return msg, nil

		}
	}
	return nil, errors.New("[KafkaIterator] Failed to read message after retries")
}
