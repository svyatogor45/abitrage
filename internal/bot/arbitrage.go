package bot

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"arbitrage/internal/models"
)

// ============ ОПТИМИЗАЦИЯ: sync.Pool для EntryConditions ============
// EntryConditions создаётся при каждой проверке арбитражных условий (100+/сек)
// Пул уменьшает нагрузку на GC

var entryConditionsPool = sync.Pool{
	New: func() interface{} {
		return &EntryConditions{
			Warnings: make([]string, 0, 4), // Предаллокация на 4 предупреждения
		}
	},
}

// acquireEntryConditions получает объект из пула
func acquireEntryConditions() *EntryConditions {
	ec := entryConditionsPool.Get().(*EntryConditions)
	// Сбрасываем поля (Warnings уже есть, просто обрезаем)
	ec.CanEnter = false
	ec.Reason = ""
	ec.SpreadOK = false
	ec.LiquidityOK = false
	ec.MarginOK = false
	ec.LimitsOK = false
	ec.MaxArbitragesOK = false
	ec.Opportunity = nil
	ec.AdjustedVolume = 0
	ec.Warnings = ec.Warnings[:0]
	return ec
}

// ReleaseEntryConditions возвращает объект в пул
// ВАЖНО: вызывать только когда объект больше не используется!
// НЕ вызывать если CanEnter=true (объект будет использоваться в executeEntry)
func ReleaseEntryConditions(ec *EntryConditions) {
	if ec == nil {
		return
	}
	// Очищаем ссылки для GC
	ec.Opportunity = nil
	ec.Reason = ""
	ec.Warnings = ec.Warnings[:0]
	entryConditionsPool.Put(ec)
}

// ============================================================
// ArbitrageDetector - определение арбитражных возможностей
// ============================================================
//
// Согласно ТЗ и Архитектуре:
// - Определение арбитражных возможностей (какие биржи, какой спред)
// - Выбор оптимальной пары бирж для арбитража (min price / max price)
// - Проверка условий входа: net_spread >= entry_spread
// - Проверка условий выхода: spread <= exit_spread
// - Логика частичного входа (разбиение на N ордеров)
// - Координация одновременного открытия/закрытия позиций
// - Обработка кейса "вторая нога не открылась"

// ArbitrageDetector определяет и анализирует арбитражные возможности
//
// ОПТИМИЗАЦИИ (согласно требованиям производительности):
// - Использует предвычисленные лучшие цены из PriceTracker (O(1))
// - Кэширует комиссии бирж (без сетевых запросов в горячем пути)
// - Lock-free чтение через atomic и sync.Map где возможно
type ArbitrageDetector struct {
	priceTracker      *PriceTracker
	spreadCalc        *SpreadCalculator
	orderBookAnalyzer *OrderBookAnalyzer
	balanceFetcher    func(ctx context.Context, exchange string) (float64, error)

	// Кэш маржинальных требований (обновляется периодически)
	marginCache sync.Map // exchange -> float64 (available margin)

	// Счётчики для мониторинга
	opportunitiesDetected int64
	entriesTriggered      int64
	exitsTriggered        int64
}

// NewArbitrageDetector создаёт детектор арбитражных возможностей
func NewArbitrageDetector(
	priceTracker *PriceTracker,
	spreadCalc *SpreadCalculator,
	orderBookAnalyzer *OrderBookAnalyzer,
	balanceFetcher func(ctx context.Context, exchange string) (float64, error),
) *ArbitrageDetector {
	return &ArbitrageDetector{
		priceTracker:      priceTracker,
		spreadCalc:        spreadCalc,
		orderBookAnalyzer: orderBookAnalyzer,
		balanceFetcher:    balanceFetcher,
	}
}

// DetectOpportunity находит лучшую арбитражную возможность для символа
//
// Возвращает:
// - ArbitrageOpportunity если найдена возможность
// - nil если нет подходящей возможности
//
// Сложность: O(1) благодаря предвычисленным данным в PriceTracker
func (ad *ArbitrageDetector) DetectOpportunity(symbol string) *ArbitrageOpportunity {
	opp := ad.spreadCalc.GetBestOpportunity(symbol)
	if opp != nil {
		atomic.AddInt64(&ad.opportunitiesDetected, 1)
	}
	return opp
}

