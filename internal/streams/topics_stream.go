package streams

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	ddbTypes "github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/db"
	"github.com/spacesedan/sentiflow/internal/models"
)

func StartTopicStreamConsumer(ctx context.Context) error {
	client := clients.GetDynamoDBStreamClient()

	streams, err := client.ListStreams(ctx, &dynamodbstreams.ListStreamsInput{
		TableName: aws.String(db.TOPICS_TABLE_NAME),
	})
	if err != nil {
		slog.Error("[TopicStreamConsumer] Error occured when listing streams...", slog.String("err", err.Error()))
		return err
	}

	streamArn := streams.Streams[0].StreamArn

	describeOutput, err := client.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: streamArn,
	})
	if err != nil {
		slog.Error("[TopicStreamConsumer] Failed to describe stream",
			slog.String("error", err.Error()))
		return err
	}

	for _, shard := range describeOutput.StreamDescription.Shards {
		shardIteratorOutput, err := client.GetShardIterator(ctx,
			&dynamodbstreams.GetShardIteratorInput{
				StreamArn:         streamArn,
				ShardId:           shard.ShardId,
				ShardIteratorType: types.ShardIteratorTypeLatest,
			})
		if err != nil {
			slog.Error("[TopicStreamConsumer] Failed to get shard iterator",
				slog.String("shard_id", *shard.ShardId),
				slog.String("error", err.Error()))

			continue
		}

		shardIterator := shardIteratorOutput.ShardIterator

		for shardIterator != nil {
			recordsOutput, err := client.GetRecords(
				ctx,
				&dynamodbstreams.GetRecordsInput{
					ShardIterator: shardIterator,
				})
			if err != nil {
				slog.Error("[TopicStreamConsumer] Failed to get records",
					slog.String("shard_id", *shard.ShardId),
					slog.String("error", err.Error()))
				break
			}

			for _, record := range recordsOutput.Records {
				if record.EventName != types.OperationTypeInsert {
					continue
				}

				newImage := record.Dynamodb.NewImage

				var topic models.Topic
				err := unmarshalStreamImage(newImage, &topic)
				if err != nil {
					slog.Error("[TopicStreamConsumer] failed to unmarshal topic",
						slog.String("error", err.Error()))
					continue
				}

				slog.Info("[TopicStreamConsumer] Received new topic",
					slog.String("topic", topic.Topic),
					slog.String("category", topic.Category))

				go processTopic(topic)

			}
			shardIterator = recordsOutput.NextShardIterator
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

func processTopic(topic models.Topic) {}

func unmarshalStreamImage[T any](image map[string]types.AttributeValue, out *T) error {
	rawJson, err := json.Marshal(image)
	if err != nil {
		return err
	}

	var ddbAttrs map[string]ddbTypes.AttributeValue
	if err := json.Unmarshal(rawJson, &ddbAttrs); err != nil {
		return err
	}

	return attributevalue.UnmarshalMap(ddbAttrs, out)
}
