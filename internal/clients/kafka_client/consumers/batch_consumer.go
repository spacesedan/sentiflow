package consumers

import (
	"context"
	"log/slog"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client/utils"
	"github.com/spacesedan/sentiflow/internal/models"
	"github.com/spacesedan/sentiflow/internal/sentiment"
)

var (
	resultsBatchBuffer   = utils.NewBatchBuffer[models.AnalyzedRedditPost]()
	ambiguousBatchBuffer = utils.NewBatchBuffer[models.AnalyzedRedditPost]()
)

func StartBatchConsumer(ctx context.Context, consumer *kafka.Consumer) {
	iterator := kafka_client.NewKafkaMessageIterator(ctx, consumer)
	committer := kafka_client.NewCommitHandler(ctx, consumer)

	slog.Info("[BatchConsumer] Listening for messages...")

	ticker := time.NewTicker(utils.BATCH_TIMEOUT)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			slog.Warn("[RequestConsumer] Stopping consumer...")
			flushBatches()
			return
		case <-ticker.C:
			flushBatches()
		default:
			msg, err := iterator.Next()
			if err != nil {
				utils.HandleConsumerError(err)
				continue
			}

			var posts []models.RedditPost
			err = utils.DeserializeFromJSON(msg.Value, &posts)
			if err != nil {
				utils.HandleConsumerError(err)
				continue
			}

			for _, post := range posts {
				score, label := sentiment.AnalyzeWithVADER(post.PostContent)
				processedPost := models.AnalyzedRedditPost{
					RedditPost:         post,
					SentimentScore:     score,
					OriginalVADERScore: score,
					SentimentLabel:     label,
					SentimentSource: models.SentimentSource{
						Initial: "VADER",
						Final:   "VADER",
					},
					Timestamp: time.Now(),
				}
				if score >= 0.20 || score <= -0.20 {
					resultsBatchBuffer.Add(processedPost)
				} else {
					processedPost.SentimentSource.Final = "BERT"
					ambiguousBatchBuffer.Add(processedPost)
				}
			}

			committer.Commit(msg)

		}
	}
}

func flushBatches() {
	if resultsBatchBuffer.HasData() {
		slog.Info("[BatchConsumer] Flushing results batch to Kafka")
		kafka_client.PublishToKafka(kafka_client.KAFKA_TOPIC_SENTIMENT_RESULTS, resultsBatchBuffer.GetAndClear())
	}

	if ambiguousBatchBuffer.HasData() {
		slog.Info("[BatchConsumer] Flushing ambiguous batch to Kafka")
		kafka_client.PublishToKafka(kafka_client.KAFKA_TOPIC_SENTIMENT_AMBIGUOUS, ambiguousBatchBuffer.GetAndClear())
	}
}
