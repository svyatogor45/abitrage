package bot

import (
	"context"
	"fmt"
	"testing"
	"time"

	"arbitrage/internal/config"
	"arbitrage/internal/exchange"
	"arbitrage/internal/models"
)

// ============================================================
// Бенчмарки Hot Path: от получения цены до выставления ордера
// ============================================================
//
// Критические метрики (из Разработка.md):
// - Tick → Order: < 5ms (критическое > 50ms)
// - Обработка одного PriceUpdate: < 1ms (критическое > 10ms)
// - Расчёт спреда и проверка условий: < 0.5ms (критическое > 5ms)
// - Пересчёт лучших цен (PriceTracker): O(k), k~6 (не O(n²))

// ============ Мок биржи для бенчмарков ============

type mockExchangeBench struct {
	name    string
	latency time.Duration
}

func newMockExchangeBench(name string, latency time.Duration) *mockExchangeBench {
	return &mockExchangeBench{name: name, latency: latency}
}

func (m *mockExchangeBench) GetName() string { return m.name }

func (m *mockExchangeBench) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*exchange.Order, error) {
	if m.latency > 0 {
		time.Sleep(m.latency)
	}
	return &exchange.Order{
		ID:           "bench-order-1",
		Symbol:       symbol,
		Side:         side,
		Type:         "market",
		Quantity:     qty,
		FilledQty:    qty,
		AvgFillPrice: 50000.0,
		Status:       "filled",
	}, nil
}

func (m *mockExchangeBench) Connect(apiKey, secretKey, passphrase string) error { return nil }
func (m *mockExchangeBench) Close() error                                       { return nil }
func (m *mockExchangeBench) GetBalance(ctx context.Context) (float64, error) {
	return 10000.0, nil
}
func (m *mockExchangeBench) GetTicker(ctx context.Context, symbol string) (*exchange.Ticker, error) {
	return &exchange.Ticker{Symbol: symbol, BidPrice: 50000.0, AskPrice: 50001.0}, nil
}
func (m *mockExchangeBench) GetOrderBook(ctx context.Context, symbol string, depth int) (*exchange.OrderBook, error) {
	return &exchange.OrderBook{Symbol: symbol}, nil
}
func (m *mockExchangeBench) GetOpenPositions(ctx context.Context) ([]*exchange.Position, error) {
	return nil, nil
}
func (m *mockExchangeBench) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	return nil
}
func (m *mockExchangeBench) GetLimits(ctx context.Context, symbol string) (*exchange.Limits, error) {
	return &exchange.Limits{MinOrderQty: 0.001, MaxOrderQty: 100, QtyStep: 0.001}, nil
}
func (m *mockExchangeBench) SubscribeTicker(symbol string, callback func(*exchange.Ticker)) error {
	return nil
}
func (m *mockExchangeBench) SubscribePositions(callback func(*exchange.Position)) error {
	return nil
}
func (m *mockExchangeBench) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	return 0.0004, nil
}

// ============ Вспомогательные функции для тестов ============

func updatePrice(tracker *PriceTracker, symbol, exch string, bid, ask float64) {
	tracker.Update(PriceUpdate{
		Symbol:    symbol,
		Exchange:  exch,
		BidPrice:  bid,
		AskPrice:  ask,
		Timestamp: time.Now(),
	})
}

// ============ Бенчмарки PriceTracker (расширенные) ============

// BenchmarkPriceTrackerUpdatePtr - обновление из PriceUpdate ptr
func BenchmarkPriceTrackerUpdatePtr(b *testing.B) {
	tracker := NewPriceTracker(16)

	// Предзаполняем данные
	updatePrice(tracker, "BTCUSDT", "binance", 50000.0, 50001.0)
	updatePrice(tracker, "BTCUSDT", "okx", 50002.0, 50003.0)

	update := &PriceUpdate{
		Symbol:    "BTCUSDT",
		Exchange:  "binance",
		BidPrice:  50000.5,
		AskPrice:  50001.5,
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		update.BidPrice = 50000.0 + float64(i%100)
		update.AskPrice = 50001.0 + float64(i%100)
		tracker.UpdateFromPtr(update)
	}
}

// BenchmarkPriceTrackerConcurrentHeavy - параллельные обновления (высокая нагрузка)
func BenchmarkPriceTrackerConcurrentHeavy(b *testing.B) {
	tracker := NewPriceTracker(16)
	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT"}
	exchanges := []string{"binance", "okx", "bybit"}

	b.ResetTimer()
	b.ReportAllocs()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sym := symbols[i%len(symbols)]
			exch := exchanges[i%len(exchanges)]
			updatePrice(tracker, sym, exch, 50000.0+float64(i%100), 50001.0+float64(i%100))
			_ = tracker.GetBestPrices(sym)
			i++
		}
	})
}

// ============ Бенчмарки ArbitrageDetector ============

