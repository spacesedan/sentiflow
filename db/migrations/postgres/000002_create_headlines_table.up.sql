CREATE TABLE IF NOT EXISTS headlines (
    id TEXT PRIMARY KEY,
    query TEXT,
    category TEXT,
    sentiment_score REAL,
    meta_source TEXT,
    meta_title TEXT,
    meta_author TEXT,
    meta_description TEXT,
    meta_published_at TIMESTAMPTZ,
    meta_url TEXT,
    meta_url_to_image TEXT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_headlines_query ON headlines(query);
CREATE INDEX IF NOT EXISTS idx_headlines_category ON headlines(category);
CREATE INDEX IF NOT EXISTS idx_headlines_meta_source ON headlines(meta_source);
CREATE INDEX IF NOT EXISTS idx_headlines_meta_published_at ON headlines(meta_published_at);
