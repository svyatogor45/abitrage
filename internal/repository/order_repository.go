package repository

import (
	"database/sql"
	"errors"
	"time"

	"arbitrage/internal/models"
)

// Ошибки репозитория ордеров
var (
	ErrOrderNotFound = errors.New("order not found")
)

// OrderRepository - работа с таблицей orders
type OrderRepository struct {
	db *sql.DB
}

// NewOrderRepository создает новый экземпляр репозитория
func NewOrderRepository(db *sql.DB) *OrderRepository {
	return &OrderRepository{db: db}
}

// Create создает запись об ордере
func (r *OrderRepository) Create(order *models.OrderRecord) error {
	query := `
		INSERT INTO orders (pair_id, exchange, side, type, part_index, quantity, price_avg, status, error_message, created_at, filled_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
		RETURNING id`

	order.CreatedAt = time.Now()

	err := r.db.QueryRow(
		query,
		order.PairID,
		order.Exchange,
		order.Side,
		order.Type,
		order.PartIndex,
		order.Quantity,
		order.PriceAvg,
		order.Status,
		order.ErrorMessage,
		order.CreatedAt,
		order.FilledAt,
	).Scan(&order.ID)

	if err != nil {
		return err
	}

	return nil
}

// GetByID возвращает ордер по ID
func (r *OrderRepository) GetByID(id int) (*models.OrderRecord, error) {
	query := `
		SELECT id, pair_id, exchange, side, type, part_index, quantity, price_avg, status, error_message, created_at, filled_at
		FROM orders
		WHERE id = $1`

	order := &models.OrderRecord{}
	err := r.db.QueryRow(query, id).Scan(
		&order.ID,
		&order.PairID,
		&order.Exchange,
		&order.Side,
		&order.Type,
		&order.PartIndex,
		&order.Quantity,
		&order.PriceAvg,
		&order.Status,
		&order.ErrorMessage,
		&order.CreatedAt,
		&order.FilledAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrOrderNotFound
		}
		return nil, err
	}

	return order, nil
}

// GetByPairID возвращает все ордера для конкретной пары
func (r *OrderRepository) GetByPairID(pairID int) ([]*models.OrderRecord, error) {
	query := `
		SELECT id, pair_id, exchange, side, type, part_index, quantity, price_avg, status, error_message, created_at, filled_at
		FROM orders
		WHERE pair_id = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query, pairID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*models.OrderRecord
	for rows.Next() {
		order := &models.OrderRecord{}
		err := rows.Scan(
			&order.ID,
			&order.PairID,
			&order.Exchange,
			&order.Side,
			&order.Type,
			&order.PartIndex,
			&order.Quantity,
			&order.PriceAvg,
			&order.Status,
			&order.ErrorMessage,
			&order.CreatedAt,
			&order.FilledAt,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// GetRecent возвращает последние N ордеров
func (r *OrderRepository) GetRecent(limit int) ([]*models.OrderRecord, error) {
	query := `
		SELECT id, pair_id, exchange, side, type, part_index, quantity, price_avg, status, error_message, created_at, filled_at
		FROM orders
		ORDER BY created_at DESC
		LIMIT $1`

	rows, err := r.db.Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*models.OrderRecord
	for rows.Next() {
		order := &models.OrderRecord{}
		err := rows.Scan(
			&order.ID,
			&order.PairID,
			&order.Exchange,
			&order.Side,
			&order.Type,
			&order.PartIndex,
			&order.Quantity,
			&order.PriceAvg,
			&order.Status,
			&order.ErrorMessage,
			&order.CreatedAt,
			&order.FilledAt,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// GetByStatus возвращает ордера с определенным статусом
func (r *OrderRepository) GetByStatus(status string) ([]*models.OrderRecord, error) {
	query := `
		SELECT id, pair_id, exchange, side, type, part_index, quantity, price_avg, status, error_message, created_at, filled_at
		FROM orders
		WHERE status = $1
		ORDER BY created_at DESC`

	rows, err := r.db.Query(query, status)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*models.OrderRecord
	for rows.Next() {
		order := &models.OrderRecord{}
		err := rows.Scan(
			&order.ID,
			&order.PairID,
			&order.Exchange,
			&order.Side,
			&order.Type,
			&order.PartIndex,
			&order.Quantity,
			&order.PriceAvg,
			&order.Status,
			&order.ErrorMessage,
			&order.CreatedAt,
			&order.FilledAt,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// GetByExchange возвращает ордера для конкретной биржи
func (r *OrderRepository) GetByExchange(exchange string, limit int) ([]*models.OrderRecord, error) {
	query := `
		SELECT id, pair_id, exchange, side, type, part_index, quantity, price_avg, status, error_message, created_at, filled_at
		FROM orders
		WHERE exchange = $1
		ORDER BY created_at DESC
		LIMIT $2`

	rows, err := r.db.Query(query, exchange, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*models.OrderRecord
	for rows.Next() {
		order := &models.OrderRecord{}
		err := rows.Scan(
			&order.ID,
			&order.PairID,
			&order.Exchange,
			&order.Side,
			&order.Type,
			&order.PartIndex,
			&order.Quantity,
			&order.PriceAvg,
			&order.Status,
			&order.ErrorMessage,
			&order.CreatedAt,
			&order.FilledAt,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}

// UpdateStatus обновляет статус ордера
func (r *OrderRepository) UpdateStatus(id int, status string, priceAvg float64, filledAt *time.Time) error {
	query := `
		UPDATE orders
		SET status = $1, price_avg = $2, filled_at = $3
		WHERE id = $4`

	result, err := r.db.Exec(query, status, priceAvg, filledAt, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrOrderNotFound
	}

	return nil
}

// SetError устанавливает сообщение об ошибке для ордера
func (r *OrderRepository) SetError(id int, errorMessage string) error {
	query := `
		UPDATE orders
		SET error_message = $1, status = $2
		WHERE id = $3`

	result, err := r.db.Exec(query, errorMessage, models.OrderStatusRejected, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrOrderNotFound
	}

	return nil
}

// Delete удаляет ордер
func (r *OrderRepository) Delete(id int) error {
	query := `DELETE FROM orders WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrOrderNotFound
	}

	return nil
}

