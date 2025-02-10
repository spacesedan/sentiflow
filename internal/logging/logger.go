package logging

import (
	"log/slog"
	"os"
	"time"

	"github.com/lmittmann/tint"
)

func InitLogger() {
	handler := tint.NewHandler(os.Stdout, &tint.Options{
		Level:      slog.LevelInfo,
		TimeFormat: time.Kitchen,
		AddSource:  true,
	})

	slog.SetDefault(slog.New(handler))
}
