package bot

import (
	"fmt"
	"sync"
	"time"
)

// ============ ОПТИМИЗАЦИЯ: Inline FNV-1a hash без аллокаций ============
// Константы FNV-1a для 32-битного хэша
const (
	fnvOffset32 = uint32(2166136261)
	fnvPrime32  = uint32(16777619)
)

// ============ ПРИМЕЧАНИЕ К sync.Pool ============
// sync.Pool для BestPrices был УДАЛЁН из-за use-after-free:
// GetBestPrices() возвращает указатель, который может быть
// освобождён в pool до того как вызывающий код закончит его читать.
//
// Альтернатива: возвращать копию, но это не даёт выигрыша над простым new().
// BestPrices достаточно маленький (~130 байт), аллокация занимает ~25ns.
// При ~1000 обновлений/сек это всего 25µs - пренебрежимо мало.

// fnvHash вычисляет FNV-1a hash строки БЕЗ аллокаций
// В отличие от fnv.New32a() не создаёт объект на куче
// Экономит ~2000+ аллокаций/сек в горячем пути
func fnvHash(s string) uint32 {
	h := fnvOffset32
	for i := 0; i < len(s); i++ {
		h ^= uint32(s[i])
		h *= fnvPrime32
	}
	return h
}

// PriceTracker - шардированный глобальный трекер лучших цен
//
// ОПТИМИЗАЦИИ:
// 1. Шардирование по символу - разные символы не блокируют друг друга
// 2. Индекс по символам - O(k) вместо O(n*m) при пересчёте лучших цен
// 3. Предвычисленные ключи - нет string concatenation в горячем пути
//
// Архитектура:
// - numShards шардов, каждый со своим мьютексом
// - Символ → шард определяется через hash(symbol) % numShards
// - Внутри шарда: индекс symbol → []exchangeKey для быстрого пересчёта
type PriceTracker struct {
	shards    []*PriceShard
	numShards uint32
}

// PriceShard - один шард с собственным мьютексом
type PriceShard struct {
	// Лучшие цены по символам в этом шарде
	bestPrices map[string]*BestPrices

	// Все цены по биржам
	// key: PriceKey{Symbol, Exchange}
	allPrices map[PriceKey]*ExchangePrice

	// Индекс: symbol → список ключей бирж для быстрого пересчёта
	// Позволяет итерировать только по биржам этого символа, а не по всем
	symbolIndex map[string][]PriceKey

	mu sync.RWMutex
}

// PriceKey - составной ключ без аллокации строк
// Go оптимизирует struct keys в map
type PriceKey struct {
	Symbol   string
	Exchange string
}

// BestPrices - лучшие цены для символа
type BestPrices struct {
	Symbol string

	// Лучший Ask (самый низкий) - для открытия лонга
	BestAsk     float64
	BestAskExch string
	BestAskTime time.Time

	// Лучший Bid (самый высокий) - для открытия шорта
	BestBid     float64
	BestBidExch string
	BestBidTime time.Time

	// Предвычисленный спред (без учёта комиссий)
	RawSpread float64 // (BestBid - BestAsk) / BestAsk * 100
}

// ExchangePrice - цена на конкретной бирже
type ExchangePrice struct {
	Exchange  string
	Symbol    string
	BidPrice  float64
	AskPrice  float64
	Timestamp time.Time
}

// NewPriceTracker создаёт шардированный трекер
// numShards должен соответствовать количеству worker'ов в Engine
func NewPriceTracker(numShards int) *PriceTracker {
	if numShards <= 0 {
		numShards = 16 // дефолт
	}

	pt := &PriceTracker{
		shards:    make([]*PriceShard, numShards),
		numShards: uint32(numShards),
	}

	for i := 0; i < numShards; i++ {
		pt.shards[i] = &PriceShard{
			bestPrices:  make(map[string]*BestPrices),
			allPrices:   make(map[PriceKey]*ExchangePrice),
			symbolIndex: make(map[string][]PriceKey),
		}
	}

	return pt
}

// getShard возвращает шард для символа (детерминированно)
// ОПТИМИЗАЦИЯ: inline FNV-1a без аллокаций (было fnv.New32a() + []byte conversion)
func (pt *PriceTracker) getShard(symbol string) *PriceShard {
	return pt.shards[fnvHash(symbol)%pt.numShards]
}

