package service

import (
	"context"
	"errors"
	"strings"
	"sync"
	"time"

	"arbitrage/internal/exchange"
	"arbitrage/internal/models"
	"arbitrage/internal/repository"
	"arbitrage/pkg/crypto"
)

// Ошибки сервиса
var (
	ErrExchangeNotSupported    = errors.New("exchange is not supported")
	ErrExchangeAlreadyConnected = errors.New("exchange is already connected")
	ErrExchangeNotConnected    = errors.New("exchange is not connected")
	ErrInvalidCredentials      = errors.New("invalid API credentials")
	ErrConnectionFailed        = errors.New("failed to connect to exchange")
	ErrHasActivePositions      = errors.New("cannot disconnect: exchange has active positions")
)

// BalanceBroadcaster - интерфейс для отправки обновлений балансов через WebSocket
type BalanceBroadcaster interface {
	BroadcastBalanceUpdate(exchange string, balance float64)
	BroadcastAllBalances(balances map[string]float64)
}

// ExchangeService - бизнес-логика для управления биржами
type ExchangeService struct {
	exchangeRepo  *repository.ExchangeRepository
	pairRepo      *repository.PairRepository
	encryptionKey []byte

	// Кэш активных соединений с биржами
	connections   map[string]exchange.Exchange
	connectionsMu sync.RWMutex // Защита от race condition при конкурентном доступе

	// WebSocket hub для broadcast балансов
	wsHub BalanceBroadcaster
}

// NewExchangeService создает новый экземпляр сервиса
func NewExchangeService(
	exchangeRepo *repository.ExchangeRepository,
	pairRepo *repository.PairRepository,
	encryptionKey string,
) *ExchangeService {
	return &ExchangeService{
		exchangeRepo:  exchangeRepo,
		pairRepo:      pairRepo,
		encryptionKey: []byte(encryptionKey),
		connections:   make(map[string]exchange.Exchange),
	}
}

// SetWebSocketHub устанавливает WebSocket hub для broadcast балансов.
//
// Вызывается после инициализации Hub в main.go:
//
//	exchangeService := service.NewExchangeService(...)
//	exchangeService.SetWebSocketHub(wsHub)
func (s *ExchangeService) SetWebSocketHub(hub BalanceBroadcaster) {
	s.wsHub = hub
}

// ConnectExchange подключает биржу с указанными API ключами
// Выполняет:
// 1. Проверку поддержки биржи
// 2. Тестовое подключение (проверка ключей)
// 3. Шифрование ключей перед сохранением
// 4. Сохранение в БД
func (s *ExchangeService) ConnectExchange(ctx context.Context, name, apiKey, secretKey, passphrase string) error {
	name = strings.ToLower(name)

	// 1. Проверяем, поддерживается ли биржа
	if !exchange.IsSupported(name) {
		return ErrExchangeNotSupported
	}

	// 2. Проверяем, не подключена ли уже биржа
	existing, err := s.exchangeRepo.GetByName(name)
	if err == nil && existing.Connected {
		return ErrExchangeAlreadyConnected
	}

	// 3. Создаем экземпляр биржи через фабрику
	exch, err := exchange.NewExchange(name)
	if err != nil {
		return err
	}

	// 4. Тестовое подключение (проверяем валидность ключей)
	if err := exch.Connect(apiKey, secretKey, passphrase); err != nil {
		return errors.Join(ErrInvalidCredentials, err)
	}

	// 5. Получаем баланс для проверки работоспособности
	balance, err := exch.GetBalance(ctx)
	if err != nil {
		// Закрываем соединение при ошибке
		_ = exch.Close()
		return errors.Join(ErrConnectionFailed, err)
	}

	// 6. Шифруем API ключи перед сохранением
	encryptedAPIKey, err := crypto.Encrypt(apiKey, s.encryptionKey)
	if err != nil {
		_ = exch.Close()
		return err
	}

	encryptedSecretKey, err := crypto.Encrypt(secretKey, s.encryptionKey)
	if err != nil {
		_ = exch.Close()
		return err
	}

	var encryptedPassphrase string
	if passphrase != "" {
		encryptedPassphrase, err = crypto.Encrypt(passphrase, s.encryptionKey)
		if err != nil {
			_ = exch.Close()
			return err
		}
	}

	// 7. Сохраняем или обновляем в БД
	if existing != nil {
		// Обновляем существующую запись
		existing.APIKey = encryptedAPIKey
		existing.SecretKey = encryptedSecretKey
		existing.Passphrase = encryptedPassphrase
		existing.Connected = true
		existing.Balance = balance
		existing.LastError = ""
		existing.UpdatedAt = time.Now()

		if err := s.exchangeRepo.Update(existing); err != nil {
			_ = exch.Close()
			return err
		}
	} else {
		// Создаем новую запись
		account := &models.ExchangeAccount{
			Name:       name,
			APIKey:     encryptedAPIKey,
			SecretKey:  encryptedSecretKey,
			Passphrase: encryptedPassphrase,
			Connected:  true,
			Balance:    balance,
			LastError:  "",
		}

		if err := s.exchangeRepo.Create(account); err != nil {
			_ = exch.Close()
			return err
		}
	}

	// 8. Сохраняем соединение в кэше
	s.connectionsMu.Lock()
	s.connections[name] = exch
	s.connectionsMu.Unlock()

	return nil
}

