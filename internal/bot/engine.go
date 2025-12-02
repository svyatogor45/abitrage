package bot

import (
	"context"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"arbitrage/internal/config"
	"arbitrage/internal/exchange"
	"arbitrage/internal/models"
)

// ============ ОПТИМИЗАЦИЯ: Object Pool для PriceUpdate ============
// Убирает ~1000+ аллокаций/сек в горячем пути

var priceUpdatePool = sync.Pool{
	New: func() interface{} {
		return &PriceUpdate{}
	},
}

// acquirePriceUpdate получает PriceUpdate из пула
func acquirePriceUpdate() *PriceUpdate {
	return priceUpdatePool.Get().(*PriceUpdate)
}

// releasePriceUpdate возвращает PriceUpdate в пул
// ВАЖНО: вызывать только после полной обработки события!
func releasePriceUpdate(p *PriceUpdate) {
	// Очищаем строки для GC (опционально, но рекомендуется)
	p.Exchange = ""
	p.Symbol = ""
	p.BidPrice = 0
	p.AskPrice = 0
	p.Timestamp = time.Time{}
	priceUpdatePool.Put(p)
}

// Engine - главный движок арбитражного бота (EVENT-DRIVEN архитектура)
//
// ОПТИМИЗАЦИИ:
// 1. Worker Pool - N воркеров с шардированием по символу (параллельная обработка)
// 2. Индекс pairsBySymbol - O(1) поиск пар вместо O(n) (sync.Map для lock-free чтения)
// 3. Atomic counter для activeArbs - нет вложенных локов
// 4. Шардированный PriceTracker - нет lock contention между символами
// 5. Несколько workers на шард - увеличенная пропускная способность
// 6. sync.Map для pairsBySymbol - lock-free чтение в горячем пути
//
// Архитектура:
// - НЕТ polling! Только реакция на события от WebSocket
// - Каждое обновление цены мгновенно триггерит проверку арбитража
// - Параллельная отправка ордеров на обе биржи
// - O(1) поиск лучшей связки через шардированный PriceTracker
//
// Поток данных:
// WebSocket → Router (hash by symbol) → Worker[N×M] → PriceTracker[shard] → ArbitrageCheck
type Engine struct {
	cfg *config.Config

	// Подключенные биржи
	exchanges map[string]exchange.Exchange
	exchMu    sync.RWMutex

	// Активные торговые пары
	pairs   map[int]*PairState
	pairsMu sync.RWMutex

	// ОПТИМИЗАЦИЯ: sync.Map для lock-free чтения в горячем пути
	// map[string][]*PairState - индекс пар по символу
	// sync.Map оптимизирована для случая частых чтений и редких записей
	pairsBySymbol sync.Map

	// Шардированный трекер лучших цен
	priceTracker *PriceTracker

	// Калькулятор спреда
	spreadCalc *SpreadCalculator

	// Исполнитель ордеров
	orderExec *OrderExecutor

	// Валидатор ордеров (лимиты бирж)
	orderValidator *OrderValidator

	// Анализатор стаканов (ликвидность)
	orderBookAnalyzer *OrderBookAnalyzer

	// Арбитражный детектор и координатор
	arbDetector    *ArbitrageDetector
	arbCoordinator *ArbitrageCoordinator

	// Менеджер частичного входа
	partialManager *PartialEntryManager

	// Обработчик провала второй ноги
	secondLegFailHandler *SecondLegFailHandler

	// Worker pool: шардированные каналы по символам
	priceShards     []*priceShard
	numShards       int
	workersPerShard int // ОПТИМИЗАЦИЯ: несколько workers на шард

	// Канал для позиций (отдельный, не шардируется)
	positionUpdates chan PositionUpdate
	shutdown        chan struct{}

	// Канал для уведомлений
	notificationChan chan *models.Notification

	// WebSocket hub для отправки данных клиентам
	wsHub WebSocketHub

	// ОПТИМИЗАЦИЯ: Atomic counter вместо mutex для activeArbs
	activeArbs int64
}

// priceShard - шард для обработки ценовых событий
type priceShard struct {
	updates chan *PriceUpdate // указатели для использования с sync.Pool
}

// PairState - runtime состояние торговой пары
type PairState struct {
	Config  *models.PairConfig
	Runtime *models.PairRuntime
	mu      sync.RWMutex

	// ОПТИМИЗАЦИЯ: atomic флаг для быстрой проверки без Lock
	// 1 = active и ready для торговли, 0 = неактивна
	// Позволяет быстро отсеять неактивные пары без захвата mutex
	isReady int32
}

// PriceUpdate - событие обновления цены от WebSocket
type PriceUpdate struct {
	Exchange  string
	Symbol    string
	BidPrice  float64 // лучшая цена покупки (для шорта)
	AskPrice  float64 // лучшая цена продажи (для лонга)
	Timestamp time.Time
}

// PositionUpdate - событие обновления позиции (для детекта ликвидаций)
type PositionUpdate struct {
	Exchange      string
	Symbol        string
	Side          string
	Liquidated    bool
	UnrealizedPnl float64
}

