package db

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/spacesedan/sentiflow/internal/models"
)

// StoreTopics batch store topics our db
func StoreTopics(ctx context.Context, topics []models.Topic) error {
	if len(topics) == 0 {
		return nil // No topics to insert
	}

	query := `INSERT INTO topics (topic, category, original_headline, created_at) VALUES `

	// Prepare placeholders dynamically
	values := []interface{}{}
	placeholderParts := []string{}

	for i, topic := range topics {
		offset := i * 3
		placeholderParts = append(placeholderParts, fmt.Sprintf("($%d, $%d, $%d, NOW())", offset+1, offset+2, offset+3))

		// Append values to the slice
		values = append(values, topic.Topic, topic.Category, topic.OriginalHeadline)
	}

	// Concatenate query with placeholders
	query += strings.Join(placeholderParts, ", ")
	query += `
        ON CONFLICT (original_headline) DO NOTHING
    `

	// Execute batch insert
	_, err := DB.Exec(ctx, query, values...)
	if err != nil {
		return fmt.Errorf("failed to insert topics: %w", err)
	}

	return nil
}

// GetRecentTopics gets the most recent topics that we have stored in out db
func GetRecentTopics(ctx context.Context) ([]string, error) {
	query := `
        SELECT topic FROM topics WHERE created_at > NOW() - INTERVAL '1 day'
    `

	rows, err := DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var topics []string
	for rows.Next() {
		var topic string
		if err := rows.Scan(&topic); err != nil {
			return nil, err
		}
		topics = append(topics, topic)
	}

	return topics, nil
}

// GetRecentHeadlines returns the most recent headlines
func GetRecentHeadlines(ctx context.Context) ([]string, error) {
	query := `
        SELECT original_headline FROM topics WHERE created_at > NOW() - INTERVAL '1 day'
    `

	rows, err := DB.Query(ctx, query)
	if err != nil {
		return nil, err
	}

	var headlines []string
	for rows.Next() {
		var headline string
		if err := rows.Scan(&headline); err != nil {
			return nil, err
		}
		headlines = append(headlines, headline)
	}

	return headlines, nil
}

// GetLatestTopics fetches topics generated in the last 24 hours
func GetLatestTopics(ctx context.Context) ([]models.Topic, error) {
	query := `
        SELECT topic, category, original_headline, created_at
        FROM topics
        WHERE created_at > NOW() - INTERVAL '1 day'
        ORDER BY created_at DESC;
    `

	rows, err := DB.Query(ctx, query)
	if err != nil {
		slog.Error("❌ Failed to query topics from PostgreSQL", slog.String("error", err.Error()))
		return nil, err
	}
	defer rows.Close()

	var topics []models.Topic
	for rows.Next() {
		var topic models.Topic
		err := rows.Scan(&topic.Topic, &topic.Category, &topic.OriginalHeadline, &topic.CreatedAt)
		if err != nil {
			slog.Warn("⚠️ Failed to scan topic row", slog.String("error", err.Error()))
			continue
		}
		topics = append(topics, topic)
	}

	if len(topics) == 0 {
		slog.Warn("⚠️ No new topics found in the last 24 hours")
	}

	return topics, nil
}
