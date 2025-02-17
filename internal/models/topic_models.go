package models

import "time"

type Topic struct {
	ID               int       `json:"id"`
	Topic            string    `json:"topic"`
	Category         string    `json:"category"`
	OriginalHeadline string    `json:"original_headline"`
	CreatedAt        time.Time `json:"created_at"`
}
