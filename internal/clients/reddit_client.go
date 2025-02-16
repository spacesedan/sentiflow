package clients

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"sync"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"
)

const (
	REDDIT_AUTH_URL = "https://www.reddit.com/api/v1/access_token"
	REDDIT_API_URL  = "https://oauth.reddit.com/"
	USER_AGENT      = "sentiflow-bot/0.1"
)

var (
	redditClientInstance *RedditClient
	redditClientOnce     sync.Once
	redditRateLimitMutex sync.Mutex
)

type RedditClient struct {
	Config *clientcredentials.Config
	Client *http.Client
	mu     *sync.Mutex
}

func GetRedditClient() *RedditClient {
	redditClientOnce.Do(func() {
		oauthConf := &clientcredentials.Config{
			ClientID:     os.Getenv("REDDIT_CLIENT_ID"),
			ClientSecret: os.Getenv("REDDIT_CLIENT_SECRET"),
			TokenURL:     REDDIT_AUTH_URL,
			AuthStyle:    oauth2.AuthStyleInHeader,
		}

		redditClientInstance = &RedditClient{
			Config: oauthConf,
			Client: oauthConf.Client(context.Background()),
			mu:     &sync.Mutex{},
		}
	})

	return redditClientInstance
}

func (rc *RedditClient) RefreshClient() {
	rc.mu.Lock()
	defer rc.mu.Unlock()
	rc.Client = rc.Config.Client(context.Background())
}

func (rc *RedditClient) FetchSubredditPosts(subreddit, topic string) ([]byte, error) {
	parsedUrl, err := url.Parse(fmt.Sprintf("%s/r/%s/search", REDDIT_API_URL, subreddit))
	if err != nil {
		return nil, fmt.Errorf("[RedditClient] Failed to parse URL: %w", err)
	}
	queryParams := parsedUrl.Query()
	queryParams.Add("q", topic)
	queryParams.Add("sort", "top")
	queryParams.Add("limit", "100")
	parsedUrl.RawQuery = queryParams.Encode()

	redditRateLimitMutex.Lock()
	time.Sleep(INITIAL_BACKOFF)
	redditRateLimitMutex.Unlock()

	req, err := http.NewRequest("GET", parsedUrl.String(), nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", USER_AGENT)

	resp, err := rc.Client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusUnauthorized:
		slog.Warn("[RedditClient] Token expired - Refreshing and Retrying...")
		rc.RefreshClient()
		return rc.FetchSubredditPosts(subreddit, topic)
	case http.StatusTooManyRequests:
		slog.Warn("[RedditClient] 429 Too Many Requests - Retrying with backoff")
		return rc.retryWithBackoff(subreddit, topic)
	case http.StatusOK:
		bytes, err := io.ReadAll(resp.Body)
		if err != nil {
			return nil, err
		}
		return bytes, nil
	}
	return nil, err
}

func (rc *RedditClient) retryWithBackoff(subreddit, topic string) ([]byte, error) {
	backoff := INITIAL_BACKOFF
	for i := 1; i < MAX_RETRIES; i++ {
		slog.Warn("[RedditClient] Retrying request",
			slog.Int("attempt", i), slog.Duration("backoff", backoff))

		time.Sleep(backoff)

		backoff *= 2
		if backoff > MAX_BACKOFF {
			backoff = MAX_BACKOFF
		}

		data, err := rc.FetchSubredditPosts(subreddit, topic)
		if err == nil {
			return data, nil
		}
	}
	return nil, fmt.Errorf("[RedditClient] Max retries reached request failed")
}
