package models

import "time"

// PairConfig представляет конфигурацию торговой пары
type PairConfig struct {
	ID             int       `json:"id" db:"id"`
	Symbol         string    `json:"symbol" db:"symbol"`                       // BTCUSDT
	Base           string    `json:"base" db:"base"`                           // BTC
	Quote          string    `json:"quote" db:"quote"`                         // USDT
	EntrySpreadPct float64   `json:"entry_spread" db:"entry_spread_pct"`       // % для входа
	ExitSpreadPct  float64   `json:"exit_spread" db:"exit_spread_pct"`         // % для выхода
	VolumeAsset    float64   `json:"volume" db:"volume_asset"`                 // объем в монетах
	NOrders        int       `json:"n_orders" db:"n_orders"`                   // количество частей
	StopLoss       float64   `json:"stop_loss" db:"stop_loss"`                 // в USDT
	Status         string    `json:"status" db:"status"`                       // paused, active
	TradesCount    int       `json:"trades_count" db:"trades_count"`           // локальная статистика
	TotalPnl       float64   `json:"total_pnl" db:"total_pnl"`                 // локальная статистика
	CreatedAt      time.Time `json:"created_at" db:"created_at"`
	UpdatedAt      time.Time `json:"updated_at" db:"updated_at"`
}

// Статусы пары
const (
	PairStatusPaused = "paused"
	PairStatusActive = "active"
)
