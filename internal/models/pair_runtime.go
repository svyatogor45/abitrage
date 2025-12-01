package models

import "time"

// PairRuntime представляет runtime состояние торговой пары
type PairRuntime struct {
	PairID        int       `json:"pair_id"`
	State         string    `json:"state"`           // PAUSED, READY, ENTERING, HOLDING, EXITING, ERROR
	Legs          []Leg     `json:"legs"`            // открытые позиции
	FilledParts   int       `json:"filled_parts"`    // сколько частей уже вошло
	CurrentSpread float64   `json:"current_spread"`  // текущий спред %
	UnrealizedPnl float64   `json:"unrealized_pnl"`  // нереализованный PNL
	RealizedPnl   float64   `json:"realized_pnl"`    // реализованный PNL
	LastUpdate    time.Time `json:"last_update"`
}

// Leg представляет одну ногу арбитражной позиции
type Leg struct {
	Exchange      string  `json:"exchange"`
	Side          string  `json:"side"`            // long, short
	EntryPrice    float64 `json:"entry_price"`
	CurrentPrice  float64 `json:"current_price"`
	Quantity      float64 `json:"quantity"`
	UnrealizedPnl float64 `json:"unrealized_pnl"`
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
