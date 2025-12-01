package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"arbitrage/internal/models"
)

// Ошибки репозитория пар
var (
	ErrPairNotFound = errors.New("pair not found")
	ErrPairExists   = errors.New("pair already exists")
)

// PairRepository - работа с таблицей pairs
type PairRepository struct {
	db *sql.DB
}

// NewPairRepository создает новый экземпляр репозитория
func NewPairRepository(db *sql.DB) *PairRepository {
	return &PairRepository{db: db}
}

// Create создает новую торговую пару
func (r *PairRepository) Create(pair *models.PairConfig) error {
	query := `
		INSERT INTO pairs (symbol, base, quote, entry_spread_pct, exit_spread_pct, volume_asset, n_orders, stop_loss, status, trades_count, total_pnl, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13)
		RETURNING id`

	now := time.Now()
	pair.CreatedAt = now
	pair.UpdatedAt = now

	// Устанавливаем значения по умолчанию
	if pair.Status == "" {
		pair.Status = models.PairStatusPaused
	}
	if pair.NOrders == 0 {
		pair.NOrders = 1
	}

	err := r.db.QueryRow(
		query,
		pair.Symbol,
		pair.Base,
		pair.Quote,
		pair.EntrySpreadPct,
		pair.ExitSpreadPct,
		pair.VolumeAsset,
		pair.NOrders,
		pair.StopLoss,
		pair.Status,
		pair.TradesCount,
		pair.TotalPnl,
		pair.CreatedAt,
		pair.UpdatedAt,
	).Scan(&pair.ID)

	if err != nil {
		if isPairUniqueViolation(err) {
			return ErrPairExists
		}
		return err
	}

	return nil
}

// GetByID возвращает пару по ID
func (r *PairRepository) GetByID(id int) (*models.PairConfig, error) {
	query := `
		SELECT id, symbol, base, quote, entry_spread_pct, exit_spread_pct, volume_asset, n_orders, stop_loss, status, trades_count, total_pnl, created_at, updated_at
		FROM pairs
		WHERE id = $1`

	pair := &models.PairConfig{}
	err := r.db.QueryRow(query, id).Scan(
		&pair.ID,
		&pair.Symbol,
		&pair.Base,
		&pair.Quote,
		&pair.EntrySpreadPct,
		&pair.ExitSpreadPct,
		&pair.VolumeAsset,
		&pair.NOrders,
		&pair.StopLoss,
		&pair.Status,
		&pair.TradesCount,
		&pair.TotalPnl,
		&pair.CreatedAt,
		&pair.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPairNotFound
		}
		return nil, err
	}

	return pair, nil
}

// GetBySymbol возвращает пару по символу
func (r *PairRepository) GetBySymbol(symbol string) (*models.PairConfig, error) {
	query := `
		SELECT id, symbol, base, quote, entry_spread_pct, exit_spread_pct, volume_asset, n_orders, stop_loss, status, trades_count, total_pnl, created_at, updated_at
		FROM pairs
		WHERE symbol = $1`

	pair := &models.PairConfig{}
	err := r.db.QueryRow(query, symbol).Scan(
		&pair.ID,
		&pair.Symbol,
		&pair.Base,
		&pair.Quote,
		&pair.EntrySpreadPct,
		&pair.ExitSpreadPct,
		&pair.VolumeAsset,
		&pair.NOrders,
		&pair.StopLoss,
		&pair.Status,
		&pair.TradesCount,
		&pair.TotalPnl,
		&pair.CreatedAt,
		&pair.UpdatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrPairNotFound
		}
		return nil, err
	}

	return pair, nil
}

