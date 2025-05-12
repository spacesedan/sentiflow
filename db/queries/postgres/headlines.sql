-- name: GetHeadline :one
SELECT * FROM headlines
WHERE id = $1 LIMIT 1;

-- name: ListHeadlines :many
SELECT * FROM headlines
ORDER BY created_at DESC
LIMIT sqlc.arg(item_limit)
OFFSET sqlc.arg(item_offset);

-- name: CreateHeadline :one
INSERT INTO headlines (
    id, query, category, sentiment_score,
    meta_source, meta_title, meta_author, meta_description,
    meta_published_at, meta_url, meta_url_to_image
) VALUES (
    $1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11
)
RETURNING *;

-- name: UpdateHeadlineSentiment :one
UPDATE headlines
SET sentiment_score = $2, updated_at = NOW()
WHERE id = $1
RETURNING *;

-- name: DeleteHeadline :exec
DELETE FROM headlines
WHERE id = $1;

-- name: CreateHeadlinesBatch :batchexec
-- Inserts multiple headlines in a single query using UNNEST.
-- The order of parameters in the UNNEST function must match the order of columns in the INSERT statement.
INSERT INTO headlines (
    id, query, category, sentiment_score,
    meta_source, meta_title, meta_author, meta_description,
    meta_published_at, meta_url, meta_url_to_image
)
SELECT
    p_id, p_query, p_category, p_sentiment_score,
    p_meta_source, p_meta_title, p_meta_author, p_meta_description,
    p_meta_published_at, p_meta_url, p_meta_url_to_image
FROM UNNEST(
    sqlc.arg(ids)::text[],
    sqlc.arg(queries)::text[],
    sqlc.arg(categories)::text[],
    sqlc.arg(sentiment_scores)::real[],
    sqlc.arg(meta_sources)::text[],
    sqlc.arg(meta_titles)::text[],
    sqlc.arg(meta_authors)::text[],
    sqlc.arg(meta_descriptions)::text[],
    sqlc.arg(meta_published_ats)::timestamptz[],
    sqlc.arg(meta_urls)::text[],
    sqlc.arg(meta_url_to_images)::text[]
) AS t(p_id, p_query, p_category, p_sentiment_score, p_meta_source, p_meta_title, p_meta_author, p_meta_description, p_meta_published_at, p_meta_url, p_meta_url_to_image);
