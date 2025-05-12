package producer

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/clients/kafka_client"
	"github.com/spacesedan/sentiflow/internal/db"
	"github.com/spacesedan/sentiflow/internal/models"
)

var CategoryToSubreddits = map[string][]string{
	"Technology":                  {"technology", "Futurology", "programming", "gadgets", "techsupport"},
	"Business & Finance":          {"wallstreetbets", "investing", "finance", "personalfinance", "entrepreneur"},
	"Politics & World Affairs":    {"politics", "worldnews", "geopolitics", "PoliticalHumor", "PoliticalDiscussion"},
	"Entertainment & Pop Culture": {"movies", "television", "popculturechat", "music"},
	"Health & Science":            {"science", "askscience", "health", "nutrition", "medicine"},
	"Sports":                      {"sports", "nba", "nfl", "soccer", "baseball"},
	"Lifestyle & Society":         {"relationships", "selfimprovement", "lifeprotips", "socialskills", "relationship_advice"},
	"Memes & Internet Trends":     {"memes", "dankmemes", "me_irl", "OutOfTheLoop", "PoliticalHumor"},
	"Crime & Law":                 {"legaladvice", "TrueCrime", "law", "CrimeScene"},
}

var (
	CategoryToSubredditsStr = make(map[string]string)
	Categories              []string
	categoryHelpersOnce     sync.Once
)

func InitCategoryHelpers() {
	categoryHelpersOnce.Do(func() {
		Categories = Categories[:0]

		for category, subreddits := range CategoryToSubreddits {
			Categories = append(Categories, category)
			CategoryToSubredditsStr[category] = strings.Join(subreddits, "+")
		}
	})
}

// mapTopicsToCategory creates a map of topics that can be used to create queries for the Reddit API
func mapHeadlinesToCategory(headlines []models.Headline) map[string][]models.Headline {
	headlineMap := make(map[string][]models.Headline)

	for _, headline := range headlines {
		headlineMap[headline.Category] = append(headlineMap[headline.Category], headline)
	}

	return headlineMap
}

// FetchRedditContentForTopics fetches Reddit posts based on stored topics & sends to Kafka
func FetchRedditContentForHeadlines(ctx context.Context) {
	slog.Info("Fetching Reddit content for stored topics...")

	headlines, err := db.GetAllHeadlines()
	if err != nil {
		slog.Error("Failed to fetch headlines from DB", slog.String("error", err.Error()))
		return
	}

	if len(headlines) == 0 {
		slog.Warn("No new topics found. Skipping Reddit fetch.")
		return
	}

	headlinesMap := mapHeadlinesToCategory(headlines)

	// Process each query
	for _, category := range Categories {
		subreddits, exists := CategoryToSubredditsStr[category]
		if !exists {
			slog.Warn("No Matching subbreddits found for topic category", slog.String("category", category))
			continue
		}

		for _, headline := range headlinesMap[category] {
			if err := fetchAndProcessHeadline(ctx, subreddits, headline); err != nil {
				slog.Error("Failed processing topic",
					slog.String("query", headline.Query))
			}
		}
	}

	slog.Info("Successfully fetched & sent Reddit content to Kafka!")
}

func fetchAndProcessHeadline(ctx context.Context, subreddits string, headline models.Headline) error {
	after := ""
	for {
		select {
		case <-ctx.Done():
			slog.Warn("Context cancelled, stopping fetch for topic",
				slog.String("query", headline.Query))
			return ctx.Err()
		default:
		}

		posts, nextAfter, err := fetchWithRetries(ctx, subreddits, headline.Query, after)
		if err != nil {
			return fmt.Errorf("fetch failed after retries: %w", err)
		}

		processPosts(ctx, posts)
		if nextAfter == "" {
			break
		}
		after = nextAfter
	}
	return nil
}

func fetchWithRetries(ctx context.Context, subreddits, query, after string) ([]models.RedditPost, string, error) {
	var posts []models.RedditPost
	var nextAfter string
	var err error

	for attempt := 1; attempt <= 3; attempt++ {
		posts, nextAfter, err = clients.GetRedditClient().FetchSubredditPosts(ctx, subreddits, query, after)
		if err == nil {
			return posts, nextAfter, nil
		}

		slog.Warn("Retrying Reddit fetch",
			slog.String("query", query),
			slog.Int("attempt", attempt),
			slog.String("error", err.Error()))

		select {
		case <-ctx.Done():
			return nil, "", ctx.Err()
		case <-time.After(2 * time.Second):
		}
	}

	return nil, "", err
}

func processPosts(ctx context.Context, posts []models.RedditPost) {
	for _, post := range posts {
		select {
		case <-ctx.Done():
			slog.Warn("Context cancelled during post processing")
			return
		default:
		}

		dedupeKey := fmt.Sprintf("%s:%s", post.Topic, post.PostID)

		if post.PostContent == "" || clients.GetValkeyClient().IsPostProcessed(ctx, "reddit", dedupeKey) {
			continue
		}

		rawContent := redditPostToRaw(post)
		if err := kafka_client.PublishToKafka(ctx, kafka_client.KAFKA_TOPIC_RAW_CONTENT, rawContent); err != nil {
			slog.Warn("Failed to publish to Kafka",
				slog.String("post_id", rawContent.ContentID),
				slog.String("error", err.Error()))
			continue
		}

		if err := clients.GetValkeyClient().MarkProcessed(ctx, "reddit", dedupeKey); err != nil {
			slog.Warn("Error marking post as processed",
				slog.String("post_id", post.PostID),
				slog.String("error", err.Error()))
		}

	}
}

func generateRedditContentID(topic, source, postID string) string {
	raw := fmt.Sprintf("%s:%s:%s", topic, source, postID)
	hash := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(hash[:])
}

func redditPostToRaw(p models.RedditPost) models.RawContent {
	source := "reddit"
	return models.RawContent{
		ContentID: generateRedditContentID(p.Topic, source, p.PostID),
		Source:    source,
		Topic:     p.Topic,
		Text:      p.PostContent,
		Metadata: models.ContentMetadata{
			Author:    p.Author,
			Timestamp: p.CreatedAt,
			Subreddit: p.Subreddit,
			PostID:    p.PostID,
		},
	}
}
