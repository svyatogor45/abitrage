package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"arbitrage/internal/exchange"
	"arbitrage/internal/models"
	"arbitrage/pkg/retry"
)

// RiskManager - централизованный менеджер рисков
//
// Функции:
// - Мониторинг Stop Loss: проверка PNL ≤ -SL
// - Автоматическое закрытие при достижении SL
// - Обнаружение ликвидаций через WebSocket биржи
// - Экстренное закрытие второй ноги при ликвидации первой
// - Автоматическая постановка пары на паузу после SL/ликвидации
// - Проверка маржинальных требований перед входом
// - Расчет margin requirement для позиции
// - Генерация уведомлений о критических событиях
type RiskManager struct {
	// Подключенные биржи для проверки маржи и закрытия позиций
	exchanges map[string]exchange.Exchange
	exchMu    sync.RWMutex

	// Кэш маржинальных данных (exchange+symbol → margin info)
	marginCache sync.Map // map[MarginKey]*MarginInfo

	// Кэш лимитов бирж (exchange+symbol → limits)
	limitsCache sync.Map // map[LimitsKey]*exchange.Limits

	// Канал для уведомлений
	notificationChan chan<- *models.Notification

	// Callback для закрытия позиций
	closePositionFn func(ctx context.Context, ps *PairState, reason ExitReason) error

	// Callback для перевода пары в паузу
	pausePairFn func(pairID int)

	// Конфигурация
	config RiskConfig
}

// RiskConfig - конфигурация риск-менеджера
type RiskConfig struct {
	// Минимальный запас маржи (множитель от required margin)
	// Например, 1.5 означает что нужно 150% от минимально необходимой маржи
	MinMarginBuffer float64

	// Интервал проверки рисков
	CheckInterval time.Duration

	// Таймаут для операций закрытия
	CloseTimeout time.Duration

	// Максимальное количество retry при закрытии
	MaxCloseRetries int
}

// DefaultRiskConfig возвращает конфигурацию по умолчанию
func DefaultRiskConfig() RiskConfig {
	return RiskConfig{
		MinMarginBuffer: 1.5,
		CheckInterval:   500 * time.Millisecond,
		CloseTimeout:    30 * time.Second,
		MaxCloseRetries: 4,
	}
}

// MarginKey - ключ для кэша маржи
type MarginKey struct {
	Exchange string
	Symbol   string
}

// MarginInfo - информация о марже
type MarginInfo struct {
	AvailableBalance float64   // Доступный баланс
	UsedMargin       float64   // Используемая маржа
	TotalEquity      float64   // Общий equity
	UpdatedAt        time.Time // Время обновления
}

// LimitsKey определён в order.go - используем его для кэша лимитов

// NewRiskManager создает новый RiskManager
func NewRiskManager(
	notifChan chan<- *models.Notification,
	closePosFn func(ctx context.Context, ps *PairState, reason ExitReason) error,
	pauseFn func(pairID int),
	config RiskConfig,
) *RiskManager {
	return &RiskManager{
		exchanges:        make(map[string]exchange.Exchange),
		notificationChan: notifChan,
		closePositionFn:  closePosFn,
		pausePairFn:      pauseFn,
		config:           config,
	}
}

// SetExchanges устанавливает биржи для риск-менеджера
func (rm *RiskManager) SetExchanges(exchanges map[string]exchange.Exchange) {
	rm.exchMu.Lock()
	rm.exchanges = exchanges
	rm.exchMu.Unlock()
}

// AddExchange добавляет биржу
func (rm *RiskManager) AddExchange(name string, exch exchange.Exchange) {
	rm.exchMu.Lock()
	rm.exchanges[name] = exch
	rm.exchMu.Unlock()
}

// ============================================================
// Stop Loss мониторинг
// ============================================================

// CheckStopLoss проверяет достижение Stop Loss для пары
//
// Возвращает true, если нужно закрывать позицию (PNL ≤ -SL)
func (rm *RiskManager) CheckStopLoss(ps *PairState) (shouldClose bool, currentPnl float64) {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// SL не установлен
	if ps.Config.StopLoss <= 0 {
		return false, ps.Runtime.UnrealizedPnl
	}

	// Проверяем только в состоянии HOLDING
	if ps.Runtime.State != models.StateHolding {
		return false, ps.Runtime.UnrealizedPnl
	}

	currentPnl = ps.Runtime.UnrealizedPnl

	// SL сработал если PNL <= -SL
	// Например: SL=100 USDT, PNL=-105 → -105 <= -100 → true
	if currentPnl <= -ps.Config.StopLoss {
		return true, currentPnl
	}

	return false, currentPnl
}

