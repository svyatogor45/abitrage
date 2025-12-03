package bot

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"arbitrage/internal/config"
	"arbitrage/internal/exchange"
	"arbitrage/internal/models"
	"arbitrage/pkg/retry"
	"arbitrage/pkg/utils"
)

// ============ ОПТИМИЗАЦИЯ: Object Pool для каналов LegResult ============
// Убирает аллокацию каналов на каждый ордер (100+ ордеров/день)
// Каналы - относительно "тяжёлые" объекты для GC

var legResultChanPool = sync.Pool{
	New: func() interface{} {
		return make(chan LegResult, 1)
	},
}

// acquireLegResultChan получает канал из пула
func acquireLegResultChan() chan LegResult {
	return legResultChanPool.Get().(chan LegResult)
}

// releaseLegResultChan очищает и возвращает канал в пул
func releaseLegResultChan(ch chan LegResult) {
	// Очищаем канал перед возвратом в пул
	select {
	case <-ch:
	default:
	}
	legResultChanPool.Put(ch)
}

// OrderExecutor - исполнитель ордеров с ПАРАЛЛЕЛЬНОЙ отправкой
//
// Ключевая оптимизация:
// - Ордера на обе биржи отправляются ОДНОВРЕМЕННО (goroutines)
// - Общее время = max(время_биржи_A, время_биржи_B), а не сумма
// - Типичное ускорение: 300ms вместо 600ms
type OrderExecutor struct {
	exchanges map[string]exchange.Exchange
	cfg       config.BotConfig
	mu        sync.RWMutex
}

// ExecuteParams - параметры для исполнения арбитража
type ExecuteParams struct {
	Symbol        string
	Volume        float64 // объём в базовой валюте
	LongExchange  string  // биржа для лонга
	ShortExchange string  // биржа для шорта
	NOrders       int     // на сколько частей разбить
}

// ExecuteResult - результат исполнения
type ExecuteResult struct {
	Success bool
	Error   error

	// Открытые позиции
	Legs []models.Leg

	// Детали исполнения
	LongOrder  *exchange.Order
	ShortOrder *exchange.Order

	// PNL при закрытии позиции
	TotalPnl float64
}

// CloseParams - параметры для закрытия позиции
type CloseParams struct {
	Symbol string
	Legs   []models.Leg
}

// LegResult - результат одной ноги
type LegResult struct {
	Order *exchange.Order
	Error error
}

// NewOrderExecutor создаёт исполнитель
func NewOrderExecutor(exchanges map[string]exchange.Exchange, cfg config.BotConfig) *OrderExecutor {
	return &OrderExecutor{
		exchanges: exchanges,
		cfg:       cfg,
	}
}

