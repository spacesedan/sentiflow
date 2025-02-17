CREATE TABLE IF NOT EXISTS topics (
    id SERIAL PRIMARY KEY,
    topic TEXT NOT NULL,
    category TEXT NOT NULL,
    original_headline TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
