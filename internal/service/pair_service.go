package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// Ошибки сервиса пар
var (
	ErrPairNotFound           = errors.New("pair not found")
	ErrPairAlreadyExists      = errors.New("pair with this symbol already exists")
	ErrInvalidEntrySpread     = errors.New("entry spread must be greater than 0")
	ErrInvalidExitSpread      = errors.New("exit spread must be greater than 0")
	ErrInvalidVolume          = errors.New("volume must be greater than 0")
	ErrInvalidNOrders         = errors.New("number of orders must be at least 1")
	ErrInvalidStopLoss        = errors.New("stop loss must be non-negative")
	ErrExitSpreadTooHigh      = errors.New("exit spread must be less than entry spread")
	ErrSymbolNotAvailable     = errors.New("symbol must be available on at least 2 connected exchanges")
	ErrNotEnoughExchanges     = errors.New("at least 2 exchanges must be connected for arbitrage")
	ErrPairHasOpenPosition    = errors.New("cannot delete pair with open position")
	ErrPairNotPaused          = errors.New("pair must be paused to delete")
	ErrPairAlreadyActive      = errors.New("pair is already active")
	ErrPairAlreadyPaused      = errors.New("pair is already paused")
	ErrMaxPairsReached        = errors.New("maximum number of pairs (30) reached")
	ErrInvalidSymbol          = errors.New("invalid symbol format")
	ErrPositionOpenCannotEdit = errors.New("cannot edit pair with open position without pending flag")
)

// MaxPairs - максимальное количество пар (из ТЗ)
const MaxPairs = 30

// BotEngine определяет интерфейс для взаимодействия с торговым движком
type BotEngine interface {
	// AddPair добавляет пару в движок
	AddPair(cfg *models.PairConfig)
	// RemovePair удаляет пару из движка
	RemovePair(pairID int)
	// StartPair запускает мониторинг пары
	StartPair(pairID int) error
	// PausePair останавливает пару (без закрытия позиций)
	PausePair(pairID int) error
	// GetPairRuntime возвращает runtime состояние пары
	GetPairRuntime(pairID int) *models.PairRuntime
	// ForceClosePair принудительно закрывает позиции пары
	ForceClosePair(ctx context.Context, pairID int) error
	// UpdatePairConfig обновляет конфигурацию пары в движке
	UpdatePairConfig(pairID int, cfg *models.PairConfig)
	// HasOpenPosition проверяет, есть ли открытая позиция
	HasOpenPosition(pairID int) bool
}

// PairService - бизнес-логика для управления торговыми парами
type PairService struct {
	pairRepo     *repository.PairRepository
	exchangeRepo *repository.ExchangeRepository
	exchangeSvc  *ExchangeService

	// Торговый движок (может быть nil при инициализации)
	engine BotEngine

	// Отложенные изменения параметров (применяются после закрытия позиции)
	// map[pairID] -> pending config
	pendingChanges map[int]*PendingConfig
	pendingMu      sync.RWMutex
}

// PendingConfig хранит отложенные изменения параметров пары
type PendingConfig struct {
	EntrySpreadPct float64   `json:"entry_spread"`
	ExitSpreadPct  float64   `json:"exit_spread"`
	VolumeAsset    float64   `json:"volume"`
	NOrders        int       `json:"n_orders"`
	StopLoss       float64   `json:"stop_loss"`
	CreatedAt      time.Time `json:"created_at"`
}

// NewPairService создает новый экземпляр сервиса пар
func NewPairService(
	pairRepo *repository.PairRepository,
	exchangeRepo *repository.ExchangeRepository,
	exchangeSvc *ExchangeService,
) *PairService {
	return &PairService{
		pairRepo:       pairRepo,
		exchangeRepo:   exchangeRepo,
		exchangeSvc:    exchangeSvc,
		pendingChanges: make(map[int]*PendingConfig),
	}
}

// SetEngine устанавливает торговый движок
// Вызывается после инициализации Engine
func (s *PairService) SetEngine(engine BotEngine) {
	s.engine = engine
}

