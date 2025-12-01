package handlers

import (
	"encoding/json"
	"net/http"
)

// NotificationHandler отвечает за управление уведомлениями
//
// Функции:
// - Получение списка уведомлений (GET /api/notifications)
// - Фильтрация по типам (GET /api/notifications?types=open,close,error)
// - Очистка журнала уведомлений (DELETE /api/notifications)
//
// Назначение:
// Обрабатывает запросы на получение журнала событий бота,
// поддерживает фильтрацию по типам событий (открытие, закрытие, SL, ошибки),
// обеспечивает пагинацию (последние 100 событий),
// позволяет очищать историю уведомлений
type NotificationHandler struct {
	// TODO: добавить зависимости (service)
}

// NewNotificationHandler создает новый NotificationHandler
func NewNotificationHandler() *NotificationHandler {
	return &NotificationHandler{}
}

// GetNotifications возвращает список уведомлений с фильтрацией
// GET /api/notifications?types=open,close,error&limit=100
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Получить query параметры:
	//    - types (comma-separated): фильтр по типам
	//    - limit (int): количество записей (по умолчанию 100)
	// 2. Парсинг типов в массив
	// 3. Получить уведомления из БД через service
	// 4. Фильтрация по типам если указано
	// 5. Вернуть массив уведомлений (сортировка: новые сверху)

	types := r.URL.Query().Get("types") // например: "open,close,sl"
	_ = types

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]map[string]interface{}{
		{
			"id":        1,
			"timestamp": "2025-12-01T12:00:00Z",
			"type":      "OPEN",
			"severity":  "info",
			"message":   "Открыт арбитраж по паре BTCUSDT",
		},
	})
}

// ClearNotifications очищает журнал уведомлений
// DELETE /api/notifications
func (h *NotificationHandler) ClearNotifications(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Удалить все уведомления из БД через service
	// 2. Вернуть статус 204 No Content

	w.WriteHeader(http.StatusNoContent)
}
