package models

type NewsAPITopHeadlinesResponse = struct {
	Status       string           `json:"status"`
	TotalResults int              `json:"totalResults"`
	Articles     []NewsAPIArticle `json:"articles"`
}

type NewsAPIArticle = struct {
	Title       string `json:"title"`
	URL         string `json:"url"`
	Author      string `json:"author"`
	UrlToImage  string `json:"urlToImage"`
	PublishedAt string `json:"publishedAt"`
	Description string `json:"description"`
}
