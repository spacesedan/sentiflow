package streams

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi" // Use opensearchapi package
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/models"
)

// ProcessHeadlineRecord handles a single DynamoDB stream record for a headline.
// It unmarshals the record and calls the business logic to process the headline.
func ProcessHeadlineRecord(ctx context.Context, record events.DynamoDBEventRecord) error {
	if record.EventName != "INSERT" { // In Lambda events, OperationType is a string e.g. "INSERT", "MODIFY", "REMOVE"
		slog.Debug("Skipping non-INSERT event for headline record", "eventId", record.EventID, "eventName", record.EventName)
		return nil
	}

	newImage := record.Change.NewImage

	var headline models.Headline
	err := UnmarshalEventStreamImage(newImage, &headline)
	if err != nil {
		slog.Error("Failed to unmarshal headline from DynamoDB stream record",
			"eventId", record.EventID,
			"error", err.Error())
		return err
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
		return err // Propagate the error
	}

	return nil
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

	// Index in OpenSearch
	opensearchClient := clients.GetOpensearchClient(ctx).Client
	if opensearchClient == nil {
		slog.Error("OpenSearch client is not initialized in processHeadline")
		return fmt.Errorf("opensearch client not initialized for headlineId %s", headline.ID)
	}

	headlineJSON, err := json.Marshal(headline)
	if err != nil {
		slog.Error("Failed to marshal headline to JSON", "headlineId", headline.ID, "error", err)
		return fmt.Errorf("failed to marshal headline %s to JSON: %w", headline.ID, err)
	}

	documentID := headline.ID

	indexReq := opensearchapi.IndexReq{
		Index:      "headlines",
		DocumentID: documentID,
		Body:       bytes.NewReader(headlineJSON),
		Params: opensearchapi.IndexParams{
			Refresh: "true",
		},
	}

	res, err := opensearchClient.Index(ctx, indexReq)
	if err != nil {
		slog.Error("Failed to index headline in OpenSearch (call failed)", "headlineId", headline.ID, "error", err)
		return fmt.Errorf("opensearch client.Index call failed for headline %s: %w", headline.ID, err)
	}
	if res.Inspect().Response.IsError() {
		errMsg := fmt.Sprintf("OpenSearch returned an error during headline indexing (response error) for headlineId %s: status %s, details %s",
			headline.ID, res.Inspect().Response.Status(), res.Inspect().Response.String())
		slog.Error(errMsg)
		return errors.New(errMsg)
	}

	slog.Info("Successfully indexed headline in OpenSearch", "headlineId", headline.ID, "documentID", documentID, "result", res.Result, "statusCode", res.Inspect().Response.StatusCode)

	// Insert into PostgreSQL
	// Assuming you will have a postgresClient available, perhaps via clients.GetPostgresClient()
	// and it will have a method like InsertHeadline.
	/*
		pgClient := clients.GetPostgresClient() // Or however you access your initialized PG client
		if pgClient != nil { // Check if client is properly initialized
			err = pgClient.InsertHeadline(ctx, headline) // Replace with actual method call
			if err != nil {
				slog.Error("Failed to insert headline into PostgreSQL", "headlineId", headline.ID, "error", err)
				// Decide on error handling: return error, log and continue, etc.
			} else {
				slog.Info("Successfully inserted headline into PostgreSQL", "headlineId", headline.ID)
			}
		} else {
			slog.Error("PostgreSQL client not initialized. Skipping database insertion for headline.", "headlineId", headline.ID)
		}
	*/
	slog.Info("PostgreSQL insertion placeholder for headline. TODO: Implement client and uncomment.", "headlineId", headline.ID)
	return nil
}