// BenchmarkArbitrageDetectorDetect - обнаружение возможности
func BenchmarkArbitrageDetectorDetect(b *testing.B) {
	tracker := NewPriceTracker(16)
	calc := NewSpreadCalculator(tracker)
	detector := NewArbitrageDetector(tracker, calc, nil)

	calc.SetFee("binance", 0.0004)
	calc.SetFee("okx", 0.0005)
	updatePrice(tracker, "BTCUSDT", "binance", 49990.0, 50000.0)
	updatePrice(tracker, "BTCUSDT", "okx", 50050.0, 50060.0)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		opp := detector.DetectOpportunity("BTCUSDT")
		if opp != nil {
			ReleaseArbitrageOpportunity(opp)
		}
	}
}

// BenchmarkArbitrageDetectorCheckEntry - полная проверка условий входа
func BenchmarkArbitrageDetectorCheckEntry(b *testing.B) {
	tracker := NewPriceTracker(16)
	calc := NewSpreadCalculator(tracker)
	detector := NewArbitrageDetector(tracker, calc, nil)

	calc.SetFee("binance", 0.0004)
	calc.SetFee("okx", 0.0005)
	updatePrice(tracker, "BTCUSDT", "binance", 49990.0, 50000.0)
	updatePrice(tracker, "BTCUSDT", "okx", 50100.0, 50110.0) // Больший спред

	ps := &PairState{
		Config: &models.PairConfig{
			ID:             1,
			Symbol:         "BTCUSDT",
			Status:         "active",
			EntrySpreadPct: 0.05, // 0.05%
			ExitSpreadPct:  0.01,
			VolumeAsset:    0.1,
		},
		Runtime: &models.PairRuntime{
			State: models.StateReady,
		},
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		conditions := detector.CheckEntryConditions(ps, 0, 10, nil)
		if conditions.CanEnter {
			ReleaseArbitrageOpportunity(conditions.Opportunity)
		}
		ReleaseEntryConditions(conditions)
	}
}

// ============ Бенчмарки OrderExecutor ============

// BenchmarkOrderExecutorParallelNoLatency - параллельное исполнение (без сети)
func BenchmarkOrderExecutorParallelNoLatency(b *testing.B) {
	exchanges := map[string]exchange.Exchange{
		"binance": newMockExchangeBench("binance", 0),
		"okx":     newMockExchangeBench("okx", 0),
	}

	executor := NewOrderExecutor(exchanges, defaultBotConfig())

	params := ExecuteParams{
		Symbol:        "BTCUSDT",
		Volume:        0.1,
		LongExchange:  "binance",
		ShortExchange: "okx",
		NOrders:       1,
	}

	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		result := executor.ExecuteParallel(ctx, params)
		_ = result
	}
}

// BenchmarkOrderExecutorLatency - с симуляцией сетевой задержки
func BenchmarkOrderExecutorLatency(b *testing.B) {
	for _, latency := range []time.Duration{0, 1 * time.Millisecond, 10 * time.Millisecond} {
		b.Run(fmt.Sprintf("latency_%v", latency), func(b *testing.B) {
			exchanges := map[string]exchange.Exchange{
				"binance": newMockExchangeBench("binance", latency),
				"okx":     newMockExchangeBench("okx", latency),
			}

			executor := NewOrderExecutor(exchanges, defaultBotConfig())

			params := ExecuteParams{
				Symbol:        "BTCUSDT",
				Volume:        0.1,
				LongExchange:  "binance",
				ShortExchange: "okx",
				NOrders:       1,
			}

			ctx := context.Background()

			b.ResetTimer()

			for i := 0; i < b.N; i++ {
				result := executor.ExecuteParallel(ctx, params)
				_ = result
			}
		})
	}
}

// ============ Бенчмарки полного цикла ============

// BenchmarkFullHotPathComplete - полный горячий путь от цены до решения
// Это основной бенчмарк для измерения Tick → Decision latency
func BenchmarkFullHotPathComplete(b *testing.B) {
	tracker := NewPriceTracker(16)
	calc := NewSpreadCalculator(tracker)
	detector := NewArbitrageDetector(tracker, calc, nil)

	calc.SetFee("binance", 0.0004)
	calc.SetFee("okx", 0.0005)

	// Начальные данные
	updatePrice(tracker, "BTCUSDT", "binance", 49990.0, 50000.0)
	updatePrice(tracker, "BTCUSDT", "okx", 50100.0, 50110.0)

	ps := &PairState{
		Config: &models.PairConfig{
			ID:             1,
			Symbol:         "BTCUSDT",
			Status:         "active",
			EntrySpreadPct: 0.05,
			ExitSpreadPct:  0.01,
			VolumeAsset:    0.1,
		},
		Runtime: &models.PairRuntime{
			State: models.StateReady,
		},
		isReady: 1,
	}

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		// 1. Обновление цены
		updatePrice(tracker, "BTCUSDT", "binance", 49990.0+float64(i%10), 50000.0+float64(i%10))

		// 2. Получение лучших цен
		_ = tracker.GetBestPrices("BTCUSDT")

		// 3. Обнаружение возможности
		opp := detector.DetectOpportunity("BTCUSDT")
		if opp == nil {
			continue
		}

		// 4. Проверка условий входа
		conditions := detector.CheckEntryConditions(ps, 0, 10, nil)

		// Cleanup
		if conditions.CanEnter {
			ReleaseArbitrageOpportunity(conditions.Opportunity)
		}
		ReleaseEntryConditions(conditions)
		ReleaseArbitrageOpportunity(opp)
	}
}

