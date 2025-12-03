package service

import (
	"errors"
	"testing"

	"arbitrage/internal/models"
)

// TestableNotificationService - версия сервиса для тестирования
type TestableNotificationService struct {
	notificationRepo NotificationRepositoryInterface
	settingsRepo     SettingsRepositoryInterface
	wsHub            WebSocketBroadcaster
}

func newTestableNotificationService(
	notifRepo NotificationRepositoryInterface,
	settingsRepo SettingsRepositoryInterface,
) *TestableNotificationService {
	return &TestableNotificationService{
		notificationRepo: notifRepo,
		settingsRepo:     settingsRepo,
	}
}

func (s *TestableNotificationService) SetWebSocketHub(hub WebSocketBroadcaster) {
	s.wsHub = hub
}

func (s *TestableNotificationService) CreateNotification(notif *models.Notification) error {
	enabled, err := s.isNotificationTypeEnabled(notif.Type)
	if err != nil {
		// fail-safe
	} else if !enabled {
		return nil
	}

	if err := s.notificationRepo.Create(notif); err != nil {
		return err
	}

	if s.wsHub != nil {
		s.wsHub.BroadcastNotification(notif)
	}

	return nil
}

func (s *TestableNotificationService) GetNotifications(types []string, limit int) ([]*models.Notification, error) {
	if limit <= 0 {
		limit = 100
	}
	if limit > 500 {
		limit = 500
	}

	normalizedTypes := make([]string, 0, len(types))
	for _, t := range types {
		normalized := toUpper(trimSpace(t))
		if normalized != "" && s.isValidNotificationType(normalized) {
			normalizedTypes = append(normalizedTypes, normalized)
		}
	}

	if len(normalizedTypes) > 0 {
		return s.notificationRepo.GetByTypes(normalizedTypes, limit)
	}

	return s.notificationRepo.GetRecent(limit)
}

func (s *TestableNotificationService) ClearNotifications() error {
	return s.notificationRepo.DeleteAll()
}

func (s *TestableNotificationService) GetNotificationCount() (int, error) {
	return s.notificationRepo.Count()
}

func (s *TestableNotificationService) GetNotificationCountByType(notifType string) (int, error) {
	return s.notificationRepo.CountByType(toUpper(notifType))
}

func (s *TestableNotificationService) CleanupOld(keepCount int) (int64, error) {
	if keepCount <= 0 {
		keepCount = 100
	}
	return s.notificationRepo.KeepRecent(keepCount)
}

func (s *TestableNotificationService) isNotificationTypeEnabled(notifType string) (bool, error) {
	prefs, err := s.settingsRepo.GetNotificationPrefs()
	if err != nil {
		return true, err
	}

	if prefs == nil {
		return true, nil
	}

	switch toUpper(notifType) {
	case models.NotificationTypeOpen:
		return prefs.Open, nil
	case models.NotificationTypeClose:
		return prefs.Close, nil
	case models.NotificationTypeSL:
		return prefs.StopLoss, nil
	case models.NotificationTypeLiquidation:
		return prefs.Liquidation, nil
	case models.NotificationTypeError:
		return prefs.APIError, nil
	case models.NotificationTypeMargin:
		return prefs.Margin, nil
	case models.NotificationTypePause:
		return prefs.Pause, nil
	case models.NotificationTypeSecondLegFail:
		return prefs.SecondLegFail, nil
	default:
		return true, nil
	}
}

func (s *TestableNotificationService) isValidNotificationType(notifType string) bool {
	validTypes := map[string]bool{
		models.NotificationTypeOpen:          true,
		models.NotificationTypeClose:         true,
		models.NotificationTypeSL:            true,
		models.NotificationTypeLiquidation:   true,
		models.NotificationTypeError:         true,
		models.NotificationTypeMargin:        true,
		models.NotificationTypePause:         true,
		models.NotificationTypeSecondLegFail: true,
	}
	return validTypes[toUpper(notifType)]
}

