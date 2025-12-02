package websocket

import (
	"time"

	"arbitrage/internal/models"
)

// MessageType определяет тип WebSocket сообщения
type MessageType string

// Типы WebSocket сообщений
const (
	// MessageTypePairUpdate - обновление состояния торговой пары
	// Отправляется каждую секунду для пар с открытыми позициями
	MessageTypePairUpdate MessageType = "pairUpdate"

	// MessageTypeNotification - новое уведомление
	// Отправляется при событиях: открытие, закрытие, SL, ликвидация, ошибки
	MessageTypeNotification MessageType = "notification"

	// MessageTypeBalanceUpdate - обновление баланса биржи
	// Отправляется каждую минуту для всех подключенных бирж
	MessageTypeBalanceUpdate MessageType = "balanceUpdate"

	// MessageTypeStatsUpdate - обновление статистики торговли
	// Отправляется при изменении статистики (после закрытия сделки)
	MessageTypeStatsUpdate MessageType = "statsUpdate"
)

// BaseMessage - базовая структура для всех WebSocket сообщений
type BaseMessage struct {
	Type      MessageType `json:"type"`
	Timestamp time.Time   `json:"timestamp"`
}

// PairUpdateMessage - сообщение об обновлении состояния пары
//
// Содержит актуальную информацию о торговой паре:
// - Текущий спред между биржами
// - Нереализованный PNL открытых позиций
// - Состояние позиций (ноги арбитража)
// - Текущие цены на биржах
//
// Отправляется каждую секунду для пар в состоянии HOLDING
type PairUpdateMessage struct {
	BaseMessage
	PairID int             `json:"pair_id"`
	Data   *PairUpdateData `json:"data"`
}

// PairUpdateData - данные обновления пары
type PairUpdateData struct {
	// Состояние пары (PAUSED, READY, ENTERING, HOLDING, EXITING, ERROR)
	State string `json:"state"`

	// Текущий спред в процентах
	CurrentSpread float64 `json:"current_spread"`

	// Нереализованный PNL (для открытых позиций)
	UnrealizedPnl float64 `json:"unrealized_pnl"`

	// Реализованный PNL (уже закрытые сделки)
	RealizedPnl float64 `json:"realized_pnl"`

	// Количество заполненных частей (для частичного входа)
	FilledParts int `json:"filled_parts"`

	// Ноги арбитража (позиции на биржах)
	Legs []LegData `json:"legs,omitempty"`

	// Время последнего обновления
	LastUpdate time.Time `json:"last_update"`
}

// LegData - данные одной ноги арбитража
type LegData struct {
	// Название биржи (bybit, okx, etc.)
	Exchange string `json:"exchange"`

	// Направление позиции (long, short)
	Side string `json:"side"`

	// Цена входа
	EntryPrice float64 `json:"entry_price"`

	// Текущая рыночная цена
	CurrentPrice float64 `json:"current_price"`

	// Объем позиции
	Quantity float64 `json:"quantity"`

	// Нереализованный PNL этой ноги
	UnrealizedPnl float64 `json:"unrealized_pnl"`
}

// NotificationMessage - сообщение о новом уведомлении
//
// Содержит информацию о событии:
// - Тип события (OPEN, CLOSE, SL, LIQUIDATION, ERROR, и т.д.)
// - Уровень важности (info, warn, error)
// - Текст сообщения
// - Дополнительные метаданные
type NotificationMessage struct {
	BaseMessage
	Data *NotificationData `json:"data"`
}

// NotificationData - данные уведомления
type NotificationData struct {
	// ID уведомления в БД
	ID int `json:"id"`

	// Тип уведомления (OPEN, CLOSE, SL, LIQUIDATION, ERROR, MARGIN, PAUSE, SECOND_LEG_FAIL)
	Type string `json:"type"`

	// Уровень важности (info, warn, error)
	Severity string `json:"severity"`

	// ID связанной торговой пары (если применимо)
	PairID *int `json:"pair_id,omitempty"`

	// Текст сообщения
	Message string `json:"message"`

	// Дополнительные метаданные (биржи, цены, PNL и т.д.)
	Meta map[string]interface{} `json:"meta,omitempty"`

	// Время создания уведомления
	Timestamp time.Time `json:"timestamp"`
}

// BalanceUpdateMessage - сообщение об обновлении баланса биржи
//
// Отправляется каждую минуту для каждой подключенной биржи
// Позволяет frontend отображать актуальные балансы в реальном времени
type BalanceUpdateMessage struct {
	BaseMessage
	Exchange string  `json:"exchange"`
	Balance  float64 `json:"balance"`
}

// StatsUpdateMessage - сообщение об обновлении статистики
//
// Отправляется после завершения каждой сделки
// Содержит актуальную агрегированную статистику
type StatsUpdateMessage struct {
	BaseMessage
	Data *StatsUpdateData `json:"data"`
}