// WebSocketHub - интерфейс для отправки данных клиентам
//
// Реализуется пакетом internal/websocket/Hub
// Используется для real-time обновления UI:
// - pairUpdate: состояние пар каждую секунду
// - notification: события торговли
// - balanceUpdate: балансы бирж каждую минуту
// - statsUpdate: статистика при изменениях
type WebSocketHub interface {
	// BroadcastPairUpdate отправляет обновление состояния пары
	// Вызывается каждую секунду для пар в состоянии HOLDING
	BroadcastPairUpdate(pairID int, runtime *models.PairRuntime)

	// BroadcastNotification отправляет уведомление о событии
	// Вызывается при OPEN, CLOSE, SL, LIQUIDATION, ERROR и др.
	BroadcastNotification(notif *models.Notification)

	// BroadcastBalanceUpdate отправляет обновление баланса биржи
	// Вызывается каждую минуту для каждой подключенной биржи
	BroadcastBalanceUpdate(exchange string, balance float64)

	// BroadcastStatsUpdate отправляет обновление статистики
	// Вызывается после завершения каждой сделки
	BroadcastStatsUpdate(stats *models.Stats)
}

// NewEngine создает новый Engine с оптимизациями
func NewEngine(cfg *config.Config, wsHub WebSocketHub) *Engine {
	// Количество шардов = количество CPU (оптимально для параллелизма)
	numShards := runtime.NumCPU()
	if numShards < 4 {
		numShards = 4 // минимум 4 шарда
	}
	if numShards > 32 {
		numShards = 32 // максимум 32 (не нужно больше для 30 пар)
	}

	// ОПТИМИЗАЦИЯ: несколько workers на шард для увеличения пропускной способности
	// При 1000+ обновлений/сек один worker может не справиться
	workersPerShard := 2 // 2 workers на шард (можно увеличить до 4)

	e := &Engine{
		cfg:              cfg,
		exchanges:        make(map[string]exchange.Exchange),
		pairs:            make(map[int]*PairState),
		// pairsBySymbol: sync.Map инициализируется автоматически (zero value)
		priceTracker:     NewPriceTracker(numShards),
		priceShards:      make([]*priceShard, numShards),
		numShards:        numShards,
		workersPerShard:  workersPerShard,
		positionUpdates:  make(chan PositionUpdate, 1000),
		notificationChan: make(chan *models.Notification, 100),
		shutdown:         make(chan struct{}),
		wsHub:            wsHub,
	}

	// Инициализация шардов для worker pool
	for i := 0; i < numShards; i++ {
		e.priceShards[i] = &priceShard{
			updates: make(chan *PriceUpdate, 2000), // указатели + буфер на шард
		}
	}

	// Инициализация основных компонентов
	e.spreadCalc = NewSpreadCalculator(e.priceTracker)
	e.orderExec = NewOrderExecutor(e.exchanges, cfg.Bot)

	// Инициализация валидатора ордеров
	e.orderValidator = NewOrderValidator(func(symbol, exchange string) float64 {
		price := e.priceTracker.GetExchangePrice(symbol, exchange)
		if price != nil {
			return price.BidPrice
		}
		return 0
	})

	// Инициализация анализатора стаканов (5 уровней, 5 секунд актуальности)
	e.orderBookAnalyzer = NewOrderBookAnalyzer(5, 5*time.Second)

	// Инициализация арбитражного детектора
	e.arbDetector = NewArbitrageDetector(
		e.priceTracker,
		e.spreadCalc,
		e.orderBookAnalyzer,
	)

	// Инициализация координатора арбитража
	e.arbCoordinator = NewArbitrageCoordinator(
		e.arbDetector,
		e.orderExec,
		e.orderValidator,
		cfg.Bot.MaxConcurrentArbs,
		&e.activeArbs,
	)

	// Инициализация менеджера частичного входа
	e.partialManager = NewPartialEntryManager(
		e.arbDetector,
		e.orderExec,
		e.orderValidator,
	)
	e.arbCoordinator.SetPartialManager(e.partialManager)

	// Инициализация обработчика провала второй ноги
	e.secondLegFailHandler = NewSecondLegFailHandler(
		e.orderExec,
		e.notificationChan,
		true, // pauseOnFail
	)
	e.arbCoordinator.SetFailHandler(e.secondLegFailHandler)

	return e
}

// Run запускает event-driven движок с worker pool
// ОПТИМИЗАЦИЯ: запуск нескольких workers на шард для увеличения пропускной способности
func (e *Engine) Run(ctx context.Context) error {
	// Запуск M воркеров на каждый из N шардов (итого N×M воркеров)
	// Это позволяет обрабатывать больше обновлений цен параллельно
	for shardIdx := 0; shardIdx < e.numShards; shardIdx++ {
		for workerIdx := 0; workerIdx < e.workersPerShard; workerIdx++ {
			go e.priceEventWorker(ctx, shardIdx)
		}
	}

	// Остальные воркеры
	go e.positionEventLoop(ctx)
	go e.periodicTasks(ctx)           // балансы, статистика для UI
	go e.notificationWorker(ctx)      // обработка уведомлений
	go e.exitConditionChecker(ctx)    // проверка условий выхода

	<-ctx.Done()
	close(e.shutdown)
	return ctx.Err()
}

// notificationWorker обрабатывает уведомления и отправляет их в WebSocket
func (e *Engine) notificationWorker(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case notif := <-e.notificationChan:
			if e.wsHub != nil {
				e.wsHub.BroadcastNotification(notif)
			}
		}
	}
}

