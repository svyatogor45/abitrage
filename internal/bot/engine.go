package bot

import (
	"context"
	"sync"
	"time"

	"arbitrage/internal/config"
	"arbitrage/internal/exchange"
	"arbitrage/internal/models"
)

// Engine - главный движок арбитражного бота (EVENT-DRIVEN архитектура)
//
// Архитектура:
// - НЕТ polling! Только реакция на события от WebSocket
// - Каждое обновление цены мгновенно триггерит проверку арбитража
// - Параллельная отправка ордеров на обе биржи
// - O(n) поиск лучшей связки через глобальный трекер лучших цен
//
// Поток данных:
// WebSocket Price Update → PriceTracker → SpreadCalculator → ArbitrageDecision → ParallelOrderExecution
type Engine struct {
	cfg *config.Config

	// Подключенные биржи
	exchanges map[string]exchange.Exchange
	exchMu    sync.RWMutex

	// Активные торговые пары
	pairs   map[int]*PairState
	pairsMu sync.RWMutex

	// Глобальный трекер лучших цен для O(n) поиска
	priceTracker *PriceTracker

	// Калькулятор спреда
	spreadCalc *SpreadCalculator

	// Исполнитель ордеров
	orderExec *OrderExecutor

	// Каналы событий (event-driven)
	priceUpdates    chan PriceUpdate    // обновления цен от WS
	positionUpdates chan PositionUpdate // обновления позиций (ликвидации)
	shutdown        chan struct{}

	// WebSocket hub для отправки данных клиентам
	wsHub WebSocketHub

	// Контроль одновременных арбитражей
	activeArbs   int
	activeArbsMu sync.Mutex
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
	Exchange    string
	Symbol      string
	Side        string
	Liquidated  bool
	UnrealizedPnl float64
}

// WebSocketHub - интерфейс для отправки данных клиентам
type WebSocketHub interface {
	BroadcastPairUpdate(pairID int, data interface{})
	BroadcastNotification(notif *models.Notification)
	BroadcastBalanceUpdate(exchange string, balance float64)
}

// NewEngine создает новый Engine
func NewEngine(cfg *config.Config, wsHub WebSocketHub) *Engine {
	e := &Engine{
		cfg:             cfg,
		exchanges:       make(map[string]exchange.Exchange),
		pairs:           make(map[int]*PairState),
		priceTracker:    NewPriceTracker(),
		priceUpdates:    make(chan PriceUpdate, 10000),    // буфер для высокой нагрузки
		positionUpdates: make(chan PositionUpdate, 1000),
		shutdown:        make(chan struct{}),
		wsHub:           wsHub,
	}

	e.spreadCalc = NewSpreadCalculator(e.priceTracker)
	e.orderExec = NewOrderExecutor(e.exchanges, cfg.Bot)

	return e
}

// Run запускает event-driven движок
func (e *Engine) Run(ctx context.Context) error {
	// Запуск воркеров для обработки событий
	go e.priceEventLoop(ctx)
	go e.positionEventLoop(ctx)
	go e.periodicTasks(ctx) // балансы, статистика для UI

	<-ctx.Done()
	close(e.shutdown)
	return ctx.Err()
}

// priceEventLoop - главный цикл обработки ценовых событий
// Вызывается МГНОВЕННО при каждом обновлении цены от WebSocket
func (e *Engine) priceEventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case update := <-e.priceUpdates:
			e.handlePriceUpdate(update)
		}
	}
}

// handlePriceUpdate - обработка обновления цены
// Время выполнения: ~1-5ms (без сетевых запросов!)
func (e *Engine) handlePriceUpdate(update PriceUpdate) {
	// 1. Обновляем глобальный трекер цен O(1)
	e.priceTracker.Update(update)

	// 2. Проверяем все активные пары с этим символом
	e.pairsMu.RLock()
	relevantPairs := e.getPairsForSymbol(update.Symbol)
	e.pairsMu.RUnlock()

	for _, pairState := range relevantPairs {
		e.checkArbitrageOpportunity(pairState)
	}
}

// checkArbitrageOpportunity - проверка арбитражной возможности для пары
func (e *Engine) checkArbitrageOpportunity(ps *PairState) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Пропускаем если пара не активна или уже в позиции
	if ps.Config.Status != "active" || ps.Runtime.State != models.StateReady {
		return
	}

	// Проверка лимита одновременных арбитражей
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
		Symbol:      ps.Config.Symbol,
		Volume:      ps.Config.VolumeAsset,
		LongExchange:  opp.LongExchange,
		ShortExchange: opp.ShortExchange,
		NOrders:      ps.Config.NOrders,
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

// ============ Вспомогательные методы ============

func (e *Engine) getPairsForSymbol(symbol string) []*PairState {
	var result []*PairState
	for _, ps := range e.pairs {
		if ps.Config.Symbol == symbol {
			result = append(result, ps)
		}
	}
	return result
}

func (e *Engine) canOpenNewArbitrage() bool {
	if e.cfg.Bot.MaxConcurrentArbs == 0 {
		return true // без лимита
	}
	e.activeArbsMu.Lock()
	defer e.activeArbsMu.Unlock()
	return e.activeArbs < e.cfg.Bot.MaxConcurrentArbs
}

func (e *Engine) incrementActiveArbs() {
	e.activeArbsMu.Lock()
	e.activeArbs++
	e.activeArbsMu.Unlock()
}

func (e *Engine) decrementActiveArbs() {
	e.activeArbsMu.Lock()
	e.activeArbs--
	e.activeArbsMu.Unlock()
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
func (e *Engine) AddPair(cfg *models.PairConfig) {
	e.pairsMu.Lock()
	defer e.pairsMu.Unlock()

	e.pairs[cfg.ID] = &PairState{
		Config: cfg,
		Runtime: &models.PairRuntime{
			PairID: cfg.ID,
			State:  models.StatePaused,
		},
	}

	// Подписываемся на цены этого символа на всех биржах
	e.subscribeToSymbol(cfg.Symbol)
}

// subscribeToSymbol подписывается на цены символа на всех биржах
func (e *Engine) subscribeToSymbol(symbol string) {
	e.exchMu.RLock()
	defer e.exchMu.RUnlock()

	for name, exch := range e.exchanges {
		exchName := name // захват для closure
		exch.SubscribeTicker(symbol, func(ticker *exchange.Ticker) {
			e.priceUpdates <- PriceUpdate{
				Exchange:  exchName,
				Symbol:    ticker.Symbol,
				BidPrice:  ticker.BidPrice,
				AskPrice:  ticker.AskPrice,
				Timestamp: ticker.Timestamp,
			}
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

// OnPriceUpdate - публичный метод для приема ценовых обновлений (вызывается из WS клиентов)
func (e *Engine) OnPriceUpdate(update PriceUpdate) {
	select {
	case e.priceUpdates <- update:
	default:
		// Буфер полон - пропускаем (не блокируем WS)
	}
}
