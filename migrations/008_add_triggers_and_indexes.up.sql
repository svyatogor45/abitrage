-- Миграция 008: Добавление триггеров updated_at и дополнительных индексов
-- Цель: Консистентность автоматического обновления updated_at во всех таблицах

-- Триггер для автоматического обновления updated_at в exchanges
CREATE TRIGGER update_exchanges_updated_at
    BEFORE UPDATE ON exchanges
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Триггер для автоматического обновления updated_at в pairs
CREATE TRIGGER update_pairs_updated_at
    BEFORE UPDATE ON pairs
    FOR EACH ROW EXECUTE FUNCTION update_updated_at_column();

-- Индекс для быстрой выборки уведомлений по паре
CREATE INDEX IF NOT EXISTS idx_notifications_pair_id ON notifications(pair_id);

-- Составной индекс для статистики trades по периодам
CREATE INDEX IF NOT EXISTS idx_trades_exit_time_pnl ON trades(exit_time DESC, pnl);

-- CHECK constraints для enum-подобных полей (если ещё не существуют)
DO $$
BEGIN
    -- Constraint для pairs.status
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_pairs_status'
    ) THEN
        ALTER TABLE pairs ADD CONSTRAINT chk_pairs_status
            CHECK (status IN ('paused', 'active'));
    END IF;

    -- Constraint для notifications.severity
    IF NOT EXISTS (
        SELECT 1 FROM pg_constraint WHERE conname = 'chk_notifications_severity'
    ) THEN
        ALTER TABLE notifications ADD CONSTRAINT chk_notifications_severity
            CHECK (severity IN ('info', 'warn', 'error'));
    END IF;
END $$;
