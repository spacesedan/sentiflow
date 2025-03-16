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

	for i := 0; i < MAX_RETRIES; i++ {
		select {
		case <-ch.ctx.Done():
			slog.Warn("[KafkaCommitHandler] Context canceled, stopping commit")
			return ch.ctx.Err()
		default:

			_, err := ch.consumer.CommitMessage(msg)
			if err == nil {
				slog.Info("[KafkaCommitHandler] Successfully committed offset",
					slog.String("partition", fmt.Sprintf("%d", msg.TopicPartition.Partition)),
					slog.String("offset", fmt.Sprintf("%d", msg.TopicPartition.Offset)))
				return nil
			}
			slog.Warn("[KafkaCommitHandler] Failed to commit offset, retrying...",
				slog.Int("attempt", i+1),
				slog.String("error", err.Error()),
				slog.String("partition", fmt.Sprintf("%d", msg.TopicPartition.Partition)),
				slog.String("offset", fmt.Sprintf("%d", msg.TopicPartition.Offset)))

			if kafkaErr, ok := err.(kafka.Error); ok && kafkaErr.Code() == kafka.ErrAllBrokersDown {
				slog.Error("[KafkaCommitHandler] All Kafka brokers are down. Aborting commit")
				return err
			}

			time.Sleep(RETRY_DELAY)
		}
	}

	return fmt.Errorf("[KafkaCommitHandler] Failed to commit message after %d retries", MAX_RETRIES)
}
