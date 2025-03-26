package clients

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/spacesedan/sentiflow/internal/models"
)

const (
	HF_SUMMARY_ENDPOINT            = "https://spacesedan-summarizer.hf.space/summarize"
	HF_SENTIMENT_ANALYSIS_ENDPOINT = "https://spacesedan-sentiment-analyzer.hf.space/analyze_batch"
)

var (
	huggingFaceInstance *HuggingFaceClient
	huggingFaceOnce     sync.Once
)

type HuggingFaceClient struct {
	Client *http.Client
}

func GetHuggingFaceClient() *HuggingFaceClient {
	var timeout time.Duration
	env := os.Getenv("APP_ENV")
	if env == "production" {
		timeout = 10 * time.Second
	} else {
		timeout = 60 * time.Second
	}
	huggingFaceOnce.Do(func() {
		slog.Info("[HuggingFaceClient] Initializing Client",
			slog.Duration("timeout", timeout),
			slog.String("env", env))
		huggingFaceInstance = &HuggingFaceClient{
			Client: &http.Client{
				Timeout: timeout,
			},
		}
	})
	return huggingFaceInstance
}

func (h *HuggingFaceClient) DoWithRetry(req *http.Request) (*http.Response, error) {
	var resp *http.Response
	var err error
	backoff := INITIAL_BACKOFF

	for attempt := 0; attempt < MAX_RETRIES; attempt++ {
		resp, err = h.Client.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}

		if resp != nil {
			resp.Body.Close()
		}

		slog.Warn("[HuggingFaceClient] Request failed, will retry",
			slog.Int("attempt", attempt+1),
			slog.String("error", errMsg(err, resp)))

		time.Sleep(backoff)
		backoff *= 2
	}

	return resp, err
}

func (h *HuggingFaceClient) GetSummaries(input interface{}) (models.SummaryBatchResponse, error) {
	var result models.SummaryBatchResponse
	slog.Info("[HuggingFaceClient] Requesting summary from summarization service")
	start := time.Now()

	err := h.postJSON(HF_SUMMARY_ENDPOINT, input, &result)
	if err != nil {
		slog.Error("[HuggingFaceClient] Summary Request Failed",
			slog.Duration("elapsed", time.Since(start)))
		return result, err
	}

	slog.Info("[HuggingFaceClient] Summary request successful",
		slog.Duration("elapsed", time.Since(start)))

	return result, nil
}

func (h *HuggingFaceClient) GetBatchedSentimentAnalysis(input interface{}) (models.SentimentAnalysisBatchResponse, error) {
	var result models.SentimentAnalysisBatchResponse
	slog.Info("[HuggingFaceClient] Requesting sentiment analysis from sentiment analysis service")
	start := time.Now()

	err := h.postJSON(HF_SENTIMENT_ANALYSIS_ENDPOINT, input, &result)
	if err != nil {
		slog.Error("[HuggingFaceClient] Sentiment Analysis request failed",
			slog.Duration("elapsed", time.Since(start)))
		return result, err
	}

	slog.Info("[HuggingFaceClient] Sentiment Analysis request successful",
		slog.Duration("elapsed", time.Since(start)))
	return result, nil
}

// helper function for posting data to my AI services
func (h *HuggingFaceClient) postJSON(endpoint string, input interface{}, output interface{}) error {
	body, err := json.Marshal(input)
	if err != nil {
		slog.Error("[HuggingFaceClient] Failed to marshal input",
			slog.String("endpoint", endpoint),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to marshal input: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		slog.Error("[HuggingFaceClient] Failed to build request",
			slog.String("endpoint", endpoint),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to build request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", USER_AGENT)

	resp, err := h.DoWithRetry(req)
	if err != nil {
		slog.Error("[HuggingFaceClient] Failed request after retries",
			slog.String("endpoint", endpoint),
			slog.String("error", err.Error()))

		return fmt.Errorf("request failed after retries: %w", err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		slog.Error("[HuggingFaceClient] Failed to read response",
			slog.String("endpoint", endpoint),
			slog.String("error", err.Error()))
		return fmt.Errorf("failed to read response: %w", err)
	}

	if err := json.Unmarshal(respBody, output); err != nil {
		slog.Error("[HuggingFaceClient] Failed to unmarshal response",
			slog.String("endpoint", endpoint),
			slog.String("error", err.Error()),
			getPreview(respBody),
			slog.Int("raw_response_length", len(string(respBody))))

		return fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return nil
}

func getPreview(respBody []byte) slog.Attr {
	raw := string(respBody)
	if len(raw) > 50 {
		raw = raw[:50]
	}
	return slog.String("raw_response", raw)
}

func errMsg(err error, resp *http.Response) string {
	if err != nil {
		return err.Error()
	}
	if resp != nil {
		return fmt.Sprintf("status code %d", resp.StatusCode)
	}
	return "unknown error"
}
