package handlers

import (
	"encoding/json"
	"net/http"
)

// StatsHandler отвечает за получение статистики работы бота
//
// Функции:
// - Получение агрегированной статистики (GET /api/stats)
// - Получение топ-5 пар (GET /api/stats/top-pairs)
// - Сброс счетчиков статистики (POST /api/stats/reset)
//
// Назначение:
// Обрабатывает запросы на получение статистики:
// - Количество завершенных арбитражей (день/неделя/месяц)
// - Общий PNL (день/неделя/месяц)
// - Количество срабатываний Stop Loss
// - Количество ликвидаций
// - Топ-5 пар по разным метрикам (сделки, прибыль, убытки)
// - Агрегирует данные из БД и возвращает в структурированном виде
type StatsHandler struct {
	// TODO: добавить зависимости (service)
}

// NewStatsHandler создает новый StatsHandler
func NewStatsHandler() *StatsHandler {
	return &StatsHandler{}
}

// GetStats возвращает агрегированную статистику
// GET /api/stats
func (h *StatsHandler) GetStats(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Получить статистику через service:
	//    - Количество сделок (сегодня, неделя, месяц)
	//    - PNL (сегодня, неделя, месяц)
	//    - Количество SL (сегодня, неделя, месяц) + детали
	//    - Количество ликвидаций (сегодня, неделя, месяц) + детали
	// 2. Получить топ-5 пар:
	//    - По количеству арбитражей
	//    - По прибыли
	//    - По убыткам
	// 3. Вернуть структурированный объект

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"total_trades": 0,
		"total_pnl":    0,
		"today_trades": 0,
		"today_pnl":    0,
		"week_trades":  0,
		"week_pnl":     0,
		"month_trades": 0,
		"month_pnl":    0,
		"stop_loss_stats": map[string]interface{}{
			"today":  0,
			"week":   0,
			"month":  0,
			"events": []interface{}{},
		},
		"liquidation_stats": map[string]interface{}{
			"today":  0,
			"week":   0,
			"month":  0,
			"events": []interface{}{},
		},
		"top_pairs_by_trades": []interface{}{},
		"top_pairs_by_profit": []interface{}{},
		"top_pairs_by_loss":   []interface{}{},
	})
}

// GetTopPairs возвращает топ-5 пар по разным метрикам
// GET /api/stats/top-pairs?metric=trades|profit|loss
func (h *StatsHandler) GetTopPairs(w http.ResponseWriter, r *http.Request) {
	metric := r.URL.Query().Get("metric") // trades, profit, loss

	// TODO:
	// 1. Получить metric из query параметров
	// 2. В зависимости от metric получить топ-5:
	//    - trades: пары с наибольшим количеством сделок
	//    - profit: пары с наибольшей прибылью
	//    - loss: пары с наибольшими убытками
	// 3. Вернуть массив объектов {symbol, value}

	_ = metric
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]map[string]interface{}{})
}

// ResetStats сбрасывает счетчики статистики
// POST /api/stats/reset
func (h *StatsHandler) ResetStats(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Сбросить счетчики через service
	//    - Обнулить количество сделок
	//    - Обнулить PNL
	//    - Очистить списки SL и ликвидаций
	// 2. Примечание: полная история в trades таблице сохраняется
	// 3. Вернуть статус успеха

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Stats reset (not implemented)",
	})
}
