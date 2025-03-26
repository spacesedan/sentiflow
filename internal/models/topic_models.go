package models

type Topic struct {
	Topic    string `json:"title"`
	Category string `json:"category"`
	URL      string `json:"url"`
}
