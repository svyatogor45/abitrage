package bot

import (
	"testing"
	"time"
)

// ============================================================
// PriceTracker Tests
// ============================================================

func TestNewPriceTracker(t *testing.T) {
	tests := []struct {
		name      string
		numShards int
		expected  int
	}{
		{"default shards", 0, 16},
		{"negative shards", -5, 16},
		{"custom shards", 8, 8},
		{"large shards", 32, 32},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pt := NewPriceTracker(tt.numShards)
			if pt == nil {
				t.Fatal("NewPriceTracker returned nil")
			}
			if int(pt.numShards) != tt.expected {
				t.Errorf("expected %d shards, got %d", tt.expected, pt.numShards)
			}
			if len(pt.shards) != tt.expected {
				t.Errorf("expected %d shard objects, got %d", tt.expected, len(pt.shards))
			}
		})
	}
}

func TestPriceTrackerUpdate(t *testing.T) {
	pt := NewPriceTracker(4)

	// Обновляем цены с двух бирж
	now := time.Now()

	pt.Update(PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "BTCUSDT",
		BidPrice:  50000.0,
		AskPrice:  50010.0,
		Timestamp: now,
	})

	pt.Update(PriceUpdate{
		Exchange:  "okx",
		Symbol:    "BTCUSDT",
		BidPrice:  50050.0, // Bid выше - для шорта
		AskPrice:  50005.0, // Ask ниже - для лонга
		Timestamp: now,
	})

	// Проверяем лучшие цены
	best := pt.GetBestPrices("BTCUSDT")
	if best == nil {
		t.Fatal("GetBestPrices returned nil")
	}

	// Лучший Ask (минимальный) должен быть с OKX = 50005
	if best.BestAsk != 50005.0 {
		t.Errorf("expected BestAsk=50005, got %f", best.BestAsk)
	}
	if best.BestAskExch != "okx" {
		t.Errorf("expected BestAskExch=okx, got %s", best.BestAskExch)
	}

	// Лучший Bid (максимальный) должен быть с OKX = 50050
	if best.BestBid != 50050.0 {
		t.Errorf("expected BestBid=50050, got %f", best.BestBid)
	}
	if best.BestBidExch != "okx" {
		t.Errorf("expected BestBidExch=okx, got %s", best.BestBidExch)
	}

	// Проверяем расчёт спреда: (50050 - 50005) / 50005 * 100 = 0.08998...%
	expectedSpread := (50050.0 - 50005.0) / 50005.0 * 100
	if abs(best.RawSpread-expectedSpread) > 0.0001 {
		t.Errorf("expected RawSpread=%f, got %f", expectedSpread, best.RawSpread)
	}
}

func TestPriceTrackerUpdateFromPtr(t *testing.T) {
	pt := NewPriceTracker(4)

	update := &PriceUpdate{
		Exchange:  "bitget",
		Symbol:    "ETHUSDT",
		BidPrice:  3000.0,
		AskPrice:  3001.0,
		Timestamp: time.Now(),
	}

	pt.UpdateFromPtr(update)

	best := pt.GetBestPrices("ETHUSDT")
	if best == nil {
		t.Fatal("GetBestPrices returned nil after UpdateFromPtr")
	}

	if best.BestAsk != 3001.0 {
		t.Errorf("expected BestAsk=3001, got %f", best.BestAsk)
	}
}

func TestPriceTrackerGetExchangePrice(t *testing.T) {
	pt := NewPriceTracker(4)

	pt.Update(PriceUpdate{
		Exchange:  "gate",
		Symbol:    "XRPUSDT",
		BidPrice:  0.50,
		AskPrice:  0.51,
		Timestamp: time.Now(),
	})

	// Проверяем получение цены для конкретной биржи
	price := pt.GetExchangePrice("XRPUSDT", "gate")
	if price == nil {
		t.Fatal("GetExchangePrice returned nil")
	}

	if price.BidPrice != 0.50 {
		t.Errorf("expected BidPrice=0.50, got %f", price.BidPrice)
	}
	if price.AskPrice != 0.51 {
		t.Errorf("expected AskPrice=0.51, got %f", price.AskPrice)
	}

	// Несуществующая биржа
	nilPrice := pt.GetExchangePrice("XRPUSDT", "unknown")
	if nilPrice != nil {
		t.Error("expected nil for unknown exchange")
	}
}

