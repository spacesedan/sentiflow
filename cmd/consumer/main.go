package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client/consumers"
	"github.com/spacesedan/sentiflow/internal/logging"
)

func main() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	config.LoadEnv(env)
	logging.InitLogger()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	cfg := kafka_client.GetKafkaConfig()

	for {
		err := kafka_client.InitProducer(cfg)
		if err == nil {
			break
		}

		slog.Warn("Kafka init failed, retrying...", slog.String("error", err.Error()))
		time.Sleep(5 * time.Second)
	}
	defer kafka_client.CloseProducer()

	kafka_client.RegisterConsumer(kafka_client.KAFKA_TOPIC_SENTIMENT_REQUEST, consumers.StartRequestConsumer)
	kafka_client.RegisterConsumer(kafka_client.KAFKA_TOPIC_SENTIMENT_BATCHES, consumers.StartBatchConsumer)
	kafka_client.RegisterConsumer(kafka_client.KAFKA_TOPIC_SENTIMENT_RESULTS, consumers.StartResultsConsumer)
	kafka_client.RegisterConsumer(kafka_client.KAFKA_TOPIC_SENTIMENT_AMBIGUOUS, consumers.StartAmbiguousConsumer)

	if err := kafka_client.StartConsumer(ctx, cfg); err != nil {
		slog.Error("[Main] Failed to start consumer",
			slog.String("error", err.Error()))
	}
}
