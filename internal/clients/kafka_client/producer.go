package kafka_client

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spacesedan/sentiflow/internal/models"
)

var (
	producer *kafka.Producer
	initOnce sync.Once
)

func generateTransactionalID(baseID string) string {
	envTransactionalID := os.Getenv("KAFKA_PRODUCER_ID")
	if envTransactionalID != "" {
		return envTransactionalID
	}

	now := time.Now().Unix()
	pid := os.Getpid()
	return fmt.Sprintf("%s-%d-%d-producer", baseID, now, pid)
}

func InitProducer(ctx context.Context) error {
	var initErr error

	initOnce.Do(func() {
		p, err := createNewProducer(ctx)
		if err != nil {
			initErr = fmt.Errorf("[KafkaClient] Failed to initialize producer: %w", err)
			return
		}

		producer = p
	})
	return initErr
}

func createNewProducer(ctx context.Context) (*kafka.Producer, error) {
	cfg := GetKafkaConfig()
	transactionalID := generateTransactionalID(cfg.Topic)
	slog.Info("[KafkaClient] Initializing Kafka Producer...",
		slog.String("transactional_id", transactionalID))
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers":                     cfg.Broker,
		"security.protocol":                     "PLAINTEXT", // Force PLAINTEXT
		"api.version.request":                   "true",      // Ensure correct API version request
		"enable.idempotence":                    true,
		"acks":                                  "all",
		"max.in.flight.requests.per.connection": 1,
		"transactional.id":                      transactionalID,
		"transaction.timeout.ms":                60000,
		"message.timeout.ms":                    50000,
		"delivery.timeout.ms":                   50000,
		"retries":                               1000000,
		"retry.backoff.ms":                      500,
		"socket.timeout.ms":                     60000,
		"message.send.max.retries":              10,
		"queue.buffering.max.ms":                100,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}
	if err := p.InitTransactions(ctx); err != nil {
		return nil, fmt.Errorf("failed to init transactions: %w", err)
	}
	slog.Info("[KafkaClient] Kafka Producer initialized successfully",
		slog.String("transactional_id", transactionalID))

	return p, nil
}

func CloseProducer() {
	slog.Info("[KafkaClient] Shutting down Kafka producer...")
	if producer != nil {
		slog.Info("[KafkaClient] Flushing Kafka producer before shutdown...")
		if remaining := producer.Flush(5000); remaining > 0 {
			slog.Warn("[KafkaClient] Not all messages were delivered before shutdown",
				slog.Int("remaining", remaining))
		}
		producer.Close()
		slog.Info("[KafkaClient] Kafka producer shut down")
	}
}

