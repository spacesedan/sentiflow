package clients

import (
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/spacesedan/sentiflow/internal/models"
)

const (
	NEWS_API_ENDPOINT = "https://newsapi.org/v2/top-headlines?country=us&pageSize=100&apiKey="
	MAX_RETRIES       = 5
	INITIAL_BACKOFF   = 1 * time.Second
	MAX_BACKOFF       = 32 * time.Second
)

var (
	newsAPIInstance *NewsAPIClient
	newsAPIOnce     sync.Once
)

type NewsAPIClient struct {
	Client *http.Client
	APIKey string
}

func GetNewsAPIClient() *NewsAPIClient {
	newsAPIOnce.Do(func() {
		newsAPIInstance = &NewsAPIClient{
			Client: &http.Client{},
			APIKey: os.Getenv("NEWS_API_KEY"),
		}
	})
	return newsAPIInstance
}

func (n NewsAPIClient) GetTopHeadlines() (*models.NewsAPITopHeadlinesResponse, error) {
	if n.APIKey == "" {
		slog.Error("[NewsAPIClient] API key is missing")
		return nil, errors.New("[NewsAPIClient] API key is missing")
	}
	url := NEWS_API_ENDPOINT + n.APIKey

	var response *models.NewsAPITopHeadlinesResponse
	var lastErr error
	backoff := INITIAL_BACKOFF

	for attempt := 1; attempt <= MAX_RETRIES; attempt++ {
		slog.Info("[NewsAPIClient] Fetching top headlines", slog.Int("attempt", attempt))
		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			return nil, err
		}

		res, err := n.Client.Do(req)
		if err != nil {
			slog.Error("[NewsAPIClient] failed to create request", slog.String("error", err.Error()))
			lastErr = err
		} else {
			defer res.Body.Close()

			switch res.StatusCode {
			case http.StatusOK:
				body, err := io.ReadAll(res.Body)
				if err != nil {
					slog.Error("[NewsAPIClient] Failed to read response body", slog.String("error", err.Error()))
					return nil, err
				}
				err = json.Unmarshal(body, &response)
				if err != nil {
					slog.Error("[NewsAPIClient] Failed to parse JSON repsonse", slog.String("error", err.Error()))
					return nil, err
				}

				slog.Info("[NewsAPIClient] Successfully fetched headlines")
				return response, nil
			case http.StatusBadRequest:
				slog.Warn("[NewsAPIClient] Bad request: check query parameters")
				return nil, errors.New("[NewsAPIClient] Bad request: check query parameters")
			case http.StatusUnauthorized:
				slog.Error("[NewsAPIClient] Invalid API Key, check credentials")
				return nil, errors.New("[NewsAPIClient] Invalid API Key, check credentials")
			case http.StatusTooManyRequests:
				slog.Warn("[NewsAPIClient] Rate limit exceeded, retrying...",
					slog.Duration("backoff", backoff), slog.Int("attempt", attempt))
				_, err = io.Copy(io.Discard, res.Body)
				if err != nil {
					slog.Error("[NewsAPIClient] Failed to reade response body", slog.String("error", err.Error()))
					return nil, err
				}
				time.Sleep(backoff)
				backoff *= 2
				if backoff > MAX_BACKOFF {
					backoff = MAX_BACKOFF
				}
			case http.StatusForbidden:
				slog.Error("[NewsAPIClient] Access forbidden, check API key permissions")
				return nil, errors.New("[NewsAPIClient] API key lacks required permissions")
			case http.StatusInternalServerError:
				slog.Warn("[NewsAPIClient] Server Error", slog.Int("statusCode", res.StatusCode),
					slog.Duration("backoff", backoff), slog.Int("attempt", attempt))
				time.Sleep(backoff)
				backoff *= 2
				if backoff > MAX_BACKOFF {
					backoff = MAX_BACKOFF
				}
			default:
				slog.Warn("[NewsAPIClient] Unexpected Response")
				return nil, errors.New("[NewsAPI] Unexpected stats code")
			}
		}
		if attempt == MAX_RETRIES {
			slog.Error("[NewsAPIClient] Failed after max retries")
			lastErr = errors.New("[NewsAPIClient] failed after max retries")
			break
		}
	}
	return nil, lastErr
}
