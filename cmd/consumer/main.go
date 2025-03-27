package main

import (
	"context"
	"log/slog"
	"os"
	"sync/atomic"
	"time"

	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/consumers"
	"github.com/spacesedan/sentiflow/internal/logging"
	"github.com/spacesedan/sentiflow/internal/monitoring"
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

	summarizerHealthy := &atomic.Bool{}
	analyzerHealthy := &atomic.Bool{}
	summarizerHealthy.Store(true)
	analyzerHealthy.Store(true)

	go monitoring.MonitorAnalyzerHealth(ctx, analyzerHealthy)
	go monitoring.MonitorSummarizerHealth(ctx, summarizerHealthy)

	kafka_client.RegisterConsumer(kafka_client.KAFKA_TOPIC_RAW_CONTENT, consumers.StartRawContentConsumer)
	kafka_client.RegisterConsumer(kafka_client.KAFKA_TOPIC_SUMMARY_REQUEST, consumers.WrapConsumer(
		consumers.StartSummaryConsumer, summarizerHealthy).WithHealthCheck(summarizerHealthy).Handler(),
	)
	kafka_client.RegisterConsumer(kafka_client.KAFKA_TOPIC_SUMMARY_REQUEST, consumers.WrapConsumer(
		consumers.StartSentimentAnalysisConsumer, analyzerHealthy).WithHealthCheck(analyzerHealthy).Handler())
	kafka_client.RegisterConsumer(kafka_client.KAFKA_TOPIC_SENTIMENT_RESULTS, consumers.StartResultsConsumer)

	if err := kafka_client.StartConsumer(ctx, cfg); err != nil {
		slog.Error("[Main] Failed to start consumer",
			slog.String("error", err.Error()))
	}
}