func TestPriceTrackerGetShardIndex(t *testing.T) {
	pt := NewPriceTracker(8)

	// Проверяем детерминированность шардирования
	idx1 := pt.GetShardIndex("BTCUSDT")
	idx2 := pt.GetShardIndex("BTCUSDT")

	if idx1 != idx2 {
		t.Errorf("shard index should be deterministic: %d != %d", idx1, idx2)
	}

	if idx1 < 0 || idx1 >= 8 {
		t.Errorf("shard index out of range: %d", idx1)
	}
}

func TestPriceTrackerMultipleSymbols(t *testing.T) {
	pt := NewPriceTracker(4)
	now := time.Now()

	// Добавляем разные символы
	symbols := []string{"BTCUSDT", "ETHUSDT", "SOLUSDT"}
	for _, sym := range symbols {
		pt.Update(PriceUpdate{
			Exchange:  "bybit",
			Symbol:    sym,
			BidPrice:  100.0,
			AskPrice:  101.0,
			Timestamp: now,
		})
	}

	// Проверяем что все символы имеют свои цены
	for _, sym := range symbols {
		best := pt.GetBestPrices(sym)
		if best == nil {
			t.Errorf("GetBestPrices returned nil for %s", sym)
		}
	}
}

func TestPriceTrackerInPlaceUpdate(t *testing.T) {
	pt := NewPriceTracker(4)

	// Первое обновление
	pt.Update(PriceUpdate{
		Exchange:  "htx",
		Symbol:    "ADAUSDT",
		BidPrice:  0.30,
		AskPrice:  0.31,
		Timestamp: time.Now(),
	})

	// Второе обновление той же биржи (in-place)
	pt.Update(PriceUpdate{
		Exchange:  "htx",
		Symbol:    "ADAUSDT",
		BidPrice:  0.32,
		AskPrice:  0.33,
		Timestamp: time.Now(),
	})

	price := pt.GetExchangePrice("ADAUSDT", "htx")
	if price.BidPrice != 0.32 {
		t.Errorf("expected updated BidPrice=0.32, got %f", price.BidPrice)
	}
}

// ============================================================
// SpreadCalculator Tests
// ============================================================

func TestNewSpreadCalculator(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)

	if sc == nil {
		t.Fatal("NewSpreadCalculator returned nil")
	}
	if sc.tracker != pt {
		t.Error("tracker not set correctly")
	}
}

func TestSpreadCalculatorSetFee(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)

	sc.SetFee("bybit", 0.0004)  // 0.04%
	sc.SetFee("okx", 0.0005)    // 0.05%

	// Проверяем через внутреннее состояние (fees map)
	sc.feesMu.RLock()
	defer sc.feesMu.RUnlock()

	if sc.fees["bybit"] != 0.0004 {
		t.Errorf("expected bybit fee=0.0004, got %f", sc.fees["bybit"])
	}
	if sc.fees["okx"] != 0.0005 {
		t.Errorf("expected okx fee=0.0005, got %f", sc.fees["okx"])
	}
}

func TestSpreadCalculatorGetBestOpportunity(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)

	sc.SetFee("bybit", 0.0004)
	sc.SetFee("okx", 0.0005)

	now := time.Now()

	// Bybit: Ask ниже (для лонга)
	pt.Update(PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "BTCUSDT",
		BidPrice:  50000.0,
		AskPrice:  50010.0, // Лучший Ask для покупки
		Timestamp: now,
	})

	// OKX: Bid выше (для шорта)
	pt.Update(PriceUpdate{
		Exchange:  "okx",
		Symbol:    "BTCUSDT",
		BidPrice:  50100.0, // Лучший Bid для продажи
		AskPrice:  50120.0,
		Timestamp: now,
	})

	opp := sc.GetBestOpportunity("BTCUSDT")
	if opp == nil {
		t.Fatal("GetBestOpportunity returned nil")
	}

	// Проверяем направления
	if opp.LongExchange != "bybit" {
		t.Errorf("expected LongExchange=bybit, got %s", opp.LongExchange)
	}
	if opp.ShortExchange != "okx" {
		t.Errorf("expected ShortExchange=okx, got %s", opp.ShortExchange)
	}
	if opp.LongPrice != 50010.0 {
		t.Errorf("expected LongPrice=50010, got %f", opp.LongPrice)
	}
	if opp.ShortPrice != 50100.0 {
		t.Errorf("expected ShortPrice=50100, got %f", opp.ShortPrice)
	}

	// Raw spread = (50100 - 50010) / 50010 * 100 = 0.17996...%
	expectedRaw := (50100.0 - 50010.0) / 50010.0 * 100
	if abs(opp.RawSpread-expectedRaw) > 0.0001 {
		t.Errorf("expected RawSpread=%f, got %f", expectedRaw, opp.RawSpread)
	}

	// Net spread = raw - 4 комиссии = raw - 2*(0.04% + 0.05%) = raw - 0.18%
	totalFees := 2 * (0.0004 + 0.0005) * 100 // 0.18%
	expectedNet := expectedRaw - totalFees
	if abs(opp.NetSpread-expectedNet) > 0.0001 {
		t.Errorf("expected NetSpread=%f, got %f", expectedNet, opp.NetSpread)
	}
}

