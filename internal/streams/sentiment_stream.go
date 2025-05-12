package streams

import (
	"context"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/spacesedan/sentiflow/internal/models"
)

// ProcessSentimentRecord handles a single DynamoDB stream record for a sentiment result.
// This function would be called by a Lambda handler configured for the sentiment analysis stream.
// It returns the processed sentiment result or an error.
func ProcessSentimentRecord(ctx context.Context, record events.DynamoDBEventRecord) (*models.SentimentAnalysisResult, error) {
	if record.EventName != "INSERT" {
		slog.Debug("Skipping non-INSERT event for sentiment record", "eventId", record.EventID, "eventName", record.EventName)
		return nil, nil
	}

	newImage := record.Change.NewImage
	var result models.SentimentAnalysisResult

	// Use the UnmarshalEventStreamImage from internal/streams/unmarshal.go (implicitly, as it's in the same package)
	err := UnmarshalEventStreamImage(newImage, &result)
	if err != nil {
		slog.Error("Failed to unmarshal sentiment result from DynamoDB stream record",
			"eventId", record.EventID,
			"error", err.Error())
		return nil, err
	}

	slog.Info("Successfully unmarshalled sentiment result",
		"eventId", record.EventID,
		"contentId", result.ContentID,
		"headlineID", result.HeadlineID, // Log the newly added HeadlineID
		"query", result.Query)

	if err := processSentimentResult(ctx, result); err != nil { // Assuming processSentimentResult might also need context
		return nil, err // Propagate the error
	}
	return &result, nil
}

// processSentimentResult is a placeholder for your business logic to handle a new sentiment analysis result.
// Adding context.Context in case it's needed for downstream calls.
func processSentimentResult(ctx context.Context, result models.SentimentAnalysisResult) error {
	slog.Info("Processing sentiment result", "contentId", result.ContentID, "label", result.SentimentLabel)

	// Business logic for a single sentiment result (if any) before batch operations.
	// For now, OpenSearch indexing and PostgreSQL insertion are handled in batch by the caller.

	return nil
}