// DisconnectExchange отключает биржу
// Выполняет:
// 1. Проверку наличия подключения
// 2. Остановку всех активных пар (ставит на паузу)
// 3. Удаление ключей из БД
func (s *ExchangeService) DisconnectExchange(ctx context.Context, name string) error {
	name = strings.ToLower(name)

	// 1. Проверяем, подключена ли биржа
	account, err := s.exchangeRepo.GetByName(name)
	if err != nil {
		if errors.Is(err, repository.ErrExchangeNotFound) {
			return ErrExchangeNotConnected
		}
		return err
	}

	if !account.Connected {
		return ErrExchangeNotConnected
	}

	// 2. Получаем все активные пары и ставим их на паузу
	// Пары, которые используют эту биржу, должны быть остановлены
	activePairs, err := s.pairRepo.GetActive()
	if err != nil {
		return err
	}

	// Примечание: в текущей архитектуре пары не привязаны к конкретным биржам,
	// бот сам выбирает биржи на основе спреда. Однако при отключении биржи
	// если осталось меньше 2 подключенных бирж, нужно остановить все пары.
	connectedCount, err := s.exchangeRepo.CountConnected()
	if err != nil {
		return err
	}

	// Если после отключения останется меньше 2 бирж - останавливаем все пары
	if connectedCount <= 2 {
		for _, pair := range activePairs {
			if err := s.pairRepo.UpdateStatus(pair.ID, models.PairStatusPaused); err != nil {
				// Логируем ошибку, но продолжаем
				continue
			}
		}
	}

	// 3. Закрываем соединение с биржей (если есть в кэше)
	s.connectionsMu.Lock()
	if conn, exists := s.connections[name]; exists {
		_ = conn.Close()
		delete(s.connections, name)
	}
	s.connectionsMu.Unlock()

	// 4. Помечаем биржу как отключенную и очищаем ключи
	account.Connected = false
	account.APIKey = ""
	account.SecretKey = ""
	account.Passphrase = ""
	account.Balance = 0
	account.LastError = ""
	account.UpdatedAt = time.Now()

	return s.exchangeRepo.Update(account)
}

// UpdateBalance обновляет баланс биржи
// Запрашивает актуальный баланс через API биржи
// После успешного обновления отправляет broadcast через WebSocket
func (s *ExchangeService) UpdateBalance(ctx context.Context, name string) (float64, error) {
	name = strings.ToLower(name)

	// 1. Получаем данные биржи из БД
	account, err := s.exchangeRepo.GetByName(name)
	if err != nil {
		if errors.Is(err, repository.ErrExchangeNotFound) {
			return 0, ErrExchangeNotConnected
		}
		return 0, err
	}

	if !account.Connected {
		return 0, ErrExchangeNotConnected
	}

	// 2. Проверяем наличие соединения в кэше или создаем новое
	conn, err := s.getOrCreateConnection(ctx, name, account)
	if err != nil {
		// Записываем ошибку в БД
		_ = s.exchangeRepo.SetLastError(account.ID, err.Error())
		return 0, err
	}

	// 3. Запрашиваем баланс
	balance, err := conn.GetBalance(ctx)
	if err != nil {
		_ = s.exchangeRepo.SetLastError(account.ID, err.Error())
		return 0, err
	}

	// 4. Обновляем баланс в БД
	if err := s.exchangeRepo.UpdateBalance(account.ID, balance); err != nil {
		return balance, err
	}

	// 5. Очищаем ошибку если была
	if account.LastError != "" {
		_ = s.exchangeRepo.SetLastError(account.ID, "")
	}

	// 6. Broadcast через WebSocket для real-time обновления UI
	if s.wsHub != nil {
		s.wsHub.BroadcastBalanceUpdate(name, balance)
	}

	return balance, nil
}

// GetAllExchanges возвращает список всех бирж с их статусами
// Для каждой поддерживаемой биржи возвращает информацию о подключении
func (s *ExchangeService) GetAllExchanges() ([]*models.ExchangeAccount, error) {
	// Получаем все биржи из БД
	dbExchanges, err := s.exchangeRepo.GetAll()
	if err != nil {
		return nil, err
	}

	// Создаем map для быстрого поиска
	dbMap := make(map[string]*models.ExchangeAccount)
	for _, ex := range dbExchanges {
		dbMap[ex.Name] = ex
	}

	// Формируем полный список (включая неподключенные биржи)
	result := make([]*models.ExchangeAccount, 0, len(exchange.SupportedExchanges))

	for _, name := range exchange.SupportedExchanges {
		if dbAccount, exists := dbMap[name]; exists {
			// Биржа есть в БД - очищаем ключи перед отправкой
			safeCopy := &models.ExchangeAccount{
				ID:        dbAccount.ID,
				Name:      dbAccount.Name,
				Connected: dbAccount.Connected,
				Balance:   dbAccount.Balance,
				LastError: dbAccount.LastError,
				UpdatedAt: dbAccount.UpdatedAt,
				CreatedAt: dbAccount.CreatedAt,
				// APIKey, SecretKey, Passphrase не возвращаем!
			}
			result = append(result, safeCopy)
		} else {
			// Биржа не в БД - возвращаем пустую запись
			result = append(result, &models.ExchangeAccount{
				Name:      name,
				Connected: false,
				Balance:   0,
			})
		}
	}

	return result, nil
}

