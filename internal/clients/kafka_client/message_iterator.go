package kafka_client

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type KafkaMessageIterator struct {
	cfg      KafkaConfig
	consumer *kafka.Consumer
	ctx      context.Context
}

func NewKafkaMessageIterator(ctx context.Context, consumer *kafka.Consumer) *KafkaMessageIterator {
	cfg := GetKafkaConfig()
	return &KafkaMessageIterator{
		cfg:      cfg,
		consumer: consumer,
		ctx:      ctx,
	}
}

func (it *KafkaMessageIterator) Next() (*kafka.Message, error) {
	if it.consumer == nil {
		return nil, errors.New("[KafkaIterator] Kafka consumer has not been initialized")
	}

	const maxBackoff = 30 * time.Second

	for i := 0; i < MAX_RETRIES; i++ {
		select {
		case <-it.ctx.Done():
			slog.Warn("[KafkaIterator] Context cancelled, stopping iterator",
				slog.String("topic", it.cfg.Topic),
				slog.String("group", it.cfg.GroupID))
			return nil, it.ctx.Err()
		default:
			msg, err := it.consumer.ReadMessage(-1)
			if err != nil {
				kafkaErr, ok := err.(kafka.Error)
				if ok && !isRetryableKafkaError(kafkaErr) {
					slog.Error("[KafkaIterator]", "Non-Retryable Kafka error",
						slog.String("topic", it.cfg.Topic),
						slog.String("group", it.cfg.GroupID))
					return nil, err
				}

				backoff := RETRY_DELAY * (1 << i)
				if backoff > maxBackoff {
					backoff = maxBackoff
				}

				slog.Warn("[KafkaIterator] Failed to read message, retrying...",
					slog.Int("attempt", i+1),
					slog.Int("max_retries", MAX_RETRIES),
					slog.Duration("backoff", backoff),
					slog.String("error", err.Error()),
					slog.String("topic", it.cfg.Topic),
					slog.String("group", it.cfg.GroupID))

				time.Sleep(backoff)
				continue
			}
			return msg, nil

		}
	}
	return nil, errors.New("[KafkaIterator] Failed to read message after retries")
}

func isRetryableKafkaError(err kafka.Error) bool {
	switch err.Code() {
	case kafka.ErrTransport,
		kafka.ErrRequestTimedOut,
		kafka.ErrTimedOut,
		kafka.ErrBrokerNotAvailable,
		kafka.ErrLeaderNotAvailable,
		kafka.ErrAllBrokersDown:
		return true
	default:
		return false
	}
}
