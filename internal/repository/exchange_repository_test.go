package repository

import (
	"database/sql"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"arbitrage/internal/models"
)

// ============================================================
// ExchangeRepository Tests
// ============================================================

func TestNewExchangeRepository(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewExchangeRepository(db)
	if repo == nil {
		t.Fatal("NewExchangeRepository returned nil")
	}
	if repo.db != db {
		t.Error("db not set correctly")
	}
}

func TestExchangeRepositoryCreate(t *testing.T) {
	tests := []struct {
		name        string
		account     *models.ExchangeAccount
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name: "success",
			account: &models.ExchangeAccount{
				Name:       "bybit",
				APIKey:     "test-api-key",
				SecretKey:  "test-secret-key",
				Passphrase: "",
				Connected:  false,
				Balance:    0,
				LastError:  "",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO exchanges`).
					WithArgs("bybit", "test-api-key", "test-secret-key", "", false, float64(0), "", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			expectError: nil,
		},
		{
			name: "duplicate key error",
			account: &models.ExchangeAccount{
				Name:      "bybit",
				APIKey:    "test-api-key",
				SecretKey: "test-secret-key",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO exchanges`).
					WithArgs("bybit", "test-api-key", "test-secret-key", "", false, float64(0), "", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(errors.New("duplicate key value violates unique constraint"))
			},
			expectError: ErrExchangeExists,
		},
		{
			name: "database error",
			account: &models.ExchangeAccount{
				Name:      "okx",
				APIKey:    "api",
				SecretKey: "secret",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO exchanges`).
					WithArgs("okx", "api", "secret", "", false, float64(0), "", sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(errors.New("connection refused"))
			},
			expectError: errors.New("connection refused"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			repo := NewExchangeRepository(db)
			err = repo.Create(tt.account)

			if tt.expectError != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectError)
				} else if tt.expectError == ErrExchangeExists && !errors.Is(err, ErrExchangeExists) {
					t.Errorf("expected ErrExchangeExists, got %v", err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.account.ID != 1 {
					t.Errorf("expected ID=1, got %d", tt.account.ID)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestExchangeRepositoryGetByID(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		id          int
		mockSetup   func(mock sqlmock.Sqlmock)
		expected    *models.ExchangeAccount
		expectError error
	}{
		{
			name: "success",
			id:   1,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "api_key", "secret_key", "passphrase", "connected", "balance", "last_error", "updated_at", "created_at"}).
					AddRow(1, "bybit", "api-key", "secret-key", "", true, 1000.50, "", now, now)
				mock.ExpectQuery(`SELECT .+ FROM exchanges WHERE id = \$1`).
					WithArgs(1).
					WillReturnRows(rows)
			},
			expected: &models.ExchangeAccount{
				ID:         1,
				Name:       "bybit",
				APIKey:     "api-key",
				SecretKey:  "secret-key",
				Passphrase: "",
				Connected:  true,
				Balance:    1000.50,
				LastError:  "",
				UpdatedAt:  now,
				CreatedAt:  now,
			},
			expectError: nil,
		},
		{
			name: "not found",
			id:   999,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM exchanges WHERE id = \$1`).
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			expected:    nil,
			expectError: ErrExchangeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			repo := NewExchangeRepository(db)
			result, err := repo.GetByID(tt.id)

			if tt.expectError != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectError)
				} else if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.ID != tt.expected.ID {
					t.Errorf("expected ID=%d, got %d", tt.expected.ID, result.ID)
				}
				if result.Name != tt.expected.Name {
					t.Errorf("expected Name=%s, got %s", tt.expected.Name, result.Name)
				}
				if result.Balance != tt.expected.Balance {
					t.Errorf("expected Balance=%f, got %f", tt.expected.Balance, result.Balance)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestExchangeRepositoryGetByName(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		exchName    string
		mockSetup   func(mock sqlmock.Sqlmock)
		expected    *models.ExchangeAccount
		expectError error
	}{
		{
			name:     "success",
			exchName: "okx",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "api_key", "secret_key", "passphrase", "connected", "balance", "last_error", "updated_at", "created_at"}).
					AddRow(2, "okx", "api", "secret", "pass", true, 500.0, "", now, now)
				mock.ExpectQuery(`SELECT .+ FROM exchanges WHERE name = \$1`).
					WithArgs("okx").
					WillReturnRows(rows)
			},
			expected: &models.ExchangeAccount{
				ID:         2,
				Name:       "okx",
				Passphrase: "pass",
				Connected:  true,
				Balance:    500.0,
			},
			expectError: nil,
		},
		{
			name:     "not found",
			exchName: "unknown",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM exchanges WHERE name = \$1`).
					WithArgs("unknown").
					WillReturnError(sql.ErrNoRows)
			},
			expected:    nil,
			expectError: ErrExchangeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			repo := NewExchangeRepository(db)
			result, err := repo.GetByName(tt.exchName)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.Name != tt.expected.Name {
					t.Errorf("expected Name=%s, got %s", tt.expected.Name, result.Name)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestExchangeRepositoryGetAll(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		mockSetup   func(mock sqlmock.Sqlmock)
		expectedLen int
		expectError bool
	}{
		{
			name: "success with multiple exchanges",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "api_key", "secret_key", "passphrase", "connected", "balance", "last_error", "updated_at", "created_at"}).
					AddRow(1, "bybit", "api1", "secret1", "", true, 1000.0, "", now, now).
					AddRow(2, "okx", "api2", "secret2", "pass", true, 500.0, "", now, now).
					AddRow(3, "bitget", "api3", "secret3", "", false, 0.0, "connection error", now, now)
				mock.ExpectQuery(`SELECT .+ FROM exchanges ORDER BY name`).
					WillReturnRows(rows)
			},
			expectedLen: 3,
			expectError: false,
		},
		{
			name: "empty result",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "name", "api_key", "secret_key", "passphrase", "connected", "balance", "last_error", "updated_at", "created_at"})
				mock.ExpectQuery(`SELECT .+ FROM exchanges ORDER BY name`).
					WillReturnRows(rows)
			},
			expectedLen: 0,
			expectError: false,
		},
		{
			name: "database error",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM exchanges ORDER BY name`).
					WillReturnError(errors.New("database error"))
			},
			expectedLen: 0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			repo := NewExchangeRepository(db)
			result, err := repo.GetAll()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if len(result) != tt.expectedLen {
					t.Errorf("expected %d exchanges, got %d", tt.expectedLen, len(result))
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestExchangeRepositoryGetConnected(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "name", "api_key", "secret_key", "passphrase", "connected", "balance", "last_error", "updated_at", "created_at"}).
		AddRow(1, "bybit", "api1", "secret1", "", true, 1000.0, "", now, now).
		AddRow(2, "okx", "api2", "secret2", "pass", true, 500.0, "", now, now)
	mock.ExpectQuery(`SELECT .+ FROM exchanges WHERE connected = true ORDER BY name`).
		WillReturnRows(rows)

	repo := NewExchangeRepository(db)
	result, err := repo.GetConnected()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 connected exchanges, got %d", len(result))
	}
	for _, acc := range result {
		if !acc.Connected {
			t.Errorf("expected Connected=true for %s", acc.Name)
		}
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestExchangeRepositoryUpdate(t *testing.T) {
	tests := []struct {
		name        string
		account     *models.ExchangeAccount
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name: "success",
			account: &models.ExchangeAccount{
				ID:         1,
				APIKey:     "new-api-key",
				SecretKey:  "new-secret-key",
				Passphrase: "",
				Connected:  true,
				Balance:    2000.0,
				LastError:  "",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE exchanges SET`).
					WithArgs("new-api-key", "new-secret-key", "", true, 2000.0, "", sqlmock.AnyArg(), 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name: "not found",
			account: &models.ExchangeAccount{
				ID:        999,
				APIKey:    "api",
				SecretKey: "secret",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE exchanges SET`).
					WithArgs("api", "secret", "", false, float64(0), "", sqlmock.AnyArg(), 999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrExchangeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			repo := NewExchangeRepository(db)
			err = repo.Update(tt.account)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestExchangeRepositoryDelete(t *testing.T) {
	tests := []struct {
		name        string
		id          int
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name: "success",
			id:   1,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM exchanges WHERE id = \$1`).
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name: "not found",
			id:   999,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM exchanges WHERE id = \$1`).
					WithArgs(999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrExchangeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			repo := NewExchangeRepository(db)
			err = repo.Delete(tt.id)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestExchangeRepositoryDeleteByName(t *testing.T) {
	tests := []struct {
		name        string
		exchName    string
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name:     "success",
			exchName: "bybit",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM exchanges WHERE name = \$1`).
					WithArgs("bybit").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name:     "not found",
			exchName: "unknown",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM exchanges WHERE name = \$1`).
					WithArgs("unknown").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrExchangeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			repo := NewExchangeRepository(db)
			err = repo.DeleteByName(tt.exchName)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestExchangeRepositoryUpdateBalance(t *testing.T) {
	tests := []struct {
		name        string
		id          int
		balance     float64
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name:    "success",
			id:      1,
			balance: 5000.0,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE exchanges SET balance = \$1, updated_at = \$2 WHERE id = \$3`).
					WithArgs(5000.0, sqlmock.AnyArg(), 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name:    "not found",
			id:      999,
			balance: 100.0,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE exchanges SET balance = \$1, updated_at = \$2 WHERE id = \$3`).
					WithArgs(100.0, sqlmock.AnyArg(), 999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrExchangeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			repo := NewExchangeRepository(db)
			err = repo.UpdateBalance(tt.id, tt.balance)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestExchangeRepositoryUpdateBalanceByName(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE exchanges SET balance = \$1, updated_at = \$2 WHERE name = \$3`).
		WithArgs(3000.0, sqlmock.AnyArg(), "bybit").
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewExchangeRepository(db)
	err = repo.UpdateBalanceByName("bybit", 3000.0)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestExchangeRepositorySetConnected(t *testing.T) {
	tests := []struct {
		name        string
		id          int
		connected   bool
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name:      "set connected true",
			id:        1,
			connected: true,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE exchanges SET connected = \$1, updated_at = \$2 WHERE id = \$3`).
					WithArgs(true, sqlmock.AnyArg(), 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name:      "set connected false",
			id:        2,
			connected: false,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE exchanges SET connected = \$1, updated_at = \$2 WHERE id = \$3`).
					WithArgs(false, sqlmock.AnyArg(), 2).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name:      "not found",
			id:        999,
			connected: true,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE exchanges SET connected = \$1, updated_at = \$2 WHERE id = \$3`).
					WithArgs(true, sqlmock.AnyArg(), 999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrExchangeNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			repo := NewExchangeRepository(db)
			err = repo.SetConnected(tt.id, tt.connected)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestExchangeRepositorySetLastError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE exchanges SET last_error = \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs("API rate limit exceeded", sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewExchangeRepository(db)
	err = repo.SetLastError(1, "API rate limit exceeded")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestExchangeRepositoryCountConnected(t *testing.T) {
	tests := []struct {
		name        string
		mockSetup   func(mock sqlmock.Sqlmock)
		expected    int
		expectError bool
	}{
		{
			name: "multiple connected",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(3)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM exchanges WHERE connected = true`).
					WillReturnRows(rows)
			},
			expected:    3,
			expectError: false,
		},
		{
			name: "none connected",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count"}).AddRow(0)
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM exchanges WHERE connected = true`).
					WillReturnRows(rows)
			},
			expected:    0,
			expectError: false,
		},
		{
			name: "database error",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT COUNT\(\*\) FROM exchanges WHERE connected = true`).
					WillReturnError(errors.New("database error"))
			},
			expected:    0,
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			tt.mockSetup(mock)

			repo := NewExchangeRepository(db)
			result, err := repo.CountConnected()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected count=%d, got %d", tt.expected, result)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestIsUniqueViolation(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected bool
	}{
		{"nil error", nil, false},
		{"duplicate key error", errors.New("duplicate key value violates unique constraint"), true},
		{"postgres error code 23505", errors.New("ERROR: 23505 duplicate key"), true},
		{"other error", errors.New("connection refused"), false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isUniqueViolation(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
