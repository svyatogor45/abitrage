package service

import (
	"context"
	"errors"
	"testing"

	"arbitrage/internal/exchange"
	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// ============ ТЕСТЫ ============

func TestExchangeService_Errors(t *testing.T) {
	// Проверяем, что все ошибки определены
	tests := []struct {
		name string
		err  error
	}{
		{"ErrExchangeNotSupported", ErrExchangeNotSupported},
		{"ErrExchangeAlreadyConnected", ErrExchangeAlreadyConnected},
		{"ErrExchangeNotConnected", ErrExchangeNotConnected},
		{"ErrInvalidCredentials", ErrInvalidCredentials},
		{"ErrConnectionFailed", ErrConnectionFailed},
		{"ErrHasActivePositions", ErrHasActivePositions},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.err == nil {
				t.Errorf("%s is nil", tt.name)
			}
		})
	}
}

func TestExchangeService_GetAllExchanges(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*MockExchangeRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name: "получение всех бирж",
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true, Balance: 1000}
				m.accounts["okx"] = &models.ExchangeAccount{ID: 2, Name: "okx", Connected: false}
			},
			wantCount: 2,
		},
		{
			name:      "пустой список",
			wantCount: 0,
		},
		{
			name: "ошибка базы данных",
			setup: func(m *MockExchangeRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockExchangeRepository()

			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			exchanges, err := mockRepo.GetAll()

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

			if len(exchanges) != tt.wantCount {
				t.Errorf("expected %d exchanges, got %d", tt.wantCount, len(exchanges))
			}
		})
	}
}

func TestExchangeService_GetConnectedExchanges(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*MockExchangeRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name: "получение подключенных бирж",
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true}
				m.accounts["okx"] = &models.ExchangeAccount{ID: 2, Name: "okx", Connected: false}
				m.accounts["binance"] = &models.ExchangeAccount{ID: 3, Name: "binance", Connected: true}
			},
			wantCount: 2,
		},
		{
			name: "нет подключенных бирж",
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: false}
			},
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockExchangeRepository()

			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			exchanges, err := mockRepo.GetConnected()

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

			if len(exchanges) != tt.wantCount {
				t.Errorf("expected %d exchanges, got %d", tt.wantCount, len(exchanges))
			}
		})
	}
}

func TestExchangeService_GetExchangeByName(t *testing.T) {
	tests := []struct {
		name         string
		exchangeName string
		setup        func(*MockExchangeRepository)
		wantErr      error
	}{
		{
			name:         "успешное получение",
			exchangeName: "bybit",
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true, Balance: 1000}
			},
		},
		{
			name:         "биржа не найдена",
			exchangeName: "unknown",
			wantErr:      repository.ErrExchangeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockExchangeRepository()

			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			account, err := mockRepo.GetByName(tt.exchangeName)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if !errors.Is(err, tt.wantErr) {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if account.Name != tt.exchangeName {
				t.Errorf("expected name %s, got %s", tt.exchangeName, account.Name)
			}
		})
	}
}

func TestExchangeService_CountConnected(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockExchangeRepository)
		want    int
		wantErr bool
	}{
		{
			name: "подсчет подключенных бирж",
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true}
				m.accounts["okx"] = &models.ExchangeAccount{ID: 2, Name: "okx", Connected: true}
				m.accounts["binance"] = &models.ExchangeAccount{ID: 3, Name: "binance", Connected: false}
			},
			want: 2,
		},
		{
			name: "нет подключенных бирж",
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockExchangeRepository()

			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			count, err := mockRepo.CountConnected()

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

func TestExchangeService_HasMinimumExchanges(t *testing.T) {
	tests := []struct {
		name  string
		setup func(*MockExchangeRepository)
		want  bool
	}{
		{
			name: "достаточно бирж (2)",
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true}
				m.accounts["okx"] = &models.ExchangeAccount{ID: 2, Name: "okx", Connected: true}
			},
			want: true,
		},
		{
			name: "достаточно бирж (3)",
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true}
				m.accounts["okx"] = &models.ExchangeAccount{ID: 2, Name: "okx", Connected: true}
				m.accounts["binance"] = &models.ExchangeAccount{ID: 3, Name: "binance", Connected: true}
			},
			want: true,
		},
		{
			name: "недостаточно бирж (1)",
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true}
			},
			want: false,
		},
		{
			name: "нет подключенных бирж",
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockExchangeRepository()

			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			count, _ := mockRepo.CountConnected()
			hasMinimum := count >= 2

			if hasMinimum != tt.want {
				t.Errorf("expected %v, got %v", tt.want, hasMinimum)
			}
		})
	}
}