// DetectWithLiquidity находит возможность с проверкой ликвидности
//
// Использует OrderBookAnalyzer для:
// - Проверки достаточности ликвидности на обеих биржах
// - Расчёта реального спреда с учётом slippage (VWAP)
// - Оценки фактической прибыли
func (ad *ArbitrageDetector) DetectWithLiquidity(symbol string, volume float64) *SpreadWithLiquidity {
	if ad.orderBookAnalyzer == nil {
		// Без анализатора стаканов возвращаем базовую возможность
		opp := ad.spreadCalc.GetBestOpportunity(symbol)
		if opp == nil {
			return nil
		}
		return &SpreadWithLiquidity{
			ArbitrageOpportunity: opp,
			IsLiquidityOK:        true, // предполагаем достаточную ликвидность
		}
	}

	return ad.spreadCalc.GetSpreadWithLiquidity(symbol, volume, ad.orderBookAnalyzer)
}

// ============================================================
// EntryConditionChecker - проверка условий входа
// ============================================================

// EntryConditions содержит результат проверки условий входа
type EntryConditions struct {
	// Основной результат
	CanEnter bool
	Reason   string // причина если нельзя войти

	// Детали проверки
	SpreadOK        bool
	LiquidityOK     bool
	MarginOK        bool
	LimitsOK        bool
	MaxArbitragesOK bool

	// Данные возможности
	Opportunity *ArbitrageOpportunity

	// Скорректированный объём (после валидации лимитов)
	AdjustedVolume float64

	// Предупреждения (не блокирующие)
	Warnings []string
}

// CheckEntryConditions выполняет полную проверку условий для входа в арбитраж
//
// Согласно ТЗ проверяет:
// 1. Спред >= entry_spread (с учётом комиссий)
// 2. Достаточная ликвидность на обеих биржах
// 3. Достаточная маржа для открытия позиций
// 4. Соблюдение лимитов бирж (min/max qty, lot size)
// 5. Лимит максимальных одновременных арбитражей
//
// ОПТИМИЗАЦИЯ: использует sync.Pool для EntryConditions
// Вызывающий код НЕ должен вызывать ReleaseEntryConditions если CanEnter=true
func (ad *ArbitrageDetector) CheckEntryConditions(
	ps *PairState,
	currentArbitrages int64,
	maxArbitrages int,
	validator *OrderValidator,
) *EntryConditions {
	// ОПТИМИЗАЦИЯ: получаем из пула
	result := acquireEntryConditions()

	config := ps.Config
	symbol := config.Symbol
	volume := config.VolumeAsset

	// 1. Проверка лимита одновременных арбитражей
	if maxArbitrages > 0 && currentArbitrages >= int64(maxArbitrages) {
		result.Reason = "max concurrent arbitrages reached"
		return result
	}
	result.MaxArbitragesOK = true

	// 2. Получаем арбитражную возможность
	var opp *ArbitrageOpportunity
	var liquidityOK bool = true
	var liquidityIssue string

	if ad.orderBookAnalyzer != nil {
		// С анализом ликвидности (более точно)
		spreadWithLiq := ad.DetectWithLiquidity(symbol, volume)
		if spreadWithLiq == nil {
			result.Reason = "no arbitrage opportunity found"
			return result
		}
		opp = spreadWithLiq.ArbitrageOpportunity
		liquidityOK = spreadWithLiq.IsLiquidityOK
		liquidityIssue = spreadWithLiq.LiquidityIssue

		if spreadWithLiq.TotalSlippage > 0.05 {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("high slippage: %.4f%%", spreadWithLiq.TotalSlippage))
		}
	} else {
		// Базовая проверка без стаканов
		opp = ad.DetectOpportunity(symbol)
		if opp == nil {
			result.Reason = "no arbitrage opportunity found"
			return result
		}
	}

	result.Opportunity = opp

	// 3. Проверка ликвидности
	if !liquidityOK {
		result.Reason = "insufficient liquidity: " + liquidityIssue
		ReleaseArbitrageOpportunity(opp) // Освобождаем opp
		result.Opportunity = nil
		return result
	}
	result.LiquidityOK = true

	// 4. Проверка спреда
	// ОПТИМИЗАЦИЯ: используем atomic read для lock-free доступа в горячем пути
	entrySpread := ps.GetEntrySpread()
	if opp.NetSpread < entrySpread {
		result.Reason = fmt.Sprintf("spread %.4f%% < entry threshold %.4f%%",
			opp.NetSpread, entrySpread)
		ReleaseArbitrageOpportunity(opp) // Освобождаем opp
		result.Opportunity = nil
		return result
	}
	result.SpreadOK = true

	// 5. Проверка лимитов ордеров (если есть валидатор)
	adjustedVolume := volume
	if validator != nil {
		validation := validator.ValidateBothLegs(
			opp.LongExchange,
			opp.ShortExchange,
			symbol,
			volume,
			opp.LongPrice,
			opp.ShortPrice,
		)

		if !validation.Valid {
			result.Reason = "order validation failed: " + validation.Error
			ReleaseArbitrageOpportunity(opp) // Освобождаем opp
			result.Opportunity = nil
			return result
		}

		adjustedVolume = validation.AdjustedQty
		result.Warnings = append(result.Warnings, validation.Warnings...)
	}
	result.LimitsOK = true
	result.AdjustedVolume = adjustedVolume

	// 6. Проверка маржи
	marginOK, marginReason := ad.checkMarginRequirement(
		opp.LongExchange, opp.ShortExchange, symbol, adjustedVolume, opp.LongPrice)

	if !marginOK {
		result.Reason = marginReason
		ReleaseArbitrageOpportunity(opp) // Освобождаем opp
		result.Opportunity = nil
		return result
	}
	result.MarginOK = true

	// Все условия выполнены!
	result.CanEnter = true
	atomic.AddInt64(&ad.entriesTriggered, 1)

	// НЕ освобождаем opp - он будет использован в executeEntry
	// НЕ освобождаем result - он будет использован в executeEntry
	return result
}