// GetShardIndex возвращает индекс шарда для символа
// Используется Engine для роутинга событий к нужному воркеру
// ОПТИМИЗАЦИЯ: inline FNV-1a без аллокаций
func (pt *PriceTracker) GetShardIndex(symbol string) int {
	return int(fnvHash(symbol) % pt.numShards)
}

// Update обновляет цену от биржи и пересчитывает лучшие цены
// Сложность: O(k) где k = количество бирж для символа (обычно 6)
// Lock: только на шарде этого символа (не блокирует другие символы)
//
// ОПТИМИЗАЦИЯ: in-place обновление существующих ExchangePrice
// Экономит ~1000+ аллокаций/сек - создаём объект только для новых ключей
func (pt *PriceTracker) Update(update PriceUpdate) {
	shard := pt.getShard(update.Symbol)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	key := PriceKey{
		Symbol:   update.Symbol,
		Exchange: update.Exchange,
	}

	// ОПТИМИЗАЦИЯ: обновляем существующий объект или создаём новый
	if existing, exists := shard.allPrices[key]; exists {
		// In-place обновление - БЕЗ аллокации!
		existing.BidPrice = update.BidPrice
		existing.AskPrice = update.AskPrice
		existing.Timestamp = update.Timestamp
	} else {
		// Новый ключ - добавляем в индекс и создаём объект
		shard.symbolIndex[update.Symbol] = append(shard.symbolIndex[update.Symbol], key)
		shard.allPrices[key] = &ExchangePrice{
			Exchange:  update.Exchange,
			Symbol:    update.Symbol,
			BidPrice:  update.BidPrice,
			AskPrice:  update.AskPrice,
			Timestamp: update.Timestamp,
		}
	}

	// Пересчитываем лучшие цены для этого символа
	shard.recalculateBest(update.Symbol)
}

// UpdateFromPtr обновляет цену от указателя (для использования с sync.Pool)
// Сложность: O(k) где k = количество бирж для символа (обычно 6)
//
// ОПТИМИЗАЦИЯ: in-place обновление существующих ExchangePrice
// Экономит ~1000+ аллокаций/сек - создаём объект только для новых ключей
func (pt *PriceTracker) UpdateFromPtr(update *PriceUpdate) {
	shard := pt.getShard(update.Symbol)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	key := PriceKey{
		Symbol:   update.Symbol,
		Exchange: update.Exchange,
	}

	// ОПТИМИЗАЦИЯ: обновляем существующий объект или создаём новый
	if existing, exists := shard.allPrices[key]; exists {
		// In-place обновление - БЕЗ аллокации!
		existing.BidPrice = update.BidPrice
		existing.AskPrice = update.AskPrice
		existing.Timestamp = update.Timestamp
	} else {
		// Новый ключ - добавляем в индекс и создаём объект
		shard.symbolIndex[update.Symbol] = append(shard.symbolIndex[update.Symbol], key)
		shard.allPrices[key] = &ExchangePrice{
			Exchange:  update.Exchange,
			Symbol:    update.Symbol,
			BidPrice:  update.BidPrice,
			AskPrice:  update.AskPrice,
			Timestamp: update.Timestamp,
		}
	}

	// Пересчитываем лучшие цены для этого символа
	shard.recalculateBest(update.Symbol)
}

