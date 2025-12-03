package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

// ============ NotificationHandler Tests ============

func TestNotificationHandler_GetNotifications(t *testing.T) {
	t.Run("returns empty list when no notifications", func(t *testing.T) {
		mockSvc := NewMockNotificationService()
		handler := NewNotificationHandler(mockSvc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
		w := httptest.NewRecorder()

		handler.GetNotifications(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response GetNotificationsResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Total != 0 {
			t.Errorf("expected total 0, got %d", response.Total)
		}
		if len(response.Notifications) != 0 {
			t.Errorf("expected 0 notifications, got %d", len(response.Notifications))
		}
	})

	t.Run("returns existing notifications", func(t *testing.T) {
		mockSvc := NewMockNotificationService()
		handler := NewNotificationHandler(mockSvc)

		// Добавляем уведомления
		mockSvc.AddNotification("OPEN", "INFO", "Opened position BTCUSDT")
		mockSvc.AddNotification("CLOSE", "INFO", "Closed position BTCUSDT")
		mockSvc.AddNotification("ERROR", "ERROR", "API error")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
		w := httptest.NewRecorder()

		handler.GetNotifications(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response GetNotificationsResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Total != 3 {
			t.Errorf("expected total 3, got %d", response.Total)
		}
	})

	t.Run("filters by types", func(t *testing.T) {
		mockSvc := NewMockNotificationService()
		handler := NewNotificationHandler(mockSvc)

		mockSvc.AddNotification("OPEN", "INFO", "Opened position")
		mockSvc.AddNotification("CLOSE", "INFO", "Closed position")
		mockSvc.AddNotification("ERROR", "ERROR", "API error")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?types=OPEN,CLOSE", nil)
		w := httptest.NewRecorder()

		handler.GetNotifications(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response GetNotificationsResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Total != 2 {
			t.Errorf("expected total 2 (filtered), got %d", response.Total)
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		mockSvc := NewMockNotificationService()
		handler := NewNotificationHandler(mockSvc)

		// Добавляем 10 уведомлений
		for i := 0; i < 10; i++ {
			mockSvc.AddNotification("OPEN", "INFO", "Notification")
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications?limit=5", nil)
		w := httptest.NewRecorder()

		handler.GetNotifications(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response GetNotificationsResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Total != 5 {
			t.Errorf("expected total 5 (limited), got %d", response.Total)
		}
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mockSvc := NewMockNotificationService()
		handler := NewNotificationHandler(mockSvc)

		mockSvc.SetError("get", ErrMockDatabase)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
		w := httptest.NewRecorder()

		handler.GetNotifications(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

func TestNotificationHandler_ClearNotifications(t *testing.T) {
	t.Run("successfully clears notifications", func(t *testing.T) {
		mockSvc := NewMockNotificationService()
		handler := NewNotificationHandler(mockSvc)

		// Добавляем уведомления
		mockSvc.AddNotification("OPEN", "INFO", "Test notification")
		mockSvc.AddNotification("CLOSE", "INFO", "Test notification 2")

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/notifications", nil)
		w := httptest.NewRecorder()

		handler.ClearNotifications(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response ClearNotificationsResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Message == "" {
			t.Error("expected non-empty message")
		}

		// Проверяем что уведомления удалены
		count, _ := mockSvc.GetNotificationCount()
		if count != 0 {
			t.Errorf("expected 0 notifications after clear, got %d", count)
		}
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mockSvc := NewMockNotificationService()
		handler := NewNotificationHandler(mockSvc)

		mockSvc.SetError("clear", ErrMockDatabase)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/notifications", nil)
		w := httptest.NewRecorder()

		handler.ClearNotifications(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

func TestNotificationHandler_DefaultLimit(t *testing.T) {
	t.Run("uses default limit when not specified", func(t *testing.T) {
		mockSvc := NewMockNotificationService()
		handler := NewNotificationHandler(mockSvc)

		// Добавляем 150 уведомлений
		for i := 0; i < 150; i++ {
			mockSvc.AddNotification("OPEN", "INFO", "Notification")
		}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/notifications", nil)
		w := httptest.NewRecorder()

		handler.GetNotifications(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response GetNotificationsResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		// По умолчанию лимит 100
		if response.Total != 100 {
			t.Errorf("expected default limit 100, got %d", response.Total)
		}
	})
}
