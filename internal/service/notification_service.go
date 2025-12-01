package service

// NotificationService - бизнес-логика для уведомлений
//
// Назначение:
// Создает и управляет уведомлениями с учетом настроек.
//
// Функции:
// - CreateNotification: создать уведомление
//   * Проверка settings (включен ли этот тип)
//   * Сохранение в БД
//   * Broadcast через WebSocket hub
// - GetNotifications: получить уведомления
//   * Фильтрация по типам
//   * Пагинация (последние N)
// - ClearNotifications: очистить журнал
//
// Типы уведомлений:
// - OPEN, CLOSE, SL, LIQUIDATION, ERROR, MARGIN, PAUSE, SECOND_LEG_FAIL
//
// TODO: реализовать создание и управление уведомлениями
