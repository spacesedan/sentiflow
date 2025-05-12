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
	REDDIT_AUTH_URL   = "https://www.reddit.com/api/v1/access_token"
	REDDIT_API_URL    = "https://oauth.reddit.com"
	REDDIT_UNAUTH_URL = "https://www.reddit.com"
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

func buildRedditAPIUrl(subreddit, topic, after string) (string, error) {
	parsedUrl, err := url.Parse(fmt.Sprintf("%s/r/%s/search", REDDIT_API_URL, subreddit))
	if err != nil {
		return "", fmt.Errorf("[RedditClient] Failed to parse URL: %w", err)
	}

	queryParams := parsedUrl.Query()
	queryParams.Add("q", topic)
	queryParams.Add("sort", "relevance")
	queryParams.Add("limit", "100")
	queryParams.Add("t", "day")
	queryParams.Add("type", "link")
	if after != "" {
		queryParams.Add("after", after)
	}
	parsedUrl.RawQuery = queryParams.Encode()

	return parsedUrl.String(), nil
}

// RefreshClient updates the OAuth2 client with a new token
func (rc *RedditClient) RefreshClient() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.Client = rc.Config.Client(context.Background())
	slog.Info("[RedditClient] Token refreshed successfully")
}

// FetchSubredditPosts fetches posts from a subreddit based on a given topic
func (rc *RedditClient) FetchSubredditPosts(ctx context.Context, subreddit, topic, after string) ([]models.RedditPost, string, error) {
	slog.Info("[RedditClient] Requesting data from reddit",
		slog.String("subreddits", subreddit),
		slog.String("topic", topic))

	url, err := buildRedditAPIUrl(subreddit, topic, after)
	if err != nil {
		return nil, "", err
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("User-Agent", USER_AGENT)

	start := time.Now()
	rawData, err := rc.doRequestWithBackoff(ctx, req, subreddit)
	if err != nil {
		return nil, "", err
	}

	posts, nextAfter, err := parseRedditResponse(rawData, topic)
	if err != nil {
		slog.Error("Failed to parse reddit response",
			slog.String("topics", topic),
			slog.String("error", err.Error()))
	}

	slog.Info("[RedditClient] Response details",
		slog.String("subreddits", subreddit),
		slog.String("topic", topic),
		slog.Int("post_count", len(posts)),
		slog.Duration("time_elapsed", time.Since(start)))

	if nextAfter == "null" || nextAfter == "" {
		return posts, "", nil
	}

	// Parse Reddit API response into structured RedditPost objects
	return posts, nextAfter, nil
}

// doRequestWithBackoff executes the request with retry logic and token refresh handling
func (rc *RedditClient) doRequestWithBackoff(ctx context.Context, req *http.Request, subreddit string) ([]byte, error) {
	backoff := INITIAL_BACKOFF
	for i := 0; i < MAX_RETRIES; i++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case rc.WorkerTokens <- struct{}{}:

		}
		resp, err := rc.Client.Do(req)
		<-rc.WorkerTokens // Release worker slot

		if err != nil {
			slog.Warn("[RedditClient] Request error, retrying...",
				slog.String("subreddit", subreddit),
				slog.Int("attempt", i+1),
				slog.String("error", err.Error()))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				continue
			}
		}
		defer resp.Body.Close()

		// Handle Rate Limit Headers
		if remaining, reset := parseRateLimitHeaders(resp); remaining <= 1 {
			slog.Warn("[RedditClient] Approaching rate limit. Sleeping until reset...",
				slog.Duration("sleep", reset))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(reset):
				continue
			}
		}

		switch resp.StatusCode {
		case http.StatusUnauthorized:
			slog.Warn("[RedditClient] Token expired - Refreshing and Retrying...")
			rc.RefreshClient()
			continue // Retry request with new token

		case http.StatusTooManyRequests:
			slog.Warn("[RedditClient] 429 Too Many Requests - Backing off and retrying...")
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff = minDuration(backoff*2, MAX_BACKOFF)
			}
			continue

		case http.StatusOK:
			return io.ReadAll(resp.Body)

		default:
			slog.Warn("[RedditClient] Unexpected status code",
				slog.Int("status", resp.StatusCode))
			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-time.After(backoff):
				backoff = minDuration(backoff*2, MAX_BACKOFF)
			}
		}
	}

	return nil, fmt.Errorf("[RedditClient] Max retries reached for subreddit %s", subreddit)
}

func minDuration(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

// parseRedditResponse parses the JSON response from Reddit API into structured RedditPost objects
func parseRedditResponse(rawData []byte, query string) ([]models.RedditPost, string, error) {
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
			Query:       query,
			Subreddit:   post.Subreddit,
			PostTitle:   post.Title,
			Author:      post.AuthorFullname,
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
