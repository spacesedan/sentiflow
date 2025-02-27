package processing

import (
	"fmt"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/spacesedan/sentiflow/internal/clients"
	"github.com/spacesedan/sentiflow/internal/db"
	"github.com/spacesedan/sentiflow/internal/models"
)

func FetchAndStoreTopics() {
	slog.Info("[FetchAndStoreTopics] Fetching new topics...")

	maxRetries := 3
	start := time.Now()
	for attempt := 1; attempt <= maxRetries; attempt++ {
		slog.Info("[FetchAndStoreTopics] Attempt", slog.Int("atttemp", attempt))

		headlines, err := clients.GetNewsAPIClient().GetTopHeadlines()
		if err != nil {
			slog.Warn("[FetchAndStoreTopics] Failed to fetch latested headlines from NewsAPI, retrying...",
				slog.String("error", err.Error()))
			time.Sleep(5 * time.Second)
			continue
		}

		fmt.Println(len(headlines))

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
	CategoryMap             = []string{
		"Technology",
		"Business & Finance",
		"Politics & World Affairs",
		"Entertainment & Pop Culture",
		"Health & Science",
		"Sports",
		"Lifestyle & Society",
		"Memes & Internet Trends",
		"Crime & Law",
	}
	categoryHelpersOnce sync.Once
)

func InitCategoryHelpers() {
	categoryHelpersOnce.Do(func() {
		CategoryMap = CategoryMap[:0]

		for category, subreddits := range CategoryToSubreddits {
			CategoryMap = append(CategoryMap, category)
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

const MAX_TOPIC_QUERY_SIZE = 512

func buildRedditAPIQuery(query string) string {
}

// FetchRedditContentForTopics fetches Reddit posts based on stored topics & sends to Kafka
func FetchRedditContentForTopics() {
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

	dedupeSet := make(map[string]struct{})

	// Process each topic
	for _, topic := range topics {
		subreddits, exists := CategoryToSubreddits[topic.Category]
		if !exists {
			slog.Warn("No matching subreddits found for topic category", slog.String("category", topic.Category))
			continue
		}

		slog.Info("Fetching Reddit posts for topic",
			slog.String("topic", topic.Topic),
			slog.String("category", topic.Category),
			slog.Any("subreddits", subreddits))

		// Fetch Reddit posts from relevant subreddits
		for _, subreddit := range subreddits {
			posts, err := clients.GetRedditClient().FetchSubredditPosts(subreddit, topic.Topic)
			if err != nil {
				slog.Warn("⚠️ Failed to fetch Reddit posts",
					slog.String("subreddit", subreddit),
					slog.String("topic", topic.Topic),
					slog.String("error", err.Error()))
				continue
			}

			// Send each post to Kafka for processing
			for _, post := range posts {

				dedupeKey := fmt.Sprintf("%s-%s", topic.Topic, post.PostID)

				if _, exists := dedupeSet[dedupeKey]; exists {
					continue
				}

				dedupeSet[dedupeKey] = struct{}{}

				err := clients.PublishToKafka(post)
				if err != nil {
					slog.Warn("⚠️ Failed to publish to Kafka",
						slog.String("topic", post.Topic),
						slog.String("subreddit", post.Subreddit),
						slog.String("error", err.Error()))
				}
			}
		}
	}

	slog.Info("Successfully fetched & sent Reddit content to Kafka!")
}
