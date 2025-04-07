package main

import (
	"context"
	"os"
	"time"

	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/logging"
	"github.com/spacesedan/sentiflow/internal/streams"
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

	switch os.Getenv("DYNAMODB_TABLE") {
	case "Topics":
		streams.StartTopicStreamBatchFlusher(ctx)
		streams.StartTopicStreamConsumer(ctx)
	case "SentimentResults":
		streams.StartSentimentStreamBatchFlusher(ctx)
		streams.StartSentimentStreamConsumer(ctx)
	}
}
