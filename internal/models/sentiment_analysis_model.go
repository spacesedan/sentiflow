package models

type SentimentAnalysisInput struct {
	RawContent
	HeadlineID    string `json:"headline_id,omitempty" dynamodbav:"headline_id,omitempty"` // ID of the original headline, if applicable
	Query         string `json:"query,omitempty"`                                          // For headline-derived content
	Category      string `json:"category,omitempty"`                                       // For headline-derived content
	Text          string `json:"text"`                                                     // Text submitted for analysis
	WasSummarized bool   `json:"was_summarized"`
	OriginalText  string `json:"original_text,omitempty"` // Original text if WasSummarized is true
}

type SentimentAnalysisResult struct {
	SentimentAnalysisInput
	SentimentScore float64 `json:"sentiment_score"`
	SentimentLabel string  `json:"sentiment_label"`
	Confidence     float64 `json:"confidence"`
}