// CreatePair создает новую торговую пару
// Выполняет:
// 1. Валидацию всех параметров
// 2. Проверку лимита количества пар (max 30)
// 3. Проверку доступности актива на ≥2 биржах
// 4. Сохранение в БД
// 5. Добавление в торговый движок
func (s *PairService) CreatePair(ctx context.Context, cfg *models.PairConfig) error {
	// 1. Валидация параметров
	if err := s.validatePairParams(cfg); err != nil {
		return err
	}

	// 2. Проверка лимита количества пар
	count, err := s.pairRepo.Count()
	if err != nil {
		return err
	}
	if count >= MaxPairs {
		return ErrMaxPairsReached
	}

	// 3. Проверка уникальности символа
	exists, err := s.pairRepo.ExistsBySymbol(cfg.Symbol)
	if err != nil {
		return err
	}
	if exists {
		return ErrPairAlreadyExists
	}

	// 4. Проверка доступности актива на ≥2 биржах
	availableExchanges, err := s.checkSymbolAvailability(ctx, cfg.Symbol)
	if err != nil {
		return err
	}
	if len(availableExchanges) < 2 {
		return ErrSymbolNotAvailable
	}

	// 5. Устанавливаем значения по умолчанию
	cfg.Status = models.PairStatusPaused
	if cfg.NOrders == 0 {
		cfg.NOrders = 1
	}

	// 6. Нормализация символа (uppercase)
	cfg.Symbol = strings.ToUpper(cfg.Symbol)
	cfg.Base = strings.ToUpper(cfg.Base)
	cfg.Quote = strings.ToUpper(cfg.Quote)

	// 7. Сохраняем в БД
	if err := s.pairRepo.Create(cfg); err != nil {
		if errors.Is(err, repository.ErrPairExists) {
			return ErrPairAlreadyExists
		}
		return err
	}

	// 8. Добавляем в торговый движок (если инициализирован)
	if s.engine != nil {
		s.engine.AddPair(cfg)
	}

	return nil
}

// UpdatePair обновляет параметры торговой пары
// Выполняет:
// 1. Валидацию параметров
// 2. Проверку наличия открытой позиции
// 3. Если позиция открыта - сохраняет изменения как отложенные
// 4. Если позиции нет - применяет немедленно
func (s *PairService) UpdatePair(ctx context.Context, id int, params UpdatePairParams) (*models.PairConfig, error) {
	// 1. Получаем текущую пару
	pair, err := s.pairRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrPairNotFound) {
			return nil, ErrPairNotFound
		}
		return nil, err
	}

	// 2. Применяем изменения к копии для валидации
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

	// 3. Валидация новых параметров
	if err := s.validatePairParams(&updated); err != nil {
		return nil, err
	}

	// 4. Проверяем, есть ли открытая позиция
	hasPosition := s.hasOpenPosition(id)

	if hasPosition {
		// Сохраняем как отложенные изменения
		s.setPendingConfig(id, &PendingConfig{
			EntrySpreadPct: updated.EntrySpreadPct,
			ExitSpreadPct:  updated.ExitSpreadPct,
			VolumeAsset:    updated.VolumeAsset,
			NOrders:        updated.NOrders,
			StopLoss:       updated.StopLoss,
			CreatedAt:      time.Now(),
		})

		// Возвращаем текущую конфигурацию (изменения отложены)
		return pair, nil
	}

	// 5. Применяем изменения немедленно
	if err := s.pairRepo.UpdateParams(
		id,
		updated.EntrySpreadPct,
		updated.ExitSpreadPct,
		updated.VolumeAsset,
		updated.NOrders,
		updated.StopLoss,
	); err != nil {
		return nil, err
	}

	// 6. Обновляем в движке
	if s.engine != nil {
		s.engine.UpdatePairConfig(id, &updated)
	}

	return &updated, nil
}