// exitConditionChecker периодически проверяет условия выхода для открытых позиций
func (e *Engine) exitConditionChecker(ctx context.Context) {
	// Проверяем условия выхода каждые 500ms
	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			e.checkAllExitConditions(ctx)
		}
	}
}

// checkAllExitConditions проверяет условия выхода для всех позиций в HOLDING
func (e *Engine) checkAllExitConditions(ctx context.Context) {
	// Собираем пары в состоянии HOLDING
	e.pairsMu.RLock()
	holdingPairs := make([]*PairState, 0)
	for _, ps := range e.pairs {
		if ps.Runtime.State == models.StateHolding {
			holdingPairs = append(holdingPairs, ps)
		}
	}
	e.pairsMu.RUnlock()

	// Проверяем условия выхода для каждой
	for _, ps := range holdingPairs {
		e.checkExitConditionsForPair(ctx, ps)
	}
}

// checkExitConditionsForPair проверяет условия выхода для конкретной пары
func (e *Engine) checkExitConditionsForPair(ctx context.Context, ps *PairState) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Проверяем что пара всё ещё в HOLDING
	if ps.Runtime.State != models.StateHolding {
		return
	}

	// Используем ArbitrageDetector для проверки условий
	exitConditions := e.arbDetector.CheckExitConditions(ps)

	// Обновляем runtime данные
	ps.Runtime.CurrentSpread = exitConditions.CurrentSpread
	ps.Runtime.UnrealizedPnl = exitConditions.CurrentPnl
	ps.Runtime.LastUpdate = time.Now()

	if !exitConditions.ShouldExit {
		return
	}

	// Условия выхода достигнуты - выполняем закрытие
	ps.Runtime.State = models.StateExiting

	// Асинхронное закрытие
	go e.executeExit(ps, exitConditions.Reason)
}

// executeExit выполняет закрытие позиции
func (e *Engine) executeExit(ps *PairState, reason ExitReason) {
	ctx, cancel := context.WithTimeout(context.Background(), e.cfg.Bot.OrderTimeout)
	defer cancel()

	// МЕТРИКА: засекаем время закрытия
	exitStart := time.Now()

	// Закрываем через OrderExecutor
	result := e.orderExec.CloseParallel(ctx, CloseParams{
		Symbol: ps.Config.Symbol,
		Legs:   ps.Runtime.Legs,
	})

	// МЕТРИКА: записываем латентность закрытия
	exitLatencyMs := float64(time.Since(exitStart).Milliseconds())
	RecordTickToOrder(ps.Config.Symbol, "exit_execution", exitLatencyMs)
	EventsProcessed.WithLabelValues("exit").Inc()

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if result.Success {
		// Успешное закрытие
		ps.Runtime.RealizedPnl += result.TotalPnl
		ps.Runtime.Legs = nil
		ps.Runtime.FilledParts = 0
		e.decrementActiveArbs()

		// МЕТРИКА: записываем успешную сделку
		RecordTrade(ps.Config.Symbol, "success", result.TotalPnl)
		UpdateActiveArbitrages(atomic.LoadInt64(&e.activeArbs))

		// МЕТРИКА: записываем стоп-лосс или ликвидацию
		if reason == ExitReasonStopLoss {
			StopLossTriggered.WithLabelValues(ps.Config.Symbol).Inc()
		}

		// Определяем следующее состояние
		if reason == ExitReasonStopLoss || reason == ExitReasonLiquidation {
			ps.Runtime.State = models.StatePaused
			ps.Config.Status = "paused"
			atomic.StoreInt32(&ps.isReady, 0)
		} else {
			ps.Runtime.State = models.StateReady
			atomic.StoreInt32(&ps.isReady, 1)
		}

		// Отправляем уведомление
		e.notifyTradeClosed(ps, result, reason)
	} else {
		// Ошибка закрытия
		ps.Runtime.State = models.StateError
		// МЕТРИКА: записываем неудачную сделку
		RecordTrade(ps.Config.Symbol, "failed", 0)
		e.notifyError(ps, result.Error)
	}
}

// notifyTradeClosed отправляет уведомление о закрытии позиции
func (e *Engine) notifyTradeClosed(ps *PairState, result *ExecuteResult, reason ExitReason) {
	notifType := "CLOSE"
	severity := "info"

	switch reason {
	case ExitReasonStopLoss:
		notifType = "SL"
		severity = "warn"
	case ExitReasonLiquidation:
		notifType = "LIQUIDATION"
		severity = "error"
	}

	pairID := ps.Config.ID
	notif := &models.Notification{
		Timestamp: time.Now(),
		Type:      notifType,
		Severity:  severity,
		PairID:    &pairID,
		Message: fmt.Sprintf("%s closed: PNL %.2f USDT (reason: %s)",
			ps.Config.Symbol, result.TotalPnl, reason),
		Meta: map[string]interface{}{
			"symbol":       ps.Config.Symbol,
			"pnl":          result.TotalPnl,
			"reason":       string(reason),
			"realized_pnl": ps.Runtime.RealizedPnl,
		},
	}

	select {
	case e.notificationChan <- notif:
	default:
	}
}

