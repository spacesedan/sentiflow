package utils

import (
	"github.com/spacesedan/sentiflow/internal/models"
)

func RawToSentimentAnalysisInput(c models.RawContent) models.SentimentAnalysisInput {
	return models.SentimentAnalysisInput{
		RawContent:    c,
		Text:          c.Text,
		WasSummarized: false,
	}
}
