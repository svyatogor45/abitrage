package repository

// StatsRepository - работа со статистикой
//
// Назначение: Агрегация и расчет статистики из таблицы trades
//
// Функции:
// - GetStats: расчет всех агрегатов из trades таблицы
// - RecordTrade: записать завершенную сделку
// - RecordStopLoss: записать событие SL
// - RecordLiquidation: записать событие ликвидации
// - GetTopPairsByTrades: топ-5 пар по количеству сделок
// - GetTopPairsByProfit: топ-5 пар по прибыли
// - GetTopPairsByLoss: топ-5 пар по убыткам
// - ResetCounters: сброс счетчиков (обнуление дисплейных данных)
//
// Данные группируются по периодам: день/неделя/месяц
//
// TODO: реализовать агрегацию статистики из trades