// ExecuteParallel выполняет вход в арбитраж ПАРАЛЛЕЛЬНО на обеих биржах
//
// Тайминги:
// - Отправка ордеров: ~0ms (мгновенный запуск goroutines)
// - Ожидание ответа: max(latency_A, latency_B) ≈ 150-300ms
// - БЕЗ параллелизма было бы: latency_A + latency_B ≈ 300-600ms
//
// ОПТИМИЗАЦИЯ: использует sync.Pool для каналов - без аллокаций на каждый ордер
func (oe *OrderExecutor) ExecuteParallel(ctx context.Context, params ExecuteParams) *ExecuteResult {
	oe.mu.RLock()
	longExch, longOk := oe.exchanges[params.LongExchange]
	shortExch, shortOk := oe.exchanges[params.ShortExchange]
	oe.mu.RUnlock()

	if !longOk || !shortOk {
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("exchange not found: long=%s(%v) short=%s(%v)",
				params.LongExchange, longOk, params.ShortExchange, shortOk),
		}
	}

	// ОПТИМИЗАЦИЯ: каналы из пула (без аллокации!)
	longCh := acquireLegResultChan()
	shortCh := acquireLegResultChan()
	defer releaseLegResultChan(longCh)
	defer releaseLegResultChan(shortCh)

	// Объём для одной части (если разбиваем на N ордеров)
	partVolume := params.Volume
	if params.NOrders > 1 {
		partVolume = params.Volume / float64(params.NOrders)
	}

	// ПАРАЛЛЕЛЬНАЯ отправка ордеров
	go func() {
		order, err := longExch.PlaceMarketOrder(ctx, params.Symbol, exchange.SideBuy, partVolume)
		longCh <- LegResult{Order: order, Error: err}
	}()

	go func() {
		order, err := shortExch.PlaceMarketOrder(ctx, params.Symbol, exchange.SideSell, partVolume)
		shortCh <- LegResult{Order: order, Error: err}
	}()

	// ОПТИМИЗАЦИЯ: параллельное ожидание обоих результатов
	// Было: ждём long → потом ждём short (если long медленный, не слушаем short)
	// Стало: слушаем оба канала одновременно
	var longResult, shortResult LegResult
	var longReceived, shortReceived bool

	for !longReceived || !shortReceived {
		select {
		case longResult = <-longCh:
			longReceived = true
		case shortResult = <-shortCh:
			shortReceived = true
		case <-ctx.Done():
			// Timeout - откатываем то, что успело исполниться
			if longReceived && longResult.Error == nil {
				oe.rollbackLong(params.Symbol, longExch, longResult.Order)
			}
			if shortReceived && shortResult.Error == nil {
				oe.rollbackShort(params.Symbol, shortExch, shortResult.Order)
			}
			return &ExecuteResult{Success: false, Error: ctx.Err()}
		}
	}

	// Обработка результатов
	return oe.handleResults(ctx, params, longExch, shortExch, longResult, shortResult)
}

// handleResults обрабатывает результаты параллельного исполнения
func (oe *OrderExecutor) handleResults(
	ctx context.Context,
	params ExecuteParams,
	longExch, shortExch exchange.Exchange,
	longRes, shortRes LegResult,
) *ExecuteResult {

	// Оба успешны
	if longRes.Error == nil && shortRes.Error == nil {
		return &ExecuteResult{
			Success:    true,
			LongOrder:  longRes.Order,
			ShortOrder: shortRes.Order,
			Legs: []models.Leg{
				{
					Exchange:   params.LongExchange,
					Side:       "long",
					EntryPrice: longRes.Order.AvgFillPrice,
					Quantity:   longRes.Order.FilledQty,
				},
				{
					Exchange:   params.ShortExchange,
					Side:       "short",
					EntryPrice: shortRes.Order.AvgFillPrice,
					Quantity:   shortRes.Order.FilledQty,
				},
			},
		}
	}

	// Лонг успешен, шорт провалился - откат лонга
	if longRes.Error == nil && shortRes.Error != nil {
		oe.rollbackLong(params.Symbol, longExch, longRes.Order)
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("short failed, long rolled back: %w", shortRes.Error),
		}
	}

	// Шорт успешен, лонг провалился - откат шорта
	if shortRes.Error == nil && longRes.Error != nil {
		oe.rollbackShort(params.Symbol, shortExch, shortRes.Order)
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("long failed, short rolled back: %w", longRes.Error),
		}
	}

	// Оба провалились
	return &ExecuteResult{
		Success: false,
		Error:   fmt.Errorf("both legs failed: long=%v, short=%v", longRes.Error, shortRes.Error),
	}
}

// rollbackLong закрывает лонг при ошибке шорта
func (oe *OrderExecutor) rollbackLong(symbol string, exch exchange.Exchange, order *exchange.Order) {
	if order == nil || order.FilledQty == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), oe.cfg.OrderTimeout)
	defer cancel()

	// Продаём то, что купили
	_, _ = exch.PlaceMarketOrder(ctx, symbol, exchange.SideSell, order.FilledQty)
}

// rollbackShort закрывает шорт при ошибке лонга
func (oe *OrderExecutor) rollbackShort(symbol string, exch exchange.Exchange, order *exchange.Order) {
	if order == nil || order.FilledQty == 0 {
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), oe.cfg.OrderTimeout)
	defer cancel()

	// Покупаем то, что продали
	_, _ = exch.PlaceMarketOrder(ctx, symbol, exchange.SideBuy, order.FilledQty)
}

