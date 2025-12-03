package handlers

import (
	"encoding/json"
	"net/http"
	"strconv"

	"arbitrage/internal/models"
	"arbitrage/internal/service"
)

// StatsHandler обрабатывает HTTP запросы для статистики работы бота.
//
// Endpoints:
// - GET /api/v1/stats - получить агрегированную статистику
// - GET /api/v1/stats/top-pairs?metric=trades|profit|loss - топ-5 пар по метрике
// - POST /api/v1/stats/reset - сброс счетчиков статистики
//
// Статистика включает:
// - Количество завершенных арбитражей (день/неделя/месяц/всего)
// - Общий PNL (день/неделя/месяц/всего)
// - Количество срабатываний Stop Loss с деталями
// - Количество ликвидаций с деталями
// - Топ-5 пар по разным метрикам
type StatsHandler struct {
	statsService service.StatsServiceInterface
}

// NewStatsHandler создает новый StatsHandler с внедрением зависимостей.
func NewStatsHandler(statsService service.StatsServiceInterface) *StatsHandler {
	return &StatsHandler{
		statsService: statsService,
	}
}

// GetStats возвращает агрегированную статистику работы бота.
//
// GET /api/v1/stats
//
// Response 200 OK:
//
//	{
//	  "total_trades": 150,
//	  "total_pnl": 1250.50,
//	  "today_trades": 5,
//	  "today_pnl": 45.20,
//	  "week_trades": 25,
//	  "week_pnl": 180.75,
//	  "month_trades": 80,
//	  "month_pnl": 620.30,
//	  "stop_loss_stats": {
//	    "today": 0,
//	    "week": 2,
//	    "month": 5,
//	    "events": [
//	      {
//	        "symbol": "BTCUSDT",
//	        "exchanges": ["bybit", "okx"],
//	        "timestamp": "2025-11-30T14:32:00Z"
//	      }
//	    ]
//	  },
//	  "liquidation_stats": {
//	    "today": 0,
//	    "week": 0,
//	    "month": 1,
//	    "events": [
//	      {
//	        "symbol": "ETHUSDT",
//	        "exchange": "bingx",
//	        "side": "short",
//	        "timestamp": "2025-11-15T03:10:00Z"
//	      }
//	    ]
//	  },
//	  "top_pairs_by_trades": [
//	    {"symbol": "BTCUSDT", "value": 50},
//	    {"symbol": "ETHUSDT", "value": 35}
//	  ],
//	  "top_pairs_by_profit": [
//	    {"symbol": "ETHUSDT", "value": 450.25},
//	    {"symbol": "BTCUSDT", "value": 320.00}
//	  ],
//	  "top_pairs_by_loss": [
//	    {"symbol": "XRPUSDT", "value": -85.50},
//	    {"symbol": "SOLUSDT", "value": -42.30}
//	  ]
//	}
//
// Response 500 Internal Server Error:
//
//	{"error": "failed to get stats", "details": "..."}
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Проверяем, что сервис инициализирован
	if h.statsService == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "stats service not initialized",
		})
		return
	}

	// Получаем статистику через сервис
	stats, err := h.statsService.GetStats()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "failed to get stats",
			"details": err.Error(),
		})
		return
	}

	// Убеждаемся, что пустые массивы возвращаются как [], а не null
	if stats.TopPairsByTrades == nil {
		stats.TopPairsByTrades = []models.PairStat{}
	}
	if stats.TopPairsByProfit == nil {
		stats.TopPairsByProfit = []models.PairStat{}
	}
	if stats.TopPairsByLoss == nil {
		stats.TopPairsByLoss = []models.PairStat{}
	}
	if stats.StopLossCount.Events == nil {
		stats.StopLossCount.Events = []models.StopLossEvent{}
	}
	if stats.LiquidationCount.Events == nil {
		stats.LiquidationCount.Events = []models.LiquidationEvent{}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(stats)
}

// GetTopPairs возвращает топ-5 пар по указанной метрике.
//
// GET /api/v1/stats/top-pairs?metric=trades|profit|loss&limit=5
//
// Query Parameters:
// - metric (optional): "trades" (default), "profit", или "loss"
// - limit (optional): количество пар (по умолчанию 5, максимум 20)
//
// Response 200 OK:
//
//	[
//	  {"symbol": "BTCUSDT", "value": 50},
//	  {"symbol": "ETHUSDT", "value": 35},
//	  {"symbol": "SOLUSDT", "value": 20},
//	  {"symbol": "XRPUSDT", "value": 15},
//	  {"symbol": "DOGEUSDT", "value": 10}
//	]
//
// Response 400 Bad Request:
//
//	{"error": "invalid metric", "valid_metrics": ["trades", "profit", "loss"]}
//
// Response 500 Internal Server Error:
//
//	{"error": "failed to get top pairs", "details": "..."}
func (h *StatsHandler) GetTopPairs(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Проверяем, что сервис инициализирован
	if h.statsService == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "stats service not initialized",
		})
		return
	}

	// Получаем параметры из query string
	metric := r.URL.Query().Get("metric")
	if metric == "" {
		metric = "trades" // значение по умолчанию
	}

	// Валидация метрики
	validMetrics := map[string]bool{
		"trades": true,
		"profit": true,
		"loss":   true,
	}
	if !validMetrics[metric] {
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]interface{}{
			"error":         "invalid metric",
			"valid_metrics": []string{"trades", "profit", "loss"},
		})
		return
	}

	// Получаем лимит
	limit := 5
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if parsedLimit, err := strconv.Atoi(limitStr); err == nil && parsedLimit > 0 {
			limit = parsedLimit
			if limit > 20 {
				limit = 20 // максимум 20 пар
			}
		}
	}

	// Получаем топ пар через сервис
	topPairs, err := h.statsService.GetTopPairs(metric, limit)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "failed to get top pairs",
			"details": err.Error(),
		})
		return
	}

	// Убеждаемся, что пустой массив возвращается как [], а не null
	if topPairs == nil {
		topPairs = []models.PairStat{}
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(topPairs)
}

// ResetStats сбрасывает счетчики статистики.
//
// POST /api/v1/stats/reset
//
// ВАЖНО: Это действие необратимо! Все данные о сделках будут удалены.
//
// Response 200 OK:
//
//	{"message": "stats reset successfully"}
//
// Response 500 Internal Server Error:
//
//	{"error": "failed to reset stats", "details": "..."}
func (h *StatsHandler) ResetStats(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Проверяем, что сервис инициализирован
	if h.statsService == nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error": "stats service not initialized",
		})
		return
	}

	// Сбрасываем статистику через сервис
	err := h.statsService.ResetStats()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(map[string]string{
			"error":   "failed to reset stats",
			"details": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "stats reset successfully",
	})
}
