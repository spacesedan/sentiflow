package models

type SentimentAnalysisInput struct {
	RawContent
	Text          string `json:"text"`
	WasSummarized bool   `json:"was_summarized"`
	OriginalText  string `json:"original_text,omitempty"`
}

type SentimentAnalysisResult struct {
	SentimentAnalysisInput
	SentimentScore float64 `json:"sentiment_score"`
	SentimentLabel string  `json:"sentiment_label"`
	Confidence     float64 `json:"confidence"`
}
