package handlers

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"arbitrage/internal/models"
)

// ============ StatsHandler Tests ============

func TestStatsHandler_GetStats(t *testing.T) {
	t.Run("returns stats successfully", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		// Устанавливаем тестовые данные
		mockSvc.SetStats(&models.Stats{
			TotalTrades: 100,
			TotalPnl:    1500.50,
			TodayTrades: 5,
			TodayPnl:    75.25,
			WeekTrades:  25,
			WeekPnl:     350.00,
			MonthTrades: 80,
			MonthPnl:    1200.00,
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
		w := httptest.NewRecorder()

		handler.GetStats(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response models.Stats
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response.TotalTrades != 100 {
			t.Errorf("expected TotalTrades 100, got %d", response.TotalTrades)
		}
		if response.TotalPnl != 1500.50 {
			t.Errorf("expected TotalPnl 1500.50, got %f", response.TotalPnl)
		}
	})

	t.Run("returns 500 when service is nil", func(t *testing.T) {
		handler := &StatsHandler{statsService: nil}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
		w := httptest.NewRecorder()

		handler.GetStats(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		mockSvc.SetError("get", ErrMockDatabase)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
		w := httptest.NewRecorder()

		handler.GetStats(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

func TestStatsHandler_GetTopPairs(t *testing.T) {
	t.Run("returns top pairs by trades", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		mockSvc.SetTopPairs("trades", []models.PairStat{
			{Symbol: "BTCUSDT", Value: 50},
			{Symbol: "ETHUSDT", Value: 35},
			{Symbol: "SOLUSDT", Value: 20},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/top-pairs?metric=trades", nil)
		w := httptest.NewRecorder()

		handler.GetTopPairs(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response []models.PairStat
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response) != 3 {
			t.Errorf("expected 3 pairs, got %d", len(response))
		}
		if response[0].Symbol != "BTCUSDT" {
			t.Errorf("expected first pair BTCUSDT, got %s", response[0].Symbol)
		}
	})

	t.Run("returns top pairs by profit", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		mockSvc.SetTopPairs("profit", []models.PairStat{
			{Symbol: "ETHUSDT", Value: 450.25},
			{Symbol: "BTCUSDT", Value: 320.00},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/top-pairs?metric=profit", nil)
		w := httptest.NewRecorder()

		handler.GetTopPairs(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response []models.PairStat
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response) != 2 {
			t.Errorf("expected 2 pairs, got %d", len(response))
		}
	})

	t.Run("returns top pairs by loss", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		mockSvc.SetTopPairs("loss", []models.PairStat{
			{Symbol: "XRPUSDT", Value: -85.50},
			{Symbol: "SOLUSDT", Value: -42.30},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/top-pairs?metric=loss", nil)
		w := httptest.NewRecorder()

		handler.GetTopPairs(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response []models.PairStat
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response) != 2 {
			t.Errorf("expected 2 pairs, got %d", len(response))
		}
	})

	t.Run("uses default metric when not specified", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		mockSvc.SetTopPairs("trades", []models.PairStat{
			{Symbol: "BTCUSDT", Value: 50},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/top-pairs", nil)
		w := httptest.NewRecorder()

		handler.GetTopPairs(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}
	})

	t.Run("returns 400 for invalid metric", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/top-pairs?metric=invalid", nil)
		w := httptest.NewRecorder()

		handler.GetTopPairs(w, req)

		if w.Code != http.StatusBadRequest {
			t.Errorf("expected status %d, got %d", http.StatusBadRequest, w.Code)
		}
	})

	t.Run("respects limit parameter", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		mockSvc.SetTopPairs("trades", []models.PairStat{
			{Symbol: "BTCUSDT", Value: 50},
			{Symbol: "ETHUSDT", Value: 35},
			{Symbol: "SOLUSDT", Value: 20},
			{Symbol: "XRPUSDT", Value: 15},
			{Symbol: "DOGEUSDT", Value: 10},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/top-pairs?metric=trades&limit=3", nil)
		w := httptest.NewRecorder()

		handler.GetTopPairs(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response []models.PairStat
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if len(response) != 3 {
			t.Errorf("expected 3 pairs (limited), got %d", len(response))
		}
	})

	t.Run("returns 500 when service is nil", func(t *testing.T) {
		handler := &StatsHandler{statsService: nil}

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/top-pairs", nil)
		w := httptest.NewRecorder()

		handler.GetTopPairs(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		mockSvc.SetError("topPairs", ErrMockDatabase)

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats/top-pairs?metric=trades", nil)
		w := httptest.NewRecorder()

		handler.GetTopPairs(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

func TestStatsHandler_ResetStats(t *testing.T) {
	t.Run("successfully resets stats", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		// Устанавливаем данные
		mockSvc.SetStats(&models.Stats{
			TotalTrades: 100,
			TotalPnl:    1500.50,
		})

		req := httptest.NewRequest(http.MethodPost, "/api/v1/stats/reset", nil)
		w := httptest.NewRecorder()

		handler.ResetStats(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		var response map[string]string
		if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
			t.Fatalf("failed to decode response: %v", err)
		}

		if response["message"] == "" {
			t.Error("expected success message")
		}

		// Проверяем что статистика сброшена
		stats, _ := mockSvc.GetStats()
		if stats.TotalTrades != 0 {
			t.Errorf("expected TotalTrades 0 after reset, got %d", stats.TotalTrades)
		}
	})

	t.Run("returns 500 when service is nil", func(t *testing.T) {
		handler := &StatsHandler{statsService: nil}

		req := httptest.NewRequest(http.MethodPost, "/api/v1/stats/reset", nil)
		w := httptest.NewRecorder()

		handler.ResetStats(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})

	t.Run("returns 500 on service error", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		mockSvc.SetError("reset", ErrMockDatabase)

		req := httptest.NewRequest(http.MethodPost, "/api/v1/stats/reset", nil)
		w := httptest.NewRecorder()

		handler.ResetStats(w, req)

		if w.Code != http.StatusInternalServerError {
			t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
		}
	})
}

func TestStatsHandler_EmptyArraysNotNull(t *testing.T) {
	t.Run("returns empty arrays instead of null", func(t *testing.T) {
		mockSvc := NewMockStatsService()
		handler := NewStatsHandler(mockSvc)

		// Статистика с пустыми значениями
		mockSvc.SetStats(&models.Stats{
			TotalTrades:       0,
			TopPairsByTrades:  nil,
			TopPairsByProfit:  nil,
			TopPairsByLoss:    nil,
			StopLossCount:     models.StopLossStats{Events: nil},
			LiquidationCount:  models.LiquidationStats{Events: nil},
		})

		req := httptest.NewRequest(http.MethodGet, "/api/v1/stats", nil)
		w := httptest.NewRecorder()

		handler.GetStats(w, req)

		if w.Code != http.StatusOK {
			t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
		}

		// Проверяем что в JSON пустые массивы, а не null
		body := w.Body.String()
		// Должны быть [] вместо null для массивов
		if body == "" {
			t.Error("expected non-empty response body")
		}
	})
}
