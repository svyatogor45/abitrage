package models

import "time"

// OrderRecord представляет запись об ордере
type OrderRecord struct {
	ID           int       `json:"id" db:"id"`
	PairID       int       `json:"pair_id" db:"pair_id"`
	Exchange     string    `json:"exchange" db:"exchange"`
	Side         string    `json:"side" db:"side"`                           // buy, sell, long, short
	Type         string    `json:"type" db:"type"`                           // market
	PartIndex    int       `json:"part_index" db:"part_index"`               // индекс части (при разбиении)
	Quantity     float64   `json:"quantity" db:"quantity"`
	PriceAvg     float64   `json:"price_avg" db:"price_avg"`                 // средняя цена исполнения
	Status       string    `json:"status" db:"status"`                       // filled, cancelled, rejected
	ErrorMessage string    `json:"error_message,omitempty" db:"error_message"`
	CreatedAt    time.Time `json:"created_at" db:"created_at"`
	FilledAt     *time.Time `json:"filled_at,omitempty" db:"filled_at"`
}

// Статусы ордера
const (
	OrderStatusFilled    = "filled"
	OrderStatusCancelled = "cancelled"
	OrderStatusRejected  = "rejected"
)
