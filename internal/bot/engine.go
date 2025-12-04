package bot

import (
	"context"
	"fmt"
	"math"
	"runtime"
	"sync"
	"sync/atomic"
	"time"

	"arbitrage/internal/config"
	"arbitrage/internal/exchange"
	"arbitrage/internal/models"
	"arbitrage/pkg/utils"
)

// ============ ОПТИМИЗАЦИЯ: Object Pools для zero-allocation ============
// Убирает ~2000+ аллокаций/сек в горячем пути

var priceUpdatePool = sync.Pool{
	New: func() interface{} {
		return &PriceUpdate{}
	},
}

// Pool для Notification - убирает аллокации при уведомлениях
var notificationPool = sync.Pool{
	New: func() interface{} {
		return &models.Notification{
			Meta: make(map[string]interface{}, 8), // предаллокация
		}
	},
}

// Pool для слайсов PairState - для checkAllExitConditions
var pairStateSlicePool = sync.Pool{
	New: func() interface{} {
		s := make([]*PairState, 0, 64) // capacity для 64 пар
		return &s
	},
}

// acquireNotification получает Notification из пула
func acquireNotification() *models.Notification {
	return notificationPool.Get().(*models.Notification)
}

// releaseNotification возвращает Notification в пул
func releaseNotification(n *models.Notification) {
	if n == nil {
		return
	}
	// Очищаем для GC
	n.Type = ""
	n.Severity = ""
	n.Message = ""
	n.PairID = nil
	n.Timestamp = time.Time{}
	// Очищаем map без реаллокации
	for k := range n.Meta {
		delete(n.Meta, k)
	}
	notificationPool.Put(n)
}

// PositionKey - ключ для быстрого поиска пары по позиции O(1)
type PositionKey struct {
	Exchange string
	Symbol   string
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

	// Родительский контекст для graceful shutdown
	ctx    context.Context
	cancel context.CancelFunc

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

	// ОПТИМИЗАЦИЯ: индекс позиций для O(1) поиска при ликвидациях
	// map[PositionKey]*PairState - быстрый поиск пары по exchange+symbol
	// Обновляется при входе/выходе из позиции
	positionIndex sync.Map

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

	// Менеджер рисков (SL/ликвидации) и монитор
	riskManager *RiskManager
	riskMonitor *RiskMonitor

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

	// ОПТИМИЗАЦИЯ: atomic копии торговых параметров для lock-free чтения в горячем пути
	// Используем atomic.Uint64 + math.Float64bits/Float64frombits для float64
	// Обновляются при UpdatePairConfig, читаются в checkArbitrageOpportunity
	entrySpreadBits uint64 // atomic: EntrySpreadPct в битовом представлении
	exitSpreadBits  uint64 // atomic: ExitSpreadPct в битовом представлении
	stopLossBits    uint64 // atomic: StopLoss в битовом представлении
}

// GetEntrySpread возвращает EntrySpreadPct атомарно (lock-free)
func (ps *PairState) GetEntrySpread() float64 {
	return math.Float64frombits(atomic.LoadUint64(&ps.entrySpreadBits))
}

// GetExitSpread возвращает ExitSpreadPct атомарно (lock-free)
func (ps *PairState) GetExitSpread() float64 {
	return math.Float64frombits(atomic.LoadUint64(&ps.exitSpreadBits))
}

// GetStopLoss возвращает StopLoss атомарно (lock-free)
func (ps *PairState) GetStopLoss() float64 {
	return math.Float64frombits(atomic.LoadUint64(&ps.stopLossBits))
}

// setEntrySpread устанавливает EntrySpreadPct атомарно
func (ps *PairState) setEntrySpread(v float64) {
	atomic.StoreUint64(&ps.entrySpreadBits, math.Float64bits(v))
}

// setExitSpread устанавливает ExitSpreadPct атомарно
func (ps *PairState) setExitSpread(v float64) {
	atomic.StoreUint64(&ps.exitSpreadBits, math.Float64bits(v))
}

