CREATE TABLE IF NOT EXISTS notifications (
    id SERIAL PRIMARY KEY,
    timestamp TIMESTAMP DEFAULT NOW(),
    type VARCHAR(50) NOT NULL,
    severity VARCHAR(10) DEFAULT 'info',
    pair_id INT REFERENCES pairs(id) ON DELETE SET NULL,
    message TEXT NOT NULL,
    meta JSONB
);

CREATE INDEX idx_notifications_timestamp ON notifications(timestamp DESC);
CREATE INDEX idx_notifications_type ON notifications(type);
CREATE INDEX idx_notifications_severity ON notifications(severity);