func TestSpreadCalculatorSameExchange(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)

	// Только одна биржа - нельзя арбитражить
	pt.Update(PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "ETHUSDT",
		BidPrice:  3000.0,
		AskPrice:  3001.0,
		Timestamp: time.Now(),
	})

	opp := sc.GetBestOpportunity("ETHUSDT")
	if opp != nil {
		t.Error("expected nil when same exchange for bid/ask")
	}
}

func TestSpreadCalculatorNegativeSpread(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)

	now := time.Now()

	// Ask > Bid (отрицательный спред - нет арбитража)
	pt.Update(PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "DOTUSDT",
		BidPrice:  10.0,
		AskPrice:  11.0, // Ask выше Bid
		Timestamp: now,
	})

	pt.Update(PriceUpdate{
		Exchange:  "okx",
		Symbol:    "DOTUSDT",
		BidPrice:  9.0,  // Bid ниже чем Ask bybit
		AskPrice:  10.5,
		Timestamp: now,
	})

	opp := sc.GetBestOpportunity("DOTUSDT")
	if opp != nil {
		t.Error("expected nil for negative spread")
	}
}

func TestSpreadCalculatorGetCurrentSpread(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)

	now := time.Now()

	pt.Update(PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "LINKUSDT",
		BidPrice:  15.0,
		AskPrice:  15.1,
		Timestamp: now,
	})

	pt.Update(PriceUpdate{
		Exchange:  "okx",
		Symbol:    "LINKUSDT",
		BidPrice:  14.8,
		AskPrice:  14.9,
		Timestamp: now,
	})

	// Для выхода: продаём лонг по Bid_long, покупаем шорт по Ask_short
	// Spread = (Bid_bybit - Ask_okx) / Ask_okx * 100
	// = (15.0 - 14.9) / 14.9 * 100 = 0.6711...%
	spread := sc.GetCurrentSpread("LINKUSDT", "bybit", "okx")

	expected := (15.0 - 14.9) / 14.9 * 100
	if abs(spread-expected) > 0.0001 {
		t.Errorf("expected CurrentSpread=%f, got %f", expected, spread)
	}
}

func TestSpreadCalculatorGetCurrentSpreadMissing(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)

	// Нет данных
	spread := sc.GetCurrentSpread("MISSING", "bybit", "okx")
	if spread != 0 {
		t.Errorf("expected 0 for missing data, got %f", spread)
	}
}

func TestSpreadCalculatorCalculatePnl(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)

	now := time.Now()

	// Текущие цены
	pt.Update(PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "AVAXUSDT",
		BidPrice:  35.0, // Для продажи лонга
		AskPrice:  35.1,
		Timestamp: now,
	})

	pt.Update(PriceUpdate{
		Exchange:  "okx",
		Symbol:    "AVAXUSDT",
		BidPrice:  34.5,
		AskPrice:  34.6, // Для покупки шорта
		Timestamp: now,
	})

	// Входы: лонг на bybit по 34.0, шорт на okx по 36.0, объём 10
	pnl := sc.CalculatePnl(
		"AVAXUSDT",
		"bybit", 34.0, // лонг вход
		"okx", 36.0,   // шорт вход
		10.0,          // объём
	)

	// PNL лонга = (35.0 - 34.0) * 10 = 10.0
	// PNL шорта = (36.0 - 34.6) * 10 = 14.0
	// Итого = 24.0
	expectedPnl := (35.0-34.0)*10.0 + (36.0-34.6)*10.0
	if abs(pnl-expectedPnl) > 0.0001 {
		t.Errorf("expected PNL=%f, got %f", expectedPnl, pnl)
	}
}

