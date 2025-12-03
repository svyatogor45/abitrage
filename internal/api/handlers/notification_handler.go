package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"arbitrage/internal/service"
)

// NotificationHandler отвечает за управление уведомлениями
//
// Endpoints:
// - GET /api/v1/notifications - получение списка уведомлений
// - GET /api/v1/notifications?types=open,close,error - с фильтрацией по типам
// - GET /api/v1/notifications?limit=50 - с ограничением количества
// - DELETE /api/v1/notifications - очистка журнала уведомлений
//
// Назначение:
// Обрабатывает запросы на получение журнала событий бота,
// поддерживает фильтрацию по типам событий (открытие, закрытие, SL, ошибки),
// обеспечивает пагинацию (по умолчанию 100 событий),
// позволяет очищать историю уведомлений
type NotificationHandler struct {
	notificationService service.NotificationServiceInterface
}

// NewNotificationHandler создает новый NotificationHandler с внедрением зависимости
func NewNotificationHandler(notificationService service.NotificationServiceInterface) *NotificationHandler {
	return &NotificationHandler{
		notificationService: notificationService,
	}
}

// GetNotificationsResponse представляет ответ списка уведомлений
type GetNotificationsResponse struct {
	Notifications []NotificationDTO `json:"notifications"`
	Total         int               `json:"total"`
}

// NotificationDTO представляет уведомление в API
type NotificationDTO struct {
	ID        int                    `json:"id"`
	Timestamp string                 `json:"timestamp"`
	Type      string                 `json:"type"`
	Severity  string                 `json:"severity"`
	PairID    *int                   `json:"pair_id,omitempty"`
	Message   string                 `json:"message"`
	Meta      map[string]interface{} `json:"meta,omitempty"`
}

// GetNotifications возвращает список уведомлений с фильтрацией
//
// GET /api/v1/notifications
//
// Query параметры:
// - types (string): фильтр по типам через запятую (open,close,sl,liquidation,error,margin,pause,second_leg_fail)
// - limit (int): количество записей (по умолчанию 100, максимум 500)
//
// Типы уведомлений:
// - OPEN: открытие арбитража
// - CLOSE: закрытие позиций
// - SL: срабатывание Stop Loss
// - LIQUIDATION: ликвидация позиции
// - ERROR: ошибка API/ордера
// - MARGIN: недостаток маржи
// - PAUSE: пауза/остановка пары
// - SECOND_LEG_FAIL: не удалось открыть вторую ногу
//
// Примеры запросов:
// - GET /api/v1/notifications - все уведомления (последние 100)
// - GET /api/v1/notifications?types=sl,liquidation,error - только критические
// - GET /api/v1/notifications?limit=50 - последние 50
// - GET /api/v1/notifications?types=open,close&limit=20 - только сделки, 20 записей
//
// HTTP коды:
// - 200 OK: успешно, возвращает массив уведомлений
// - 500 Internal Server Error: ошибка сервера
func (h *NotificationHandler) GetNotifications(w http.ResponseWriter, r *http.Request) {
	// Парсинг query параметров
	typesParam := r.URL.Query().Get("types")
	limitParam := r.URL.Query().Get("limit")

	// Парсинг типов
	var types []string
	if typesParam != "" {
		// Разбиваем по запятой и нормализуем
		parts := strings.Split(typesParam, ",")
		for _, part := range parts {
			trimmed := strings.TrimSpace(part)
			if trimmed != "" {
				types = append(types, strings.ToUpper(trimmed))
			}
		}
	}

	// Парсинг лимита
	limit := 100 // по умолчанию
	if limitParam != "" {
		if parsed, err := strconv.Atoi(limitParam); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	// Получение уведомлений через service
	notifications, err := h.notificationService.GetNotifications(types, limit)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get notifications: "+err.Error())
		return
	}

	// Преобразуем в DTO
	dtos := make([]NotificationDTO, 0, len(notifications))
	for _, n := range notifications {
		dto := NotificationDTO{
			ID:        n.ID,
			Timestamp: n.Timestamp.Format("2006-01-02T15:04:05Z07:00"),
			Type:      n.Type,
			Severity:  n.Severity,
			PairID:    n.PairID,
			Message:   n.Message,
			Meta:      n.Meta,
		}
		dtos = append(dtos, dto)
	}

	// Формируем ответ
	response := GetNotificationsResponse{
		Notifications: dtos,
		Total:         len(dtos),
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

// ClearNotificationsResponse представляет ответ очистки уведомлений
type ClearNotificationsResponse struct {
	Message string `json:"message"`
}

// ClearNotifications очищает журнал уведомлений
//
// DELETE /api/v1/notifications
//
// Удаляет все уведомления из базы данных.
// Это действие необратимо.
//
// HTTP коды:
// - 200 OK: журнал успешно очищен
// - 500 Internal Server Error: ошибка при очистке
func (h *NotificationHandler) ClearNotifications(w http.ResponseWriter, r *http.Request) {
	// Удаляем все уведомления
	if err := h.notificationService.ClearNotifications(); err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to clear notifications: "+err.Error())
		return
	}

	response := ClearNotificationsResponse{
		Message: "Notifications cleared successfully",
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

// respondWithError отправляет JSON ошибку
func (h *NotificationHandler) respondWithError(w http.ResponseWriter, code int, message string) {
	h.respondWithJSON(w, code, map[string]string{"error": message})
}

// respondWithJSON отправляет JSON ответ
func (h *NotificationHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(payload)
}