// checkMarginRequirement проверяет достаточность маржи
func (ad *ArbitrageDetector) checkMarginRequirement(
	longExch, shortExch, symbol string,
	volume, price float64,
) (bool, string) {
	// Примерный расчёт требуемой маржи
	// Для фьючерсов с плечом 10x: margin = notional / 10
	notional := volume * price
	requiredMargin := notional / 10 * 2 // на обеих биржах

	for _, exch := range []string{longExch, shortExch} {
		// Сначала проверяем кэш
		if margin, ok := ad.marginCache.Load(exch); ok {
			if margin.(float64) < requiredMargin/2 {
				return false, fmt.Sprintf("insufficient margin on %s: need %.2f USDT", exch, requiredMargin/2)
			}
			continue
		}

		// При отсутствии кэша — прямой запрос баланса (короткий тайм-аут)
		if ad.balanceFetcher == nil {
			return false, fmt.Sprintf("margin data unavailable for %s", exch)
		}

		ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
		available, err := ad.balanceFetcher(ctx, exch)
		cancel()
		if err != nil {
			return false, fmt.Sprintf("failed to fetch margin for %s: %v", exch, err)
		}

		ad.marginCache.Store(exch, available)
		if available < requiredMargin/2 {
			return false, fmt.Sprintf("insufficient margin on %s: need %.2f USDT", exch, requiredMargin/2)
		}
	}

	return true, ""
}

// UpdateMarginCache обновляет кэш маржи для биржи
func (ad *ArbitrageDetector) UpdateMarginCache(exchange string, availableMargin float64) {
	ad.marginCache.Store(exchange, availableMargin)
}

// ============================================================
// ExitConditionChecker - проверка условий выхода
// ============================================================

// ExitConditions содержит результат проверки условий выхода
type ExitConditions struct {
	ShouldExit bool
	Reason     ExitReason

	// Текущие показатели
	CurrentSpread float64
	CurrentPnl    float64
}

// ExitReason причина выхода из позиции
type ExitReason string

const (
	ExitReasonNone        ExitReason = ""
	ExitReasonSpread      ExitReason = "spread_reached" // спред достиг порога выхода
	ExitReasonStopLoss    ExitReason = "stop_loss"      // достигнут stop loss
	ExitReasonLiquidation ExitReason = "liquidation"    // ликвидация позиции
	ExitReasonManual      ExitReason = "manual"         // ручное закрытие
	ExitReasonError       ExitReason = "error"          // ошибка
)

