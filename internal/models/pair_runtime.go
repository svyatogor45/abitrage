package models

import "time"

// PairRuntime представляет runtime состояние торговой пары
type PairRuntime struct {
	PairID        int        `json:"pair_id"`
	State         string     `json:"state"`                 // PAUSED, READY, ENTERING, HOLDING, EXITING, ERROR
	Legs          []Leg      `json:"legs"`                  // открытые позиции
	FilledParts   int        `json:"filled_parts"`          // сколько частей уже вошло
	CurrentSpread float64    `json:"current_spread"`        // текущий спред %
	UnrealizedPnl float64    `json:"unrealized_pnl"`        // нереализованный PNL
	RealizedPnl   float64    `json:"realized_pnl"`          // реализованный PNL
	EntryTime     *time.Time `json:"entry_time,omitempty"`  // время открытия позиции
	LastUpdate    time.Time  `json:"last_update"`
}

// TotalPnl возвращает общий PNL (реализованный + нереализованный)
func (pr *PairRuntime) TotalPnl() float64 {
	return pr.UnrealizedPnl + pr.RealizedPnl
}

// IsOpen возвращает true если позиция открыта или в процессе открытия/закрытия
func (pr *PairRuntime) IsOpen() bool {
	return pr.State == StateHolding || pr.State == StateEntering || pr.State == StateExiting
}

// Leg представляет одну ногу арбитражной позиции
type Leg struct {
	Exchange           string  `json:"exchange"`
	Side               string  `json:"side"`                             // long, short
	EntryPrice         float64 `json:"entry_price"`
	CurrentPrice       float64 `json:"current_price"`
	Quantity           float64 `json:"quantity"`
	UnrealizedPnl      float64 `json:"unrealized_pnl"`
	ExchangeOrderID    string  `json:"exchange_order_id,omitempty"`      // ID ордера на бирже
	ExchangePositionID string  `json:"exchange_position_id,omitempty"`   // ID позиции на бирже
}

// Состояния пары (state machine)
const (
	StatePaused   = "PAUSED"   // пара на паузе
	StateReady    = "READY"    // мониторинг активен, ожидание условий
	StateEntering = "ENTERING" // процесс входа в позицию
	StateHolding  = "HOLDING"  // позиция открыта, ожидание выхода
	StateExiting  = "EXITING"  // процесс закрытия позиции
	StateError    = "ERROR"    // ошибка, требуется вмешательство
)
