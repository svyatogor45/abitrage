package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"arbitrage/internal/models"
)

// Ошибки репозитория черного списка
var (
	ErrBlacklistEntryNotFound = errors.New("blacklist entry not found")
	ErrBlacklistEntryExists   = errors.New("symbol already in blacklist")
)

// BlacklistRepository - работа с таблицей blacklist
type BlacklistRepository struct {
	db *sql.DB
}

// NewBlacklistRepository создает новый экземпляр репозитория
func NewBlacklistRepository(db *sql.DB) *BlacklistRepository {
	return &BlacklistRepository{db: db}
}

// Create добавляет пару в черный список
func (r *BlacklistRepository) Create(entry *models.BlacklistEntry) error {
	query := `
		INSERT INTO blacklist (symbol, reason, created_at)
		VALUES ($1, $2, $3)
		RETURNING id`

	entry.CreatedAt = time.Now()

	err := r.db.QueryRow(
		query,
		strings.ToUpper(entry.Symbol), // Приводим к верхнему регистру для консистентности
		entry.Reason,
		entry.CreatedAt,
	).Scan(&entry.ID)

	if err != nil {
		if isBlacklistUniqueViolation(err) {
			return ErrBlacklistEntryExists
		}
		return err
	}

	return nil
}

// GetAll возвращает весь черный список
func (r *BlacklistRepository) GetAll() ([]*models.BlacklistEntry, error) {
	query := `
		SELECT id, symbol, reason, created_at
		FROM blacklist
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.BlacklistEntry
	for rows.Next() {
		entry := &models.BlacklistEntry{}
		err := rows.Scan(
			&entry.ID,
			&entry.Symbol,
			&entry.Reason,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// GetByID возвращает запись по ID
func (r *BlacklistRepository) GetByID(id int) (*models.BlacklistEntry, error) {
	query := `
		SELECT id, symbol, reason, created_at
		FROM blacklist
		WHERE id = $1`

	entry := &models.BlacklistEntry{}
	err := r.db.QueryRow(query, id).Scan(
		&entry.ID,
		&entry.Symbol,
		&entry.Reason,
		&entry.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBlacklistEntryNotFound
		}
		return nil, err
	}

	return entry, nil
}

// GetBySymbol возвращает запись по символу
func (r *BlacklistRepository) GetBySymbol(symbol string) (*models.BlacklistEntry, error) {
	query := `
		SELECT id, symbol, reason, created_at
		FROM blacklist
		WHERE symbol = $1`

	entry := &models.BlacklistEntry{}
	err := r.db.QueryRow(query, strings.ToUpper(symbol)).Scan(
		&entry.ID,
		&entry.Symbol,
		&entry.Reason,
		&entry.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrBlacklistEntryNotFound
		}
		return nil, err
	}

	return entry, nil
}

// Delete удаляет пару из черного списка по символу
func (r *BlacklistRepository) Delete(symbol string) error {
	query := `DELETE FROM blacklist WHERE symbol = $1`

	result, err := r.db.Exec(query, strings.ToUpper(symbol))
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrBlacklistEntryNotFound
	}

	return nil
}

// DeleteByID удаляет запись по ID
func (r *BlacklistRepository) DeleteByID(id int) error {
	query := `DELETE FROM blacklist WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrBlacklistEntryNotFound
	}

	return nil
}

// Exists проверяет наличие пары в черном списке
func (r *BlacklistRepository) Exists(symbol string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM blacklist WHERE symbol = $1)`

	var exists bool
	err := r.db.QueryRow(query, strings.ToUpper(symbol)).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// UpdateReason обновляет причину добавления в черный список
func (r *BlacklistRepository) UpdateReason(symbol string, reason string) error {
	query := `
		UPDATE blacklist
		SET reason = $1
		WHERE symbol = $2`

	result, err := r.db.Exec(query, reason, strings.ToUpper(symbol))
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrBlacklistEntryNotFound
	}

	return nil
}

// Count возвращает количество записей в черном списке
func (r *BlacklistRepository) Count() (int, error) {
	query := `SELECT COUNT(*) FROM blacklist`

	var count int
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// DeleteAll очищает весь черный список
func (r *BlacklistRepository) DeleteAll() error {
	query := `DELETE FROM blacklist`
	_, err := r.db.Exec(query)
	return err
}

// Search ищет записи по части символа
func (r *BlacklistRepository) Search(query string) ([]*models.BlacklistEntry, error) {
	sqlQuery := `
		SELECT id, symbol, reason, created_at
		FROM blacklist
		WHERE UPPER(symbol) LIKE UPPER($1)
		ORDER BY symbol`

	searchPattern := "%" + query + "%"
	rows, err := r.db.Query(sqlQuery, searchPattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var entries []*models.BlacklistEntry
	for rows.Next() {
		entry := &models.BlacklistEntry{}
		err := rows.Scan(
			&entry.ID,
			&entry.Symbol,
			&entry.Reason,
			&entry.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		entries = append(entries, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return entries, nil
}

// isBlacklistUniqueViolation проверяет, является ли ошибка нарушением UNIQUE constraint
func isBlacklistUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "23505")
}
