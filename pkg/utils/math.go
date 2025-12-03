package utils

import (
	"math"
)

// math.go - математические утилиты для арбитражной торговли
//
// Назначение:
// Вспомогательные математические функции для торговых операций.
// Все функции являются чистыми (pure functions) без побочных эффектов.
//
// Функции:
// - RoundToLotSize: округление до lot size биржи
// - CalculateSpread: расчет спреда между ценами
// - CalculateNetSpread: чистый спред с учетом комиссий
// - CalculateWeightedAverage: средневзвешенная цена (VWAP)

// RoundToLotSize округляет значение ВНИЗ до ближайшего кратного lotSize.
//
// Используется для округления объёма ордера до минимального шага биржи.
// Округление вниз гарантирует, что мы не превысим доступные средства.
//
// Параметры:
//   - value: исходное значение (объём в монетах актива)
//   - lotSize: минимальный шаг изменения объёма на бирже
//
// Возвращает:
//   - Округлённое значение, кратное lotSize
//   - Если lotSize <= 0, возвращает исходное значение
//
// Примеры:
//   - RoundToLotSize(0.123456, 0.001) = 0.123
//   - RoundToLotSize(1.999, 0.01) = 1.99
//   - RoundToLotSize(100.5, 1.0) = 100.0
func RoundToLotSize(value, lotSize float64) float64 {
	if lotSize <= 0 {
		return value
	}
	// Используем math.Floor для округления вниз
	// Это безопаснее для торговли - не превысим доступные средства
	return math.Floor(value/lotSize) * lotSize
}

// RoundToLotSizeUp округляет значение ВВЕРХ до ближайшего кратного lotSize.
//
// Используется когда нужно гарантировать минимальный объём (например, для minQty).
//
// Параметры:
//   - value: исходное значение
//   - lotSize: минимальный шаг
//
// Возвращает:
//   - Округлённое вверх значение, кратное lotSize
func RoundToLotSizeUp(value, lotSize float64) float64 {
	if lotSize <= 0 {
		return value
	}
	return math.Ceil(value/lotSize) * lotSize
}

// RoundToLotSizeNearest округляет к ближайшему кратному lotSize.
//
// Стандартное математическое округление.
//
// Параметры:
//   - value: исходное значение
//   - lotSize: минимальный шаг
//
// Возвращает:
//   - Округлённое значение к ближайшему кратному lotSize
func RoundToLotSizeNearest(value, lotSize float64) float64 {
	if lotSize <= 0 {
		return value
	}
	return math.Round(value/lotSize) * lotSize
}

// CalculateSpread расчитывает спред между двумя ценами в процентах.
//
// Формула по ТЗ:
//
//	Спред (%) = ((P_высокая - P_низкая) / P_низкая) × 100
//
// Параметры:
//   - priceHigh: более высокая цена (цена продажи, шорт)
//   - priceLow: более низкая цена (цена покупки, лонг)
//
// Возвращает:
//   - Спред в процентах (например, 1.5 означает 1.5%)
//   - Если priceLow <= 0, возвращает 0
//
// Примеры:
//   - CalculateSpread(101.0, 100.0) = 1.0 (1%)
//   - CalculateSpread(25050, 25000) = 0.2 (0.2%)
func CalculateSpread(priceHigh, priceLow float64) float64 {
	if priceLow <= 0 {
		return 0
	}
	return (priceHigh - priceLow) / priceLow * 100
}

// CalculateSpreadFromPrices расчитывает спред, автоматически определяя high/low.
//
// Удобная обёртка когда неизвестно какая цена выше.
//
// Параметры:
//   - priceA: цена на бирже A
//   - priceB: цена на бирже B
//
// Возвращает:
//   - Абсолютное значение спреда в процентах (всегда >= 0)
func CalculateSpreadFromPrices(priceA, priceB float64) float64 {
	if priceA <= 0 || priceB <= 0 {
		return 0
	}
	if priceA > priceB {
		return CalculateSpread(priceA, priceB)
	}
	return CalculateSpread(priceB, priceA)
}

// CalculateNetSpread расчитывает чистый спред с учётом торговых комиссий.
//
// Формула по ТЗ:
//
//	Чистый спред = Спред (%) - 2 × (fee_A + fee_B)
//
// При арбитраже совершаются 4 тейкер-сделки:
// 1. Открытие лонга на бирже A
// 2. Открытие шорта на бирже B
// 3. Закрытие лонга на бирже A
// 4. Закрытие шорта на бирже B
//
// Поэтому комиссии учитываются с множителем 2.
//
// Параметры:
//   - spreadPct: спред в процентах (результат CalculateSpread)
//   - feeA: комиссия тейкера на бирже A в долях (0.0004 = 0.04%)
//   - feeB: комиссия тейкера на бирже B в долях (0.0005 = 0.05%)
//
// Возвращает:
//   - Чистый спред в процентах после вычета комиссий
//
// Примеры:
//   - CalculateNetSpread(1.0, 0.0004, 0.0005) = 1.0 - 0.18 = 0.82
//   - CalculateNetSpread(0.5, 0.0005, 0.0005) = 0.5 - 0.2 = 0.3
func CalculateNetSpread(spreadPct, feeA, feeB float64) float64 {
	// Комиссии в долях переводим в проценты: fee * 100
	// 4 сделки = 2 × (feeA + feeB)
	totalFeePct := 2 * (feeA + feeB) * 100
	return spreadPct - totalFeePct
}