// HandleStopLoss обрабатывает срабатывание Stop Loss
//
// 1. Закрывает обе позиции по рынку
// 2. Ставит пару на паузу
// 3. Генерирует уведомление
func (rm *RiskManager) HandleStopLoss(ctx context.Context, ps *PairState) error {
	// Закрываем позицию
	if rm.closePositionFn != nil {
		if err := rm.closePositionFn(ctx, ps, ExitReasonStopLoss); err != nil {
			rm.notifyError(ps, fmt.Errorf("failed to close position on SL: %w", err))
			return err
		}
	}

	// Ставим на паузу
	if rm.pausePairFn != nil {
		rm.pausePairFn(ps.Config.ID)
	}

	// Уведомление
	rm.notifyStopLoss(ps)

	return nil
}

// ============================================================
// Детекция и обработка ликвидаций
// ============================================================

// LiquidationEvent - событие ликвидации
type LiquidationEvent struct {
	Exchange string
	Symbol   string
	Side     string // "long" или "short"
	Size     float64
	Time     time.Time
}

// HandleLiquidation обрабатывает событие ликвидации позиции
//
// 1. Определяет какая нога ликвидирована
// 2. Экстренно закрывает вторую ногу
// 3. Ставит пару на паузу
// 4. Генерирует уведомление
func (rm *RiskManager) HandleLiquidation(ctx context.Context, ps *PairState, event LiquidationEvent) error {
	ps.mu.Lock()

	// Проверяем что пара в состоянии HOLDING
	if ps.Runtime.State != models.StateHolding {
		ps.mu.Unlock()
		return nil
	}

	// Находим какая нога ликвидирована и какую нужно закрыть
	var liquidatedLeg, remainingLeg *models.Leg
	for i := range ps.Runtime.Legs {
		leg := &ps.Runtime.Legs[i]
		if leg.Exchange == event.Exchange && leg.Side == event.Side {
			liquidatedLeg = leg
		} else {
			remainingLeg = leg
		}
	}

	if liquidatedLeg == nil || remainingLeg == nil {
		ps.mu.Unlock()
		return fmt.Errorf("could not identify legs for liquidation event")
	}

	// Переводим в состояние EXITING через state machine
	oldState := ps.Runtime.State
	ForceTransition(ps.Runtime, models.StateExiting)
	RecordTransition(oldState, models.StateExiting)
	ps.mu.Unlock()

	// Экстренное закрытие оставшейся ноги с retry
	closeErr := rm.emergencyCloseLeg(ctx, ps.Config.Symbol, remainingLeg)

	ps.mu.Lock()
	if closeErr != nil {
		// Ошибка закрытия - переводим в ERROR через state machine
		oldState := ps.Runtime.State
		ForceTransition(ps.Runtime, models.StateError)
		RecordTransition(oldState, models.StateError)
		ps.mu.Unlock()
		rm.notifyError(ps, fmt.Errorf("emergency close failed after liquidation: %w", closeErr))
		return closeErr
	}

	// Очищаем позицию и ставим на паузу через state machine
	ps.Runtime.Legs = nil
	oldState = ps.Runtime.State
	ForceTransition(ps.Runtime, models.StatePaused)
	RecordTransition(oldState, models.StatePaused)
	ps.Config.Status = "paused"
	ps.mu.Unlock()

	// Вызываем callback паузы
	if rm.pausePairFn != nil {
		rm.pausePairFn(ps.Config.ID)
	}

	// Уведомление о ликвидации
	rm.notifyLiquidation(ps, event)

	return nil
}

// emergencyCloseLeg экстренно закрывает одну ногу с retry
func (rm *RiskManager) emergencyCloseLeg(ctx context.Context, symbol string, leg *models.Leg) error {
	rm.exchMu.RLock()
	exch, ok := rm.exchanges[leg.Exchange]
	rm.exchMu.RUnlock()

	if !ok {
		return fmt.Errorf("exchange %s not found", leg.Exchange)
	}

	// Определяем направление закрытия
	closeSide := exchange.SideSell
	if leg.Side == "short" {
		closeSide = exchange.SideBuy
	}

	// Используем aggressive retry для критической операции
	cfg := retry.AggressiveConfig()
	cfg.MaxRetries = rm.config.MaxCloseRetries

	return retry.Do(ctx, func() error {
		return exch.ClosePosition(ctx, symbol, closeSide, leg.Quantity)
	}, cfg)
}