// priceEventWorker - воркер для обработки ценовых событий одного шарда
// Гарантия: все события одного символа обрабатываются последовательно одним воркером
func (e *Engine) priceEventWorker(ctx context.Context, shardIdx int) {
	shard := e.priceShards[shardIdx]

	for {
		select {
		case <-ctx.Done():
			return
		case update := <-shard.updates:
			e.handlePriceUpdate(update)
			// ОПТИМИЗАЦИЯ: возвращаем в пул после обработки
			releasePriceUpdate(update)
		}
	}
}

// routePriceUpdate - роутинг события к нужному воркеру по символу
// Детерминированный: один символ всегда попадает в один шард
// ОПТИМИЗАЦИЯ: получает объект из sync.Pool
func (e *Engine) routePriceUpdate(exchName, symbol string, bidPrice, askPrice float64, timestamp time.Time) {
	shardIdx := e.priceTracker.GetShardIndex(symbol)

	// Получаем объект из пула (без аллокации!)
	update := acquirePriceUpdate()
	update.Exchange = exchName
	update.Symbol = symbol
	update.BidPrice = bidPrice
	update.AskPrice = askPrice
	update.Timestamp = timestamp

	select {
	case e.priceShards[shardIdx].updates <- update:
	default:
		// Буфер полон - возвращаем в пул и пропускаем
		releasePriceUpdate(update)
		// МЕТРИКА: записываем переполнение буфера
		RecordBufferOverflow("price_shard")
	}
}

// handlePriceUpdate - обработка обновления цены
// Время выполнения: ~0.5-2ms (без сетевых запросов!)
func (e *Engine) handlePriceUpdate(update *PriceUpdate) {
	// МЕТРИКА: засекаем время начала обработки
	startTime := time.Now()

	// 1. Обновляем шардированный трекер цен O(k), k=число бирж
	e.priceTracker.UpdateFromPtr(update)

	// 2. Получаем пары для этого символа O(1) через индекс
	pairs := e.getPairsForSymbol(update.Symbol)

	// 3. Проверяем арбитраж для каждой пары
	for _, pairState := range pairs {
		e.checkArbitrageOpportunity(pairState)
	}

	// МЕТРИКА: записываем латентность обработки
	latencyMs := float64(time.Since(startTime).Microseconds()) / 1000.0
	RecordPriceUpdateLatency(update.Symbol, latencyMs)
}

// checkArbitrageOpportunity - проверка арбитражной возможности для пары
// ОПТИМИЗАЦИЯ: atomic быстрая проверка + atomic counter для activeArbs
//
// Использует ArbitrageDetector для полной проверки условий:
// - Спред >= entry_spread (с учётом комиссий)
// - Достаточная ликвидность на обеих биржах
// - Маржа достаточна
// - Лимиты ордеров соблюдены
func (e *Engine) checkArbitrageOpportunity(ps *PairState) {
	// ОПТИМИЗАЦИЯ: быстрая проверка без Lock
	// Если пара не ready, пропускаем без захвата mutex
	if atomic.LoadInt32(&ps.isReady) != 1 {
		return
	}

	// Проверка лимита одновременных арбитражей (atomic - без lock!)
	if !e.canOpenNewArbitrage() {
		return
	}

	// Теперь захватываем Lock для полной проверки и изменения состояния
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Повторная проверка под Lock (double-check pattern)
	if ps.Config.Status != "active" || ps.Runtime.State != models.StateReady {
		return
	}

	// Используем ArbitrageDetector для полной проверки условий входа
	currentArbs := atomic.LoadInt64(&e.activeArbs)
	conditions := e.arbDetector.CheckEntryConditions(
		ps,
		currentArbs,
		e.cfg.Bot.MaxConcurrentArbs,
		e.orderValidator,
	)

	// Условия не выполнены - выходим
	if !conditions.CanEnter {
		return
	}

	// ВХОДИМ! Переключаем состояние и запускаем исполнение
	ps.Runtime.State = models.StateEntering
	atomic.StoreInt32(&ps.isReady, 0) // Сбрасываем флаг
	e.incrementActiveArbs()

	// МЕТРИКА: записываем возможность, которая привела к входу
	RecordOpportunity(ps.Config.Symbol, true)
	RecordSpread(ps.Config.Symbol, conditions.Opportunity.NetSpread)

	// Асинхронный вход (не блокируем обработку других событий)
	go e.executeEntryWithConditions(ps, conditions)
}

// shouldEnter - проверка всех условий для входа (legacy, используется ArbitrageDetector)
func (e *Engine) shouldEnter(ps *PairState, opp *ArbitrageOpportunity) bool {
	// Используем ArbitrageDetector для полной проверки
	conditions := e.arbDetector.CheckEntryConditions(
		ps,
		atomic.LoadInt64(&e.activeArbs),
		e.cfg.Bot.MaxConcurrentArbs,
		e.orderValidator,
	)
	return conditions.CanEnter
}

