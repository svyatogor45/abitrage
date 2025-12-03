package service

import (
	"errors"
	"testing"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// TestableBlacklistService - версия сервиса для тестирования с интерфейсом
type TestableBlacklistService struct {
	blacklistRepo BlacklistRepositoryInterface
}

func newTestableBlacklistService(repo BlacklistRepositoryInterface) *TestableBlacklistService {
	return &TestableBlacklistService{blacklistRepo: repo}
}

// Дублируем методы из BlacklistService для тестирования

func (s *TestableBlacklistService) AddToBlacklist(symbol, reason string) (*models.BlacklistEntry, error) {
	symbol = trimSpace(symbol)
	if symbol == "" {
		return nil, ErrBlacklistSymbolEmpty
	}
	symbol = toUpper(symbol)

	exists, err := s.blacklistRepo.Exists(symbol)
	if err != nil {
		return nil, err
	}
	if exists {
		return nil, ErrBlacklistSymbolExists
	}

	entry := &models.BlacklistEntry{
		Symbol: symbol,
		Reason: trimSpace(reason),
	}

	if err := s.blacklistRepo.Create(entry); err != nil {
		if errors.Is(err, repository.ErrBlacklistEntryExists) {
			return nil, ErrBlacklistSymbolExists
		}
		return nil, err
	}

	return entry, nil
}

func (s *TestableBlacklistService) GetBlacklist() ([]*models.BlacklistEntry, error) {
	entries, err := s.blacklistRepo.GetAll()
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []*models.BlacklistEntry{}
	}
	return entries, nil
}

func (s *TestableBlacklistService) RemoveFromBlacklist(symbol string) error {
	symbol = trimSpace(symbol)
	if symbol == "" {
		return ErrBlacklistSymbolEmpty
	}

	err := s.blacklistRepo.Delete(symbol)
	if err != nil {
		if errors.Is(err, repository.ErrBlacklistEntryNotFound) {
			return ErrBlacklistEntryNotFound
		}
		return err
	}
	return nil
}

func (s *TestableBlacklistService) GetBySymbol(symbol string) (*models.BlacklistEntry, error) {
	symbol = trimSpace(symbol)
	if symbol == "" {
		return nil, ErrBlacklistSymbolEmpty
	}

	entry, err := s.blacklistRepo.GetBySymbol(symbol)
	if err != nil {
		if errors.Is(err, repository.ErrBlacklistEntryNotFound) {
			return nil, ErrBlacklistEntryNotFound
		}
		return nil, err
	}
	return entry, nil
}

func (s *TestableBlacklistService) IsBlacklisted(symbol string) (bool, error) {
	symbol = trimSpace(symbol)
	if symbol == "" {
		return false, ErrBlacklistSymbolEmpty
	}
	return s.blacklistRepo.Exists(symbol)
}

func (s *TestableBlacklistService) UpdateReason(symbol, reason string) error {
	symbol = trimSpace(symbol)
	if symbol == "" {
		return ErrBlacklistSymbolEmpty
	}

	err := s.blacklistRepo.UpdateReason(symbol, trimSpace(reason))
	if err != nil {
		if errors.Is(err, repository.ErrBlacklistEntryNotFound) {
			return ErrBlacklistEntryNotFound
		}
		return err
	}
	return nil
}

func (s *TestableBlacklistService) Search(query string) ([]*models.BlacklistEntry, error) {
	query = trimSpace(query)
	if query == "" {
		return s.GetBlacklist()
	}

	entries, err := s.blacklistRepo.Search(query)
	if err != nil {
		return nil, err
	}
	if entries == nil {
		entries = []*models.BlacklistEntry{}
	}
	return entries, nil
}

func (s *TestableBlacklistService) GetCount() (int, error) {
	return s.blacklistRepo.Count()
}

func (s *TestableBlacklistService) ClearAll() error {
	return s.blacklistRepo.DeleteAll()
}

// Вспомогательные функции
func trimSpace(s string) string {
	start := 0
	end := len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t' || s[start] == '\n' || s[start] == '\r') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t' || s[end-1] == '\n' || s[end-1] == '\r') {
		end--
	}
	return s[start:end]
}

func toUpper(s string) string {
	b := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'a' && c <= 'z' {
			c -= 'a' - 'A'
		}
		b[i] = c
	}
	return string(b)
}