// Вспомогательные методы для создания уведомлений
func (s *TestableNotificationService) CreateOpenNotification(pairID int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeOpen,
		Severity: models.SeverityInfo,
		PairID:   &pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

func (s *TestableNotificationService) CreateCloseNotification(pairID int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeClose,
		Severity: models.SeverityInfo,
		PairID:   &pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

func (s *TestableNotificationService) CreateSLNotification(pairID int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeSL,
		Severity: models.SeverityWarn,
		PairID:   &pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

func (s *TestableNotificationService) CreateErrorNotification(pairID *int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeError,
		Severity: models.SeverityError,
		PairID:   pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

// ============ ТЕСТЫ ============

func TestNotificationService_CreateNotification(t *testing.T) {
	tests := []struct {
		name           string
		notif          *models.Notification
		setupSettings  func(*MockSettingsRepository)
		setupNotif     func(*MockNotificationRepository)
		wantErr        bool
		wantBroadcast  bool
		wantSkipped    bool
	}{
		{
			name: "успешное создание уведомления OPEN",
			notif: &models.Notification{
				Type:     models.NotificationTypeOpen,
				Severity: models.SeverityInfo,
				Message:  "Открытие позиции",
			},
			wantBroadcast: true,
		},
		{
			name: "уведомление отключено в настройках",
			notif: &models.Notification{
				Type:     models.NotificationTypeOpen,
				Severity: models.SeverityInfo,
				Message:  "Открытие позиции",
			},
			setupSettings: func(m *MockSettingsRepository) {
				m.settings.NotificationPrefs.Open = false
			},
			wantSkipped: true,
		},
		{
			name: "уведомление SL включено",
			notif: &models.Notification{
				Type:     models.NotificationTypeSL,
				Severity: models.SeverityWarn,
				Message:  "Stop Loss сработал",
			},
			wantBroadcast: true,
		},
		{
			name: "ошибка создания",
			notif: &models.Notification{
				Type:    models.NotificationTypeOpen,
				Message: "тест",
			},
			setupNotif: func(m *MockNotificationRepository) {
				m.createErr = errors.New("create error")
			},
			wantErr: true,
		},
		{
			name: "неизвестный тип - все равно создаем",
			notif: &models.Notification{
				Type:    "UNKNOWN",
				Message: "тест",
			},
			wantBroadcast: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNotifRepo := NewMockNotificationRepository()
			mockSettingsRepo := NewMockSettingsRepository()
			mockWsHub := NewMockWebSocketBroadcaster()

			if tt.setupSettings != nil {
				tt.setupSettings(mockSettingsRepo)
			}
			if tt.setupNotif != nil {
				tt.setupNotif(mockNotifRepo)
			}

			svc := newTestableNotificationService(mockNotifRepo, mockSettingsRepo)
			svc.SetWebSocketHub(mockWsHub)

			err := svc.CreateNotification(tt.notif)

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

			if tt.wantSkipped {
				if len(mockNotifRepo.notifications) > 0 {
					t.Error("expected notification to be skipped")
				}
				return
			}

			if tt.wantBroadcast {
				if len(mockWsHub.notifications) == 0 {
					t.Error("expected broadcast, got none")
				}
			}
		})
	}
}

func TestNotificationService_GetNotifications(t *testing.T) {
	tests := []struct {
		name      string
		types     []string
		limit     int
		setup     func(*MockNotificationRepository)
		wantCount int
		wantErr   bool
	}{
		{
			name:  "получение всех уведомлений",
			limit: 100,
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
					{ID: 2, Type: models.NotificationTypeClose},
					{ID: 3, Type: models.NotificationTypeSL},
				}
			},
			wantCount: 3,
		},
		{
			name:  "фильтрация по типу",
			types: []string{models.NotificationTypeOpen},
			limit: 100,
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
					{ID: 2, Type: models.NotificationTypeClose},
					{ID: 3, Type: models.NotificationTypeOpen},
				}
			},
			wantCount: 2,
		},
		{
			name:  "дефолтный лимит при 0",
			types: nil,
			limit: 0,
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
				}
			},
			wantCount: 1,
		},
		{
			name:  "максимальный лимит 500",
			types: nil,
			limit: 1000,
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
				}
			},
			wantCount: 1,
		},
		{
			name:  "игнорирование невалидных типов",
			types: []string{"INVALID_TYPE", models.NotificationTypeOpen},
			limit: 100,
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
					{ID: 2, Type: models.NotificationTypeClose},
				}
			},
			wantCount: 1,
		},
		{
			name:  "ошибка базы данных",
			limit: 100,
			setup: func(m *MockNotificationRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNotifRepo := NewMockNotificationRepository()
			mockSettingsRepo := NewMockSettingsRepository()

			if tt.setup != nil {
				tt.setup(mockNotifRepo)
			}

			svc := newTestableNotificationService(mockNotifRepo, mockSettingsRepo)
			notifications, err := svc.GetNotifications(tt.types, tt.limit)

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

			if len(notifications) != tt.wantCount {
				t.Errorf("expected %d notifications, got %d", tt.wantCount, len(notifications))
			}
		})
	}
}