// recalculateBest пересчитывает лучшие Ask и Bid для символа
// Сложность: O(k) где k = количество бирж для этого символа
// ВАЖНО: вызывается под lock'ом шарда
//
// ОПТИМИЗАЦИЯ: in-place обновление существующего BestPrices объекта
// когда это возможно (экономит ~1000+ аллокаций/сек)
func (shard *PriceShard) recalculateBest(symbol string) {
	keys := shard.symbolIndex[symbol]
	if len(keys) == 0 {
		return
	}

	var bestAsk float64 = 0
	var bestAskExch string
	var bestAskTime time.Time

	var bestBid float64 = 0
	var bestBidExch string
	var bestBidTime time.Time

	// Проходим ТОЛЬКО по биржам этого символа (не по всем!)
	for _, key := range keys {
		price := shard.allPrices[key]
		if price == nil {
			continue
		}

		// Ищем минимальный Ask (для лонга - покупаем дёшево)
		if price.AskPrice > 0 && (bestAsk == 0 || price.AskPrice < bestAsk) {
			bestAsk = price.AskPrice
			bestAskExch = price.Exchange
			bestAskTime = price.Timestamp
		}

		// Ищем максимальный Bid (для шорта - продаём дорого)
		if price.BidPrice > 0 && price.BidPrice > bestBid {
			bestBid = price.BidPrice
			bestBidExch = price.Exchange
			bestBidTime = price.Timestamp
		}
	}

	// Сохраняем лучшие цены
	if bestAsk > 0 && bestBid > 0 {
		rawSpread := (bestBid - bestAsk) / bestAsk * 100

		// ОПТИМИЗАЦИЯ: in-place обновление если объект уже существует
		// Это безопасно, т.к. все поля примитивные и записываются атомарно
		// (нет промежуточных состояний при записи float64/string)
		if existing := shard.bestPrices[symbol]; existing != nil {
			existing.Symbol = symbol
			existing.BestAsk = bestAsk
			existing.BestAskExch = bestAskExch
			existing.BestAskTime = bestAskTime
			existing.BestBid = bestBid
			existing.BestBidExch = bestBidExch
			existing.BestBidTime = bestBidTime
			existing.RawSpread = rawSpread
		} else {
			// Первый раз - создаём новый объект
			shard.bestPrices[symbol] = &BestPrices{
				Symbol:      symbol,
				BestAsk:     bestAsk,
				BestAskExch: bestAskExch,
				BestAskTime: bestAskTime,
				BestBid:     bestBid,
				BestBidExch: bestBidExch,
				BestBidTime: bestBidTime,
				RawSpread:   rawSpread,
			}
		}
	}
}

// GetBestPrices возвращает КОПИЮ лучших цен для символа
// Сложность: O(1)
//
// ВАЖНО: возвращается копия, а не оригинал, чтобы избежать race condition
// когда вызывающий код читает данные после освобождения read lock.
// BestPrices ~130 байт, копирование занимает ~10ns - пренебрежимо мало.
func (pt *PriceTracker) GetBestPrices(symbol string) *BestPrices {
	shard := pt.getShard(symbol)
	shard.mu.RLock()
	bp := shard.bestPrices[symbol]
	if bp == nil {
		shard.mu.RUnlock()
		return nil
	}
	// Создаём копию под lock'ом
	copy := *bp
	shard.mu.RUnlock()
	return &copy
}

// GetExchangePrice возвращает КОПИЮ цены конкретной биржи
// Возвращает копию для thread safety (аналогично GetBestPrices)
func (pt *PriceTracker) GetExchangePrice(symbol, exchange string) *ExchangePrice {
	shard := pt.getShard(symbol)
	shard.mu.RLock()

	key := PriceKey{Symbol: symbol, Exchange: exchange}
	ep := shard.allPrices[key]
	if ep == nil {
		shard.mu.RUnlock()
		return nil
	}
	// Создаём копию под lock'ом
	copy := *ep
	shard.mu.RUnlock()
	return &copy
}

// ============================================================
// SpreadCalculator - расчёт спреда с учётом комиссий
// ============================================================

// SpreadCalculator вычисляет арбитражные возможности
type SpreadCalculator struct {
	tracker *PriceTracker

	// Кэш комиссий по биржам (загружается один раз)
	fees   map[string]float64 // exchange -> taker fee (например 0.0005 = 0.05%)
	feesMu sync.RWMutex
}

// ArbitrageOpportunity - найденная арбитражная возможность
type ArbitrageOpportunity struct {
	Symbol string

	// Где открываем лонг (покупаем дёшево)
	LongExchange string
	LongPrice    float64 // Ask price

	// Где открываем шорт (продаём дорого)
	ShortExchange string
	ShortPrice    float64 // Bid price

	// Спреды
	RawSpread float64 // без комиссий
	NetSpread float64 // после вычета комиссий (4 сделки)

	// Используем timestamp из лучших цен (без syscall!)
	Timestamp time.Time
}