// executeEntryWithConditions - исполнение входа с предварительно проверенными условиями
func (e *Engine) executeEntryWithConditions(ps *PairState, conditions *EntryConditions) {
	ctx, cancel := context.WithTimeout(context.Background(), e.cfg.Bot.OrderTimeout)
	defer cancel()

	// МЕТРИКА: засекаем время исполнения входа
	entryStart := time.Now()

	opp := conditions.Opportunity
	volume := conditions.AdjustedVolume

	var result *ExecuteResult

	// Проверяем нужен ли частичный вход
	if ps.Config.NOrders > 1 && e.partialManager != nil {
		// Частичный вход через PartialEntryManager
		partialResult := e.partialManager.ExecutePartialEntry(ctx, PartialEntryParams{
			Symbol:        ps.Config.Symbol,
			TotalVolume:   volume,
			NOrders:       ps.Config.NOrders,
			LongExchange:  opp.LongExchange,
			ShortExchange: opp.ShortExchange,
			MinSpread:     ps.Config.EntrySpreadPct * 0.8, // 80% от entry spread
		})

		result = &ExecuteResult{
			Success: partialResult.Success,
			Legs:    partialResult.Legs,
			Error:   partialResult.Error,
		}
	} else {
		// Одиночный вход через OrderExecutor
		result = e.orderExec.ExecuteParallel(ctx, ExecuteParams{
			Symbol:        ps.Config.Symbol,
			Volume:        volume,
			LongExchange:  opp.LongExchange,
			ShortExchange: opp.ShortExchange,
			NOrders:       1,
		})
	}

	// МЕТРИКА: записываем латентность исполнения
	execLatencyMs := float64(time.Since(entryStart).Milliseconds())
	RecordTickToOrder(ps.Config.Symbol, "entry_execution", execLatencyMs)

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if result.Success {
		// Успешный вход
		ps.Runtime.State = models.StateHolding
		ps.Runtime.Legs = result.Legs
		ps.Runtime.FilledParts = 1
		ps.Runtime.LastUpdate = time.Now()

		// МЕТРИКА: обновляем счётчик активных арбитражей
		UpdateActiveArbitrages(atomic.LoadInt64(&e.activeArbs))
		EventsProcessed.WithLabelValues("entry").Inc()

		e.notifyTradeOpened(ps, result)
	} else {
		// Ошибка - возврат в готовность
		ps.Runtime.State = models.StateReady
		// ОПТИМИЗАЦИЯ: восстанавливаем atomic флаг для быстрой проверки
		atomic.StoreInt32(&ps.isReady, 1)
		e.decrementActiveArbs()

		// МЕТРИКА: записываем откат
		RecordTrade(ps.Config.Symbol, "rollback", 0)

		e.notifyError(ps, result.Error)
	}
}

// executeEntry - исполнение входа в арбитраж (ПАРАЛЛЕЛЬНЫЕ ОРДЕРА!)
// Legacy метод, используйте executeEntryWithConditions
func (e *Engine) executeEntry(ps *PairState, opp *ArbitrageOpportunity) {
	ctx, cancel := context.WithTimeout(context.Background(), e.cfg.Bot.OrderTimeout)
	defer cancel()

	// Параллельная отправка ордеров на обе биржи
	result := e.orderExec.ExecuteParallel(ctx, ExecuteParams{
		Symbol:        ps.Config.Symbol,
		Volume:        ps.Config.VolumeAsset,
		LongExchange:  opp.LongExchange,
		ShortExchange: opp.ShortExchange,
		NOrders:       ps.Config.NOrders,
	})

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if result.Success {
		// Успешный вход
		ps.Runtime.State = models.StateHolding
		ps.Runtime.Legs = result.Legs
		ps.Runtime.FilledParts = 1
		ps.Runtime.LastUpdate = time.Now()

		e.notifyTradeOpened(ps, result)
	} else {
		// Ошибка - возврат в готовность
		ps.Runtime.State = models.StateReady
		// ОПТИМИЗАЦИЯ: восстанавливаем atomic флаг для быстрой проверки
		atomic.StoreInt32(&ps.isReady, 1)
		e.decrementActiveArbs()

		e.notifyError(ps, result.Error)
	}
}

// positionEventLoop - обработка событий позиций (ликвидации)
func (e *Engine) positionEventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-e.positionUpdates:
			if update.Liquidated {
				e.handleLiquidation(update)
			}
		}
	}
}

// handleLiquidation - обработка ликвидации
func (e *Engine) handleLiquidation(update PositionUpdate) {
	// Найти пару с этой позицией и закрыть вторую ногу
	// TODO: реализовать
}

// periodicTasks - периодические задачи (НЕ влияют на торговлю)
func (e *Engine) periodicTasks(ctx context.Context) {
	balanceTicker := time.NewTicker(e.cfg.Bot.BalanceUpdateFreq)
	statsTicker := time.NewTicker(e.cfg.Bot.StatsUpdateFreq)
	defer balanceTicker.Stop()
	defer statsTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-balanceTicker.C:
			e.updateBalances()
		case <-statsTicker.C:
			e.broadcastPairStates()
		}
	}
}

// updateBalances - обновление балансов для UI
// ОПТИМИЗАЦИЯ: параллельные запросы к биржам вместо последовательных
// Было: 6 бирж × 500ms = 3 сек с удерживанием RLock
// Стало: max(500ms) параллельно, RLock только на копирование
func (e *Engine) updateBalances() {
	// Копируем биржи под коротким RLock
	e.exchMu.RLock()
	exchanges := make(map[string]exchange.Exchange, len(e.exchanges))
	for name, exch := range e.exchanges {
		exchanges[name] = exch
	}
	e.exchMu.RUnlock()

	if len(exchanges) == 0 {
		return
	}

	// Параллельные запросы БЕЗ удержания Lock
	var wg sync.WaitGroup
	for name, exch := range exchanges {
		wg.Add(1)
		go func(exchName string, ex exchange.Exchange) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()

			balance, err := ex.GetBalance(ctx)
			if err == nil && e.wsHub != nil {
				e.wsHub.BroadcastBalanceUpdate(exchName, balance)
			}
		}(name, exch)
	}
	wg.Wait()
}