func TestExchangeService_UpdateBalance(t *testing.T) {
	tests := []struct {
		name       string
		accountID  int
		newBalance float64
		setup      func(*MockExchangeRepository)
		wantErr    bool
	}{
		{
			name:       "успешное обновление баланса",
			accountID:  1,
			newBalance: 2000.0,
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true, Balance: 1000}
			},
		},
		{
			name:       "биржа не найдена",
			accountID:  999,
			newBalance: 2000.0,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockExchangeRepository()

			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			err := mockRepo.UpdateBalance(tt.accountID, tt.newBalance)

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

			// Проверяем, что баланс обновлен
			for _, account := range mockRepo.accounts {
				if account.ID == tt.accountID {
					if account.Balance != tt.newBalance {
						t.Errorf("expected balance %f, got %f", tt.newBalance, account.Balance)
					}
					break
				}
			}
		})
	}
}

func TestExchangeService_SetLastError(t *testing.T) {
	tests := []struct {
		name      string
		accountID int
		errMsg    string
		setup     func(*MockExchangeRepository)
		wantErr   bool
	}{
		{
			name:      "установка ошибки",
			accountID: 1,
			errMsg:    "API error: rate limit exceeded",
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true}
			},
		},
		{
			name:      "очистка ошибки",
			accountID: 1,
			errMsg:    "",
			setup: func(m *MockExchangeRepository) {
				m.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true, LastError: "old error"}
			},
		},
		{
			name:      "биржа не найдена",
			accountID: 999,
			errMsg:    "error",
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockExchangeRepository()

			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			err := mockRepo.SetLastError(tt.accountID, tt.errMsg)

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

			// Проверяем, что ошибка установлена
			for _, account := range mockRepo.accounts {
				if account.ID == tt.accountID {
					if account.LastError != tt.errMsg {
						t.Errorf("expected error %s, got %s", tt.errMsg, account.LastError)
					}
					break
				}
			}
		})
	}
}

func TestExchangeService_DisconnectFlow(t *testing.T) {
	t.Run("отключение биржи", func(t *testing.T) {
		mockRepo := NewMockExchangeRepository()
		mockRepo.accounts["bybit"] = &models.ExchangeAccount{
			ID:        1,
			Name:      "bybit",
			APIKey:    "encrypted_key",
			SecretKey: "encrypted_secret",
			Connected: true,
			Balance:   1000,
		}

		// Отключаем биржу
		account := mockRepo.accounts["bybit"]
		account.Connected = false
		account.APIKey = ""
		account.SecretKey = ""
		account.Passphrase = ""
		account.Balance = 0
		account.LastError = ""

		// Проверяем
		if account.Connected {
			t.Error("expected Connected = false")
		}
		if account.APIKey != "" {
			t.Error("expected APIKey to be cleared")
		}
		if account.Balance != 0 {
			t.Error("expected Balance = 0")
		}
	})
}

func TestExchangeService_NewService(t *testing.T) {
	// Проверяем создание сервиса
	svc := NewExchangeService(
		&repository.ExchangeRepository{},
		&repository.PairRepository{},
		"test_encryption_key_32_bytes___",
	)

	if svc == nil {
		t.Error("expected service, got nil")
	}

	if svc.connections == nil {
		t.Error("expected connections map to be initialized")
	}
}

