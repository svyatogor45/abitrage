package handlers

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gorilla/mux"
)

// ============ BlacklistHandler Tests ============

func TestBlacklistHandler_GetBlacklist(t *testing.T) {
	t.Run("returns empty list when no entries", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/blacklist", nil)
		w := httptest.NewRecorder()

		handler.GetBlacklist(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response blacklistResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Total != 0 {
			t.Errorf("expected total 0, got %d", response.Total)
		}
		if len(response.Entries) != 0 {
			t.Errorf("expected 0 entries, got %d", len(response.Entries))
		}
	})

	t.Run("returns existing entries", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		// Добавляем записи
		mockSvc.AddEntry("BTCUSDT", "High volatility")
		mockSvc.AddEntry("ETHUSDT", "Low liquidity")

		req := httptest.NewRequest(http.MethodGet, "/api/v1/blacklist", nil)
		w := httptest.NewRecorder()

		handler.GetBlacklist(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response blacklistResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Total != 2 {
			t.Errorf("expected total 2, got %d", response.Total)
		}
		if len(response.Entries) != 2 {
			t.Errorf("expected 2 entries, got %d", len(response.Entries))
		}
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		mockSvc.SetError("get", ErrMockDatabase)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/blacklist", nil)
		w := httptest.NewRecorder()

		handler.GetBlacklist(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

func TestBlacklistHandler_AddToBlacklist(t *testing.T) {
	t.Run("successfully adds symbol to blacklist", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		body := addToBlacklistRequest{
			Symbol: "BTCUSDT",
			Reason: "Test reason",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/blacklist", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.AddToBlacklist(w, req)

		if w.Code != http.StatusCreated {
			t.Errorf("expected status %d, got %d", http.StatusCreated, w.Code)
		}

		var response blacklistEntryResponse
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.Symbol != "BTCUSDT" {
			t.Errorf("expected symbol BTCUSDT, got %s", response.Symbol)
		}
		if response.Reason != "Test reason" {
			t.Errorf("expected reason 'Test reason', got %s", response.Reason)
		}
		if response.ID == 0 {
			t.Error("expected non-zero ID")
		}
	})

	t.Run("returns 400 when symbol is empty", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		body := addToBlacklistRequest{
			Symbol: "",
			Reason: "Test reason",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/blacklist", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.AddToBlacklist(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("returns 400 on invalid JSON", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/blacklist", bytes.NewReader([]byte("invalid json")))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.AddToBlacklist(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("returns 409 when symbol already exists", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		// Добавляем существующую запись
		mockSvc.AddEntry("BTCUSDT", "Existing reason")

		body := addToBlacklistRequest{
			Symbol: "BTCUSDT",
			Reason: "New reason",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/blacklist", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.AddToBlacklist(w, req)

		if w.Code != http.StatusConflict {
			t.Errorf("expected status %d, got %d", http.StatusConflict, w.Code)
		}
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		mockSvc.SetError("add", ErrMockDatabase)

		body := addToBlacklistRequest{
			Symbol: "BTCUSDT",
			Reason: "Test reason",
		}
		jsonBody, _ := json.Marshal(body)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/blacklist", bytes.NewReader(jsonBody))
		req.Header.Set("Content-Type", "application/json")
		w := httptest.NewRecorder()

		handler.AddToBlacklist(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

func TestBlacklistHandler_RemoveFromBlacklist(t *testing.T) {
	t.Run("successfully removes symbol from blacklist", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		mockSvc.AddEntry("BTCUSDT", "Test reason")

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/blacklist/BTCUSDT", nil)
		req = mux.SetURLVars(req, map[string]string{"symbol": "BTCUSDT"})
		w := httptest.NewRecorder()

		handler.RemoveFromBlacklist(w, req)

		if w.Code != http.StatusNoContent {
			t.Errorf("expected status %d, got %d", http.StatusNoContent, w.Code)
		}
	})

	t.Run("returns 400 when symbol is empty", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/blacklist/", nil)
		req = mux.SetURLVars(req, map[string]string{"symbol": ""})
		w := httptest.NewRecorder()

		handler.RemoveFromBlacklist(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("returns 404 when symbol not found", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/blacklist/UNKNOWN", nil)
		req = mux.SetURLVars(req, map[string]string{"symbol": "UNKNOWN"})
		w := httptest.NewRecorder()

		handler.RemoveFromBlacklist(w, req)

		if w.Code != http.StatusNotFound {
			t.Errorf("expected status %d, got %d", http.StatusNotFound, w.Code)
		}
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mockSvc := NewMockBlacklistService()
		handler := NewBlacklistHandler(mockSvc)

		mockSvc.AddEntry("BTCUSDT", "Test reason")
		mockSvc.SetError("remove", ErrMockDatabase)

		req := httptest.NewRequest(http.MethodDelete, "/api/v1/blacklist/BTCUSDT", nil)
		req = mux.SetURLVars(req, map[string]string{"symbol": "BTCUSDT"})
		w := httptest.NewRecorder()

		handler.RemoveFromBlacklist(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

// Тест helper функций respondJSON и respondError
func TestBlacklistHandler_ResponseHelpers(t *testing.T) {
	t.Run("respondJSON sets correct content type", func(t *testing.T) {
		w := httptest.NewRecorder()
		respondJSON(w, http.StatusOK, map[string]string{"test": "value"})

		if ct := w.Header().Get("Content-Type"); ct != "application/json" {
			t.Errorf("expected Content-Type application/json, got %s", ct)
		}
	})

	t.Run("respondError returns error message", func(t *testing.T) {
		w := httptest.NewRecorder()
		respondError(w, http.StatusBadRequest, "test error")

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}

		var response map[string]string
		json.NewDecoder(w.Body).Decode(&response)

		if response["error"] != "test error" {
			t.Errorf("expected error 'test error', got %s", response["error"])
		}
	})
}
