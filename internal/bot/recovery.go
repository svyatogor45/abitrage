package bot

import (
	"context"
	"fmt"
	"sync"
	"time"

	"arbitrage/internal/config"
	"arbitrage/internal/exchange"
	"arbitrage/internal/models"
	"arbitrage/internal/repository"
	"arbitrage/pkg/crypto"
)

// RecoveryManager отвечает за восстановление работы бота после перезапуска сервера.
//
// Функциональность:
// - Чтение конфигурации пар из БД при старте
// - Восстановление подключений к биржам (дешифровка API ключей)
// - Обнаружение открытых позиций на биржах через API
// - Попытка идентификации позиций, принадлежащих боту
// - Восстановление runtime состояния пар
// - Уведомление пользователя о найденных позициях
// - Опциональное автоматическое закрытие "потерянных" позиций
// - Продолжение мониторинга активных пар
type RecoveryManager struct {
	cfg *config.Config

	// Репозитории для доступа к БД
	exchangeRepo *repository.ExchangeRepository
	pairRepo     *repository.PairRepository

	// Ключ шифрования для API ключей
	encryptionKey []byte

	// Обработчик уведомлений
	notificationChan chan<- *models.Notification

	// Engine для добавления бирж и пар
	engine *Engine

	// Настройки восстановления
	autoCloseOrphaned bool  // Автоматически закрывать "потерянные" позиции
	recoveryTimeout   time.Duration
}

// RecoveryConfig - конфигурация для RecoveryManager
type RecoveryConfig struct {
	// AutoCloseOrphaned - автоматически закрывать позиции, не принадлежащие ни одной паре
	AutoCloseOrphaned bool

	// RecoveryTimeout - таймаут на операции восстановления
	RecoveryTimeout time.Duration
}

// DefaultRecoveryConfig возвращает конфигурацию по умолчанию
func DefaultRecoveryConfig() *RecoveryConfig {
	return &RecoveryConfig{
		AutoCloseOrphaned: false, // Безопасное значение - не закрывать автоматически
		RecoveryTimeout:   30 * time.Second,
	}
}

// NewRecoveryManager создает новый менеджер восстановления
func NewRecoveryManager(
	cfg *config.Config,
	exchangeRepo *repository.ExchangeRepository,
	pairRepo *repository.PairRepository,
	engine *Engine,
	notificationChan chan<- *models.Notification,
	recoveryConfig *RecoveryConfig,
) *RecoveryManager {
	if recoveryConfig == nil {
		recoveryConfig = DefaultRecoveryConfig()
	}

	return &RecoveryManager{
		cfg:               cfg,
		exchangeRepo:      exchangeRepo,
		pairRepo:          pairRepo,
		engine:            engine,
		encryptionKey:     []byte(cfg.Security.EncryptionKey),
		notificationChan:  notificationChan,
		autoCloseOrphaned: recoveryConfig.AutoCloseOrphaned,
		recoveryTimeout:   recoveryConfig.RecoveryTimeout,
	}
}

// RecoveryResult содержит результаты процесса восстановления
type RecoveryResult struct {
	// ExchangesRestored - количество успешно восстановленных подключений к биржам
	ExchangesRestored int

	// ExchangesFailed - биржи с ошибками подключения
	ExchangesFailed map[string]error

	// PairsLoaded - количество загруженных пар
	PairsLoaded int

	// PairsActivated - количество активированных пар (были в статусе active)
	PairsActivated int

	// OpenPositionsFound - найденные открытые позиции на биржах
	OpenPositionsFound []*DiscoveredPosition

	// MatchedPositions - позиции, которые удалось связать с парами бота
	MatchedPositions []*MatchedPosition

	// OrphanedPositions - "потерянные" позиции (не связаны с парами бота)
	OrphanedPositions []*DiscoveredPosition

	// ClosedOrphaned - закрытые "потерянные" позиции (если autoCloseOrphaned=true)
	ClosedOrphaned int

	// Errors - ошибки в процессе восстановления
	Errors []error
}

// DiscoveredPosition представляет найденную позицию на бирже
type DiscoveredPosition struct {
	Exchange      string
	Symbol        string
	Side          string  // "long" или "short"
	Size          float64
	EntryPrice    float64
	UnrealizedPnl float64
}

