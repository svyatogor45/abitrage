package models

import (
	"fmt"
	"time"
)

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

// Validate проверяет корректность параметров пары
func (p *PairConfig) Validate() error {
	if p.Symbol == "" {
		return fmt.Errorf("symbol is required")
	}
	if p.Base == "" {
		return fmt.Errorf("base currency is required")
	}
	if p.Quote == "" {
		return fmt.Errorf("quote currency is required")
	}
	if p.EntrySpreadPct <= 0 {
		return fmt.Errorf("entry_spread must be positive, got %f", p.EntrySpreadPct)
	}
	if p.ExitSpreadPct < 0 {
		return fmt.Errorf("exit_spread cannot be negative, got %f", p.ExitSpreadPct)
	}
	if p.ExitSpreadPct >= p.EntrySpreadPct {
		return fmt.Errorf("exit_spread (%f) must be less than entry_spread (%f)", p.ExitSpreadPct, p.EntrySpreadPct)
	}
	if p.VolumeAsset <= 0 {
		return fmt.Errorf("volume must be positive, got %f", p.VolumeAsset)
	}
	if p.NOrders < 1 {
		return fmt.Errorf("n_orders must be at least 1, got %d", p.NOrders)
	}
	if p.StopLoss < 0 {
		return fmt.Errorf("stop_loss cannot be negative, got %f", p.StopLoss)
	}
	if p.Status != "" && p.Status != PairStatusPaused && p.Status != PairStatusActive {
		return fmt.Errorf("invalid status: %s, must be '%s' or '%s'", p.Status, PairStatusPaused, PairStatusActive)
	}
	return nil
}

// IsActive возвращает true если пара активна
func (p *PairConfig) IsActive() bool {
	return p.Status == PairStatusActive
}
