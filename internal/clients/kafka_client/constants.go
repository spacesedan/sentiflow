package kafka_client

import "time"

const (
	KAFKA_TOPIC_RAW_CONTENT       = "raw-content"       // data from multiple content outlets
	KAFKA_TOPIC_SUMMARY_REQUEST   = "summary-request"   // longer content that will need to be summarized before processing
	KAFKA_TOPIC_SENTIMENT_REQUEST = "sentiment-request" // batched messages to be sent for analysis
	KAFKA_TOPIC_SENTIMENT_RESULTS = "sentiment-results" // batched results from sentiment analysis
)

const (
	BATCH_SIZE    = 50
	BATCH_TIMEOUT = 5 * time.Second
	MAX_RETRIES   = 5
	RETRY_DELAY   = 2 * time.Second
)
