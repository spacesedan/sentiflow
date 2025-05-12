package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"log/slog"
	"os"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/opensearch-project/opensearch-go/v4/opensearchapi"
	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/clients" // Import clients package
	"github.com/spacesedan/sentiflow/internal/db"      // SQLC generated package
	"github.com/spacesedan/sentiflow/internal/logging"
	"github.com/spacesedan/sentiflow/internal/models" // Assuming models are needed here
	"github.com/spacesedan/sentiflow/internal/streams"
)

const (
	ProcessingModeHeadlines = "headlines"
	ProcessingModeSentiment = "sentiment"
)

// processingMode determines which type of records this Lambda instance will process.
// Set via the PROCESSING_MODE environment variable.
var processingMode string

// init runs once per Lambda cold start
func init() {
	slog.Info("Lambda cold start: Initializing...")
	env := os.Getenv("APP_ENV")
	if env == "" {
		env = "dev" // Default environment
	}
	config.LoadEnv(env)  // Ensure this is safe to call multiple times or guarded
	logging.InitLogger() // Ensure this is safe to call multiple times or guarded

	// Initialize AWS clients if they are used by the record processing logic
	// For example, if processTopic needs to write to another DynamoDB table or S3.
	// clients.GetAWSConfig() // This will initialize the awsCfg and endpoint in clients package

	// Initialize OpenSearch client
	// Using context.Background() as this is a one-time initialization.
	_ = clients.GetOpensearchClient(context.Background())
	slog.Info("OpenSearch client initialized or initialization triggered.")

	// Initialize PostgreSQL client
	// Assuming you will create a GetPostgresClient function in your clients package
	// that handles initialization, similar to GetOpensearchClient.
	// _ = clients.GetPostgresClient(context.Background()) // Or however you design its init
	_, err := clients.GetPostgresClient(context.Background())
	if err != nil {
		log.Fatal(err)
	}
	slog.Info("PostgreSQL client initialization triggered.")
	// For now, we'll add a log message indicating it's a placeholder.
	// You will need to implement clients.GetPostgresClient() and its underlying logic.

	// Determine processing mode
	mode := os.Getenv("PROCESSING_MODE")
	switch mode {
	case ProcessingModeHeadlines:
		processingMode = ProcessingModeHeadlines
		slog.Info("Lambda configured to process HEADLINE records.")
	case ProcessingModeSentiment:
		processingMode = ProcessingModeSentiment
		slog.Info("Lambda configured to process SENTIMENT records.")
	default:
		processingMode = "" // Or a default, or panic
		slog.Error("PROCESSING_MODE environment variable not set or invalid. Must be 'headlines' or 'sentiment'. Lambda will not process records.", "receivedMode", mode)
		// Consider panicking here if a valid mode is critical:
		// panic("PROCESSING_MODE environment variable not set or invalid")
	}

	slog.Info("Initialization complete.", "environment", env, "processingMode", processingMode)
}

