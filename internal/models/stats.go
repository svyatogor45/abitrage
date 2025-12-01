package models

import "time"

// Stats представляет агрегированную статистику
type Stats struct {
	TotalTrades      int              `json:"total_trades"`
	TotalPnl         float64          `json:"total_pnl"`
	TodayTrades      int              `json:"today_trades"`
	TodayPnl         float64          `json:"today_pnl"`
	WeekTrades       int              `json:"week_trades"`
	WeekPnl          float64          `json:"week_pnl"`
	MonthTrades      int              `json:"month_trades"`
	MonthPnl         float64          `json:"month_pnl"`
	StopLossCount    StopLossStats    `json:"stop_loss_stats"`
	LiquidationCount LiquidationStats `json:"liquidation_stats"`
	TopPairsByTrades []PairStat       `json:"top_pairs_by_trades"` // топ-5
	TopPairsByProfit []PairStat       `json:"top_pairs_by_profit"` // топ-5
	TopPairsByLoss   []PairStat       `json:"top_pairs_by_loss"`   // топ-5
}

// StopLossStats представляет статистику срабатываний Stop Loss
type StopLossStats struct {
	Today  int             `json:"today"`
	Week   int             `json:"week"`
	Month  int             `json:"month"`
	Events []StopLossEvent `json:"events"`
}

// StopLossEvent представляет событие срабатывания SL
type StopLossEvent struct {
	Symbol    string    `json:"symbol"`
	Exchanges [2]string `json:"exchanges"`
	Timestamp time.Time `json:"timestamp"`
}

// LiquidationStats представляет статистику ликвидаций
type LiquidationStats struct {
	Today  int                `json:"today"`
	Week   int                `json:"week"`
	Month  int                `json:"month"`
	Events []LiquidationEvent `json:"events"`
}

// LiquidationEvent представляет событие ликвидации
type LiquidationEvent struct {
	Symbol    string    `json:"symbol"`
	Exchange  string    `json:"exchange"`
	Side      string    `json:"side"`
	Timestamp time.Time `json:"timestamp"`
}

// PairStat представляет статистику по паре
type PairStat struct {
	Symbol string  `json:"symbol"`
	Value  float64 `json:"value"` // количество сделок или PNL
}
