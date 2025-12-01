CREATE TABLE IF NOT EXISTS settings (
    id INT PRIMARY KEY DEFAULT 1,
    consider_funding BOOLEAN DEFAULT false,
    max_concurrent_trades INT,
    notification_prefs JSONB DEFAULT '{"open": true, "close": true, "stop_loss": true, "liquidation": true, "api_error": true, "margin": true, "pause": true, "second_leg_fail": true}'::jsonb,
    updated_at TIMESTAMP DEFAULT NOW()
);

-- Вставляем начальную запись настроек
INSERT INTO settings (id) VALUES (1) ON CONFLICT DO NOTHING;

-- Триггер для обновления updated_at
CREATE OR REPLACE FUNCTION update_updated_at_column()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = NOW();
    RETURN NEW;
END;
$$ language 'plpgsql';

CREATE TRIGGER update_settings_updated_at BEFORE UPDATE ON settings
FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();
