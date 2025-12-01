CREATE TABLE IF NOT EXISTS blacklist (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) UNIQUE NOT NULL,
    reason TEXT,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_blacklist_symbol ON blacklist(symbol);
