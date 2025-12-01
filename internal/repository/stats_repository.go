package repository

import (
	"database/sql"
	"strings"
	"time"

	"arbitrage/internal/models"
)

// Trade представляет запись о завершенной сделке (для таблицы trades)
type Trade struct {
	ID             int       `db:"id"`
	PairID         int       `db:"pair_id"`
	Symbol         string    `db:"symbol"`
	Exchanges      string    `db:"exchanges"` // "bybit,okx"
	EntryTime      time.Time `db:"entry_time"`
	ExitTime       time.Time `db:"exit_time"`
	PNL            float64   `db:"pnl"`
	WasStopLoss    bool      `db:"was_stop_loss"`
	WasLiquidation bool      `db:"was_liquidation"`
	CreatedAt      time.Time `db:"created_at"`
}

// StatsRepository - работа со статистикой (таблица trades)
type StatsRepository struct {
	db *sql.DB
}

// NewStatsRepository создает новый экземпляр репозитория
func NewStatsRepository(db *sql.DB) *StatsRepository {
	return &StatsRepository{db: db}
}

// RecordTrade записывает завершенную сделку
func (r *StatsRepository) RecordTrade(pairID int, symbol string, exchanges [2]string, entryTime, exitTime time.Time, pnl float64, wasStopLoss, wasLiquidation bool) error {
	query := `
		INSERT INTO trades (pair_id, symbol, exchanges, entry_time, exit_time, pnl, was_stop_loss, was_liquidation, created_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	exchangesStr := exchanges[0] + "," + exchanges[1]
	_, err := r.db.Exec(query, pairID, symbol, exchangesStr, entryTime, exitTime, pnl, wasStopLoss, wasLiquidation, time.Now())
	return err
}

// GetStats возвращает агрегированную статистику
func (r *StatsRepository) GetStats() (*models.Stats, error) {
	stats := &models.Stats{}
	now := time.Now()
	dayStart := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	weekStart := dayStart.AddDate(0, 0, -int(now.Weekday()))
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())

	// Общая статистика
	var err error
	stats.TotalTrades, stats.TotalPnl, err = r.getTradesStats(time.Time{}, time.Time{})
	if err != nil {
		return nil, err
	}

	// За сегодня
	stats.TodayTrades, stats.TodayPnl, err = r.getTradesStats(dayStart, now)
	if err != nil {
		return nil, err
	}

	// За неделю
	stats.WeekTrades, stats.WeekPnl, err = r.getTradesStats(weekStart, now)
	if err != nil {
		return nil, err
	}

	// За месяц
	stats.MonthTrades, stats.MonthPnl, err = r.getTradesStats(monthStart, now)
	if err != nil {
		return nil, err
	}

	// Stop Loss статистика
	stats.StopLossCount, err = r.getStopLossStats(dayStart, weekStart, monthStart, now)
	if err != nil {
		return nil, err
	}

	// Liquidation статистика
	stats.LiquidationCount, err = r.getLiquidationStats(dayStart, weekStart, monthStart, now)
	if err != nil {
		return nil, err
	}

	// Топ-5 пар
	stats.TopPairsByTrades, err = r.GetTopPairsByTrades(5)
	if err != nil {
		return nil, err
	}

	stats.TopPairsByProfit, err = r.GetTopPairsByProfit(5)
	if err != nil {
		return nil, err
	}

	stats.TopPairsByLoss, err = r.GetTopPairsByLoss(5)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// getTradesStats возвращает количество сделок и PNL за период
func (r *StatsRepository) getTradesStats(from, to time.Time) (int, float64, error) {
	var query string
	var args []interface{}

	if from.IsZero() {
		query = `SELECT COUNT(*), COALESCE(SUM(pnl), 0) FROM trades`
	} else {
		query = `SELECT COUNT(*), COALESCE(SUM(pnl), 0) FROM trades WHERE exit_time >= $1 AND exit_time <= $2`
		args = []interface{}{from, to}
	}

	var count int
	var pnl float64
	err := r.db.QueryRow(query, args...).Scan(&count, &pnl)
	if err != nil {
		return 0, 0, err
	}

	return count, pnl, nil
}

// getStopLossStats возвращает статистику Stop Loss
func (r *StatsRepository) getStopLossStats(dayStart, weekStart, monthStart, now time.Time) (models.StopLossStats, error) {
	stats := models.StopLossStats{}

	// Счетчики
	query := `SELECT COUNT(*) FROM trades WHERE was_stop_loss = true AND exit_time >= $1 AND exit_time <= $2`

	if err := r.db.QueryRow(query, dayStart, now).Scan(&stats.Today); err != nil {
		return stats, err
	}
	if err := r.db.QueryRow(query, weekStart, now).Scan(&stats.Week); err != nil {
		return stats, err
	}
	if err := r.db.QueryRow(query, monthStart, now).Scan(&stats.Month); err != nil {
		return stats, err
	}

	// События (последние 10)
	eventsQuery := `
		SELECT symbol, exchanges, exit_time
		FROM trades
		WHERE was_stop_loss = true
		ORDER BY exit_time DESC
		LIMIT 10`

	rows, err := r.db.Query(eventsQuery)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var event models.StopLossEvent
		var exchangesStr string
		if err := rows.Scan(&event.Symbol, &exchangesStr, &event.Timestamp); err != nil {
			return stats, err
		}
		exchanges := strings.Split(exchangesStr, ",")
		if len(exchanges) >= 2 {
			event.Exchanges = [2]string{exchanges[0], exchanges[1]}
		}
		stats.Events = append(stats.Events, event)
	}

	return stats, rows.Err()
}

// getLiquidationStats возвращает статистику ликвидаций
func (r *StatsRepository) getLiquidationStats(dayStart, weekStart, monthStart, now time.Time) (models.LiquidationStats, error) {
	stats := models.LiquidationStats{}

	// Счетчики
	query := `SELECT COUNT(*) FROM trades WHERE was_liquidation = true AND exit_time >= $1 AND exit_time <= $2`

	if err := r.db.QueryRow(query, dayStart, now).Scan(&stats.Today); err != nil {
		return stats, err
	}
	if err := r.db.QueryRow(query, weekStart, now).Scan(&stats.Week); err != nil {
		return stats, err
	}
	if err := r.db.QueryRow(query, monthStart, now).Scan(&stats.Month); err != nil {
		return stats, err
	}

	// События (последние 10)
	eventsQuery := `
		SELECT symbol, exchanges, exit_time
		FROM trades
		WHERE was_liquidation = true
		ORDER BY exit_time DESC
		LIMIT 10`

	rows, err := r.db.Query(eventsQuery)
	if err != nil {
		return stats, err
	}
	defer rows.Close()

	for rows.Next() {
		var event models.LiquidationEvent
		var exchangesStr string
		if err := rows.Scan(&event.Symbol, &exchangesStr, &event.Timestamp); err != nil {
			return stats, err
		}
		exchanges := strings.Split(exchangesStr, ",")
		if len(exchanges) > 0 {
			event.Exchange = exchanges[0]
		}
		stats.Events = append(stats.Events, event)
	}

	return stats, rows.Err()
}

// GetTopPairsByTrades возвращает топ пар по количеству сделок
func (r *StatsRepository) GetTopPairsByTrades(limit int) ([]models.PairStat, error) {
	query := `
		SELECT symbol, COUNT(*) as trade_count
		FROM trades
		GROUP BY symbol
		ORDER BY trade_count DESC
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.PairStat
	for rows.Next() {
		var stat models.PairStat
		if err := rows.Scan(&stat.Symbol, &stat.Value); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	return stats, rows.Err()
}

// GetTopPairsByProfit возвращает топ пар по прибыли
func (r *StatsRepository) GetTopPairsByProfit(limit int) ([]models.PairStat, error) {
	query := `
		SELECT symbol, SUM(pnl) as total_pnl
		FROM trades
		GROUP BY symbol
		HAVING SUM(pnl) > 0
		ORDER BY total_pnl DESC
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.PairStat
	for rows.Next() {
		var stat models.PairStat
		if err := rows.Scan(&stat.Symbol, &stat.Value); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	return stats, rows.Err()
}

// GetTopPairsByLoss возвращает топ пар по убыткам
func (r *StatsRepository) GetTopPairsByLoss(limit int) ([]models.PairStat, error) {
	query := `
		SELECT symbol, SUM(pnl) as total_pnl
		FROM trades
		GROUP BY symbol
		HAVING SUM(pnl) < 0
		ORDER BY total_pnl ASC
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []models.PairStat
	for rows.Next() {
		var stat models.PairStat
		if err := rows.Scan(&stat.Symbol, &stat.Value); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	return stats, rows.Err()
}

// ResetCounters сбрасывает счетчики (удаляет все записи)
func (r *StatsRepository) ResetCounters() error {
	query := `DELETE FROM trades`
	_, err := r.db.Exec(query)
	return err
}

// DeleteOlderThan удаляет записи старше указанной даты
func (r *StatsRepository) DeleteOlderThan(timestamp time.Time) (int64, error) {
	query := `DELETE FROM trades WHERE exit_time < $1`
	result, err := r.db.Exec(query, timestamp)
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// GetTradesByPairID возвращает сделки для конкретной пары
func (r *StatsRepository) GetTradesByPairID(pairID int, limit int) ([]*Trade, error) {
	query := `
		SELECT id, pair_id, symbol, exchanges, entry_time, exit_time, pnl, was_stop_loss, was_liquidation, created_at
		FROM trades
		WHERE pair_id = $1
		ORDER BY exit_time DESC
		LIMIT $2`

	rows, err := r.db.Query(query, pairID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		trade := &Trade{}
		if err := rows.Scan(
			&trade.ID,
			&trade.PairID,
			&trade.Symbol,
			&trade.Exchanges,
			&trade.EntryTime,
			&trade.ExitTime,
			&trade.PNL,
			&trade.WasStopLoss,
			&trade.WasLiquidation,
			&trade.CreatedAt,
		); err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}

	return trades, rows.Err()
}

// GetTradesInTimeRange возвращает сделки за период
func (r *StatsRepository) GetTradesInTimeRange(from, to time.Time, limit int) ([]*Trade, error) {
	query := `
		SELECT id, pair_id, symbol, exchanges, entry_time, exit_time, pnl, was_stop_loss, was_liquidation, created_at
		FROM trades
		WHERE exit_time >= $1 AND exit_time <= $2
		ORDER BY exit_time DESC
		LIMIT $3`

	rows, err := r.db.Query(query, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trades []*Trade
	for rows.Next() {
		trade := &Trade{}
		if err := rows.Scan(
			&trade.ID,
			&trade.PairID,
			&trade.Symbol,
			&trade.Exchanges,
			&trade.EntryTime,
			&trade.ExitTime,
			&trade.PNL,
			&trade.WasStopLoss,
			&trade.WasLiquidation,
			&trade.CreatedAt,
		); err != nil {
			return nil, err
		}
		trades = append(trades, trade)
	}

	return trades, rows.Err()
}

// Count возвращает общее количество сделок
func (r *StatsRepository) Count() (int, error) {
	query := `SELECT COUNT(*) FROM trades`

	var count int
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetPNLBySymbol возвращает суммарный PNL по символу
func (r *StatsRepository) GetPNLBySymbol(symbol string) (float64, error) {
	query := `SELECT COALESCE(SUM(pnl), 0) FROM trades WHERE symbol = $1`

	var pnl float64
	err := r.db.QueryRow(query, symbol).Scan(&pnl)
	if err != nil {
		return 0, err
	}

	return pnl, nil
}