// MatchedPosition представляет позицию, связанную с парой бота
type MatchedPosition struct {
	PairID       int
	PairSymbol   string
	LongLeg      *DiscoveredPosition
	ShortLeg     *DiscoveredPosition
	TotalPnl     float64
	IsComplete   bool // обе ноги найдены
}

// Recover выполняет полный процесс восстановления
//
// Шаги:
// 1. Загрузка и подключение бирж из БД
// 2. Загрузка торговых пар из БД
// 3. Обнаружение открытых позиций на биржах
// 4. Сопоставление позиций с парами бота
// 5. Восстановление runtime состояния для найденных пар
// 6. Уведомление пользователя
// 7. Обработка "потерянных" позиций
// 8. Активация мониторинга
func (rm *RecoveryManager) Recover(ctx context.Context) (*RecoveryResult, error) {
	result := &RecoveryResult{
		ExchangesFailed:    make(map[string]error),
		OpenPositionsFound: make([]*DiscoveredPosition, 0),
		MatchedPositions:   make([]*MatchedPosition, 0),
		OrphanedPositions:  make([]*DiscoveredPosition, 0),
		Errors:             make([]error, 0),
	}

	ctx, cancel := context.WithTimeout(ctx, rm.recoveryTimeout)
	defer cancel()

	// Шаг 1: Загрузка и подключение бирж
	exchanges, err := rm.restoreExchanges(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to restore exchanges: %w", err))
		return result, err
	}
	result.ExchangesRestored = len(exchanges)

	// Если нет подключенных бирж - выходим
	if len(exchanges) == 0 {
		rm.notify("RECOVERY", "info", "No connected exchanges found during recovery", nil)
		return result, nil
	}

	// Шаг 2: Загрузка торговых пар
	pairs, err := rm.loadPairs(ctx)
	if err != nil {
		result.Errors = append(result.Errors, fmt.Errorf("failed to load pairs: %w", err))
		return result, err
	}
	result.PairsLoaded = len(pairs)

	// Шаг 3: Обнаружение открытых позиций на биржах
	positions := rm.discoverOpenPositions(ctx, exchanges)
	result.OpenPositionsFound = positions

	// Если позиций нет - просто активируем мониторинг активных пар
	if len(positions) == 0 {
		result.PairsActivated = rm.activateMonitoring(pairs)
		rm.notify("RECOVERY", "info",
			fmt.Sprintf("Recovery complete: %d exchanges, %d pairs loaded, %d activated",
				result.ExchangesRestored, result.PairsLoaded, result.PairsActivated), nil)
		return result, nil
	}

	// Шаг 4: Сопоставление позиций с парами бота
	matched, orphaned := rm.matchPositionsToPairs(positions, pairs)
	result.MatchedPositions = matched
	result.OrphanedPositions = orphaned

	// Шаг 5: Восстановление runtime состояния для найденных пар
	rm.restoreRuntimeState(matched)

	// Шаг 6: Уведомление пользователя о найденных позициях
	rm.notifyAboutPositions(result)

	// Шаг 7: Обработка "потерянных" позиций
	if len(orphaned) > 0 && rm.autoCloseOrphaned {
		closed, errs := rm.closeOrphanedPositions(ctx, orphaned, exchanges)
		result.ClosedOrphaned = closed
		result.Errors = append(result.Errors, errs...)
	}

	// Шаг 8: Активация мониторинга (с учётом восстановленных состояний)
	result.PairsActivated = rm.activateMonitoring(pairs)

	return result, nil
}