// StatsUpdateData - данные статистики
type StatsUpdateData struct {
	// Количество сделок по периодам
	TodayTrades int `json:"today_trades"`
	WeekTrades  int `json:"week_trades"`
	MonthTrades int `json:"month_trades"`
	TotalTrades int `json:"total_trades"`

	// PNL по периодам
	TodayPnl float64 `json:"today_pnl"`
	WeekPnl  float64 `json:"week_pnl"`
	MonthPnl float64 `json:"month_pnl"`
	TotalPnl float64 `json:"total_pnl"`

	// Статистика Stop Loss
	StopLossToday  int `json:"stop_loss_today"`
	StopLossWeek   int `json:"stop_loss_week"`
	StopLossMonth  int `json:"stop_loss_month"`

	// Статистика ликвидаций
	LiquidationsToday  int `json:"liquidations_today"`
	LiquidationsWeek   int `json:"liquidations_week"`
	LiquidationsMonth  int `json:"liquidations_month"`
}

// ============ Фабричные функции для создания сообщений ============

// NewPairUpdateMessage создает сообщение обновления пары
func NewPairUpdateMessage(pairID int, runtime *models.PairRuntime) *PairUpdateMessage {
	data := &PairUpdateData{
		State:         runtime.State,
		CurrentSpread: runtime.CurrentSpread,
		UnrealizedPnl: runtime.UnrealizedPnl,
		RealizedPnl:   runtime.RealizedPnl,
		FilledParts:   runtime.FilledParts,
		LastUpdate:    runtime.LastUpdate,
	}

	// Конвертируем ноги
	if len(runtime.Legs) > 0 {
		data.Legs = make([]LegData, len(runtime.Legs))
		for i, leg := range runtime.Legs {
			data.Legs[i] = LegData{
				Exchange:      leg.Exchange,
				Side:          leg.Side,
				EntryPrice:    leg.EntryPrice,
				CurrentPrice:  leg.CurrentPrice,
				Quantity:      leg.Quantity,
				UnrealizedPnl: leg.UnrealizedPnl,
			}
		}
	}

	return &PairUpdateMessage{
		BaseMessage: BaseMessage{
			Type:      MessageTypePairUpdate,
			Timestamp: time.Now(),
		},
		PairID: pairID,
		Data:   data,
	}
}

// NewNotificationMessage создает сообщение уведомления
func NewNotificationMessage(notif *models.Notification) *NotificationMessage {
	return &NotificationMessage{
		BaseMessage: BaseMessage{
			Type:      MessageTypeNotification,
			Timestamp: time.Now(),
		},
		Data: &NotificationData{
			ID:        notif.ID,
			Type:      notif.Type,
			Severity:  notif.Severity,
			PairID:    notif.PairID,
			Message:   notif.Message,
			Meta:      notif.Meta,
			Timestamp: notif.Timestamp,
		},
	}
}

// NewBalanceUpdateMessage создает сообщение обновления баланса
func NewBalanceUpdateMessage(exchange string, balance float64) *BalanceUpdateMessage {
	return &BalanceUpdateMessage{
		BaseMessage: BaseMessage{
			Type:      MessageTypeBalanceUpdate,
			Timestamp: time.Now(),
		},
		Exchange: exchange,
		Balance:  balance,
	}
}

// NewStatsUpdateMessage создает сообщение обновления статистики
func NewStatsUpdateMessage(stats *models.Stats) *StatsUpdateMessage {
	data := &StatsUpdateData{
		TodayTrades: stats.TodayTrades,
		WeekTrades:  stats.WeekTrades,
		MonthTrades: stats.MonthTrades,
		TotalTrades: stats.TotalTrades,

		TodayPnl: stats.TodayPnl,
		WeekPnl:  stats.WeekPnl,
		MonthPnl: stats.MonthPnl,
		TotalPnl: stats.TotalPnl,

		StopLossToday:  stats.StopLossCount.Today,
		StopLossWeek:   stats.StopLossCount.Week,
		StopLossMonth:  stats.StopLossCount.Month,

		LiquidationsToday:  stats.LiquidationCount.Today,
		LiquidationsWeek:   stats.LiquidationCount.Week,
		LiquidationsMonth:  stats.LiquidationCount.Month,
	}

	return &StatsUpdateMessage{
		BaseMessage: BaseMessage{
			Type:      MessageTypeStatsUpdate,
			Timestamp: time.Now(),
		},
		Data: data,
	}
}

// ============ Дополнительные типы для совместимости ============

// AllBalancesUpdateMessage - сообщение с балансами всех бирж
// Используется при начальной загрузке или массовом обновлении
type AllBalancesUpdateMessage struct {
	BaseMessage
	Balances map[string]float64 `json:"balances"`
}

// NewAllBalancesUpdateMessage создает сообщение со всеми балансами
func NewAllBalancesUpdateMessage(balances map[string]float64) *AllBalancesUpdateMessage {
	return &AllBalancesUpdateMessage{
		BaseMessage: BaseMessage{
			Type:      MessageTypeBalanceUpdate,
			Timestamp: time.Now(),
		},
		Balances: balances,
	}
}