func TestSpreadCalculatorDefaultFees(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)

	// Не устанавливаем комиссии - должен использоваться дефолт 0.05%
	now := time.Now()

	pt.Update(PriceUpdate{
		Exchange:  "newexch1",
		Symbol:    "TESTUSDT",
		BidPrice:  100.0,
		AskPrice:  100.1,
		Timestamp: now,
	})

	pt.Update(PriceUpdate{
		Exchange:  "newexch2",
		Symbol:    "TESTUSDT",
		BidPrice:  100.5,
		AskPrice:  100.6,
		Timestamp: now,
	})

	opp := sc.GetBestOpportunity("TESTUSDT")
	if opp == nil {
		t.Fatal("GetBestOpportunity returned nil")
	}

	// Дефолт комиссии: 0.0005 (0.05%)
	// totalFees = 2 * (0.0005 + 0.0005) * 100 = 0.2%
	expectedNet := opp.RawSpread - 0.2
	if abs(opp.NetSpread-expectedNet) > 0.0001 {
		t.Errorf("expected NetSpread with default fees=%f, got %f", expectedNet, opp.NetSpread)
	}
}

// ============================================================
// OrderBookAnalyzer Tests
// ============================================================

func TestNewOrderBookAnalyzer(t *testing.T) {
	tests := []struct {
		name     string
		depth    int
		maxAge   time.Duration
		expDepth int
		expAge   time.Duration
	}{
		{"default values", 0, 0, 5, 5 * time.Second},
		{"negative depth", -1, 0, 5, 5 * time.Second},
		{"custom values", 10, 10 * time.Second, 10, 10 * time.Second},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			oba := NewOrderBookAnalyzer(tt.depth, tt.maxAge)
			if oba == nil {
				t.Fatal("NewOrderBookAnalyzer returned nil")
			}
			if oba.depth != tt.expDepth {
				t.Errorf("expected depth=%d, got %d", tt.expDepth, oba.depth)
			}
			if oba.maxAge != tt.expAge {
				t.Errorf("expected maxAge=%v, got %v", tt.expAge, oba.maxAge)
			}
		})
	}
}

func TestOrderBookAnalyzerUpdateAndGet(t *testing.T) {
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	bids := []PriceLevel{
		{Price: 50000.0, Volume: 1.0},
		{Price: 49999.0, Volume: 2.0},
		{Price: 49998.0, Volume: 3.0},
	}
	asks := []PriceLevel{
		{Price: 50001.0, Volume: 0.5},
		{Price: 50002.0, Volume: 1.5},
		{Price: 50003.0, Volume: 2.5},
	}

	oba.UpdateOrderBook("BTCUSDT", "bybit", bids, asks)

	// Получаем обратно
	ob := oba.GetOrderBook("BTCUSDT", "bybit")
	if ob == nil {
		t.Fatal("GetOrderBook returned nil")
	}

	if len(ob.Bids) != 3 {
		t.Errorf("expected 3 bids, got %d", len(ob.Bids))
	}
	if len(ob.Asks) != 3 {
		t.Errorf("expected 3 asks, got %d", len(ob.Asks))
	}

	if ob.Bids[0].Price != 50000.0 {
		t.Errorf("expected first bid=50000, got %f", ob.Bids[0].Price)
	}
	if ob.Asks[0].Price != 50001.0 {
		t.Errorf("expected first ask=50001, got %f", ob.Asks[0].Price)
	}
}

func TestOrderBookAnalyzerDepthLimit(t *testing.T) {
	oba := NewOrderBookAnalyzer(3, 5*time.Second) // лимит 3 уровня

	bids := make([]PriceLevel, 10)
	asks := make([]PriceLevel, 10)
	for i := 0; i < 10; i++ {
		bids[i] = PriceLevel{Price: float64(1000 - i), Volume: 1.0}
		asks[i] = PriceLevel{Price: float64(1001 + i), Volume: 1.0}
	}

	oba.UpdateOrderBook("TESTUSDT", "okx", bids, asks)

	ob := oba.GetOrderBook("TESTUSDT", "okx")
	if ob == nil {
		t.Fatal("GetOrderBook returned nil")
	}

	// Должно быть обрезано до 3 уровней
	if len(ob.Bids) != 3 {
		t.Errorf("expected 3 bids after limit, got %d", len(ob.Bids))
	}
	if len(ob.Asks) != 3 {
		t.Errorf("expected 3 asks after limit, got %d", len(ob.Asks))
	}
}

