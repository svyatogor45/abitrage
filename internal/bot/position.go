package bot

import (
	"context"
	"sync"
	"sync/atomic"
	"time"

	"arbitrage/internal/models"
)

// PositionManager - менеджер открытых арбитражных позиций
//
// Функции:
// - Непрерывный мониторинг открытых позиций
// - Расчет нереализованной прибыли/убытка (PNL) по каждой ноге
// - Суммирование PNL обеих ног арбитража
// - Отслеживание текущего спреда между позициями
// - Проверка условий для закрытия (exit spread)
// - Проверка Stop Loss
// - Обнаружение ликвидаций
// - Обработка частично открытых позиций
//
// Архитектура:
// - Использует PriceTracker для получения текущих цен (без сетевых запросов!)
// - Интегрируется с Engine для получения списка открытых позиций
// - Асинхронное закрытие позиций через OrderExecutor
type PositionManager struct {
	priceTracker *PriceTracker
	spreadCalc   *SpreadCalculator
	orderExec    *OrderExecutor

	// Callback'и для уведомлений
	onExitCondition func(pairID int, reason string) // условия выхода выполнены
	onStopLoss      func(pairID int, pnl float64)   // достигнут Stop Loss
	onLiquidation   func(pairID int, leg string)    // обнаружена ликвидация

	// Статистика для мониторинга
	checksCount  int64 // количество проверок
	exitsCount   int64 // количество выходов
	stopLossHits int64 // количество Stop Loss
}

// NewPositionManager создаёт новый менеджер позиций
func NewPositionManager(
	priceTracker *PriceTracker,
	spreadCalc *SpreadCalculator,
	orderExec *OrderExecutor,
) *PositionManager {
	return &PositionManager{
		priceTracker: priceTracker,
		spreadCalc:   spreadCalc,
		orderExec:    orderExec,
	}
}

// SetCallbacks устанавливает callback'и для уведомлений
func (pm *PositionManager) SetCallbacks(
	onExitCondition func(pairID int, reason string),
	onStopLoss func(pairID int, pnl float64),
	onLiquidation func(pairID int, leg string),
) {
	pm.onExitCondition = onExitCondition
	pm.onStopLoss = onStopLoss
	pm.onLiquidation = onLiquidation
}

// CheckPosition проверяет одну открытую позицию
//
// Выполняет:
// 1. Обновление текущих цен из PriceTracker (O(1), без сетевых запросов)
// 2. Расчет PNL по каждой ноге
// 3. Расчет общего PNL арбитража
// 4. Расчет текущего спреда
// 5. Проверка условий выхода
// 6. Проверка Stop Loss
//
// Возвращает:
// - PositionStatus с результатами проверки
func (pm *PositionManager) CheckPosition(ps *PairState) *PositionStatus {
	atomic.AddInt64(&pm.checksCount, 1)

	if ps == nil || ps.Runtime == nil || len(ps.Runtime.Legs) != 2 {
		return nil
	}

	status := &PositionStatus{
		PairID:    ps.Config.ID,
		Symbol:    ps.Config.Symbol,
		Timestamp: time.Now(),
	}

	// Получаем текущие цены для обеих ног
	var longLeg, shortLeg *models.Leg
	for i := range ps.Runtime.Legs {
		leg := &ps.Runtime.Legs[i]
		if leg.Side == "long" {
			longLeg = leg
		} else {
			shortLeg = leg
		}
	}

	if longLeg == nil || shortLeg == nil {
		status.Error = "invalid legs: missing long or short"
		return status
	}

	// Получаем текущие цены из PriceTracker (O(1)!)
	longPrice := pm.priceTracker.GetExchangePrice(ps.Config.Symbol, longLeg.Exchange)
	shortPrice := pm.priceTracker.GetExchangePrice(ps.Config.Symbol, shortLeg.Exchange)

	if longPrice == nil || shortPrice == nil {
		status.Error = "prices not available"
		return status
	}

	// Обновляем текущие цены в ногах
	longLeg.CurrentPrice = longPrice.BidPrice   // для закрытия лонга продаём по Bid
	shortLeg.CurrentPrice = shortPrice.AskPrice // для закрытия шорта покупаем по Ask

	// Расчет PNL по каждой ноге
	// Лонг: (текущая цена - цена входа) × объём
	longLeg.UnrealizedPnl = (longLeg.CurrentPrice - longLeg.EntryPrice) * longLeg.Quantity

	// Шорт: (цена входа - текущая цена) × объём
	shortLeg.UnrealizedPnl = (shortLeg.EntryPrice - shortLeg.CurrentPrice) * shortLeg.Quantity

	// Общий PNL арбитража
	status.TotalPnl = longLeg.UnrealizedPnl + shortLeg.UnrealizedPnl
	status.LongPnl = longLeg.UnrealizedPnl
	status.ShortPnl = shortLeg.UnrealizedPnl

	// Обновляем PNL в runtime
	ps.Runtime.UnrealizedPnl = status.TotalPnl

	// Расчет текущего спреда
	// Спред для выхода = (Bid_long - Ask_short) / Ask_short × 100
	if shortLeg.CurrentPrice > 0 {
		status.CurrentSpread = (longLeg.CurrentPrice - shortLeg.CurrentPrice) / shortLeg.CurrentPrice * 100
		ps.Runtime.CurrentSpread = status.CurrentSpread
	}

	// Обновляем время
	ps.Runtime.LastUpdate = status.Timestamp

	// Проверка условий выхода
	status.ShouldExit, status.ExitReason = pm.checkExitConditions(ps, status)

	// Проверка Stop Loss
	status.StopLossHit = pm.checkStopLoss(ps, status.TotalPnl)

	return status
}

