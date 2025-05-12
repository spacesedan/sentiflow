package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/logging"
	"github.com/spacesedan/sentiflow/internal/producer"
)

func main() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	config.LoadEnv(env)
	logging.InitLogger()

	ctx, cancel := context.WithCancel(context.Background())

	for {
		err := kafka_client.InitProducer(ctx)
		if err == nil {
			break
		}

		slog.Warn("Kafka init failed, retrying...", slog.String("error", err.Error()))
		time.Sleep(5 * time.Second)
	}
	defer kafka_client.CloseProducer()

	clients.InitValkey()
	defer clients.CloseValkey()

	redditFetchInterval, err := strconv.Atoi(os.Getenv("REDDIT_FETCH_INTERVAL"))
	if err != nil {
		redditFetchInterval = 300 // Default to 5 minutes (in seconds)
	}

	redditTicker := time.NewTicker(time.Duration(redditFetchInterval) * time.Second)
	defer redditTicker.Stop()

	producer.InitCategoryHelpers()

	// Handle graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	producer.FetchRedditContentForHeadlines(ctx)

	for {
		select {

		case <-redditTicker.C:
			producer.FetchRedditContentForHeadlines(ctx)

		case <-stopChan:
			slog.Info("Shutting down producer gracefully...")
			cancel()
			return
		}
	}
}
