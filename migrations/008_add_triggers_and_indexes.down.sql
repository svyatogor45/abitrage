-- Откат миграции 008

-- Удаление триггеров
DROP TRIGGER IF EXISTS update_exchanges_updated_at ON exchanges;
DROP TRIGGER IF EXISTS update_pairs_updated_at ON pairs;

-- Удаление индексов
DROP INDEX IF EXISTS idx_notifications_pair_id;
DROP INDEX IF EXISTS idx_trades_exit_time_pnl;

-- Удаление constraints
ALTER TABLE pairs DROP CONSTRAINT IF EXISTS chk_pairs_status;
ALTER TABLE notifications DROP CONSTRAINT IF EXISTS chk_notifications_severity;
