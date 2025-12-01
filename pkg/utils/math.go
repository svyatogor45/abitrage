package utils

// math.go - математические утилиты
//
// Назначение:
// Вспомогательные математические функции для торговли.
//
// Функции:
// - RoundToLotSize: округление до lot size биржи
//   * Пример: 0.123456 BTC с lot size 0.001 → 0.123 BTC
// - CalculateSpread: расчет спреда между ценами
//   * Formula: (priceHigh - priceLow) / priceLow * 100
// - CalculateNetSpread: чистый спред с учетом комиссий
//   * spread - 2*(feeA + feeB)
// - CalculateWeightedAverage: средневзвешенная цена
//   * Используется для расчета цены по стакану ордеров
//
// TODO: реализовать математические функции
