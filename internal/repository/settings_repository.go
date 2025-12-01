package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"time"

	"arbitrage/internal/models"
)

// Ошибки репозитория настроек
var (
	ErrSettingsNotFound = errors.New("settings not found")
)

// SettingsRepository - работа с таблицей settings
type SettingsRepository struct {
	db *sql.DB
}

// NewSettingsRepository создает новый экземпляр репозитория
func NewSettingsRepository(db *sql.DB) *SettingsRepository {
	return &SettingsRepository{db: db}
}

// Get возвращает глобальные настройки (всегда id=1, одна запись)
func (r *SettingsRepository) Get() (*models.Settings, error) {
	query := `
		SELECT id, consider_funding, max_concurrent_trades, notification_prefs, updated_at
		FROM settings
		WHERE id = 1`

	settings := &models.Settings{}
	var prefsJSON []byte
	err := r.db.QueryRow(query).Scan(
		&settings.ID,
		&settings.ConsiderFunding,
		&settings.MaxConcurrentTrades,
		&prefsJSON,
		&settings.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			// Если записи нет, создаем ее с дефолтными значениями
			return r.createDefault()
		}
		return nil, err
	}

	// Десериализуем notification_prefs из JSON
	if len(prefsJSON) > 0 {
		if err := json.Unmarshal(prefsJSON, &settings.NotificationPrefs); err != nil {
			return nil, err
		}
	} else {
		// Устанавливаем дефолтные значения
		settings.NotificationPrefs = defaultNotificationPrefs()
	}

	return settings, nil
}

// Update обновляет настройки
func (r *SettingsRepository) Update(settings *models.Settings) error {
	// Сериализуем notification_prefs в JSON
	prefsJSON, err := json.Marshal(settings.NotificationPrefs)
	if err != nil {
		return err
	}

	query := `
		UPDATE settings
		SET consider_funding = $1, max_concurrent_trades = $2, notification_prefs = $3, updated_at = $4
		WHERE id = 1`

	settings.UpdatedAt = time.Now()

	result, err := r.db.Exec(query,
		settings.ConsiderFunding,
		settings.MaxConcurrentTrades,
		prefsJSON,
		settings.UpdatedAt,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrSettingsNotFound
	}

	return nil
}

// UpdateNotificationPrefs обновляет только настройки уведомлений
func (r *SettingsRepository) UpdateNotificationPrefs(prefs models.NotificationPreferences) error {
	prefsJSON, err := json.Marshal(prefs)
	if err != nil {
		return err
	}

	query := `
		UPDATE settings
		SET notification_prefs = $1, updated_at = $2
		WHERE id = 1`

	result, err := r.db.Exec(query, prefsJSON, time.Now())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrSettingsNotFound
	}

	return nil
}

// UpdateConsiderFunding обновляет настройку учета фандинга
func (r *SettingsRepository) UpdateConsiderFunding(consider bool) error {
	query := `
		UPDATE settings
		SET consider_funding = $1, updated_at = $2
		WHERE id = 1`

	result, err := r.db.Exec(query, consider, time.Now())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrSettingsNotFound
	}

	return nil
}

// UpdateMaxConcurrentTrades обновляет лимит одновременных арбитражей
func (r *SettingsRepository) UpdateMaxConcurrentTrades(maxTrades *int) error {
	query := `
		UPDATE settings
		SET max_concurrent_trades = $1, updated_at = $2
		WHERE id = 1`

	result, err := r.db.Exec(query, maxTrades, time.Now())
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrSettingsNotFound
	}

	return nil
}

// GetNotificationPrefs возвращает только настройки уведомлений
func (r *SettingsRepository) GetNotificationPrefs() (*models.NotificationPreferences, error) {
	query := `SELECT notification_prefs FROM settings WHERE id = 1`

	var prefsJSON []byte
	err := r.db.QueryRow(query).Scan(&prefsJSON)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			prefs := defaultNotificationPrefs()
			return &prefs, nil
		}
		return nil, err
	}

	var prefs models.NotificationPreferences
	if len(prefsJSON) > 0 {
		if err := json.Unmarshal(prefsJSON, &prefs); err != nil {
			return nil, err
		}
	} else {
		prefs = defaultNotificationPrefs()
	}

	return &prefs, nil
}

// GetMaxConcurrentTrades возвращает лимит одновременных арбитражей
func (r *SettingsRepository) GetMaxConcurrentTrades() (*int, error) {
	query := `SELECT max_concurrent_trades FROM settings WHERE id = 1`

	var maxTrades *int
	err := r.db.QueryRow(query).Scan(&maxTrades)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil // null = без ограничений
		}
		return nil, err
	}

	return maxTrades, nil
}

// createDefault создает запись настроек с дефолтными значениями
func (r *SettingsRepository) createDefault() (*models.Settings, error) {
	settings := &models.Settings{
		ID:                  1,
		ConsiderFunding:     false,
		MaxConcurrentTrades: nil,
		NotificationPrefs:   defaultNotificationPrefs(),
		UpdatedAt:           time.Now(),
	}

	prefsJSON, err := json.Marshal(settings.NotificationPrefs)
	if err != nil {
		return nil, err
	}

	query := `
		INSERT INTO settings (id, consider_funding, max_concurrent_trades, notification_prefs, updated_at)
		VALUES (1, $1, $2, $3, $4)
		ON CONFLICT (id) DO NOTHING`

	_, err = r.db.Exec(query,
		settings.ConsiderFunding,
		settings.MaxConcurrentTrades,
		prefsJSON,
		settings.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}

	return settings, nil
}

// defaultNotificationPrefs возвращает дефолтные настройки уведомлений
func defaultNotificationPrefs() models.NotificationPreferences {
	return models.NotificationPreferences{
		Open:          true,
		Close:         true,
		StopLoss:      true,
		Liquidation:   true,
		APIError:      true,
		Margin:        true,
		Pause:         true,
		SecondLegFail: true,
	}
}

// ResetToDefaults сбрасывает настройки к значениям по умолчанию
func (r *SettingsRepository) ResetToDefaults() error {
	settings := &models.Settings{
		ID:                  1,
		ConsiderFunding:     false,
		MaxConcurrentTrades: nil,
		NotificationPrefs:   defaultNotificationPrefs(),
		UpdatedAt:           time.Now(),
	}

	return r.Update(settings)
}