// restoreExchanges загружает и подключает все биржи из БД
func (rm *RecoveryManager) restoreExchanges(ctx context.Context) (map[string]exchange.Exchange, error) {
	// Получаем все подключенные биржи из БД
	accounts, err := rm.exchangeRepo.GetConnected()
	if err != nil {
		return nil, fmt.Errorf("failed to get connected exchanges: %w", err)
	}

	exchanges := make(map[string]exchange.Exchange)
	var mu sync.Mutex
	var wg sync.WaitGroup
	errChan := make(chan error, len(accounts))

	for _, account := range accounts {
		wg.Add(1)
		go func(acc *models.ExchangeAccount) {
			defer wg.Done()

			exch, err := rm.connectExchange(ctx, acc)
			if err != nil {
				errChan <- fmt.Errorf("exchange %s: %w", acc.Name, err)
				return
			}

			mu.Lock()
			exchanges[acc.Name] = exch
			// Добавляем биржу в engine
			rm.engine.AddExchange(acc.Name, exch)
			mu.Unlock()
		}(account)
	}

	wg.Wait()
	close(errChan)

	// Собираем ошибки (не фатальные)
	for err := range errChan {
		rm.notify("RECOVERY", "warn", fmt.Sprintf("Exchange connection error: %v", err), nil)
	}

	return exchanges, nil
}

// connectExchange подключается к бирже с расшифровкой API ключей
func (rm *RecoveryManager) connectExchange(ctx context.Context, account *models.ExchangeAccount) (exchange.Exchange, error) {
	// Создаём экземпляр биржи через фабрику
	exch, err := exchange.NewExchange(account.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to create exchange instance: %w", err)
	}

	// Расшифровываем API ключ
	apiKey, err := crypto.Decrypt(account.APIKey, rm.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt API key: %w", err)
	}

	// Расшифровываем Secret ключ
	secretKey, err := crypto.Decrypt(account.SecretKey, rm.encryptionKey)
	if err != nil {
		return nil, fmt.Errorf("failed to decrypt secret key: %w", err)
	}

	// Расшифровываем Passphrase (если есть)
	var passphrase string
	if account.Passphrase != "" {
		passphrase, err = crypto.Decrypt(account.Passphrase, rm.encryptionKey)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt passphrase: %w", err)
		}
	}

	// Подключаемся к бирже
	if err := exch.Connect(apiKey, secretKey, passphrase); err != nil {
		return nil, fmt.Errorf("failed to connect: %w", err)
	}

	return exch, nil
}

// loadPairs загружает все торговые пары из БД
func (rm *RecoveryManager) loadPairs(ctx context.Context) ([]*models.PairConfig, error) {
	pairs, err := rm.pairRepo.GetAll()
	if err != nil {
		return nil, fmt.Errorf("failed to get pairs: %w", err)
	}

	// Добавляем пары в engine
	for _, pair := range pairs {
		rm.engine.AddPair(pair)
	}

	return pairs, nil
}

// discoverOpenPositions обнаруживает открытые позиции на всех биржах
func (rm *RecoveryManager) discoverOpenPositions(ctx context.Context, exchanges map[string]exchange.Exchange) []*DiscoveredPosition {
	var positions []*DiscoveredPosition
	var mu sync.Mutex
	var wg sync.WaitGroup

	for name, exch := range exchanges {
		wg.Add(1)
		go func(exchName string, ex exchange.Exchange) {
			defer wg.Done()

			openPositions, err := ex.GetOpenPositions(ctx)
			if err != nil {
				rm.notify("RECOVERY", "warn",
					fmt.Sprintf("Failed to get positions from %s: %v", exchName, err), nil)
				return
			}

			mu.Lock()
			for _, pos := range openPositions {
				if pos.Size > 0 { // Игнорируем нулевые позиции
					positions = append(positions, &DiscoveredPosition{
						Exchange:      exchName,
						Symbol:        pos.Symbol,
						Side:          pos.Side,
						Size:          pos.Size,
						EntryPrice:    pos.EntryPrice,
						UnrealizedPnl: pos.UnrealizedPnl,
					})
				}
			}
			mu.Unlock()
		}(name, exch)
	}

	wg.Wait()
	return positions
}

