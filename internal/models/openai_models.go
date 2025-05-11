package models

type OpenAIRequest struct {
	ID       string `json:"id"`
	Headline string `json:"headline"`
}

type OpenAIHeadlineResponse struct {
	Headlines []OpenAIHeadline `json:"headlines"`
}

type OpenAIHeadline = struct {
	ID       string `json:"id"`
	Headline string `json:"headline"`
	Query    string `json:"query"`
	Category string `json:"category"`
}