func TestOrderBookAnalyzerExpiry(t *testing.T) {
	oba := NewOrderBookAnalyzer(5, 50*time.Millisecond) // очень короткий maxAge

	bids := []PriceLevel{{Price: 100.0, Volume: 1.0}}
	asks := []PriceLevel{{Price: 101.0, Volume: 1.0}}

	oba.UpdateOrderBook("EXPIRETEST", "gate", bids, asks)

	// Сразу должен быть доступен
	ob := oba.GetOrderBook("EXPIRETEST", "gate")
	if ob == nil {
		t.Fatal("GetOrderBook should return data immediately after update")
	}

	// Ждём истечения
	time.Sleep(60 * time.Millisecond)

	// Теперь должен быть nil (устарел)
	ob = oba.GetOrderBook("EXPIRETEST", "gate")
	if ob != nil {
		t.Error("GetOrderBook should return nil for expired data")
	}
}

func TestOrderBookAnalyzerSimulateBuy(t *testing.T) {
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	// Стакан Asks: покупаем по Ask
	asks := []PriceLevel{
		{Price: 100.0, Volume: 1.0},  // level 0
		{Price: 100.5, Volume: 2.0},  // level 1
		{Price: 101.0, Volume: 3.0},  // level 2
	}
	bids := []PriceLevel{{Price: 99.0, Volume: 1.0}}

	oba.UpdateOrderBook("SIMTEST", "bybit", bids, asks)

	// Покупаем 2.5 единицы
	// Берём 1.0 @ 100.0 + 1.5 @ 100.5
	// VWAP = (100*1 + 100.5*1.5) / 2.5 = (100 + 150.75) / 2.5 = 100.3
	sim := oba.SimulateBuy("SIMTEST", "bybit", 2.5)
	if sim == nil {
		t.Fatal("SimulateBuy returned nil")
	}

	expectedVWAP := (100.0*1.0 + 100.5*1.5) / 2.5
	if abs(sim.AvgPrice-expectedVWAP) > 0.0001 {
		t.Errorf("expected AvgPrice=%f, got %f", expectedVWAP, sim.AvgPrice)
	}

	if sim.FillableVolume != 2.5 {
		t.Errorf("expected FillableVolume=2.5, got %f", sim.FillableVolume)
	}

	if !sim.FullyFillable {
		t.Error("expected FullyFillable=true")
	}

	// Slippage = (VWAP - bestAsk) / bestAsk * 100
	expectedSlippage := (expectedVWAP - 100.0) / 100.0 * 100
	if abs(sim.Slippage-expectedSlippage) > 0.0001 {
		t.Errorf("expected Slippage=%f, got %f", expectedSlippage, sim.Slippage)
	}

	if sim.LevelsUsed != 2 {
		t.Errorf("expected LevelsUsed=2, got %d", sim.LevelsUsed)
	}
}

func TestOrderBookAnalyzerSimulateSell(t *testing.T) {
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	// Стакан Bids: продаём по Bid
	bids := []PriceLevel{
		{Price: 100.0, Volume: 2.0},  // level 0 (лучший)
		{Price: 99.5, Volume: 3.0},   // level 1
		{Price: 99.0, Volume: 5.0},   // level 2
	}
	asks := []PriceLevel{{Price: 101.0, Volume: 1.0}}

	oba.UpdateOrderBook("SELLTEST", "okx", bids, asks)

	// Продаём 4.0 единицы
	// Берём 2.0 @ 100.0 + 2.0 @ 99.5
	// VWAP = (100*2 + 99.5*2) / 4 = 399 / 4 = 99.75
	sim := oba.SimulateSell("SELLTEST", "okx", 4.0)
	if sim == nil {
		t.Fatal("SimulateSell returned nil")
	}

	expectedVWAP := (100.0*2.0 + 99.5*2.0) / 4.0
	if abs(sim.AvgPrice-expectedVWAP) > 0.0001 {
		t.Errorf("expected AvgPrice=%f, got %f", expectedVWAP, sim.AvgPrice)
	}

	// Slippage для продажи = (bestBid - VWAP) / bestBid * 100
	expectedSlippage := (100.0 - expectedVWAP) / 100.0 * 100
	if abs(sim.Slippage-expectedSlippage) > 0.0001 {
		t.Errorf("expected Slippage=%f, got %f", expectedSlippage, sim.Slippage)
	}
}

func TestOrderBookAnalyzerInsufficientLiquidity(t *testing.T) {
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	// Мало ликвидности
	asks := []PriceLevel{
		{Price: 100.0, Volume: 1.0},
		{Price: 101.0, Volume: 1.0},
	}
	bids := []PriceLevel{{Price: 99.0, Volume: 1.0}}

	oba.UpdateOrderBook("LOWLIQ", "htx", bids, asks)

	// Пытаемся купить 5.0 (доступно только 2.0)
	sim := oba.SimulateBuy("LOWLIQ", "htx", 5.0)
	if sim == nil {
		t.Fatal("SimulateBuy returned nil")
	}

	if sim.FullyFillable {
		t.Error("expected FullyFillable=false for insufficient liquidity")
	}

	if sim.FillableVolume != 2.0 {
		t.Errorf("expected FillableVolume=2.0, got %f", sim.FillableVolume)
	}
}