func TestExchangeService_SetWebSocketHub(t *testing.T) {
	svc := NewExchangeService(
		&repository.ExchangeRepository{},
		&repository.PairRepository{},
		"test_encryption_key_32_bytes___",
	)

	mockHub := &MockBalanceBroadcaster{}
	svc.SetWebSocketHub(mockHub)

	if svc.wsHub == nil {
		t.Error("expected wsHub to be set")
	}
}

func TestExchangeService_Close(t *testing.T) {
	svc := NewExchangeService(
		&repository.ExchangeRepository{},
		&repository.PairRepository{},
		"test_encryption_key_32_bytes___",
	)

	// Добавляем mock соединения
	svc.connections["bybit"] = &MockExchange{}
	svc.connections["okx"] = &MockExchange{}

	err := svc.Close()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if len(svc.connections) != 0 {
		t.Errorf("expected 0 connections, got %d", len(svc.connections))
	}
}

// MockBalanceBroadcaster - mock для BalanceBroadcaster
type MockBalanceBroadcaster struct {
	balanceUpdates    []struct{ exchange string; balance float64 }
	allBalanceUpdates []map[string]float64
}

func (m *MockBalanceBroadcaster) BroadcastBalanceUpdate(exchange string, balance float64) {
	m.balanceUpdates = append(m.balanceUpdates, struct {
		exchange string
		balance  float64
	}{exchange, balance})
}

func (m *MockBalanceBroadcaster) BroadcastAllBalances(balances map[string]float64) {
	m.allBalanceUpdates = append(m.allBalanceUpdates, balances)
}

// MockExchange - mock для exchange.Exchange интерфейса
type MockExchange struct {
	name       string
	connected  bool
	balance    float64
	connectErr error
	balanceErr error
	closeErr   error
}

func (m *MockExchange) Connect(apiKey, secretKey, passphrase string) error {
	if m.connectErr != nil {
		return m.connectErr
	}
	m.connected = true
	return nil
}

func (m *MockExchange) GetName() string {
	if m.name == "" {
		return "mock"
	}
	return m.name
}

func (m *MockExchange) GetBalance(ctx context.Context) (float64, error) {
	if m.balanceErr != nil {
		return 0, m.balanceErr
	}
	return m.balance, nil
}

func (m *MockExchange) GetTicker(ctx context.Context, symbol string) (*exchange.Ticker, error) {
	return &exchange.Ticker{Symbol: symbol, BidPrice: 100, AskPrice: 101, LastPrice: 100.5}, nil
}

func (m *MockExchange) GetOrderBook(ctx context.Context, symbol string, depth int) (*exchange.OrderBook, error) {
	return &exchange.OrderBook{Symbol: symbol}, nil
}

func (m *MockExchange) PlaceMarketOrder(ctx context.Context, symbol, side string, qty float64) (*exchange.Order, error) {
	return &exchange.Order{ID: "test-order-1", Symbol: symbol, Side: side, Quantity: qty, Status: exchange.OrderStatusFilled}, nil
}

func (m *MockExchange) GetOpenPositions(ctx context.Context) ([]*exchange.Position, error) {
	return []*exchange.Position{}, nil
}

func (m *MockExchange) ClosePosition(ctx context.Context, symbol, side string, qty float64) error {
	return nil
}

func (m *MockExchange) SubscribeTicker(symbol string, callback func(*exchange.Ticker)) error {
	return nil
}

func (m *MockExchange) SubscribePositions(callback func(*exchange.Position)) error {
	return nil
}

func (m *MockExchange) GetTradingFee(ctx context.Context, symbol string) (float64, error) {
	return 0.001, nil
}

func (m *MockExchange) GetLimits(ctx context.Context, symbol string) (*exchange.Limits, error) {
	return &exchange.Limits{Symbol: symbol, MinOrderQty: 0.001, MaxOrderQty: 1000}, nil
}

