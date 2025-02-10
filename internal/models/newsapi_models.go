package models

type NewsAPITopHeadlinesResponse = struct {
	Status       string            `json:"status"`
	TotalResults int               `json:"totalResults"`
	Articles     []NewsAPIArticles `json:"articles"`
}

type NewsAPIArticles = struct {
	Source struct {
		ID   string `json:"id"`
		Name string `json:"name"`
	} `json:"source"`
	Title string `json:"title"`
}