func TestOrderBookAnalyzerEmptyOrderBook(t *testing.T) {
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	// Нет данных
	sim := oba.SimulateBuy("NODATA", "bingx", 1.0)
	if sim != nil {
		t.Error("expected nil for missing orderbook")
	}
}

func TestOrderBookAnalyzerZeroVolume(t *testing.T) {
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	asks := []PriceLevel{{Price: 100.0, Volume: 1.0}}
	bids := []PriceLevel{{Price: 99.0, Volume: 1.0}}
	oba.UpdateOrderBook("ZEROVOL", "gate", bids, asks)

	sim := oba.SimulateBuy("ZEROVOL", "gate", 0)
	if sim != nil {
		t.Error("expected nil for zero volume")
	}
}

func TestOrderBookAnalyzerAnalyzeLiquidity(t *testing.T) {
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	// Long exchange (покупаем)
	longAsks := []PriceLevel{
		{Price: 100.0, Volume: 5.0},
		{Price: 100.1, Volume: 5.0},
	}
	longBids := []PriceLevel{{Price: 99.0, Volume: 1.0}}
	oba.UpdateOrderBook("LIQTEST", "bybit", longBids, longAsks)

	// Short exchange (продаём)
	shortBids := []PriceLevel{
		{Price: 100.5, Volume: 5.0},
		{Price: 100.4, Volume: 5.0},
	}
	shortAsks := []PriceLevel{{Price: 101.0, Volume: 1.0}}
	oba.UpdateOrderBook("LIQTEST", "okx", shortBids, shortAsks)

	analysis := oba.AnalyzeLiquidity("LIQTEST", 3.0, "bybit", "okx")
	if analysis == nil {
		t.Fatal("AnalyzeLiquidity returned nil")
	}

	if !analysis.IsLiquidityOK {
		t.Error("expected IsLiquidityOK=true")
	}

	// VWAP buy = 100.0 (полностью из первого уровня)
	// VWAP sell = 100.5 (полностью из первого уровня)
	// Adjusted spread = (100.5 - 100.0) / 100.0 * 100 = 0.5%
	expectedSpread := (100.5 - 100.0) / 100.0 * 100
	if abs(analysis.AdjustedSpread-expectedSpread) > 0.0001 {
		t.Errorf("expected AdjustedSpread=%f, got %f", expectedSpread, analysis.AdjustedSpread)
	}

	// Profit = (100.5 - 100.0) * 3.0 = 1.5
	expectedProfit := (100.5 - 100.0) * 3.0
	if abs(analysis.EstimatedProfit-expectedProfit) > 0.0001 {
		t.Errorf("expected EstimatedProfit=%f, got %f", expectedProfit, analysis.EstimatedProfit)
	}
}

func TestOrderBookAnalyzerAnalyzeLiquidityInsufficient(t *testing.T) {
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	// Long - мало ликвидности
	longAsks := []PriceLevel{{Price: 100.0, Volume: 1.0}}
	longBids := []PriceLevel{{Price: 99.0, Volume: 1.0}}
	oba.UpdateOrderBook("LOWLIQTEST", "bybit", longBids, longAsks)

	// Short - достаточно
	shortBids := []PriceLevel{{Price: 100.5, Volume: 10.0}}
	shortAsks := []PriceLevel{{Price: 101.0, Volume: 1.0}}
	oba.UpdateOrderBook("LOWLIQTEST", "okx", shortBids, shortAsks)

	analysis := oba.AnalyzeLiquidity("LOWLIQTEST", 5.0, "bybit", "okx")
	if analysis == nil {
		t.Fatal("AnalyzeLiquidity returned nil")
	}

	if analysis.IsLiquidityOK {
		t.Error("expected IsLiquidityOK=false for insufficient long liquidity")
	}

	if len(analysis.Warnings) == 0 {
		t.Error("expected warnings about insufficient liquidity")
	}
}

