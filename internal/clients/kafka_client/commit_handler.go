package kafka_client

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
)

type KafkaCommitHandler struct {
	consumer *kafka.Consumer
	ctx      context.Context
}

func NewCommitHandler(ctx context.Context, consumer *kafka.Consumer) *KafkaCommitHandler {
	return &KafkaCommitHandler{
		consumer: consumer,
		ctx:      ctx,
	}
}

func (ch *KafkaCommitHandler) Commit(msg *kafka.Message) error {
	if ch.consumer == nil {
		return errors.New("[KafkaCommitHandler] Kafka consumer has not been initialized")
	}

	const maxBackoff = time.Second * 30

	for i := 0; i < MAX_RETRIES; i++ {
		select {
		case <-ch.ctx.Done():
			slog.Warn("[KafkaCommitHandler] Context canceled, stopping commit",
				slog.String("partition", fmt.Sprintf("%d", msg.TopicPartition.Partition)),
				slog.String("offset", fmt.Sprintf("%d", msg.TopicPartition.Offset)))
			return ch.ctx.Err()
		default:

			_, err := ch.consumer.CommitMessage(msg)
			if err == nil {
				slog.Info("[KafkaCommitHandler] Successfully committed offset",
					slog.String("partition", fmt.Sprintf("%d", msg.TopicPartition.Partition)),
					slog.String("offset", fmt.Sprintf("%d", msg.TopicPartition.Offset)))
				return nil
			}

			kafkaErr, ok := err.(kafka.Error)
			if ok && !isRetryableKafkaError(kafkaErr) {
				slog.Error("[KafkaCommitHandler] Non-retyable commit error. Aborting commit",
					slog.String("partition", fmt.Sprintf("%d", msg.TopicPartition.Partition)),
					slog.String("offset", fmt.Sprintf("%d", msg.TopicPartition.Offset)),
					slog.String("error", kafkaErr.Error()))
				return err
			}
			backoff := RETRY_DELAY * (1 << i)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}

			slog.Warn("[KafkaCommitHandler] Failed to commit offset, retrying...",
				slog.Int("attempt", i+1),
				slog.Int("max_retries", MAX_RETRIES),
				slog.Duration("backoff", backoff),
				slog.String("error", err.Error()),
				slog.String("partition", fmt.Sprintf("%d", msg.TopicPartition.Partition)),
				slog.String("offset", fmt.Sprintf("%d", msg.TopicPartition.Offset)))

			time.Sleep(backoff)
		}
	}

	return fmt.Errorf("[KafkaCommitHandler] Failed to commit message after %d retries", MAX_RETRIES)
}
