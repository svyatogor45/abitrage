package service

import (
	"errors"
	"testing"
	"time"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// TestableStatsService - версия сервиса для тестирования
type TestableStatsService struct {
	statsRepo StatsRepositoryInterface
	pairRepo  PairRepositoryInterface
	wsHub     StatsBroadcaster
}

func newTestableStatsService(
	statsRepo StatsRepositoryInterface,
	pairRepo PairRepositoryInterface,
) *TestableStatsService {
	return &TestableStatsService{
		statsRepo: statsRepo,
		pairRepo:  pairRepo,
	}
}

func (s *TestableStatsService) SetWebSocketHub(hub StatsBroadcaster) {
	s.wsHub = hub
}

func (s *TestableStatsService) GetStats() (*models.Stats, error) {
	return s.statsRepo.GetStats()
}

func (s *TestableStatsService) GetTopPairs(metric string, limit int) ([]models.PairStat, error) {
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
		return s.statsRepo.GetTopPairsByTrades(limit)
	}
}

func (s *TestableStatsService) ResetStats() error {
	if err := s.statsRepo.ResetCounters(); err != nil {
		return err
	}

	if s.wsHub != nil {
		stats, err := s.statsRepo.GetStats()
		if err == nil && stats != nil {
			s.wsHub.BroadcastStatsUpdate(stats)
		}
	}

	return nil
}

func (s *TestableStatsService) RecordTradeCompletion(
	pairID int,
	symbol string,
	exchanges [2]string,
	entryTime, exitTime time.Time,
	pnl float64,
	wasStopLoss, wasLiquidation bool,
) error {
	err := s.statsRepo.RecordTrade(pairID, symbol, exchanges, entryTime, exitTime, pnl, wasStopLoss, wasLiquidation)
	if err != nil {
		return err
	}

	if s.pairRepo != nil {
		_ = s.pairRepo.IncrementTrades(pairID)
		_ = s.pairRepo.UpdatePnl(pairID, pnl)
	}

	if s.wsHub != nil {
		stats, err := s.statsRepo.GetStats()
		if err == nil && stats != nil {
			s.wsHub.BroadcastStatsUpdate(stats)
		}
	}

	return nil
}

func (s *TestableStatsService) GetTradesByPair(pairID int, limit int) ([]*repository.Trade, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.statsRepo.GetTradesByPairID(pairID, limit)
}

func (s *TestableStatsService) GetTradesInRange(from, to time.Time, limit int) ([]*repository.Trade, error) {
	if limit <= 0 {
		limit = 100
	}
	return s.statsRepo.GetTradesInTimeRange(from, to, limit)
}

func (s *TestableStatsService) GetTotalTradesCount() (int, error) {
	return s.statsRepo.Count()
}

func (s *TestableStatsService) GetPNLBySymbol(symbol string) (float64, error) {
	return s.statsRepo.GetPNLBySymbol(symbol)
}

func (s *TestableStatsService) CleanupOldTrades(olderThan time.Time) (int64, error) {
	return s.statsRepo.DeleteOlderThan(olderThan)
}

// ============ ТЕСТЫ ============

func TestStatsService_GetStats(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockStatsRepository)
		check   func(*testing.T, *models.Stats)
		wantErr bool
	}{
		{
			name: "получение пустой статистики",
			check: func(t *testing.T, s *models.Stats) {
				if s.TotalTrades != 0 {
					t.Errorf("expected 0 total trades, got %d", s.TotalTrades)
				}
				if s.TotalPnl != 0 {
					t.Errorf("expected 0 total PNL, got %f", s.TotalPnl)
				}
			},
		},
		{
			name: "получение статистики с данными",
			setup: func(m *MockStatsRepository) {
				now := time.Now()
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now.Add(-1*time.Hour), now, 100.0, false, false)
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now.Add(-2*time.Hour), now.Add(-1*time.Hour), -50.0, true, false)
			},
			check: func(t *testing.T, s *models.Stats) {
				if s.TotalTrades != 2 {
					t.Errorf("expected 2 total trades, got %d", s.TotalTrades)
				}
				if s.TotalPnl != 50.0 {
					t.Errorf("expected 50.0 total PNL, got %f", s.TotalPnl)
				}
			},
		},
		{
			name: "ошибка базы данных",
			setup: func(m *MockStatsRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStatsRepo := NewMockStatsRepository()
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockStatsRepo)
			}

			svc := newTestableStatsService(mockStatsRepo, mockPairRepo)
			stats, err := svc.GetStats()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.check != nil {
				tt.check(t, stats)
			}
		})
	}
}