func TestOrderBookAnalyzerCheckLiquidityForVolume(t *testing.T) {
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	// Оба стакана с достаточной ликвидностью
	oba.UpdateOrderBook("CHECKLIQ", "bybit",
		[]PriceLevel{{Price: 99.0, Volume: 10.0}},
		[]PriceLevel{{Price: 100.0, Volume: 10.0}})

	oba.UpdateOrderBook("CHECKLIQ", "okx",
		[]PriceLevel{{Price: 100.5, Volume: 10.0}},
		[]PriceLevel{{Price: 101.0, Volume: 10.0}})

	// Проверяем достаточность
	ok, issue := oba.CheckLiquidityForVolume("CHECKLIQ", 5.0, "bybit", "okx")
	if !ok {
		t.Errorf("expected OK=true, issue: %s", issue)
	}

	// Проверяем недостаточность
	ok, issue = oba.CheckLiquidityForVolume("CHECKLIQ", 50.0, "bybit", "okx")
	if ok {
		t.Error("expected OK=false for large volume")
	}
	if issue == "" {
		t.Error("expected issue message")
	}
}

// ============================================================
// SpreadCalculator + OrderBookAnalyzer Integration Tests
// ============================================================

func TestSpreadCalculatorGetRealSpread(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	sc.SetFee("bybit", 0.0004)
	sc.SetFee("okx", 0.0005)

	now := time.Now()

	// Цены в PriceTracker
	pt.Update(PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "BTCUSDT",
		BidPrice:  50000.0,
		AskPrice:  50010.0,
		Timestamp: now,
	})
	pt.Update(PriceUpdate{
		Exchange:  "okx",
		Symbol:    "BTCUSDT",
		BidPrice:  50100.0,
		AskPrice:  50120.0,
		Timestamp: now,
	})

	// Стаканы
	oba.UpdateOrderBook("BTCUSDT", "bybit",
		[]PriceLevel{{Price: 50000.0, Volume: 10.0}},
		[]PriceLevel{{Price: 50010.0, Volume: 10.0}})

	oba.UpdateOrderBook("BTCUSDT", "okx",
		[]PriceLevel{{Price: 50100.0, Volume: 10.0}},
		[]PriceLevel{{Price: 50120.0, Volume: 10.0}})

	opp := sc.GetRealSpread("BTCUSDT", 1.0, oba)
	if opp == nil {
		t.Fatal("GetRealSpread returned nil")
	}

	// VWAP должен быть равен уровням (т.к. объём 1.0 полностью на первом уровне)
	if opp.LongPrice != 50010.0 {
		t.Errorf("expected LongPrice=50010 (VWAP), got %f", opp.LongPrice)
	}
	if opp.ShortPrice != 50100.0 {
		t.Errorf("expected ShortPrice=50100 (VWAP), got %f", opp.ShortPrice)
	}
}

func TestSpreadCalculatorGetRealSpreadNoLiquidity(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	now := time.Now()

	pt.Update(PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "LOWLIQSYM",
		BidPrice:  100.0,
		AskPrice:  100.1,
		Timestamp: now,
	})
	pt.Update(PriceUpdate{
		Exchange:  "okx",
		Symbol:    "LOWLIQSYM",
		BidPrice:  100.5,
		AskPrice:  100.6,
		Timestamp: now,
	})

	// Мало ликвидности в стаканах
	oba.UpdateOrderBook("LOWLIQSYM", "bybit",
		[]PriceLevel{{Price: 100.0, Volume: 0.1}},
		[]PriceLevel{{Price: 100.1, Volume: 0.1}})

	oba.UpdateOrderBook("LOWLIQSYM", "okx",
		[]PriceLevel{{Price: 100.5, Volume: 0.1}},
		[]PriceLevel{{Price: 100.6, Volume: 0.1}})

	// Запрашиваем большой объём
	opp := sc.GetRealSpread("LOWLIQSYM", 10.0, oba)
	if opp != nil {
		t.Error("expected nil for insufficient liquidity")
	}
}

