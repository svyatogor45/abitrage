package repository

// NotificationRepository - работа с таблицей notifications
//
// Назначение: Data Access Layer для уведомлений
//
// Функции:
// - Create: создать новое уведомление
// - GetRecent: получить последние N уведомлений
// - GetByTypes: получить уведомления определенных типов
// - DeleteAll: очистить журнал уведомлений
// - DeleteOlderThan: автоочистка старых уведомлений
//
// Типы уведомлений:
// - OPEN, CLOSE, SL, LIQUIDATION, ERROR, MARGIN, PAUSE, SECOND_LEG_FAIL
//
// TODO: реализовать операции для notifications таблицы