// GetAll возвращает все пары
func (r *PairRepository) GetAll() ([]*models.PairConfig, error) {
	query := `
		SELECT id, symbol, base, quote, entry_spread_pct, exit_spread_pct, volume_asset, n_orders, stop_loss, status, trades_count, total_pnl, created_at, updated_at
		FROM pairs
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []*models.PairConfig
	for rows.Next() {
		pair := &models.PairConfig{}
		err := rows.Scan(
			&pair.ID,
			&pair.Symbol,
			&pair.Base,
			&pair.Quote,
			&pair.EntrySpreadPct,
			&pair.ExitSpreadPct,
			&pair.VolumeAsset,
			&pair.NOrders,
			&pair.StopLoss,
			&pair.Status,
			&pair.TradesCount,
			&pair.TotalPnl,
			&pair.CreatedAt,
			&pair.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, pair)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return pairs, nil
}

// GetActive возвращает только активные пары
func (r *PairRepository) GetActive() ([]*models.PairConfig, error) {
	query := `
		SELECT id, symbol, base, quote, entry_spread_pct, exit_spread_pct, volume_asset, n_orders, stop_loss, status, trades_count, total_pnl, created_at, updated_at
		FROM pairs
		WHERE status = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query, models.PairStatusActive)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []*models.PairConfig
	for rows.Next() {
		pair := &models.PairConfig{}
		err := rows.Scan(
			&pair.ID,
			&pair.Symbol,
			&pair.Base,
			&pair.Quote,
			&pair.EntrySpreadPct,
			&pair.ExitSpreadPct,
			&pair.VolumeAsset,
			&pair.NOrders,
			&pair.StopLoss,
			&pair.Status,
			&pair.TradesCount,
			&pair.TotalPnl,
			&pair.CreatedAt,
			&pair.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, pair)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return pairs, nil
}

// GetPaused возвращает только приостановленные пары
func (r *PairRepository) GetPaused() ([]*models.PairConfig, error) {
	query := `
		SELECT id, symbol, base, quote, entry_spread_pct, exit_spread_pct, volume_asset, n_orders, stop_loss, status, trades_count, total_pnl, created_at, updated_at
		FROM pairs
		WHERE status = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query, models.PairStatusPaused)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []*models.PairConfig
	for rows.Next() {
		pair := &models.PairConfig{}
		err := rows.Scan(
			&pair.ID,
			&pair.Symbol,
			&pair.Base,
			&pair.Quote,
			&pair.EntrySpreadPct,
			&pair.ExitSpreadPct,
			&pair.VolumeAsset,
			&pair.NOrders,
			&pair.StopLoss,
			&pair.Status,
			&pair.TradesCount,
			&pair.TotalPnl,
			&pair.CreatedAt,
			&pair.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, pair)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return pairs, nil
}

// Update обновляет параметры пары
func (r *PairRepository) Update(pair *models.PairConfig) error {
	query := `
		UPDATE pairs
		SET symbol = $1, base = $2, quote = $3, entry_spread_pct = $4, exit_spread_pct = $5, volume_asset = $6, n_orders = $7, stop_loss = $8, status = $9, trades_count = $10, total_pnl = $11, updated_at = $12
		WHERE id = $13`

	pair.UpdatedAt = time.Now()

	result, err := r.db.Exec(
		query,
		pair.Symbol,
		pair.Base,
		pair.Quote,
		pair.EntrySpreadPct,
		pair.ExitSpreadPct,
		pair.VolumeAsset,
		pair.NOrders,
		pair.StopLoss,
		pair.Status,
		pair.TradesCount,
		pair.TotalPnl,
		pair.UpdatedAt,
		pair.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrPairNotFound
	}

	return nil
}

// UpdateParams обновляет только торговые параметры пары (без статуса и статистики)
func (r *PairRepository) UpdateParams(id int, entrySpread, exitSpread, volume float64, nOrders int, stopLoss float64) error {
	query := `
		UPDATE pairs
		SET entry_spread_pct = $1, exit_spread_pct = $2, volume_asset = $3, n_orders = $4, stop_loss = $5, updated_at = $6
		WHERE id = $7`

	result, err := r.db.Exec(query, entrySpread, exitSpread, volume, nOrders, stopLoss, time.Now(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrPairNotFound
	}

	return nil
}

// Delete удаляет пару
func (r *PairRepository) Delete(id int) error {
	query := `DELETE FROM pairs WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrPairNotFound
	}

	return nil
}

