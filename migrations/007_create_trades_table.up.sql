CREATE TABLE IF NOT EXISTS trades (
    id SERIAL PRIMARY KEY,
    pair_id INT REFERENCES pairs(id) ON DELETE CASCADE,
    symbol VARCHAR(20) NOT NULL,
    exchanges VARCHAR(100),  -- "bybit,okx"
    entry_time TIMESTAMP NOT NULL,
    exit_time TIMESTAMP NOT NULL,
    pnl DECIMAL(20, 2) NOT NULL,
    was_stop_loss BOOLEAN DEFAULT false,
    was_liquidation BOOLEAN DEFAULT false,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_trades_exit_time ON trades(exit_time DESC);
CREATE INDEX idx_trades_pair_id ON trades(pair_id);
CREATE INDEX idx_trades_symbol ON trades(symbol);
CREATE INDEX idx_trades_pnl ON trades(pnl DESC);
