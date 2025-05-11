package models

// Updated the schema to be more clear the older "Topic" was too confusing,
//   - Added a SentimentScore that is an aggregate of all the score for that given topic
//   - Added an ID to help in the SSG in the frontend
//   - Added a source to help in the ID generation
type Headline struct {
	ID             string       `json:"id" dynamodbav:"id"`
	Query          string       `json:"query" dynamodbav:"query"`
	Category       string       `json:"category" dynamodbav:"category"`
	SentimentScore float32      `json:"sentiment_score,omitempty" dynamodbav:"sentiment_score"`
	HeadlineMeta   HeadlineMeta `json:"headline_meta" dynamodbav:"headline_meta"`
}

// Headline Meta moved any data having to due with the orignal headline here
type HeadlineMeta struct {
	// Source where headline was retrieved (e.g. HeadlinesAPI, Google Trends)
	Source      string `json:"source" dynamodbav:"source"`
	Title       string `json:"title" dynamodbav:"title"`
	Author      string `json:"author" dynamodbav:"author"`
	Description string `json:"description" dynamodbav:"description"`
	PublishedAt string `json:"publishedAt" dynamodbav:"publishedAt"`
	Url         string `json:"url" dynamodbav:"url"`
	UrlToImage  string `json:"urlToImage" dynamodbav:"urlToImage"`
}
