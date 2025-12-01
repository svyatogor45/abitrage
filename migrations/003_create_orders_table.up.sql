CREATE TABLE IF NOT EXISTS orders (
    id SERIAL PRIMARY KEY,
    pair_id INT REFERENCES pairs(id) ON DELETE CASCADE,
    exchange VARCHAR(50) NOT NULL,
    side VARCHAR(10) NOT NULL,
    type VARCHAR(20) DEFAULT 'market',
    part_index INT DEFAULT 0,
    quantity DECIMAL(20, 8) NOT NULL,
    price_avg DECIMAL(20, 8),
    status VARCHAR(20) NOT NULL,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT NOW(),
    filled_at TIMESTAMP
);

CREATE INDEX idx_orders_pair_id ON orders(pair_id);
CREATE INDEX idx_orders_status ON orders(status);
CREATE INDEX idx_orders_created_at ON orders(created_at DESC);
