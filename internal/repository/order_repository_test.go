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
// OrderRepository Tests
// ============================================================

func TestNewOrderRepository(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewOrderRepository(db)
	if repo == nil {
		t.Fatal("NewOrderRepository returned nil")
	}
	if repo.db != db {
		t.Error("db not set correctly")
	}
}

func TestOrderRepositoryCreate(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		order       *models.OrderRecord
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError bool
	}{
		{
			name: "success",
			order: &models.OrderRecord{
				PairID:       1,
				Exchange:     "bybit",
				Side:         "buy",
				Type:         "market",
				PartIndex:    0,
				Quantity:     0.01,
				PriceAvg:     50000.0,
				Status:       models.OrderStatusFilled,
				ErrorMessage: "",
				FilledAt:     &now,
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO orders`).
					WithArgs(1, "bybit", "buy", "market", 0, 0.01, 50000.0, models.OrderStatusFilled, "", sqlmock.AnyArg(), &now).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			expectError: false,
		},
		{
			name: "database error",
			order: &models.OrderRecord{
				PairID:   1,
				Exchange: "bybit",
				Side:     "buy",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO orders`).
					WithArgs(1, "bybit", "buy", "", 0, float64(0), float64(0), "", "", sqlmock.AnyArg(), (*time.Time)(nil)).
					WillReturnError(errors.New("database error"))
			},
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

			repo := NewOrderRepository(db)
			err = repo.Create(tt.order)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.order.ID != 1 {
					t.Errorf("expected ID=1, got %d", tt.order.ID)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrderRepositoryGetByID(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		id          int
		mockSetup   func(mock sqlmock.Sqlmock)
		expected    *models.OrderRecord
		expectError error
	}{
		{
			name: "success",
			id:   1,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "pair_id", "exchange", "side", "type", "part_index", "quantity", "price_avg", "status", "error_message", "created_at", "filled_at"}).
					AddRow(1, 1, "bybit", "buy", "market", 0, 0.01, 50000.0, "filled", "", now, &now)
				mock.ExpectQuery(`SELECT .+ FROM orders WHERE id = \$1`).
					WithArgs(1).
					WillReturnRows(rows)
			},
			expected: &models.OrderRecord{
				ID:       1,
				PairID:   1,
				Exchange: "bybit",
				Side:     "buy",
				Type:     "market",
				Status:   "filled",
			},
			expectError: nil,
		},
		{
			name: "not found",
			id:   999,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM orders WHERE id = \$1`).
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			expected:    nil,
			expectError: ErrOrderNotFound,
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

			repo := NewOrderRepository(db)
			result, err := repo.GetByID(tt.id)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.Exchange != tt.expected.Exchange {
					t.Errorf("expected Exchange=%s, got %s", tt.expected.Exchange, result.Exchange)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestOrderRepositoryGetByPairID(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "pair_id", "exchange", "side", "type", "part_index", "quantity", "price_avg", "status", "error_message", "created_at", "filled_at"}).
		AddRow(1, 1, "bybit", "buy", "market", 0, 0.01, 50000.0, "filled", "", now, &now).
		AddRow(2, 1, "okx", "sell", "market", 0, 0.01, 50100.0, "filled", "", now, &now)
	mock.ExpectQuery(`SELECT .+ FROM orders WHERE pair_id = \$1 ORDER BY created_at DESC`).
		WithArgs(1).
		WillReturnRows(rows)

	repo := NewOrderRepository(db)
	result, err := repo.GetByPairID(1)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 orders, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrderRepositoryGetRecent(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "pair_id", "exchange", "side", "type", "part_index", "quantity", "price_avg", "status", "error_message", "created_at", "filled_at"}).
		AddRow(3, 2, "bitget", "buy", "market", 0, 0.1, 3000.0, "filled", "", now, &now).
		AddRow(2, 1, "okx", "sell", "market", 0, 0.01, 50100.0, "filled", "", now, &now).
		AddRow(1, 1, "bybit", "buy", "market", 0, 0.01, 50000.0, "filled", "", now, &now)
	mock.ExpectQuery(`SELECT .+ FROM orders ORDER BY created_at DESC LIMIT \$1`).
		WithArgs(10).
		WillReturnRows(rows)

	repo := NewOrderRepository(db)
	result, err := repo.GetRecent(10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 orders, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrderRepositoryGetByStatus(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "pair_id", "exchange", "side", "type", "part_index", "quantity", "price_avg", "status", "error_message", "created_at", "filled_at"}).
		AddRow(1, 1, "bybit", "buy", "market", 0, 0.01, 50000.0, "filled", "", now, &now)
	mock.ExpectQuery(`SELECT .+ FROM orders WHERE status = \$1 ORDER BY created_at DESC`).
		WithArgs(models.OrderStatusFilled).
		WillReturnRows(rows)

	repo := NewOrderRepository(db)
	result, err := repo.GetByStatus(models.OrderStatusFilled)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 order, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrderRepositoryGetByExchange(t *testing.T) {
	now := time.Now()

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "pair_id", "exchange", "side", "type", "part_index", "quantity", "price_avg", "status", "error_message", "created_at", "filled_at"}).
		AddRow(1, 1, "bybit", "buy", "market", 0, 0.01, 50000.0, "filled", "", now, &now)
	mock.ExpectQuery(`SELECT .+ FROM orders WHERE exchange = \$1 ORDER BY created_at DESC LIMIT \$2`).
		WithArgs("bybit", 10).
		WillReturnRows(rows)

	repo := NewOrderRepository(db)
	result, err := repo.GetByExchange("bybit", 10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 order, got %d", len(result))
	}
	if result[0].Exchange != "bybit" {
		t.Errorf("expected Exchange=bybit, got %s", result[0].Exchange)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrderRepositoryUpdateStatus(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name        string
		id          int
		status      string
		priceAvg    float64
		filledAt    *time.Time
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name:     "success",
			id:       1,
			status:   models.OrderStatusFilled,
			priceAvg: 50000.0,
			filledAt: &now,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE orders SET status = \$1, price_avg = \$2, filled_at = \$3 WHERE id = \$4`).
					WithArgs(models.OrderStatusFilled, 50000.0, &now, 1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name:     "not found",
			id:       999,
			status:   models.OrderStatusFilled,
			priceAvg: 50000.0,
			filledAt: &now,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE orders SET status = \$1, price_avg = \$2, filled_at = \$3 WHERE id = \$4`).
					WithArgs(models.OrderStatusFilled, 50000.0, &now, 999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrOrderNotFound,
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

			repo := NewOrderRepository(db)
			err = repo.UpdateStatus(tt.id, tt.status, tt.priceAvg, tt.filledAt)

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

func TestOrderRepositorySetError(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE orders SET error_message = \$1, status = \$2 WHERE id = \$3`).
		WithArgs("Insufficient balance", models.OrderStatusRejected, 1).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewOrderRepository(db)
	err = repo.SetError(1, "Insufficient balance")

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrderRepositoryDelete(t *testing.T) {
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
				mock.ExpectExec(`DELETE FROM orders WHERE id = \$1`).
					WithArgs(1).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name: "not found",
			id:   999,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`DELETE FROM orders WHERE id = \$1`).
					WithArgs(999).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrOrderNotFound,
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

			repo := NewOrderRepository(db)
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