// CloseParallel закрывает обе позиции параллельно
//
// ОПТИМИЗАЦИЯ: использует sync.Pool для каналов - без аллокаций на каждый ордер
func (oe *OrderExecutor) CloseParallel(ctx context.Context, params CloseParams) *ExecuteResult {
	legs := params.Legs
	symbol := params.Symbol

	if len(legs) != 2 {
		return &ExecuteResult{Success: false, Error: fmt.Errorf("expected 2 legs, got %d", len(legs))}
	}

	oe.mu.RLock()
	exch1, ok1 := oe.exchanges[legs[0].Exchange]
	exch2, ok2 := oe.exchanges[legs[1].Exchange]
	oe.mu.RUnlock()

	if !ok1 || !ok2 {
		return &ExecuteResult{Success: false, Error: fmt.Errorf("exchange not found")}
	}

	// ОПТИМИЗАЦИЯ: каналы из пула (без аллокации!)
	ch1 := acquireLegResultChan()
	ch2 := acquireLegResultChan()
	defer releaseLegResultChan(ch1)
	defer releaseLegResultChan(ch2)

	// Параллельное закрытие
	go func() {
		var side string
		if legs[0].Side == "long" {
			side = exchange.SideSell // закрываем лонг продажей
		} else {
			side = exchange.SideBuy // закрываем шорт покупкой
		}
		order, err := exch1.PlaceMarketOrder(ctx, symbol, side, legs[0].Quantity)
		ch1 <- LegResult{Order: order, Error: err}
	}()

	go func() {
		var side string
		if legs[1].Side == "long" {
			side = exchange.SideSell
		} else {
			side = exchange.SideBuy
		}
		order, err := exch2.PlaceMarketOrder(ctx, symbol, side, legs[1].Quantity)
		ch2 <- LegResult{Order: order, Error: err}
	}()

	// ОПТИМИЗАЦИЯ: параллельное ожидание обоих результатов
	var res1, res2 LegResult
	var res1Received, res2Received bool

	for !res1Received || !res2Received {
		select {
		case res1 = <-ch1:
			res1Received = true
		case res2 = <-ch2:
			res2Received = true
		case <-ctx.Done():
			return &ExecuteResult{Success: false, Error: ctx.Err()}
		}
	}

	// Проверяем результаты
	if res1.Error != nil || res2.Error != nil {
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("close failed: leg1=%v, leg2=%v", res1.Error, res2.Error),
		}
	}

	// Рассчитываем PNL для каждой ноги
	var totalPnl float64
	for i, leg := range legs {
		var result LegResult
		if i == 0 {
			result = res1
		} else {
			result = res2
		}

		if result.Order != nil {
			closePrice := result.Order.AvgFillPrice
			if leg.Side == "long" {
				// Long: PNL = (цена закрытия - цена входа) * количество
				totalPnl += (closePrice - leg.EntryPrice) * leg.Quantity
			} else {
				// Short: PNL = (цена входа - цена закрытия) * количество
				totalPnl += (leg.EntryPrice - closePrice) * leg.Quantity
			}
		}
	}

	return &ExecuteResult{
		Success:    true,
		LongOrder:  res1.Order,
		ShortOrder: res2.Order,
		TotalPnl:   totalPnl,
	}
}

// UpdateExchanges обновляет карту бирж (потокобезопасно)
func (oe *OrderExecutor) UpdateExchanges(exchanges map[string]exchange.Exchange) {
	oe.mu.Lock()
	oe.exchanges = exchanges
	oe.mu.Unlock()
}

// ============================================================
// OrderValidator - валидация ордеров согласно лимитам биржи
// ============================================================
//
// Согласно ТЗ раздел "Проверка маржи и лимитов перед входом":
// - Проверка минимального/максимального размера ордера
// - Округление объёмов до lot size биржи
// - Проверка min notional (минимальная сумма сделки в USDT)
// - Предотвращение отклонения ордеров биржей

