package main

import (
	"context"
	"log/slog"
	"os"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/spacesedan/sentiflow/config"
	"github.com/spacesedan/sentiflow/internal/clients" // Import clients package
	"github.com/spacesedan/sentiflow/internal/logging"
	"github.com/spacesedan/sentiflow/internal/streams"
)

const (
	ProcessingModeHeadlines = "headlines"
	ProcessingModeSentiment = "sentiment"
)

var (
	// processingMode determines which type of records this Lambda instance will process.
	// Set via the PROCESSING_MODE environment variable.
	processingMode string
)

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
	// slog.Info("PostgreSQL client initialization triggered.")
	// For now, we'll add a log message indicating it's a placeholder.
	// You will need to implement clients.GetPostgresClient() and its underlying logic.
	slog.Info("PostgreSQL client initialization placeholder. TODO: Implement and uncomment client initialization.")

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

	for _, record := range event.Records {
		// Log basic record information
		slog.Info("Processing record",
			"eventId", record.EventID,
			"eventName", record.EventName,
			"eventSourceArn", record.EventSourceArn)

		// Assuming this Lambda is configured for the HEADLINES_TABLE_NAME stream.
		// If it could receive records from other streams, you'd need to check
		// record.EventSourceArn or other attributes to route appropriately.

		var err error
		switch processingMode {
		case ProcessingModeHeadlines:
			err = streams.ProcessHeadlineRecord(ctx, record)
			if err != nil {
				slog.Error("Error processing headline record, failing batch.", "eventId", record.EventID, "error", err)
				return err // Fail the batch for this record
			}
		case ProcessingModeSentiment:
			err = streams.ProcessSentimentRecord(ctx, record)
			if err != nil {
				slog.Error("Error processing sentiment record, failing batch.", "eventId", record.EventID, "error", err)
				return err // Fail the batch for this record
			}
		default:
			slog.Error("Lambda started with an invalid or unset PROCESSING_MODE. Skipping record.", "eventId", record.EventID, "configuredMode", processingMode)
			// Optionally, return an error here to indicate a critical configuration issue
			// return fmt.Errorf("invalid PROCESSING_MODE: %s", processingMode)
			// For now, we'll skip the record if the mode is not set correctly after init.
			// The init function already logs an error if the mode is invalid at startup.
		}
	}

	slog.Info("Successfully processed all records in the event.")
	return nil
}

func main() {
	// The aws-lambda-go library handles the main loop and passes events to HandleRequest.
	lambda.Start(HandleRequest)
}
