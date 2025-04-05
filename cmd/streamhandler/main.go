package main

import (
	"context"
	"os"
	"time"

	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/logging"
)

func mani() {
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev"
	}
	config.LoadEnv(env)
	logging.InitLogger()

	ctx, cancel := context.WithTimeout(context.Background(), time.Minute*2)
	defer cancel()
}