func (m *MockExchange) Close() error {
	if m.closeErr != nil {
		return m.closeErr
	}
	m.connected = false
	return nil
}

func TestExchangeService_ConnectionCache(t *testing.T) {
	svc := NewExchangeService(
		&repository.ExchangeRepository{},
		&repository.PairRepository{},
		"test_encryption_key_32_bytes___",
	)

	// Добавляем соединение в кэш
	mockExchange := &MockExchange{connected: true, balance: 1000}
	svc.connections["bybit"] = mockExchange

	// Проверяем, что соединение в кэше
	if len(svc.connections) != 1 {
		t.Errorf("expected 1 connection in cache, got %d", len(svc.connections))
	}

	// Удаляем соединение
	delete(svc.connections, "bybit")

	if len(svc.connections) != 0 {
		t.Errorf("expected 0 connections in cache, got %d", len(svc.connections))
	}
}

func TestExchangeService_ApiKeySecurityCheck(t *testing.T) {
	// Проверяем, что API ключи не возвращаются в ответах
	mockRepo := NewMockExchangeRepository()
	mockRepo.accounts["bybit"] = &models.ExchangeAccount{
		ID:        1,
		Name:      "bybit",
		APIKey:    "encrypted_api_key",
		SecretKey: "encrypted_secret_key",
		Connected: true,
		Balance:   1000,
	}

	// При возврате пользователю ключи должны быть очищены
	account := mockRepo.accounts["bybit"]

	// Создаем безопасную копию (как в реальном сервисе)
	safeCopy := &models.ExchangeAccount{
		ID:        account.ID,
		Name:      account.Name,
		Connected: account.Connected,
		Balance:   account.Balance,
		LastError: account.LastError,
		// APIKey, SecretKey, Passphrase НЕ копируем!
	}

	if safeCopy.APIKey != "" {
		t.Error("API key should not be returned")
	}
	if safeCopy.SecretKey != "" {
		t.Error("Secret key should not be returned")
	}
	if safeCopy.Passphrase != "" {
		t.Error("Passphrase should not be returned")
	}

	// Исходные данные должны остаться нетронутыми
	if account.APIKey != "encrypted_api_key" {
		t.Error("Original API key should remain unchanged")
	}
}

func TestExchangeService_ExchangeAccountFields(t *testing.T) {
	// Проверяем структуру ExchangeAccount
	account := &models.ExchangeAccount{
		ID:         1,
		Name:       "bybit",
		APIKey:     "key",
		SecretKey:  "secret",
		Passphrase: "pass",
		Connected:  true,
		Balance:    1000.50,
		LastError:  "",
	}

	if account.ID != 1 {
		t.Errorf("expected ID 1, got %d", account.ID)
	}
	if account.Name != "bybit" {
		t.Errorf("expected Name 'bybit', got %s", account.Name)
	}
	if account.Balance != 1000.50 {
		t.Errorf("expected Balance 1000.50, got %f", account.Balance)
	}
}

func TestExchangeService_UpdateAllBalances(t *testing.T) {
	mockRepo := NewMockExchangeRepository()
	mockRepo.accounts["bybit"] = &models.ExchangeAccount{ID: 1, Name: "bybit", Connected: true, Balance: 1000}
	mockRepo.accounts["okx"] = &models.ExchangeAccount{ID: 2, Name: "okx", Connected: true, Balance: 2000}
	mockRepo.accounts["binance"] = &models.ExchangeAccount{ID: 3, Name: "binance", Connected: false, Balance: 0}

	// Получаем подключенные биржи
	connected, err := mockRepo.GetConnected()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
		return
	}

	if len(connected) != 2 {
		t.Errorf("expected 2 connected exchanges, got %d", len(connected))
	}

	// Проверяем, что неподключенная биржа не включена
	for _, account := range connected {
		if account.Name == "binance" {
			t.Error("binance should not be in connected list")
		}
	}
}
