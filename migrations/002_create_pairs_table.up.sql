CREATE TABLE IF NOT EXISTS pairs (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(20) NOT NULL,
    base VARCHAR(10) NOT NULL,
    quote VARCHAR(10) NOT NULL,
    entry_spread_pct DECIMAL(10, 4) NOT NULL,
    exit_spread_pct DECIMAL(10, 4) NOT NULL,
    volume_asset DECIMAL(20, 8) NOT NULL,
    n_orders INT DEFAULT 1,
    stop_loss DECIMAL(20, 2),
    status VARCHAR(20) DEFAULT 'paused',
    trades_count INT DEFAULT 0,
    total_pnl DECIMAL(20, 2) DEFAULT 0,
    created_at TIMESTAMP DEFAULT NOW(),
    updated_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_pairs_symbol ON pairs(symbol);
CREATE INDEX idx_pairs_status ON pairs(status);
