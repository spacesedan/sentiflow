package models

type SummaryBatchRequest struct {
	Inputs []SummaryRequest `json:"inputs"`
}

type SummaryRequest struct {
	ContentID string `json:"content_id"`
	Text      string `json:"text"`
}

type SummaryBatchResponse struct {
	Summaries []SummaryResponse `json:"summaries"`
}

type SummaryResponse struct {
	ContentID string `json:"content_id"`
	Summary   string `json:"summary"`
}

type SentimentAnalysisBatchRequest struct {
	Posts []SentimentAnalysisRequest `json:"posts"`
}

type (
	SentimentAnalysisRequest struct {
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
