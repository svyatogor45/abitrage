package models

import "time"

// Settings представляет глобальные настройки бота
type Settings struct {
	ID                  int                     `json:"id" db:"id"`
	ConsiderFunding     bool                    `json:"consider_funding" db:"consider_funding"`           // учитывать фандинг (future)
	MaxConcurrentTrades *int                    `json:"max_concurrent_trades" db:"max_concurrent_trades"` // null = без ограничений
	NotificationPrefs   NotificationPreferences `json:"notification_prefs" db:"notification_prefs"`       // JSON в БД
	UpdatedAt           time.Time               `json:"updated_at" db:"updated_at"`
}

// NotificationPreferences представляет настройки уведомлений
type NotificationPreferences struct {
	Open          bool `json:"open"`
	Close         bool `json:"close"`
	StopLoss      bool `json:"stop_loss"`
	Liquidation   bool `json:"liquidation"`
	APIError      bool `json:"api_error"`
	Margin        bool `json:"margin"`
	Pause         bool `json:"pause"`
	SecondLegFail bool `json:"second_leg_fail"`
}
