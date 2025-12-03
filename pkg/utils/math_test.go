package utils

import (
	"math"
	"testing"
)

// ============================================================
// Тесты RoundToLotSize
// ============================================================

func TestRoundToLotSize(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		lotSize  float64
		expected float64
	}{
		// Базовые кейсы
		{"exact match", 0.123, 0.001, 0.123},
		{"round down", 0.123456, 0.001, 0.123},
		{"round down 2", 1.999, 0.01, 1.99},
		{"whole numbers", 100.5, 1.0, 100.0},

		// Граничные случаи
		{"zero value", 0, 0.001, 0},
		{"zero lotSize", 0.123, 0, 0.123},
		{"negative lotSize", 0.123, -0.001, 0.123},
		{"very small lotSize", 1.23456789, 0.00000001, 1.23456789},

		// BTC примеры (по ТЗ)
		{"BTC lot 0.001", 0.5, 0.001, 0.5},
		{"BTC lot 0.001 round", 0.1234, 0.001, 0.123},
		{"BTC split 4 parts", 0.25, 0.001, 0.25},

		// Большие числа
		{"large number", 12345.6789, 0.01, 12345.67},
		{"very large", 1000000.999, 1.0, 1000000.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundToLotSize(tt.value, tt.lotSize)
			if !floatEquals(result, tt.expected) {
				t.Errorf("RoundToLotSize(%v, %v) = %v, want %v",
					tt.value, tt.lotSize, result, tt.expected)
			}
		})
	}
}

func TestRoundToLotSizeUp(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		lotSize  float64
		expected float64
	}{
		{"exact match", 0.123, 0.001, 0.123},
		{"round up", 0.1231, 0.001, 0.124},
		{"round up 2", 1.991, 0.01, 2.0},
		{"zero lotSize", 0.123, 0, 0.123},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundToLotSizeUp(tt.value, tt.lotSize)
			if !floatEquals(result, tt.expected) {
				t.Errorf("RoundToLotSizeUp(%v, %v) = %v, want %v",
					tt.value, tt.lotSize, result, tt.expected)
			}
		})
	}
}

func TestRoundToLotSizeNearest(t *testing.T) {
	tests := []struct {
		name     string
		value    float64
		lotSize  float64
		expected float64
	}{
		{"exact match", 0.123, 0.001, 0.123},
		{"round down", 0.1234, 0.001, 0.123},
		{"round up", 0.1236, 0.001, 0.124},
		{"midpoint rounds up", 0.1235, 0.001, 0.124}, // Go округляет 0.5 вверх
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := RoundToLotSizeNearest(tt.value, tt.lotSize)
			if !floatEquals(result, tt.expected) {
				t.Errorf("RoundToLotSizeNearest(%v, %v) = %v, want %v",
					tt.value, tt.lotSize, result, tt.expected)
			}
		})
	}
}

// ============================================================
// Тесты CalculateSpread
// ============================================================

func TestCalculateSpread(t *testing.T) {
	tests := []struct {
		name      string
		priceHigh float64
		priceLow  float64
		expected  float64
	}{
		// Базовые кейсы по ТЗ
		{"1% spread", 101.0, 100.0, 1.0},
		{"0.2% spread", 25050.0, 25000.0, 0.2},
		{"0.5% spread", 100.5, 100.0, 0.5},

		// Граничные случаи
		{"zero spread", 100.0, 100.0, 0.0},
		{"zero priceLow", 100.0, 0.0, 0.0},
		{"negative priceLow", 100.0, -50.0, 0.0},

		// Большие спреды
		{"10% spread", 110.0, 100.0, 10.0},
		{"50% spread", 150.0, 100.0, 50.0},

		// Маленькие спреды (важно для арбитража)
		{"0.01% spread", 100.01, 100.0, 0.01},
		{"0.05% spread", 100.05, 100.0, 0.05},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSpread(tt.priceHigh, tt.priceLow)
			if !floatEquals(result, tt.expected) {
				t.Errorf("CalculateSpread(%v, %v) = %v, want %v",
					tt.priceHigh, tt.priceLow, result, tt.expected)
			}
		})
	}
}

