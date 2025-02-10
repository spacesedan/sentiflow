package main

import (
	"log/slog"
	"os"

	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/logging"
	"github.com/spacesedan/sentiflow/internal/processing"
)

const (
	NEWS_API_ENDPOINT = "https://newsapi.org/v2/top-headlines?country=us&pageSize=100&apiKey="
)

type ProducerConfig = struct {
	debug bool
}

func main() {
	_ = ProducerConfig{
		debug: true,
	}

	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	config.LoadEnv(env)
	logging.InitLogger()

	topHeadlines, err := clients.GetNewsAPIClient().GetTopHeadlines()
	if err != nil {
		panic(err)
	}

	topics, err := processing.GenerateTopicsFromHeadlines(topHeadlines)
	if err != nil {
		panic(err)
	}
	for _, t := range topics.Topics {
		slog.Info("", slog.String("topic", t.Topic), slog.String("category", t.Category))
	}
}
