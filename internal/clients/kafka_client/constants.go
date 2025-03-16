package kafka_client

import "time"

const (
	KAFKA_TOPIC_REDDIT_CONTENT      = "reddit-content" // data sent to kafka from the web
	KAFKA_TOPIC_SUMMARY_REQUEST     = "summary-request"
	KAFKA_TOPIC_SENTIMENT_BATCHES   = "sentiment-batches"   // batched messages to be sent for analysis
	KAFKA_TOPIC_SENTIMENT_RESULTS   = "sentiment-results"   // batched results from sentiment analysis
	KAFKA_TOPIC_SENTIMENT_AMBIGUOUS = "sentiment-ambiguous" // ambiguous analysis that needs AI followup analysis
)

const (
	BATCH_SIZE    = 50
	BATCH_TIMEOUT = 5 * time.Second
	MAX_RETRIES   = 5
	RETRY_DELAY   = 2 * time.Second
)