// OrderValidator проверяет и корректирует параметры ордеров
type OrderValidator struct {
	// Кэш лимитов по биржам и символам
	// Key: LimitsKey{Exchange, Symbol}
	limits sync.Map

	// Кэш текущих цен для расчёта notional
	priceGetter func(symbol, exchange string) float64

	// Дефолтные значения если лимиты не загружены
	defaultMinQty    float64
	defaultMaxQty    float64
	defaultQtyStep   float64
	defaultMinNotional float64
}

// LimitsKey - ключ для кэша лимитов
type LimitsKey struct {
	Exchange string
	Symbol   string
}

// CachedLimits - кэшированные лимиты с timestamp
type CachedLimits struct {
	MinOrderQty  float64
	MaxOrderQty  float64
	QtyStep      float64 // lot size
	MinNotional  float64
	PriceStep    float64 // tick size
	MaxLeverage  int
	UpdatedAt    time.Time
}

// ValidationResult - результат валидации
type ValidationResult struct {
	Valid        bool
	AdjustedQty  float64 // скорректированный объём
	Error        string  // описание ошибки если Invalid
	Warnings     []string
}

// NewOrderValidator создаёт валидатор ордеров
func NewOrderValidator(priceGetter func(symbol, exchange string) float64) *OrderValidator {
	return &OrderValidator{
		priceGetter:      priceGetter,
		defaultMinQty:    0.001,    // 0.001 BTC
		defaultMaxQty:    1000.0,   // 1000 BTC
		defaultQtyStep:   0.001,    // 0.001 BTC
		defaultMinNotional: 5.0,    // 5 USDT минимальная сумма
	}
}

// UpdateLimits обновляет кэшированные лимиты для пары биржа+символ
func (ov *OrderValidator) UpdateLimits(exchangeName, symbol string, limits *exchange.Limits) {
	if limits == nil {
		return
	}

	key := LimitsKey{Exchange: exchangeName, Symbol: symbol}
	cached := &CachedLimits{
		MinOrderQty:  limits.MinOrderQty,
		MaxOrderQty:  limits.MaxOrderQty,
		QtyStep:      limits.QtyStep,
		MinNotional:  limits.MinNotional,
		PriceStep:    limits.PriceStep,
		MaxLeverage:  limits.MaxLeverage,
		UpdatedAt:    time.Now(),
	}

	ov.limits.Store(key, cached)
}

// GetLimits возвращает кэшированные лимиты
func (ov *OrderValidator) GetLimits(exchangeName, symbol string) *CachedLimits {
	key := LimitsKey{Exchange: exchangeName, Symbol: symbol}
	if v, ok := ov.limits.Load(key); ok {
		return v.(*CachedLimits)
	}
	return nil
}