// UpdateStatus обновляет статус пары (paused/active)
func (r *PairRepository) UpdateStatus(id int, status string) error {
	// Валидация статуса
	if status != models.PairStatusPaused && status != models.PairStatusActive {
		return errors.New("invalid status: must be 'paused' or 'active'")
	}

	query := `
		UPDATE pairs
		SET status = $1, updated_at = $2
		WHERE id = $3`

	result, err := r.db.Exec(query, status, time.Now(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrPairNotFound
	}

	return nil
}

// IncrementTrades увеличивает счетчик сделок на 1
func (r *PairRepository) IncrementTrades(id int) error {
	query := `
		UPDATE pairs
		SET trades_count = trades_count + 1, updated_at = $1
		WHERE id = $2`

	result, err := r.db.Exec(query, time.Now(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrPairNotFound
	}

	return nil
}

// UpdatePnl добавляет значение к локальному PNL пары
func (r *PairRepository) UpdatePnl(id int, pnlDelta float64) error {
	query := `
		UPDATE pairs
		SET total_pnl = total_pnl + $1, updated_at = $2
		WHERE id = $3`

	result, err := r.db.Exec(query, pnlDelta, time.Now(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrPairNotFound
	}

	return nil
}

// ResetStats сбрасывает локальную статистику пары
func (r *PairRepository) ResetStats(id int) error {
	query := `
		UPDATE pairs
		SET trades_count = 0, total_pnl = 0, updated_at = $1
		WHERE id = $2`

	result, err := r.db.Exec(query, time.Now(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrPairNotFound
	}

	return nil
}

// Count возвращает общее количество пар
func (r *PairRepository) Count() (int, error) {
	query := `SELECT COUNT(*) FROM pairs`

	var count int
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountActive возвращает количество активных пар
func (r *PairRepository) CountActive() (int, error) {
	query := `SELECT COUNT(*) FROM pairs WHERE status = $1`

	var count int
	err := r.db.QueryRow(query, models.PairStatusActive).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// ExistsBySymbol проверяет существование пары по символу
func (r *PairRepository) ExistsBySymbol(symbol string) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM pairs WHERE symbol = $1)`

	var exists bool
	err := r.db.QueryRow(query, symbol).Scan(&exists)
	if err != nil {
		return false, err
	}

	return exists, nil
}

// Search ищет пары по части символа
func (r *PairRepository) Search(searchQuery string) ([]*models.PairConfig, error) {
	query := `
		SELECT id, symbol, base, quote, entry_spread_pct, exit_spread_pct, volume_asset, n_orders, stop_loss, status, trades_count, total_pnl, created_at, updated_at
		FROM pairs
		WHERE LOWER(symbol) LIKE LOWER($1) OR LOWER(base) LIKE LOWER($2)
		ORDER BY symbol`

	searchPattern := "%" + searchQuery + "%"
	rows, err := r.db.Query(query, searchPattern, searchPattern)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var pairs []*models.PairConfig
	for rows.Next() {
		pair := &models.PairConfig{}
		err := rows.Scan(
			&pair.ID,
			&pair.Symbol,
			&pair.Base,
			&pair.Quote,
			&pair.EntrySpreadPct,
			&pair.ExitSpreadPct,
			&pair.VolumeAsset,
			&pair.NOrders,
			&pair.StopLoss,
			&pair.Status,
			&pair.TradesCount,
			&pair.TotalPnl,
			&pair.CreatedAt,
			&pair.UpdatedAt,
		)
		if err != nil {
			return nil, err
		}
		pairs = append(pairs, pair)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return pairs, nil
}

// isPairUniqueViolation проверяет, является ли ошибка нарушением UNIQUE constraint
func isPairUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "23505")
}