func TestCalculateSpreadFromPrices(t *testing.T) {
	tests := []struct {
		name     string
		priceA   float64
		priceB   float64
		expected float64
	}{
		{"A higher", 101.0, 100.0, 1.0},
		{"B higher", 100.0, 101.0, 1.0},
		{"equal", 100.0, 100.0, 0.0},
		{"zero A", 0.0, 100.0, 0.0},
		{"zero B", 100.0, 0.0, 0.0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateSpreadFromPrices(tt.priceA, tt.priceB)
			if !floatEquals(result, tt.expected) {
				t.Errorf("CalculateSpreadFromPrices(%v, %v) = %v, want %v",
					tt.priceA, tt.priceB, result, tt.expected)
			}
		})
	}
}

// ============================================================
// Тесты CalculateNetSpread
// ============================================================

func TestCalculateNetSpread(t *testing.T) {
	tests := []struct {
		name      string
		spreadPct float64
		feeA      float64
		feeB      float64
		expected  float64
	}{
		// Примеры из документации
		// fee 0.04% + 0.05% = 0.09%, total = 2*0.09 = 0.18%
		{"ТЗ example 1", 1.0, 0.0004, 0.0005, 0.82},

		// fee 0.05% + 0.05% = 0.10%, total = 2*0.10 = 0.20%
		{"ТЗ example 2", 0.5, 0.0005, 0.0005, 0.3},

		// Граничные случаи
		{"zero fees", 1.0, 0, 0, 1.0},
		{"zero spread", 0, 0.0005, 0.0005, -0.2},
		{"high fees eat all profit", 0.1, 0.0005, 0.0005, -0.1},

		// Реальные биржевые комиссии
		{"Bybit 0.06% both", 1.0, 0.0006, 0.0006, 0.76},
		{"Bitget 0.04% both", 1.0, 0.0004, 0.0004, 0.84},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateNetSpread(tt.spreadPct, tt.feeA, tt.feeB)
			if !floatEquals(result, tt.expected) {
				t.Errorf("CalculateNetSpread(%v, %v, %v) = %v, want %v",
					tt.spreadPct, tt.feeA, tt.feeB, result, tt.expected)
			}
		})
	}
}

func TestCalculateNetSpreadDirect(t *testing.T) {
	// Комбинированный тест
	priceHigh := 101.0
	priceLow := 100.0
	feeA := 0.0004
	feeB := 0.0005

	// Спред = 1%, комиссии = 0.18%, чистый = 0.82%
	expected := 0.82

	result := CalculateNetSpreadDirect(priceHigh, priceLow, feeA, feeB)
	if !floatEquals(result, expected) {
		t.Errorf("CalculateNetSpreadDirect(%v, %v, %v, %v) = %v, want %v",
			priceHigh, priceLow, feeA, feeB, result, expected)
	}
}

// ============================================================
// Тесты CalculateWeightedAverage (VWAP)
// ============================================================

func TestCalculateWeightedAverage(t *testing.T) {
	tests := []struct {
		name     string
		values   []float64
		weights  []float64
		expected float64
	}{
		// Пример из документации
		{
			"doc example",
			[]float64{100.0, 101.0, 102.0},
			[]float64{10.0, 20.0, 10.0},
			101.0, // (100*10 + 101*20 + 102*10) / 40 = 4040/40 = 101
		},

		// Равные веса = простое среднее
		{
			"equal weights",
			[]float64{100.0, 102.0},
			[]float64{1.0, 1.0},
			101.0,
		},

		// Один элемент
		{
			"single element",
			[]float64{100.0},
			[]float64{10.0},
			100.0,
		},

		// Граничные случаи
		{"empty values", []float64{}, []float64{}, 0},
		{"empty weights", []float64{100}, []float64{}, 0},
		{"length mismatch", []float64{100, 101}, []float64{1}, 0},
		{"zero weights", []float64{100, 101}, []float64{0, 0}, 0},

		// Отрицательные веса игнорируются
		{
			"negative weight ignored",
			[]float64{100.0, 101.0, 102.0},
			[]float64{10.0, -5.0, 10.0},
			101.0, // (100*10 + 102*10) / 20 = 2020/20 = 101
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculateWeightedAverage(tt.values, tt.weights)
			if !floatEquals(result, tt.expected) {
				t.Errorf("CalculateWeightedAverage(%v, %v) = %v, want %v",
					tt.values, tt.weights, result, tt.expected)
			}
		})
	}
}