func TestOrderRepositoryDeleteByPairID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM orders WHERE pair_id = \$1`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 5))

	repo := NewOrderRepository(db)
	err = repo.DeleteByPairID(1)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrderRepositoryDeleteOlderThan(t *testing.T) {
	threshold := time.Now().AddDate(0, 0, -30)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM orders WHERE created_at < \$1`).
		WithArgs(threshold).
		WillReturnResult(sqlmock.NewResult(0, 10))

	repo := NewOrderRepository(db)
	deleted, err := repo.DeleteOlderThan(threshold)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deleted != 10 {
		t.Errorf("expected 10 deleted, got %d", deleted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrderRepositoryCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(25)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM orders`).
		WillReturnRows(rows)

	repo := NewOrderRepository(db)
	count, err := repo.Count()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count != 25 {
		t.Errorf("expected count=25, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrderRepositoryCountByStatus(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(20)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM orders WHERE status = \$1`).
		WithArgs(models.OrderStatusFilled).
		WillReturnRows(rows)

	repo := NewOrderRepository(db)
	count, err := repo.CountByStatus(models.OrderStatusFilled)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count != 20 {
		t.Errorf("expected count=20, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestOrderRepositoryGetFilledByPairIDInTimeRange(t *testing.T) {
	now := time.Now()
	from := now.AddDate(0, 0, -7)
	to := now

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "pair_id", "exchange", "side", "type", "part_index", "quantity", "price_avg", "status", "error_message", "created_at", "filled_at"}).
		AddRow(1, 1, "bybit", "buy", "market", 0, 0.01, 50000.0, "filled", "", now, &now)
	mock.ExpectQuery(`SELECT .+ FROM orders WHERE pair_id = \$1 AND status = \$2 AND filled_at >= \$3 AND filled_at <= \$4`).
		WithArgs(1, models.OrderStatusFilled, from, to).
		WillReturnRows(rows)

	repo := NewOrderRepository(db)
	result, err := repo.GetFilledByPairIDInTimeRange(1, from, to)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 order, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}