// ValidateOrderQty проверяет и корректирует количество для ордера
//
// Выполняет:
// 1. Проверку минимального количества (min_order_qty)
// 2. Проверку максимального количества (max_order_qty)
// 3. Округление до lot size (qty_step)
// 4. Проверку минимальной суммы сделки (min_notional)
//
// Возвращает скорректированное количество и информацию о валидности
func (ov *OrderValidator) ValidateOrderQty(
	exchangeName string,
	symbol string,
	qty float64,
	currentPrice float64,
) *ValidationResult {
	result := &ValidationResult{
		Valid:       true,
		AdjustedQty: qty,
		Warnings:    make([]string, 0),
	}

	// Получаем лимиты (или дефолты)
	limits := ov.GetLimits(exchangeName, symbol)
	var minQty, maxQty, qtyStep, minNotional float64

	if limits != nil {
		minQty = limits.MinOrderQty
		maxQty = limits.MaxOrderQty
		qtyStep = limits.QtyStep
		minNotional = limits.MinNotional
	} else {
		// Используем дефолты если лимиты не загружены
		minQty = ov.defaultMinQty
		maxQty = ov.defaultMaxQty
		qtyStep = ov.defaultQtyStep
		minNotional = ov.defaultMinNotional
		result.Warnings = append(result.Warnings,
			"using default limits for "+exchangeName+":"+symbol)
	}

	// 1. Округляем до lot size (qtyStep)
	if qtyStep > 0 {
		result.AdjustedQty = ov.roundToStep(qty, qtyStep)
		if result.AdjustedQty != qty {
			result.Warnings = append(result.Warnings,
				fmt.Sprintf("qty adjusted from %.8f to %.8f (lot size: %.8f)",
					qty, result.AdjustedQty, qtyStep))
		}
	}

	// 2. Проверяем минимальное количество
	if result.AdjustedQty < minQty {
		result.Valid = false
		result.Error = fmt.Sprintf("qty %.8f below minimum %.8f on %s",
			result.AdjustedQty, minQty, exchangeName)
		return result
	}

	// 3. Проверяем максимальное количество
	if maxQty > 0 && result.AdjustedQty > maxQty {
		// Ограничиваем до максимума
		oldQty := result.AdjustedQty
		result.AdjustedQty = ov.roundToStep(maxQty, qtyStep)
		result.Warnings = append(result.Warnings,
			fmt.Sprintf("qty limited from %.8f to max %.8f on %s",
				oldQty, result.AdjustedQty, exchangeName))
	}

	// 4. Проверяем min notional (минимальная сумма в USDT)
	if minNotional > 0 && currentPrice > 0 {
		notional := result.AdjustedQty * currentPrice
		if notional < minNotional {
			// Рассчитываем минимальное количество для достижения min notional
			requiredQty := minNotional / currentPrice
			requiredQty = ov.roundToStep(requiredQty, qtyStep)

			// Проверяем что требуемое qty не меньше minQty
			if requiredQty < minQty {
				requiredQty = minQty
			}

			result.Valid = false
			result.Error = fmt.Sprintf(
				"notional %.2f USDT below minimum %.2f USDT on %s (need qty >= %.8f)",
				notional, minNotional, exchangeName, requiredQty)
			return result
		}
	}

	return result
}

// ValidateBothLegs проверяет обе ноги арбитража
//
// Находит максимальный объём, который удовлетворяет лимитам обеих бирж
func (ov *OrderValidator) ValidateBothLegs(
	longExchange string,
	shortExchange string,
	symbol string,
	qty float64,
	longPrice float64,
	shortPrice float64,
) *ValidationResult {
	// Валидируем для лонга
	longResult := ov.ValidateOrderQty(longExchange, symbol, qty, longPrice)
	if !longResult.Valid {
		return longResult
	}

	// Валидируем для шорта
	shortResult := ov.ValidateOrderQty(shortExchange, symbol, qty, shortPrice)
	if !shortResult.Valid {
		return shortResult
	}

	// Берём минимальный из скорректированных объёмов
	adjustedQty := longResult.AdjustedQty
	if shortResult.AdjustedQty < adjustedQty {
		adjustedQty = shortResult.AdjustedQty
	}

	// Собираем предупреждения
	warnings := make([]string, 0)
	warnings = append(warnings, longResult.Warnings...)
	warnings = append(warnings, shortResult.Warnings...)

	if adjustedQty != qty {
		warnings = append(warnings,
			fmt.Sprintf("final qty adjusted from %.8f to %.8f", qty, adjustedQty))
	}

	return &ValidationResult{
		Valid:       true,
		AdjustedQty: adjustedQty,
		Warnings:    warnings,
	}
}

// roundToStep округляет значение до ближайшего кратного step (в меньшую сторону)
// Делегирует в utils.RoundToLotSize для единообразия
func (ov *OrderValidator) roundToStep(value, step float64) float64 {
	return utils.RoundToLotSize(value, step)
}

// RoundToLotSize публичная функция для округления до lot size
// Обёртка над utils.RoundToLotSize для обратной совместимости
func RoundToLotSize(value, lotSize float64) float64 {
	return utils.RoundToLotSize(value, lotSize)
}