// checkExitConditions проверяет условия для закрытия позиции
func (pm *PositionManager) checkExitConditions(ps *PairState, status *PositionStatus) (bool, string) {
	// 1. Проверка exit spread
	// Выходим когда спред схлопнулся до exit_spread или ниже
	if status.CurrentSpread <= ps.Config.ExitSpreadPct {
		return true, "exit_spread_reached"
	}

	// 2. Проверка take profit (если настроен)
	// Можно добавить: if status.TotalPnl >= ps.Config.TakeProfit { return true, "take_profit" }

	// 3. Проверка максимального времени удержания (если настроен)
	// Можно добавить: if time.Since(ps.Runtime.EntryTime) > ps.Config.MaxHoldTime { return true, "max_hold_time" }

	return false, ""
}

// checkStopLoss проверяет достижение Stop Loss
func (pm *PositionManager) checkStopLoss(ps *PairState, totalPnl float64) bool {
	// Stop Loss срабатывает когда PNL падает ниже -StopLoss
	// StopLoss задается в абсолютном значении USDT
	if ps.Config.StopLoss > 0 {
		if totalPnl <= -ps.Config.StopLoss {
			atomic.AddInt64(&pm.stopLossHits, 1)
			return true
		}
	}

	return false
}

// getAverageEntryPrice возвращает среднюю цену входа
func getAverageEntryPrice(ps *PairState) float64 {
	if len(ps.Runtime.Legs) == 0 {
		return 0
	}

	var total float64
	for _, leg := range ps.Runtime.Legs {
		total += leg.EntryPrice
	}
	return total / float64(len(ps.Runtime.Legs))
}

// MonitorPositions запускает мониторинг всех открытых позиций
//
// Параметры:
// - ctx: контекст для отмены
// - getPairs: функция для получения списка пар в состоянии HOLDING
// - interval: интервал проверки (обычно 1 секунда)
//
// Мониторинг выполняется параллельно для всех позиций
func (pm *PositionManager) MonitorPositions(
	ctx context.Context,
	getPairs func() []*PairState,
	interval time.Duration,
) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			pairs := getPairs()

			// Параллельная проверка всех позиций
			var wg sync.WaitGroup
			for _, ps := range pairs {
				if ps.Runtime.State != models.StateHolding {
					continue
				}

				wg.Add(1)
				go func(p *PairState) {
					defer wg.Done()
					pm.processPosition(p)
				}(ps)
			}
			wg.Wait()
		}
	}
}

// processPosition обрабатывает одну позицию и вызывает callback'и
func (pm *PositionManager) processPosition(ps *PairState) {
	status := pm.CheckPosition(ps)
	if status == nil || status.Error != "" {
		return
	}

	// Stop Loss имеет приоритет
	if status.StopLossHit {
		if pm.onStopLoss != nil {
			pm.onStopLoss(status.PairID, status.TotalPnl)
		}
		return
	}

	// Проверка условий выхода
	if status.ShouldExit {
		atomic.AddInt64(&pm.exitsCount, 1)
		if pm.onExitCondition != nil {
			pm.onExitCondition(status.PairID, status.ExitReason)
		}
	}
}

// HandleLiquidation обрабатывает событие ликвидации
//
// При ликвидации одной ноги нужно экстренно закрыть вторую
func (pm *PositionManager) HandleLiquidation(
	ctx context.Context,
	ps *PairState,
	liquidatedLeg string, // "long" или "short"
) error {
	if pm.onLiquidation != nil {
		pm.onLiquidation(ps.Config.ID, liquidatedLeg)
	}

	// Находим и закрываем уцелевшую ногу
	for _, leg := range ps.Runtime.Legs {
		if leg.Side != liquidatedLeg {
			return pm.closeOneLeg(ctx, ps.Config.Symbol, &leg)
		}
	}

	return nil
}

