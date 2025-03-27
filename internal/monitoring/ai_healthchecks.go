package monitoring

import (
	"context"
	"log/slog"
	"sync/atomic"
	"time"

	"github.com/spacesedan/sentiflow/internal/clients"
)

const HEALTHCHECK_TIMER = 15

func MonitorSummarizerHealth(ctx context.Context, healthy *atomic.Bool) {
	ticker := time.NewTicker(time.Second * HEALTHCHECK_TIMER)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			isHealthy := clients.GetHuggingFaceClient().SummarizerHealthCheck()
			healthy.Store(isHealthy)
			if !isHealthy {
				slog.Warn("[HealthCheck] Summarizer is unhealthy")
			}
		}
	}
}

func MonitorAnalyzerHealth(ctx context.Context, healthy *atomic.Bool) {
	ticker := time.NewTicker(time.Second * HEALTHCHECK_TIMER)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			isHealthy := clients.GetHuggingFaceClient().AnalyzerHealthCheck()
			healthy.Store(isHealthy)
			if !isHealthy {
				slog.Warn("[HealthCheck] Analyzer is unhealthy")
			}
		}
	}
}
