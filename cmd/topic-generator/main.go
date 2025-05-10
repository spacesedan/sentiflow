package main

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/logging"
	topicgeneration "github.com/spacesedan/sentiflow/internal/topic_generation"
)

func main() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	config.LoadEnv(env)
	logging.InitLogger()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()

	articles, err := clients.GetNewsAPIClient().GetTopHeadlinesByCategory()
	if err != nil {
		slog.Warn("[TopicGenerator] Failed to get Top headlines from the NewsAPI",
			slog.String("error", err.Error()))
		os.Exit(1)
	}
	topicgeneration.GenerateTopicsFromHeadlines(ctx, articles)
	slog.Info("[TopicGenerator] Topic generation completed successfully")
}
