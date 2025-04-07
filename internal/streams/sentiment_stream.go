package streams

import (
	"context"
	"log/slog"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams"
	"github.com/aws/aws-sdk-go-v2/service/dynamodbstreams/types"
	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/db"
	"github.com/spacesedan/sentiflow/internal/models"
	"github.com/spacesedan/sentiflow/internal/utils"
)

var resultsStreamBuffer = utils.NewBatchBuffer[models.SentimentAnalysisResult]()

func StartSentimentStreamBatchFlusher(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Second * 5)
		defer ticker.Stop()

		for range ticker.C {
			if resultsStreamBuffer.Size() > 0 {
				processSentimentResults(ctx)
			}
		}
	}()
}

func StartSentimentStreamConsumer(ctx context.Context) error {
	client := clients.GetDynamoDBStreamClient()

	streams, err := client.ListStreams(ctx, &dynamodbstreams.ListStreamsInput{
		TableName: aws.String(db.SENTIMENT_ANALYSIS_TABLE_NAME),
	})
	if err != nil {
		slog.Error("[SentimentResultsStreamConsumer] Error occured when listing streams...", slog.String("err", err.Error()))
		return err
	}

	streamArn := streams.Streams[0].StreamArn

	describeOutput, err := client.DescribeStream(ctx, &dynamodbstreams.DescribeStreamInput{
		StreamArn: streamArn,
	})
	if err != nil {
		slog.Error("[SentimentResultsStreamConsumer] Failed to describe stream",
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
			slog.Error("[SentimentResultsStreamConsumer] Failed to get shard iterator",
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
				slog.Error("[SentimentResultsStreamConsumer] Failed to get records",
					slog.String("shard_id", *shard.ShardId),
					slog.String("error", err.Error()))
				break
			}

			for _, record := range recordsOutput.Records {
				if record.EventName != types.OperationTypeInsert {
					continue
				}

				newImage := record.Dynamodb.NewImage

				var result models.SentimentAnalysisResult
				err := unmarshalStreamImage(newImage, &result)
				if err != nil {
					slog.Error("[SentimentResultsStreamConsumer] failed to unmarshal topic",
						slog.String("error", err.Error()))
					continue
				}

				slog.Info("[SentimentResultsStreamConsumer] Received new result",
					slog.String("result_id", result.ContentID),
					slog.String("topic", result.Topic))

				resultsStreamBuffer.Add(result)
				if resultsStreamBuffer.Size() >= utils.STREAM_BATCH_SIZE {
					go processSentimentResults(ctx)
				}

			}
			shardIterator = recordsOutput.NextShardIterator
			time.Sleep(500 * time.Millisecond)
		}
	}
	return nil
}

func processSentimentResults(ctx context.Context) {}