func TestStatsService_GetTopPairs(t *testing.T) {
	tests := []struct {
		name      string
		metric    string
		limit     int
		setup     func(*MockStatsRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "топ по сделкам",
			metric: "trades",
			limit:  5,
			setup: func(m *MockStatsRepository) {
				now := time.Now()
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 10.0, false, false)
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 10.0, false, false)
				_ = m.RecordTrade(2, "ETHUSDT", [2]string{"bybit", "okx"}, now, now, 5.0, false, false)
			},
			wantCount: 2,
		},
		{
			name:   "топ по прибыли",
			metric: "profit",
			limit:  5,
			setup: func(m *MockStatsRepository) {
				now := time.Now()
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 100.0, false, false)
				_ = m.RecordTrade(2, "ETHUSDT", [2]string{"bybit", "okx"}, now, now, -50.0, false, false)
			},
			wantCount: 1, // только прибыльные
		},
		{
			name:   "топ по убыткам",
			metric: "loss",
			limit:  5,
			setup: func(m *MockStatsRepository) {
				now := time.Now()
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 100.0, false, false)
				_ = m.RecordTrade(2, "ETHUSDT", [2]string{"bybit", "okx"}, now, now, -50.0, false, false)
			},
			wantCount: 1, // только убыточные
		},
		{
			name:      "дефолтный лимит",
			metric:    "trades",
			limit:     0,
			wantCount: 0,
		},
		{
			name:      "неизвестная метрика - используем trades",
			metric:    "unknown",
			limit:     5,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStatsRepo := NewMockStatsRepository()
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockStatsRepo)
			}

			svc := newTestableStatsService(mockStatsRepo, mockPairRepo)
			pairs, err := svc.GetTopPairs(tt.metric, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(pairs) != tt.wantCount {
				t.Errorf("expected %d pairs, got %d", tt.wantCount, len(pairs))
			}
		})
	}
}

func TestStatsService_ResetStats(t *testing.T) {
	tests := []struct {
		name          string
		setup         func(*MockStatsRepository)
		wantErr       bool
		wantBroadcast bool
	}{
		{
			name: "успешный сброс",
			setup: func(m *MockStatsRepository) {
				now := time.Now()
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 100.0, false, false)
			},
			wantBroadcast: true,
		},
		{
			name: "ошибка сброса",
			setup: func(m *MockStatsRepository) {
				m.deleteErr = errors.New("delete error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStatsRepo := NewMockStatsRepository()
			mockPairRepo := NewMockPairRepository()
			mockWsHub := NewMockStatsBroadcaster()

			if tt.setup != nil {
				tt.setup(mockStatsRepo)
			}

			svc := newTestableStatsService(mockStatsRepo, mockPairRepo)
			svc.SetWebSocketHub(mockWsHub)

			err := svc.ResetStats()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantBroadcast && len(mockWsHub.updates) == 0 {
				t.Error("expected broadcast, got none")
			}
		})
	}
}