// GetConnectedExchanges возвращает только подключенные биржи
func (s *ExchangeService) GetConnectedExchanges() ([]*models.ExchangeAccount, error) {
	return s.exchangeRepo.GetConnected()
}

// GetExchangeByName возвращает биржу по имени
func (s *ExchangeService) GetExchangeByName(name string) (*models.ExchangeAccount, error) {
	name = strings.ToLower(name)
	account, err := s.exchangeRepo.GetByName(name)
	if err != nil {
		return nil, err
	}

	// Очищаем ключи перед возвратом
	return &models.ExchangeAccount{
		ID:        account.ID,
		Name:      account.Name,
		Connected: account.Connected,
		Balance:   account.Balance,
		LastError: account.LastError,
		UpdatedAt: account.UpdatedAt,
		CreatedAt: account.CreatedAt,
	}, nil
}

// GetConnection возвращает активное соединение с биржей
// Используется торговым движком для выполнения операций
func (s *ExchangeService) GetConnection(ctx context.Context, name string) (exchange.Exchange, error) {
	name = strings.ToLower(name)

	// Проверяем кэш (read lock)
	s.connectionsMu.RLock()
	conn, exists := s.connections[name]
	s.connectionsMu.RUnlock()

	if exists {
		return conn, nil
	}

	// Получаем данные из БД и создаем соединение
	account, err := s.exchangeRepo.GetByName(name)
	if err != nil {
		return nil, err
	}

	if !account.Connected {
		return nil, ErrExchangeNotConnected
	}

	return s.getOrCreateConnection(ctx, name, account)
}

// UpdateAllBalances обновляет балансы всех подключенных бирж
// Вызывается периодически (каждую минуту) из торгового движка
// После обновления отправляет broadcast всех балансов через WebSocket
func (s *ExchangeService) UpdateAllBalances(ctx context.Context) map[string]float64 {
	result := make(map[string]float64)

	connected, err := s.exchangeRepo.GetConnected()
	if err != nil {
		return result
	}

	for _, account := range connected {
		balance, err := s.UpdateBalance(ctx, account.Name)
		if err != nil {
			// Логируем ошибку, но продолжаем
			continue
		}
		result[account.Name] = balance
	}

	// Broadcast всех балансов одним сообщением (для начальной загрузки UI)
	if s.wsHub != nil && len(result) > 0 {
		s.wsHub.BroadcastAllBalances(result)
	}

	return result
}

// getOrCreateConnection получает соединение из кэша или создает новое
func (s *ExchangeService) getOrCreateConnection(ctx context.Context, name string, account *models.ExchangeAccount) (exchange.Exchange, error) {
	// Проверяем кэш (read lock)
	s.connectionsMu.RLock()
	conn, exists := s.connections[name]
	s.connectionsMu.RUnlock()

	if exists {
		return conn, nil
	}

	// Расшифровываем ключи
	apiKey, err := crypto.Decrypt(account.APIKey, s.encryptionKey)
	if err != nil {
		return nil, err
	}

	secretKey, err := crypto.Decrypt(account.SecretKey, s.encryptionKey)
	if err != nil {
		return nil, err
	}

	var passphrase string
	if account.Passphrase != "" {
		passphrase, err = crypto.Decrypt(account.Passphrase, s.encryptionKey)
		if err != nil {
			return nil, err
		}
	}

	// Создаем новое соединение
	conn, err = exchange.NewExchange(name)
	if err != nil {
		return nil, err
	}

	if err := conn.Connect(apiKey, secretKey, passphrase); err != nil {
		return nil, err
	}

	// Сохраняем в кэш (write lock)
	s.connectionsMu.Lock()
	s.connections[name] = conn
	s.connectionsMu.Unlock()

	return conn, nil
}

// Close закрывает все соединения с биржами
// Вызывается при graceful shutdown
func (s *ExchangeService) Close() error {
	s.connectionsMu.Lock()
	defer s.connectionsMu.Unlock()

	for name, conn := range s.connections {
		_ = conn.Close()
		delete(s.connections, name)
	}
	return nil
}

// CountConnected возвращает количество подключенных бирж
func (s *ExchangeService) CountConnected() (int, error) {
	return s.exchangeRepo.CountConnected()
}

// HasMinimumExchanges проверяет, подключено ли минимум 2 биржи
// Необходимо для работы арбитража
func (s *ExchangeService) HasMinimumExchanges() (bool, error) {
	count, err := s.exchangeRepo.CountConnected()
	if err != nil {
		return false, err
	}
	return count >= 2, nil
}
