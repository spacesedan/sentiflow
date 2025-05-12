package streams

import (
	"context"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/spacesedan/sentiflow/internal/models"
)

// ProcessHeadlineRecord handles a single DynamoDB stream record for a headline.
// It unmarshals the record and calls the business logic to process the headline.
// It returns the processed headline or an error.
func ProcessHeadlineRecord(ctx context.Context, record events.DynamoDBEventRecord) (*models.Headline, error) {
	if record.EventName != "INSERT" { // In Lambda events, OperationType is a string e.g. "INSERT", "MODIFY", "REMOVE"
		slog.Debug("Skipping non-INSERT event for headline record", "eventId", record.EventID, "eventName", record.EventName)
		return nil, nil
	}

	newImage := record.Change.NewImage

	var headline models.Headline
	err := UnmarshalEventStreamImage(newImage, &headline)
	if err != nil {
		slog.Error("Failed to unmarshal headline from DynamoDB stream record",
			"eventId", record.EventID,
			"error", err.Error())
		return nil, err
	}

	slog.Info("Successfully unmarshalled headline",
		"eventId", record.EventID,
		"headlineId", headline.ID,
		"query", headline.Query,
		"category", headline.Category,
		"sentimentScore", headline.SentimentScore,
		"source", headline.HeadlineMeta.Source,
		"title", headline.HeadlineMeta.Title,
		"author", headline.HeadlineMeta.Author,
		"publishedAt", headline.HeadlineMeta.PublishedAt)

	// processHeadline contains the actual business logic for what to do with the headline.
	// Consider if this needs to be a goroutine or if synchronous processing is fine.
	// For Lambda, synchronous processing per record is often simpler to manage unless
	// processHeadline involves long-running I/O operations.
	if err := processHeadline(ctx, headline); err != nil { // Assuming processHeadline might also need context
		// Error is already logged within processHeadline
		return nil, err // Propagate the error
	}

	return &headline, nil
}

// processHeadline is a placeholder for your business logic to handle a new headline.
// It might involve database operations, API calls, etc.
// Adding context.Context in case it's needed for downstream calls.
func processHeadline(ctx context.Context, headline models.Headline) error {
	slog.Info("Processing headline",
		"headlineId", headline.ID,
		"query", headline.Query,
		"category", headline.Category,
		"sentimentScore", headline.SentimentScore,
		"source", headline.HeadlineMeta.Source,
		"title", headline.HeadlineMeta.Title,
		"author", headline.HeadlineMeta.Author,
		"description", headline.HeadlineMeta.Description,
		"publishedAt", headline.HeadlineMeta.PublishedAt,
		"url", headline.HeadlineMeta.Url,
		"urlToImage", headline.HeadlineMeta.UrlToImage)

	// Business logic for a single headline (if any) before batch operations.
	// For now, OpenSearch indexing and PostgreSQL insertion are handled in batch by the caller.

	return nil
}
