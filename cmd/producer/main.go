package main

import (
	"fmt"
	"log/slog"
	"os"

	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/logging"
	"github.com/spacesedan/sentiflow/internal/processing"
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
	fmt.Println(len(topics.Topics))
	for _, t := range topics.Topics {
		slog.Info("", slog.String("topic", t.Topic), slog.String("category", t.Category))
	}

	_, err = clients.GetRedditClient().FetchSubredditPosts("technology", "tesla")
	if err != nil {
		panic(err)
	}
}