// DeleteByPairID удаляет все ордера для пары
func (r *OrderRepository) DeleteByPairID(pairID int) error {
	query := `DELETE FROM orders WHERE pair_id = $1`

	_, err := r.db.Exec(query, pairID)
	return err
}

// DeleteOlderThan удаляет ордера старше указанной даты
func (r *OrderRepository) DeleteOlderThan(timestamp time.Time) (int64, error) {
	query := `DELETE FROM orders WHERE created_at < $1`

	result, err := r.db.Exec(query, timestamp)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

// Count возвращает общее количество ордеров
func (r *OrderRepository) Count() (int, error) {
	query := `SELECT COUNT(*) FROM orders`

	var count int
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// CountByStatus возвращает количество ордеров с определенным статусом
func (r *OrderRepository) CountByStatus(status string) (int, error) {
	query := `SELECT COUNT(*) FROM orders WHERE status = $1`

	var count int
	err := r.db.QueryRow(query, status).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// GetFilledByPairIDInTimeRange возвращает исполненные ордера пары за период
func (r *OrderRepository) GetFilledByPairIDInTimeRange(pairID int, from, to time.Time) ([]*models.OrderRecord, error) {
	query := `
		SELECT id, pair_id, exchange, side, type, part_index, quantity, price_avg, status, error_message, created_at, filled_at
		FROM orders
		WHERE pair_id = $1 AND status = $2 AND filled_at >= $3 AND filled_at <= $4
		ORDER BY filled_at DESC`

	rows, err := r.db.Query(query, pairID, models.OrderStatusFilled, from, to)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var orders []*models.OrderRecord
	for rows.Next() {
		order := &models.OrderRecord{}
		err := rows.Scan(
			&order.ID,
			&order.PairID,
			&order.Exchange,
			&order.Side,
			&order.Type,
			&order.PartIndex,
			&order.Quantity,
			&order.PriceAvg,
			&order.Status,
			&order.ErrorMessage,
			&order.CreatedAt,
			&order.FilledAt,
		)
		if err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return orders, nil
}