// BenchmarkFullHotPathConcurrentMulti - параллельный горячий путь (реалистичный сценарий)
func BenchmarkFullHotPathConcurrentMulti(b *testing.B) {
	tracker := NewPriceTracker(16)
	calc := NewSpreadCalculator(tracker)
	detector := NewArbitrageDetector(tracker, calc, nil)

	calc.SetFee("binance", 0.0004)
	calc.SetFee("okx", 0.0005)
	calc.SetFee("bybit", 0.0004)

	symbols := []string{"BTCUSDT", "ETHUSDT", "BNBUSDT", "SOLUSDT"}
	exchanges := []string{"binance", "okx", "bybit"}

	// Инициализация
	for _, sym := range symbols {
		for i, exch := range exchanges {
			updatePrice(tracker, sym, exch, 50000.0+float64(i*10), 50001.0+float64(i*10))
		}
	}

	pairs := make([]*PairState, len(symbols))
	for i, sym := range symbols {
		pairs[i] = &PairState{
			Config: &models.PairConfig{
				ID:             i + 1,
				Symbol:         sym,
				Status:         "active",
				EntrySpreadPct: 0.05,
				ExitSpreadPct:  0.01,
				VolumeAsset:    0.1,
			},
			Runtime: &models.PairRuntime{
				State: models.StateReady,
			},
			isReady: 1,
		}
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			sym := symbols[i%len(symbols)]
			exch := exchanges[i%len(exchanges)]
			ps := pairs[i%len(pairs)]

			// Обновление цены
			updatePrice(tracker, sym, exch, 50000.0+float64(i%100), 50001.0+float64(i%100))

			// Получение лучших цен
			_ = tracker.GetBestPrices(sym)

			// Обнаружение возможности
			opp := detector.DetectOpportunity(sym)
			if opp != nil {
				// Проверка условий
				conditions := detector.CheckEntryConditions(ps, 0, 10, nil)
				if conditions.CanEnter {
					ReleaseArbitrageOpportunity(conditions.Opportunity)
				}
				ReleaseEntryConditions(conditions)
				ReleaseArbitrageOpportunity(opp)
			}

			i++
		}
	})
}

// ============ Бенчмарки sync.Pool эффективности ============

// BenchmarkPoolArbitrageOpp - эффективность пула ArbitrageOpportunity
func BenchmarkPoolArbitrageOpp(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			opp := acquireArbitrageOpportunity()
			opp.Symbol = "BTCUSDT"
			opp.NetSpread = 0.1
			ReleaseArbitrageOpportunity(opp)
		}
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			opp := &ArbitrageOpportunity{
				Symbol:    "BTCUSDT",
				NetSpread: 0.1,
			}
			_ = opp
		}
	})
}

// BenchmarkPoolEntryConditions - эффективность пула EntryConditions
func BenchmarkPoolEntryConditions(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ec := acquireEntryConditions()
			ec.CanEnter = true
			ec.Reason = "test"
			ReleaseEntryConditions(ec)
		}
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ec := &EntryConditions{
				CanEnter: true,
				Reason:   "test",
				Warnings: make([]string, 0, 4),
			}
			_ = ec
		}
	})
}

// BenchmarkPoolLegResultChan - эффективность пула каналов
func BenchmarkPoolLegResultChan(b *testing.B) {
	b.Run("WithPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ch := acquireLegResultChan()
			releaseLegResultChan(ch)
		}
	})

	b.Run("WithoutPool", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			ch := make(chan LegResult, 1)
			_ = ch
		}
	})
}

// ============ Бенчмарки отдельных компонентов ============

// BenchmarkSpreadCalculation - только расчёт спреда
func BenchmarkSpreadCalculation(b *testing.B) {
	tracker := NewPriceTracker(16)
	calc := NewSpreadCalculator(tracker)

	calc.SetFee("binance", 0.0004)
	calc.SetFee("okx", 0.0005)
	updatePrice(tracker, "BTCUSDT", "binance", 49990.0, 50000.0)
	updatePrice(tracker, "BTCUSDT", "okx", 50050.0, 50060.0)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		opp := calc.GetBestOpportunity("BTCUSDT")
		if opp != nil {
			ReleaseArbitrageOpportunity(opp)
		}
	}
}

// BenchmarkStateTransition - проверка перехода состояния O(1)
func BenchmarkStateTransition(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = CanTransition(models.StateReady, models.StateEntering)
	}
}

// ============ Вспомогательные функции ============

func defaultBotConfig() config.BotConfig {
	return config.BotConfig{
		OrderTimeout:      5 * time.Second,
		MaxConcurrentArbs: 10,
	}
}