// NewSpreadCalculator создаёт калькулятор
func NewSpreadCalculator(tracker *PriceTracker) *SpreadCalculator {
	return &SpreadCalculator{
		tracker: tracker,
		fees:    make(map[string]float64),
	}
}

// SetFee устанавливает комиссию для биржи
func (sc *SpreadCalculator) SetFee(exchange string, fee float64) {
	sc.feesMu.Lock()
	sc.fees[exchange] = fee
	sc.feesMu.Unlock()
}

// GetBestOpportunity возвращает лучшую арбитражную возможность
// Сложность: O(1) - все данные уже предвычислены в PriceTracker
func (sc *SpreadCalculator) GetBestOpportunity(symbol string) *ArbitrageOpportunity {
	best := sc.tracker.GetBestPrices(symbol)
	if best == nil {
		return nil
	}

	// Проверяем что биржи разные (нельзя арбитражить на одной бирже)
	if best.BestAskExch == best.BestBidExch {
		return nil
	}

	// Проверяем что спред положительный
	if best.RawSpread <= 0 {
		return nil
	}

	// Рассчитываем чистый спред с учётом комиссий
	netSpread := sc.calculateNetSpread(best)

	return &ArbitrageOpportunity{
		Symbol:        symbol,
		LongExchange:  best.BestAskExch,
		LongPrice:     best.BestAsk,
		ShortExchange: best.BestBidExch,
		ShortPrice:    best.BestBid,
		RawSpread:     best.RawSpread,
		NetSpread:     netSpread,
		Timestamp:     best.BestAskTime, // Используем timestamp из события (проблема 8)
	}
}

// calculateNetSpread вычисляет чистый спред после комиссий
// 4 тейкер-сделки: открытие лонга, открытие шорта, закрытие лонга, закрытие шорта
func (sc *SpreadCalculator) calculateNetSpread(best *BestPrices) float64 {
	sc.feesMu.RLock()
	feeLong := sc.fees[best.BestAskExch]
	feeShort := sc.fees[best.BestBidExch]
	sc.feesMu.RUnlock()

	// Если комиссии не установлены, используем дефолт 0.05%
	if feeLong == 0 {
		feeLong = 0.0005
	}
	if feeShort == 0 {
		feeShort = 0.0005
	}

	// Суммарные комиссии: 2 сделки на каждой бирже (открытие + закрытие)
	totalFees := 2 * (feeLong + feeShort) * 100 // в процентах

	return best.RawSpread - totalFees
}

// ============================================================
// Дополнительные утилиты
// ============================================================

// GetCurrentSpread возвращает текущий спред для открытой позиции
// Используется для проверки условий выхода
func (sc *SpreadCalculator) GetCurrentSpread(symbol, longExch, shortExch string) float64 {
	longPrice := sc.tracker.GetExchangePrice(symbol, longExch)
	shortPrice := sc.tracker.GetExchangePrice(symbol, shortExch)

	if longPrice == nil || shortPrice == nil {
		return 0
	}

	// Для выхода: продаём лонг по Bid, покупаем шорт по Ask
	// Спред = (Bid_long - Ask_short) / Ask_short * 100
	if shortPrice.AskPrice == 0 {
		return 0
	}

	return (longPrice.BidPrice - shortPrice.AskPrice) / shortPrice.AskPrice * 100
}

// CalculatePnl рассчитывает PNL для открытой позиции
func (sc *SpreadCalculator) CalculatePnl(
	symbol string,
	longExch string, longEntryPrice float64,
	shortExch string, shortEntryPrice float64,
	volume float64,
) float64 {
	longPrice := sc.tracker.GetExchangePrice(symbol, longExch)
	shortPrice := sc.tracker.GetExchangePrice(symbol, shortExch)

	if longPrice == nil || shortPrice == nil {
		return 0
	}

	// PNL лонга = (текущий Bid - цена входа) * объём
	pnlLong := (longPrice.BidPrice - longEntryPrice) * volume

	// PNL шорта = (цена входа - текущий Ask) * объём
	pnlShort := (shortEntryPrice - shortPrice.AskPrice) * volume

	return pnlLong + pnlShort
}

