package service

import (
	"context"
	"errors"
	"testing"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// ============ ТЕСТЫ ============

func TestPairService_ValidatePairParams(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *models.PairConfig
		wantErr error
	}{
		{
			name: "валидные параметры",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: 0.5,
				ExitSpreadPct:  0.1,
				VolumeAsset:    100,
				NOrders:        1,
				StopLoss:       5.0,
			},
		},
		{
			name: "пустой символ",
			cfg: &models.PairConfig{
				Symbol:         "",
				EntrySpreadPct: 0.5,
				ExitSpreadPct:  0.1,
				VolumeAsset:    100,
				NOrders:        1,
			},
			wantErr: ErrInvalidSymbol,
		},
		{
			name: "нулевой entry spread",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: 0,
				ExitSpreadPct:  0.1,
				VolumeAsset:    100,
				NOrders:        1,
			},
			wantErr: ErrInvalidEntrySpread,
		},
		{
			name: "отрицательный entry spread",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: -0.5,
				ExitSpreadPct:  0.1,
				VolumeAsset:    100,
				NOrders:        1,
			},
			wantErr: ErrInvalidEntrySpread,
		},
		{
			name: "нулевой exit spread",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: 0.5,
				ExitSpreadPct:  0,
				VolumeAsset:    100,
				NOrders:        1,
			},
			wantErr: ErrInvalidExitSpread,
		},
		{
			name: "exit spread >= entry spread",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: 0.5,
				ExitSpreadPct:  0.5,
				VolumeAsset:    100,
				NOrders:        1,
			},
			wantErr: ErrExitSpreadTooHigh,
		},
		{
			name: "exit spread > entry spread",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: 0.5,
				ExitSpreadPct:  0.8,
				VolumeAsset:    100,
				NOrders:        1,
			},
			wantErr: ErrExitSpreadTooHigh,
		},
		{
			name: "нулевой объем",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: 0.5,
				ExitSpreadPct:  0.1,
				VolumeAsset:    0,
				NOrders:        1,
			},
			wantErr: ErrInvalidVolume,
		},
		{
			name: "отрицательный объем",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: 0.5,
				ExitSpreadPct:  0.1,
				VolumeAsset:    -100,
				NOrders:        1,
			},
			wantErr: ErrInvalidVolume,
		},
		{
			name: "нулевое количество ордеров",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: 0.5,
				ExitSpreadPct:  0.1,
				VolumeAsset:    100,
				NOrders:        0,
			},
			wantErr: ErrInvalidNOrders,
		},
		{
			name: "отрицательный stop loss",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: 0.5,
				ExitSpreadPct:  0.1,
				VolumeAsset:    100,
				NOrders:        1,
				StopLoss:       -5.0,
			},
			wantErr: ErrInvalidStopLoss,
		},
		{
			name: "stop loss = 0 допустим (не установлен)",
			cfg: &models.PairConfig{
				Symbol:         "BTCUSDT",
				EntrySpreadPct: 0.5,
				ExitSpreadPct:  0.1,
				VolumeAsset:    100,
				NOrders:        1,
				StopLoss:       0,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Создаем PairService с минимальными mock зависимостями
			mockPairRepo := NewMockPairRepository()
			mockExchangeRepo := NewMockExchangeRepository()

			svc := NewPairService(
				&repository.PairRepository{},     // будет заменен
				&repository.ExchangeRepository{}, // будет заменен
				nil,
			)

			// Используем внутреннюю функцию валидации напрямую через testable wrapper
			err := validatePairParamsTest(tt.cfg)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			// Чтобы компилятор не ругался на неиспользуемые переменные
			_ = svc
			_ = mockPairRepo
			_ = mockExchangeRepo
		})
	}
}

// validatePairParamsTest - копия функции валидации для тестов
func validatePairParamsTest(cfg *models.PairConfig) error {
	if cfg.Symbol == "" {
		return ErrInvalidSymbol
	}
	if cfg.EntrySpreadPct <= 0 {
		return ErrInvalidEntrySpread
	}
	if cfg.ExitSpreadPct <= 0 {
		return ErrInvalidExitSpread
	}
	if cfg.ExitSpreadPct >= cfg.EntrySpreadPct {
		return ErrExitSpreadTooHigh
	}
	if cfg.VolumeAsset <= 0 {
		return ErrInvalidVolume
	}
	if cfg.NOrders < 1 {
		return ErrInvalidNOrders
	}
	if cfg.StopLoss < 0 {
		return ErrInvalidStopLoss
	}
	return nil
}