// UpdatePairParams содержит параметры для обновления пары
type UpdatePairParams struct {
	EntrySpreadPct *float64 `json:"entry_spread,omitempty"`
	ExitSpreadPct  *float64 `json:"exit_spread,omitempty"`
	VolumeAsset    *float64 `json:"volume,omitempty"`
	NOrders        *int     `json:"n_orders,omitempty"`
	StopLoss       *float64 `json:"stop_loss,omitempty"`
}

// DeletePair удаляет торговую пару
// Выполняет:
// 1. Проверку, что пара на паузе
// 2. Проверку отсутствия открытых позиций
// 3. Удаление из БД и движка
func (s *PairService) DeletePair(ctx context.Context, id int) error {
	// 1. Получаем пару
	pair, err := s.pairRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrPairNotFound) {
			return ErrPairNotFound
		}
		return err
	}

	// 2. Проверяем, что пара на паузе
	if pair.Status != models.PairStatusPaused {
		return ErrPairNotPaused
	}

	// 3. Проверяем отсутствие открытых позиций
	if s.hasOpenPosition(id) {
		return ErrPairHasOpenPosition
	}

	// 4. Удаляем из движка
	if s.engine != nil {
		s.engine.RemovePair(id)
	}

	// 5. Очищаем отложенные изменения
	s.clearPendingConfig(id)

	// 6. Удаляем из БД
	return s.pairRepo.Delete(id)
}

// StartPair запускает мониторинг торговой пары
// Выполняет:
// 1. Проверку, что пара существует
// 2. Проверку, что пара не активна
// 3. Проверку подключения ≥2 бирж
// 4. Изменение статуса и запуск в движке
func (s *PairService) StartPair(ctx context.Context, id int) error {
	// 1. Получаем пару
	pair, err := s.pairRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrPairNotFound) {
			return ErrPairNotFound
		}
		return err
	}

	// 2. Проверяем статус
	if pair.Status == models.PairStatusActive {
		return ErrPairAlreadyActive
	}

	// 3. Проверяем, что подключено ≥2 бирж
	hasMinExchanges, err := s.exchangeSvc.HasMinimumExchanges()
	if err != nil {
		return err
	}
	if !hasMinExchanges {
		return ErrNotEnoughExchanges
	}

	// 4. Проверяем доступность символа
	available, err := s.checkSymbolAvailability(ctx, pair.Symbol)
	if err != nil {
		return err
	}
	if len(available) < 2 {
		return ErrSymbolNotAvailable
	}

	// 5. Обновляем статус в БД
	if err := s.pairRepo.UpdateStatus(id, models.PairStatusActive); err != nil {
		return err
	}

	// 6. Запускаем в движке
	if s.engine != nil {
		return s.engine.StartPair(id)
	}

	return nil
}

// PausePair приостанавливает торговую пару
// Параметр forceClose: если true и есть открытая позиция - принудительно закрыть
func (s *PairService) PausePair(ctx context.Context, id int, forceClose bool) error {
	// 1. Получаем пару
	pair, err := s.pairRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrPairNotFound) {
			return ErrPairNotFound
		}
		return err
	}

	// 2. Проверяем статус
	if pair.Status == models.PairStatusPaused {
		return ErrPairAlreadyPaused
	}

	// 3. Проверяем наличие открытой позиции
	hasPosition := s.hasOpenPosition(id)

	if hasPosition {
		if !forceClose {
			// Возвращаем ошибку - нужно явное подтверждение для закрытия
			return ErrPairHasOpenPosition
		}

		// Принудительное закрытие позиции
		if s.engine != nil {
			if err := s.engine.ForceClosePair(ctx, id); err != nil {
				return err
			}
		}
	}

	// 4. Останавливаем в движке
	if s.engine != nil {
		if err := s.engine.PausePair(id); err != nil {
			return err
		}
	}

	// 5. Обновляем статус в БД
	if err := s.pairRepo.UpdateStatus(id, models.PairStatusPaused); err != nil {
		return err
	}

	return nil
}

