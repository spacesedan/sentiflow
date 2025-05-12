package models

import "time"

type RawContent struct {
	ContentID string          `json:"content_id"`
	Source    string          `json:"source"`
	Query     string          `json:"query"`
	Topic     string          `json:"topic,omitempty" dynamodbav:"topic,omitempty"` // Added Topic field
	Text      string          `json:"text"`
	Metadata  ContentMetadata `json:"metadata"`
}

type ContentMetadata struct {
	Timestamp time.Time `json:"timestamp"`
	Author    string    `json:"author"`
	Subreddit string    `json:"subreddit,omitempty"`
	PostID    string    `json:"post_id,omitempty"`
	URL       string    `json:"url,omitempty"`
}
