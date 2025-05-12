package db

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/models"
)

const (
	HEADLINES_TABLE_NAME          = "Headlines"
	SENTIMENT_ANALYSIS_TABLE_NAME = "SentimentResults"
)

var dbClient *dynamodb.Client

func InitDynamoDB() {
	dbClient = clients.GetDynamoDBClient()
}

// StoreBatchedHeadlines store batches of incoming headlines, dynamo can only do 25 at a time
func StoreBatchedHeadlines(ctx context.Context, headlines []models.Headline) error {
	if dbClient == nil {
		dbClient = clients.GetDynamoDBClient()
	}

	const maxBatchSize = 25
	for i := 0; i < len(headlines); i += maxBatchSize {
		select {
		case <-ctx.Done():
			slog.Warn("[DynamoDB] context canceled")
			return ctx.Err()
		default:

			end := i + maxBatchSize
			if end > len(headlines) {
				end = len(headlines)
			}

			writeRequests := make([]types.WriteRequest, 0, maxBatchSize)

			for _, headline := range headlines[i:end] {
				writeRequests = append(writeRequests, types.WriteRequest{
					PutRequest: &types.PutRequest{
						Item: headlineToDynamonDBItem(headline),
					},
				})
			}

			out, err := dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					HEADLINES_TABLE_NAME: writeRequests,
				},
			})
			if err != nil {
				return fmt.Errorf("[DynamoDB] Failed to batch write topics: %w", err)
			}

			// Retry writing unprocessed topics
			retryCount := 0
			backoffDuration := time.Millisecond * 500
			for len(out.UnprocessedItems) > 0 && retryCount < 3 {
				time.Sleep(backoffDuration)
				backoffDuration *= 2
				slog.Warn("[DynamoDB] Retrying unprocessed items...",
					slog.Int("retry_attempt", retryCount+1),
					slog.Int("remaining_items", len(out.UnprocessedItems[HEADLINES_TABLE_NAME])),
				)

				out, err = dbClient.BatchWriteItem(context.TODO(), &dynamodb.BatchWriteItemInput{
					RequestItems: out.UnprocessedItems,
				})
				if err != nil {
					slog.Error("[DynamoDB] Error retrying batch write",
						slog.String("error", err.Error()))
					return fmt.Errorf("[DynamoDB] Failed to retry batch write: %w", err)
				}
				retryCount++
			}

			if len(out.UnprocessedItems) > 0 {
				slog.Error("[DynamoDB] Some items were not written even after retries",
					slog.Int("remaining_items", len(out.UnprocessedItems[HEADLINES_TABLE_NAME])))
			}
		}
	}
	slog.Info("[DynamoDB] Successfully stored all topics")
	return nil
}

// headlineToDynamonDBItem - helper function that converts a headline object into a dynamodb object that can be stored
func headlineToDynamonDBItem(headline models.Headline) map[string]types.AttributeValue {
	// TTL for headlines
	expirationTime := time.Now().Add(24 * time.Hour).Unix()

	item := make(map[string]types.AttributeValue)

	item["id"] = &types.AttributeValueMemberS{Value: headline.ID}
	item["query"] = &types.AttributeValueMemberS{Value: headline.Query}
	item["category"] = &types.AttributeValueMemberS{Value: headline.Category}
	item["expires_at"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expirationTime)}

	// TODO: hopefully this doesn't cause any bugs, see if it is possible to come up a different way to do this
	if headline.SentimentScore != 0 {
		item["sentiment_score"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", headline.SentimentScore)}
	}

	metadata := make(map[string]types.AttributeValue)

	metadata["source"] = &types.AttributeValueMemberS{Value: headline.HeadlineMeta.Source}
	metadata["author"] = &types.AttributeValueMemberS{Value: headline.HeadlineMeta.Author}
	metadata["title"] = &types.AttributeValueMemberS{Value: headline.HeadlineMeta.Title}
	metadata["description"] = &types.AttributeValueMemberS{Value: headline.HeadlineMeta.Description}
	metadata["publishedAt"] = &types.AttributeValueMemberS{Value: headline.HeadlineMeta.PublishedAt}
	metadata["url"] = &types.AttributeValueMemberS{Value: headline.HeadlineMeta.Url}
	metadata["urlToImage"] = &types.AttributeValueMemberS{Value: headline.HeadlineMeta.UrlToImage}

	item["headline_meta"] = &types.AttributeValueMemberM{Value: metadata}

	return item
}