// CalculateNetSpreadDirect расчитывает чистый спред напрямую из цен.
//
// Комбинирует CalculateSpread и CalculateNetSpread в одну функцию.
//
// Параметры:
//   - priceHigh: более высокая цена
//   - priceLow: более низкая цена
//   - feeA: комиссия тейкера на бирже A в долях
//   - feeB: комиссия тейкера на бирже B в долях
//
// Возвращает:
//   - Чистый спред в процентах
func CalculateNetSpreadDirect(priceHigh, priceLow, feeA, feeB float64) float64 {
	rawSpread := CalculateSpread(priceHigh, priceLow)
	return CalculateNetSpread(rawSpread, feeA, feeB)
}

// CalculateWeightedAverage вычисляет средневзвешенное значение (VWAP).
//
// Используется для расчёта средневзвешенной цены по стакану ордеров.
// VWAP (Volume-Weighted Average Price) показывает реальную цену исполнения
// рыночного ордера заданного объёма.
//
// Формула:
//
//	VWAP = Σ(price_i × volume_i) / Σ(volume_i)
//
// Параметры:
//   - values: слайс цен (price levels)
//   - weights: слайс объёмов (volumes на каждом уровне)
//
// Возвращает:
//   - Средневзвешенное значение
//   - 0 если входные данные некорректны
//
// Примеры:
//
//	values  = [100.0, 101.0, 102.0]
//	weights = [10.0, 20.0, 10.0]
//	VWAP = (100*10 + 101*20 + 102*10) / (10+20+10) = 4040/40 = 101.0
func CalculateWeightedAverage(values, weights []float64) float64 {
	if len(values) == 0 || len(weights) == 0 {
		return 0
	}
	if len(values) != len(weights) {
		return 0
	}

	var sumWeighted, sumWeights float64
	for i := range values {
		if weights[i] < 0 {
			continue // Пропускаем отрицательные веса
		}
		sumWeighted += values[i] * weights[i]
		sumWeights += weights[i]
	}

	if sumWeights == 0 {
		return 0
	}
	return sumWeighted / sumWeights
}

// OrderBookLevel представляет один уровень стакана ордеров
type OrderBookLevel struct {
	Price  float64
	Volume float64
}

// SimulateMarketBuy моделирует рыночную покупку заданного объёма.
//
// Проходит по уровням Ask (от лучшего к худшему) и рассчитывает
// средневзвешенную цену покупки с учётом глубины стакана.
//
// Параметры:
//   - asks: уровни Ask (заявки на продажу), отсортированы по возрастанию цены
//   - targetVolume: требуемый объём покупки
//
// Возвращает:
//   - avgPrice: средневзвешенная цена покупки
//   - filledVolume: реально доступный объём (может быть < targetVolume)
//   - slippage: проскальзывание в процентах относительно лучшей цены
func SimulateMarketBuy(asks []OrderBookLevel, targetVolume float64) (avgPrice, filledVolume, slippage float64) {
	if len(asks) == 0 || targetVolume <= 0 {
		return 0, 0, 0
	}

	bestPrice := asks[0].Price
	if bestPrice <= 0 {
		return 0, 0, 0
	}

	var sumCost float64 // Σ(price × volume)
	remaining := targetVolume

	for _, level := range asks {
		if level.Price <= 0 || level.Volume <= 0 {
			continue
		}

		take := math.Min(remaining, level.Volume)
		sumCost += level.Price * take
		filledVolume += take
		remaining -= take

		if remaining <= 0 {
			break
		}
	}

	if filledVolume == 0 {
		return 0, 0, 0
	}

	avgPrice = sumCost / filledVolume
	slippage = (avgPrice - bestPrice) / bestPrice * 100

	return avgPrice, filledVolume, slippage
}

// SimulateMarketSell моделирует рыночную продажу заданного объёма.
//
// Проходит по уровням Bid (от лучшего к худшему) и рассчитывает
// средневзвешенную цену продажи с учётом глубины стакана.
//
// Параметры:
//   - bids: уровни Bid (заявки на покупку), отсортированы по убыванию цены
//   - targetVolume: требуемый объём продажи
//
// Возвращает:
//   - avgPrice: средневзвешенная цена продажи
//   - filledVolume: реально доступный объём
//   - slippage: проскальзывание в процентах (отрицательное, т.к. цена падает)
func SimulateMarketSell(bids []OrderBookLevel, targetVolume float64) (avgPrice, filledVolume, slippage float64) {
	if len(bids) == 0 || targetVolume <= 0 {
		return 0, 0, 0
	}

	bestPrice := bids[0].Price
	if bestPrice <= 0 {
		return 0, 0, 0
	}

	var sumCost float64
	remaining := targetVolume

	for _, level := range bids {
		if level.Price <= 0 || level.Volume <= 0 {
			continue
		}

		take := math.Min(remaining, level.Volume)
		sumCost += level.Price * take
		filledVolume += take
		remaining -= take

		if remaining <= 0 {
			break
		}
	}

	if filledVolume == 0 {
		return 0, 0, 0
	}

	avgPrice = sumCost / filledVolume
	// Для продажи slippage отрицательный (получаем меньше чем лучшая цена)
	slippage = (avgPrice - bestPrice) / bestPrice * 100

	return avgPrice, filledVolume, slippage
}