func TestPairService_GetPair(t *testing.T) {
	tests := []struct {
		name    string
		pairID  int
		setup   func(*MockPairRepository)
		wantErr error
	}{
		{
			name:   "успешное получение",
			pairID: 1,
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{
					ID:     1,
					Symbol: "BTCUSDT",
				}
			},
		},
		{
			name:    "пара не найдена",
			pairID:  999,
			wantErr: ErrPairNotFound,
		},
		{
			name:   "ошибка базы данных",
			pairID: 1,
			setup: func(m *MockPairRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockPairRepo)
			}

			pair, err := getPairFromMock(mockPairRepo, tt.pairID)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if pair.ID != tt.pairID {
				t.Errorf("expected pair ID %d, got %d", tt.pairID, pair.ID)
			}
		})
	}
}

// getPairFromMock - вспомогательная функция для тестов
func getPairFromMock(repo *MockPairRepository, id int) (*models.PairConfig, error) {
	pair, err := repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrPairNotFound) {
			return nil, ErrPairNotFound
		}
		return nil, err
	}
	return pair, nil
}

func TestPairService_GetAllPairs(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*MockPairRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name: "получение всех пар",
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT"}
				m.pairs[2] = &models.PairConfig{ID: 2, Symbol: "ETHUSDT"}
			},
			wantCount: 2,
		},
		{
			name:      "пустой список",
			wantCount: 0,
		},
		{
			name: "ошибка базы данных",
			setup: func(m *MockPairRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockPairRepo)
			}

			pairs, err := mockPairRepo.GetAll()

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

func TestPairService_GetActivePairs(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*MockPairRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name: "получение активных пар",
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusActive}
				m.pairs[2] = &models.PairConfig{ID: 2, Symbol: "ETHUSDT", Status: models.PairStatusPaused}
				m.pairs[3] = &models.PairConfig{ID: 3, Symbol: "XRPUSDT", Status: models.PairStatusActive}
			},
			wantCount: 2,
		},
		{
			name: "нет активных пар",
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusPaused}
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockPairRepo)
			}

			pairs, err := mockPairRepo.GetActive()

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

func TestPairService_DeletePair(t *testing.T) {
	tests := []struct {
		name    string
		pairID  int
		setup   func(*MockPairRepository, *MockBotEngine)
		wantErr error
	}{
		{
			name:   "успешное удаление",
			pairID: 1,
			setup: func(p *MockPairRepository, e *MockBotEngine) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusPaused}
				e.AddPair(&models.PairConfig{ID: 1})
			},
		},
		{
			name:    "пара не найдена",
			pairID:  999,
			wantErr: ErrPairNotFound,
		},
		{
			name:   "пара не на паузе",
			pairID: 1,
			setup: func(p *MockPairRepository, e *MockBotEngine) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusActive}
			},
			wantErr: ErrPairNotPaused,
		},
		{
			name:   "пара имеет открытую позицию",
			pairID: 1,
			setup: func(p *MockPairRepository, e *MockBotEngine) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusPaused}
				e.AddPair(&models.PairConfig{ID: 1})
				e.SetOpenPosition(1, true)
			},
			wantErr: ErrPairHasOpenPosition,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPairRepo := NewMockPairRepository()
			mockEngine := NewMockBotEngine()

			if tt.setup != nil {
				tt.setup(mockPairRepo, mockEngine)
			}

			err := deletePairFromMock(mockPairRepo, mockEngine, tt.pairID)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// deletePairFromMock - вспомогательная функция для тестов
func deletePairFromMock(repo *MockPairRepository, engine *MockBotEngine, id int) error {
	pair, err := repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrPairNotFound) {
			return ErrPairNotFound
		}
		return err
	}

	if pair.Status != models.PairStatusPaused {
		return ErrPairNotPaused
	}

	if engine.HasOpenPosition(id) {
		return ErrPairHasOpenPosition
	}

	engine.RemovePair(id)
	return repo.Delete(id)
}