// CheckExitConditions проверяет условия для выхода из позиции
//
// Согласно ТЗ проверяет:
// 1. Спред <= exit_spread
// 2. PNL <= -StopLoss
// 3. Ликвидация одной из ног
func (ad *ArbitrageDetector) CheckExitConditions(ps *PairState) *ExitConditions {
	result := &ExitConditions{
		ShouldExit: false,
		Reason:     ExitReasonNone,
	}

	config := ps.Config
	runtime := ps.Runtime

	// Защита от nil runtime
	if runtime == nil {
		return result
	}

	// Проверяем только если есть открытая позиция
	if runtime.State != models.StateHolding || len(runtime.Legs) != 2 {
		return result
	}

	var longLeg, shortLeg *models.Leg
	for i := range runtime.Legs {
		if runtime.Legs[i].Side == "long" {
			longLeg = &runtime.Legs[i]
		} else {
			shortLeg = &runtime.Legs[i]
		}
	}

	if longLeg == nil || shortLeg == nil {
		return result
	}

	// 1. Рассчитываем текущий спред для выхода
	// Для выхода: продаём лонг по Bid, покупаем шорт по Ask
	currentSpread := ad.spreadCalc.GetCurrentSpread(
		config.Symbol,
		longLeg.Exchange,
		shortLeg.Exchange,
	)
	result.CurrentSpread = currentSpread

	// 2. Рассчитываем текущий PNL
	currentPnl := ad.spreadCalc.CalculatePnl(
		config.Symbol,
		longLeg.Exchange, longLeg.EntryPrice,
		shortLeg.Exchange, shortLeg.EntryPrice,
		longLeg.Quantity,
	)
	result.CurrentPnl = currentPnl

	// 3. Проверяем Stop Loss
	// ОПТИМИЗАЦИЯ: используем atomic read для lock-free доступа
	stopLoss := ps.GetStopLoss()
	if stopLoss > 0 && currentPnl <= -stopLoss {
		result.ShouldExit = true
		result.Reason = ExitReasonStopLoss
		atomic.AddInt64(&ad.exitsTriggered, 1)
		return result
	}

	// 4. Проверяем достижение спреда выхода
	// ОПТИМИЗАЦИЯ: используем atomic read для lock-free доступа
	exitSpread := ps.GetExitSpread()
	if currentSpread <= exitSpread {
		result.ShouldExit = true
		result.Reason = ExitReasonSpread
		atomic.AddInt64(&ad.exitsTriggered, 1)
		return result
	}

	return result
}

// ============================================================
// PartialEntryManager - логика частичного входа
// ============================================================

// PartialEntryManager управляет частичным входом в позицию (N ордеров)
//
// Согласно ТЗ:
// - Разбиение большого объёма на N меньших ордеров
// - Последовательный вход с проверкой спреда перед каждой частью
// - Возможность остановки при ухудшении спреда
type PartialEntryManager struct {
	detector  *ArbitrageDetector
	orderExec *OrderExecutor
	validator *OrderValidator
}

// PartialEntryParams параметры частичного входа
type PartialEntryParams struct {
	Symbol        string
	TotalVolume   float64
	NOrders       int
	LongExchange  string
	ShortExchange string
	MinSpread     float64 // минимальный спред для продолжения
}

// PartialEntryResult результат частичного входа
type PartialEntryResult struct {
	Success       bool
	FilledParts   int
	TotalFilled   float64
	Legs          []models.Leg
	Error         error
	PartialErrors []error // ошибки отдельных частей
}

// NewPartialEntryManager создаёт менеджер частичного входа
func NewPartialEntryManager(
	detector *ArbitrageDetector,
	orderExec *OrderExecutor,
	validator *OrderValidator,
) *PartialEntryManager {
	return &PartialEntryManager{
		detector:  detector,
		orderExec: orderExec,
		validator: validator,
	}
}