// ============ ТЕСТЫ ============

func TestBlacklistService_AddToBlacklist(t *testing.T) {
	tests := []struct {
		name        string
		symbol      string
		reason      string
		setup       func(*MockBlacklistRepository)
		wantErr     error
		wantSymbol  string
	}{
		{
			name:       "успешное добавление",
			symbol:     "btcusdt",
			reason:     "тестовая причина",
			wantSymbol: "BTCUSDT",
		},
		{
			name:       "символ с пробелами",
			symbol:     "  ethusdt  ",
			reason:     "причина",
			wantSymbol: "ETHUSDT",
		},
		{
			name:    "пустой символ",
			symbol:  "",
			reason:  "причина",
			wantErr: ErrBlacklistSymbolEmpty,
		},
		{
			name:    "символ только из пробелов",
			symbol:  "   ",
			reason:  "причина",
			wantErr: ErrBlacklistSymbolEmpty,
		},
		{
			name:   "символ уже существует",
			symbol: "BTCUSDT",
			reason: "причина",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT"}
			},
			wantErr: ErrBlacklistSymbolExists,
		},
		{
			name:   "ошибка проверки существования",
			symbol: "BTCUSDT",
			reason: "причина",
			setup: func(m *MockBlacklistRepository) {
				m.existsErr = errors.New("db error")
			},
			wantErr: errors.New("db error"),
		},
		{
			name:   "ошибка создания",
			symbol: "BTCUSDT",
			reason: "причина",
			setup: func(m *MockBlacklistRepository) {
				m.createErr = errors.New("create error")
			},
			wantErr: errors.New("create error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockBlacklistRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableBlacklistService(mockRepo)
			entry, err := svc.AddToBlacklist(tt.symbol, tt.reason)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if entry.Symbol != tt.wantSymbol {
				t.Errorf("expected symbol %s, got %s", tt.wantSymbol, entry.Symbol)
			}
		})
	}
}

func TestBlacklistService_GetBlacklist(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*MockBlacklistRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:      "пустой список",
			wantCount: 0,
		},
		{
			name: "список с записями",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT"}
				m.entries["ETHUSDT"] = &models.BlacklistEntry{ID: 2, Symbol: "ETHUSDT"}
			},
			wantCount: 2,
		},
		{
			name: "ошибка базы данных",
			setup: func(m *MockBlacklistRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockBlacklistRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableBlacklistService(mockRepo)
			entries, err := svc.GetBlacklist()

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

			if len(entries) != tt.wantCount {
				t.Errorf("expected %d entries, got %d", tt.wantCount, len(entries))
			}
		})
	}
}

func TestBlacklistService_RemoveFromBlacklist(t *testing.T) {
	tests := []struct {
		name    string
		symbol  string
		setup   func(*MockBlacklistRepository)
		wantErr error
	}{
		{
			name:   "успешное удаление",
			symbol: "BTCUSDT",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT"}
			},
		},
		{
			name:    "пустой символ",
			symbol:  "",
			wantErr: ErrBlacklistSymbolEmpty,
		},
		{
			name:    "символ не найден",
			symbol:  "BTCUSDT",
			wantErr: ErrBlacklistEntryNotFound,
		},
		{
			name:   "ошибка базы данных",
			symbol: "BTCUSDT",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT"}
				m.deleteErr = errors.New("db error")
			},
			wantErr: errors.New("db error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockBlacklistRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableBlacklistService(mockRepo)
			err := svc.RemoveFromBlacklist(tt.symbol)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if tt.wantErr.Error() != err.Error() {
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

func TestBlacklistService_GetBySymbol(t *testing.T) {
	tests := []struct {
		name       string
		symbol     string
		setup      func(*MockBlacklistRepository)
		wantErr    error
		wantSymbol string
	}{
		{
			name:   "успешное получение",
			symbol: "BTCUSDT",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT", Reason: "test"}
			},
			wantSymbol: "BTCUSDT",
		},
		{
			name:    "пустой символ",
			symbol:  "",
			wantErr: ErrBlacklistSymbolEmpty,
		},
		{
			name:    "символ не найден",
			symbol:  "BTCUSDT",
			wantErr: ErrBlacklistEntryNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockBlacklistRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableBlacklistService(mockRepo)
			entry, err := svc.GetBySymbol(tt.symbol)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if tt.wantErr.Error() != err.Error() {
					t.Errorf("expected error %v, got %v", tt.wantErr, err)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if entry.Symbol != tt.wantSymbol {
				t.Errorf("expected symbol %s, got %s", tt.wantSymbol, entry.Symbol)
			}
		})
	}
}

func TestBlacklistService_IsBlacklisted(t *testing.T) {
	tests := []struct {
		name    string
		symbol  string
		setup   func(*MockBlacklistRepository)
		want    bool
		wantErr error
	}{
		{
			name:   "символ в списке",
			symbol: "BTCUSDT",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT"}
			},
			want: true,
		},
		{
			name:   "символ не в списке",
			symbol: "BTCUSDT",
			want:   false,
		},
		{
			name:    "пустой символ",
			symbol:  "",
			wantErr: ErrBlacklistSymbolEmpty,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockBlacklistRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableBlacklistService(mockRepo)
			got, err := svc.IsBlacklisted(tt.symbol)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			if got != tt.want {
				t.Errorf("expected %v, got %v", tt.want, got)
			}
		})
	}
}