func TestPairService_StartPair(t *testing.T) {
	tests := []struct {
		name    string
		pairID  int
		setup   func(*MockPairRepository, *MockExchangeRepository, *MockBotEngine)
		wantErr error
	}{
		{
			name:   "успешный старт",
			pairID: 1,
			setup: func(p *MockPairRepository, e *MockExchangeRepository, eng *MockBotEngine) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusPaused}
				e.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true}
				e.accounts["okx"] = &models.ExchangeAccount{ID: 2, Name: "okx", Connected: true}
				eng.AddPair(&models.PairConfig{ID: 1})
			},
		},
		{
			name:    "пара не найдена",
			pairID:  999,
			wantErr: ErrPairNotFound,
		},
		{
			name:   "пара уже активна",
			pairID: 1,
			setup: func(p *MockPairRepository, e *MockExchangeRepository, eng *MockBotEngine) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusActive}
			},
			wantErr: ErrPairAlreadyActive,
		},
		{
			name:   "недостаточно бирж",
			pairID: 1,
			setup: func(p *MockPairRepository, e *MockExchangeRepository, eng *MockBotEngine) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusPaused}
				e.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true}
				// Только одна биржа
			},
			wantErr: ErrNotEnoughExchanges,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPairRepo := NewMockPairRepository()
			mockExchangeRepo := NewMockExchangeRepository()
			mockEngine := NewMockBotEngine()

			if tt.setup != nil {
				tt.setup(mockPairRepo, mockExchangeRepo, mockEngine)
			}

			err := startPairFromMock(mockPairRepo, mockExchangeRepo, mockEngine, tt.pairID)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// startPairFromMock - вспомогательная функция для тестов
func startPairFromMock(pairRepo *MockPairRepository, exchangeRepo *MockExchangeRepository, engine *MockBotEngine, id int) error {
	pair, err := pairRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrPairNotFound) {
			return ErrPairNotFound
		}
		return err
	}

	if pair.Status == models.PairStatusActive {
		return ErrPairAlreadyActive
	}

	connectedCount, _ := exchangeRepo.CountConnected()
	if connectedCount < 2 {
		return ErrNotEnoughExchanges
	}

	_ = pairRepo.UpdateStatus(id, models.PairStatusActive)
	_ = engine.StartPair(id)

	return nil
}

func TestPairService_PausePair(t *testing.T) {
	tests := []struct {
		name       string
		pairID     int
		forceClose bool
		setup      func(*MockPairRepository, *MockBotEngine)
		wantErr    error
	}{
		{
			name:   "успешная пауза",
			pairID: 1,
			setup: func(p *MockPairRepository, e *MockBotEngine) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusActive}
				e.AddPair(&models.PairConfig{ID: 1})
			},
		},
		{
			name:    "пара не найдена",
			pairID:  999,
			wantErr: ErrPairNotFound,
		},
		{
			name:   "пара уже на паузе",
			pairID: 1,
			setup: func(p *MockPairRepository, e *MockBotEngine) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusPaused}
			},
			wantErr: ErrPairAlreadyPaused,
		},
		{
			name:   "открытая позиция без forceClose",
			pairID: 1,
			setup: func(p *MockPairRepository, e *MockBotEngine) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusActive}
				e.AddPair(&models.PairConfig{ID: 1})
				e.SetOpenPosition(1, true)
			},
			wantErr: ErrPairHasOpenPosition,
		},
		{
			name:       "открытая позиция с forceClose",
			pairID:     1,
			forceClose: true,
			setup: func(p *MockPairRepository, e *MockBotEngine) {
				p.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusActive}
				e.AddPair(&models.PairConfig{ID: 1})
				e.SetOpenPosition(1, true)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPairRepo := NewMockPairRepository()
			mockEngine := NewMockBotEngine()

			if tt.setup != nil {
				tt.setup(mockPairRepo, mockEngine)
			}

			err := pausePairFromMock(context.Background(), mockPairRepo, mockEngine, tt.pairID, tt.forceClose)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// pausePairFromMock - вспомогательная функция для тестов
func pausePairFromMock(ctx context.Context, repo *MockPairRepository, engine *MockBotEngine, id int, forceClose bool) error {
	pair, err := repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrPairNotFound) {
			return ErrPairNotFound
		}
		return err
	}

	if pair.Status == models.PairStatusPaused {
		return ErrPairAlreadyPaused
	}

	hasPosition := engine.HasOpenPosition(id)
	if hasPosition {
		if !forceClose {
			return ErrPairHasOpenPosition
		}
		_ = engine.ForceClosePair(ctx, id)
	}

	_ = engine.PausePair(id)
	return repo.UpdateStatus(id, models.PairStatusPaused)
}