func GetAllHeadlines() ([]models.Headline, error) {
	if dbClient == nil {
		dbClient = clients.GetDynamoDBClient()
	}

	var headlines []models.Headline
	input := &dynamodb.ScanInput{
		TableName: aws.String(HEADLINES_TABLE_NAME),
	}

	paginator := dynamodb.NewScanPaginator(dbClient, input)

	for paginator.HasMorePages() {
		out, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("[DynamoDB] Scan for headlines failed: %w", err)
		}
		var headlinePage []models.Headline
		err = attributevalue.UnmarshalListOfMaps(out.Items, &headlinePage)
		if err != nil {
			slog.Error("[DynamoDB] Unable to marshal current topic page", slog.String("error", err.Error()))
			return nil, err
		}
		headlines = append(headlines, headlinePage...)

	}
	slog.Info("[DynamoDB] Successfully retrieved headlines", slog.Int("count", len(headlines)))
	return headlines, nil
}

func BatchInsertSentimentResults(ctx context.Context, results []models.SentimentAnalysisResult) error {
	var out *dynamodb.BatchWriteItemOutput
	var err error
	if dbClient == nil {
		dbClient = clients.GetDynamoDBClient()
	}

	writeRequests := make([]types.WriteRequest, 0, len(results))
	for _, result := range results {
		item := ResultToDynamoDBItem(result)
		writeRequests = append(writeRequests, types.WriteRequest{
			PutRequest: &types.PutRequest{
				Item: item,
			},
		})
	}

	out, err = dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			SENTIMENT_ANALYSIS_TABLE_NAME: writeRequests,
		},
	})
	if err != nil {
		return fmt.Errorf("[DynamoDB] Failed to batch write sentiment results: %w", err)
	}

	retryCount := 0
	backoff := 500 * time.Millisecond
	for len(out.UnprocessedItems) > 0 && retryCount < 3 {
		time.Sleep(backoff)
		backoff *= 2

		slog.Warn("[DynamoDB] Retrying unprocessed sentiment items...",
			slog.Int("attempt", retryCount+1),
			slog.Int("remaining", len(out.UnprocessedItems[SENTIMENT_ANALYSIS_TABLE_NAME])))

		out, err = dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
			RequestItems: out.UnprocessedItems,
		})
		if err != nil {
			return fmt.Errorf("[DynamoDB] Retry error %w", err)
		}

		retryCount++
	}

	if len(out.UnprocessedItems) > 0 {
		slog.Error("[DynamoDB] Some sentiment items failed after retries",
			slog.Int("remaining", len(out.UnprocessedItems[SENTIMENT_ANALYSIS_TABLE_NAME])))
	}

	slog.Info("[DynamoDB] Successfully stored sentiment results")

	return nil
}

func ResultToDynamoDBItem(result models.SentimentAnalysisResult) map[string]types.AttributeValue {
	item := make(map[string]types.AttributeValue)

	// Required fields (snake_case keys)
	item["content_id"] = &types.AttributeValueMemberS{Value: result.ContentID}
	item["query"] = &types.AttributeValueMemberS{Value: result.Query}
	item["sentiment_score"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", result.SentimentScore)}
	item["sentiment_label"] = &types.AttributeValueMemberS{Value: result.SentimentLabel}
	item["confidence"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%f", result.Confidence)}
	item["created_at"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Unix())}
	item["ttl"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", time.Now().Add(24*time.Hour).Unix())}

	// Optional: metadata as nested map
	metadata := make(map[string]types.AttributeValue)
	if result.Metadata.Author != "" {
		metadata["author"] = &types.AttributeValueMemberS{Value: result.Metadata.Author}
	}
	if result.Metadata.PostID != "" {
		metadata["post_id"] = &types.AttributeValueMemberS{Value: result.Metadata.PostID}
	}
	if result.Metadata.URL != "" {
		metadata["url"] = &types.AttributeValueMemberS{Value: result.Metadata.URL}
	}
	if result.Metadata.Subreddit != "" {
		metadata["subreddit"] = &types.AttributeValueMemberS{Value: result.Metadata.Subreddit}
	}
	if !result.Metadata.Timestamp.IsZero() {
		metadata["timestamp"] = &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", result.Metadata.Timestamp.Unix())}
	}
	if len(metadata) > 0 {
		item["metadata"] = &types.AttributeValueMemberM{Value: metadata}
	}

	// Optional fields
	if result.Text != "" {
		item["text"] = &types.AttributeValueMemberS{Value: result.Text}
	}
	if result.OriginalText != "" {
		item["original_text"] = &types.AttributeValueMemberS{Value: result.OriginalText}
	}
	if result.WasSummarized {
		item["was_summarized"] = &types.AttributeValueMemberBOOL{Value: true}
	}

	return item
}