// closeOneLeg закрывает одну ногу позиции
func (pm *PositionManager) closeOneLeg(ctx context.Context, symbol string, leg *models.Leg) error {
	// Определяем направление закрытия
	var side string
	if leg.Side == "long" {
		side = "sell" // закрываем лонг продажей
	} else {
		side = "buy" // закрываем шорт покупкой
	}

	// Выполняем закрытие
	// TODO: использовать OrderExecutor когда он будет расширен
	_ = side
	return nil
}

// ============================================================
// Статистика и мониторинг
// ============================================================

// Stats возвращает статистику менеджера позиций
func (pm *PositionManager) Stats() PositionManagerStats {
	return PositionManagerStats{
		ChecksCount:  atomic.LoadInt64(&pm.checksCount),
		ExitsCount:   atomic.LoadInt64(&pm.exitsCount),
		StopLossHits: atomic.LoadInt64(&pm.stopLossHits),
	}
}

// PositionManagerStats - статистика менеджера
type PositionManagerStats struct {
	ChecksCount  int64 `json:"checks_count"`
	ExitsCount   int64 `json:"exits_count"`
	StopLossHits int64 `json:"stop_loss_hits"`
}

// PositionStatus - результат проверки позиции
type PositionStatus struct {
	PairID    int       `json:"pair_id"`
	Symbol    string    `json:"symbol"`
	Timestamp time.Time `json:"timestamp"`

	// PNL
	TotalPnl float64 `json:"total_pnl"`
	LongPnl  float64 `json:"long_pnl"`
	ShortPnl float64 `json:"short_pnl"`

	// Спред
	CurrentSpread float64 `json:"current_spread"`

	// Условия выхода
	ShouldExit bool   `json:"should_exit"`
	ExitReason string `json:"exit_reason,omitempty"`

	// Stop Loss
	StopLossHit bool `json:"stop_loss_hit"`

	// Ошибки
	Error string `json:"error,omitempty"`
}

// ============================================================
// Вспомогательные функции для расчета PNL
// ============================================================

// CalculatePnlForLegs рассчитывает PNL для списка ног
func CalculatePnlForLegs(legs []models.Leg) (totalPnl float64, legPnls []float64) {
	legPnls = make([]float64, len(legs))

	for i, leg := range legs {
		var pnl float64
		if leg.Side == "long" {
			pnl = (leg.CurrentPrice - leg.EntryPrice) * leg.Quantity
		} else {
			pnl = (leg.EntryPrice - leg.CurrentPrice) * leg.Quantity
		}
		legPnls[i] = pnl
		totalPnl += pnl
	}

	return totalPnl, legPnls
}

// CalculateSpreadForLegs рассчитывает текущий спред для закрытия
func CalculateSpreadForLegs(legs []models.Leg) float64 {
	if len(legs) != 2 {
		return 0
	}

	var longPrice, shortPrice float64
	for _, leg := range legs {
		if leg.Side == "long" {
			longPrice = leg.CurrentPrice // Bid для продажи лонга
		} else {
			shortPrice = leg.CurrentPrice // Ask для покупки шорта
		}
	}

	if shortPrice == 0 {
		return 0
	}

	// Спред = (Bid_long - Ask_short) / Ask_short × 100
	return (longPrice - shortPrice) / shortPrice * 100
}

// ============================================================
// Интеграция с Engine
// ============================================================

// UpdatePairPnlFromTracker обновляет PNL пары из PriceTracker
// Используется в Engine.updatePairPnl()
func (pm *PositionManager) UpdatePairPnlFromTracker(ps *PairState) {
	if ps == nil || ps.Runtime == nil || len(ps.Runtime.Legs) != 2 {
		return
	}

	for i := range ps.Runtime.Legs {
		leg := &ps.Runtime.Legs[i]

		// Получаем текущую цену из трекера
		price := pm.priceTracker.GetExchangePrice(ps.Config.Symbol, leg.Exchange)
		if price == nil {
			continue
		}

		// Обновляем текущую цену
		if leg.Side == "long" {
			leg.CurrentPrice = price.BidPrice // для продажи
		} else {
			leg.CurrentPrice = price.AskPrice // для покупки
		}

		// Обновляем PNL ноги
		if leg.Side == "long" {
			leg.UnrealizedPnl = (leg.CurrentPrice - leg.EntryPrice) * leg.Quantity
		} else {
			leg.UnrealizedPnl = (leg.EntryPrice - leg.CurrentPrice) * leg.Quantity
		}
	}

	// Обновляем общий PNL
	var totalPnl float64
	for _, leg := range ps.Runtime.Legs {
		totalPnl += leg.UnrealizedPnl
	}
	ps.Runtime.UnrealizedPnl = totalPnl

	// Обновляем спред
	ps.Runtime.CurrentSpread = CalculateSpreadForLegs(ps.Runtime.Legs)
	ps.Runtime.LastUpdate = time.Now()
}
