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
// BlacklistRepository Tests
// ============================================================

func TestNewBlacklistRepository(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewBlacklistRepository(db)
	if repo == nil {
		t.Fatal("NewBlacklistRepository returned nil")
	}
	if repo.db != db {
		t.Error("db not set correctly")
	}
}

func TestBlacklistRepositoryCreate(t *testing.T) {
	tests := []struct {
		name        string
		entry       *models.BlacklistEntry
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name: "success",
			entry: &models.BlacklistEntry{
				Symbol: "btcusdt",
				Reason: "High volatility",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO blacklist`).
					WithArgs("BTCUSDT", "High volatility", sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			expectError: nil,
		},
		{
			name: "duplicate entry",
			entry: &models.BlacklistEntry{
				Symbol: "ETHUSDT",
				Reason: "Test",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO blacklist`).
					WithArgs("ETHUSDT", "Test", sqlmock.AnyArg()).
					WillReturnError(errors.New("duplicate key value violates unique constraint"))
			},
			expectError: ErrBlacklistEntryExists,
		},
		{
			name: "uppercase conversion",
			entry: &models.BlacklistEntry{
				Symbol: "solusdt",
				Reason: "Low liquidity",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO blacklist`).
					WithArgs("SOLUSDT", "Low liquidity", sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
			},
			expectError: nil,
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

			repo := NewBlacklistRepository(db)
			err = repo.Create(tt.entry)

			if tt.expectError != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectError)
				} else if tt.expectError == ErrBlacklistEntryExists && !errors.Is(err, ErrBlacklistEntryExists) {
					t.Errorf("expected ErrBlacklistEntryExists, got %v", err)
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

func TestBlacklistRepositoryGetAll(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "symbol", "reason", "created_at"}).
		AddRow(1, "BTCUSDT", "High volatility", now).
		AddRow(2, "ETHUSDT", "Low liquidity", now)
	mock.ExpectQuery(`SELECT .+ FROM blacklist ORDER BY created_at DESC`).
		WillReturnRows(rows)

	repo := NewBlacklistRepository(db)
	result, err := repo.GetAll()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 entries, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestBlacklistRepositoryGetByID(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		id          int
		mockSetup   func(mock sqlmock.Sqlmock)
		expected    *models.BlacklistEntry
		expectError error
	}{
		{
			name: "success",
			id:   1,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "symbol", "reason", "created_at"}).
					AddRow(1, "BTCUSDT", "High volatility", now)
				mock.ExpectQuery(`SELECT .+ FROM blacklist WHERE id = \$1`).
					WithArgs(1).
					WillReturnRows(rows)
			},
			expected: &models.BlacklistEntry{
				ID:     1,
				Symbol: "BTCUSDT",
				Reason: "High volatility",
			},
			expectError: nil,
		},
		{
			name: "not found",
			id:   999,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM blacklist WHERE id = \$1`).
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			expected:    nil,
			expectError: ErrBlacklistEntryNotFound,
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

			repo := NewBlacklistRepository(db)
			result, err := repo.GetByID(tt.id)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.Symbol != tt.expected.Symbol {
					t.Errorf("expected Symbol=%s, got %s", tt.expected.Symbol, result.Symbol)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestBlacklistRepositoryGetBySymbol(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		symbol      string
		mockSetup   func(mock sqlmock.Sqlmock)
		expected    *models.BlacklistEntry
		expectError error
	}{
		{
			name:   "success - uppercase",
			symbol: "BTCUSDT",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "symbol", "reason", "created_at"}).
					AddRow(1, "BTCUSDT", "High volatility", now)
				mock.ExpectQuery(`SELECT .+ FROM blacklist WHERE symbol = \$1`).
					WithArgs("BTCUSDT").
					WillReturnRows(rows)
			},
			expected: &models.BlacklistEntry{
				Symbol: "BTCUSDT",
			},
			expectError: nil,
		},
		{
			name:   "success - lowercase converted",
			symbol: "ethusdt",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "symbol", "reason", "created_at"}).
					AddRow(2, "ETHUSDT", "Test", now)
				mock.ExpectQuery(`SELECT .+ FROM blacklist WHERE symbol = \$1`).
					WithArgs("ETHUSDT").
					WillReturnRows(rows)
			},
			expected: &models.BlacklistEntry{
				Symbol: "ETHUSDT",
			},
			expectError: nil,
		},
		{
			name:   "not found",
			symbol: "UNKNOWN",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM blacklist WHERE symbol = \$1`).
					WithArgs("UNKNOWN").
					WillReturnError(sql.ErrNoRows)
			},
			expected:    nil,
			expectError: ErrBlacklistEntryNotFound,
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

			repo := NewBlacklistRepository(db)
			result, err := repo.GetBySymbol(tt.symbol)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.Symbol != tt.expected.Symbol {
					t.Errorf("expected Symbol=%s, got %s", tt.expected.Symbol, result.Symbol)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestBlacklistRepositoryDelete(t *testing.T) {
	tests := []struct {
		name        string
		symbol      string
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name:   "success",
			symbol: "BTCUSDT",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM blacklist WHERE symbol = \$1`).
					WithArgs("BTCUSDT").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name:   "lowercase converted",
			symbol: "ethusdt",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM blacklist WHERE symbol = \$1`).
					WithArgs("ETHUSDT").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name:   "not found",
			symbol: "UNKNOWN",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM blacklist WHERE symbol = \$1`).
					WithArgs("UNKNOWN").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrBlacklistEntryNotFound,
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

			repo := NewBlacklistRepository(db)
			err = repo.Delete(tt.symbol)

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