// ============================================================
// Тесты SimulateMarketBuy / SimulateMarketSell
// ============================================================

func TestSimulateMarketBuy(t *testing.T) {
	asks := []OrderBookLevel{
		{Price: 100.0, Volume: 10.0},
		{Price: 101.0, Volume: 20.0},
		{Price: 102.0, Volume: 30.0},
	}

	tests := []struct {
		name           string
		asks           []OrderBookLevel
		targetVolume   float64
		expectedPrice  float64
		expectedFilled float64
		expectedSlip   float64
	}{
		// Весь объём на первом уровне
		{
			"single level",
			asks,
			5.0,
			100.0,
			5.0,
			0.0,
		},

		// Два уровня
		{
			"two levels",
			asks,
			20.0,                      // 10 @ 100 + 10 @ 101
			100.5,                     // (10*100 + 10*101) / 20 = 2010/20 = 100.5
			20.0,                      // filled
			0.5,                       // (100.5-100)/100 * 100 = 0.5%
		},

		// Больше чем есть в стакане
		// 10*100 + 20*101 + 30*102 = 1000 + 2020 + 3060 = 6080
		// avgPrice = 6080/60 = 101.333...
		// slippage = (101.333-100)/100 * 100 = 1.333%
		{
			"exceed liquidity",
			asks,
			100.0,
			101.333333, // (10*100 + 20*101 + 30*102) / 60 = 6080/60
			60.0,       // только 60 доступно
			1.333333,   // (101.333-100)/100 * 100
		},

		// Пустой стакан
		{
			"empty orderbook",
			[]OrderBookLevel{},
			10.0,
			0, 0, 0,
		},

		// Нулевой объём
		{
			"zero volume",
			asks,
			0,
			0, 0, 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			price, filled, slip := SimulateMarketBuy(tt.asks, tt.targetVolume)

			if !floatEquals(price, tt.expectedPrice) {
				t.Errorf("price = %v, want %v", price, tt.expectedPrice)
			}
			if !floatEquals(filled, tt.expectedFilled) {
				t.Errorf("filled = %v, want %v", filled, tt.expectedFilled)
			}
			if !floatEquals(slip, tt.expectedSlip) {
				t.Errorf("slippage = %v, want %v", slip, tt.expectedSlip)
			}
		})
	}
}

func TestSimulateMarketSell(t *testing.T) {
	bids := []OrderBookLevel{
		{Price: 100.0, Volume: 10.0},
		{Price: 99.0, Volume: 20.0},
		{Price: 98.0, Volume: 30.0},
	}

	price, filled, slip := SimulateMarketSell(bids, 20.0) // 10 @ 100 + 10 @ 99

	expectedPrice := 99.5  // (10*100 + 10*99) / 20 = 1990/20 = 99.5
	expectedSlip := -0.5   // (99.5-100)/100 * 100 = -0.5%

	if !floatEquals(price, expectedPrice) {
		t.Errorf("price = %v, want %v", price, expectedPrice)
	}
	if !floatEquals(filled, 20.0) {
		t.Errorf("filled = %v, want 20", filled)
	}
	if !floatEquals(slip, expectedSlip) {
		t.Errorf("slippage = %v, want %v", slip, expectedSlip)
	}
}

