CREATE TABLE IF NOT EXISTS sentiment_results (
    content_id TEXT PRIMARY KEY,
    headline_id TEXT,
    headline_query TEXT,
    headline_category TEXT,
    source TEXT,
    raw_content_topic TEXT,
    text_analyzed TEXT,
    was_summarized BOOLEAN,
    original_text TEXT,
    metadata_timestamp TIMESTAMPTZ,
    metadata_author TEXT,
    metadata_subreddit TEXT,
    metadata_post_id TEXT,
    metadata_url TEXT,
    sentiment_score DOUBLE PRECISION,
    sentiment_label TEXT,
    confidence DOUBLE PRECISION,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    CONSTRAINT "fk_headline"
        -- Quoted constraint name
        FOREIGN KEY(headline_id)
        REFERENCES headlines(id)
        ON DELETE SET NULL -- Or ON DELETE CASCADE, depending on desired behavior
);

CREATE INDEX IF NOT EXISTS idx_sentiment_results_headline_id ON sentiment_results(headline_id);
CREATE INDEX IF NOT EXISTS idx_sentiment_results_source ON sentiment_results(source);
CREATE INDEX IF NOT EXISTS idx_sentiment_results_raw_content_topic ON sentiment_results(raw_content_topic);
CREATE INDEX IF NOT EXISTS idx_sentiment_results_sentiment_label ON sentiment_results(sentiment_label);
CREATE INDEX IF NOT EXISTS idx_sentiment_results_metadata_timestamp ON sentiment_results(metadata_timestamp);