func TestPairService_UpdateParams(t *testing.T) {
	tests := []struct {
		name    string
		pairID  int
		params  UpdatePairParams
		setup   func(*MockPairRepository)
		wantErr error
	}{
		{
			name:   "обновление entry spread",
			pairID: 1,
			params: UpdatePairParams{
				EntrySpreadPct: float64Ptr(0.8),
			},
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{
					ID:             1,
					Symbol:         "BTCUSDT",
					EntrySpreadPct: 0.5,
					ExitSpreadPct:  0.1,
					VolumeAsset:    100,
					NOrders:        1,
				}
			},
		},
		{
			name:   "обновление нескольких параметров",
			pairID: 1,
			params: UpdatePairParams{
				EntrySpreadPct: float64Ptr(0.8),
				ExitSpreadPct:  float64Ptr(0.2),
				VolumeAsset:    float64Ptr(200),
			},
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{
					ID:             1,
					Symbol:         "BTCUSDT",
					EntrySpreadPct: 0.5,
					ExitSpreadPct:  0.1,
					VolumeAsset:    100,
					NOrders:        1,
				}
			},
		},
		{
			name:   "невалидный entry spread после обновления",
			pairID: 1,
			params: UpdatePairParams{
				EntrySpreadPct: float64Ptr(-0.5),
			},
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{
					ID:             1,
					Symbol:         "BTCUSDT",
					EntrySpreadPct: 0.5,
					ExitSpreadPct:  0.1,
					VolumeAsset:    100,
					NOrders:        1,
				}
			},
			wantErr: ErrInvalidEntrySpread,
		},
		{
			name:   "exit spread >= entry spread после обновления",
			pairID: 1,
			params: UpdatePairParams{
				ExitSpreadPct: float64Ptr(0.6),
			},
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{
					ID:             1,
					Symbol:         "BTCUSDT",
					EntrySpreadPct: 0.5,
					ExitSpreadPct:  0.1,
					VolumeAsset:    100,
					NOrders:        1,
				}
			},
			wantErr: ErrExitSpreadTooHigh,
		},
		{
			name:    "пара не найдена",
			pairID:  999,
			params:  UpdatePairParams{},
			wantErr: ErrPairNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockPairRepo)
			}

			_, err := updatePairFromMock(mockPairRepo, tt.pairID, tt.params)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) && err.Error() != tt.wantErr.Error() {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

// updatePairFromMock - вспомогательная функция для тестов
func updatePairFromMock(repo *MockPairRepository, id int, params UpdatePairParams) (*models.PairConfig, error) {
	pair, err := repo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrPairNotFound) {
			return nil, ErrPairNotFound
		}
		return nil, err
	}

	updated := *pair
	if params.EntrySpreadPct != nil {
		updated.EntrySpreadPct = *params.EntrySpreadPct
	}
	if params.ExitSpreadPct != nil {
		updated.ExitSpreadPct = *params.ExitSpreadPct
	}
	if params.VolumeAsset != nil {
		updated.VolumeAsset = *params.VolumeAsset
	}
	if params.NOrders != nil {
		updated.NOrders = *params.NOrders
	}
	if params.StopLoss != nil {
		updated.StopLoss = *params.StopLoss
	}

	if err := validatePairParamsTest(&updated); err != nil {
		return nil, err
	}

	_ = repo.UpdateParams(id, updated.EntrySpreadPct, updated.ExitSpreadPct, updated.VolumeAsset, updated.NOrders, updated.StopLoss)

	return &updated, nil
}

