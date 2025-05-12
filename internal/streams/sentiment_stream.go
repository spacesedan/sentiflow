package streams

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"

	"github.com/aws/aws-lambda-go/events"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/models"
)

// ProcessSentimentRecord handles a single DynamoDB stream record for a sentiment result.
// This function would be called by a Lambda handler configured for the sentiment analysis stream.
func ProcessSentimentRecord(ctx context.Context, record events.DynamoDBEventRecord) error {
	if record.EventName != "INSERT" {
		slog.Debug("Skipping non-INSERT event for sentiment record", "eventId", record.EventID, "eventName", record.EventName)
		return nil
	}

	newImage := record.Change.NewImage
	var result models.SentimentAnalysisResult

	// Use the UnmarshalEventStreamImage from internal/streams/unmarshal.go (implicitly, as it's in the same package)
	err := UnmarshalEventStreamImage(newImage, &result)
	if err != nil {
		slog.Error("Failed to unmarshal sentiment result from DynamoDB stream record",
			"eventId", record.EventID,
			"error", err.Error())
		return err
	}

	slog.Info("Successfully unmarshalled sentiment result",
		"eventId", record.EventID,
		"contentId", result.ContentID,
		"headline", result.Topic) // Assuming result.Topic from RawContent now holds the headline string

	if err := processSentimentResult(ctx, result); err != nil { // Assuming processSentimentResult might also need context
		return err // Propagate the error
	}
	return nil
}

// processSentimentResult is a placeholder for your business logic to handle a new sentiment analysis result.
// Adding context.Context in case it's needed for downstream calls.
func processSentimentResult(ctx context.Context, result models.SentimentAnalysisResult) error {
	slog.Info("Processing sentiment result", "contentId", result.ContentID, "label", result.SentimentLabel)

	// Index in OpenSearch
	opensearchClient := clients.GetOpensearchClient(ctx).Client
	if opensearchClient == nil {
		slog.Error("OpenSearch client is not initialized in processSentimentResult")
		return fmt.Errorf("opensearch client not initialized for contentId %s", result.ContentID)
	}

	resultJSON, err := json.Marshal(result)
	if err != nil {
		slog.Error("Failed to marshal sentiment result to JSON", "contentId", result.ContentID, "error", err)
		return fmt.Errorf("failed to marshal sentiment result %s to JSON: %w", result.ContentID, err)
	}

	indexReq := opensearchapi.IndexReq{
		Index:      "sentiment_results",
		DocumentID: result.ContentID,
		Body:       bytes.NewReader(resultJSON),
		Params: opensearchapi.IndexParams{ // Pass Refresh via Params
			Refresh: "true",
		},
	}

	res, err := opensearchClient.Index(ctx, indexReq) // Use client.Index
	if err != nil {
		slog.Error("Failed to index sentiment result in OpenSearch (call failed)", "contentId", result.ContentID, "error", err)
		return fmt.Errorf("opensearch client.Index call failed for contentId %s: %w", result.ContentID, err)
	}
	if res.Inspect().Response.IsError() {
		errMsg := fmt.Sprintf("OpenSearch returned an error during sentiment result indexing (response error) for contentId %s: status %s, details %s",
			result.ContentID, res.Inspect().Response.Status(), res.Inspect().Response.String())
		slog.Error(errMsg)
		return errors.New(errMsg)
	}

	slog.Info("Successfully indexed sentiment result in OpenSearch", "contentId", result.ContentID, "documentID", result.ContentID, "result", res.Result, "statusCode", res.Inspect().Response.StatusCode)

	//TODO: Insert/Update in PostgreSQL
	// Assuming you will have a postgresClient available, perhaps via clients.GetPostgresClient()
	// and it will have a method like StoreSentimentResult.
	/*
		pgClient := clients.GetPostgresClient() // Or however you access your initialized PG client
		if pgClient != nil { // Check if client is properly initialized
			err = pgClient.StoreSentimentResult(ctx, result) // Replace with actual method call
			if err != nil {
				slog.Error("Failed to store sentiment result in PostgreSQL", "contentId", result.ContentID, "error", err)
				// Decide on error handling
			} else {
				slog.Info("Successfully stored sentiment result in PostgreSQL", "contentId", result.ContentID)
			}
		} else {
			slog.Error("PostgreSQL client not initialized. Skipping database insertion for sentiment result.", "contentId", result.ContentID)
		}
	*/
	slog.Info("PostgreSQL insertion placeholder for sentiment result. TODO: Implement client and uncomment.", "contentId", result.ContentID)
	return nil
}
