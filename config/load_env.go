package config

import (
	"log"

	"github.com/subosito/gotenv"
)

func LoadEnv(env string) {
	envFile := "config/envs/.env." + env
	if err := gotenv.Load(envFile); err != nil {
		log.Fatalf("Error loading %s file", envFile)
	}
}
