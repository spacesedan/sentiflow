package models

type Topic struct {
	Title    string `json:"title"`
	Topic    string `json:"topic"`
	Category string `json:"category"`
	URL      string `json:"url"`
}