// ============================================================
// OrderBookAnalyzer - анализ стакана ордеров
// ============================================================
//
// Согласно ТЗ раздел "Проверка глубины рынка (ликвидности)":
// - Анализ стакана ордеров (Level 2) - 5 уровней глубины
// - Суммирование объёма до достижения требуемого объёма
// - Расчёт средневзвешенной цены (VWAP)
// - Моделирование исполнения рыночного ордера
// - Оценка slippage

// OrderBookAnalyzer анализирует стаканы ордеров для оценки ликвидности
type OrderBookAnalyzer struct {
	// Кэш стаканов по биржам (обновляется через WebSocket)
	// Key: OrderBookKey{Symbol, Exchange}
	orderBooks sync.Map

	// Глубина анализа (количество уровней)
	depth int

	// Максимальное время актуальности стакана
	maxAge time.Duration
}

// OrderBookKey - ключ для кэша стаканов
type OrderBookKey struct {
	Symbol   string
	Exchange string
}

// CachedOrderBook - кэшированный стакан с timestamp
type CachedOrderBook struct {
	Bids      []PriceLevel // заявки на покупку (от высокой к низкой)
	Asks      []PriceLevel // заявки на продажу (от низкой к высокой)
	Timestamp time.Time
}

// PriceLevel - уровень цены в стакане
type PriceLevel struct {
	Price  float64
	Volume float64
}

// ExecutionSimulation - результат моделирования исполнения
type ExecutionSimulation struct {
	// Средневзвешенная цена исполнения (VWAP)
	AvgPrice float64

	// Сколько объёма реально можно исполнить
	FillableVolume float64

	// Slippage относительно лучшей цены (в %)
	Slippage float64

	// Достаточно ли ликвидности для полного объёма
	FullyFillable bool

	// Количество использованных уровней
	LevelsUsed int
}

// LiquidityAnalysis - полный анализ ликвидности для арбитража
type LiquidityAnalysis struct {
	Symbol string
	Volume float64 // запрошенный объём

	// Анализ для лонга (покупка по Ask)
	LongExchange  string
	LongSimulation *ExecutionSimulation

	// Анализ для шорта (продажа по Bid)
	ShortExchange  string
	ShortSimulation *ExecutionSimulation

	// Итоговые показатели
	IsLiquidityOK   bool    // достаточно ликвидности на обеих биржах
	AdjustedSpread  float64 // спред с учётом slippage (%)
	EstimatedProfit float64 // примерная прибыль в USDT

	// Предупреждения
	Warnings []string
}

// NewOrderBookAnalyzer создаёт анализатор стаканов
// depth - количество уровней для анализа (по умолчанию 5)
// maxAge - максимальный возраст данных (по умолчанию 5 секунд)
func NewOrderBookAnalyzer(depth int, maxAge time.Duration) *OrderBookAnalyzer {
	if depth <= 0 {
		depth = 5 // по умолчанию 5 уровней согласно ТЗ
	}
	if maxAge <= 0 {
		maxAge = 5 * time.Second
	}

	return &OrderBookAnalyzer{
		depth:  depth,
		maxAge: maxAge,
	}
}

// UpdateOrderBook обновляет кэшированный стакан
// Вызывается из WebSocket обработчика при получении данных
func (oba *OrderBookAnalyzer) UpdateOrderBook(symbol, exchange string, bids, asks []PriceLevel) {
	key := OrderBookKey{Symbol: symbol, Exchange: exchange}

	// Ограничиваем глубину
	if len(bids) > oba.depth {
		bids = bids[:oba.depth]
	}
	if len(asks) > oba.depth {
		asks = asks[:oba.depth]
	}

	cached := &CachedOrderBook{
		Bids:      bids,
		Asks:      asks,
		Timestamp: time.Now(),
	}

	oba.orderBooks.Store(key, cached)
}

// GetOrderBook возвращает кэшированный стакан (если актуален)
func (oba *OrderBookAnalyzer) GetOrderBook(symbol, exchange string) *CachedOrderBook {
	key := OrderBookKey{Symbol: symbol, Exchange: exchange}

	if v, ok := oba.orderBooks.Load(key); ok {
		cached := v.(*CachedOrderBook)
		// Проверяем актуальность
		if time.Since(cached.Timestamp) <= oba.maxAge {
			return cached
		}
	}
	return nil
}