// OnPositionUpdate обрабатывает обновление позиции от биржи
//
// Проверяет флаг Liquidation и инициирует обработку ликвидации
func (rm *RiskManager) OnPositionUpdate(ps *PairState, update PositionUpdate) {
	if !update.Liquidated {
		return
	}

	event := LiquidationEvent{
		Exchange: update.Exchange,
		Symbol:   update.Symbol,
		Side:     update.Side,
		Time:     time.Now(),
	}

	ctx, cancel := context.WithTimeout(context.Background(), rm.config.CloseTimeout)
	defer cancel()

	if err := rm.HandleLiquidation(ctx, ps, event); err != nil {
		// Логируем ошибку, уведомление уже отправлено
		fmt.Printf("ERROR: HandleLiquidation failed: %v\n", err)
	}
}

// ============================================================
// Проверка маржинальных требований
// ============================================================

// MarginCheck - результат проверки маржи
type MarginCheck struct {
	Sufficient       bool    // Достаточно ли маржи
	RequiredMargin   float64 // Необходимая маржа
	AvailableMargin  float64 // Доступная маржа
	Deficit          float64 // Дефицит (если недостаточно)
	Exchange         string  // Биржа
}

// CheckMarginRequirement проверяет достаточность маржи для открытия позиции
//
// Параметры:
// - exchange: название биржи
// - symbol: торговая пара
// - volume: объем в базовой валюте
// - price: ожидаемая цена входа
// - leverage: плечо (по умолчанию 1)
//
// Возвращает MarginCheck с результатом проверки
func (rm *RiskManager) CheckMarginRequirement(
	ctx context.Context,
	exchangeName string,
	symbol string,
	volume float64,
	price float64,
	leverage int,
) (*MarginCheck, error) {
	rm.exchMu.RLock()
	exch, ok := rm.exchanges[exchangeName]
	rm.exchMu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("exchange %s not found", exchangeName)
	}

	// Получаем баланс
	balance, err := exch.GetBalance(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get balance: %w", err)
	}

	// Рассчитываем необходимую маржу
	// Notional value = volume * price
	// Required margin = notional / leverage
	if leverage <= 0 {
		leverage = 1
	}

	notionalValue := volume * price
	requiredMargin := notionalValue / float64(leverage)

	// Применяем буфер безопасности
	requiredWithBuffer := requiredMargin * rm.config.MinMarginBuffer

	result := &MarginCheck{
		RequiredMargin:  requiredWithBuffer,
		AvailableMargin: balance,
		Exchange:        exchangeName,
	}

	if balance >= requiredWithBuffer {
		result.Sufficient = true
	} else {
		result.Sufficient = false
		result.Deficit = requiredWithBuffer - balance
	}

	// Кэшируем информацию о марже
	rm.marginCache.Store(MarginKey{Exchange: exchangeName, Symbol: symbol}, &MarginInfo{
		AvailableBalance: balance,
		TotalEquity:      balance,
		UpdatedAt:        time.Now(),
	})

	return result, nil
}

// CheckBothLegsMargin проверяет маржу для обеих ног арбитража
func (rm *RiskManager) CheckBothLegsMargin(
	ctx context.Context,
	symbol string,
	volume float64,
	longExchange string,
	longPrice float64,
	shortExchange string,
	shortPrice float64,
	leverage int,
) (longCheck, shortCheck *MarginCheck, err error) {
	// Проверяем параллельно обе биржи
	var wg sync.WaitGroup
	var longErr, shortErr error

	wg.Add(2)

	go func() {
		defer wg.Done()
		longCheck, longErr = rm.CheckMarginRequirement(ctx, longExchange, symbol, volume, longPrice, leverage)
	}()

	go func() {
		defer wg.Done()
		shortCheck, shortErr = rm.CheckMarginRequirement(ctx, shortExchange, symbol, volume, shortPrice, leverage)
	}()

	wg.Wait()

	if longErr != nil {
		return nil, nil, fmt.Errorf("long margin check failed: %w", longErr)
	}
	if shortErr != nil {
		return nil, nil, fmt.Errorf("short margin check failed: %w", shortErr)
	}

	return longCheck, shortCheck, nil
}

// ============================================================
// Проверка лимитов биржи
// ============================================================

