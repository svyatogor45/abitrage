package models

import "time"

// Notification представляет уведомление о событии
type Notification struct {
	ID        int                    `json:"id" db:"id"`
	Timestamp time.Time              `json:"timestamp" db:"timestamp"`
	Type      string                 `json:"type" db:"type"`               // OPEN, CLOSE, SL, LIQUIDATION, ERROR, MARGIN, PAUSE, SECOND_LEG_FAIL
	Severity  string                 `json:"severity" db:"severity"`       // info, warn, error
	PairID    *int                   `json:"pair_id,omitempty" db:"pair_id"`
	Message   string                 `json:"message" db:"message"`
	Meta      map[string]interface{} `json:"meta,omitempty" db:"meta"`     // дополнительные данные (JSON в БД)
}

// Типы уведомлений
const (
	NotificationTypeOpen          = "OPEN"            // открытие арбитража
	NotificationTypeClose         = "CLOSE"           // закрытие позиций
	NotificationTypeSL            = "SL"              // срабатывание Stop Loss
	NotificationTypeLiquidation   = "LIQUIDATION"     // ликвидация позиции
	NotificationTypeError         = "ERROR"           // ошибка API/ордера
	NotificationTypeMargin        = "MARGIN"          // недостаток маржи
	NotificationTypePause         = "PAUSE"           // пауза/остановка пары
	NotificationTypeSecondLegFail = "SECOND_LEG_FAIL" // не удалось открыть вторую ногу
)

// Уровни важности
const (
	SeverityInfo  = "info"
	SeverityWarn  = "warn"
	SeverityError = "error"
)