// SimulateBuy моделирует покупку (market buy) заданного объёма
// Проходит по Ask уровням от лучшей цены вверх
//
// Возвращает:
// - Средневзвешенную цену покупки (VWAP)
// - Slippage относительно лучшего Ask
// - Можно ли исполнить весь объём
func (oba *OrderBookAnalyzer) SimulateBuy(symbol, exchange string, volume float64) *ExecutionSimulation {
	ob := oba.GetOrderBook(symbol, exchange)
	if ob == nil || len(ob.Asks) == 0 {
		return nil
	}

	return oba.simulateMarketOrder(ob.Asks, volume, true)
}

// SimulateSell моделирует продажу (market sell) заданного объёма
// Проходит по Bid уровням от лучшей цены вниз
//
// Возвращает:
// - Средневзвешенную цену продажи (VWAP)
// - Slippage относительно лучшего Bid
// - Можно ли исполнить весь объём
func (oba *OrderBookAnalyzer) SimulateSell(symbol, exchange string, volume float64) *ExecutionSimulation {
	ob := oba.GetOrderBook(symbol, exchange)
	if ob == nil || len(ob.Bids) == 0 {
		return nil
	}

	return oba.simulateMarketOrder(ob.Bids, volume, false)
}

// simulateMarketOrder общая логика моделирования исполнения
// isBuy: true = покупка (идём по Asks), false = продажа (идём по Bids)
func (oba *OrderBookAnalyzer) simulateMarketOrder(levels []PriceLevel, volume float64, isBuy bool) *ExecutionSimulation {
	if len(levels) == 0 || volume <= 0 {
		return nil
	}

	result := &ExecutionSimulation{}

	bestPrice := levels[0].Price
	var totalCost float64    // сумма (price × volume) для VWAP
	var filledVolume float64 // исполненный объём
	remainingVolume := volume

	// Проходим по уровням стакана
	for i, level := range levels {
		if remainingVolume <= 0 {
			break
		}

		result.LevelsUsed = i + 1

		// Сколько можем взять с этого уровня
		takeVolume := level.Volume
		if takeVolume > remainingVolume {
			takeVolume = remainingVolume
		}

		// Накапливаем для VWAP
		totalCost += level.Price * takeVolume
		filledVolume += takeVolume
		remainingVolume -= takeVolume
	}

	// Рассчитываем результаты
	if filledVolume > 0 {
		result.AvgPrice = totalCost / filledVolume
		result.FillableVolume = filledVolume
		result.FullyFillable = remainingVolume <= 0

		// Slippage: разница между VWAP и лучшей ценой
		if isBuy {
			// Для покупки: slippage положительный если VWAP > bestAsk
			result.Slippage = (result.AvgPrice - bestPrice) / bestPrice * 100
		} else {
			// Для продажи: slippage положительный если VWAP < bestBid
			result.Slippage = (bestPrice - result.AvgPrice) / bestPrice * 100
		}
	}

	return result
}