// ============================================================
// Тесты CalculatePNL
// ============================================================

func TestCalculatePNL(t *testing.T) {
	tests := []struct {
		name         string
		side         string
		entryPrice   float64
		currentPrice float64
		quantity     float64
		expected     float64
	}{
		// Long PNL
		{"long profit", "long", 100.0, 110.0, 1.0, 10.0},
		{"long loss", "long", 100.0, 90.0, 1.0, -10.0},
		{"long breakeven", "long", 100.0, 100.0, 1.0, 0.0},

		// Short PNL
		{"short profit", "short", 100.0, 90.0, 1.0, 10.0},
		{"short loss", "short", 100.0, 110.0, 1.0, -10.0},
		{"short breakeven", "short", 100.0, 100.0, 1.0, 0.0},

		// С объёмом
		{"long with qty", "long", 100.0, 110.0, 0.5, 5.0},
		{"short with qty", "short", 100.0, 90.0, 2.0, 20.0},

		// Граничные случаи
		{"zero quantity", "long", 100.0, 110.0, 0, 0},
		{"invalid side", "buy", 100.0, 110.0, 1.0, 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := CalculatePNL(tt.side, tt.entryPrice, tt.currentPrice, tt.quantity)
			if !floatEquals(result, tt.expected) {
				t.Errorf("CalculatePNL(%s, %v, %v, %v) = %v, want %v",
					tt.side, tt.entryPrice, tt.currentPrice, tt.quantity,
					result, tt.expected)
			}
		})
	}
}

func TestCalculateTotalPNL(t *testing.T) {
	// Арбитражный сценарий: лонг на дешёвой бирже, шорт на дорогой
	// Вход: Long @ 100, Short @ 101
	// Выход (цены сошлись): Long @ 100.5, Short @ 100.5
	// Long PNL = (100.5 - 100) * 1 = 0.5
	// Short PNL = (101 - 100.5) * 1 = 0.5
	// Total = 1.0

	result := CalculateTotalPNL(100.0, 100.5, 101.0, 100.5, 1.0)
	expected := 1.0

	if !floatEquals(result, expected) {
		t.Errorf("CalculateTotalPNL = %v, want %v", result, expected)
	}

	// Убыточный сценарий: спред расширился
	// Long @ 100 -> 99, Short @ 101 -> 102
	// Long PNL = -1, Short PNL = -1, Total = -2
	result2 := CalculateTotalPNL(100.0, 99.0, 101.0, 102.0, 1.0)
	expected2 := -2.0

	if !floatEquals(result2, expected2) {
		t.Errorf("CalculateTotalPNL (loss) = %v, want %v", result2, expected2)
	}
}

// ============================================================
// Тесты SplitVolume
// ============================================================

func TestSplitVolume(t *testing.T) {
	tests := []struct {
		name        string
		totalVolume float64
		nParts      int
		lotSize     float64
		expected    []float64
	}{
		// По ТЗ: 1 BTC на 4 части = 0.25 BTC каждая
		{"BTC 4 parts", 1.0, 4, 0.001, []float64{0.25, 0.25, 0.25, 0.25}},

		// Один ордер
		{"single order", 0.5, 1, 0.001, []float64{0.5}},

		// С округлением
		{"with rounding", 1.0, 3, 0.01, []float64{0.33, 0.33, 0.33}},

		// Граничные случаи
		{"zero parts", 1.0, 0, 0.001, nil},
		{"zero volume", 0, 4, 0.001, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := SplitVolume(tt.totalVolume, tt.nParts, tt.lotSize)

			if tt.expected == nil {
				if result != nil {
					t.Errorf("expected nil, got %v", result)
				}
				return
			}

			if len(result) != len(tt.expected) {
				t.Errorf("len = %d, want %d", len(result), len(tt.expected))
				return
			}

			for i := range result {
				if !floatEquals(result[i], tt.expected[i]) {
					t.Errorf("part[%d] = %v, want %v", i, result[i], tt.expected[i])
				}
			}
		})
	}
}

