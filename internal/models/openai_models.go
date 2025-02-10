package models

type OpenAITopicResponse = struct {
	Topics []struct {
		Topic    string `json:"topic"`
		Category string `json:"category"`
	} `json:"topics"`
}
