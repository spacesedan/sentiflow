package models

type SummaryRequest struct {
	Inputs string `json:"inputs"`
}

type SummaryResponse struct {
	Summary string `json:"summary"`
}

type (
	SentimentAnalysisBatchRequest []SentimentAnalysisRequest
	SentimentAnalysisRequest      struct {
		ContentID string `json:"content_id"`
		Text      string `json:"text"`
	}
)

type (
	SentimentAnalysisBatchResponse []SentimentAnalysisResponse
	SentimentAnalysisResponse      struct {
		ContentID      string  `json:"content_id"`
		SentimentScore float64 `json:"sentiment_score"`
		SentimentLabel string  `json:"sentiment_label"`
		Confidence     float64 `json:"confidence"`
	}
)