func TestSpreadCalculatorGetSpreadWithLiquidity(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	sc.SetFee("gate", 0.0005)
	sc.SetFee("htx", 0.0006)

	now := time.Now()

	pt.Update(PriceUpdate{
		Exchange:  "gate",
		Symbol:    "ARBUSDT",
		BidPrice:  1.00,
		AskPrice:  1.01, // лучший Ask
		Timestamp: now,
	})
	pt.Update(PriceUpdate{
		Exchange:  "htx",
		Symbol:    "ARBUSDT",
		BidPrice:  1.05, // лучший Bid
		AskPrice:  1.06,
		Timestamp: now,
	})

	// Стаканы с slippage
	oba.UpdateOrderBook("ARBUSDT", "gate",
		[]PriceLevel{{Price: 1.00, Volume: 100.0}},
		[]PriceLevel{
			{Price: 1.01, Volume: 50.0},
			{Price: 1.02, Volume: 50.0},
		})

	oba.UpdateOrderBook("ARBUSDT", "htx",
		[]PriceLevel{
			{Price: 1.05, Volume: 50.0},
			{Price: 1.04, Volume: 50.0},
		},
		[]PriceLevel{{Price: 1.06, Volume: 100.0}})

	result := sc.GetSpreadWithLiquidity("ARBUSDT", 75.0, oba)
	if result == nil {
		t.Fatal("GetSpreadWithLiquidity returned nil")
	}

	if !result.IsLiquidityOK {
		t.Errorf("expected IsLiquidityOK=true, issue: %s", result.LiquidityIssue)
	}

	// Проверяем slippage присутствует
	if result.LongSlippage <= 0 {
		t.Error("expected positive LongSlippage for multi-level fill")
	}
	if result.ShortSlippage <= 0 {
		t.Error("expected positive ShortSlippage for multi-level fill")
	}
	if result.TotalSlippage <= 0 {
		t.Error("expected positive TotalSlippage")
	}
}

func TestSpreadCalculatorGetSpreadWithLiquidityNoAnalyzer(t *testing.T) {
	pt := NewPriceTracker(4)
	sc := NewSpreadCalculator(pt)

	now := time.Now()

	pt.Update(PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "NOOBTEST",
		BidPrice:  10.0,
		AskPrice:  10.1,
		Timestamp: now,
	})
	pt.Update(PriceUpdate{
		Exchange:  "okx",
		Symbol:    "NOOBTEST",
		BidPrice:  10.5,
		AskPrice:  10.6,
		Timestamp: now,
	})

	// Без analyzer - должен вернуть базовые данные
	result := sc.GetSpreadWithLiquidity("NOOBTEST", 1.0, nil)
	if result == nil {
		t.Fatal("GetSpreadWithLiquidity returned nil without analyzer")
	}

	if !result.IsLiquidityOK {
		t.Error("expected IsLiquidityOK=true when no analyzer")
	}
}

// ============================================================
// Benchmark Tests
// ============================================================

func BenchmarkPriceTrackerUpdate(b *testing.B) {
	pt := NewPriceTracker(16)
	update := PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "BTCUSDT",
		BidPrice:  50000.0,
		AskPrice:  50001.0,
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		update.BidPrice = 50000.0 + float64(i%100)
		pt.Update(update)
	}
}

func BenchmarkPriceTrackerGetBestPrices(b *testing.B) {
	pt := NewPriceTracker(16)

	// Инициализируем данные
	for _, exch := range []string{"bybit", "okx", "bitget", "gate", "htx", "bingx"} {
		pt.Update(PriceUpdate{
			Exchange:  exch,
			Symbol:    "BTCUSDT",
			BidPrice:  50000.0,
			AskPrice:  50001.0,
			Timestamp: time.Now(),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = pt.GetBestPrices("BTCUSDT")
	}
}

func BenchmarkSpreadCalculatorGetBestOpportunity(b *testing.B) {
	pt := NewPriceTracker(16)
	sc := NewSpreadCalculator(pt)

	sc.SetFee("bybit", 0.0004)
	sc.SetFee("okx", 0.0005)

	pt.Update(PriceUpdate{
		Exchange:  "bybit",
		Symbol:    "BTCUSDT",
		BidPrice:  50000.0,
		AskPrice:  50010.0,
		Timestamp: time.Now(),
	})
	pt.Update(PriceUpdate{
		Exchange:  "okx",
		Symbol:    "BTCUSDT",
		BidPrice:  50100.0,
		AskPrice:  50120.0,
		Timestamp: time.Now(),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = sc.GetBestOpportunity("BTCUSDT")
	}
}

func BenchmarkOrderBookAnalyzerSimulateBuy(b *testing.B) {
	oba := NewOrderBookAnalyzer(5, 5*time.Second)

	asks := []PriceLevel{
		{Price: 100.0, Volume: 10.0},
		{Price: 100.1, Volume: 10.0},
		{Price: 100.2, Volume: 10.0},
		{Price: 100.3, Volume: 10.0},
		{Price: 100.4, Volume: 10.0},
	}
	bids := []PriceLevel{{Price: 99.0, Volume: 10.0}}

	oba.UpdateOrderBook("BENCHSYM", "bybit", bids, asks)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = oba.SimulateBuy("BENCHSYM", "bybit", 15.0)
	}
}

// ============================================================
// Helper Functions
// ============================================================

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
