package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============ SettingsHandler Tests ============

func TestSettingsHandler_GetSettings(t *testing.T) {
	t.Run("successfully returns settings", func(t *testing.T) {
		mockSvc := NewMockSettingsService()
		handler := NewSettingsHandler(mockSvc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
		w := httptest.NewRecorder()

		handler.GetSettings(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response map[string]interface{}
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// Проверяем что ответ содержит обязательные поля
		if _, ok := response["consider_funding"]; !ok {
			t.Error("response should contain consider_funding field")
		}
		if _, ok := response["notification_prefs"]; !ok {
			t.Error("response should contain notification_prefs field")
		}
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mockSvc := NewMockSettingsService()
		handler := NewSettingsHandler(mockSvc)

		mockSvc.SetError("get", ErrMockDatabase)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/settings", nil)
		w := httptest.NewRecorder()

		handler.GetSettings(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

func TestSettingsHandler_UpdateSettings(t *testing.T) {
	t.Run("successfully updates consider_funding", func(t *testing.T) {
		mockSvc := NewMockSettingsService()
		handler := NewSettingsHandler(mockSvc)

		body := map[string]interface{}{
			"consider_funding": true,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.UpdateSettings(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Проверяем что настройки обновились
		settings, _ := mockSvc.GetSettings()
		if !settings.ConsiderFunding {
			t.Error("consider_funding should be true after update")
		}
	})

	t.Run("successfully updates max_concurrent_trades", func(t *testing.T) {
		mockSvc := NewMockSettingsService()
		handler := NewSettingsHandler(mockSvc)

		maxTrades := 5
		body := map[string]interface{}{
			"max_concurrent_trades": maxTrades,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.UpdateSettings(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Проверяем что настройки обновились
		settings, _ := mockSvc.GetSettings()
		if settings.MaxConcurrentTrades == nil || *settings.MaxConcurrentTrades != maxTrades {
			t.Errorf("max_concurrent_trades should be %d", maxTrades)
		}
	})

	t.Run("successfully clears max_concurrent_trades with clear flag", func(t *testing.T) {
		mockSvc := NewMockSettingsService()
		handler := NewSettingsHandler(mockSvc)

		// Сначала установим значение
		initialVal := 3
		mockSvc.settings.MaxConcurrentTrades = &initialVal

		// Для очистки используем специальный флаг clear_max_concurrent_trades
		body := map[string]interface{}{
			"clear_max_concurrent_trades": true,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.UpdateSettings(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Проверяем что значение сброшено
		settings, _ := mockSvc.GetSettings()
		if settings.MaxConcurrentTrades != nil {
			t.Error("max_concurrent_trades should be nil after clearing")
		}
	})

	t.Run("returns 400 on invalid JSON", func(t *testing.T) {
		mockSvc := NewMockSettingsService()
		handler := NewSettingsHandler(mockSvc)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.UpdateSettings(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mockSvc := NewMockSettingsService()
		handler := NewSettingsHandler(mockSvc)

		mockSvc.SetError("update", ErrMockDatabase)

		body := map[string]interface{}{
			"consider_funding": true,
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.UpdateSettings(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})

	t.Run("updates notification preferences", func(t *testing.T) {
		mockSvc := NewMockSettingsService()
		handler := NewSettingsHandler(mockSvc)

		body := map[string]interface{}{
			"notification_prefs": map[string]bool{
				"open":           false,
				"close":          true,
				"sl":             true,
				"liquidation":    false,
				"error":          true,
				"margin":         false,
				"pause":          true,
				"second_leg_fail": true,
			},
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPatch, "/api/v1/settings", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.UpdateSettings(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})
}