// GetPair возвращает пару по ID
func (s *PairService) GetPair(ctx context.Context, id int) (*models.PairConfig, error) {
	pair, err := s.pairRepo.GetByID(id)
	if err != nil {
		if errors.Is(err, repository.ErrPairNotFound) {
			return nil, ErrPairNotFound
		}
		return nil, err
	}
	return pair, nil
}

// GetAllPairs возвращает все пары
func (s *PairService) GetAllPairs(ctx context.Context) ([]*models.PairConfig, error) {
	return s.pairRepo.GetAll()
}

// GetActivePairs возвращает только активные пары
func (s *PairService) GetActivePairs(ctx context.Context) ([]*models.PairConfig, error) {
	return s.pairRepo.GetActive()
}

// GetPairRuntime возвращает runtime состояние пары из движка
func (s *PairService) GetPairRuntime(id int) *models.PairRuntime {
	if s.engine == nil {
		return nil
	}
	return s.engine.GetPairRuntime(id)
}

// GetPairWithRuntime возвращает пару с runtime данными
func (s *PairService) GetPairWithRuntime(ctx context.Context, id int) (*PairWithRuntime, error) {
	pair, err := s.GetPair(ctx, id)
	if err != nil {
		return nil, err
	}

	result := &PairWithRuntime{
		Config:        pair,
		Runtime:       s.GetPairRuntime(id),
		PendingConfig: s.GetPendingConfig(id),
	}

	return result, nil
}

// PairWithRuntime объединяет конфигурацию и runtime данные пары
type PairWithRuntime struct {
	Config        *models.PairConfig  `json:"config"`
	Runtime       *models.PairRuntime `json:"runtime,omitempty"`
	PendingConfig *PendingConfig      `json:"pending_config,omitempty"`
}

// GetPendingConfig возвращает отложенные изменения для пары
func (s *PairService) GetPendingConfig(id int) *PendingConfig {
	s.pendingMu.RLock()
	defer s.pendingMu.RUnlock()
	return s.pendingChanges[id]
}

// HasPendingConfig проверяет наличие отложенных изменений
func (s *PairService) HasPendingConfig(id int) bool {
	s.pendingMu.RLock()
	defer s.pendingMu.RUnlock()
	_, exists := s.pendingChanges[id]
	return exists
}

// ApplyPendingConfig применяет отложенные изменения
// Вызывается движком после закрытия позиции
func (s *PairService) ApplyPendingConfig(ctx context.Context, id int) error {
	s.pendingMu.Lock()
	pending, exists := s.pendingChanges[id]
	if !exists {
		s.pendingMu.Unlock()
		return nil // Нет отложенных изменений
	}
	delete(s.pendingChanges, id)
	s.pendingMu.Unlock()

	// Применяем изменения в БД
	if err := s.pairRepo.UpdateParams(
		id,
		pending.EntrySpreadPct,
		pending.ExitSpreadPct,
		pending.VolumeAsset,
		pending.NOrders,
		pending.StopLoss,
	); err != nil {
		return err
	}

	// Обновляем в движке
	if s.engine != nil {
		pair, err := s.pairRepo.GetByID(id)
		if err == nil {
			s.engine.UpdatePairConfig(id, pair)
		}
	}

	return nil
}

// GetPairsCount возвращает количество пар
func (s *PairService) GetPairsCount(ctx context.Context) (int, error) {
	return s.pairRepo.Count()
}

// GetActivePairsCount возвращает количество активных пар
func (s *PairService) GetActivePairsCount(ctx context.Context) (int, error) {
	return s.pairRepo.CountActive()
}

// SearchPairs ищет пары по символу
func (s *PairService) SearchPairs(ctx context.Context, query string) ([]*models.PairConfig, error) {
	return s.pairRepo.Search(query)
}

// ============ Вспомогательные методы ============

