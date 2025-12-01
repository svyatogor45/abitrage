package models

import "time"

// BlacklistEntry представляет запись в черном списке торговых пар
type BlacklistEntry struct {
	ID        int       `json:"id" db:"id"`
	Symbol    string    `json:"symbol" db:"symbol"`       // BTCUSDT
	Reason    string    `json:"reason" db:"reason"`       // пользовательская заметка
	CreatedAt time.Time `json:"created_at" db:"created_at"`
}
