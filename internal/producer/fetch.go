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

func FetchAndStoreTopics(ctx context.Context) {
	slog.Info("[FetchAndStoreTopics] Fetching new topics...")

	maxRetries := 3
	start := time.Now()
	for attempt := 1; attempt <= maxRetries; attempt++ {

		select {
		case <-ctx.Done():
			slog.Warn("[FetchAndStoreTopics] Context cancelled, stopping...")
		default:
		}

		slog.Info("[FetchAndStoreTopics] Attempt", slog.Int("atttemp", attempt))

		// Rate limited :(
		headlines, err := clients.GetNewsAPIClient().GetTopHeadlinesByCategory()
		if err != nil {
			slog.Warn("[FetchAndStoreTopics] Failed to fetch latested headlines from NewsAPI, retrying...",
				slog.String("error", err.Error()))
			time.Sleep(5 * time.Second)
			continue
		}

		// For when NewsAPI account gets rate-limited
		// headlines, err := clients.GetNewsAPIClient().GetTopHeadlinesFromFile()
		// if err != nil {
		// 	slog.Warn("[FetchAndStoreTopics] Failed to fetch latested headlines from NewsAPI, retrying...",
		// 		slog.String("error", err.Error()))
		// 	time.Sleep(5 * time.Second)
		// 	continue
		// }

		topics, err := GenerateTopicsFromHeadlines(headlines)
		if err != nil {
			slog.Warn("[FetchAndStoreTopics] Failed to generate topics, retrying...", slog.String("error", err.Error()))
			time.Sleep(5 * time.Second)
			continue
		}

		err = db.StoreBatchedTopics(topics.Topics)
		if err != nil {
			slog.Error("[FetchAndStoreTopics] Failed to store topics in DB", slog.String("error", err.Error()))
			time.Sleep(5 * time.Second)
			continue
		}

		slog.Info("[FetchAndStoreTopics] Successfully stored generated topics to DB", slog.Duration("duration", time.Since(start)))
		return
	}

	slog.Error("Maximum retries reached, skipping this fetch cycle")
}

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

					// publish post to kafka
					err = kafka_client.PublishToKafka(post)
					if err != nil {
						slog.Warn("Failed to publish to Kafka",
							slog.String("topic", post.Topic),
							slog.String("post_id", post.PostID),
							slog.String("subreddit", post.Subreddit),
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