// ExecutePartialEntry выполняет частичный вход в позицию
//
// Алгоритм:
// 1. Разбиваем объём на N частей
// 2. Для каждой части:
//   - Проверяем текущий спред (должен оставаться >= minSpread)
//   - Отправляем ордера параллельно на обе биржи
//   - Аккумулируем результаты
//
// 3. Если спред ухудшился - останавливаемся (частичная позиция)
func (pem *PartialEntryManager) ExecutePartialEntry(
	ctx context.Context,
	params PartialEntryParams,
) *PartialEntryResult {
	result := &PartialEntryResult{
		Success:       false,
		PartialErrors: make([]error, 0),
	}

	if params.NOrders <= 0 {
		params.NOrders = 1
	}

	partVolume := params.TotalVolume / float64(params.NOrders)

	// Аккумуляторы для ног
	var totalLongQty, totalShortQty float64
	var avgLongPrice, avgShortPrice float64
	var longPriceSum, shortPriceSum float64

	for i := 0; i < params.NOrders; i++ {
		// Проверяем контекст
		select {
		case <-ctx.Done():
			result.Error = ctx.Err()
			return result
		default:
		}

		// Проверяем текущий спред (кроме первой части)
		if i > 0 {
			opp := pem.detector.DetectOpportunity(params.Symbol)
			if opp == nil || opp.NetSpread < params.MinSpread {
				// Спред ухудшился - останавливаем вход
				result.Success = result.FilledParts > 0
				if opp != nil {
					result.Error = fmt.Errorf("spread degraded to %.4f%%, stopping at part %d/%d",
						opp.NetSpread, i, params.NOrders)
					ReleaseArbitrageOpportunity(opp) // Освобождаем перед выходом
				}
				break
			}
			// ВАЖНО: освобождаем opp после проверки - он больше не нужен
			ReleaseArbitrageOpportunity(opp)
		}

		// Выполняем часть входа
		execParams := ExecuteParams{
			Symbol:        params.Symbol,
			Volume:        partVolume,
			LongExchange:  params.LongExchange,
			ShortExchange: params.ShortExchange,
			NOrders:       1, // уже разбили
		}

		partResult := pem.orderExec.ExecuteParallel(ctx, execParams)

		if !partResult.Success {
			result.PartialErrors = append(result.PartialErrors, partResult.Error)
			// Продолжаем с остальными частями (можно изменить на остановку)
			continue
		}

		// Аккумулируем результаты
		result.FilledParts++

		if partResult.LongOrder != nil {
			totalLongQty += partResult.LongOrder.FilledQty
			longPriceSum += partResult.LongOrder.AvgFillPrice * partResult.LongOrder.FilledQty
		}
		if partResult.ShortOrder != nil {
			totalShortQty += partResult.ShortOrder.FilledQty
			shortPriceSum += partResult.ShortOrder.AvgFillPrice * partResult.ShortOrder.FilledQty
		}
	}

	// Формируем итоговые ноги
	if totalLongQty > 0 {
		avgLongPrice = longPriceSum / totalLongQty
	}
	if totalShortQty > 0 {
		avgShortPrice = shortPriceSum / totalShortQty
	}

	if result.FilledParts > 0 {
		result.Success = true
		result.TotalFilled = totalLongQty // или min(longQty, shortQty)
		result.Legs = []models.Leg{
			{
				Exchange:   params.LongExchange,
				Side:       "long",
				EntryPrice: avgLongPrice,
				Quantity:   totalLongQty,
			},
			{
				Exchange:   params.ShortExchange,
				Side:       "short",
				EntryPrice: avgShortPrice,
				Quantity:   totalShortQty,
			},
		}
	}

	return result
}

// ============================================================
// SecondLegFailHandler - обработка "вторая нога не открылась"
// ============================================================

// SecondLegFailHandler обрабатывает ситуацию когда одна нога открылась, а вторая нет
//
// Согласно ТЗ:
// - При успехе первой ноги и провале второй - откат первой
// - Уведомление пользователя о событии
// - Постановка пары на паузу (опционально)
type SecondLegFailHandler struct {
	orderExec   *OrderExecutor
	notifyChan  chan<- *models.Notification
	pauseOnFail bool
}

// SecondLegFailEvent событие провала второй ноги
type SecondLegFailEvent struct {
	PairID          int
	Symbol          string
	SuccessfulLeg   string // "long" или "short"
	SuccessExchange string
	SuccessOrder    interface{} // *exchange.Order
	FailedLeg       string
	FailExchange    string
	FailError       error
	RollbackResult  *RollbackResult
}

// RollbackResult результат отката первой ноги
type RollbackResult struct {
	Success bool
	Error   error
	PnlLoss float64 // убыток от отката (slippage)
}

// NewSecondLegFailHandler создаёт обработчик провала второй ноги
func NewSecondLegFailHandler(
	orderExec *OrderExecutor,
	notifyChan chan<- *models.Notification,
	pauseOnFail bool,
) *SecondLegFailHandler {
	return &SecondLegFailHandler{
		orderExec:   orderExec,
		notifyChan:  notifyChan,
		pauseOnFail: pauseOnFail,
	}
}

