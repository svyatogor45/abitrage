CREATE TABLE IF NOT EXISTS exchanges (
    id SERIAL PRIMARY KEY,
    name VARCHAR(50) UNIQUE NOT NULL,
    api_key TEXT NOT NULL,
    secret_key TEXT NOT NULL,
    passphrase TEXT,
    connected BOOLEAN DEFAULT false,
    balance DECIMAL(20, 8) DEFAULT 0,
    last_error TEXT,
    updated_at TIMESTAMP DEFAULT NOW(),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_exchanges_name ON exchanges(name);
CREATE INDEX idx_exchanges_connected ON exchanges(connected);
