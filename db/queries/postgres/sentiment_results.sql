-- name: CreateSentimentResult :one
INSERT INTO sentiment_results (
    content_id, headline_id, headline_query, headline_category,
    source, raw_content_topic, text_analyzed, was_summarized, original_text,
    metadata_timestamp, metadata_author, metadata_subreddit, metadata_post_id, metadata_url,
    sentiment_score, sentiment_label, confidence
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17
)
RETURNING *;

-- name: GetSentimentResult :one
SELECT * FROM sentiment_results
WHERE content_id = $1 LIMIT 1;

-- name: ListSentimentResultsByHeadline :many
SELECT * FROM sentiment_results
WHERE headline_id = $1
ORDER BY created_at DESC
LIMIT sqlc.arg(item_limit)
OFFSET sqlc.arg(item_offset);

-- name: ListSentimentResultsBySource :many
-- Example read query: lists results by their raw content source
SELECT * FROM sentiment_results
WHERE source = $1
ORDER BY created_at DESC
LIMIT sqlc.arg(item_limit)
OFFSET sqlc.arg(item_offset);

-- name: DeleteSentimentResult :exec
DELETE FROM sentiment_results
WHERE content_id = $1;

-- name: CreateSentimentResultsBatch :batchexec
-- Inserts multiple sentiment results in a single query using UNNEST.
INSERT INTO sentiment_results (
    content_id, headline_id, headline_query, headline_category,
    source, raw_content_topic, text_analyzed, was_summarized, original_text,
    metadata_timestamp, metadata_author, metadata_subreddit, metadata_post_id, metadata_url,
    sentiment_score, sentiment_label, confidence
)
SELECT
    p_content_id, p_headline_id, p_headline_query, p_headline_category,
    p_source, p_raw_content_topic, p_text_analyzed, p_was_summarized, p_original_text,
    p_metadata_timestamp, p_metadata_author, p_metadata_subreddit, p_metadata_post_id, p_metadata_url,
    p_sentiment_score, p_sentiment_label, p_confidence
FROM UNNEST(
    sqlc.arg(content_ids)::text[],
    sqlc.arg(headline_ids)::text[],
    sqlc.arg(headline_queries)::text[],
    sqlc.arg(headline_categories)::text[],
    sqlc.arg(sources)::text[],
    sqlc.arg(raw_content_topics)::text[],
    sqlc.arg(texts_analyzed)::text[],
    sqlc.arg(were_summarized)::boolean[],
    sqlc.arg(original_texts)::text[],
    sqlc.arg(metadata_timestamps)::timestamptz[],
    sqlc.arg(metadata_authors)::text[],
    sqlc.arg(metadata_subreddits)::text[],
    sqlc.arg(metadata_post_ids)::text[],
    sqlc.arg(metadata_urls)::text[],
    sqlc.arg(sentiment_scores)::double precision[],
    sqlc.arg(sentiment_labels)::text[],
    sqlc.arg(confidences)::double precision[]
) AS t(p_content_id, p_headline_id, p_headline_query, p_headline_category, p_source, p_raw_content_topic, p_text_analyzed, p_was_summarized, p_original_text, p_metadata_timestamp, p_metadata_author, p_metadata_subreddit, p_metadata_post_id, p_metadata_url, p_sentiment_score, p_sentiment_label, p_confidence);
