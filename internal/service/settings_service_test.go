package service

import (
	"errors"
	"testing"
	"time"

	"arbitrage/internal/models"
)

// TestableSettingsService - версия сервиса для тестирования
type TestableSettingsService struct {
	settingsRepo SettingsRepositoryInterface
}

func newTestableSettingsService(repo SettingsRepositoryInterface) *TestableSettingsService {
	return &TestableSettingsService{settingsRepo: repo}
}

func (s *TestableSettingsService) GetSettings() (*models.Settings, error) {
	return s.settingsRepo.Get()
}

func (s *TestableSettingsService) UpdateSettings(req *UpdateSettingsRequest) (*models.Settings, error) {
	settings, err := s.settingsRepo.Get()
	if err != nil {
		return nil, err
	}

	if req.ConsiderFunding != nil {
		settings.ConsiderFunding = *req.ConsiderFunding
	}

	if req.ClearMaxConcurrentTrades {
		settings.MaxConcurrentTrades = nil
	} else if req.MaxConcurrentTrades != nil {
		if *req.MaxConcurrentTrades < 1 {
			return nil, ErrInvalidMaxConcurrentTrades
		}
		settings.MaxConcurrentTrades = req.MaxConcurrentTrades
	}

	if req.NotificationPrefs != nil {
		settings.NotificationPrefs = *req.NotificationPrefs
	}

	if err := s.settingsRepo.Update(settings); err != nil {
		return nil, err
	}

	return settings, nil
}

func (s *TestableSettingsService) UpdateNotificationPrefs(prefs models.NotificationPreferences) error {
	return s.settingsRepo.UpdateNotificationPrefs(prefs)
}

func (s *TestableSettingsService) UpdateMaxConcurrentTrades(maxTrades *int) error {
	if maxTrades != nil && *maxTrades < 1 {
		return ErrInvalidMaxConcurrentTrades
	}
	return s.settingsRepo.UpdateMaxConcurrentTrades(maxTrades)
}

func (s *TestableSettingsService) UpdateConsiderFunding(consider bool) error {
	return s.settingsRepo.UpdateConsiderFunding(consider)
}

func (s *TestableSettingsService) GetNotificationPrefs() (*models.NotificationPreferences, error) {
	return s.settingsRepo.GetNotificationPrefs()
}

func (s *TestableSettingsService) GetMaxConcurrentTrades() (*int, error) {
	return s.settingsRepo.GetMaxConcurrentTrades()
}

func (s *TestableSettingsService) ResetToDefaults() error {
	return s.settingsRepo.ResetToDefaults()
}

// ============ ТЕСТЫ ============

func TestSettingsService_GetSettings(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockSettingsRepository)
		wantErr bool
	}{
		{
			name: "успешное получение настроек",
		},
		{
			name: "ошибка базы данных",
			setup: func(m *MockSettingsRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockSettingsRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableSettingsService(mockRepo)
			settings, err := svc.GetSettings()

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

			if settings == nil {
				t.Error("expected settings, got nil")
			}
		})
	}
}

