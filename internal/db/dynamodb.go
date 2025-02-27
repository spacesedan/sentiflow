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

const TOPICS_TABLE_NAME = "Topics"

var dbClient *dynamodb.Client

func InitDynamoDB() {
	dbClient = clients.GetDynamoDBClient()
}

func StoreBatchedTopics(topics []models.Topic) error {
	if dbClient == nil {
		dbClient = clients.GetDynamoDBClient()
	}

	// TTL for topics
	expirationTime := time.Now().Add(24 * time.Hour).Unix()

	const maxBatchSize = 25
	for i := 0; i < len(topics); i += maxBatchSize {
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
						"expires_at": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", expirationTime)},
					},
				},
			})
		}

		out, err := dbClient.BatchWriteItem(context.TODO(), &dynamodb.BatchWriteItemInput{
			RequestItems: map[string][]types.WriteRequest{
				TOPICS_TABLE_NAME: writeRequests,
			},
		})
		if err != nil {
			return fmt.Errorf("[DynamoDB] Failed to batch write topics: %w", err)
		}

		// Retry writing unprocessed topics
		retryCount := 0
		for len(out.UnprocessedItems) > 0 && retryCount < 3 {
			time.Sleep(500 * time.Millisecond)
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
			slog.Error("[DynamoDB] Some items were not written even after retires",
				slog.Int("remaining_items", len(out.UnprocessedItems[TOPICS_TABLE_NAME])))
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