// PublishToKafka sends a Reddit post to Kafka
func PublishToKafka(ctx context.Context, topic string, message interface{}) error {
	const maxBeginRetries = 5
	for i := 0; i < maxBeginRetries; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		err := producer.BeginTransaction()
		if err == nil {
			break
		}
		if kafkaErr, ok := err.(kafka.Error); ok && kafkaErr.Code() == kafka.ErrState {
			slog.Warn("[KafkaClient] Producer still aborting, waiting before retry",
				slog.Int("attempt", i+1))
			time.Sleep(2 * time.Second)
			continue
		}

		return fmt.Errorf("[KafkaClient] failed to begin transaction: %v", err)
	}

	postID := extractPostID(message)

	jsonData, err := json.Marshal(message)
	if err != nil {
		abortTransaction(ctx)
		return fmt.Errorf("[KafkaClient] failed to marshal message: %w", err)
	}

	msg := &kafka.Message{
		TopicPartition: kafka.TopicPartition{Topic: &topic, Partition: kafka.PartitionAny},
		Key:            []byte(postID),
		Value:          jsonData,
	}

	const maxBackoff = time.Second * 30
	var produceErr error

	for i := 0; i < MAX_RETRIES; i++ {
		select {
		case <-ctx.Done():
			slog.Warn("[KafkaClient] Context cancelled during publish",
				slog.String("post_id", postID))
			abortTransaction(ctx)
			return ctx.Err()
		default:
		}
		deliveryChan := make(chan kafka.Event, 1)

		produceErr = producer.Produce(msg, deliveryChan)
		if produceErr != nil {
			kafkaErr, ok := produceErr.(kafka.Error)
			if !ok || !isRetryableKafkaError(kafkaErr) {
				slog.Error("[KafkaClient] Non-retryable produce error",
					slog.String("error", produceErr.Error()),
					slog.String("post_id", postID),
					slog.String("topic", topic))
				abortTransaction(ctx)
				return produceErr
			}
			backoff := RETRY_DELAY * (1 << i)
			if backoff > maxBackoff {
				backoff = maxBackoff
			}

			recreateProducer(ctx, kafkaErr)

			slog.Warn("[KafkaClient] Failed to produce message, retrying...",
				slog.Int("attempt", i+1),
				slog.Duration("backoff", backoff),
				slog.String("error", produceErr.Error()),
				slog.String("post_id", postID),
				slog.String("topic", topic))

			time.Sleep(backoff)
			continue
		}

		ev := <-deliveryChan
		m, ok := ev.(*kafka.Message)
		if !ok || m.TopicPartition.Error != nil {
			deliveryErr := m.TopicPartition.Error
			slog.Warn("[KafkaClient] Message delivery failed, aborting transaction",
				slog.String("post_id", postID),
				slog.String("topic", topic),
				slog.String("error", deliveryErr.Error()))
			abortTransaction(ctx)
			return fmt.Errorf("[KafkaClient] message delivery failed: %w", deliveryErr)
		}

		break

	}

	if produceErr != nil {
		abortTransaction(ctx)
		slog.Error("[KafkaClient] Max retries exceeded while producing",
			slog.String("topic", topic),
			slog.String("post_id", postID),
			slog.String("error", produceErr.Error()))
		return fmt.Errorf("[KafkaClient] failed to produce message after retries: %w", produceErr)
	}

	if err := commitTransaction(ctx); err != nil {
		return err
	}
	slog.Info("[KafkaClient] Published Content to Kafka transactionally",
		slog.String("topic", topic),
		slog.String("post_id", postID))
	return nil
}

func extractPostID(message interface{}) string {
	switch msg := message.(type) {
	case models.RedditPost:
		return msg.PostID
	case models.RawContent:
		return msg.ContentID
	default:
		return "unknown"
	}
}

func abortTransaction(ctx context.Context) {
	if err := producer.AbortTransaction(ctx); err != nil {
		slog.Warn("[KafkaClient] Failed to abort transaction",
			slog.String("error", err.Error()))
		if kafkaErr, ok := err.(kafka.Error); ok {
			recreateProducer(ctx, kafkaErr)
		}
	}
}

func recreateProducer(ctx context.Context, err kafka.Error) {
	switch err.Code() {
	case kafka.ErrInvalidProducerEpoch,
		kafka.ErrFatal,
		kafka.ErrState,
		kafka.ErrInvalidProducerIDMapping:
		slog.Warn("[KafkaClient] Fatal error on abort. Reinitializing producer...")

		producer.Close()
		newProducer, newErr := createNewProducer(ctx)
		if newErr != nil {
			slog.Error("[KafkaClient] Failed to reinitialize producer after fatal abort",
				slog.String("error", newErr.Error()))
		} else {
			producer = newProducer
		}

	default:
		slog.Info("[KafkaClient] Recreate Skipped: error code not recoverable",
			slog.String("code", err.Code().String()))
	}
}

func commitTransaction(ctx context.Context) error {
	for i := 0; i < 3; i++ {
		if err := producer.CommitTransaction(ctx); err == nil {
			return nil
		}
		slog.Warn("[KafkaClient] Failed to commit transaction, retrying...",
			slog.Int("attempt", i+1))
	}

	return errors.New("[KafkaClient] Failed to commit transaction after 3 retries")
}
