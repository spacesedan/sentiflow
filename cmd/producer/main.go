package main

import (
	"log/slog"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/logging"
	"github.com/spacesedan/sentiflow/internal/processing"
)

type ProducerConfig struct {
	debug bool
}

func main() {
	_ = ProducerConfig{debug: true}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	config.LoadEnv(env)
	logging.InitLogger()

	for {
		err := clients.InitKafka()
		if err == nil {
			break
		}

		slog.Warn("Kafka init failed, retrying...", slog.String("error", err.Error()))
		time.Sleep(5 * time.Second)
	}
	defer clients.CloseKafka()

	// Load intervals from environment
	topicFetchInterval, err := strconv.Atoi(os.Getenv("TOPIC_FETCH_INTERVAL"))
	if err != nil {
		topicFetchInterval = 21600 // Default to 6 hours (in seconds)
	}

	redditFetchInterval, err := strconv.Atoi(os.Getenv("REDDIT_FETCH_INTERVAL"))
	if err != nil {
		redditFetchInterval = 1800 // Default to 30 minutes (in seconds)
	}

	topicTicker := time.NewTicker(time.Duration(topicFetchInterval) * time.Second)
	redditTicker := time.NewTicker(time.Duration(redditFetchInterval) * time.Second)
	defer topicTicker.Stop()
	defer redditTicker.Stop()

	processing.InitCategoryHelpers()

	// ✅ Run both fetchers immediately
	go func() {
		processing.FetchAndStoreTopics()
		processing.FetchRedditContentForTopics()
	}()

	// Handle graceful shutdown
	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	for {
		select {
		case <-topicTicker.C:
			go processing.FetchAndStoreTopics()

		case <-redditTicker.C:
			go processing.FetchRedditContentForTopics()

		case <-stopChan:
			slog.Info("🔄 Shutting down producer gracefully...")
			return
		}
	}
}
