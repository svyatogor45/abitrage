package bot

import (
	"sync"
	"time"
)

// PriceTracker - глобальный трекер лучших цен для O(n) поиска связки
//
// Архитектура:
// - Хранит лучший Ask (для лонга) и лучший Bid (для шорта) по каждому символу
// - При обновлении цены от любой биржи - O(1) обновление лучших цен
// - Поиск лучшей арбитражной связки: O(1) - просто берём лучший Ask и лучший Bid
//
// Почему это O(n) а не O(n²):
// - БЕЗ оптимизации: для каждой биржи сравниваем со всеми другими = O(n²)
// - С оптимизацией: храним только лучшие цены, сравнение = O(1)
type PriceTracker struct {
	// Лучшие цены по символам
	// key: symbol, value: лучшие Ask и Bid со всех бирж
	bestPrices map[string]*BestPrices

	// Все цены по биржам (для пересчёта при обновлении)
	// key: "symbol:exchange"
	allPrices map[string]*ExchangePrice

	mu sync.RWMutex
}

// BestPrices - лучшие цены для символа
type BestPrices struct {
	Symbol string

	// Лучший Ask (самый низкий) - для открытия лонга
	BestAsk      float64
	BestAskExch  string
	BestAskTime  time.Time

	// Лучший Bid (самый высокий) - для открытия шорта
	BestBid      float64
	BestBidExch  string
	BestBidTime  time.Time

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

// NewPriceTracker создаёт новый трекер
func NewPriceTracker() *PriceTracker {
	return &PriceTracker{
		bestPrices: make(map[string]*BestPrices),
		allPrices:  make(map[string]*ExchangePrice),
	}
}

// Update обновляет цену от биржи и пересчитывает лучшие цены
// Сложность: O(n) где n = количество бирж (обычно 6)
func (pt *PriceTracker) Update(update PriceUpdate) {
	pt.mu.Lock()
	defer pt.mu.Unlock()

	key := update.Symbol + ":" + update.Exchange

	// Сохраняем цену этой биржи
	pt.allPrices[key] = &ExchangePrice{
		Exchange:  update.Exchange,
		Symbol:    update.Symbol,
		BidPrice:  update.BidPrice,
		AskPrice:  update.AskPrice,
		Timestamp: update.Timestamp,
	}

	// Пересчитываем лучшие цены для этого символа
	pt.recalculateBest(update.Symbol)
}

// recalculateBest пересчитывает лучшие Ask и Bid для символа
// Сложность: O(n) где n = количество бирж
func (pt *PriceTracker) recalculateBest(symbol string) {
	var bestAsk float64 = 0
	var bestAskExch string
	var bestAskTime time.Time

	var bestBid float64 = 0
	var bestBidExch string
	var bestBidTime time.Time

	// Проходим по всем биржам для этого символа
	for key, price := range pt.allPrices {
		if price.Symbol != symbol {
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

		_ = key // используем key для итерации
	}

	// Сохраняем лучшие цены
	if bestAsk > 0 && bestBid > 0 {
		rawSpread := (bestBid - bestAsk) / bestAsk * 100

		pt.bestPrices[symbol] = &BestPrices{
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
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.bestPrices[symbol]
}

// GetExchangePrice возвращает цену конкретной биржи
func (pt *PriceTracker) GetExchangePrice(symbol, exchange string) *ExchangePrice {
	pt.mu.RLock()
	defer pt.mu.RUnlock()
	return pt.allPrices[symbol+":"+exchange]
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
		Timestamp:     time.Now(),
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
	totalFees := 2*(feeLong+feeShort) * 100 // в процентах

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
