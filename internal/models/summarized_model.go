package models

type SummarizedRedditPost struct {
	RedditPost
	SummarizedContent string `json:"summarized_content"`
}