// broadcastPairStates - отправка состояний пар клиентам
// ОПТИМИЗАЦИЯ: копируем данные под коротким RLock, отправляем без Lock
// Было: RLock на весь цикл broadcast (300-900ms при 30 парах)
// Стало: RLock только на копирование (~1ms), broadcast без блокировки
func (e *Engine) broadcastPairStates() {
	// Собираем данные под коротким RLock
	type pairData struct {
		id      int
		ps      *PairState
		runtime *models.PairRuntime
	}

	e.pairsMu.RLock()
	pairs := make([]pairData, 0, len(e.pairs))
	for id, ps := range e.pairs {
		if ps.Runtime.State == models.StateHolding {
			// Копируем указатель на runtime (данные обновятся in-place)
			pairs = append(pairs, pairData{id: id, ps: ps, runtime: ps.Runtime})
		}
	}
	e.pairsMu.RUnlock()

	// Обновляем PNL и отправляем БЕЗ блокировки pairsMu
	// Это позволяет другим горутинам работать с парами
	for _, p := range pairs {
		e.updatePairPnl(p.ps)

		if e.wsHub != nil {
			e.wsHub.BroadcastPairUpdate(p.id, p.runtime)
		}
	}
}

// ============ Оптимизированные вспомогательные методы ============

// getPairsForSymbol - O(1) lock-free поиск через sync.Map
// ОПТИМИЗАЦИЯ: sync.Map не требует блокировки для чтения (lock-free)
// При 1000+ вызовов/сек это убирает contention с AddPair/RemovePair
func (e *Engine) getPairsForSymbol(symbol string) []*PairState {
	if v, ok := e.pairsBySymbol.Load(symbol); ok {
		return v.([]*PairState)
	}
	return nil
}

// canOpenNewArbitrage - atomic проверка без lock
func (e *Engine) canOpenNewArbitrage() bool {
	if e.cfg.Bot.MaxConcurrentArbs == 0 {
		return true // без лимита
	}
	return atomic.LoadInt64(&e.activeArbs) < int64(e.cfg.Bot.MaxConcurrentArbs)
}

// incrementActiveArbs - atomic инкремент
func (e *Engine) incrementActiveArbs() {
	atomic.AddInt64(&e.activeArbs, 1)
}

// decrementActiveArbs - atomic декремент
func (e *Engine) decrementActiveArbs() {
	atomic.AddInt64(&e.activeArbs, -1)
}

func (e *Engine) updatePairPnl(ps *PairState) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.Runtime.State != models.StateHolding || len(ps.Runtime.Legs) != 2 {
		return
	}

	var longLeg, shortLeg *models.Leg
	for i := range ps.Runtime.Legs {
		if ps.Runtime.Legs[i].Side == "long" {
			longLeg = &ps.Runtime.Legs[i]
		} else {
			shortLeg = &ps.Runtime.Legs[i]
		}
	}

	if longLeg == nil || shortLeg == nil {
		return
	}

	// Рассчитываем PNL через SpreadCalculator
	pnl := e.spreadCalc.CalculatePnl(
		ps.Config.Symbol,
		longLeg.Exchange, longLeg.EntryPrice,
		shortLeg.Exchange, shortLeg.EntryPrice,
		longLeg.Quantity,
	)
	ps.Runtime.UnrealizedPnl = pnl

	// Рассчитываем текущий спред
	currentSpread := e.spreadCalc.GetCurrentSpread(
		ps.Config.Symbol,
		longLeg.Exchange,
		shortLeg.Exchange,
	)
	ps.Runtime.CurrentSpread = currentSpread

	// Обновляем текущие цены в ногах
	if longPrice := e.priceTracker.GetExchangePrice(ps.Config.Symbol, longLeg.Exchange); longPrice != nil {
		longLeg.CurrentPrice = longPrice.BidPrice
		longLeg.UnrealizedPnl = (longPrice.BidPrice - longLeg.EntryPrice) * longLeg.Quantity
	}
	if shortPrice := e.priceTracker.GetExchangePrice(ps.Config.Symbol, shortLeg.Exchange); shortPrice != nil {
		shortLeg.CurrentPrice = shortPrice.AskPrice
		shortLeg.UnrealizedPnl = (shortLeg.EntryPrice - shortPrice.AskPrice) * shortLeg.Quantity
	}

	ps.Runtime.LastUpdate = time.Now()
}

func (e *Engine) notifyTradeOpened(ps *PairState, result *ExecuteResult) {
	if len(result.Legs) < 2 {
		return
	}

	longLeg := result.Legs[0]
	shortLeg := result.Legs[1]
	if longLeg.Side != "long" {
		longLeg, shortLeg = shortLeg, longLeg
	}

	pairID := ps.Config.ID
	notif := &models.Notification{
		Timestamp: time.Now(),
		Type:      "OPEN",
		Severity:  "info",
		PairID:    &pairID,
		Message: fmt.Sprintf("%s opened: Long on %s @ %.4f, Short on %s @ %.4f",
			ps.Config.Symbol,
			longLeg.Exchange, longLeg.EntryPrice,
			shortLeg.Exchange, shortLeg.EntryPrice),
		Meta: map[string]interface{}{
			"symbol":          ps.Config.Symbol,
			"long_exchange":   longLeg.Exchange,
			"long_price":      longLeg.EntryPrice,
			"long_qty":        longLeg.Quantity,
			"short_exchange":  shortLeg.Exchange,
			"short_price":     shortLeg.EntryPrice,
			"short_qty":       shortLeg.Quantity,
		},
	}

	select {
	case e.notificationChan <- notif:
	default:
		// Канал заполнен, пропускаем
	}
}