func TestSettingsService_UpdateSettings(t *testing.T) {
	tests := []struct {
		name    string
		req     *UpdateSettingsRequest
		setup   func(*MockSettingsRepository)
		check   func(*testing.T, *models.Settings)
		wantErr error
	}{
		{
			name: "обновление consider_funding",
			req: &UpdateSettingsRequest{
				ConsiderFunding: boolPtr(true),
			},
			check: func(t *testing.T, s *models.Settings) {
				if !s.ConsiderFunding {
					t.Error("expected ConsiderFunding to be true")
				}
			},
		},
		{
			name: "обновление max_concurrent_trades",
			req: &UpdateSettingsRequest{
				MaxConcurrentTrades: intPtr(5),
			},
			check: func(t *testing.T, s *models.Settings) {
				if s.MaxConcurrentTrades == nil || *s.MaxConcurrentTrades != 5 {
					t.Error("expected MaxConcurrentTrades to be 5")
				}
			},
		},
		{
			name: "сброс max_concurrent_trades",
			req: &UpdateSettingsRequest{
				ClearMaxConcurrentTrades: true,
			},
			setup: func(m *MockSettingsRepository) {
				m.settings.MaxConcurrentTrades = intPtr(10)
			},
			check: func(t *testing.T, s *models.Settings) {
				if s.MaxConcurrentTrades != nil {
					t.Error("expected MaxConcurrentTrades to be nil")
				}
			},
		},
		{
			name: "обновление notification_prefs",
			req: &UpdateSettingsRequest{
				NotificationPrefs: &models.NotificationPreferences{
					Open:  false,
					Close: false,
				},
			},
			check: func(t *testing.T, s *models.Settings) {
				if s.NotificationPrefs.Open {
					t.Error("expected Open to be false")
				}
				if s.NotificationPrefs.Close {
					t.Error("expected Close to be false")
				}
			},
		},
		{
			name: "невалидный max_concurrent_trades (0)",
			req: &UpdateSettingsRequest{
				MaxConcurrentTrades: intPtr(0),
			},
			wantErr: ErrInvalidMaxConcurrentTrades,
		},
		{
			name: "невалидный max_concurrent_trades (отрицательный)",
			req: &UpdateSettingsRequest{
				MaxConcurrentTrades: intPtr(-1),
			},
			wantErr: ErrInvalidMaxConcurrentTrades,
		},
		{
			name: "ошибка получения настроек",
			req:  &UpdateSettingsRequest{},
			setup: func(m *MockSettingsRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: errors.New("db error"),
		},
		{
			name: "ошибка обновления",
			req: &UpdateSettingsRequest{
				ConsiderFunding: boolPtr(true),
			},
			setup: func(m *MockSettingsRepository) {
				m.updateErr = errors.New("update error")
			},
			wantErr: errors.New("update error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockSettingsRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableSettingsService(mockRepo)
			settings, err := svc.UpdateSettings(tt.req)

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

			if tt.check != nil {
				tt.check(t, settings)
			}
		})
	}
}

func TestSettingsService_UpdateNotificationPrefs(t *testing.T) {
	tests := []struct {
		name    string
		prefs   models.NotificationPreferences
		setup   func(*MockSettingsRepository)
		wantErr bool
	}{
		{
			name: "успешное обновление",
			prefs: models.NotificationPreferences{
				Open:     false,
				Close:    false,
				StopLoss: true,
			},
		},
		{
			name:  "все уведомления включены",
			prefs: models.NotificationPreferences{
				Open:          true,
				Close:         true,
				StopLoss:      true,
				Liquidation:   true,
				APIError:      true,
				Margin:        true,
				Pause:         true,
				SecondLegFail: true,
			},
		},
		{
			name:  "ошибка обновления",
			prefs: models.NotificationPreferences{},
			setup: func(m *MockSettingsRepository) {
				m.updateErr = errors.New("update error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockSettingsRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableSettingsService(mockRepo)
			err := svc.UpdateNotificationPrefs(tt.prefs)

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

func TestSettingsService_UpdateMaxConcurrentTrades(t *testing.T) {
	tests := []struct {
		name      string
		maxTrades *int
		setup     func(*MockSettingsRepository)
		wantErr   error
	}{
		{
			name:      "установка лимита",
			maxTrades: intPtr(5),
		},
		{
			name:      "снятие лимита (nil)",
			maxTrades: nil,
		},
		{
			name:      "невалидное значение (0)",
			maxTrades: intPtr(0),
			wantErr:   ErrInvalidMaxConcurrentTrades,
		},
		{
			name:      "невалидное значение (отрицательное)",
			maxTrades: intPtr(-5),
			wantErr:   ErrInvalidMaxConcurrentTrades,
		},
		{
			name:      "минимальное валидное значение",
			maxTrades: intPtr(1),
		},
		{
			name:      "ошибка обновления",
			maxTrades: intPtr(5),
			setup: func(m *MockSettingsRepository) {
				m.updateErr = errors.New("update error")
			},
			wantErr: errors.New("update error"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockSettingsRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableSettingsService(mockRepo)
			err := svc.UpdateMaxConcurrentTrades(tt.maxTrades)

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

func TestSettingsService_UpdateConsiderFunding(t *testing.T) {
	tests := []struct {
		name     string
		consider bool
		setup    func(*MockSettingsRepository)
		wantErr  bool
	}{
		{
			name:     "включить учет фандинга",
			consider: true,
		},
		{
			name:     "отключить учет фандинга",
			consider: false,
		},
		{
			name:     "ошибка обновления",
			consider: true,
			setup: func(m *MockSettingsRepository) {
				m.updateErr = errors.New("update error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockSettingsRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableSettingsService(mockRepo)
			err := svc.UpdateConsiderFunding(tt.consider)

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

func TestSettingsService_GetNotificationPrefs(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockSettingsRepository)
		check   func(*testing.T, *models.NotificationPreferences)
		wantErr bool
	}{
		{
			name: "успешное получение",
			check: func(t *testing.T, prefs *models.NotificationPreferences) {
				if prefs == nil {
					t.Error("expected prefs, got nil")
				}
			},
		},
		{
			name: "ошибка получения",
			setup: func(m *MockSettingsRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockSettingsRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableSettingsService(mockRepo)
			prefs, err := svc.GetNotificationPrefs()

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
				tt.check(t, prefs)
			}
		})
	}
}

func TestSettingsService_GetMaxConcurrentTrades(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockSettingsRepository)
		want    *int
		wantErr bool
	}{
		{
			name: "без ограничения",
			want: nil,
		},
		{
			name: "с ограничением",
			setup: func(m *MockSettingsRepository) {
				m.settings.MaxConcurrentTrades = intPtr(10)
			},
			want: intPtr(10),
		},
		{
			name: "ошибка получения",
			setup: func(m *MockSettingsRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockSettingsRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableSettingsService(mockRepo)
			got, err := svc.GetMaxConcurrentTrades()

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

			if tt.want == nil && got != nil {
				t.Errorf("expected nil, got %d", *got)
			}
			if tt.want != nil && (got == nil || *got != *tt.want) {
				t.Errorf("expected %d, got %v", *tt.want, got)
			}
		})
	}
}

func TestSettingsService_ResetToDefaults(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockSettingsRepository)
		wantErr bool
	}{
		{
			name: "успешный сброс",
			setup: func(m *MockSettingsRepository) {
				m.settings.ConsiderFunding = true
				m.settings.MaxConcurrentTrades = intPtr(10)
				m.settings.NotificationPrefs.Open = false
			},
		},
		{
			name: "ошибка сброса",
			setup: func(m *MockSettingsRepository) {
				m.updateErr = errors.New("update error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockRepo := NewMockSettingsRepository()
			if tt.setup != nil {
				tt.setup(mockRepo)
			}

			svc := newTestableSettingsService(mockRepo)
			err := svc.ResetToDefaults()

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

func TestSettingsService_DefaultValues(t *testing.T) {
	mockRepo := NewMockSettingsRepository()
	svc := newTestableSettingsService(mockRepo)

	settings, err := svc.GetSettings()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Проверяем дефолтные значения
	if settings.ConsiderFunding != false {
		t.Error("default ConsiderFunding should be false")
	}

	if settings.MaxConcurrentTrades != nil {
		t.Error("default MaxConcurrentTrades should be nil")
	}

	// Все уведомления должны быть включены по умолчанию
	prefs := settings.NotificationPrefs
	if !prefs.Open || !prefs.Close || !prefs.StopLoss || !prefs.Liquidation ||
		!prefs.APIError || !prefs.Margin || !prefs.Pause || !prefs.SecondLegFail {
		t.Error("all notification types should be enabled by default")
	}
}

// Вспомогательные функции для создания указателей
func intPtr(i int) *int {
	return &i
}

func boolPtr(b bool) *bool {
	return &b
}

func timePtr(t time.Time) *time.Time {
	return &t
}

func float64Ptr(f float64) *float64 {
	return &f
}
