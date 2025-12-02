package service

import (
	"time"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// StatsBroadcaster - интерфейс для отправки обновлений статистики через WebSocket
type StatsBroadcaster interface {
	BroadcastStatsUpdate(stats *models.Stats)
}

// StatsService предоставляет бизнес-логику для работы со статистикой.
//
// Функции:
// - GetStats: получить полную агрегированную статистику
// - GetTopPairs: получить топ-5 пар по указанной метрике
// - ResetStats: сброс счетчиков статистики
// - RecordTradeCompletion: записать завершенную сделку
//
// WebSocket интеграция:
// - После каждой записи сделки отправляет statsUpdate через WebSocket
type StatsService struct {
	statsRepo *repository.StatsRepository
	pairRepo  *repository.PairRepository
	wsHub     StatsBroadcaster
}

// NewStatsService создает новый экземпляр StatsService
func NewStatsService(statsRepo *repository.StatsRepository, pairRepo *repository.PairRepository) *StatsService {
	return &StatsService{
		statsRepo: statsRepo,
		pairRepo:  pairRepo,
	}
}

// SetWebSocketHub устанавливает WebSocket hub для broadcast статистики.
//
// Вызывается после инициализации Hub в main.go:
//
//	statsService := service.NewStatsService(statsRepo, pairRepo)
//	statsService.SetWebSocketHub(wsHub)
func (s *StatsService) SetWebSocketHub(hub StatsBroadcaster) {
	s.wsHub = hub
}

// GetStats возвращает полную агрегированную статистику.
//
// Включает:
// - Количество сделок (сегодня/неделя/месяц/всего)
// - PNL (сегодня/неделя/месяц/всего)
// - Статистика Stop Loss (счетчики + последние события)
// - Статистика ликвидаций (счетчики + последние события)
// - Топ-5 пар по количеству сделок
// - Топ-5 пар по прибыли
// - Топ-5 пар по убыткам
func (s *StatsService) GetStats() (*models.Stats, error) {
	return s.statsRepo.GetStats()
}

// GetTopPairs возвращает топ-5 пар по указанной метрике.
//
// Поддерживаемые метрики:
// - "trades": пары с наибольшим количеством сделок
// - "profit": пары с наибольшей прибылью (PNL > 0)
// - "loss": пары с наибольшими убытками (PNL < 0)
//
// Возвращает массив PairStat с полями Symbol и Value.
func (s *StatsService) GetTopPairs(metric string, limit int) ([]models.PairStat, error) {
	if limit <= 0 {
		limit = 5
	}

	switch metric {
	case "trades":
		return s.statsRepo.GetTopPairsByTrades(limit)
	case "profit":
		return s.statsRepo.GetTopPairsByProfit(limit)
	case "loss":
		return s.statsRepo.GetTopPairsByLoss(limit)
	default:
		// По умолчанию возвращаем топ по сделкам
		return s.statsRepo.GetTopPairsByTrades(limit)
	}
}

// ResetStats сбрасывает все счетчики статистики.
//
// ВАЖНО: Это действие необратимо!
// Удаляет все записи из таблицы trades.
// Используется по явному запросу пользователя.
// После сброса отправляет statsUpdate через WebSocket.
func (s *StatsService) ResetStats() error {
	if err := s.statsRepo.ResetCounters(); err != nil {
		return err
	}

	// Broadcast обнуленной статистики через WebSocket
	if s.wsHub != nil {
		stats, err := s.statsRepo.GetStats()
		if err == nil && stats != nil {
			s.wsHub.BroadcastStatsUpdate(stats)
		}
	}

	return nil
}

// RecordTradeCompletion записывает завершенную сделку.
//
// Вызывается после успешного закрытия арбитражной позиции.
// Обновляет:
// - Глобальную статистику (таблица trades)
// - Локальную статистику пары (trades_count, total_pnl)
// - Отправляет statsUpdate через WebSocket
//
// Параметры:
// - pairID: ID торговой пары
// - symbol: символ пары (например, "BTCUSDT")
// - exchanges: биржи [long_exchange, short_exchange]
// - entryTime: время входа в позицию
// - exitTime: время выхода из позиции
// - pnl: реализованный PNL в USDT
// - wasStopLoss: сделка закрыта по Stop Loss
// - wasLiquidation: была ликвидация
func (s *StatsService) RecordTradeCompletion(
	pairID int,
	symbol string,
	exchanges [2]string,
	entryTime, exitTime time.Time,
	pnl float64,
	wasStopLoss, wasLiquidation bool,
) error {
	// Записываем в таблицу trades
	err := s.statsRepo.RecordTrade(pairID, symbol, exchanges, entryTime, exitTime, pnl, wasStopLoss, wasLiquidation)
	if err != nil {
		return err
	}

	// Обновляем локальную статистику пары (если pairRepo доступен)
	if s.pairRepo != nil {
		// Увеличиваем счетчик сделок
		if err := s.pairRepo.IncrementTrades(pairID); err != nil {
			// Логируем ошибку, но не прерываем - основная запись уже сделана
		}

		// Обновляем PNL пары
		if err := s.pairRepo.UpdatePnl(pairID, pnl); err != nil {
			// Логируем ошибку, но не прерываем
		}
	}

	// Broadcast обновленной статистики через WebSocket
	if s.wsHub != nil {
		// Получаем актуальную статистику и отправляем
		stats, err := s.statsRepo.GetStats()
		if err == nil && stats != nil {
			s.wsHub.BroadcastStatsUpdate(stats)
		}
	}

	return nil
}

// GetTradesByPair возвращает историю сделок для конкретной пары.
//
// Используется для отображения детальной статистики по паре.
func (s *StatsService) GetTradesByPair(pairID int, limit int) ([]*repository.Trade, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.statsRepo.GetTradesByPairID(pairID, limit)
}

// GetTradesInRange возвращает сделки за указанный период.
//
// Используется для построения отчетов и анализа.
func (s *StatsService) GetTradesInRange(from, to time.Time, limit int) ([]*repository.Trade, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.statsRepo.GetTradesInTimeRange(from, to, limit)
}

// GetTotalTradesCount возвращает общее количество сделок.
func (s *StatsService) GetTotalTradesCount() (int, error) {
	return s.statsRepo.Count()
}

// GetPNLBySymbol возвращает суммарный PNL по символу.
func (s *StatsService) GetPNLBySymbol(symbol string) (float64, error) {
	return s.statsRepo.GetPNLBySymbol(symbol)
}

// CleanupOldTrades удаляет записи старше указанной даты.
//
// Используется для автоматической очистки старых данных.
// Возвращает количество удаленных записей.
func (s *StatsService) CleanupOldTrades(olderThan time.Time) (int64, error) {
	return s.statsRepo.DeleteOlderThan(olderThan)
}
