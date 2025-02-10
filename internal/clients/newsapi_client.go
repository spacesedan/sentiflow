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

func (n NewsAPIClient) GetTopHeadlines() ([]models.NewsAPITopHeadlinesResponse, error) {
	if n.APIKey == "" {
		slog.Error("[NewsAPIClient] API key is missing")
		return nil, errors.New("[NewsAPIClient] API key is missing")
	}

	var responses []models.NewsAPITopHeadlinesResponse
	categories := []string{
		"business", "entertainment", "general",
		"health", "science", "sports", "technology",
	}

	for _, category := range categories {
		slog.Info("[NewsAPIClient] Fetching headlines", slog.String("category", category))

		response, err := n.fetchCategoryHeadlines(category)
		if err != nil {
			slog.Warn("[NewsAPIClient] Skipping category due to repeated failures",
				slog.String("category", category), slog.String("error", err.Error()))
			continue
		}

		slog.Debug("[NewsAPIClient] Number of headlines in this request",
			slog.Int("headlines", len(response.Articles)), slog.String("category", category))
		responses = append(responses, *response)
	}

	if len(responses) == 0 {
		return []models.NewsAPITopHeadlinesResponse{}, errors.New("[NewsAPIClient] No results fetched")
	}
	return responses, nil
}

// fetchCategoryHeadlines fetches headlines for a single category with retries
func (n NewsAPIClient) fetchCategoryHeadlines(category string) (*models.NewsAPITopHeadlinesResponse, error) {
	url := NEWS_API_ENDPOINT + n.APIKey + "&category=" + category
	backoff := INITIAL_BACKOFF

	for attempt := 1; attempt <= MAX_RETRIES; attempt++ {
		slog.Info("[NewsAPIClient] Attempting request", slog.String("category", category), slog.Int("attempt", attempt))

		req, err := http.NewRequest(http.MethodGet, url, nil)
		if err != nil {
			slog.Error("[NewsAPIClient] Failed to create request", slog.String("error", err.Error()))
			return nil, err
		}

		res, err := n.Client.Do(req)
		if err != nil {
			slog.Error("[NewsAPIClient] Request failed", slog.String("error", err.Error()))
			if attempt < MAX_RETRIES {
				time.Sleep(backoff)
				backoff *= 2
				if backoff > MAX_BACKOFF {
					backoff = MAX_BACKOFF
				}
			}
			continue
		}
		defer res.Body.Close()

		// Read response body
		body, err := io.ReadAll(res.Body)
		if err != nil {
			slog.Error("[NewsAPIClient] Failed to read response body", slog.String("error", err.Error()))
			return nil, err
		}

		switch res.StatusCode {
		case http.StatusOK:
			var response models.NewsAPITopHeadlinesResponse
			err = json.Unmarshal(body, &response)
			if err != nil {
				slog.Error("[NewsAPIClient] Failed to parse JSON", slog.String("error", err.Error()))
				return nil, err
			}
			slog.Info("[NewsAPIClient] Successfully fetched headlines", slog.String("category", category))
			return &response, nil

		case http.StatusBadRequest:
			return nil, errors.New("[NewsAPIClient] Bad request, check query parameters")

		case http.StatusUnauthorized:
			return nil, errors.New("[NewsAPIClient] Invalid API Key, check credentials")

		case http.StatusTooManyRequests:
			slog.Warn("[NewsAPIClient] Rate limit exceeded, retrying...",
				slog.Duration("backoff", backoff), slog.Int("attempt", attempt))
			if attempt < MAX_RETRIES {
				time.Sleep(backoff)
				backoff *= 2
				if backoff > MAX_BACKOFF {
					backoff = MAX_BACKOFF
				}
			}
			continue

		case http.StatusForbidden:
			return nil, errors.New("[NewsAPIClient] API key lacks required permissions")

		case http.StatusInternalServerError, http.StatusBadGateway, http.StatusServiceUnavailable, http.StatusGatewayTimeout:
			slog.Warn("[NewsAPIClient] Server error, retrying...",
				slog.Int("statusCode", res.StatusCode), slog.Duration("backoff", backoff), slog.Int("attempt", attempt))
			if attempt < MAX_RETRIES {
				time.Sleep(backoff)
				backoff *= 2
				if backoff > MAX_BACKOFF {
					backoff = MAX_BACKOFF
				}
			}
			continue

		default:
			slog.Warn("[NewsAPIClient] Unexpected response",
				slog.String("category", category), slog.Int("statusCode", res.StatusCode))
			return nil, errors.New("[NewsAPIClient] Unexpected status code")
		}
	}

	slog.Error("[NewsAPIClient] Failed after max retries", slog.String("category", category))
	return nil, errors.New("[NewsAPIClient] Failed after max retries")
}
