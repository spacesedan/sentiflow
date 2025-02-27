package models

type NewsAPITopHeadlinesResponse = struct {
	Status       string            `json:"status"`
	TotalResults int               `json:"totalResults"`
	Articles     []NewsAPIArticles `json:"articles"`
}

type NewsAPIArticles = struct {
	Title string `json:"title"`
	URL   string `json:"url"`
}
