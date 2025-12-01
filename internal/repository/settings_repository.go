package repository

// SettingsRepository - работа с таблицей settings
//
// Назначение: Data Access Layer для глобальных настроек
//
// Функции:
// - Get: получить настройки (всегда id=1, одна запись)
// - Update: обновить настройки
// - UpdateNotificationPrefs: обновить только preferences уведомлений
//
// Настройки:
// - consider_funding: учитывать ли фандинг-рейты
// - max_concurrent_trades: лимит одновременных арбитражей
// - notification_prefs: какие уведомления показывать
//
// TODO: реализовать операции для settings таблицы
