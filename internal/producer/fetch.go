package producer

import (
	"context"
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
func mapTopicsToCategory(topics []models.Topic) map[string][]models.Topic {
	topicMap := make(map[string][]models.Topic)

	for _, topic := range topics {
		topicMap[topic.Category] = append(topicMap[topic.Category], topic)
	}

	return topicMap
}

const VALKEY_POSTS_KEY = "reddit:processed_posts"

// FetchRedditContentForTopics fetches Reddit posts based on stored topics & sends to Kafka
func FetchRedditContentForTopics(ctx context.Context) {
	slog.Info("Fetching Reddit content for stored topics...")

	topics, err := db.GetAllTopics()
	if err != nil {
		slog.Error("Failed to fetch topics from DB", slog.String("error", err.Error()))
		return
	}

	if len(topics) == 0 {
		slog.Warn("No new topics found. Skipping Reddit fetch.")
		return
	}

	topicMap := mapTopicsToCategory(topics)

	valkeyClient := clients.GetValkeyClient()

	// Process each query
	for _, category := range Categories {
		subreddits, exists := CategoryToSubredditsStr[category]
		if !exists {
			slog.Warn("No Matching subbreddits found for topic category", slog.String("category", category))
			continue
		}

		for _, topic := range topicMap[category] {
			after := ""
			for {

				select {
				case <-ctx.Done():
					slog.Warn("[FetchRedditContentForTopics] Context cancelled, stopping...")
					return
				default:
				}

				retryCount := 3
				var posts []models.RedditPost
				var nextAfter string
				for attempt := 1; attempt <= retryCount; attempt++ {
					var fetchErr error
					posts, nextAfter, fetchErr = clients.GetRedditClient().FetchSubredditPosts(subreddits, topic.Topic, after)
					if fetchErr != nil {
						slog.Warn("Failed to fetch Reddit posts",
							slog.String("subreddits", subreddits),
							slog.String("topic", topic.Topic),
							slog.String("error", fetchErr.Error()),
							slog.Int("attempt", attempt))
						time.Sleep(2 * time.Second)
						continue
					}
					break // break on success
				}

				slog.Debug("Reddit fetch results", slog.String("query", topic.Topic), slog.Int("post amount", len(posts)))

				// Send each post to Kafka for processing
				for _, post := range posts {

					select {
					case <-ctx.Done():
						slog.Warn("[FetchRedditContentForTopics] Context cancelled, stopping...")
						return
					default:
					}
					// Ignore any reddit posts that have no text content content
					if post.PostContent == "" {
						continue
					}

					dedupeKey := post.PostID
					key := fmt.Sprintf("%s:%s", VALKEY_POSTS_KEY, dedupeKey)

					// Check to see if post has been processed in the last 24 hours
					exists, err := valkeyClient.Do(ctx, valkeyClient.B().Sismember().Key(key).Member(dedupeKey).Build()).AsBool()
					if err != nil {
						fmt.Println(err)
					}

					// if the post exists skip it
					if exists {
						slog.Debug("Skipping duplicate post", slog.String("topic", topic.Topic), slog.String("post_id", post.PostID))
						continue
					}

					// add the post id to valkey and set a 24 expiration timer
					responses := valkeyClient.DoMulti(ctx,
						valkeyClient.B().Sadd().Key(key).Member(dedupeKey).Build(),
						valkeyClient.B().Expire().Key(key).Seconds(86400).Build())

					for _, resp := range responses {
						if err := resp.Error(); err != nil {
							slog.Warn("Failed to add post to Valkey",
								slog.String("post_id", post.PostID),
								slog.String("error", err.Error()))
							continue
						}
					}

					rawContent := redditPostToRaw(post)

					// publish post to kafka
					err = kafka_client.PublishToKafka(kafka_client.KAFKA_TOPIC_RAW_CONTENT, rawContent)
					if err != nil {
						slog.Warn("Failed to publish to Kafka",
							slog.String("topic", rawContent.Topic),
							slog.String("post_id", rawContent.ContentID),
							slog.String("subreddit", rawContent.Metadata.Subreddit),
							slog.String("error", err.Error()))
					}
				}
				// stop paginating when there are no more results
				if nextAfter == "" {
					break
				}
				after = nextAfter
			}
		}
	}

	slog.Info("Successfully fetched & sent Reddit content to Kafka!")
}

func redditPostToRaw(p models.RedditPost) models.RawContent {
	return models.RawContent{
		ContentID: p.PostID,
		Source:    "reddit",
		Topic:     p.Topic,
		Text:      p.PostContent,
		Metadata: models.ContentMetadata{
			Author:    p.Author,
			Timestamp: p.CreatedAt,
			Subreddit: p.Subreddit,
		},
	}
}