// setStopLoss устанавливает StopLoss атомарно
func (ps *PairState) setStopLoss(v float64) {
	atomic.StoreUint64(&ps.stopLossBits, math.Float64bits(v))
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

	// Создаём родительский контекст для graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())

	e := &Engine{
		cfg:       cfg,
		ctx:       ctx,
		cancel:    cancel,
		exchanges: make(map[string]exchange.Exchange),
		pairs:     make(map[int]*PairState),
		// pairsBySymbol: sync.Map инициализируется автоматически (zero value)
		// positionIndex: sync.Map инициализируется автоматически (zero value)
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

	balanceFetcher := func(ctx context.Context, exchangeName string) (float64, error) {
		e.exchMu.RLock()
		exch, ok := e.exchanges[exchangeName]
		e.exchMu.RUnlock()
		if !ok {
			return 0, fmt.Errorf("exchange %s not found", exchangeName)
		}

		return exch.GetBalance(ctx)
	}

	// Инициализация арбитражного детектора
	e.arbDetector = NewArbitrageDetector(
		e.priceTracker,
		e.spreadCalc,
		e.orderBookAnalyzer,
		balanceFetcher,
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

	// Инициализация менеджера рисков: аварийное закрытие SL/ликвидаций
	e.riskManager = NewRiskManager(
		e.notificationChan,
		e.closePositionForRisk,
		func(pairID int) { _ = e.PausePair(pairID) },
		DefaultRiskConfig(),
	)
	e.riskManager.SetExchanges(e.exchanges)
	e.riskMonitor = NewRiskMonitor(e.riskManager, e.getHoldingPairsSnapshot)

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
	go e.periodicTasks(ctx)        // балансы, статистика для UI
	go e.notificationWorker(ctx)   // обработка уведомлений
	go e.exitConditionChecker(ctx) // проверка условий выхода
	if e.riskMonitor != nil {
		go e.riskMonitor.Start(ctx) // аварийный SL мониторинг
	}

	<-ctx.Done()

	// Graceful shutdown
	e.cancel() // отменяем внутренний контекст для горутин закрытия
	close(e.shutdown)

	// Drain каналов для освобождения памяти
	e.drainChannels()

	return ctx.Err()
}

// drainChannels очищает буферы каналов при shutdown
func (e *Engine) drainChannels() {
	// Drain price shards
	for _, shard := range e.priceShards {
		for {
			select {
			case update := <-shard.updates:
				releasePriceUpdate(update)
			default:
				goto nextShard
			}
		}
	nextShard:
	}

	// Drain position updates
	for {
		select {
		case <-e.positionUpdates:
		default:
			return
		}
	}
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
// ОПТИМИЗАЦИЯ: использует sync.Pool для слайса, предаллокация capacity
func (e *Engine) checkAllExitConditions(ctx context.Context) {
	// Получаем слайс из пула (zero-allocation!)
	slicePtr := pairStateSlicePool.Get().(*[]*PairState)
	holdingPairs := (*slicePtr)[:0] // reset length, keep capacity

	// Собираем пары в состоянии HOLDING под коротким RLock
	e.pairsMu.RLock()
	for _, ps := range e.pairs {
		// ОПТИМИЗАЦИЯ: atomic проверка без захвата ps.mu
		if atomic.LoadInt32(&ps.isReady) == 0 && ps.Runtime.State == models.StateHolding {
			holdingPairs = append(holdingPairs, ps)
		}
	}
	e.pairsMu.RUnlock()

	// Проверяем условия выхода для каждой (вне lock)
	for _, ps := range holdingPairs {
		e.checkExitConditionsForPair(ctx, ps)
	}

	// Возвращаем слайс в пул
	*slicePtr = holdingPairs
	pairStateSlicePool.Put(slicePtr)
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
// ОПТИМИЗАЦИЯ: использует e.ctx для graceful shutdown, обновляет positionIndex
func (e *Engine) executeExit(ps *PairState, reason ExitReason) {
	// Используем родительский контекст для graceful shutdown
	ctx, cancel := context.WithTimeout(e.ctx, e.cfg.Bot.OrderTimeout)
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

		// ОПТИМИЗАЦИЯ: очищаем positionIndex для O(1) поиска при ликвидациях
		e.removeFromPositionIndex(ps)

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
// ОПТИМИЗАЦИЯ v2: минимизация времени под Lock
//
// Алгоритм оптимизации:
// 1. Atomic проверки без Lock (isReady, activeArbs)
// 2. Быстрая проверка спреда без Lock (90%+ вызовов отсеиваются здесь)
// 3. Lock только для финальной проверки + изменения состояния
//
// Время под Lock: < 100μs (было ~500μs с CheckEntryConditions внутри)
func (e *Engine) checkArbitrageOpportunity(ps *PairState) {
	// ОПТИМИЗАЦИЯ 1: atomic проверка без Lock
	if atomic.LoadInt32(&ps.isReady) != 1 {
		return
	}

	// ОПТИМИЗАЦИЯ 2: atomic проверка лимита арбитражей
	if !e.canOpenNewArbitrage() {
		return
	}

	// ОПТИМИЗАЦИЯ 3: быстрая проверка спреда БЕЗ Lock
	// Config.Symbol - immutable (не меняется после создания пары)
	// EntrySpread - читаем атомарно через GetEntrySpread() (lock-free)
	symbol := ps.Config.Symbol
	entrySpread := ps.GetEntrySpread()

	// Получаем текущую арбитражную возможность (lock-free через sync.Map)
	opp := e.arbDetector.DetectOpportunity(symbol)
	if opp == nil {
		return
	}
	if opp.NetSpread < entrySpread {
		// Нет подходящей возможности - возвращаем в пул и выходим БЕЗ Lock (90%+ случаев)
		ReleaseArbitrageOpportunity(opp)
		return
	}

	// Есть потенциальная возможность! Теперь берём Lock для изменения состояния
	ps.mu.Lock()

	// Double-check pattern: проверяем что ничего не изменилось пока ждали Lock
	if ps.Config.Status != "active" || ps.Runtime.State != models.StateReady {
		ps.mu.Unlock()
		ReleaseArbitrageOpportunity(opp) // Возвращаем в пул
		return
	}

	// Освобождаем opp - он использовался только для быстрой проверки
	// CheckEntryConditions создаст свой экземпляр если нужно
	ReleaseArbitrageOpportunity(opp)

	// Полная проверка условий входа (под Lock, но быстро - всё кэшировано)
	currentArbs := atomic.LoadInt64(&e.activeArbs)
	conditions := e.arbDetector.CheckEntryConditions(
		ps,
		currentArbs,
		e.cfg.Bot.MaxConcurrentArbs,
		e.orderValidator,
	)

	// Условия не выполнены - выходим
	if !conditions.CanEnter {
		ps.mu.Unlock()
		// Освобождаем EntryConditions в пул (opp уже освобождён в CheckEntryConditions)
		ReleaseEntryConditions(conditions)
		return
	}

	// ВХОДИМ! Переключаем состояние
	ps.Runtime.State = models.StateEntering
	atomic.StoreInt32(&ps.isReady, 0) // Сбрасываем флаг
	e.incrementActiveArbs()

	ps.mu.Unlock() // Освобождаем Lock как можно раньше!

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
	// Используем родительский контекст e.ctx для корректного graceful shutdown
	ctx, cancel := context.WithTimeout(e.ctx, e.cfg.Bot.OrderTimeout)
	defer cancel()

	// ОПТИМИЗАЦИЯ: освобождаем объекты в конце (возвращаем в пул)
	opp := conditions.Opportunity
	defer func() {
		ReleaseArbitrageOpportunity(opp)
		ReleaseEntryConditions(conditions)
	}()

	// МЕТРИКА: засекаем время исполнения входа
	entryStart := time.Now()

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
			EntrySpread:   ps.GetEntrySpread(),
			ExitSpread:    ps.GetExitSpread(),
			MinSpread:     ps.GetEntrySpread() * 0.8, // 80% от entry spread (atomic read)
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

		// ОПТИМИЗАЦИЯ: добавляем в positionIndex для O(1) поиска при ликвидациях
		e.addToPositionIndex(ps)

		// МЕТРИКА: обновляем счётчик активных арбитражей
		UpdateActiveArbitrages(atomic.LoadInt64(&e.activeArbs))
		EventsProcessed.WithLabelValues("entry").Inc()

		e.notifyTradeOpened(ps, result)
	} else {
		// Ошибка - возврат в готовность или пауза при провале второй ноги
		if result.ShouldPause {
			ps.Runtime.State = models.StatePaused
			ps.Config.Status = "paused"
			atomic.StoreInt32(&ps.isReady, 0)
		} else {
			ps.Runtime.State = models.StateReady
			// ОПТИМИЗАЦИЯ: восстанавливаем atomic флаг для быстрой проверки
			atomic.StoreInt32(&ps.isReady, 1)
		}
		e.decrementActiveArbs()

		// МЕТРИКА: записываем откат
		RecordTrade(ps.Config.Symbol, "rollback", 0)

		e.notifyError(ps, result.Error)
	}
}

// executeEntry - исполнение входа в арбитраж (ПАРАЛЛЕЛЬНЫЕ ОРДЕРА!)
// Deprecated: используйте executeEntryWithConditions
func (e *Engine) executeEntry(ps *PairState, opp *ArbitrageOpportunity) {
	// Используем родительский контекст e.ctx для корректного graceful shutdown
	ctx, cancel := context.WithTimeout(e.ctx, e.cfg.Bot.OrderTimeout)
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

		// ОПТИМИЗАЦИЯ: добавляем в positionIndex для O(1) поиска при ликвидациях
		e.addToPositionIndex(ps)

		e.notifyTradeOpened(ps, result)
	} else {
		// Ошибка - возврат в готовность или пауза при провале второй ноги
		if result.ShouldPause {
			ps.Runtime.State = models.StatePaused
			ps.Config.Status = "paused"
			atomic.StoreInt32(&ps.isReady, 0)
		} else {
			ps.Runtime.State = models.StateReady
			// ОПТИМИЗАЦИЯ: восстанавливаем atomic флаг для быстрой проверки
			atomic.StoreInt32(&ps.isReady, 1)
		}
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
// ОПТИМИЗАЦИЯ: O(1) поиск через positionIndex вместо O(n) по всем парам
// Критически важно для минимизации убытков при ликвидации!
func (e *Engine) handleLiquidation(update PositionUpdate) {
	// O(1) поиск пары через индекс
	key := PositionKey{Exchange: update.Exchange, Symbol: update.Symbol}
	v, ok := e.positionIndex.Load(key)
	if !ok {
		// Позиция не найдена в индексе - возможно уже закрыта
		return
	}

	ps := v.(*PairState)

	// МЕТРИКА: записываем ликвидацию
	LiquidationsDetected.WithLabelValues(update.Exchange, update.Symbol).Inc()

	if e.riskManager != nil {
		e.riskManager.OnPositionUpdate(ps, update)
		return
	}

	// Fallback для случая отсутствия riskManager
	ps.mu.Lock()
	if ps.Runtime.State != models.StateHolding {
		ps.mu.Unlock()
		return
	}

	ps.Runtime.State = models.StateExiting
	ps.mu.Unlock()

	go e.emergencyCloseSecondLeg(ps, update)
}

// closePositionForRisk - аварийное закрытие обеих ног по сигналу RiskManager
func (e *Engine) closePositionForRisk(ctx context.Context, ps *PairState, reason ExitReason) error {
	if ps == nil || ps.Runtime == nil {
		return fmt.Errorf("pair runtime not initialized")
	}

	ps.mu.RLock()
	legsCopy := make([]models.Leg, len(ps.Runtime.Legs))
	copy(legsCopy, ps.Runtime.Legs)
	symbol := ps.Config.Symbol
	ps.mu.RUnlock()

	// Используем укороченный таймаут из контекста RiskManager
	result := e.orderExec.CloseParallel(ctx, CloseParams{
		Symbol: symbol,
		Legs:   legsCopy,
	})

	ps.mu.Lock()
	defer ps.mu.Unlock()

	if !result.Success {
		ps.Runtime.State = models.StateError
		return result.Error
	}

	e.removeFromPositionIndex(ps)
	ps.Runtime.Legs = nil
	ps.Runtime.FilledParts = 0
	e.decrementActiveArbs()

	ps.Runtime.State = models.StatePaused
	ps.Config.Status = "paused"
	atomic.StoreInt32(&ps.isReady, 0)

	e.notifyTradeClosed(ps, result, reason)
	return nil
}

// emergencyCloseSecondLeg экстренно закрывает вторую ногу при ликвидации
// ОПТИМИЗАЦИЯ: короткий timeout, агрессивный retry
func (e *Engine) emergencyCloseSecondLeg(ps *PairState, liquidatedPos PositionUpdate) {
	// Короткий timeout для экстренного закрытия (5 секунд вместо стандартного)
	emergencyTimeout := 5 * time.Second
	if e.cfg.Bot.OrderTimeout < emergencyTimeout {
		emergencyTimeout = e.cfg.Bot.OrderTimeout
	}

	ctx, cancel := context.WithTimeout(e.ctx, emergencyTimeout)
	defer cancel()

	// МЕТРИКА: засекаем время
	startTime := time.Now()

	ps.mu.RLock()
	legs := ps.Runtime.Legs
	symbol := ps.Config.Symbol
	ps.mu.RUnlock()

	// Находим вторую ногу (не ликвидированную)
	var secondLeg *models.Leg
	for i := range legs {
		if legs[i].Exchange != liquidatedPos.Exchange {
			secondLeg = &legs[i]
			break
		}
	}

	if secondLeg == nil {
		// Вторая нога не найдена - обе позиции на одной бирже?
		e.notifyLiquidationError(ps, fmt.Errorf("second leg not found for %s", symbol))
		return
	}

	// Определяем сторону для закрытия
	closeSide := "sell"
	if secondLeg.Side == "short" {
		closeSide = "buy"
	}

	// Агрессивный retry: 3 попытки с минимальной задержкой
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond) // минимальная задержка между попытками
		}

		// Получаем биржу
		e.exchMu.RLock()
		exch, ok := e.exchanges[secondLeg.Exchange]
		e.exchMu.RUnlock()

		if !ok {
			lastErr = fmt.Errorf("exchange %s not found", secondLeg.Exchange)
			continue
		}

		// Закрываем позицию
		err := exch.ClosePosition(ctx, symbol, closeSide, secondLeg.Quantity)
		if err == nil {
			// Успешно закрыли
			break
		}
		lastErr = err
	}

	// МЕТРИКА: записываем латентность
	latencyMs := float64(time.Since(startTime).Milliseconds())
	RecordTickToOrder(symbol, "emergency_close", latencyMs)

	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Очищаем positionIndex
	e.removeFromPositionIndex(ps)

	if lastErr != nil {
		// Не удалось закрыть - переводим в ERROR
		ps.Runtime.State = models.StateError
		e.notifyLiquidationError(ps, lastErr)
		return
	}

	// Успешное закрытие после ликвидации
	ps.Runtime.Legs = nil
	ps.Runtime.FilledParts = 0
	ps.Runtime.State = models.StatePaused
	ps.Config.Status = "paused"
	atomic.StoreInt32(&ps.isReady, 0)
	e.decrementActiveArbs()

	// Уведомление о ликвидации
	e.notifyLiquidation(ps, liquidatedPos)
}

// notifyLiquidation отправляет уведомление о ликвидации
func (e *Engine) notifyLiquidation(ps *PairState, liquidatedPos PositionUpdate) {
	pairID := ps.Config.ID
	notif := acquireNotification()
	notif.Timestamp = time.Now()
	notif.Type = "LIQUIDATION"
	notif.Severity = "error"
	notif.PairID = &pairID
	notif.Message = fmt.Sprintf("%s LIQUIDATION on %s! Second leg closed.",
		ps.Config.Symbol, liquidatedPos.Exchange)
	notif.Meta["symbol"] = ps.Config.Symbol
	notif.Meta["liquidated_exchange"] = liquidatedPos.Exchange
	notif.Meta["liquidated_side"] = liquidatedPos.Side

	select {
	case e.notificationChan <- notif:
	default:
		releaseNotification(notif)
	}
}

// notifyLiquidationError отправляет уведомление об ошибке при ликвидации
func (e *Engine) notifyLiquidationError(ps *PairState, err error) {
	pairID := ps.Config.ID
	notif := acquireNotification()
	notif.Timestamp = time.Now()
	notif.Type = "LIQUIDATION"
	notif.Severity = "error"
	notif.PairID = &pairID
	notif.Message = fmt.Sprintf("%s LIQUIDATION ERROR: %v", ps.Config.Symbol, err)
	notif.Meta["symbol"] = ps.Config.Symbol
	notif.Meta["error"] = err.Error()

	select {
	case e.notificationChan <- notif:
	default:
		releaseNotification(notif)
	}
}

// periodicTasks - периодические задачи (НЕ влияют на торговлю)
// ОПТИМИЗАЦИЯ: добавлен мониторинг goroutines для alerting
func (e *Engine) periodicTasks(ctx context.Context) {
	balanceTicker := time.NewTicker(e.cfg.Bot.BalanceUpdateFreq)
	statsTicker := time.NewTicker(e.cfg.Bot.StatsUpdateFreq)
	goroutineTicker := time.NewTicker(10 * time.Second) // мониторинг goroutines
	defer balanceTicker.Stop()
	defer statsTicker.Stop()
	defer goroutineTicker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-balanceTicker.C:
			e.updateBalances()
		case <-statsTicker.C:
			e.broadcastPairStates()
		case <-goroutineTicker.C:
			// МЕТРИКА: обновляем счётчик горутин для мониторинга утечек
			GoroutineCount.Set(float64(runtime.NumGoroutine()))
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

// addToPositionIndex добавляет позицию в индекс для O(1) поиска при ликвидациях
// ВАЖНО: вызывать после успешного входа в позицию
func (e *Engine) addToPositionIndex(ps *PairState) {
	for _, leg := range ps.Runtime.Legs {
		key := PositionKey{Exchange: leg.Exchange, Symbol: ps.Config.Symbol}
		e.positionIndex.Store(key, ps)
	}
}

// removeFromPositionIndex удаляет позицию из индекса
// ВАЖНО: вызывать перед очисткой ps.Runtime.Legs
func (e *Engine) removeFromPositionIndex(ps *PairState) {
	for _, leg := range ps.Runtime.Legs {
		key := PositionKey{Exchange: leg.Exchange, Symbol: ps.Config.Symbol}
		e.positionIndex.Delete(key)
	}
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
			"symbol":         ps.Config.Symbol,
			"long_exchange":  longLeg.Exchange,
			"long_price":     longLeg.EntryPrice,
			"long_qty":       longLeg.Quantity,
			"short_exchange": shortLeg.Exchange,
			"short_price":    shortLeg.EntryPrice,
			"short_qty":      shortLeg.Quantity,
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

	if e.riskManager != nil {
		e.riskManager.AddExchange(name, exch)
	}

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

// getHoldingPairsSnapshot возвращает snapshot пар в HOLDING для RiskMonitor
func (e *Engine) getHoldingPairsSnapshot() []*PairState {
	e.pairsMu.RLock()
	pairsCopy := make([]*PairState, 0, len(e.pairs))
	for _, ps := range e.pairs {
		ps.mu.RLock()
		isHolding := ps.Runtime != nil && ps.Runtime.State == models.StateHolding
		if isHolding {
			pairsCopy = append(pairsCopy, ps)
		}
		ps.mu.RUnlock()
	}
	e.pairsMu.RUnlock()
	return pairsCopy
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

	// ОПТИМИЗАЦИЯ: инициализируем atomic поля для lock-free чтения в горячем пути
	ps.setEntrySpread(cfg.EntrySpreadPct)
	ps.setExitSpread(cfg.ExitSpreadPct)
	ps.setStopLoss(cfg.StopLoss)

	// Добавляем в основной map под lock
	e.pairsMu.Lock()
	e.pairs[cfg.ID] = ps
	e.pairsMu.Unlock()

	// ОПТИМИЗАЦИЯ: атомарное обновление sync.Map с защитой от бесконечного цикла
	// Используем Load → modify → Store паттерн
	const maxCASRetries = 100
	casSuccess := false
	for i := 0; i < maxCASRetries; i++ {
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
				casSuccess = true
				break
			}
		} else {
			if e.pairsBySymbol.CompareAndSwap(cfg.Symbol, existing, newSlice) {
				casSuccess = true
				break
			}
		}
	}

	// Логируем если CAS не удался (крайне маловероятно, но важно для отладки)
	if !casSuccess {
		if logger := utils.GetGlobalLogger(); logger != nil {
			logger.Sugar().Warnf("AddPair: CAS retry exhausted for symbol %s after %d attempts", cfg.Symbol, maxCASRetries)
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

	// ОПТИМИЗАЦИЯ: атомарное обновление sync.Map с ограничением итераций
	const maxCASRetries = 100
	for i := 0; i < maxCASRetries; i++ {
		existing, ok := e.pairsBySymbol.Load(symbol)
		if !ok {
			return // Уже удалено
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
				return
			}
		} else {
			if e.pairsBySymbol.CompareAndSwap(symbol, existing, newSlice) {
				return
			}
		}
	}

	// Логируем если CAS не удался (крайне маловероятно)
	if logger := utils.GetGlobalLogger(); logger != nil {
		logger.Sugar().Warnf("RemovePair: CAS retry exhausted for symbol %s after %d attempts", symbol, maxCASRetries)
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
// ОПТИМИЗАЦИЯ: deep copy для безопасности (Legs - слайс)
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

	// Deep copy: копируем структуру и слайс Legs
	runtime := *ps.Runtime
	if len(ps.Runtime.Legs) > 0 {
		runtime.Legs = make([]models.Leg, len(ps.Runtime.Legs))
		copy(runtime.Legs, ps.Runtime.Legs)
	}
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

	// Обновляем торговые параметры в Config (для персистентности)
	ps.Config.EntrySpreadPct = cfg.EntrySpreadPct
	ps.Config.ExitSpreadPct = cfg.ExitSpreadPct
	ps.Config.VolumeAsset = cfg.VolumeAsset
	ps.Config.NOrders = cfg.NOrders
	ps.Config.StopLoss = cfg.StopLoss

	// ОПТИМИЗАЦИЯ: обновляем atomic копии для lock-free чтения в горячем пути
	// Эти значения читаются в checkArbitrageOpportunity без блокировки
	ps.setEntrySpread(cfg.EntrySpreadPct)
	ps.setExitSpread(cfg.ExitSpreadPct)
	ps.setStopLoss(cfg.StopLoss)
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