// validatePairParams выполняет валидацию параметров пары
func (s *PairService) validatePairParams(cfg *models.PairConfig) error {
	// Валидация символа
	if cfg.Symbol == "" {
		return ErrInvalidSymbol
	}

	// Валидация спреда входа (> 0)
	if cfg.EntrySpreadPct <= 0 {
		return ErrInvalidEntrySpread
	}

	// Валидация спреда выхода (> 0)
	if cfg.ExitSpreadPct <= 0 {
		return ErrInvalidExitSpread
	}

	// Спред выхода должен быть меньше спреда входа
	if cfg.ExitSpreadPct >= cfg.EntrySpreadPct {
		return ErrExitSpreadTooHigh
	}

	// Валидация объема (> 0)
	if cfg.VolumeAsset <= 0 {
		return ErrInvalidVolume
	}

	// Валидация количества ордеров (≥ 1)
	if cfg.NOrders < 1 {
		return ErrInvalidNOrders
	}

	// Валидация стоп-лосса (≥ 0, может быть 0 - не установлен)
	if cfg.StopLoss < 0 {
		return ErrInvalidStopLoss
	}

	return nil
}

// checkSymbolAvailability проверяет доступность символа на подключенных биржах
// Возвращает список бирж, где символ доступен
func (s *PairService) checkSymbolAvailability(ctx context.Context, symbol string) ([]string, error) {
	// Получаем подключенные биржи
	connected, err := s.exchangeRepo.GetConnected()
	if err != nil {
		return nil, err
	}

	if len(connected) < 2 {
		return nil, ErrNotEnoughExchanges
	}

	var available []string

	// Проверяем каждую биржу
	for _, account := range connected {
		// Получаем соединение
		conn, err := s.exchangeSvc.GetConnection(ctx, account.Name)
		if err != nil {
			continue // Пропускаем биржу с ошибкой соединения
		}

		// Пробуем получить тикер для проверки доступности символа
		_, err = conn.GetTicker(ctx, symbol)
		if err == nil {
			available = append(available, account.Name)
		}
	}

	return available, nil
}

// hasOpenPosition проверяет, есть ли открытая позиция у пары
func (s *PairService) hasOpenPosition(id int) bool {
	if s.engine == nil {
		return false
	}

	// Используем метод движка
	if checker, ok := s.engine.(interface{ HasOpenPosition(int) bool }); ok {
		return checker.HasOpenPosition(id)
	}

	// Fallback: проверяем через runtime
	runtime := s.engine.GetPairRuntime(id)
	if runtime == nil {
		return false
	}

	// Позиция открыта в состояниях HOLDING или EXITING
	return runtime.State == models.StateHolding || runtime.State == models.StateExiting
}

// setPendingConfig сохраняет отложенные изменения
func (s *PairService) setPendingConfig(id int, cfg *PendingConfig) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	s.pendingChanges[id] = cfg
}

// clearPendingConfig очищает отложенные изменения
func (s *PairService) clearPendingConfig(id int) {
	s.pendingMu.Lock()
	defer s.pendingMu.Unlock()
	delete(s.pendingChanges, id)
}

// GetSymbolAvailability возвращает список бирж, на которых доступен символ
// Публичный метод для использования в handlers
func (s *PairService) GetSymbolAvailability(ctx context.Context, symbol string) ([]string, error) {
	return s.checkSymbolAvailability(ctx, strings.ToUpper(symbol))
}

// RecordTradeCompletion записывает завершение сделки и обновляет статистику пары
func (s *PairService) RecordTradeCompletion(ctx context.Context, id int, pnl float64) error {
	// Увеличиваем счетчик сделок
	if err := s.pairRepo.IncrementTrades(id); err != nil {
		return err
	}

	// Добавляем PNL
	if err := s.pairRepo.UpdatePnl(id, pnl); err != nil {
		return err
	}

	// Применяем отложенные изменения (если есть)
	return s.ApplyPendingConfig(ctx, id)
}

// ResetPairStats сбрасывает локальную статистику пары
func (s *PairService) ResetPairStats(ctx context.Context, id int) error {
	return s.pairRepo.ResetStats(id)
}
