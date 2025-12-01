package handlers

import (
	"encoding/json"
	"net/http"
)

// SettingsHandler отвечает за управление глобальными настройками бота
//
// Функции:
// - Получение глобальных настроек (GET /api/settings)
// - Обновление настроек (PATCH /api/settings)
//
// Назначение:
// Обрабатывает запросы для управления глобальными параметрами бота:
// - max_concurrent_trades: ограничение на количество одновременных арбитражей
// - consider_funding: учитывать ли фандинг-рейты (future feature)
// - notification_prefs: настройки отображения типов уведомлений
// Позволяет пользователю настроить поведение бота и UI
type SettingsHandler struct {
	// TODO: добавить зависимости (service)
}

// NewSettingsHandler создает новый SettingsHandler
func NewSettingsHandler() *SettingsHandler {
	return &SettingsHandler{}
}

// GetSettings возвращает текущие глобальные настройки
// GET /api/settings
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Получить настройки из БД через service
	//    (в БД всегда только одна запись с id=1)
	// 2. Вернуть объект настроек

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":                    1,
		"consider_funding":      false,
		"max_concurrent_trades": nil,
		"notification_prefs": map[string]bool{
			"open":            true,
			"close":           true,
			"stop_loss":       true,
			"liquidation":     true,
			"api_error":       true,
			"margin":          true,
			"pause":           true,
			"second_leg_fail": true,
		},
	})
}

// UpdateSettings обновляет глобальные настройки
// PATCH /api/settings
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Декодировать JSON body с новыми значениями
	// 2. Валидировать параметры:
	//    - max_concurrent_trades: >= 1 или null
	//    - notification_prefs: все ключи валидны
	// 3. Обновить настройки в БД через service
	// 4. Применить изменения в runtime боте
	// 5. Вернуть обновленный объект настроек

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Settings updated (not implemented)",
	})
}
