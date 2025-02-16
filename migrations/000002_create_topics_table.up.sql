CREATE TABLE IF NOT EXISTS topics (
    id SERIAL PRIMARY KEY,
    topic TEXT NOT NULL,
    categroy TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);