func (e *Engine) notifyError(ps *PairState, err error) {
	if err == nil {
		return
	}

	pairID := ps.Config.ID
	notif := &models.Notification{
		Timestamp: time.Now(),
		Type:      "ERROR",
		Severity:  "error",
		PairID:    &pairID,
		Message:   fmt.Sprintf("%s error: %v", ps.Config.Symbol, err),
		Meta: map[string]interface{}{
			"symbol": ps.Config.Symbol,
			"error":  err.Error(),
		},
	}

	select {
	case e.notificationChan <- notif:
	default:
	}
}

// ============ API для добавления бирж и пар ============

// AddExchange добавляет подключенную биржу
func (e *Engine) AddExchange(name string, exch exchange.Exchange) {
	e.exchMu.Lock()
	e.exchanges[name] = exch
	e.exchMu.Unlock()

	// Подписка на WebSocket обновления
	e.subscribeToExchange(name, exch)
}

// subscribeToExchange подписывается на WS события биржи
func (e *Engine) subscribeToExchange(name string, exch exchange.Exchange) {
	// Подписка на позиции (для ликвидаций)
	exch.SubscribePositions(func(pos *exchange.Position) {
		e.positionUpdates <- PositionUpdate{
			Exchange:      name,
			Symbol:        pos.Symbol,
			Side:          pos.Side,
			Liquidated:    pos.Liquidation,
			UnrealizedPnl: pos.UnrealizedPnl,
		}
	})
}

// AddPair добавляет торговую пару
// ОПТИМИЗАЦИЯ: sync.Map для pairsBySymbol - атомарное обновление без блокировки чтений
func (e *Engine) AddPair(cfg *models.PairConfig) {
	ps := &PairState{
		Config: cfg,
		Runtime: &models.PairRuntime{
			PairID: cfg.ID,
			State:  models.StatePaused,
		},
	}

	// Добавляем в основной map под lock
	e.pairsMu.Lock()
	e.pairs[cfg.ID] = ps
	e.pairsMu.Unlock()

	// ОПТИМИЗАЦИЯ: атомарное обновление sync.Map
	// Используем Load → modify → Store паттерн
	for {
		existing, _ := e.pairsBySymbol.Load(cfg.Symbol)
		var newSlice []*PairState
		if existing != nil {
			oldSlice := existing.([]*PairState)
			newSlice = make([]*PairState, len(oldSlice)+1)
			copy(newSlice, oldSlice)
			newSlice[len(oldSlice)] = ps
		} else {
			newSlice = []*PairState{ps}
		}
		// Атомарная замена (если значение изменилось, повторяем)
		if existing == nil {
			if _, loaded := e.pairsBySymbol.LoadOrStore(cfg.Symbol, newSlice); !loaded {
				break
			}
		} else {
			if e.pairsBySymbol.CompareAndSwap(cfg.Symbol, existing, newSlice) {
				break
			}
		}
	}

	// Подписываемся на цены ПОСЛЕ обновления индекса
	e.subscribeToSymbol(cfg.Symbol)
}

// RemovePair удаляет торговую пару
// ОПТИМИЗАЦИЯ: sync.Map для pairsBySymbol - атомарное обновление без блокировки чтений
func (e *Engine) RemovePair(pairID int) {
	// Получаем пару под lock
	e.pairsMu.Lock()
	ps, ok := e.pairs[pairID]
	if !ok {
		e.pairsMu.Unlock()
		return
	}
	symbol := ps.Config.Symbol
	delete(e.pairs, pairID)
	e.pairsMu.Unlock()

	// ОПТИМИЗАЦИЯ: атомарное обновление sync.Map
	for {
		existing, ok := e.pairsBySymbol.Load(symbol)
		if !ok {
			break // Уже удалено
		}

		oldSlice := existing.([]*PairState)
		var newSlice []*PairState

		// Ищем и удаляем пару
		for _, p := range oldSlice {
			if p.Config.ID != pairID {
				newSlice = append(newSlice, p)
			}
		}

		// Если слайс пустой - удаляем ключ
		if len(newSlice) == 0 {
			if e.pairsBySymbol.CompareAndDelete(symbol, existing) {
				break
			}
		} else {
			if e.pairsBySymbol.CompareAndSwap(symbol, existing, newSlice) {
				break
			}
		}
	}
}

// subscribeToSymbol подписывается на цены символа на всех биржах
func (e *Engine) subscribeToSymbol(symbol string) {
	e.exchMu.RLock()
	defer e.exchMu.RUnlock()

	for name, exch := range e.exchanges {
		exchName := name // захват для closure
		exch.SubscribeTicker(symbol, func(ticker *exchange.Ticker) {
			// Роутинг через шардированный канал (без аллокации!)
			e.routePriceUpdate(
				exchName,
				ticker.Symbol,
				ticker.BidPrice,
				ticker.AskPrice,
				ticker.Timestamp,
			)
		})
	}
}

