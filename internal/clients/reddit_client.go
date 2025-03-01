package clients

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"github.com/spacesedan/sentiflow/internal/models"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

// Reddit API Constants
const (
	REDDIT_AUTH_URL = "https://www.reddit.com/api/v1/access_token"
	REDDIT_API_URL  = "https://oauth.reddit.com"
	USER_AGENT      = "sentiflow-bot/0.1"
)

// Concurrency & Rate Limits
const (
	MAX_CONCURRENT_WORKERS = 2 // Limit number of parallel API calls
)

// Singleton Reddit Client
var (
	redditClientInstance *RedditClient
	redditClientOnce     sync.Once
)

// RedditClient handles OAuth authentication and API requests
type RedditClient struct {
	Config       *clientcredentials.Config
	Client       *http.Client
	WorkerTokens chan struct{} // Semaphore for limiting workers
	mu           *sync.Mutex
}

// GetRedditClient returns a singleton RedditClient instance
func GetRedditClient() *RedditClient {
	redditClientOnce.Do(func() {
		oauthConf := &clientcredentials.Config{
			ClientID:     os.Getenv("REDDIT_CLIENT_ID"),
			ClientSecret: os.Getenv("REDDIT_CLIENT_SECRET"),
			TokenURL:     REDDIT_AUTH_URL,
			AuthStyle:    oauth2.AuthStyleInHeader,
		}

		redditClientInstance = &RedditClient{
			Config:       oauthConf,
			Client:       oauthConf.Client(context.Background()),
			WorkerTokens: make(chan struct{}, MAX_CONCURRENT_WORKERS),
			mu:           &sync.Mutex{},
		}
	})

	return redditClientInstance
}

// RefreshClient updates the OAuth2 client with a new token
func (rc *RedditClient) RefreshClient() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.Client = rc.Config.Client(context.Background())
	slog.Info("[RedditClient] Token refreshed successfully")
}

// FetchSubredditPosts fetches posts from a subreddit based on a given topic
func (rc *RedditClient) FetchSubredditPosts(subreddit, topic, after string) ([]models.RedditPost, string, error) {
	parsedUrl, err := url.Parse(fmt.Sprintf("%s/r/%s/search", REDDIT_API_URL, subreddit))
	if err != nil {
		return nil, "", fmt.Errorf("[RedditClient] Failed to parse URL: %w", err)
	}

	queryParams := parsedUrl.Query()
	queryParams.Add("q", url.QueryEscape(topic))
	queryParams.Add("sort", "top")
	queryParams.Add("limit", "100")
	queryParams.Add("t", "day")
	queryParams.Add("type", "link+self")
	if after != "" {
		queryParams.Add("after", after)
	}
	parsedUrl.RawQuery = queryParams.Encode()

	req, err := http.NewRequest("GET", parsedUrl.String(), nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", USER_AGENT)

	rawData, err := rc.doRequestWithBackoff(req, subreddit)
	if err != nil {
		return nil, "", err
	}

	posts, nextAfter, err := parseRedditResponse(rawData, topic)
	if err != nil {
		slog.Error("Failed to parse reddit response", slog.String("topics", topic), slog.String("error", err.Error()))
	}

	if nextAfter == "null" || nextAfter == "" {
		return posts, "", nil
	}

	// Parse Reddit API response into structured RedditPost objects
	return posts, nextAfter, nil
}

// doRequestWithBackoff executes the request with retry logic and token refresh handling
func (rc *RedditClient) doRequestWithBackoff(req *http.Request, subreddit string) ([]byte, error) {
	backoff := INITIAL_BACKOFF
	for i := 0; i < MAX_RETRIES; i++ {
		rc.WorkerTokens <- struct{}{} // Acquire worker slot
		resp, err := rc.Client.Do(req)
		<-rc.WorkerTokens // Release worker slot

		if err != nil {
			slog.Warn("[RedditClient] Request error, retrying...",
				slog.String("subreddit", subreddit),
				slog.Int("attempt", i+1))
			time.Sleep(backoff)
			continue
		}
		defer resp.Body.Close()

		// Handle Rate Limit Headers
		if remaining, reset := parseRateLimitHeaders(resp); remaining <= 1 {
			slog.Warn("[RedditClient] Approaching rate limit. Sleeping until reset...",
				slog.Duration("sleep", reset))
			time.Sleep(reset)
		}

		switch resp.StatusCode {
		case http.StatusUnauthorized:
			slog.Warn("[RedditClient] Token expired - Refreshing and Retrying...")
			rc.RefreshClient()
			continue // Retry request with new token

		case http.StatusTooManyRequests:
			slog.Warn("[RedditClient] 429 Too Many Requests - Backing off and retrying...")
			time.Sleep(backoff)
			backoff *= 2
			if backoff > MAX_BACKOFF {
				backoff = MAX_BACKOFF
			}
			continue

		case http.StatusOK:
			bytes, err := io.ReadAll(resp.Body)
			if err != nil {
				return nil, err
			}
			return bytes, nil

		default:
			slog.Warn("[RedditClient] Unexpected status code",
				slog.Int("status", resp.StatusCode))
			time.Sleep(backoff)
		}
	}

	return nil, fmt.Errorf("[RedditClient] Max retries reached for subreddit %s", subreddit)
}

// parseRedditResponse parses the JSON response from Reddit API into structured RedditPost objects
func parseRedditResponse(rawData []byte, topic string) ([]models.RedditPost, string, error) {
	var redditResponse models.RedditAPIResponse
	err := json.Unmarshal(rawData, &redditResponse)
	if err != nil {
		slog.Error("[RedditClient] Failed to parse Reddit API response", slog.String("error", err.Error()))
		return nil, "", err
	}

	var posts []models.RedditPost
	for _, item := range redditResponse.Data.Children {
		post := item.Data
		posts = append(posts, models.RedditPost{
			Topic:       topic,
			Subreddit:   post.Subreddit,
			PostTitle:   post.Title,
			PostContent: post.Selftext,
			Upvotes:     post.Ups,
			CreatedAt:   time.Unix(int64(post.CreatedUTC), 0),
			PostID:      post.ID,
		})
	}

	return posts, redditResponse.Data.After, nil
}

// parseRateLimitHeaders extracts Reddit's rate limit headers
func parseRateLimitHeaders(resp *http.Response) (remaining int, reset time.Duration) {
	remaining = 60
	reset = 60 * time.Second

	if val := resp.Header.Get("X-Ratelimit-Remaining"); val != "" {
		if _, err := fmt.Sscanf(val, "%d", &remaining); err != nil {
			slog.Warn("[RedditClient] Failed to parse X-Ratelimit-Remaining", slog.String("error", err.Error()))
		}
		if remaining < 1 {
			remaining = 1 // Prevent divide-by-zero errors
		}
	}

	if val := resp.Header.Get("X-Ratelimit-Reset"); val != "" {
		var resetSeconds int
		if _, err := fmt.Sscanf(val, "%d", &resetSeconds); err != nil {
			slog.Warn("[RedditClient] Failed to parse X-Ratelimit-Reset", slog.String("error", err.Error()))
		}
		reset = time.Duration(resetSeconds) * time.Second
	}

	return
}
