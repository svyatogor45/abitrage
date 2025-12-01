DROP TRIGGER IF EXISTS update_settings_updated_at ON settings;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS settings CASCADE;