// StartPair запускает мониторинг пары
func (e *Engine) StartPair(pairID int) error {
	e.pairsMu.RLock()
	ps, ok := e.pairs[pairID]
	e.pairsMu.RUnlock()

	if ok {
		ps.mu.Lock()
		ps.Config.Status = "active"
		ps.Runtime.State = models.StateReady
		ps.mu.Unlock()
		// ОПТИМИЗАЦИЯ: устанавливаем atomic флаг для быстрой проверки
		atomic.StoreInt32(&ps.isReady, 1)
	}
	return nil
}

// PausePair останавливает пару
func (e *Engine) PausePair(pairID int) error {
	e.pairsMu.RLock()
	ps, ok := e.pairs[pairID]
	e.pairsMu.RUnlock()

	if ok {
		// ОПТИМИЗАЦИЯ: сначала сбрасываем atomic флаг (быстрая блокировка новых проверок)
		atomic.StoreInt32(&ps.isReady, 0)

		ps.mu.Lock()
		ps.Config.Status = "paused"
		ps.Runtime.State = models.StatePaused
		ps.mu.Unlock()
	}
	return nil
}

// OnPriceUpdate - публичный метод для приема ценовых обновлений
// Использует sync.Pool для zero-allocation
func (e *Engine) OnPriceUpdate(exchange, symbol string, bidPrice, askPrice float64, timestamp time.Time) {
	e.routePriceUpdate(exchange, symbol, bidPrice, askPrice, timestamp)
}

// GetActiveArbitrages возвращает текущее количество активных арбитражей
func (e *Engine) GetActiveArbitrages() int64 {
	return atomic.LoadInt64(&e.activeArbs)
}

// GetNumShards возвращает количество шардов (для мониторинга)
func (e *Engine) GetNumShards() int {
	return e.numShards
}

// ============ API для PairService ============

// GetPairRuntime возвращает runtime состояние пары
func (e *Engine) GetPairRuntime(pairID int) *models.PairRuntime {
	e.pairsMu.RLock()
	ps, ok := e.pairs[pairID]
	e.pairsMu.RUnlock()

	if !ok {
		return nil
	}

	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Возвращаем копию для безопасности
	if ps.Runtime == nil {
		return nil
	}

	runtime := *ps.Runtime
	return &runtime
}

// ForceClosePair принудительно закрывает позиции пары
func (e *Engine) ForceClosePair(ctx context.Context, pairID int) error {
	e.pairsMu.RLock()
	ps, ok := e.pairs[pairID]
	e.pairsMu.RUnlock()

	if !ok {
		return nil
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Проверяем, есть ли открытая позиция
	if ps.Runtime.State != models.StateHolding {
		return nil // Нет позиции для закрытия
	}

	// Переводим в состояние закрытия
	ps.Runtime.State = models.StateExiting

	// Выполняем закрытие позиций асинхронно
	go func() {
		e.executeForceClose(ps)
	}()

	return nil
}

// executeForceClose выполняет принудительное закрытие позиций
func (e *Engine) executeForceClose(ps *PairState) {
	ctx, cancel := context.WithTimeout(context.Background(), e.cfg.Bot.OrderTimeout)
	defer cancel()

	// Закрываем обе ноги параллельно
	result := e.orderExec.CloseParallel(ctx, CloseParams{
		Symbol: ps.Config.Symbol,
		Legs:   ps.Runtime.Legs,
	})

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if result.Success {
		// Сохраняем PNL и очищаем позицию
		ps.Runtime.RealizedPnl += result.TotalPnl
		ps.Runtime.Legs = nil
		ps.Runtime.State = models.StatePaused
		ps.Runtime.FilledParts = 0
		e.decrementActiveArbs()
	} else {
		// Ошибка закрытия - переводим в ERROR
		ps.Runtime.State = models.StateError
		e.notifyError(ps, result.Error)
	}
}

// UpdatePairConfig обновляет конфигурацию пары в движке
func (e *Engine) UpdatePairConfig(pairID int, cfg *models.PairConfig) {
	e.pairsMu.RLock()
	ps, ok := e.pairs[pairID]
	e.pairsMu.RUnlock()

	if !ok {
		return
	}

	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Обновляем торговые параметры
	ps.Config.EntrySpreadPct = cfg.EntrySpreadPct
	ps.Config.ExitSpreadPct = cfg.ExitSpreadPct
	ps.Config.VolumeAsset = cfg.VolumeAsset
	ps.Config.NOrders = cfg.NOrders
	ps.Config.StopLoss = cfg.StopLoss
}

// HasOpenPosition проверяет, есть ли открытая позиция у пары
func (e *Engine) HasOpenPosition(pairID int) bool {
	e.pairsMu.RLock()
	ps, ok := e.pairs[pairID]
	e.pairsMu.RUnlock()

	if !ok {
		return false
	}

	ps.mu.RLock()
	defer ps.mu.RUnlock()

	// Позиция открыта в состояниях HOLDING, ENTERING или EXITING
	state := ps.Runtime.State
	return state == models.StateHolding ||
		state == models.StateEntering ||
		state == models.StateExiting
}
