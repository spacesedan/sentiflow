package models

type OpenAIRequest struct {
	ID       string `json:"id"`
	Headline string `json:"headline"`
}

type OpenAITopicResponse = struct {
	Headlines []Headline `json:"headlines"`
}