// Handle обрабатывает провал второй ноги
//
// Алгоритм:
// 1. Откатываем успешную первую ногу
// 2. Создаём уведомление SECOND_LEG_FAIL
// 3. Возвращаем событие для логирования
func (h *SecondLegFailHandler) Handle(
	ctx context.Context,
	pairID int,
	symbol string,
	successLeg string, // "long" или "short"
	successExchange string,
	successQty float64,
	failLeg string,
	failExchange string,
	failError error,
) *SecondLegFailEvent {
	event := &SecondLegFailEvent{
		PairID:          pairID,
		Symbol:          symbol,
		SuccessfulLeg:   successLeg,
		SuccessExchange: successExchange,
		FailedLeg:       failLeg,
		FailExchange:    failExchange,
		FailError:       failError,
	}

	// Откатываем успешную ногу
	rollbackResult := h.rollback(ctx, symbol, successLeg, successExchange, successQty)
	event.RollbackResult = rollbackResult

	// Создаём уведомление
	if h.notifyChan != nil {
		// Формируем сообщение в зависимости от результата отката
		var message string
		var severity string
		if rollbackResult.Success {
			severity = "warning"
			message = fmt.Sprintf(
				"Second leg (%s on %s) failed: %v. First leg (%s on %s) successfully rolled back.",
				failLeg, failExchange, failError,
				successLeg, successExchange,
			)
		} else {
			// КРИТИЧНО: откат не удался - есть открытая позиция без хеджа!
			severity = "critical"
			message = fmt.Sprintf(
				"CRITICAL: Second leg (%s on %s) failed: %v. ROLLBACK FAILED for first leg (%s on %s): %v. Manual intervention required!",
				failLeg, failExchange, failError,
				successLeg, successExchange, rollbackResult.Error,
			)
		}

		notif := &models.Notification{
			Timestamp: time.Now(),
			Type:      "SECOND_LEG_FAIL",
			Severity:  severity,
			PairID:    &pairID,
			Message:   message,
			Meta: map[string]interface{}{
				"symbol":           symbol,
				"success_leg":      successLeg,
				"success_exchange": successExchange,
				"fail_leg":         failLeg,
				"fail_exchange":    failExchange,
				"rollback_success": rollbackResult.Success,
				"rollback_pnl":     rollbackResult.PnlLoss,
				"rollback_error":   fmt.Sprintf("%v", rollbackResult.Error),
			},
		}

		select {
		case h.notifyChan <- notif:
		default:
			// Канал заполнен, пропускаем
		}
	}

	return event
}

// rollback откатывает успешную ногу
// ВАЖНО: правильно обрабатывает результат закрытия!
func (h *SecondLegFailHandler) rollback(
	ctx context.Context,
	symbol string,
	leg string, // "long" или "short"
	exchange string,
	qty float64,
) *RollbackResult {
	result := &RollbackResult{
		Success: false,
	}

	// Формируем единственную ногу для закрытия
	// Side указывает направление ПОЗИЦИИ (long/short), не действия
	legToClose := models.Leg{
		Exchange: exchange,
		Side:     leg,
		Quantity: qty,
	}

	// Закрываем через OrderExecutor с агрессивным таймаутом
	closeCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	closeResult := h.orderExec.CloseParallel(closeCtx, CloseParams{
		Symbol: symbol,
		Legs:   []models.Leg{legToClose},
	})

	// КРИТИЧНО: проверяем результат закрытия!
	if closeResult == nil {
		result.Error = fmt.Errorf("rollback failed: CloseParallel returned nil")
		return result
	}

	if !closeResult.Success {
		result.Error = fmt.Errorf("rollback failed: %v", closeResult.Error)
		return result
	}

	result.Success = true

	// Рассчитываем убыток от slippage если есть данные
	if closeResult.LongOrder != nil && closeResult.LongOrder.AvgFillPrice > 0 {
		// TODO: добавить расчёт PnL при наличии entry price
		result.PnlLoss = 0
	}

	return result
}

// ============================================================
// ArbitrageCoordinator - координация арбитражных операций
// ============================================================

// ArbitrageCoordinator координирует весь процесс арбитража
//
// Объединяет:
// - Детекцию возможностей
// - Проверку условий
// - Исполнение входа/выхода
// - Обработку ошибок
type ArbitrageCoordinator struct {
	detector       *ArbitrageDetector
	orderExec      *OrderExecutor
	validator      *OrderValidator
	partialManager *PartialEntryManager
	failHandler    *SecondLegFailHandler

	// Конфигурация
	maxArbitrages int
	activeArbs    *int64 // указатель на atomic counter в Engine
}