func TestBlacklistRepositoryDeleteByID(t *testing.T) {
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
				mock.ExpectExec(`DELETE FROM blacklist WHERE id = \$1`).
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name: "not found",
			id:   999,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM blacklist WHERE id = \$1`).
					WithArgs(999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrBlacklistEntryNotFound,
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

			repo := NewBlacklistRepository(db)
			err = repo.DeleteByID(tt.id)

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

func TestBlacklistRepositoryExists(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		expected bool
	}{
		{"exists - uppercase", "BTCUSDT", true},
		{"exists - lowercase", "ethusdt", true},
		{"not exists", "UNKNOWN", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to create mock: %v", err)
			}
			defer db.Close()

			rows := sqlmock.NewRows([]string{"exists"}).AddRow(tt.expected)
			mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM blacklist WHERE symbol = \$1\)`).
				WithArgs(sqlmock.AnyArg()).
				WillReturnRows(rows)

			repo := NewBlacklistRepository(db)
			exists, err := repo.Exists(tt.symbol)

			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if exists != tt.expected {
				t.Errorf("expected exists=%v, got %v", tt.expected, exists)
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestBlacklistRepositoryUpdateReason(t *testing.T) {
	tests := []struct {
		name        string
		symbol      string
		reason      string
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name:   "success",
			symbol: "BTCUSDT",
			reason: "Updated reason",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE blacklist SET reason = \$1 WHERE symbol = \$2`).
					WithArgs("Updated reason", "BTCUSDT").
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name:   "not found",
			symbol: "UNKNOWN",
			reason: "Test",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE blacklist SET reason = \$1 WHERE symbol = \$2`).
					WithArgs("Test", "UNKNOWN").
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrBlacklistEntryNotFound,
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

			repo := NewBlacklistRepository(db)
			err = repo.UpdateReason(tt.symbol, tt.reason)

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

func TestBlacklistRepositoryCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(10)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM blacklist`).
		WillReturnRows(rows)

	repo := NewBlacklistRepository(db)
	count, err := repo.Count()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count != 10 {
		t.Errorf("expected count=10, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestBlacklistRepositoryDeleteAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM blacklist`).
		WillReturnResult(sqlmock.NewResult(0, 10))

	repo := NewBlacklistRepository(db)
	err = repo.DeleteAll()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestBlacklistRepositorySearch(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "symbol", "reason", "created_at"}).
		AddRow(1, "BTCUSDT", "High volatility", now)
	mock.ExpectQuery(`SELECT .+ FROM blacklist WHERE UPPER\(symbol\) LIKE UPPER\(\$1\)`).
		WithArgs("%BTC%").
		WillReturnRows(rows)

	repo := NewBlacklistRepository(db)
	result, err := repo.Search("BTC")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 result, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestIsBlacklistUniqueViolation(t *testing.T) {
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
			result := isBlacklistUniqueViolation(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