// AnalyzeLiquidity выполняет полный анализ ликвидности для арбитража
//
// Согласно ТЗ:
// - На дешёвой бирже (long): проходим по Asks, считаем VWAP для покупки
// - На дорогой бирже (short): проходим по Bids, считаем VWAP для продажи
// - Рассчитываем реальный спред с учётом slippage
func (oba *OrderBookAnalyzer) AnalyzeLiquidity(
	symbol string,
	volume float64,
	longExchange string,  // биржа для лонга (покупка)
	shortExchange string, // биржа для шорта (продажа)
) *LiquidityAnalysis {
	analysis := &LiquidityAnalysis{
		Symbol:        symbol,
		Volume:        volume,
		LongExchange:  longExchange,
		ShortExchange: shortExchange,
		Warnings:      make([]string, 0),
	}

	// Моделируем покупку на бирже для лонга
	analysis.LongSimulation = oba.SimulateBuy(symbol, longExchange, volume)
	if analysis.LongSimulation == nil {
		analysis.Warnings = append(analysis.Warnings,
			"no orderbook data for "+longExchange)
		return analysis
	}

	// Моделируем продажу на бирже для шорта
	analysis.ShortSimulation = oba.SimulateSell(symbol, shortExchange, volume)
	if analysis.ShortSimulation == nil {
		analysis.Warnings = append(analysis.Warnings,
			"no orderbook data for "+shortExchange)
		return analysis
	}

	// Проверяем достаточность ликвидности
	longOK := analysis.LongSimulation.FullyFillable
	shortOK := analysis.ShortSimulation.FullyFillable
	analysis.IsLiquidityOK = longOK && shortOK

	if !longOK {
		analysis.Warnings = append(analysis.Warnings,
			"insufficient liquidity on "+longExchange+
				" (available: "+formatVolume(analysis.LongSimulation.FillableVolume)+")")
	}
	if !shortOK {
		analysis.Warnings = append(analysis.Warnings,
			"insufficient liquidity on "+shortExchange+
				" (available: "+formatVolume(analysis.ShortSimulation.FillableVolume)+")")
	}

	// Рассчитываем реальный спред с учётом VWAP
	// Спред = (VWAP_sell - VWAP_buy) / VWAP_buy × 100
	if analysis.LongSimulation.AvgPrice > 0 {
		analysis.AdjustedSpread = (analysis.ShortSimulation.AvgPrice -
			analysis.LongSimulation.AvgPrice) / analysis.LongSimulation.AvgPrice * 100

		// Примерная прибыль (без учёта комиссий)
		analysis.EstimatedProfit = (analysis.ShortSimulation.AvgPrice -
			analysis.LongSimulation.AvgPrice) * volume
	}

	// Предупреждение о высоком slippage
	totalSlippage := analysis.LongSimulation.Slippage + analysis.ShortSimulation.Slippage
	if totalSlippage > 0.1 { // > 0.1%
		analysis.Warnings = append(analysis.Warnings,
			"high total slippage: "+formatPercent(totalSlippage))
	}

	return analysis
}

// CheckLiquidityForVolume проверяет достаточность ликвидности
// Быстрая проверка без полного анализа
func (oba *OrderBookAnalyzer) CheckLiquidityForVolume(
	symbol string,
	volume float64,
	longExchange string,
	shortExchange string,
) (bool, string) {
	// Проверяем ликвидность для покупки (long)
	longSim := oba.SimulateBuy(symbol, longExchange, volume)
	if longSim == nil {
		return false, "no orderbook for " + longExchange
	}
	if !longSim.FullyFillable {
		return false, "insufficient liquidity on " + longExchange
	}

	// Проверяем ликвидность для продажи (short)
	shortSim := oba.SimulateSell(symbol, shortExchange, volume)
	if shortSim == nil {
		return false, "no orderbook for " + shortExchange
	}
	if !shortSim.FullyFillable {
		return false, "insufficient liquidity on " + shortExchange
	}

	return true, ""
}

// GetRealSpread рассчитывает реальный спред с учётом объёма и slippage
// Используется вместо GetBestOpportunity когда важна точность
func (sc *SpreadCalculator) GetRealSpread(
	symbol string,
	volume float64,
	orderBookAnalyzer *OrderBookAnalyzer,
) *ArbitrageOpportunity {
	// Сначала получаем базовую возможность (лучшие цены)
	best := sc.tracker.GetBestPrices(symbol)
	if best == nil || best.BestAskExch == best.BestBidExch || best.RawSpread <= 0 {
		return nil
	}

	// Анализируем ликвидность
	analysis := orderBookAnalyzer.AnalyzeLiquidity(
		symbol, volume, best.BestAskExch, best.BestBidExch)

	if analysis == nil || !analysis.IsLiquidityOK {
		return nil
	}

	// Рассчитываем чистый спред с учётом slippage и комиссий
	adjustedSpread := analysis.AdjustedSpread

	// Вычитаем комиссии
	sc.feesMu.RLock()
	feeLong := sc.fees[best.BestAskExch]
	feeShort := sc.fees[best.BestBidExch]
	sc.feesMu.RUnlock()

	if feeLong == 0 {
		feeLong = 0.0005
	}
	if feeShort == 0 {
		feeShort = 0.0005
	}

	totalFees := 2 * (feeLong + feeShort) * 100
	netSpread := adjustedSpread - totalFees

	return &ArbitrageOpportunity{
		Symbol:        symbol,
		LongExchange:  best.BestAskExch,
		LongPrice:     analysis.LongSimulation.AvgPrice, // VWAP вместо лучшей цены
		ShortExchange: best.BestBidExch,
		ShortPrice:    analysis.ShortSimulation.AvgPrice, // VWAP вместо лучшей цены
		RawSpread:     adjustedSpread,                    // спред с учётом slippage
		NetSpread:     netSpread,                         // минус комиссии
		Timestamp:     best.BestAskTime,
	}
}