func TestStatsService_RecordTradeCompletion(t *testing.T) {
	tests := []struct {
		name           string
		pairID         int
		symbol         string
		exchanges      [2]string
		pnl            float64
		wasStopLoss    bool
		wasLiquidation bool
		setup          func(*MockStatsRepository, *MockPairRepository)
		wantErr        bool
		wantBroadcast  bool
	}{
		{
			name:      "успешная запись прибыльной сделки",
			pairID:    1,
			symbol:    "BTCUSDT",
			exchanges: [2]string{"bybit", "okx"},
			pnl:       100.0,
			setup: func(s *MockStatsRepository, p *MockPairRepository) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT"}
			},
			wantBroadcast: true,
		},
		{
			name:        "запись сделки с SL",
			pairID:      1,
			symbol:      "BTCUSDT",
			exchanges:   [2]string{"bybit", "okx"},
			pnl:         -50.0,
			wasStopLoss: true,
			setup: func(s *MockStatsRepository, p *MockPairRepository) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT"}
			},
			wantBroadcast: true,
		},
		{
			name:           "запись сделки с ликвидацией",
			pairID:         1,
			symbol:         "BTCUSDT",
			exchanges:      [2]string{"bybit", "okx"},
			pnl:            -200.0,
			wasLiquidation: true,
			setup: func(s *MockStatsRepository, p *MockPairRepository) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT"}
			},
			wantBroadcast: true,
		},
		{
			name:      "ошибка записи",
			pairID:    1,
			symbol:    "BTCUSDT",
			exchanges: [2]string{"bybit", "okx"},
			pnl:       100.0,
			setup: func(s *MockStatsRepository, p *MockPairRepository) {
				s.createErr = errors.New("create error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStatsRepo := NewMockStatsRepository()
			mockPairRepo := NewMockPairRepository()
			mockWsHub := NewMockStatsBroadcaster()

			if tt.setup != nil {
				tt.setup(mockStatsRepo, mockPairRepo)
			}

			svc := newTestableStatsService(mockStatsRepo, mockPairRepo)
			svc.SetWebSocketHub(mockWsHub)

			now := time.Now()
			err := svc.RecordTradeCompletion(
				tt.pairID,
				tt.symbol,
				tt.exchanges,
				now.Add(-1*time.Hour),
				now,
				tt.pnl,
				tt.wasStopLoss,
				tt.wasLiquidation,
			)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if tt.wantBroadcast && len(mockWsHub.updates) == 0 {
				t.Error("expected broadcast, got none")
			}

			// Проверяем, что сделка записана
			count, _ := mockStatsRepo.Count()
			if count != 1 {
				t.Errorf("expected 1 trade, got %d", count)
			}
		})
	}
}

func TestStatsService_GetTradesByPair(t *testing.T) {
	tests := []struct {
		name      string
		pairID    int
		limit     int
		setup     func(*MockStatsRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:   "получение сделок по паре",
			pairID: 1,
			limit:  100,
			setup: func(m *MockStatsRepository) {
				now := time.Now()
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 100.0, false, false)
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 50.0, false, false)
				_ = m.RecordTrade(2, "ETHUSDT", [2]string{"bybit", "okx"}, now, now, 25.0, false, false)
			},
			wantCount: 2,
		},
		{
			name:      "дефолтный лимит",
			pairID:    1,
			limit:     0,
			wantCount: 0,
		},
		{
			name:   "ошибка получения",
			pairID: 1,
			limit:  100,
			setup: func(m *MockStatsRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStatsRepo := NewMockStatsRepository()
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockStatsRepo)
			}

			svc := newTestableStatsService(mockStatsRepo, mockPairRepo)
			trades, err := svc.GetTradesByPair(tt.pairID, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(trades) != tt.wantCount {
				t.Errorf("expected %d trades, got %d", tt.wantCount, len(trades))
			}
		})
	}
}

func TestStatsService_GetTradesInRange(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		from      time.Time
		to        time.Time
		limit     int
		setup     func(*MockStatsRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:  "получение сделок за период",
			from:  now.Add(-2 * time.Hour),
			to:    now,
			limit: 100,
			setup: func(m *MockStatsRepository) {
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now.Add(-3*time.Hour), now.Add(-1*time.Hour), 100.0, false, false)
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now.Add(-5*time.Hour), now.Add(-4*time.Hour), 50.0, false, false)
			},
			wantCount: 1, // только первая попадает в диапазон
		},
		{
			name:      "дефолтный лимит",
			from:      now.Add(-1 * time.Hour),
			to:        now,
			limit:     0,
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStatsRepo := NewMockStatsRepository()
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockStatsRepo)
			}

			svc := newTestableStatsService(mockStatsRepo, mockPairRepo)
			trades, err := svc.GetTradesInRange(tt.from, tt.to, tt.limit)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if len(trades) != tt.wantCount {
				t.Errorf("expected %d trades, got %d", tt.wantCount, len(trades))
			}
		})
	}
}