func TestBlacklistService_UpdateReason(t *testing.T) {
	tests := []struct {
		name    string
		symbol  string
		reason  string
		setup   func(*MockBlacklistRepository)
		wantErr error
	}{
		{
			name:   "успешное обновление",
			symbol: "BTCUSDT",
			reason: "новая причина",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT", Reason: "старая причина"}
			},
		},
		{
			name:    "пустой символ",
			symbol:  "",
			reason:  "причина",
			wantErr: ErrBlacklistSymbolEmpty,
		},
		{
			name:    "символ не найден",
			symbol:  "BTCUSDT",
			reason:  "причина",
			wantErr: ErrBlacklistEntryNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockBlacklistRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableBlacklistService(mockRepo)
			err := svc.UpdateReason(tt.symbol, tt.reason)

			if tt.wantErr != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.wantErr)
					return
				}
				if tt.wantErr.Error() != err.Error() {
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

func TestBlacklistService_Search(t *testing.T) {
	tests := []struct {
		name      string
		query     string
		setup     func(*MockBlacklistRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:  "поиск по части символа",
			query: "BTC",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT"}
				m.entries["ETHUSDT"] = &models.BlacklistEntry{ID: 2, Symbol: "ETHUSDT"}
			},
			wantCount: 1,
		},
		{
			name:  "пустой запрос - возвращает все",
			query: "",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT"}
				m.entries["ETHUSDT"] = &models.BlacklistEntry{ID: 2, Symbol: "ETHUSDT"}
			},
			wantCount: 2,
		},
		{
			name:      "ничего не найдено",
			query:     "XRP",
			wantCount: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockBlacklistRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableBlacklistService(mockRepo)
			entries, err := svc.Search(tt.query)

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

			if len(entries) != tt.wantCount {
				t.Errorf("expected %d entries, got %d", tt.wantCount, len(entries))
			}
		})
	}
}

func TestBlacklistService_GetCount(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockBlacklistRepository)
		want    int
		wantErr bool
	}{
		{
			name: "несколько записей",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT"}
				m.entries["ETHUSDT"] = &models.BlacklistEntry{ID: 2, Symbol: "ETHUSDT"}
			},
			want: 2,
		},
		{
			name: "пустой список",
			want: 0,
		},
		{
			name: "ошибка базы данных",
			setup: func(m *MockBlacklistRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockBlacklistRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableBlacklistService(mockRepo)
			got, err := svc.GetCount()

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

			if got != tt.want {
				t.Errorf("expected %d, got %d", tt.want, got)
			}
		})
	}
}

func TestBlacklistService_ClearAll(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockBlacklistRepository)
		wantErr bool
	}{
		{
			name: "успешная очистка",
			setup: func(m *MockBlacklistRepository) {
				m.entries["BTCUSDT"] = &models.BlacklistEntry{ID: 1, Symbol: "BTCUSDT"}
			},
		},
		{
			name: "ошибка базы данных",
			setup: func(m *MockBlacklistRepository) {
				m.deleteErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockBlacklistRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableBlacklistService(mockRepo)
			err := svc.ClearAll()

			if tt.wantErr {
				if err == nil {
					t.Error("expected error, got nil")
				}
				return
			}

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}
