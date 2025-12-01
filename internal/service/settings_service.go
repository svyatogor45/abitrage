package service

import (
	"errors"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// Ошибки сервиса настроек
var (
	ErrInvalidMaxConcurrentTrades = errors.New("max_concurrent_trades must be >= 1 or null")
)

// SettingsService предоставляет бизнес-логику для управления глобальными настройками.
//
// Отвечает за:
// - Получение и обновление глобальных настроек бота
// - Валидацию параметров настроек
// - Управление notification_prefs, max_concurrent_trades, consider_funding
type SettingsService struct {
	settingsRepo *repository.SettingsRepository
}

// NewSettingsService создает новый экземпляр SettingsService.
func NewSettingsService(settingsRepo *repository.SettingsRepository) *SettingsService {
	return &SettingsService{
		settingsRepo: settingsRepo,
	}
}

// GetSettings возвращает текущие глобальные настройки.
//
// Если записи в БД нет, создается запись с дефолтными значениями.
func (s *SettingsService) GetSettings() (*models.Settings, error) {
	return s.settingsRepo.Get()
}

// UpdateSettingsRequest представляет запрос на обновление настроек.
// Все поля опциональны - обновляются только переданные.
type UpdateSettingsRequest struct {
	ConsiderFunding     *bool                          `json:"consider_funding,omitempty"`
	MaxConcurrentTrades *int                           `json:"max_concurrent_trades,omitempty"`
	NotificationPrefs   *models.NotificationPreferences `json:"notification_prefs,omitempty"`
	// Флаг для явного сброса max_concurrent_trades в null (без ограничений)
	ClearMaxConcurrentTrades bool `json:"clear_max_concurrent_trades,omitempty"`
}

// UpdateSettings обновляет глобальные настройки.
//
// Принимает только те поля, которые нужно обновить.
// Валидирует параметры перед сохранением.
//
// Правила валидации:
// - max_concurrent_trades: >= 1 или null (без ограничений)
// - notification_prefs: все поля bool, валидация не требуется
// - consider_funding: bool, валидация не требуется
func (s *SettingsService) UpdateSettings(req *UpdateSettingsRequest) (*models.Settings, error) {
	// Получаем текущие настройки
	settings, err := s.settingsRepo.Get()
	if err != nil {
		return nil, err
	}

	// Обновляем только переданные поля
	if req.ConsiderFunding != nil {
		settings.ConsiderFunding = *req.ConsiderFunding
	}

	// Обработка max_concurrent_trades
	if req.ClearMaxConcurrentTrades {
		// Явный сброс в null (без ограничений)
		settings.MaxConcurrentTrades = nil
	} else if req.MaxConcurrentTrades != nil {
		// Валидация: должно быть >= 1
		if *req.MaxConcurrentTrades < 1 {
			return nil, ErrInvalidMaxConcurrentTrades
		}
		settings.MaxConcurrentTrades = req.MaxConcurrentTrades
	}

	// Обновление notification_prefs
	if req.NotificationPrefs != nil {
		settings.NotificationPrefs = *req.NotificationPrefs
	}

	// Сохраняем в БД
	if err := s.settingsRepo.Update(settings); err != nil {
		return nil, err
	}

	return settings, nil
}

// UpdateNotificationPrefs обновляет только настройки уведомлений.
func (s *SettingsService) UpdateNotificationPrefs(prefs models.NotificationPreferences) error {
	return s.settingsRepo.UpdateNotificationPrefs(prefs)
}

// UpdateMaxConcurrentTrades обновляет лимит одновременных арбитражей.
//
// Передайте nil для снятия ограничения.
// Значение должно быть >= 1 или nil.
func (s *SettingsService) UpdateMaxConcurrentTrades(maxTrades *int) error {
	// Валидация
	if maxTrades != nil && *maxTrades < 1 {
		return ErrInvalidMaxConcurrentTrades
	}
	return s.settingsRepo.UpdateMaxConcurrentTrades(maxTrades)
}

// UpdateConsiderFunding обновляет настройку учета фандинга.
func (s *SettingsService) UpdateConsiderFunding(consider bool) error {
	return s.settingsRepo.UpdateConsiderFunding(consider)
}

// GetNotificationPrefs возвращает только настройки уведомлений.
func (s *SettingsService) GetNotificationPrefs() (*models.NotificationPreferences, error) {
	return s.settingsRepo.GetNotificationPrefs()
}

// GetMaxConcurrentTrades возвращает текущий лимит одновременных арбитражей.
// Возвращает nil, если ограничение не установлено.
func (s *SettingsService) GetMaxConcurrentTrades() (*int, error) {
	return s.settingsRepo.GetMaxConcurrentTrades()
}

// ResetToDefaults сбрасывает все настройки к значениям по умолчанию.
//
// Дефолтные значения:
// - consider_funding: false
// - max_concurrent_trades: null (без ограничений)
// - notification_prefs: все типы включены (true)
func (s *SettingsService) ResetToDefaults() error {
	return s.settingsRepo.ResetToDefaults()
}
