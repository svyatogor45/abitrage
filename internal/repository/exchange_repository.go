package repository

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"arbitrage/internal/models"
)

// Ошибки репозитория
var (
	ErrExchangeNotFound = errors.New("exchange not found")
	ErrExchangeExists   = errors.New("exchange already exists")
)

// ExchangeRepository - работа с таблицей exchanges
type ExchangeRepository struct {
	db *sql.DB
}

// NewExchangeRepository создает новый экземпляр репозитория
func NewExchangeRepository(db *sql.DB) *ExchangeRepository {
	return &ExchangeRepository{db: db}
}

// Create создает новый аккаунт биржи
func (r *ExchangeRepository) Create(account *models.ExchangeAccount) error {
	query := `
		INSERT INTO exchanges (name, api_key, secret_key, passphrase, connected, balance, last_error, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9)
		RETURNING id`

	now := time.Now()
	account.CreatedAt = now
	account.UpdatedAt = now

	err := r.db.QueryRow(
		query,
		account.Name,
		account.APIKey,
		account.SecretKey,
		account.Passphrase,
		account.Connected,
		account.Balance,
		account.LastError,
		account.CreatedAt,
		account.UpdatedAt,
	).Scan(&account.ID)

	if err != nil {
		if isUniqueViolation(err) {
			return ErrExchangeExists
		}
		return err
	}

	return nil
}

// GetByID возвращает биржу по ID
func (r *ExchangeRepository) GetByID(id int) (*models.ExchangeAccount, error) {
	query := `
		SELECT id, name, api_key, secret_key, passphrase, connected, balance, last_error, updated_at, created_at
		FROM exchanges
		WHERE id = $1`

	account := &models.ExchangeAccount{}
	err := r.db.QueryRow(query, id).Scan(
		&account.ID,
		&account.Name,
		&account.APIKey,
		&account.SecretKey,
		&account.Passphrase,
		&account.Connected,
		&account.Balance,
		&account.LastError,
		&account.UpdatedAt,
		&account.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExchangeNotFound
		}
		return nil, err
	}

	return account, nil
}

// GetByName возвращает биржу по имени (bybit, bitget, etc.)
func (r *ExchangeRepository) GetByName(name string) (*models.ExchangeAccount, error) {
	query := `
		SELECT id, name, api_key, secret_key, passphrase, connected, balance, last_error, updated_at, created_at
		FROM exchanges
		WHERE name = $1`

	account := &models.ExchangeAccount{}
	err := r.db.QueryRow(query, name).Scan(
		&account.ID,
		&account.Name,
		&account.APIKey,
		&account.SecretKey,
		&account.Passphrase,
		&account.Connected,
		&account.Balance,
		&account.LastError,
		&account.UpdatedAt,
		&account.CreatedAt,
	)

	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrExchangeNotFound
		}
		return nil, err
	}

	return account, nil
}

// GetAll возвращает все биржи
func (r *ExchangeRepository) GetAll() ([]*models.ExchangeAccount, error) {
	query := `
		SELECT id, name, api_key, secret_key, passphrase, connected, balance, last_error, updated_at, created_at
		FROM exchanges
		ORDER BY name`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*models.ExchangeAccount
	for rows.Next() {
		account := &models.ExchangeAccount{}
		err := rows.Scan(
			&account.ID,
			&account.Name,
			&account.APIKey,
			&account.SecretKey,
			&account.Passphrase,
			&account.Connected,
			&account.Balance,
			&account.LastError,
			&account.UpdatedAt,
			&account.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return accounts, nil
}

// GetConnected возвращает все подключенные биржи
func (r *ExchangeRepository) GetConnected() ([]*models.ExchangeAccount, error) {
	query := `
		SELECT id, name, api_key, secret_key, passphrase, connected, balance, last_error, updated_at, created_at
		FROM exchanges
		WHERE connected = true
		ORDER BY name`

	rows, err := r.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var accounts []*models.ExchangeAccount
	for rows.Next() {
		account := &models.ExchangeAccount{}
		err := rows.Scan(
			&account.ID,
			&account.Name,
			&account.APIKey,
			&account.SecretKey,
			&account.Passphrase,
			&account.Connected,
			&account.Balance,
			&account.LastError,
			&account.UpdatedAt,
			&account.CreatedAt,
		)
		if err != nil {
			return nil, err
		}
		accounts = append(accounts, account)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	return accounts, nil
}

// Update обновляет данные аккаунта биржи
func (r *ExchangeRepository) Update(account *models.ExchangeAccount) error {
	query := `
		UPDATE exchanges
		SET api_key = $1, secret_key = $2, passphrase = $3, connected = $4, balance = $5, last_error = $6, updated_at = $7
		WHERE id = $8`

	account.UpdatedAt = time.Now()

	result, err := r.db.Exec(
		query,
		account.APIKey,
		account.SecretKey,
		account.Passphrase,
		account.Connected,
		account.Balance,
		account.LastError,
		account.UpdatedAt,
		account.ID,
	)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrExchangeNotFound
	}

	return nil
}

// Delete удаляет аккаунт биржи
func (r *ExchangeRepository) Delete(id int) error {
	query := `DELETE FROM exchanges WHERE id = $1`

	result, err := r.db.Exec(query, id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrExchangeNotFound
	}

	return nil
}

// DeleteByName удаляет аккаунт биржи по имени
func (r *ExchangeRepository) DeleteByName(name string) error {
	query := `DELETE FROM exchanges WHERE name = $1`

	result, err := r.db.Exec(query, name)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrExchangeNotFound
	}

	return nil
}

// UpdateBalance обновляет баланс биржи
func (r *ExchangeRepository) UpdateBalance(id int, balance float64) error {
	query := `
		UPDATE exchanges
		SET balance = $1, updated_at = $2
		WHERE id = $3`

	result, err := r.db.Exec(query, balance, time.Now(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrExchangeNotFound
	}

	return nil
}

// UpdateBalanceByName обновляет баланс биржи по имени
func (r *ExchangeRepository) UpdateBalanceByName(name string, balance float64) error {
	query := `
		UPDATE exchanges
		SET balance = $1, updated_at = $2
		WHERE name = $3`

	result, err := r.db.Exec(query, balance, time.Now(), name)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrExchangeNotFound
	}

	return nil
}

// SetConnected обновляет статус подключения биржи
func (r *ExchangeRepository) SetConnected(id int, connected bool) error {
	query := `
		UPDATE exchanges
		SET connected = $1, updated_at = $2
		WHERE id = $3`

	result, err := r.db.Exec(query, connected, time.Now(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrExchangeNotFound
	}

	return nil
}

// SetLastError обновляет последнюю ошибку биржи
func (r *ExchangeRepository) SetLastError(id int, lastError string) error {
	query := `
		UPDATE exchanges
		SET last_error = $1, updated_at = $2
		WHERE id = $3`

	result, err := r.db.Exec(query, lastError, time.Now(), id)
	if err != nil {
		return err
	}

	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return err
	}

	if rowsAffected == 0 {
		return ErrExchangeNotFound
	}

	return nil
}

// CountConnected возвращает количество подключенных бирж
func (r *ExchangeRepository) CountConnected() (int, error) {
	query := `SELECT COUNT(*) FROM exchanges WHERE connected = true`

	var count int
	err := r.db.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, err
	}

	return count, nil
}

// isUniqueViolation проверяет, является ли ошибка нарушением UNIQUE constraint
func isUniqueViolation(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	return strings.Contains(errStr, "duplicate key") || strings.Contains(errStr, "23505")
}