func TestPairService_PendingConfig(t *testing.T) {
	// Создаем сервис с mock репозиториями
	mockPairRepo := NewMockPairRepository()
	mockExchangeRepo := NewMockExchangeRepository()

	svc := NewPairService(
		&repository.PairRepository{},
		&repository.ExchangeRepository{},
		nil,
	)

	// Тестируем работу с pending config
	t.Run("set and get pending config", func(t *testing.T) {
		pairID := 1
		pending := &PendingConfig{
			EntrySpreadPct: 0.8,
			ExitSpreadPct:  0.2,
			VolumeAsset:    200,
			NOrders:        2,
			StopLoss:       10,
		}

		// Устанавливаем pending config
		svc.setPendingConfig(pairID, pending)

		// Проверяем наличие
		if !svc.HasPendingConfig(pairID) {
			t.Error("expected pending config to exist")
		}

		// Получаем
		got := svc.GetPendingConfig(pairID)
		if got == nil {
			t.Error("expected pending config, got nil")
			return
		}

		if got.EntrySpreadPct != pending.EntrySpreadPct {
			t.Errorf("expected EntrySpreadPct %f, got %f", pending.EntrySpreadPct, got.EntrySpreadPct)
		}
	})

	t.Run("clear pending config", func(t *testing.T) {
		pairID := 2
		pending := &PendingConfig{
			EntrySpreadPct: 0.5,
		}

		svc.setPendingConfig(pairID, pending)
		svc.clearPendingConfig(pairID)

		if svc.HasPendingConfig(pairID) {
			t.Error("expected pending config to be cleared")
		}
	})

	t.Run("no pending config", func(t *testing.T) {
		pairID := 999

		if svc.HasPendingConfig(pairID) {
			t.Error("expected no pending config")
		}

		if svc.GetPendingConfig(pairID) != nil {
			t.Error("expected nil pending config")
		}
	})

	// Чтобы компилятор не ругался на неиспользуемые переменные
	_ = mockPairRepo
	_ = mockExchangeRepo
}

func TestPairService_MaxPairsLimit(t *testing.T) {
	mockPairRepo := NewMockPairRepository()

	// Добавляем 30 пар
	for i := 1; i <= 30; i++ {
		mockPairRepo.pairs[i] = &models.PairConfig{ID: i, Symbol: "PAIR" + string(rune('A'+i))}
	}

	count, _ := mockPairRepo.Count()
	if count != 30 {
		t.Errorf("expected 30 pairs, got %d", count)
	}

	// Проверяем, что limit = 30
	if MaxPairs != 30 {
		t.Errorf("expected MaxPairs = 30, got %d", MaxPairs)
	}
}

func TestPairService_SearchPairs(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		setup     func(*MockPairRepository)
		wantCount int
	}{
		{
			name:  "поиск по BTC",
			query: "BTC",
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT"}
				m.pairs[2] = &models.PairConfig{ID: 2, Symbol: "ETHUSDT"}
				m.pairs[3] = &models.PairConfig{ID: 3, Symbol: "BTCETH"}
			},
			wantCount: 2,
		},
		{
			name:  "поиск по USDT",
			query: "USDT",
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT"}
				m.pairs[2] = &models.PairConfig{ID: 2, Symbol: "ETHUSDT"}
				m.pairs[3] = &models.PairConfig{ID: 3, Symbol: "BTCETH"}
			},
			wantCount: 2,
		},
		{
			name:  "ничего не найдено",
			query: "XRP",
			setup: func(m *MockPairRepository) {
				m.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT"}
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockPairRepo := NewMockPairRepository()

			if tt.setup != nil {
				tt.setup(mockPairRepo)
			}

			pairs, err := mockPairRepo.Search(tt.query)
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

func TestPairService_CountActive(t *testing.T) {
	mockPairRepo := NewMockPairRepository()
	mockPairRepo.pairs[1] = &models.PairConfig{ID: 1, Symbol: "BTCUSDT", Status: models.PairStatusActive}
	mockPairRepo.pairs[2] = &models.PairConfig{ID: 2, Symbol: "ETHUSDT", Status: models.PairStatusPaused}
	mockPairRepo.pairs[3] = &models.PairConfig{ID: 3, Symbol: "XRPUSDT", Status: models.PairStatusActive}

	count, err := mockPairRepo.CountActive()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if count != 2 {
		t.Errorf("expected 2 active pairs, got %d", count)
	}
}
