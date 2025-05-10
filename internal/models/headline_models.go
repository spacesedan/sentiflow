package models

// Updated the schema to be more clear the older "Topic" was too confusing,
//   - Added a SentimentScore that is an aggregate of all the score for that given topic
//   - Added an ID to help in the SSG in the frontend
//   - Added a source to help in the ID generation
type Headline struct {
	ID             string       `json:"id"`
	Query          string       `json:"query"`
	Category       string       `json:"category"`
	SentimentScore float32      `json:"sentiment_score,omitempty"`
	HeadlineMeta   HeadlineMeta `json:"headline_meta"`
}

// Headline Meta moved any data having to due with the orignal headline here
type HeadlineMeta struct {
	// Source where headline was retrieved (e.g. HeadlinesAPI, Google Trends)
	Source      string `json:"source"`
	Title       string `json:"title"`
	Author      string `json:"author"`
	Description string `json:"description"`
	PublishedAt string `json:"publishedAt"`
	Url         string `json:"url"`
	UrlToImage  string `json:"urlToImage"`
}
