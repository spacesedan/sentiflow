package logging

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

func InitLogger() {
	var level slog.Level

	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG":
		level = slog.LevelDebug
	default:
		level = slog.LevelInfo
	}

	handler := tint.NewHandler(os.Stdout, &tint.Options{
		Level:      level,
		TimeFormat: time.Kitchen,
		AddSource:  true,
	})

	slog.SetDefault(slog.New(handler))
}
