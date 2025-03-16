package consumers

import (
	"context"
	"log/slog"
	"os"
	"time"

	"github.com/confluentinc/confluent-kafka-go/kafka"
	"github.com/knights-analytics/hugot"
	"github.com/knights-analytics/hugot/pipelines"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client/utils"
	"github.com/spacesedan/sentiflow/internal/models"
)

var summaryBatchBuffer = utils.NewBatchBuffer[models.RedditPost]()

const (
	modelDir      = "./internal/transformers/models"
	bartModelPath = modelDir + "faceboo_bart.onnx"
)

func StartBartConsumer(ctx context.Context, consumer *kafka.Consumer) {
	iterator := kafka_client.NewKafkaMessageIterator(ctx, consumer)
	committer := kafka_client.NewCommitHandler(ctx, consumer)

	slog.Info("[BARTConsumer] Listening for summarization requests")

	ticker := time.NewTicker(utils.BATCH_TIMEOUT)
	defer ticker.Stop()

	if err := os.MkdirAll(modelDir, os.ModePerm); err != nil {
		slog.Error("[BARTConsumer] Failed to create model directory",
			slog.String("error", err.Error()))
	}

	if _, err := os.Stat(bartModelPath); os.IsNotExist(err) {
		slog.Info("[BARTConsumer] Model not found, downloading...")
		modelPath, err := hugot.DownloadModel("facebook/bart-large-cnn", modelDir, hugot.NewDownloadOptions())
		if err != nil {
			slog.Error("[BARTConsumer] Failed to download BART model",
				slog.String("error", err.Error()))
			return
		}
		slog.Info("[BARTConsumer] Model downloaded successfully", slog.String("path", modelPath))
	} else {
		slog.Info("[BARTConsumer] Using existing model", slog.String("path", bartModelPath))
	}

	session, err := hugot.NewORTSession()
	if err != nil {
		slog.Error("[BARTConsumer] Failed to initialize Hugot session", slog.String("error", err.Error()))
		return
	}
	defer session.Destroy()

	config := hugot.TextClassificationConfig{
		ModelPath: bartModelPath,
		Name:      "bartSummarizationPipeline",
	}
	summarizationPipeline, err := hugot.NewPipeline(session, config)
	if err != nil {
		slog.Error("[BARTConsumer] Failed to initialize BART pipeline", slog.String("error", err.Error()))
	}

	for {
		select {
		case <-ctx.Done():
			slog.Warn("[BARTConsumer] Stopping Consumer...")
			processSummarizationBatch(committer, summarizationPipeline)
			return
		case <-ticker.C:
			processSummarizationBatch(committer, summarizationPipeline)
		default:
			msg, err := iterator.Next()
			if err != nil {
				utils.HandleConsumerError(err)
				return
			}

			var posts []models.RedditPost
			utils.DeserializeFromJSON(msg.Value, &posts)

			utils.TrackMessage(posts[0].PostID, msg)
			for _, post := range posts {
				summaryBatchBuffer.Add(post)
			}

			if summaryBatchBuffer.Size() >= utils.BATCH_SIZE {
				go processSummarizationBatch(committer, summarizationPipeline)
			}

		}
	}
}

func processSummarizationBatch(committer *kafka_client.KafkaCommitHandler, pipeline *pipelines.TextClassificationPipeline) {
	batch := summaryBatchBuffer.GetAndClear()
	if batch == nil || len(batch) == 0 {
		return
	}

	var results []models.SummarizedRedditPost

	for _, post := range batch {

		output, err := pipeline.RunPipeline([]string{post.PostContent})
		if err != nil {
			slog.Warn("[BARTConsumer] Summarization failed", slog.String("error", err.Error()))
			continue
		}

		var summary models.BartGeneratedResponse
		if len(output.GetOutput()) > 0 {
			firstOutput, ok := output.GetOutput()[0].(string)
			if !ok {
				slog.Warn("[BARTConsumer] Unexpected output format from Hugot")
				continue
			}

			utils.DeserializeFromJSON([]byte(firstOutput), &summary)
		}

		if summary.SummaryText != "" {
			results = append(results, models.SummarizedRedditPost{
				RedditPost:        post,
				SummarizedContent: summary.SummaryText,
			})
		}

	}

	for i := 0; i < 3; i++ {
		err := kafka_client.PublishToKafka("summarize-posts", results)
		if err == nil {
			slog.Info("[BARTConsumer] Summaries published to Kafka successfully")
			break
		}
		slog.Warn("[BARTConsumer] Batch publishing Failed",
			slog.Int("attempt", i+1),
			slog.String("error", err.Error()))
		time.Sleep(2 * time.Second)
	}
	msg, found := utils.GetMessageForPost(batch[0].PostID)
	if found {
		if err := committer.Commit(msg); err != nil {
			slog.Warn("[BARTConsumer] Failed to commit offset",
				slog.String("error", err.Error()))
		}
	}
}