// matchPositionsToPairs сопоставляет найденные позиции с парами бота
func (rm *RecoveryManager) matchPositionsToPairs(
	positions []*DiscoveredPosition,
	pairs []*models.PairConfig,
) ([]*MatchedPosition, []*DiscoveredPosition) {
	var matched []*MatchedPosition
	var orphaned []*DiscoveredPosition

	// Создаём индекс позиций по символу
	positionsBySymbol := make(map[string][]*DiscoveredPosition)
	for _, pos := range positions {
		positionsBySymbol[pos.Symbol] = append(positionsBySymbol[pos.Symbol], pos)
	}

	// Отмечаем использованные позиции
	usedPositions := make(map[*DiscoveredPosition]bool)

	// Для каждой пары ищем matching позиции
	for _, pair := range pairs {
		symbolPositions := positionsBySymbol[pair.Symbol]
		if len(symbolPositions) == 0 {
			continue
		}

		// Ищем long и short ноги
		var longLeg, shortLeg *DiscoveredPosition
		for _, pos := range symbolPositions {
			if usedPositions[pos] {
				continue
			}
			if pos.Side == exchange.SideLong && longLeg == nil {
				longLeg = pos
			} else if pos.Side == exchange.SideShort && shortLeg == nil {
				shortLeg = pos
			}
		}

		// Если найдена хотя бы одна нога
		if longLeg != nil || shortLeg != nil {
			mp := &MatchedPosition{
				PairID:     pair.ID,
				PairSymbol: pair.Symbol,
				LongLeg:    longLeg,
				ShortLeg:   shortLeg,
				IsComplete: longLeg != nil && shortLeg != nil,
			}

			// Рассчитываем суммарный PNL
			if longLeg != nil {
				mp.TotalPnl += longLeg.UnrealizedPnl
				usedPositions[longLeg] = true
			}
			if shortLeg != nil {
				mp.TotalPnl += shortLeg.UnrealizedPnl
				usedPositions[shortLeg] = true
			}

			matched = append(matched, mp)
		}
	}

	// Все неиспользованные позиции - orphaned
	for _, pos := range positions {
		if !usedPositions[pos] {
			orphaned = append(orphaned, pos)
		}
	}

	return matched, orphaned
}

// restoreRuntimeState восстанавливает runtime состояние для найденных пар
func (rm *RecoveryManager) restoreRuntimeState(matched []*MatchedPosition) {
	for _, mp := range matched {
		if !mp.IsComplete {
			// Неполная позиция - отмечаем как ERROR для внимания пользователя
			rm.engine.pairsMu.RLock()
			ps, ok := rm.engine.pairs[mp.PairID]
			rm.engine.pairsMu.RUnlock()

			if ok {
				ps.mu.Lock()
				ps.Runtime.State = models.StateError
				ps.Config.Status = models.PairStatusPaused
				ps.mu.Unlock()
			}
			continue
		}

		// Полная позиция - восстанавливаем состояние HOLDING
		rm.engine.pairsMu.RLock()
		ps, ok := rm.engine.pairs[mp.PairID]
		rm.engine.pairsMu.RUnlock()

		if !ok {
			continue
		}

		ps.mu.Lock()

		// Создаём ноги
		legs := make([]models.Leg, 0, 2)

		if mp.LongLeg != nil {
			legs = append(legs, models.Leg{
				Exchange:      mp.LongLeg.Exchange,
				Side:          "long",
				EntryPrice:    mp.LongLeg.EntryPrice,
				CurrentPrice:  mp.LongLeg.EntryPrice, // будет обновлено
				Quantity:      mp.LongLeg.Size,
				UnrealizedPnl: mp.LongLeg.UnrealizedPnl,
			})
		}

		if mp.ShortLeg != nil {
			legs = append(legs, models.Leg{
				Exchange:      mp.ShortLeg.Exchange,
				Side:          "short",
				EntryPrice:    mp.ShortLeg.EntryPrice,
				CurrentPrice:  mp.ShortLeg.EntryPrice, // будет обновлено
				Quantity:      mp.ShortLeg.Size,
				UnrealizedPnl: mp.ShortLeg.UnrealizedPnl,
			})
		}

		ps.Runtime.State = models.StateHolding
		ps.Runtime.Legs = legs
		ps.Runtime.UnrealizedPnl = mp.TotalPnl
		ps.Runtime.LastUpdate = time.Now()
		ps.Config.Status = models.PairStatusActive

		// Инкрементируем счётчик активных арбитражей
		rm.engine.incrementActiveArbs()

		ps.mu.Unlock()
	}
}

