package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"arbitrage/internal/service"
)

// SettingsHandler отвечает за управление глобальными настройками бота.
//
// Endpoints:
// - GET /api/v1/settings - получение глобальных настроек
// - PATCH /api/v1/settings - обновление настроек
//
// Настройки включают:
// - max_concurrent_trades: ограничение на количество одновременных арбитражей (null = без ограничений)
// - consider_funding: учитывать ли фандинг-рейты (future feature)
// - notification_prefs: настройки отображения типов уведомлений
type SettingsHandler struct {
	settingsService service.SettingsServiceInterface
}

// NewSettingsHandler создает новый SettingsHandler с внедрением зависимостей.
func NewSettingsHandler(settingsService service.SettingsServiceInterface) *SettingsHandler {
	return &SettingsHandler{
		settingsService: settingsService,
	}
}

// GetSettings возвращает текущие глобальные настройки.
//
// GET /api/v1/settings
//
// Response 200 OK:
//
//	{
//	  "id": 1,
//	  "consider_funding": false,
//	  "max_concurrent_trades": null,
//	  "notification_prefs": {
//	    "open": true,
//	    "close": true,
//	    "stop_loss": true,
//	    "liquidation": true,
//	    "api_error": true,
//	    "margin": true,
//	    "pause": true,
//	    "second_leg_fail": true
//	  },
//	  "updated_at": "2025-12-01T12:00:00Z"
//	}
//
// Response 500 Internal Server Error:
//
//	{"error": "failed to get settings", "details": "..."}
func (h *SettingsHandler) GetSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Проверяем, что сервис инициализирован
	if h.settingsService == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "settings service not initialized",
		})
		return
	}

	// Получаем настройки через сервис
	settings, err := h.settingsService.GetSettings()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "failed to get settings",
			"details": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(settings)
}

// UpdateSettingsRequest представляет тело запроса на обновление настроек.
// Все поля опциональны - обновляются только переданные.
type UpdateSettingsRequest struct {
	ConsiderFunding          *bool                    `json:"consider_funding,omitempty"`
	MaxConcurrentTrades      *int                     `json:"max_concurrent_trades,omitempty"`
	NotificationPrefs        *NotificationPrefsUpdate `json:"notification_prefs,omitempty"`
	ClearMaxConcurrentTrades *bool                    `json:"clear_max_concurrent_trades,omitempty"`
}

// NotificationPrefsUpdate представляет обновление настроек уведомлений.
// Все поля опциональны для частичного обновления.
type NotificationPrefsUpdate struct {
	Open          *bool `json:"open,omitempty"`
	Close         *bool `json:"close,omitempty"`
	StopLoss      *bool `json:"stop_loss,omitempty"`
	Liquidation   *bool `json:"liquidation,omitempty"`
	APIError      *bool `json:"api_error,omitempty"`
	Margin        *bool `json:"margin,omitempty"`
	Pause         *bool `json:"pause,omitempty"`
	SecondLegFail *bool `json:"second_leg_fail,omitempty"`
}

// UpdateSettings обновляет глобальные настройки.
//
// PATCH /api/v1/settings
//
// Request Body (все поля опциональны):
//
//	{
//	  "consider_funding": true,
//	  "max_concurrent_trades": 5,
//	  "notification_prefs": {
//	    "open": true,
//	    "close": false
//	  },
//	  "clear_max_concurrent_trades": false
//	}
//
// Особенности:
// - Обновляются только переданные поля
// - Для сброса max_concurrent_trades в null используйте "clear_max_concurrent_trades": true
// - notification_prefs поддерживает частичное обновление (только указанные типы)
//
// Response 200 OK:
//
//	{
//	  "id": 1,
//	  "consider_funding": true,
//	  "max_concurrent_trades": 5,
//	  "notification_prefs": { ... },
//	  "updated_at": "2025-12-01T12:30:00Z"
//	}
//
// Response 400 Bad Request:
//
//	{"error": "invalid request body", "details": "..."}
//	{"error": "validation error", "details": "max_concurrent_trades must be >= 1 or null"}
//
// Response 500 Internal Server Error:
//
//	{"error": "failed to update settings", "details": "..."}
func (h *SettingsHandler) UpdateSettings(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Проверяем, что сервис инициализирован
	if h.settingsService == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "settings service not initialized",
		})
		return
	}

	// Декодируем тело запроса
	var req UpdateSettingsRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "invalid request body",
			"details": err.Error(),
		})
		return
	}

	// Проверяем, что хотя бы одно поле передано
	if req.ConsiderFunding == nil &&
		req.MaxConcurrentTrades == nil &&
		req.NotificationPrefs == nil &&
		(req.ClearMaxConcurrentTrades == nil || !*req.ClearMaxConcurrentTrades) {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "no fields to update",
			"details": "at least one field must be provided",
		})
		return
	}

	// Получаем текущие настройки для частичного обновления notification_prefs
	currentSettings, err := h.settingsService.GetSettings()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "failed to get current settings",
			"details": err.Error(),
		})
		return
	}

	// Формируем запрос на обновление
	updateReq := &service.UpdateSettingsRequest{
		ConsiderFunding:     req.ConsiderFunding,
		MaxConcurrentTrades: req.MaxConcurrentTrades,
	}

	// Обработка clear_max_concurrent_trades
	if req.ClearMaxConcurrentTrades != nil && *req.ClearMaxConcurrentTrades {
		updateReq.ClearMaxConcurrentTrades = true
	}

	// Обработка частичного обновления notification_prefs
	if req.NotificationPrefs != nil {
		// Берем текущие настройки и обновляем только указанные поля
		prefs := currentSettings.NotificationPrefs

		if req.NotificationPrefs.Open != nil {
			prefs.Open = *req.NotificationPrefs.Open
		}
		if req.NotificationPrefs.Close != nil {
			prefs.Close = *req.NotificationPrefs.Close
		}
		if req.NotificationPrefs.StopLoss != nil {
			prefs.StopLoss = *req.NotificationPrefs.StopLoss
		}
		if req.NotificationPrefs.Liquidation != nil {
			prefs.Liquidation = *req.NotificationPrefs.Liquidation
		}
		if req.NotificationPrefs.APIError != nil {
			prefs.APIError = *req.NotificationPrefs.APIError
		}
		if req.NotificationPrefs.Margin != nil {
			prefs.Margin = *req.NotificationPrefs.Margin
		}
		if req.NotificationPrefs.Pause != nil {
			prefs.Pause = *req.NotificationPrefs.Pause
		}
		if req.NotificationPrefs.SecondLegFail != nil {
			prefs.SecondLegFail = *req.NotificationPrefs.SecondLegFail
		}

		updateReq.NotificationPrefs = &prefs
	}

	// Обновляем настройки через сервис
	updatedSettings, err := h.settingsService.UpdateSettings(updateReq)
	if err != nil {
		// Проверяем тип ошибки для правильного HTTP кода
		if errors.Is(err, service.ErrInvalidMaxConcurrentTrades) {
			w.WriteHeader(http.StatusBadRequest)
			json.NewEncoder(w).Encode(map[string]string{
				"error":   "validation error",
				"details": err.Error(),
			})
			return
		}

		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "failed to update settings",
			"details": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(updatedSettings)
}
