package models

import "time"

// ExchangeAccount представляет биржевой аккаунт с API ключами
type ExchangeAccount struct {
	ID         int       `json:"id" db:"id"`
	Name       string    `json:"name" db:"name"`                     // bybit, bitget, okx, gate, htx, bingx
	APIKey     string    `json:"-" db:"api_key"`                     // зашифрован, не возвращается в JSON
	SecretKey  string    `json:"-" db:"secret_key"`                  // зашифрован
	Passphrase string    `json:"-" db:"passphrase"`                  // для OKX, зашифрован
	Connected  bool      `json:"connected" db:"connected"`
	Balance    float64   `json:"balance" db:"balance"`               // equity в USDT
	LastError  string    `json:"last_error,omitempty" db:"last_error"`
	UpdatedAt  time.Time `json:"updated_at" db:"updated_at"`
	CreatedAt  time.Time `json:"created_at" db:"created_at"`
}
