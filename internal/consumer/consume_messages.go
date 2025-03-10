package consumer

import (
	"context"
	"encoding/json"
	"log/slog"

	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/models"
)

func ConsumeMessages(ctx context.Context) {
	for {
		msg, err := kafka_client.KafkaMessageIterator(ctx)
		if err != nil {
			slog.Error("[Consumer] Failed to fetch message", slog.String("error", err.Error()))
		}

		var post models.RedditPost
		if err := json.Unmarshal(msg.Value, &post); err != nil {
			slog.Warn("[Consumer] Failed to deserialize message, skipping...",
				slog.String("error", err.Error()))
			continue
		}

	}
}