// NewArbitrageCoordinator создаёт координатор
func NewArbitrageCoordinator(
	detector *ArbitrageDetector,
	orderExec *OrderExecutor,
	validator *OrderValidator,
	maxArbitrages int,
	activeArbs *int64,
) *ArbitrageCoordinator {
	return &ArbitrageCoordinator{
		detector:      detector,
		orderExec:     orderExec,
		validator:     validator,
		maxArbitrages: maxArbitrages,
		activeArbs:    activeArbs,
	}
}

// SetPartialManager устанавливает менеджер частичного входа
func (ac *ArbitrageCoordinator) SetPartialManager(pm *PartialEntryManager) {
	ac.partialManager = pm
}

// SetFailHandler устанавливает обработчик провала второй ноги
func (ac *ArbitrageCoordinator) SetFailHandler(fh *SecondLegFailHandler) {
	ac.failHandler = fh
}

// TryEnter пытается войти в арбитраж для пары
//
// Возвращает:
// - true если вход выполнен успешно
// - false если условия не выполнены или произошла ошибка
func (ac *ArbitrageCoordinator) TryEnter(ctx context.Context, ps *PairState) (bool, *ExecuteResult, error) {
	// Проверяем условия входа
	var currentArbs int64
	if ac.activeArbs != nil {
		currentArbs = atomic.LoadInt64(ac.activeArbs)
	}
	conditions := ac.detector.CheckEntryConditions(ps, currentArbs, ac.maxArbitrages, ac.validator)

	if !conditions.CanEnter {
		return false, nil, nil // Условия не выполнены (нормальная ситуация)
	}

	config := ps.Config
	opp := conditions.Opportunity

	// Выполняем вход
	var result *ExecuteResult

	if config.NOrders > 1 && ac.partialManager != nil {
		// Частичный вход
		partialResult := ac.partialManager.ExecutePartialEntry(ctx, PartialEntryParams{
			Symbol:        config.Symbol,
			TotalVolume:   conditions.AdjustedVolume,
			NOrders:       config.NOrders,
			LongExchange:  opp.LongExchange,
			ShortExchange: opp.ShortExchange,
			MinSpread:     config.EntrySpreadPct * 0.8, // 80% от entry spread
		})

		result = &ExecuteResult{
			Success: partialResult.Success,
			Legs:    partialResult.Legs,
			Error:   partialResult.Error,
		}
	} else {
		// Одиночный вход
		execParams := ExecuteParams{
			Symbol:        config.Symbol,
			Volume:        conditions.AdjustedVolume,
			LongExchange:  opp.LongExchange,
			ShortExchange: opp.ShortExchange,
			NOrders:       1,
		}

		result = ac.orderExec.ExecuteParallel(ctx, execParams)
	}

	if result.Success {
		return true, result, nil
	}

	return false, result, result.Error
}

// TryExit пытается выйти из позиции
//
// Возвращает:
// - true если выход выполнен
// - false если условия выхода не достигнуты
func (ac *ArbitrageCoordinator) TryExit(ctx context.Context, ps *PairState) (bool, *ExecuteResult, ExitReason) {
	conditions := ac.detector.CheckExitConditions(ps)

	if !conditions.ShouldExit {
		return false, nil, ExitReasonNone
	}

	// Выполняем закрытие
	result := ac.orderExec.CloseParallel(ctx, CloseParams{
		Symbol: ps.Config.Symbol,
		Legs:   ps.Runtime.Legs,
	})

	return result.Success, result, conditions.Reason
}

// ============================================================
// Метрики для мониторинга
// ============================================================

// ArbitrageMetrics метрики арбитражного модуля
type ArbitrageMetrics struct {
	OpportunitiesDetected int64
	EntriesTriggered      int64
	ExitsTriggered        int64
}

// GetMetrics возвращает текущие метрики детектора
func (ad *ArbitrageDetector) GetMetrics() ArbitrageMetrics {
	return ArbitrageMetrics{
		OpportunitiesDetected: atomic.LoadInt64(&ad.opportunitiesDetected),
		EntriesTriggered:      atomic.LoadInt64(&ad.entriesTriggered),
		ExitsTriggered:        atomic.LoadInt64(&ad.exitsTriggered),
	}
}
