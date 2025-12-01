package bot

import (
	"sync"
	"time"
)

// ============ ОПТИМИЗАЦИЯ: Inline FNV-1a hash без аллокаций ============
// Константы FNV-1a для 32-битного хэша
const (
	fnvOffset32 = uint32(2166136261)
	fnvPrime32  = uint32(16777619)
)

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
func (pt *PriceTracker) Update(update PriceUpdate) {
	shard := pt.getShard(update.Symbol)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	key := PriceKey{
		Symbol:   update.Symbol,
		Exchange: update.Exchange,
	}

	// Добавляем в индекс если это новая биржа для символа
	if _, exists := shard.allPrices[key]; !exists {
		shard.symbolIndex[update.Symbol] = append(shard.symbolIndex[update.Symbol], key)
	}

	// Сохраняем цену этой биржи
	shard.allPrices[key] = &ExchangePrice{
		Exchange:  update.Exchange,
		Symbol:    update.Symbol,
		BidPrice:  update.BidPrice,
		AskPrice:  update.AskPrice,
		Timestamp: update.Timestamp,
	}

	// Пересчитываем лучшие цены для этого символа
	shard.recalculateBest(update.Symbol)
}

// UpdateFromPtr обновляет цену от указателя (для использования с sync.Pool)
// Сложность: O(k) где k = количество бирж для символа (обычно 6)
func (pt *PriceTracker) UpdateFromPtr(update *PriceUpdate) {
	shard := pt.getShard(update.Symbol)

	shard.mu.Lock()
	defer shard.mu.Unlock()

	key := PriceKey{
		Symbol:   update.Symbol,
		Exchange: update.Exchange,
	}

	// Добавляем в индекс если это новая биржа для символа
	if _, exists := shard.allPrices[key]; !exists {
		shard.symbolIndex[update.Symbol] = append(shard.symbolIndex[update.Symbol], key)
	}

	// Сохраняем цену этой биржи
	shard.allPrices[key] = &ExchangePrice{
		Exchange:  update.Exchange,
		Symbol:    update.Symbol,
		BidPrice:  update.BidPrice,
		AskPrice:  update.AskPrice,
		Timestamp: update.Timestamp,
	}

	// Пересчитываем лучшие цены для этого символа
	shard.recalculateBest(update.Symbol)
}

// recalculateBest пересчитывает лучшие Ask и Bid для символа
// Сложность: O(k) где k = количество бирж для этого символа
// ВАЖНО: вызывается под lock'ом шарда
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

// GetBestPrices возвращает лучшие цены для символа
// Сложность: O(1)
func (pt *PriceTracker) GetBestPrices(symbol string) *BestPrices {
	shard := pt.getShard(symbol)
	shard.mu.RLock()
	defer shard.mu.RUnlock()
	return shard.bestPrices[symbol]
}

// GetExchangePrice возвращает цену конкретной биржи
func (pt *PriceTracker) GetExchangePrice(symbol, exchange string) *ExchangePrice {
	shard := pt.getShard(symbol)
	shard.mu.RLock()
	defer shard.mu.RUnlock()

	key := PriceKey{Symbol: symbol, Exchange: exchange}
	return shard.allPrices[key]
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