// ValidateOrderLimits проверяет соответствие ордера лимитам биржи
func (rm *RiskManager) ValidateOrderLimits(
	ctx context.Context,
	exchangeName string,
	symbol string,
	volume float64,
	price float64,
) error {
	rm.exchMu.RLock()
	exch, ok := rm.exchanges[exchangeName]
	rm.exchMu.RUnlock()

	if !ok {
		return fmt.Errorf("exchange %s not found", exchangeName)
	}

	// Пробуем получить из кэша
	key := LimitsKey{Exchange: exchangeName, Symbol: symbol}
	var limits *exchange.Limits

	if cached, ok := rm.limitsCache.Load(key); ok {
		limits = cached.(*exchange.Limits)
	} else {
		// Запрашиваем лимиты
		var err error
		limits, err = exch.GetLimits(ctx, symbol)
		if err != nil {
			return fmt.Errorf("failed to get limits: %w", err)
		}
		rm.limitsCache.Store(key, limits)
	}

	// Проверяем минимальный объем
	if volume < limits.MinOrderQty {
		return fmt.Errorf("volume %.8f below minimum %.8f for %s on %s",
			volume, limits.MinOrderQty, symbol, exchangeName)
	}

	// Проверяем максимальный объем
	if limits.MaxOrderQty > 0 && volume > limits.MaxOrderQty {
		return fmt.Errorf("volume %.8f exceeds maximum %.8f for %s on %s",
			volume, limits.MaxOrderQty, symbol, exchangeName)
	}

	// Проверяем минимальную сумму (notional)
	notional := volume * price
	if limits.MinNotional > 0 && notional < limits.MinNotional {
		return fmt.Errorf("notional value %.2f below minimum %.2f for %s on %s",
			notional, limits.MinNotional, symbol, exchangeName)
	}

	return nil
}

// ============================================================
// Уведомления
// ============================================================

// notifyStopLoss отправляет уведомление о срабатывании SL
func (rm *RiskManager) notifyStopLoss(ps *PairState) {
	if rm.notificationChan == nil {
		return
	}

	pairID := ps.Config.ID
	notif := &models.Notification{
		Timestamp: time.Now(),
		Type:      models.NotificationTypeSL,
		Severity:  models.SeverityWarn,
		PairID:    &pairID,
		Message: fmt.Sprintf("🚫 Stop Loss triggered for %s. Positions closed with loss %.2f USDT",
			ps.Config.Symbol, ps.Runtime.UnrealizedPnl),
		Meta: map[string]interface{}{
			"symbol":       ps.Config.Symbol,
			"pnl":          ps.Runtime.UnrealizedPnl,
			"stop_loss":    ps.Config.StopLoss,
			"realized_pnl": ps.Runtime.RealizedPnl,
		},
	}

	select {
	case rm.notificationChan <- notif:
	default:
		// Канал заполнен
	}
}

// notifyLiquidation отправляет уведомление о ликвидации
func (rm *RiskManager) notifyLiquidation(ps *PairState, event LiquidationEvent) {
	if rm.notificationChan == nil {
		return
	}

	pairID := ps.Config.ID
	notif := &models.Notification{
		Timestamp: time.Now(),
		Type:      models.NotificationTypeLiquidation,
		Severity:  models.SeverityError,
		PairID:    &pairID,
		Message: fmt.Sprintf("💥 Position LIQUIDATED on %s (%s %s). Second leg closed.",
			event.Exchange, event.Symbol, event.Side),
		Meta: map[string]interface{}{
			"symbol":     event.Symbol,
			"exchange":   event.Exchange,
			"side":       event.Side,
			"liquidated": true,
		},
	}

	select {
	case rm.notificationChan <- notif:
	default:
	}
}

// notifyMarginInsufficient отправляет уведомление о недостатке маржи
func (rm *RiskManager) notifyMarginInsufficient(ps *PairState, check *MarginCheck) {
	if rm.notificationChan == nil {
		return
	}

	pairID := ps.Config.ID
	notif := &models.Notification{
		Timestamp: time.Now(),
		Type:      models.NotificationTypeMargin,
		Severity:  models.SeverityWarn,
		PairID:    &pairID,
		Message: fmt.Sprintf("⚠️ Insufficient margin on %s for %s. Required: %.2f, Available: %.2f, Deficit: %.2f USDT",
			check.Exchange, ps.Config.Symbol, check.RequiredMargin, check.AvailableMargin, check.Deficit),
		Meta: map[string]interface{}{
			"symbol":           ps.Config.Symbol,
			"exchange":         check.Exchange,
			"required_margin":  check.RequiredMargin,
			"available_margin": check.AvailableMargin,
			"deficit":          check.Deficit,
		},
	}

	select {
	case rm.notificationChan <- notif:
	default:
	}
}