// notifyAboutPositions отправляет уведомления о найденных позициях
func (rm *RecoveryManager) notifyAboutPositions(result *RecoveryResult) {
	// Уведомление о восстановленных позициях
	if len(result.MatchedPositions) > 0 {
		for _, mp := range result.MatchedPositions {
			status := "complete"
			if !mp.IsComplete {
				status = "incomplete (requires attention)"
			}

			rm.notify("RECOVERY", "warn", fmt.Sprintf(
				"Recovered position for %s: %s, PNL: %.2f USDT",
				mp.PairSymbol, status, mp.TotalPnl,
			), map[string]interface{}{
				"pair_id":     mp.PairID,
				"symbol":      mp.PairSymbol,
				"is_complete": mp.IsComplete,
				"total_pnl":   mp.TotalPnl,
			})
		}
	}

	// Уведомление о "потерянных" позициях
	if len(result.OrphanedPositions) > 0 {
		for _, pos := range result.OrphanedPositions {
			rm.notify("RECOVERY", "error", fmt.Sprintf(
				"Orphaned position found: %s %s on %s (size: %.4f, PNL: %.2f USDT)",
				pos.Side, pos.Symbol, pos.Exchange, pos.Size, pos.UnrealizedPnl,
			), map[string]interface{}{
				"exchange":       pos.Exchange,
				"symbol":         pos.Symbol,
				"side":           pos.Side,
				"size":           pos.Size,
				"unrealized_pnl": pos.UnrealizedPnl,
			})
		}
	}

	// Общая сводка
	rm.notify("RECOVERY", "info", fmt.Sprintf(
		"Recovery summary: %d exchanges, %d pairs, %d matched positions, %d orphaned positions",
		result.ExchangesRestored, result.PairsLoaded,
		len(result.MatchedPositions), len(result.OrphanedPositions),
	), nil)
}

// closeOrphanedPositions закрывает "потерянные" позиции
func (rm *RecoveryManager) closeOrphanedPositions(
	ctx context.Context,
	orphaned []*DiscoveredPosition,
	exchanges map[string]exchange.Exchange,
) (int, []error) {
	var closed int
	var errors []error

	for _, pos := range orphaned {
		exch, ok := exchanges[pos.Exchange]
		if !ok {
			errors = append(errors, fmt.Errorf("exchange %s not found for closing position", pos.Exchange))
			continue
		}

		err := exch.ClosePosition(ctx, pos.Symbol, pos.Side, pos.Size)
		if err != nil {
			errors = append(errors, fmt.Errorf("failed to close %s %s on %s: %w",
				pos.Side, pos.Symbol, pos.Exchange, err))
			rm.notify("RECOVERY", "error", fmt.Sprintf(
				"Failed to close orphaned position: %s %s on %s - %v",
				pos.Side, pos.Symbol, pos.Exchange, err,
			), nil)
		} else {
			closed++
			rm.notify("RECOVERY", "warn", fmt.Sprintf(
				"Closed orphaned position: %s %s on %s (size: %.4f)",
				pos.Side, pos.Symbol, pos.Exchange, pos.Size,
			), nil)
		}
	}

	return closed, errors
}

// activateMonitoring активирует мониторинг для активных пар
func (rm *RecoveryManager) activateMonitoring(pairs []*models.PairConfig) int {
	activated := 0

	for _, pair := range pairs {
		// Пропускаем пары, у которых уже восстановлено состояние HOLDING или ERROR
		rm.engine.pairsMu.RLock()
		ps, ok := rm.engine.pairs[pair.ID]
		rm.engine.pairsMu.RUnlock()

		if !ok {
			continue
		}

		ps.mu.RLock()
		currentState := ps.Runtime.State
		ps.mu.RUnlock()

		// Если пара уже в HOLDING или ERROR - не трогаем
		if currentState == models.StateHolding || currentState == models.StateError {
			if currentState == models.StateHolding {
				activated++ // Считаем как активированную
			}
			continue
		}

		// Активируем пару если она была в статусе active
		if pair.Status == models.PairStatusActive {
			err := rm.engine.StartPair(pair.ID)
			if err == nil {
				activated++
			}
		}
	}

	return activated
}

// notify отправляет уведомление
func (rm *RecoveryManager) notify(notifType, severity, message string, meta map[string]interface{}) {
	if rm.notificationChan == nil {
		return
	}

	notif := &models.Notification{
		Timestamp: time.Now(),
		Type:      notifType,
		Severity:  severity,
		Message:   message,
		Meta:      meta,
	}

	select {
	case rm.notificationChan <- notif:
	default:
		// Канал заполнен, пропускаем
	}
}

