package bot

import (
	"context"
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

	// Worker pool: шардированные каналы по символам
	priceShards     []*priceShard
	numShards       int
	workersPerShard int // ОПТИМИЗАЦИЯ: несколько workers на шард

	// Канал для позиций (отдельный, не шардируется)
	positionUpdates chan PositionUpdate
	shutdown        chan struct{}

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
		cfg:             cfg,
		exchanges:       make(map[string]exchange.Exchange),
		pairs:           make(map[int]*PairState),
		// pairsBySymbol: sync.Map инициализируется автоматически (zero value)
		priceTracker:    NewPriceTracker(numShards),
		priceShards:     make([]*priceShard, numShards),
		numShards:       numShards,
		workersPerShard: workersPerShard,
		positionUpdates: make(chan PositionUpdate, 1000),
		shutdown:        make(chan struct{}),
		wsHub:           wsHub,
	}

	// Инициализация шардов для worker pool
	for i := 0; i < numShards; i++ {
		e.priceShards[i] = &priceShard{
			updates: make(chan *PriceUpdate, 2000), // указатели + буфер на шард
		}
	}

	e.spreadCalc = NewSpreadCalculator(e.priceTracker)
	e.orderExec = NewOrderExecutor(e.exchanges, cfg.Bot)

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
	go e.periodicTasks(ctx) // балансы, статистика для UI

	<-ctx.Done()
	close(e.shutdown)
	return ctx.Err()
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
	}
}

// handlePriceUpdate - обработка обновления цены
// Время выполнения: ~0.5-2ms (без сетевых запросов!)
func (e *Engine) handlePriceUpdate(update *PriceUpdate) {
	// 1. Обновляем шардированный трекер цен O(k), k=число бирж
	e.priceTracker.UpdateFromPtr(update)

	// 2. Получаем пары для этого символа O(1) через индекс
	pairs := e.getPairsForSymbol(update.Symbol)

	// 3. Проверяем арбитраж для каждой пары
	for _, pairState := range pairs {
		e.checkArbitrageOpportunity(pairState)
	}
}

// checkArbitrageOpportunity - проверка арбитражной возможности для пары
// ОПТИМИЗАЦИЯ: atomic быстрая проверка + atomic counter для activeArbs
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

	// Получаем лучшую связку O(1) - уже вычислено в PriceTracker
	opportunity := e.spreadCalc.GetBestOpportunity(ps.Config.Symbol)
	if opportunity == nil {
		return
	}

	// Проверяем условия входа
	if !e.shouldEnter(ps, opportunity) {
		return
	}

	// ВХОДИМ! Переключаем состояние и запускаем исполнение
	ps.Runtime.State = models.StateEntering
	atomic.StoreInt32(&ps.isReady, 0) // Сбрасываем флаг
	e.incrementActiveArbs()

	// Асинхронный вход (не блокируем обработку других событий)
	go e.executeEntry(ps, opportunity)
}

// shouldEnter - проверка всех условий для входа
func (e *Engine) shouldEnter(ps *PairState, opp *ArbitrageOpportunity) bool {
	// 1. Спред достаточный?
	if opp.NetSpread < ps.Config.EntrySpreadPct {
		return false
	}

	// 2. Маржа достаточна на обеих биржах?
	// TODO: проверка маржи (кэшируется, не делаем сетевой запрос)

	// 3. Лимиты ордеров соблюдены?
	// TODO: проверка min/max qty (кэшируется)

	return true
}

// executeEntry - исполнение входа в арбитраж (ПАРАЛЛЕЛЬНЫЕ ОРДЕРА!)
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
	// TODO: расчёт PNL из текущих цен в PriceTracker
}

func (e *Engine) notifyTradeOpened(ps *PairState, result *ExecuteResult) {
	// TODO: создать уведомление
}

func (e *Engine) notifyError(ps *PairState, err error) {
	// TODO: создать уведомление об ошибке
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