// ============================================================
// Тесты проверок условий
// ============================================================

func TestIsSpreadSufficient(t *testing.T) {
	// Спред >= порога = true
	if !IsSpreadSufficient(1.0, 0.5) {
		t.Error("1.0 >= 0.5 should be true")
	}

	// Спред < порога = false
	if IsSpreadSufficient(0.3, 0.5) {
		t.Error("0.3 < 0.5 should be false")
	}

	// Равно = true
	if !IsSpreadSufficient(0.5, 0.5) {
		t.Error("0.5 >= 0.5 should be true")
	}
}

func TestShouldExit(t *testing.T) {
	// Спред <= порога выхода = true
	if !ShouldExit(0.1, 0.2) {
		t.Error("0.1 <= 0.2 should trigger exit")
	}

	// Спред > порога = false
	if ShouldExit(0.5, 0.2) {
		t.Error("0.5 > 0.2 should not trigger exit")
	}
}

func TestIsStopLossHit(t *testing.T) {
	// PNL = -100, SL = 100 -> true
	if !IsStopLossHit(-100, 100) {
		t.Error("-100 <= -100 should hit SL")
	}

	// PNL = -50, SL = 100 -> false
	if IsStopLossHit(-50, 100) {
		t.Error("-50 > -100 should not hit SL")
	}

	// SL не задан (0) -> false
	if IsStopLossHit(-100, 0) {
		t.Error("SL=0 means disabled")
	}

	// Положительный PNL -> false
	if IsStopLossHit(50, 100) {
		t.Error("positive PNL should never hit SL")
	}
}

// ============================================================
// Тесты утилит
// ============================================================

func TestClamp(t *testing.T) {
	tests := []struct {
		value, min, max, expected float64
	}{
		{5, 0, 10, 5},   // в диапазоне
		{-5, 0, 10, 0},  // ниже min
		{15, 0, 10, 10}, // выше max
		{0, 0, 10, 0},   // на границе min
		{10, 0, 10, 10}, // на границе max
	}

	for _, tt := range tests {
		result := Clamp(tt.value, tt.min, tt.max)
		if result != tt.expected {
			t.Errorf("Clamp(%v, %v, %v) = %v, want %v",
				tt.value, tt.min, tt.max, result, tt.expected)
		}
	}
}

// ============================================================
// Бенчмарки
// ============================================================

func BenchmarkRoundToLotSize(b *testing.B) {
	for i := 0; i < b.N; i++ {
		RoundToLotSize(0.123456789, 0.001)
	}
}

func BenchmarkCalculateSpread(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateSpread(25050, 25000)
	}
}

func BenchmarkCalculateNetSpread(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculateNetSpread(1.0, 0.0004, 0.0005)
	}
}

func BenchmarkCalculateWeightedAverage(b *testing.B) {
	values := []float64{100.0, 101.0, 102.0, 103.0, 104.0}
	weights := []float64{10.0, 20.0, 30.0, 20.0, 10.0}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateWeightedAverage(values, weights)
	}
}

func BenchmarkSimulateMarketBuy(b *testing.B) {
	asks := []OrderBookLevel{
		{Price: 100.0, Volume: 10.0},
		{Price: 101.0, Volume: 20.0},
		{Price: 102.0, Volume: 30.0},
		{Price: 103.0, Volume: 40.0},
		{Price: 104.0, Volume: 50.0},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		SimulateMarketBuy(asks, 50.0)
	}
}

func BenchmarkCalculatePNL(b *testing.B) {
	for i := 0; i < b.N; i++ {
		CalculatePNL("long", 100.0, 110.0, 0.5)
	}
}

// ============================================================
// Вспомогательные функции
// ============================================================

const floatEpsilon = 1e-6

func floatEquals(a, b float64) bool {
	return math.Abs(a-b) < floatEpsilon
}