// notifyError отправляет уведомление об ошибке
func (rm *RiskManager) notifyError(ps *PairState, err error) {
	if rm.notificationChan == nil || err == nil {
		return
	}

	pairID := ps.Config.ID
	notif := &models.Notification{
		Timestamp: time.Now(),
		Type:      models.NotificationTypeError,
		Severity:  models.SeverityError,
		PairID:    &pairID,
		Message:   fmt.Sprintf("❌ Risk error for %s: %v", ps.Config.Symbol, err),
		Meta: map[string]interface{}{
			"symbol": ps.Config.Symbol,
			"error":  err.Error(),
		},
	}

	select {
	case rm.notificationChan <- notif:
	default:
	}
}

// ============================================================
// Периодический мониторинг рисков
// ============================================================

// RiskMonitor - воркер для периодической проверки рисков
type RiskMonitor struct {
	rm          *RiskManager
	getPairs    func() []*PairState
	stopCh      chan struct{}
	interval    time.Duration
}

// NewRiskMonitor создает монитор рисков
func NewRiskMonitor(rm *RiskManager, getPairs func() []*PairState) *RiskMonitor {
	return &RiskMonitor{
		rm:       rm,
		getPairs: getPairs,
		stopCh:   make(chan struct{}),
		interval: rm.config.CheckInterval,
	}
}

// Start запускает мониторинг
func (mon *RiskMonitor) Start(ctx context.Context) {
	ticker := time.NewTicker(mon.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-mon.stopCh:
			return
		case <-ticker.C:
			mon.checkAllRisks(ctx)
		}
	}
}

// Stop останавливает мониторинг
func (mon *RiskMonitor) Stop() {
	close(mon.stopCh)
}

// checkAllRisks проверяет риски для всех активных пар
func (mon *RiskMonitor) checkAllRisks(ctx context.Context) {
	pairs := mon.getPairs()

	for _, ps := range pairs {
		// Проверяем только пары в HOLDING
		ps.mu.RLock()
		state := ps.Runtime.State
		ps.mu.RUnlock()

		if state != models.StateHolding {
			continue
		}

		// Проверяем Stop Loss
		shouldClose, pnl := mon.rm.CheckStopLoss(ps)
		if shouldClose {
			// Записываем метрику
			RecordTrade(ps.Config.Symbol, "stop_loss", pnl)
			StopLossTriggered.WithLabelValues(ps.Config.Symbol).Inc()

			// Обрабатываем SL
			if err := mon.rm.HandleStopLoss(ctx, ps); err != nil {
				// Ошибка уже отправлена в уведомления
				continue
			}
		}
	}
}

// ============================================================
// Утилиты
// ============================================================

// ClearMarginCache очищает кэш маржи
func (rm *RiskManager) ClearMarginCache() {
	rm.marginCache = sync.Map{}
}

// ClearLimitsCache очищает кэш лимитов
func (rm *RiskManager) ClearLimitsCache() {
	rm.limitsCache = sync.Map{}
}

// GetCachedMargin возвращает кэшированную информацию о марже
func (rm *RiskManager) GetCachedMargin(exchangeName, symbol string) *MarginInfo {
	if cached, ok := rm.marginCache.Load(MarginKey{Exchange: exchangeName, Symbol: symbol}); ok {
		return cached.(*MarginInfo)
	}
	return nil
}

// GetCachedLimits возвращает кэшированные лимиты
func (rm *RiskManager) GetCachedLimits(exchangeName, symbol string) *exchange.Limits {
	if cached, ok := rm.limitsCache.Load(LimitsKey{Exchange: exchangeName, Symbol: symbol}); ok {
		return cached.(*exchange.Limits)
	}
	return nil
}

// PreloadLimits предзагружает лимиты для списка символов
func (rm *RiskManager) PreloadLimits(ctx context.Context, symbols []string) error {
	rm.exchMu.RLock()
	exchanges := make(map[string]exchange.Exchange, len(rm.exchanges))
	for name, exch := range rm.exchanges {
		exchanges[name] = exch
	}
	rm.exchMu.RUnlock()

	var wg sync.WaitGroup
	errChan := make(chan error, len(exchanges)*len(symbols))

	for exchName, exch := range exchanges {
		for _, symbol := range symbols {
			wg.Add(1)
			go func(name string, ex exchange.Exchange, sym string) {
				defer wg.Done()

				limits, err := ex.GetLimits(ctx, sym)
				if err != nil {
					errChan <- fmt.Errorf("%s/%s: %w", name, sym, err)
					return
				}

				rm.limitsCache.Store(LimitsKey{Exchange: name, Symbol: sym}, limits)
			}(exchName, exch, symbol)
		}
	}

	wg.Wait()
	close(errChan)

	// Собираем ошибки (не критично если некоторые не загрузились)
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("failed to preload some limits: %v", errors)
	}

	return nil
}
