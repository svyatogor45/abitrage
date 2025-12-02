package service

import (
	"strings"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// WebSocketBroadcaster - интерфейс для отправки WebSocket сообщений
//
// Позволяет избежать циклических зависимостей между пакетами
// и упрощает тестирование (можно подставить mock)
type WebSocketBroadcaster interface {
	BroadcastNotification(notif *models.Notification)
}

// NotificationService предоставляет бизнес-логику для управления уведомлениями.
//
// Отвечает за:
// - Создание уведомлений с проверкой настроек
// - Получение списка уведомлений с фильтрацией
// - Очистку журнала уведомлений
// - Broadcast уведомлений через WebSocket
//
// Типы уведомлений:
// - OPEN: открытие арбитража
// - CLOSE: закрытие позиций
// - SL: срабатывание Stop Loss
// - LIQUIDATION: ликвидация позиции
// - ERROR: ошибка API/ордера
// - MARGIN: недостаток маржи
// - PAUSE: пауза/остановка пары
// - SECOND_LEG_FAIL: не удалось открыть вторую ногу
type NotificationService struct {
	notificationRepo *repository.NotificationRepository
	settingsRepo     *repository.SettingsRepository
	wsHub            WebSocketBroadcaster
}

// NewNotificationService создает новый экземпляр NotificationService.
func NewNotificationService(
	notificationRepo *repository.NotificationRepository,
	settingsRepo *repository.SettingsRepository,
) *NotificationService {
	return &NotificationService{
		notificationRepo: notificationRepo,
		settingsRepo:     settingsRepo,
	}
}

// SetWebSocketHub устанавливает WebSocket hub для broadcast уведомлений.
//
// Вызывается после инициализации Hub в main.go:
//
//	notifService := service.NewNotificationService(notifRepo, settingsRepo)
//	notifService.SetWebSocketHub(wsHub)
func (s *NotificationService) SetWebSocketHub(hub WebSocketBroadcaster) {
	s.wsHub = hub
}

// CreateNotification создает новое уведомление.
//
// Перед созданием проверяет настройки уведомлений (notification_prefs).
// Если данный тип уведомлений отключен в настройках, уведомление не создается.
//
// После успешного создания отправляет broadcast через WebSocket (если hub настроен).
//
// Параметры:
// - notif: данные уведомления (Type, Severity, PairID, Message, Meta)
//
// Возвращает:
// - error: nil если уведомление создано или пропущено из-за настроек
func (s *NotificationService) CreateNotification(notif *models.Notification) error {
	// Проверяем настройки - включен ли данный тип уведомлений
	enabled, err := s.isNotificationTypeEnabled(notif.Type)
	if err != nil {
		// При ошибке получения настроек все равно создаем уведомление
		// (fail-safe: лучше уведомить, чем пропустить важное событие)
	} else if !enabled {
		// Тип уведомлений отключен в настройках - пропускаем
		return nil
	}

	// Создаем уведомление в БД
	if err := s.notificationRepo.Create(notif); err != nil {
		return err
	}

	// Broadcast через WebSocket hub для real-time обновления UI
	if s.wsHub != nil {
		s.wsHub.BroadcastNotification(notif)
	}

	return nil
}

// GetNotifications возвращает список уведомлений с фильтрацией.
//
// Параметры:
// - types: список типов для фильтрации (например: ["OPEN", "CLOSE", "SL"])
//          если пустой - возвращаются все типы
// - limit: максимальное количество записей (по умолчанию 100)
//
// Возвращает уведомления отсортированные по времени (новые сверху).
func (s *NotificationService) GetNotifications(types []string, limit int) ([]*models.Notification, error) {
	// Устанавливаем дефолтный лимит
	if limit <= 0 {
		limit = 100
	}

	// Ограничиваем максимальный лимит
	if limit > 500 {
		limit = 500
	}

	// Нормализуем типы (приводим к верхнему регистру)
	normalizedTypes := make([]string, 0, len(types))
	for _, t := range types {
		normalized := strings.ToUpper(strings.TrimSpace(t))
		if normalized != "" && s.isValidNotificationType(normalized) {
			normalizedTypes = append(normalizedTypes, normalized)
		}
	}

	// Если типы указаны, фильтруем по ним
	if len(normalizedTypes) > 0 {
		return s.notificationRepo.GetByTypes(normalizedTypes, limit)
	}

	// Если типы не указаны, возвращаем все
	return s.notificationRepo.GetRecent(limit)
}

// ClearNotifications очищает журнал уведомлений.
//
// Удаляет все уведомления из базы данных.
func (s *NotificationService) ClearNotifications() error {
	return s.notificationRepo.DeleteAll()
}

// GetNotificationCount возвращает общее количество уведомлений.
func (s *NotificationService) GetNotificationCount() (int, error) {
	return s.notificationRepo.Count()
}

// GetNotificationCountByType возвращает количество уведомлений определенного типа.
func (s *NotificationService) GetNotificationCountByType(notifType string) (int, error) {
	return s.notificationRepo.CountByType(strings.ToUpper(notifType))
}

// CleanupOld удаляет уведомления, оставляя только последние N записей.
//
// Используется для автоматической очистки при превышении лимита.
// По ТЗ лимит - 100 последних событий.
func (s *NotificationService) CleanupOld(keepCount int) (int64, error) {
	if keepCount <= 0 {
		keepCount = 100
	}
	return s.notificationRepo.KeepRecent(keepCount)
}

// isNotificationTypeEnabled проверяет, включен ли тип уведомлений в настройках.
func (s *NotificationService) isNotificationTypeEnabled(notifType string) (bool, error) {
	prefs, err := s.settingsRepo.GetNotificationPrefs()
	if err != nil {
		return true, err // При ошибке считаем включенным
	}

	if prefs == nil {
		return true, nil // Если настроек нет, все включено
	}

	// Маппинг типов уведомлений на поля настроек
	switch strings.ToUpper(notifType) {
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
		// Неизвестный тип - считаем включенным
		return true, nil
	}
}

// isValidNotificationType проверяет, является ли тип допустимым.
func (s *NotificationService) isValidNotificationType(notifType string) bool {
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
	return validTypes[strings.ToUpper(notifType)]
}

// CreateOpenNotification создает уведомление об открытии арбитража.
//
// Вспомогательный метод для удобного создания уведомлений.
func (s *NotificationService) CreateOpenNotification(pairID int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeOpen,
		Severity: models.SeverityInfo,
		PairID:   &pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

// CreateCloseNotification создает уведомление о закрытии позиций.
func (s *NotificationService) CreateCloseNotification(pairID int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeClose,
		Severity: models.SeverityInfo,
		PairID:   &pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

// CreateSLNotification создает уведомление о срабатывании Stop Loss.
func (s *NotificationService) CreateSLNotification(pairID int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeSL,
		Severity: models.SeverityWarn,
		PairID:   &pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

// CreateLiquidationNotification создает уведомление о ликвидации.
func (s *NotificationService) CreateLiquidationNotification(pairID int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeLiquidation,
		Severity: models.SeverityError,
		PairID:   &pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

// CreateErrorNotification создает уведомление об ошибке API.
func (s *NotificationService) CreateErrorNotification(pairID *int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeError,
		Severity: models.SeverityError,
		PairID:   pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

// CreateMarginNotification создает уведомление о недостатке маржи.
func (s *NotificationService) CreateMarginNotification(pairID int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeMargin,
		Severity: models.SeverityWarn,
		PairID:   &pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

// CreatePauseNotification создает уведомление о паузе/остановке.
func (s *NotificationService) CreatePauseNotification(pairID int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypePause,
		Severity: models.SeverityWarn,
		PairID:   &pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}

// CreateSecondLegFailNotification создает уведомление о неудаче второй ноги.
func (s *NotificationService) CreateSecondLegFailNotification(pairID int, message string, meta map[string]interface{}) error {
	notif := &models.Notification{
		Type:     models.NotificationTypeSecondLegFail,
		Severity: models.SeverityError,
		PairID:   &pairID,
		Message:  message,
		Meta:     meta,
	}
	return s.CreateNotification(notif)
}
