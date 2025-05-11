package main

import (
	"context"
	"log/slog"
	"os"
	"strconv"
	"time"

	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/logging"
	topicgeneration "github.com/spacesedan/sentiflow/internal/topic_generation"
)

const (
	defaultApplicationTimeoutMinutes = 20
)

func main() {
	appEnv := os.Getenv("APP_ENV")
	if appEnv == "" {
		appEnv = "dev"
	}
	config.LoadEnv(appEnv) // Your existing environment loader
	logging.InitLogger()

	slog.Info("Starting SentiFlow Topic Generator...")

	// Determine application timeout
	timeoutMinutesStr := os.Getenv("APP_TIMEOUT_MINUTES")
	timeoutMinutes, err := strconv.Atoi(timeoutMinutesStr)
	if err != nil || timeoutMinutes <= 0 {
		slog.Info("Using default application timeout", slog.Int("minutes", defaultApplicationTimeoutMinutes))
		timeoutMinutes = defaultApplicationTimeoutMinutes
	} else {
		slog.Info("Application timeout set from environment", slog.Int("minutes", timeoutMinutes))
	}
	applicationTimeout := time.Duration(timeoutMinutes) * time.Minute

	ctx, cancel := context.WithTimeout(context.Background(), applicationTimeout)
	defer cancel()

	articles, err := clients.GetNewsAPIClient().GetTopHeadlinesByCategory()
	if err != nil {
		slog.Warn("[TopicGenerator] Failed to get Top headlines from the NewsAPI",
			slog.String("error", err.Error()))
		os.Exit(1)
	}

	topicgeneration.GenerateTopicsFromHeadlines(ctx, articles)

	// Check if the context was cancelled (e.g., due to timeout).
	if ctx.Err() == context.DeadlineExceeded {
		slog.Error("[TopicGenerator] Application timed out.", slog.Duration("duration", applicationTimeout))
		os.Exit(1) // Exit with an error code if timeout occurred.
	} else if ctx.Err() != nil {
		slog.Error("[TopicGenerator] Application context error.", slog.String("error", ctx.Err().Error()))
		os.Exit(1)
	}

	slog.Info("[TopicGenerator] Topic generation completed successfully.")
}