// HandleRequest is the main Lambda handler function.
// It processes a batch of DynamoDB stream records.
// This handler is specifically set up to process records assumed to be from the "headlines" stream.
func HandleRequest(ctx context.Context, event events.DynamoDBEvent) error {
	slog.Info("Received DynamoDB event", "recordCount", len(event.Records))

	// Get PostgreSQL client and sqlc Querier
	pgClient, err := clients.GetPostgresClient(ctx)
	if err != nil {
		slog.Error("Failed to get PostgreSQL client", "error", err)
		return err // Critical error, cannot proceed
	}
	queries := db.New(pgClient.Pool) // Renamed pgQueries to queries

	// Get OpenSearch client
	// OpenSearch client is initialized in init()
	osClient := clients.GetOpensearchClient(ctx).Client
	if osClient == nil {
		slog.Error("Failed to get OpenSearch client")
		return fmt.Errorf("opensearch client not initialized")
	}

	var headlinesToBatch []*models.Headline
	var sentimentResultsToBatch []*models.SentimentAnalysisResult

	for _, record := range event.Records {
		slog.Info("Processing record",
			"eventId", record.EventID,
			"eventName", record.EventName,
			"eventSourceArn", record.EventSourceArn)

		switch processingMode {
		case ProcessingModeHeadlines:
			// IMPORTANT: streams.ProcessHeadlineRecord must be adapted to return *models.Headline, error
			headline, err := streams.ProcessHeadlineRecord(ctx, record)
			if err != nil {
				slog.Error("Error processing headline record, continuing with next.", "eventId", record.EventID, "error", err)
				// Decide if you want to fail the whole batch or just skip this record
				// For now, skipping and continuing. If critical, return err.
				continue
			}
			if headline != nil {
				headlinesToBatch = append(headlinesToBatch, headline)
			}
		case ProcessingModeSentiment:
			// streams.ProcessSentimentRecord returns (*models.SentimentAnalysisResult, error)
			sentimentResult, processErr := streams.ProcessSentimentRecord(ctx, record) // Use a different var name for error
			if processErr != nil {
				slog.Error("Error processing sentiment record, continuing with next.", "eventId", record.EventID, "error", processErr)
				// Decide if you want to fail the whole batch or just skip this record
				continue
			}
			if sentimentResult != nil {
				sentimentResultsToBatch = append(sentimentResultsToBatch, sentimentResult)
			}
		default:
			slog.Error("Lambda started with an invalid or unset PROCESSING_MODE. Skipping record.", "eventId", record.EventID, "configuredMode", processingMode)
		}
	}

	// Batch insert headlines if any were processed
	if len(headlinesToBatch) > 0 {
		slog.Info("Batch inserting headlines to PostgreSQL", "count", len(headlinesToBatch))
		params := db.CreateHeadlinesBatchParams{
			Ids:              make([]string, len(headlinesToBatch)),
			Queries:          make([]string, len(headlinesToBatch)),
			Categories:       make([]string, len(headlinesToBatch)),
			SentimentScores:  make([]float32, len(headlinesToBatch)), // float32 for REAL
			MetaSources:      make([]string, len(headlinesToBatch)),
			MetaTitles:       make([]string, len(headlinesToBatch)),
			MetaAuthors:      make([]string, len(headlinesToBatch)),             // Based on compiler error, sqlc expects []string
			MetaDescriptions: make([]string, len(headlinesToBatch)),             // Based on compiler error, sqlc expects []string
			MetaPublishedAts: make([]pgtype.Timestamptz, len(headlinesToBatch)), // pgtype for TIMESTAMPTZ
			MetaUrls:         make([]string, len(headlinesToBatch)),
			MetaUrlToImages:  make([]string, len(headlinesToBatch)), // Based on compiler error, sqlc expects []string
		}
		for i, h := range headlinesToBatch {
			params.Ids[i] = h.ID
			params.Queries[i] = h.Query
			params.Categories[i] = h.Category
			params.SentimentScores[i] = h.SentimentScore
			params.MetaSources[i] = h.HeadlineMeta.Source
			params.MetaTitles[i] = h.HeadlineMeta.Title
			params.MetaAuthors[i] = h.HeadlineMeta.Author           // Assign string directly
			params.MetaDescriptions[i] = h.HeadlineMeta.Description // Assign string directly
			if parsedTime, pErr := time.Parse(time.RFC3339, h.HeadlineMeta.PublishedAt); pErr == nil {
				params.MetaPublishedAts[i] = pgtype.Timestamptz{Time: parsedTime, Valid: true}
			} else {
				params.MetaPublishedAts[i] = pgtype.Timestamptz{Valid: false}
				slog.Warn("Failed to parse MetaPublishedAt for headline, will insert NULL", "headlineID", h.ID, "publishedAt", h.HeadlineMeta.PublishedAt, "error", pErr)
			}
			params.MetaUrls[i] = h.HeadlineMeta.Url
			params.MetaUrlToImages[i] = h.HeadlineMeta.UrlToImage // Assign string directly
		}
		// If the generated CreateHeadlinesBatch expects a slice of params:
		batchResults := queries.CreateHeadlinesBatch(ctx, []db.CreateHeadlinesBatchParams{params})
		err = batchResults.Close() // Important to close to check for errors
		if err != nil {
			slog.Error("Error batch inserting headlines to PostgreSQL", "error", err)
			return err // Fail the entire Lambda invocation if batch insert fails
		}
		slog.Info("Successfully batch inserted headlines.")
	}

	// Batch insert sentiment results if any were processed
	if len(sentimentResultsToBatch) > 0 {
		slog.Info("Batch inserting sentiment results to PostgreSQL", "count", len(sentimentResultsToBatch))
		params := db.CreateSentimentResultsBatchParams{
			ContentIds:         make([]string, len(sentimentResultsToBatch)),
			HeadlineIds:        make([]string, len(sentimentResultsToBatch)), // Based on compiler error, sqlc expects []string
			HeadlineQueries:    make([]string, len(sentimentResultsToBatch)), // Based on compiler error, sqlc expects []string
			HeadlineCategories: make([]string, len(sentimentResultsToBatch)), // Based on compiler error, sqlc expects []string
			Sources:            make([]string, len(sentimentResultsToBatch)),
			RawContentTopics:   make([]string, len(sentimentResultsToBatch)), // Based on compiler error, sqlc expects []string
			TextsAnalyzed:      make([]string, len(sentimentResultsToBatch)),
			WereSummarized:     make([]bool, len(sentimentResultsToBatch)),
			OriginalTexts:      make([]string, len(sentimentResultsToBatch)),             // Based on compiler error, sqlc expects []string
			MetadataTimestamps: make([]pgtype.Timestamptz, len(sentimentResultsToBatch)), // pgtype for TIMESTAMPTZ
			MetadataAuthors:    make([]string, len(sentimentResultsToBatch)),             // Based on compiler error, sqlc expects []string
			MetadataSubreddits: make([]string, len(sentimentResultsToBatch)),             // Based on compiler error, sqlc expects []string
			MetadataPostIds:    make([]string, len(sentimentResultsToBatch)),             // Based on compiler error, sqlc expects []string
			MetadataUrls:       make([]string, len(sentimentResultsToBatch)),             // Based on compiler error, sqlc expects []string
			SentimentScores:    make([]float64, len(sentimentResultsToBatch)),            // float64 for DOUBLE PRECISION
			SentimentLabels:    make([]string, len(sentimentResultsToBatch)),
			Confidences:        make([]float64, len(sentimentResultsToBatch)), // float64 for DOUBLE PRECISION
		}
		for i, sr := range sentimentResultsToBatch {
			params.ContentIds[i] = sr.ContentID        // Simplified access due to embedding
			params.HeadlineIds[i] = sr.HeadlineID      // Simplified access
			params.HeadlineQueries[i] = sr.Query       // Simplified access
			params.HeadlineCategories[i] = sr.Category // Simplified access

			params.Sources[i] = sr.Source               // Simplified access
			params.RawContentTopics[i] = sr.Topic       // Simplified access
			params.TextsAnalyzed[i] = sr.Text           // Simplified access
			params.WereSummarized[i] = sr.WasSummarized // Simplified access
			params.OriginalTexts[i] = sr.OriginalText   // Simplified access
			// Simplified access for Metadata fields
			params.MetadataTimestamps[i] = pgtype.Timestamptz{Time: sr.Metadata.Timestamp, Valid: !sr.Metadata.Timestamp.IsZero()} // Simplified access
			params.MetadataAuthors[i] = sr.Metadata.Author                                                                         // Simplified access
			params.MetadataSubreddits[i] = sr.Metadata.Subreddit                                                                   // Simplified access
			params.MetadataPostIds[i] = sr.Metadata.PostID                                                                         // Simplified access
			params.MetadataUrls[i] = sr.Metadata.URL                                                                               // Simplified access
			// SentimentScore, SentimentLabel, Confidence are direct fields of SentimentAnalysisResult
			params.SentimentScores[i] = sr.SentimentScore
			params.SentimentLabels[i] = sr.SentimentLabel
			params.Confidences[i] = sr.Confidence
		}
		// If the generated CreateSentimentResultsBatch expects a slice of params:
		batchResults := queries.CreateSentimentResultsBatch(ctx, []db.CreateSentimentResultsBatchParams{params})
		err = batchResults.Close() // Important to close to check for errors
		if err != nil {
			slog.Error("Error batch inserting sentiment results to PostgreSQL", "error", err)
			return err // Fail the entire Lambda invocation if batch insert fails
		}
		slog.Info("Successfully batch inserted sentiment results.")
	}

	// Bulk index headlines to OpenSearch
	if len(headlinesToBatch) > 0 {
		slog.Info("Bulk indexing headlines to OpenSearch", "count", len(headlinesToBatch))
		var bulkRequestBody bytes.Buffer
		for _, h := range headlinesToBatch {
			meta := fmt.Sprintf(`{"index": {"_index": "headlines", "_id": "%s"}}%s`, h.ID, "\n")
			bulkRequestBody.WriteString(meta)
			data, err := json.Marshal(h)
			if err != nil {
				slog.Error("Failed to marshal headline for OpenSearch bulk request", "headlineID", h.ID, "error", err)
				// Decide if this error should fail the entire batch or just skip this document
				continue // Skipping this document for now
			}
			bulkRequestBody.Write(data)
			bulkRequestBody.WriteString("\n")
		}

		bulkReq := opensearchapi.BulkReq{
			Body: &bulkRequestBody,
			Params: opensearchapi.BulkParams{
				Refresh: "true", // Or "wait_for" or false depending on consistency needs
			},
		}
		res, err := osClient.Bulk(ctx, bulkReq)
		if err != nil {
			slog.Error("Error performing OpenSearch bulk request for headlines", "error", err)
			// Potentially return err here to fail the Lambda invocation
		} else {
			defer res.Inspect().Response.Body.Close() // Ensure body is closed
			if res.Inspect().Response.IsError() {
				bodyBytes, _ := io.ReadAll(res.Inspect().Response.Body)
				slog.Error("OpenSearch bulk request for headlines returned an error response", "statusCode", res.Inspect().Response.StatusCode, "responseBody", string(bodyBytes))
			} else {
				// Optionally, parse the response to check for item-level errors
				slog.Info("Successfully sent OpenSearch bulk request for headlines", "statusCode", res.Inspect().Response.StatusCode)
			}
		}
	}

	// Bulk index sentiment results to OpenSearch
	if len(sentimentResultsToBatch) > 0 {
		slog.Info("Bulk indexing sentiment results to OpenSearch", "count", len(sentimentResultsToBatch))
		var bulkRequestBody bytes.Buffer
		for _, sr := range sentimentResultsToBatch {
			meta := fmt.Sprintf(`{"index": {"_index": "sentiment_results", "_id": "%s"}}%s`, sr.ContentID, "\n")
			bulkRequestBody.WriteString(meta)
			data, err := json.Marshal(sr)
			if err != nil {
				slog.Error("Failed to marshal sentiment result for OpenSearch bulk request", "contentID", sr.ContentID, "error", err)
				continue // Skipping this document
			}
			bulkRequestBody.Write(data)
			bulkRequestBody.WriteString("\n")
		}

		bulkReq := opensearchapi.BulkReq{
			Body: &bulkRequestBody,
			Params: opensearchapi.BulkParams{
				Refresh: "true",
			},
		}
		res, err := osClient.Bulk(ctx, bulkReq)
		if err != nil {
			slog.Error("Error performing OpenSearch bulk request for sentiment results", "error", err)
		} else {
			defer res.Inspect().Response.Body.Close()
			if res.Inspect().Response.IsError() {
				bodyBytes, _ := io.ReadAll(res.Inspect().Response.Body)
				slog.Error("OpenSearch bulk request for sentiment results returned an error response", "statusCode", res.Inspect().Response.StatusCode, "responseBody", string(bodyBytes))
			} else {
				slog.Info("Successfully sent OpenSearch bulk request for sentiment results", "statusCode", res.Inspect().Response.StatusCode)
			}
		}
	}

	slog.Info("Successfully processed all records in the event.")
	return nil
}

func main() {
	// The aws-lambda-go library handles the main loop and passes events to HandleRequest.
	lambda.Start(HandleRequest)
}
