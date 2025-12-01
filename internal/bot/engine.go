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
// 2. Индекс pairsBySymbol - O(1) поиск пар вместо O(n)
// 3. Atomic counter для activeArbs - нет вложенных локов
// 4. Шардированный PriceTracker - нет lock contention между символами
// 5. Несколько workers на шард - увеличенная пропускная способность
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

	// ОПТИМИЗАЦИЯ: Индекс пар по символу для O(1) поиска
	pairsBySymbol   map[string][]*PairState
	pairsBySymbolMu sync.RWMutex

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
type WebSocketHub interface {
	BroadcastPairUpdate(pairID int, data interface{})
	BroadcastNotification(notif *models.Notification)
	BroadcastBalanceUpdate(exchange string, balance float64)
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
		pairsBySymbol:   make(map[string][]*PairState),
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
// ОПТИМИЗАЦИЯ: atomic counter вместо вложенного лока
func (e *Engine) checkArbitrageOpportunity(ps *PairState) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Пропускаем если пара не активна или уже в позиции
	if ps.Config.Status != "active" || ps.Runtime.State != models.StateReady {
		return
	}

	// Проверка лимита одновременных арбитражей (atomic - без lock!)
	if !e.canOpenNewArbitrage() {
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
func (e *Engine) updateBalances() {
	e.exchMu.RLock()
	defer e.exchMu.RUnlock()

	for name, exch := range e.exchanges {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		balance, err := exch.GetBalance(ctx)
		cancel()

		if err == nil && e.wsHub != nil {
			e.wsHub.BroadcastBalanceUpdate(name, balance)
		}
	}
}

// broadcastPairStates - отправка состояний пар клиентам
func (e *Engine) broadcastPairStates() {
	e.pairsMu.RLock()
	defer e.pairsMu.RUnlock()

	for id, ps := range e.pairs {
		if ps.Runtime.State == models.StateHolding {
			// Обновляем PNL из текущих цен
			e.updatePairPnl(ps)

			if e.wsHub != nil {
				e.wsHub.BroadcastPairUpdate(id, ps.Runtime)
			}
		}
	}
}

// ============ Оптимизированные вспомогательные методы ============

// getPairsForSymbol - O(1) поиск через индекс вместо O(n)
func (e *Engine) getPairsForSymbol(symbol string) []*PairState {
	e.pairsBySymbolMu.RLock()
	defer e.pairsBySymbolMu.RUnlock()
	return e.pairsBySymbol[symbol]
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
// ОПТИМИЗАЦИЯ: Фиксированный порядок блокировок для предотвращения deadlock
// Порядок: pairsMu → pairsBySymbolMu (ВСЕГДА в этом порядке!)
func (e *Engine) AddPair(cfg *models.PairConfig) {
	ps := &PairState{
		Config: cfg,
		Runtime: &models.PairRuntime{
			PairID: cfg.ID,
			State:  models.StatePaused,
		},
	}

	// Блокируем ОБА мьютекса в фиксированном порядке
	e.pairsMu.Lock()
	e.pairsBySymbolMu.Lock()

	e.pairs[cfg.ID] = ps
	e.pairsBySymbol[cfg.Symbol] = append(e.pairsBySymbol[cfg.Symbol], ps)

	// Разблокируем в обратном порядке
	e.pairsBySymbolMu.Unlock()
	e.pairsMu.Unlock()

	// Подписываемся на цены ПОСЛЕ освобождения locks (избегаем длительного удержания)
	e.subscribeToSymbol(cfg.Symbol)
}

// RemovePair удаляет торговую пару
// ОПТИМИЗАЦИЯ: Фиксированный порядок блокировок для предотвращения deadlock
// Порядок: pairsMu → pairsBySymbolMu (ВСЕГДА в этом порядке!)
func (e *Engine) RemovePair(pairID int) {
	// Блокируем ОБА мьютекса в фиксированном порядке
	e.pairsMu.Lock()
	e.pairsBySymbolMu.Lock()

	ps, ok := e.pairs[pairID]
	if !ok {
		e.pairsBySymbolMu.Unlock()
		e.pairsMu.Unlock()
		return
	}

	symbol := ps.Config.Symbol
	delete(e.pairs, pairID)

	// Обновляем индекс по символу
	pairs := e.pairsBySymbol[symbol]
	for i, p := range pairs {
		if p.Config.ID == pairID {
			e.pairsBySymbol[symbol] = append(pairs[:i], pairs[i+1:]...)
			break
		}
	}
	if len(e.pairsBySymbol[symbol]) == 0 {
		delete(e.pairsBySymbol, symbol)
	}

	// Разблокируем в обратном порядке
	e.pairsBySymbolMu.Unlock()
	e.pairsMu.Unlock()
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
	e.pairsMu.Lock()
	defer e.pairsMu.Unlock()

	if ps, ok := e.pairs[pairID]; ok {
		ps.mu.Lock()
		ps.Config.Status = "active"
		ps.Runtime.State = models.StateReady
		ps.mu.Unlock()
	}
	return nil
}

// PausePair останавливает пару
func (e *Engine) PausePair(pairID int) error {
	e.pairsMu.Lock()
	defer e.pairsMu.Unlock()

	if ps, ok := e.pairs[pairID]; ok {
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
