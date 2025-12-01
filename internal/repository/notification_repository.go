package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"arbitrage/internal/models"
)

// Ошибки репозитория уведомлений
var (
	ErrNotificationNotFound = errors.New("notification not found")
)

// NotificationRepository - работа с таблицей notifications
type NotificationRepository struct {
	db *sql.DB
}

// NewNotificationRepository создает новый экземпляр репозитория
func NewNotificationRepository(db *sql.DB) *NotificationRepository {
	return &NotificationRepository{db: db}
}

// Create создает новое уведомление
func (r *NotificationRepository) Create(notif *models.Notification) error {
	query := `
		INSERT INTO notifications (timestamp, type, severity, pair_id, message, meta)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id`

	notif.Timestamp = time.Now()

	// Сериализуем meta в JSON
	var metaJSON []byte
	var err error
	if notif.Meta != nil {
		metaJSON, err = json.Marshal(notif.Meta)
		if err != nil {
			return err
		}
	}

	err = r.db.QueryRow(
		query,
		notif.Timestamp,
		notif.Type,
		notif.Severity,
		notif.PairID,
		notif.Message,
		metaJSON,
	).Scan(&notif.ID)

	if err != nil {
		return err
	}

	return nil
}

// GetByID возвращает уведомление по ID
func (r *NotificationRepository) GetByID(id int) (*models.Notification, error) {
	query := `
		SELECT id, timestamp, type, severity, pair_id, message, meta
		FROM notifications
		WHERE id = $1`

	notif := &models.Notification{}
	var metaJSON []byte
	err := r.db.QueryRow(query, id).Scan(
		&notif.ID,
		&notif.Timestamp,
		&notif.Type,
		&notif.Severity,
		&notif.PairID,
		&notif.Message,
		&metaJSON,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotificationNotFound
		}
		return nil, err
	}

	// Десериализуем meta из JSON
	if len(metaJSON) > 0 {
		if err := json.Unmarshal(metaJSON, &notif.Meta); err != nil {
			return nil, err
		}
	}

	return notif, nil
}

// GetRecent возвращает последние N уведомлений
func (r *NotificationRepository) GetRecent(limit int) ([]*models.Notification, error) {
	query := `
		SELECT id, timestamp, type, severity, pair_id, message, meta
		FROM notifications
		ORDER BY timestamp DESC
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanNotifications(rows)
}

// GetByTypes возвращает уведомления определенных типов
func (r *NotificationRepository) GetByTypes(types []string, limit int) ([]*models.Notification, error) {
	if len(types) == 0 {
		return r.GetRecent(limit)
	}

	// Создаем плейсхолдеры для IN clause
	placeholders := make([]string, len(types))
	args := make([]interface{}, len(types)+1)
	for i, t := range types {
		placeholders[i] = "$" + string(rune('1'+i))
		args[i] = t
	}
	args[len(types)] = limit

	query := `
		SELECT id, timestamp, type, severity, pair_id, message, meta
		FROM notifications
		WHERE type IN (` + strings.Join(placeholders, ",") + `)
		ORDER BY timestamp DESC
		LIMIT $` + string(rune('1'+len(types)))

	rows, err := r.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanNotifications(rows)
}

// GetByPairID возвращает уведомления для конкретной пары
func (r *NotificationRepository) GetByPairID(pairID int, limit int) ([]*models.Notification, error) {
	query := `
		SELECT id, timestamp, type, severity, pair_id, message, meta
		FROM notifications
		WHERE pair_id = $1
		ORDER BY timestamp DESC
		LIMIT $2`

	rows, err := r.db.Query(query, pairID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanNotifications(rows)
}

// GetBySeverity возвращает уведомления определенной важности
func (r *NotificationRepository) GetBySeverity(severity string, limit int) ([]*models.Notification, error) {
	query := `
		SELECT id, timestamp, type, severity, pair_id, message, meta
		FROM notifications
		WHERE severity = $1
		ORDER BY timestamp DESC
		LIMIT $2`

	rows, err := r.db.Query(query, severity, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanNotifications(rows)
}

// GetInTimeRange возвращает уведомления за период
func (r *NotificationRepository) GetInTimeRange(from, to time.Time, limit int) ([]*models.Notification, error) {
	query := `
		SELECT id, timestamp, type, severity, pair_id, message, meta
		FROM notifications
		WHERE timestamp >= $1 AND timestamp <= $2
		ORDER BY timestamp DESC
		LIMIT $3`

	rows, err := r.db.Query(query, from, to, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanNotifications(rows)
}

// DeleteAll очищает журнал уведомлений
func (r *NotificationRepository) DeleteAll() error {
	query := `DELETE FROM notifications`
	_, err := r.db.Exec(query)
	return err
}

// DeleteOlderThan удаляет уведомления старше указанной даты
func (r *NotificationRepository) DeleteOlderThan(timestamp time.Time) (int64, error) {
	query := `DELETE FROM notifications WHERE timestamp < $1`

	result, err := r.db.Exec(query, timestamp)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// DeleteByPairID удаляет все уведомления для пары
func (r *NotificationRepository) DeleteByPairID(pairID int) error {
	query := `DELETE FROM notifications WHERE pair_id = $1`
	_, err := r.db.Exec(query, pairID)
	return err
}

// Count возвращает общее количество уведомлений
func (r *NotificationRepository) Count() (int, error) {
	query := `SELECT COUNT(*) FROM notifications`

	var count int
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountByType возвращает количество уведомлений определенного типа
func (r *NotificationRepository) CountByType(notifType string) (int, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE type = $1`

	var count int
	err := r.db.QueryRow(query, notifType).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountBySeverity возвращает количество уведомлений определенной важности
func (r *NotificationRepository) CountBySeverity(severity string) (int, error) {
	query := `SELECT COUNT(*) FROM notifications WHERE severity = $1`

	var count int
	err := r.db.QueryRow(query, severity).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// KeepRecent оставляет только последние N уведомлений, удаляя остальные
func (r *NotificationRepository) KeepRecent(keep int) (int64, error) {
	query := `
		DELETE FROM notifications
		WHERE id NOT IN (
			SELECT id FROM notifications
			ORDER BY timestamp DESC
			LIMIT $1
		)`

	result, err := r.db.Exec(query, keep)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// scanNotifications сканирует строки результата в слайс уведомлений
func (r *NotificationRepository) scanNotifications(rows *sql.Rows) ([]*models.Notification, error) {
	var notifications []*models.Notification
	for rows.Next() {
		notif := &models.Notification{}
		var metaJSON []byte
		err := rows.Scan(
			&notif.ID,
			&notif.Timestamp,
			&notif.Type,
			&notif.Severity,
			&notif.PairID,
			&notif.Message,
			&metaJSON,
		)
		if err != nil {
			return nil, err
		}

		// Десериализуем meta из JSON
		if len(metaJSON) > 0 {
			if err := json.Unmarshal(metaJSON, &notif.Meta); err != nil {
				return nil, err
			}
		}

		notifications = append(notifications, notif)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	return notifications, nil
}
