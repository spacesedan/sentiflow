package models

import "time"

type AnalyzedRedditPost struct {
	RedditPost
	SentimentScore     float64         `json:"sentiment_score"`
	OriginalVADERScore float64         `json:"original_vader_score"`
	SentimentLabel     string          `json:"sentiment_label"`
	SentimentSource    SentimentSource `json:"sentiment_source"`
	Timestamp          time.Time       `json:"timestamp"`
}

type SentimentSource struct {
	Initial string `json:"initial"`
	Final   string `json:"final"`
}