// ============================================================
// Вспомогательные функции форматирования
// ============================================================

func formatVolume(v float64) string {
	if v >= 1 {
		return fmt.Sprintf("%.4f", v)
	}
	return fmt.Sprintf("%.8f", v)
}

func formatPercent(p float64) string {
	return fmt.Sprintf("%.4f%%", p)
}

// ============================================================
// Расширение SpreadCalculator для работы с OrderBookAnalyzer
// ============================================================

// SpreadWithLiquidity - расширенная информация о спреде
type SpreadWithLiquidity struct {
	*ArbitrageOpportunity

	// Данные о ликвидности
	LongSlippage   float64 // slippage при покупке (%)
	ShortSlippage  float64 // slippage при продаже (%)
	TotalSlippage  float64 // суммарный slippage (%)
	IsLiquidityOK  bool    // достаточно ликвидности
	LiquidityIssue string  // описание проблемы (если есть)
}

// GetSpreadWithLiquidity возвращает спред с учётом ликвидности
func (sc *SpreadCalculator) GetSpreadWithLiquidity(
	symbol string,
	volume float64,
	orderBookAnalyzer *OrderBookAnalyzer,
) *SpreadWithLiquidity {
	// Базовая возможность
	opp := sc.GetBestOpportunity(symbol)
	if opp == nil {
		return nil
	}

	result := &SpreadWithLiquidity{
		ArbitrageOpportunity: opp,
		IsLiquidityOK:        true,
	}

	// Если нет анализатора стаканов - возвращаем базовые данные
	if orderBookAnalyzer == nil {
		return result
	}

	// Проверяем ликвидность
	ok, issue := orderBookAnalyzer.CheckLiquidityForVolume(
		symbol, volume, opp.LongExchange, opp.ShortExchange)

	result.IsLiquidityOK = ok
	result.LiquidityIssue = issue

	if !ok {
		return result
	}

	// Получаем детальный анализ для slippage
	longSim := orderBookAnalyzer.SimulateBuy(symbol, opp.LongExchange, volume)
	shortSim := orderBookAnalyzer.SimulateSell(symbol, opp.ShortExchange, volume)

	if longSim != nil {
		result.LongSlippage = longSim.Slippage
		// Обновляем цену на VWAP
		opp.LongPrice = longSim.AvgPrice
	}

	if shortSim != nil {
		result.ShortSlippage = shortSim.Slippage
		// Обновляем цену на VWAP
		opp.ShortPrice = shortSim.AvgPrice
	}

	result.TotalSlippage = result.LongSlippage + result.ShortSlippage

	// Пересчитываем спред с VWAP ценами
	if opp.LongPrice > 0 {
		opp.RawSpread = (opp.ShortPrice - opp.LongPrice) / opp.LongPrice * 100
		opp.NetSpread = sc.calculateNetSpreadFromPrices(
			opp.RawSpread, opp.LongExchange, opp.ShortExchange)
	}

	return result
}

// calculateNetSpreadFromPrices вычисляет чистый спред для заданных бирж
func (sc *SpreadCalculator) calculateNetSpreadFromPrices(
	rawSpread float64,
	longExchange string,
	shortExchange string,
) float64 {
	sc.feesMu.RLock()
	feeLong := sc.fees[longExchange]
	feeShort := sc.fees[shortExchange]
	sc.feesMu.RUnlock()

	if feeLong == 0 {
		feeLong = 0.0005
	}
	if feeShort == 0 {
		feeShort = 0.0005
	}

	totalFees := 2 * (feeLong + feeShort) * 100
	return rawSpread - totalFees
}