func TestStatsService_GetTotalTradesCount(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockStatsRepository)
		want    int
		wantErr bool
	}{
		{
			name: "подсчет сделок",
			setup: func(m *MockStatsRepository) {
				now := time.Now()
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 100.0, false, false)
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 50.0, false, false)
			},
			want: 2,
		},
		{
			name: "нет сделок",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStatsRepo := NewMockStatsRepository()
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockStatsRepo)
			}

			svc := newTestableStatsService(mockStatsRepo, mockPairRepo)
			count, err := svc.GetTotalTradesCount()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if count != tt.want {
				t.Errorf("expected %d, got %d", tt.want, count)
			}
		})
	}
}

func TestStatsService_GetPNLBySymbol(t *testing.T) {
	tests := []struct {
		name    string
		symbol  string
		setup   func(*MockStatsRepository)
		want    float64
		wantErr bool
	}{
		{
			name:   "суммарный PNL по символу",
			symbol: "BTCUSDT",
			setup: func(m *MockStatsRepository) {
				now := time.Now()
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 100.0, false, false)
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, -30.0, false, false)
				_ = m.RecordTrade(2, "ETHUSDT", [2]string{"bybit", "okx"}, now, now, 50.0, false, false)
			},
			want: 70.0,
		},
		{
			name:   "нет сделок по символу",
			symbol: "XRPUSDT",
			setup: func(m *MockStatsRepository) {
				now := time.Now()
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now, now, 100.0, false, false)
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStatsRepo := NewMockStatsRepository()
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockStatsRepo)
			}

			svc := newTestableStatsService(mockStatsRepo, mockPairRepo)
			pnl, err := svc.GetPNLBySymbol(tt.symbol)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if pnl != tt.want {
				t.Errorf("expected %f, got %f", tt.want, pnl)
			}
		})
	}
}

func TestStatsService_CleanupOldTrades(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		olderThan time.Time
		setup     func(*MockStatsRepository)
		want      int64
		wantErr   bool
	}{
		{
			name:      "очистка старых сделок",
			olderThan: now.Add(-1 * time.Hour),
			setup: func(m *MockStatsRepository) {
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now.Add(-3*time.Hour), now.Add(-2*time.Hour), 100.0, false, false)
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now.Add(-30*time.Minute), now, 50.0, false, false)
			},
			want: 1,
		},
		{
			name:      "нечего удалять",
			olderThan: now.Add(-10 * time.Hour),
			setup: func(m *MockStatsRepository) {
				_ = m.RecordTrade(1, "BTCUSDT", [2]string{"bybit", "okx"}, now.Add(-1*time.Hour), now, 100.0, false, false)
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockStatsRepo := NewMockStatsRepository()
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockStatsRepo)
			}

			svc := newTestableStatsService(mockStatsRepo, mockPairRepo)
			deleted, err := svc.CleanupOldTrades(tt.olderThan)

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if deleted != tt.want {
				t.Errorf("expected %d deleted, got %d", tt.want, deleted)
			}
		})
	}
}

func TestStatsService_PairStatsUpdate(t *testing.T) {
	mockStatsRepo := NewMockStatsRepository()
	mockPairRepo := NewMockPairRepository()
	mockWsHub := NewMockStatsBroadcaster()

	// Создаем пару
	mockPairRepo.pairs[1] = &models.PairConfig{
		ID:          1,
		Symbol:      "BTCUSDT",
		TradesCount: 0,
		TotalPnl:    0,
	}

	svc := newTestableStatsService(mockStatsRepo, mockPairRepo)
	svc.SetWebSocketHub(mockWsHub)

	now := time.Now()

	// Записываем несколько сделок
	_ = svc.RecordTradeCompletion(1, "BTCUSDT", [2]string{"bybit", "okx"}, now.Add(-1*time.Hour), now, 100.0, false, false)
	_ = svc.RecordTradeCompletion(1, "BTCUSDT", [2]string{"bybit", "okx"}, now.Add(-2*time.Hour), now.Add(-1*time.Hour), -30.0, true, false)

	// Проверяем обновление статистики пары
	pair := mockPairRepo.pairs[1]
	if pair.TradesCount != 2 {
		t.Errorf("expected 2 trades, got %d", pair.TradesCount)
	}
	if pair.TotalPnl != 70.0 {
		t.Errorf("expected 70.0 PNL, got %f", pair.TotalPnl)
	}

	// Проверяем broadcast
	if len(mockWsHub.updates) != 2 {
		t.Errorf("expected 2 broadcasts, got %d", len(mockWsHub.updates))
	}
}