// CalculatePNL расчитывает прибыль/убыток по позиции.
//
// Формулы по ТЗ:
//   - Long PNL = (P_close - P_open) × qty
//   - Short PNL = (P_open - P_close) × qty
//
// Параметры:
//   - side: "long" или "short"
//   - entryPrice: цена входа
//   - currentPrice: текущая/выходная цена
//   - quantity: объём позиции
//
// Возвращает:
//   - PNL в валюте котировки (обычно USDT)
func CalculatePNL(side string, entryPrice, currentPrice, quantity float64) float64 {
	if quantity <= 0 {
		return 0
	}

	switch side {
	case "long":
		// Лонг: прибыль если цена выросла
		return (currentPrice - entryPrice) * quantity
	case "short":
		// Шорт: прибыль если цена упала
		return (entryPrice - currentPrice) * quantity
	default:
		return 0
	}
}

// CalculateTotalPNL расчитывает суммарный PNL арбитражной позиции.
//
// Параметры:
//   - longEntry: цена входа в лонг
//   - longCurrent: текущая цена лонга
//   - shortEntry: цена входа в шорт
//   - shortCurrent: текущая цена шорта
//   - quantity: объём (одинаковый для обеих ног)
//
// Возвращает:
//   - Суммарный PNL в валюте котировки
func CalculateTotalPNL(longEntry, longCurrent, shortEntry, shortCurrent, quantity float64) float64 {
	longPNL := CalculatePNL("long", longEntry, longCurrent, quantity)
	shortPNL := CalculatePNL("short", shortEntry, shortCurrent, quantity)
	return longPNL + shortPNL
}

// SplitVolume разбивает общий объём на N равных частей.
//
// Используется для частичного входа в позицию (N ордеров).
// Каждая часть округляется до lotSize.
//
// Параметры:
//   - totalVolume: общий объём
//   - nParts: количество частей
//   - lotSize: минимальный шаг объёма
//
// Возвращает:
//   - Слайс объёмов для каждой части
//   - Сумма частей может быть меньше totalVolume из-за округления
func SplitVolume(totalVolume float64, nParts int, lotSize float64) []float64 {
	if nParts <= 0 || totalVolume <= 0 {
		return nil
	}

	if nParts == 1 {
		return []float64{RoundToLotSize(totalVolume, lotSize)}
	}

	partSize := totalVolume / float64(nParts)
	roundedPart := RoundToLotSize(partSize, lotSize)

	if roundedPart <= 0 {
		// Если часть слишком маленькая, возвращаем один ордер
		return []float64{RoundToLotSize(totalVolume, lotSize)}
	}

	parts := make([]float64, nParts)
	for i := range parts {
		parts[i] = roundedPart
	}

	return parts
}

// IsSpreadSufficient проверяет достаточность спреда для входа.
//
// Параметры:
//   - netSpread: чистый спред в процентах
//   - entryThreshold: порог входа в процентах
//
// Возвращает:
//   - true если спред >= порога
func IsSpreadSufficient(netSpread, entryThreshold float64) bool {
	return netSpread >= entryThreshold
}

// ShouldExit проверяет условие выхода из позиции.
//
// Параметры:
//   - currentSpread: текущий спред в процентах
//   - exitThreshold: порог выхода в процентах
//
// Возвращает:
//   - true если спред <= порога (цены сошлись)
func ShouldExit(currentSpread, exitThreshold float64) bool {
	return currentSpread <= exitThreshold
}

// IsStopLossHit проверяет достижение Stop Loss.
//
// Параметры:
//   - totalPNL: текущий суммарный PNL
//   - stopLoss: величина Stop Loss (положительное число в USDT)
//
// Возвращает:
//   - true если PNL <= -stopLoss
func IsStopLossHit(totalPNL, stopLoss float64) bool {
	if stopLoss <= 0 {
		return false // SL не задан
	}
	return totalPNL <= -stopLoss
}

// Abs возвращает абсолютное значение числа.
func Abs(x float64) float64 {
	return math.Abs(x)
}

// Min возвращает минимум из двух чисел.
func Min(a, b float64) float64 {
	return math.Min(a, b)
}

// Max возвращает максимум из двух чисел.
func Max(a, b float64) float64 {
	return math.Max(a, b)
}

// Clamp ограничивает значение диапазоном [min, max].
func Clamp(value, min, max float64) float64 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}
