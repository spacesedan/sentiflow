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
	TOPICS_TABLE_NAME             = "Topics"
	SENTIMENT_ANALYSIS_TABLE_NAME = "SentimentResults"
)

var dbClient *dynamodb.Client

func InitDynamoDB() {
	dbClient = clients.GetDynamoDBClient()
}

func StoreBatchedTopics(ctx context.Context, topics []models.Topic) error {
	if dbClient == nil {
		dbClient = clients.GetDynamoDBClient()
	}

	// TTL for topics
	expirationTime := time.Now().Add(24 * time.Hour).Unix()

	const maxBatchSize = 25
	for i := 0; i < len(topics); i += maxBatchSize {
		select {
		case <-ctx.Done():
			slog.Warn("[DynamoDB] context canceled")
			return ctx.Err()
		default:

			end := i + maxBatchSize
			if end > len(topics) {
				end = len(topics)
			}

			writeRequests := make([]types.WriteRequest, 0, maxBatchSize)
			for _, topic := range topics[i:end] {
				writeRequests = append(writeRequests, types.WriteRequest{
					PutRequest: &types.PutRequest{
						Item: map[string]types.AttributeValue{
							"url":        &types.AttributeValueMemberS{Value: topic.URL},
							"category":   &types.AttributeValueMemberS{Value: topic.Category},
							"topic":      &types.AttributeValueMemberS{Value: topic.Topic},
							"title":      &types.AttributeValueMemberS{Value: topic.Title},
							"expires_at": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expirationTime)},
						},
					},
				})
			}

			out, err := dbClient.BatchWriteItem(ctx, &dynamodb.BatchWriteItemInput{
				RequestItems: map[string][]types.WriteRequest{
					TOPICS_TABLE_NAME: writeRequests,
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
					slog.Int("remaining_items", len(out.UnprocessedItems[TOPICS_TABLE_NAME])),
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
					slog.Int("remaining_items", len(out.UnprocessedItems[TOPICS_TABLE_NAME])))
			}
		}
	}
	slog.Info("[DynamoDB] Successfully stored all topics")
	return nil
}

func GetAllTopics() ([]models.Topic, error) {
	if dbClient == nil {
		dbClient = clients.GetDynamoDBClient()
	}

	var topics []models.Topic
	input := &dynamodb.ScanInput{
		TableName: aws.String(TOPICS_TABLE_NAME),
	}

	paginator := dynamodb.NewScanPaginator(dbClient, input)

	for paginator.HasMorePages() {
		out, err := paginator.NextPage(context.TODO())
		if err != nil {
			return nil, fmt.Errorf("[DynamoDB] Scan for topics failed: %w", err)
		}
		var topicPage []models.Topic
		err = attributevalue.UnmarshalListOfMaps(out.Items, &topicPage)
		if err != nil {
			slog.Error("[DynamoDB] Unable to marshal current topic page", slog.String("error", err.Error()))
			return nil, err
		}
		topics = append(topics, topicPage...)

	}
	slog.Info("[DynamoDB] Successfully retrieved topics", slog.Int("count", len(topics)))
	return topics, nil
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
	item["topic"] = &types.AttributeValueMemberS{Value: result.Topic}
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