func TestNotificationService_ClearNotifications(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockNotificationRepository)
		wantErr bool
	}{
		{
			name: "успешная очистка",
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
					{ID: 2, Type: models.NotificationTypeClose},
				}
			},
		},
		{
			name: "ошибка базы данных",
			setup: func(m *MockNotificationRepository) {
				m.deleteErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNotifRepo := NewMockNotificationRepository()
			mockSettingsRepo := NewMockSettingsRepository()

			if tt.setup != nil {
				tt.setup(mockNotifRepo)
			}

			svc := newTestableNotificationService(mockNotifRepo, mockSettingsRepo)
			err := svc.ClearNotifications()

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

func TestNotificationService_GetNotificationCount(t *testing.T) {
	tests := []struct {
		name    string
		setup   func(*MockNotificationRepository)
		want    int
		wantErr bool
	}{
		{
			name: "подсчет уведомлений",
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
					{ID: 2, Type: models.NotificationTypeClose},
					{ID: 3, Type: models.NotificationTypeSL},
				}
			},
			want: 3,
		},
		{
			name: "пустой список",
			want: 0,
		},
		{
			name: "ошибка базы данных",
			setup: func(m *MockNotificationRepository) {
				m.getErr = errors.New("db error")
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNotifRepo := NewMockNotificationRepository()
			mockSettingsRepo := NewMockSettingsRepository()

			if tt.setup != nil {
				tt.setup(mockNotifRepo)
			}

			svc := newTestableNotificationService(mockNotifRepo, mockSettingsRepo)
			count, err := svc.GetNotificationCount()

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

func TestNotificationService_GetNotificationCountByType(t *testing.T) {
	tests := []struct {
		name      string
		notifType string
		setup     func(*MockNotificationRepository)
		want      int
		wantErr   bool
	}{
		{
			name:      "подсчет OPEN уведомлений",
			notifType: models.NotificationTypeOpen,
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
					{ID: 2, Type: models.NotificationTypeClose},
					{ID: 3, Type: models.NotificationTypeOpen},
				}
			},
			want: 2,
		},
		{
			name:      "нет уведомлений данного типа",
			notifType: models.NotificationTypeSL,
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
				}
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNotifRepo := NewMockNotificationRepository()
			mockSettingsRepo := NewMockSettingsRepository()

			if tt.setup != nil {
				tt.setup(mockNotifRepo)
			}

			svc := newTestableNotificationService(mockNotifRepo, mockSettingsRepo)
			count, err := svc.GetNotificationCountByType(tt.notifType)

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

func TestNotificationService_CleanupOld(t *testing.T) {
	tests := []struct {
		name      string
		keepCount int
		setup     func(*MockNotificationRepository)
		want      int64
		wantErr   bool
	}{
		{
			name:      "очистка старых уведомлений",
			keepCount: 2,
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
					{ID: 2, Type: models.NotificationTypeClose},
					{ID: 3, Type: models.NotificationTypeSL},
					{ID: 4, Type: models.NotificationTypeError},
				}
			},
			want: 2,
		},
		{
			name:      "дефолтный keepCount при 0",
			keepCount: 0,
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
				}
			},
			want: 0,
		},
		{
			name:      "ничего не удалено",
			keepCount: 10,
			setup: func(m *MockNotificationRepository) {
				m.notifications = []*models.Notification{
					{ID: 1, Type: models.NotificationTypeOpen},
				}
			},
			want: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNotifRepo := NewMockNotificationRepository()
			mockSettingsRepo := NewMockSettingsRepository()

			if tt.setup != nil {
				tt.setup(mockNotifRepo)
			}

			svc := newTestableNotificationService(mockNotifRepo, mockSettingsRepo)
			deleted, err := svc.CleanupOld(tt.keepCount)

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

func TestNotificationService_HelperMethods(t *testing.T) {
	mockNotifRepo := NewMockNotificationRepository()
	mockSettingsRepo := NewMockSettingsRepository()
	mockWsHub := NewMockWebSocketBroadcaster()

	svc := newTestableNotificationService(mockNotifRepo, mockSettingsRepo)
	svc.SetWebSocketHub(mockWsHub)

	pairID := 1

	t.Run("CreateOpenNotification", func(t *testing.T) {
		err := svc.CreateOpenNotification(pairID, "тест", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("CreateCloseNotification", func(t *testing.T) {
		err := svc.CreateCloseNotification(pairID, "тест", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("CreateSLNotification", func(t *testing.T) {
		err := svc.CreateSLNotification(pairID, "тест", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("CreateErrorNotification", func(t *testing.T) {
		err := svc.CreateErrorNotification(&pairID, "тест", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})

	t.Run("CreateErrorNotification без pairID", func(t *testing.T) {
		err := svc.CreateErrorNotification(nil, "общая ошибка", nil)
		if err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
}

func TestNotificationService_ValidNotificationTypes(t *testing.T) {
	mockNotifRepo := NewMockNotificationRepository()
	mockSettingsRepo := NewMockSettingsRepository()
	svc := newTestableNotificationService(mockNotifRepo, mockSettingsRepo)

	validTypes := []string{
		models.NotificationTypeOpen,
		models.NotificationTypeClose,
		models.NotificationTypeSL,
		models.NotificationTypeLiquidation,
		models.NotificationTypeError,
		models.NotificationTypeMargin,
		models.NotificationTypePause,
		models.NotificationTypeSecondLegFail,
	}

	for _, typ := range validTypes {
		t.Run(typ, func(t *testing.T) {
			if !svc.isValidNotificationType(typ) {
				t.Errorf("expected %s to be valid", typ)
			}
		})
	}

	invalidTypes := []string{"INVALID", "unknown", "", "TEST"}
	for _, typ := range invalidTypes {
		t.Run("invalid_"+typ, func(t *testing.T) {
			if svc.isValidNotificationType(typ) {
				t.Errorf("expected %s to be invalid", typ)
			}
		})
	}
}

func TestNotificationService_NotificationPrefsFiltering(t *testing.T) {
	tests := []struct {
		name       string
		notifType  string
		setupPrefs func(*MockSettingsRepository)
		wantCreate bool
	}{
		{
			name:      "OPEN включен",
			notifType: models.NotificationTypeOpen,
			setupPrefs: func(m *MockSettingsRepository) {
				m.settings.NotificationPrefs.Open = true
			},
			wantCreate: true,
		},
		{
			name:      "OPEN отключен",
			notifType: models.NotificationTypeOpen,
			setupPrefs: func(m *MockSettingsRepository) {
				m.settings.NotificationPrefs.Open = false
			},
			wantCreate: false,
		},
		{
			name:      "SL включен",
			notifType: models.NotificationTypeSL,
			setupPrefs: func(m *MockSettingsRepository) {
				m.settings.NotificationPrefs.StopLoss = true
			},
			wantCreate: true,
		},
		{
			name:      "SL отключен",
			notifType: models.NotificationTypeSL,
			setupPrefs: func(m *MockSettingsRepository) {
				m.settings.NotificationPrefs.StopLoss = false
			},
			wantCreate: false,
		},
		{
			name:      "LIQUIDATION включен",
			notifType: models.NotificationTypeLiquidation,
			setupPrefs: func(m *MockSettingsRepository) {
				m.settings.NotificationPrefs.Liquidation = true
			},
			wantCreate: true,
		},
		{
			name:      "LIQUIDATION отключен",
			notifType: models.NotificationTypeLiquidation,
			setupPrefs: func(m *MockSettingsRepository) {
				m.settings.NotificationPrefs.Liquidation = false
			},
			wantCreate: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockNotifRepo := NewMockNotificationRepository()
			mockSettingsRepo := NewMockSettingsRepository()

			if tt.setupPrefs != nil {
				tt.setupPrefs(mockSettingsRepo)
			}

			svc := newTestableNotificationService(mockNotifRepo, mockSettingsRepo)

			notif := &models.Notification{
				Type:    tt.notifType,
				Message: "тест",
			}

			err := svc.CreateNotification(notif)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
				return
			}

			created := len(mockNotifRepo.notifications) > 0
			if created != tt.wantCreate {
				t.Errorf("expected created=%v, got %v", tt.wantCreate, created)
			}
		})
	}
}
