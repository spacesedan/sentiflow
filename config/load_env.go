package config

import (
	"github.com/subosito/gotenv"
	"golang.org/x/exp/slog"
)

func LoadEnv(env string) {
	envFile := "config/envs/.env." + env
	if err := gotenv.Load(envFile); err != nil {
		slog.Warn("No .env file found, using OS environment")
	}
}
