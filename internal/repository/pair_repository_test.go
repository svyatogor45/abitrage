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
// PairRepository Tests
// ============================================================

func TestNewPairRepository(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewPairRepository(db)
	if repo == nil {
		t.Fatal("NewPairRepository returned nil")
	}
	if repo.db != db {
		t.Error("db not set correctly")
	}
}

func TestPairRepositoryCreate(t *testing.T) {
	tests := []struct {
		name        string
		pair        *models.PairConfig
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name: "success",
			pair: &models.PairConfig{
				Symbol:         "BTCUSDT",
				Base:           "BTC",
				Quote:          "USDT",
				EntrySpreadPct: 0.1,
				ExitSpreadPct:  0.05,
				VolumeAsset:    0.01,
				NOrders:        1,
				StopLoss:       50.0,
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO pairs`).
					WithArgs("BTCUSDT", "BTC", "USDT", 0.1, 0.05, 0.01, 1, 50.0, models.PairStatusPaused, 0, float64(0), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			expectError: nil,
		},
		{
			name: "duplicate key error",
			pair: &models.PairConfig{
				Symbol: "BTCUSDT",
				Base:   "BTC",
				Quote:  "USDT",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO pairs`).
					WithArgs("BTCUSDT", "BTC", "USDT", float64(0), float64(0), float64(0), 1, float64(0), models.PairStatusPaused, 0, float64(0), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnError(errors.New("duplicate key value violates unique constraint"))
			},
			expectError: ErrPairExists,
		},
		{
			name: "with active status",
			pair: &models.PairConfig{
				Symbol:         "ETHUSDT",
				Base:           "ETH",
				Quote:          "USDT",
				EntrySpreadPct: 0.15,
				ExitSpreadPct:  0.1,
				VolumeAsset:    0.1,
				NOrders:        2,
				StopLoss:       30.0,
				Status:         models.PairStatusActive,
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO pairs`).
					WithArgs("ETHUSDT", "ETH", "USDT", 0.15, 0.1, 0.1, 2, 30.0, models.PairStatusActive, 0, float64(0), sqlmock.AnyArg(), sqlmock.AnyArg()).
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

			repo := NewPairRepository(db)
			err = repo.Create(tt.pair)

			if tt.expectError != nil {
				if err == nil {
					t.Errorf("expected error %v, got nil", tt.expectError)
				} else if tt.expectError == ErrPairExists && !errors.Is(err, ErrPairExists) {
					t.Errorf("expected ErrPairExists, got %v", err)
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

func TestPairRepositoryGetByID(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		id          int
		mockSetup   func(mock sqlmock.Sqlmock)
		expected    *models.PairConfig
		expectError error
	}{
		{
			name: "success",
			id:   1,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "symbol", "base", "quote", "entry_spread_pct", "exit_spread_pct", "volume_asset", "n_orders", "stop_loss", "status", "trades_count", "total_pnl", "created_at", "updated_at"}).
					AddRow(1, "BTCUSDT", "BTC", "USDT", 0.1, 0.05, 0.01, 1, 50.0, "active", 10, 100.5, now, now)
				mock.ExpectQuery(`SELECT .+ FROM pairs WHERE id = \$1`).
					WithArgs(1).
					WillReturnRows(rows)
			},
			expected: &models.PairConfig{
				ID:             1,
				Symbol:         "BTCUSDT",
				Base:           "BTC",
				Quote:          "USDT",
				EntrySpreadPct: 0.1,
				ExitSpreadPct:  0.05,
				VolumeAsset:    0.01,
				NOrders:        1,
				StopLoss:       50.0,
				Status:         "active",
				TradesCount:    10,
				TotalPnl:       100.5,
			},
			expectError: nil,
		},
		{
			name: "not found",
			id:   999,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM pairs WHERE id = \$1`).
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			expected:    nil,
			expectError: ErrPairNotFound,
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

			repo := NewPairRepository(db)
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
				if result.Status != tt.expected.Status {
					t.Errorf("expected Status=%s, got %s", tt.expected.Status, result.Status)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestPairRepositoryGetBySymbol(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "symbol", "base", "quote", "entry_spread_pct", "exit_spread_pct", "volume_asset", "n_orders", "stop_loss", "status", "trades_count", "total_pnl", "created_at", "updated_at"}).
		AddRow(1, "ETHUSDT", "ETH", "USDT", 0.15, 0.1, 0.1, 2, 30.0, "paused", 5, 50.0, now, now)
	mock.ExpectQuery(`SELECT .+ FROM pairs WHERE symbol = \$1`).
		WithArgs("ETHUSDT").
		WillReturnRows(rows)

	repo := NewPairRepository(db)
	result, err := repo.GetBySymbol("ETHUSDT")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if result.Symbol != "ETHUSDT" {
		t.Errorf("expected Symbol=ETHUSDT, got %s", result.Symbol)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPairRepositoryGetAll(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "symbol", "base", "quote", "entry_spread_pct", "exit_spread_pct", "volume_asset", "n_orders", "stop_loss", "status", "trades_count", "total_pnl", "created_at", "updated_at"}).
		AddRow(1, "BTCUSDT", "BTC", "USDT", 0.1, 0.05, 0.01, 1, 50.0, "active", 10, 100.5, now, now).
		AddRow(2, "ETHUSDT", "ETH", "USDT", 0.15, 0.1, 0.1, 2, 30.0, "paused", 5, 50.0, now, now)
	mock.ExpectQuery(`SELECT .+ FROM pairs ORDER BY created_at DESC`).
		WillReturnRows(rows)

	repo := NewPairRepository(db)
	result, err := repo.GetAll()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 pairs, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPairRepositoryGetActive(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "symbol", "base", "quote", "entry_spread_pct", "exit_spread_pct", "volume_asset", "n_orders", "stop_loss", "status", "trades_count", "total_pnl", "created_at", "updated_at"}).
		AddRow(1, "BTCUSDT", "BTC", "USDT", 0.1, 0.05, 0.01, 1, 50.0, "active", 10, 100.5, now, now)
	mock.ExpectQuery(`SELECT .+ FROM pairs WHERE status = \$1`).
		WithArgs(models.PairStatusActive).
		WillReturnRows(rows)

	repo := NewPairRepository(db)
	result, err := repo.GetActive()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 active pair, got %d", len(result))
	}
	if result[0].Status != models.PairStatusActive {
		t.Errorf("expected Status=active, got %s", result[0].Status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPairRepositoryGetPaused(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "symbol", "base", "quote", "entry_spread_pct", "exit_spread_pct", "volume_asset", "n_orders", "stop_loss", "status", "trades_count", "total_pnl", "created_at", "updated_at"}).
		AddRow(2, "ETHUSDT", "ETH", "USDT", 0.15, 0.1, 0.1, 2, 30.0, "paused", 5, 50.0, now, now)
	mock.ExpectQuery(`SELECT .+ FROM pairs WHERE status = \$1`).
		WithArgs(models.PairStatusPaused).
		WillReturnRows(rows)

	repo := NewPairRepository(db)
	result, err := repo.GetPaused()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 paused pair, got %d", len(result))
	}
	if result[0].Status != models.PairStatusPaused {
		t.Errorf("expected Status=paused, got %s", result[0].Status)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPairRepositoryUpdate(t *testing.T) {
	tests := []struct {
		name        string
		pair        *models.PairConfig
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name: "success",
			pair: &models.PairConfig{
				ID:             1,
				Symbol:         "BTCUSDT",
				Base:           "BTC",
				Quote:          "USDT",
				EntrySpreadPct: 0.2,
				ExitSpreadPct:  0.1,
				VolumeAsset:    0.02,
				NOrders:        2,
				StopLoss:       100.0,
				Status:         "active",
				TradesCount:    10,
				TotalPnl:       200.0,
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pairs SET`).
					WithArgs("BTCUSDT", "BTC", "USDT", 0.2, 0.1, 0.02, 2, 100.0, "active", 10, 200.0, sqlmock.AnyArg(), 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name: "not found",
			pair: &models.PairConfig{
				ID:     999,
				Symbol: "UNKNOWN",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pairs SET`).
					WithArgs("UNKNOWN", "", "", float64(0), float64(0), float64(0), 0, float64(0), "", 0, float64(0), sqlmock.AnyArg(), 999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrPairNotFound,
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

			repo := NewPairRepository(db)
			err = repo.Update(tt.pair)

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

func TestPairRepositoryUpdateParams(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE pairs SET entry_spread_pct = \$1, exit_spread_pct = \$2, volume_asset = \$3, n_orders = \$4, stop_loss = \$5, updated_at = \$6 WHERE id = \$7`).
		WithArgs(0.25, 0.15, 0.05, 3, 75.0, sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewPairRepository(db)
	err = repo.UpdateParams(1, 0.25, 0.15, 0.05, 3, 75.0)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPairRepositoryDelete(t *testing.T) {
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
				mock.ExpectExec(`DELETE FROM pairs WHERE id = \$1`).
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name: "not found",
			id:   999,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM pairs WHERE id = \$1`).
					WithArgs(999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrPairNotFound,
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

			repo := NewPairRepository(db)
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

func TestPairRepositoryUpdateStatus(t *testing.T) {
	tests := []struct {
		name        string
		id          int
		status      string
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError bool
	}{
		{
			name:   "success - set active",
			id:     1,
			status: models.PairStatusActive,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pairs SET status = \$1, updated_at = \$2 WHERE id = \$3`).
					WithArgs(models.PairStatusActive, sqlmock.AnyArg(), 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:   "success - set paused",
			id:     1,
			status: models.PairStatusPaused,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE pairs SET status = \$1, updated_at = \$2 WHERE id = \$3`).
					WithArgs(models.PairStatusPaused, sqlmock.AnyArg(), 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:        "invalid status",
			id:          1,
			status:      "invalid",
			mockSetup:   func(mock sqlmock.Sqlmock) {},
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

			repo := NewPairRepository(db)
			err = repo.UpdateStatus(tt.id, tt.status)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
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

func TestPairRepositoryIncrementTrades(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE pairs SET trades_count = trades_count \+ 1, updated_at = \$1 WHERE id = \$2`).
		WithArgs(sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewPairRepository(db)
	err = repo.IncrementTrades(1)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPairRepositoryUpdatePnl(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE pairs SET total_pnl = total_pnl \+ \$1, updated_at = \$2 WHERE id = \$3`).
		WithArgs(25.5, sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewPairRepository(db)
	err = repo.UpdatePnl(1, 25.5)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPairRepositoryResetStats(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE pairs SET trades_count = 0, total_pnl = 0, updated_at = \$1 WHERE id = \$2`).
		WithArgs(sqlmock.AnyArg(), 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewPairRepository(db)
	err = repo.ResetStats(1)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPairRepositoryCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(5)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pairs`).
		WillReturnRows(rows)

	repo := NewPairRepository(db)
	count, err := repo.Count()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count != 5 {
		t.Errorf("expected count=5, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPairRepositoryCountActive(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(3)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM pairs WHERE status = \$1`).
		WithArgs(models.PairStatusActive).
		WillReturnRows(rows)

	repo := NewPairRepository(db)
	count, err := repo.CountActive()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count != 3 {
		t.Errorf("expected count=3, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestPairRepositoryExistsBySymbol(t *testing.T) {
	tests := []struct {
		name     string
		symbol   string
		expected bool
	}{
		{"exists", "BTCUSDT", true},
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
			mock.ExpectQuery(`SELECT EXISTS\(SELECT 1 FROM pairs WHERE symbol = \$1\)`).
				WithArgs(tt.symbol).
				WillReturnRows(rows)

			repo := NewPairRepository(db)
			exists, err := repo.ExistsBySymbol(tt.symbol)

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

func TestPairRepositorySearch(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "symbol", "base", "quote", "entry_spread_pct", "exit_spread_pct", "volume_asset", "n_orders", "stop_loss", "status", "trades_count", "total_pnl", "created_at", "updated_at"}).
		AddRow(1, "BTCUSDT", "BTC", "USDT", 0.1, 0.05, 0.01, 1, 50.0, "active", 10, 100.5, now, now)
	mock.ExpectQuery(`SELECT .+ FROM pairs WHERE LOWER\(symbol\) LIKE LOWER\(\$1\) OR LOWER\(base\) LIKE LOWER\(\$2\)`).
		WithArgs("%BTC%", "%BTC%").
		WillReturnRows(rows)

	repo := NewPairRepository(db)
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

func TestIsPairUniqueViolation(t *testing.T) {
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
			result := isPairUniqueViolation(tt.err)
			if result != tt.expected {
				t.Errorf("expected %v, got %v", tt.expected, result)
			}
		})
	}
}