// RecoverAsync запускает восстановление асинхронно и возвращает канал с результатом
func (rm *RecoveryManager) RecoverAsync(ctx context.Context) <-chan *RecoveryResult {
	resultChan := make(chan *RecoveryResult, 1)

	go func() {
		defer close(resultChan)
		result, _ := rm.Recover(ctx)
		resultChan <- result
	}()

	return resultChan
}

// GetRecoveryStatus возвращает текущий статус восстановления (для API)
type RecoveryStatus struct {
	InProgress         bool                `json:"in_progress"`
	LastRecoveryTime   *time.Time          `json:"last_recovery_time,omitempty"`
	ExchangesConnected int                 `json:"exchanges_connected"`
	PairsLoaded        int                 `json:"pairs_loaded"`
	OpenPositions      int                 `json:"open_positions"`
	OrphanedPositions  int                 `json:"orphaned_positions"`
	Errors             []string            `json:"errors,omitempty"`
}

// VerifyPositions проверяет соответствие позиций в engine с позициями на биржах
// Может использоваться для периодической проверки согласованности
func (rm *RecoveryManager) VerifyPositions(ctx context.Context) ([]string, error) {
	var inconsistencies []string

	// Получаем все пары в состоянии HOLDING
	rm.engine.pairsMu.RLock()
	holdingPairs := make([]*PairState, 0)
	for _, ps := range rm.engine.pairs {
		if ps.Runtime.State == models.StateHolding {
			holdingPairs = append(holdingPairs, ps)
		}
	}
	rm.engine.pairsMu.RUnlock()

	// Получаем текущие подключенные биржи
	rm.engine.exchMu.RLock()
	exchanges := make(map[string]exchange.Exchange, len(rm.engine.exchanges))
	for name, exch := range rm.engine.exchanges {
		exchanges[name] = exch
	}
	rm.engine.exchMu.RUnlock()

	// Получаем позиции с бирж
	positions := rm.discoverOpenPositions(ctx, exchanges)

	// Создаём индекс позиций для быстрого поиска
	type posKey struct {
		exchange string
		symbol   string
		side     string
	}
	posIndex := make(map[posKey]*DiscoveredPosition)
	for _, pos := range positions {
		key := posKey{pos.Exchange, pos.Symbol, pos.Side}
		posIndex[key] = pos
	}

	// Проверяем каждую пару в HOLDING
	for _, ps := range holdingPairs {
		ps.mu.RLock()
		for _, leg := range ps.Runtime.Legs {
			key := posKey{leg.Exchange, ps.Config.Symbol, leg.Side}
			pos, exists := posIndex[key]

			if !exists {
				inconsistencies = append(inconsistencies, fmt.Sprintf(
					"Pair %s: leg %s on %s not found on exchange",
					ps.Config.Symbol, leg.Side, leg.Exchange,
				))
			} else if pos.Size != leg.Quantity {
				inconsistencies = append(inconsistencies, fmt.Sprintf(
					"Pair %s: leg %s on %s size mismatch (engine: %.4f, exchange: %.4f)",
					ps.Config.Symbol, leg.Side, leg.Exchange, leg.Quantity, pos.Size,
				))
			}
		}
		ps.mu.RUnlock()
	}

	return inconsistencies, nil
}

// ClosePositionManually позволяет вручную закрыть позицию (для API)
func (rm *RecoveryManager) ClosePositionManually(
	ctx context.Context,
	exchangeName, symbol, side string,
	quantity float64,
) error {
	rm.engine.exchMu.RLock()
	exch, ok := rm.engine.exchanges[exchangeName]
	rm.engine.exchMu.RUnlock()

	if !ok {
		return fmt.Errorf("exchange %s not connected", exchangeName)
	}

	err := exch.ClosePosition(ctx, symbol, side, quantity)
	if err != nil {
		return fmt.Errorf("failed to close position: %w", err)
	}

	rm.notify("RECOVERY", "info", fmt.Sprintf(
		"Manually closed position: %s %s on %s (size: %.4f)",
		side, symbol, exchangeName, quantity,
	), nil)

	return nil
}