// ============================================================
// Расширение OrderExecutor для использования валидации
// ============================================================

// ExecuteWithValidation выполняет ордер с предварительной валидацией
func (oe *OrderExecutor) ExecuteWithValidation(
	ctx context.Context,
	params ExecuteParams,
	validator *OrderValidator,
	longPrice float64,
	shortPrice float64,
) *ExecuteResult {
	// Валидируем параметры
	validation := validator.ValidateBothLegs(
		params.LongExchange,
		params.ShortExchange,
		params.Symbol,
		params.Volume,
		longPrice,
		shortPrice,
	)

	if !validation.Valid {
		return &ExecuteResult{
			Success: false,
			Error:   fmt.Errorf("validation failed: %s", validation.Error),
		}
	}

	// Используем скорректированный объём
	params.Volume = validation.AdjustedQty

	// Выполняем ордер
	return oe.ExecuteParallel(ctx, params)
}

// ============================================================
// Утилиты для работы с лимитами
// ============================================================

// LoadLimitsFromExchange загружает лимиты для символа с биржи
func (ov *OrderValidator) LoadLimitsFromExchange(
	ctx context.Context,
	exch exchange.Exchange,
	symbol string,
) error {
	limits, err := exch.GetLimits(ctx, symbol)
	if err != nil {
		return fmt.Errorf("failed to get limits from %s: %w", exch.GetName(), err)
	}

	ov.UpdateLimits(exch.GetName(), symbol, limits)
	return nil
}

// PreloadLimits загружает лимиты для всех активных пар
func (ov *OrderValidator) PreloadLimits(
	ctx context.Context,
	exchanges map[string]exchange.Exchange,
	symbols []string,
) map[string]error {
	errors := make(map[string]error)

	for exchName, exch := range exchanges {
		for _, symbol := range symbols {
			key := exchName + ":" + symbol
			if err := ov.LoadLimitsFromExchange(ctx, exch, symbol); err != nil {
				errors[key] = err
			}
		}
	}

	return errors
}

// LimitsInfo возвращает информацию о загруженных лимитах (для отладки)
func (ov *OrderValidator) LimitsInfo() map[string]*CachedLimits {
	result := make(map[string]*CachedLimits)

	ov.limits.Range(func(key, value interface{}) bool {
		k := key.(LimitsKey)
		v := value.(*CachedLimits)
		result[k.Exchange+":"+k.Symbol] = v
		return true
	})

	return result
}

// ============================================================
// Интеграция с pkg/retry для повторных попыток
// ============================================================

// ExecuteWithRetry выполняет ордер с retry при сетевых ошибках
func (oe *OrderExecutor) ExecuteWithRetry(
	ctx context.Context,
	params ExecuteParams,
	retryConfig retry.Config,
) *ExecuteResult {
	var result *ExecuteResult

	operation := func() error {
		result = oe.ExecuteParallel(ctx, params)
		if result.Success {
			return nil
		}
		// Проверяем, является ли ошибка retriable
		if isRetriableError(result.Error) {
			return result.Error
		}
		// Не-retriable ошибка - не повторяем
		return retry.Permanent(result.Error)
	}

	err := retry.Do(ctx, operation, retryConfig)
	if err != nil && result == nil {
		result = &ExecuteResult{
			Success: false,
			Error:   err,
		}
	}

	return result
}

// isRetriableError определяет, можно ли повторить операцию
func isRetriableError(err error) bool {
	if err == nil {
		return false
	}

	errStr := err.Error()

	// Сетевые ошибки - можно повторить
	retriablePatterns := []string{
		"timeout",
		"connection refused",
		"connection reset",
		"temporary failure",
		"network unreachable",
		"i/o timeout",
		"EOF",
		"rate limit", // rate limit - подождать и повторить
	}

	for _, pattern := range retriablePatterns {
		if containsIgnoreCase(errStr, pattern) {
			return true
		}
	}

	return false
}

// containsIgnoreCase проверяет наличие подстроки без учёта регистра
func containsIgnoreCase(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}
