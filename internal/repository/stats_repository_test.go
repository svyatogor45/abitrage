package repository

import (
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
)

// ============================================================
// StatsRepository Tests
// ============================================================

func TestNewStatsRepository(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewStatsRepository(db)
	if repo == nil {
		t.Fatal("NewStatsRepository returned nil")
	}
	if repo.db != db {
		t.Error("db not set correctly")
	}
}

func TestStatsRepositoryRecordTrade(t *testing.T) {
	now := time.Now()
	entryTime := now.Add(-time.Hour)
	exitTime := now

	tests := []struct {
		name           string
		pairID         int
		symbol         string
		exchanges      [2]string
		entryTime      time.Time
		exitTime       time.Time
		pnl            float64
		wasStopLoss    bool
		wasLiquidation bool
		mockSetup      func(mock sqlmock.Sqlmock)
		expectError    bool
	}{
		{
			name:           "success - normal trade",
			pairID:         1,
			symbol:         "BTCUSDT",
			exchanges:      [2]string{"bybit", "okx"},
			entryTime:      entryTime,
			exitTime:       exitTime,
			pnl:            100.50,
			wasStopLoss:    false,
			wasLiquidation: false,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO trades`).
					WithArgs(1, "BTCUSDT", "bybit,okx", entryTime, exitTime, 100.50, false, false, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expectError: false,
		},
		{
			name:           "success - stop loss trade",
			pairID:         2,
			symbol:         "ETHUSDT",
			exchanges:      [2]string{"bitget", "gate"},
			entryTime:      entryTime,
			exitTime:       exitTime,
			pnl:            -50.0,
			wasStopLoss:    true,
			wasLiquidation: false,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO trades`).
					WithArgs(2, "ETHUSDT", "bitget,gate", entryTime, exitTime, -50.0, true, false, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(2, 1))
			},
			expectError: false,
		},
		{
			name:           "success - liquidation trade",
			pairID:         3,
			symbol:         "SOLUSDT",
			exchanges:      [2]string{"htx", "bingx"},
			entryTime:      entryTime,
			exitTime:       exitTime,
			pnl:            -200.0,
			wasStopLoss:    false,
			wasLiquidation: true,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO trades`).
					WithArgs(3, "SOLUSDT", "htx,bingx", entryTime, exitTime, -200.0, false, true, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(3, 1))
			},
			expectError: false,
		},
		{
			name:           "database error",
			pairID:         1,
			symbol:         "BTCUSDT",
			exchanges:      [2]string{"bybit", "okx"},
			entryTime:      entryTime,
			exitTime:       exitTime,
			pnl:            100.0,
			wasStopLoss:    false,
			wasLiquidation: false,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`INSERT INTO trades`).
					WithArgs(1, "BTCUSDT", "bybit,okx", entryTime, exitTime, 100.0, false, false, sqlmock.AnyArg()).
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

			repo := NewStatsRepository(db)
			err = repo.RecordTrade(tt.pairID, tt.symbol, tt.exchanges, tt.entryTime, tt.exitTime, tt.pnl, tt.wasStopLoss, tt.wasLiquidation)

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

func TestStatsRepositoryGetTopPairsByTrades(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"symbol", "trade_count"}).
		AddRow("BTCUSDT", float64(100)).
		AddRow("ETHUSDT", float64(75)).
		AddRow("SOLUSDT", float64(50))
	mock.ExpectQuery(`SELECT symbol, COUNT\(\*\) as trade_count FROM trades GROUP BY symbol ORDER BY trade_count DESC LIMIT \$1`).
		WithArgs(5).
		WillReturnRows(rows)

	repo := NewStatsRepository(db)
	result, err := repo.GetTopPairsByTrades(5)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 3 {
		t.Errorf("expected 3 results, got %d", len(result))
	}
	if result[0].Symbol != "BTCUSDT" {
		t.Errorf("expected first symbol=BTCUSDT, got %s", result[0].Symbol)
	}
	if result[0].Value != 100 {
		t.Errorf("expected first value=100, got %f", result[0].Value)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStatsRepositoryGetTopPairsByProfit(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"symbol", "total_pnl"}).
		AddRow("BTCUSDT", 500.0).
		AddRow("ETHUSDT", 300.0)
	mock.ExpectQuery(`SELECT symbol, SUM\(pnl\) as total_pnl FROM trades GROUP BY symbol HAVING SUM\(pnl\) > 0 ORDER BY total_pnl DESC LIMIT \$1`).
		WithArgs(5).
		WillReturnRows(rows)

	repo := NewStatsRepository(db)
	result, err := repo.GetTopPairsByProfit(5)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
	if result[0].Value != 500.0 {
		t.Errorf("expected first value=500, got %f", result[0].Value)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStatsRepositoryGetTopPairsByLoss(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"symbol", "total_pnl"}).
		AddRow("XRPUSDT", -150.0).
		AddRow("DOGEUSDT", -100.0)
	mock.ExpectQuery(`SELECT symbol, SUM\(pnl\) as total_pnl FROM trades GROUP BY symbol HAVING SUM\(pnl\) < 0 ORDER BY total_pnl ASC LIMIT \$1`).
		WithArgs(5).
		WillReturnRows(rows)

	repo := NewStatsRepository(db)
	result, err := repo.GetTopPairsByLoss(5)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 results, got %d", len(result))
	}
	if result[0].Value != -150.0 {
		t.Errorf("expected first value=-150, got %f", result[0].Value)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStatsRepositoryResetCounters(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM trades`).
		WillReturnResult(sqlmock.NewResult(0, 100))

	repo := NewStatsRepository(db)
	err = repo.ResetCounters()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStatsRepositoryDeleteOlderThan(t *testing.T) {
	threshold := time.Now().AddDate(0, -1, 0)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM trades WHERE exit_time < \$1`).
		WithArgs(threshold).
		WillReturnResult(sqlmock.NewResult(0, 50))

	repo := NewStatsRepository(db)
	deleted, err := repo.DeleteOlderThan(threshold)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if deleted != 50 {
		t.Errorf("expected 50 deleted, got %d", deleted)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStatsRepositoryGetTradesByPairID(t *testing.T) {
	now := time.Now()
	entryTime := now.Add(-time.Hour)
	exitTime := now

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "pair_id", "symbol", "exchanges", "entry_time", "exit_time", "pnl", "was_stop_loss", "was_liquidation", "created_at"}).
		AddRow(1, 1, "BTCUSDT", "bybit,okx", entryTime, exitTime, 100.0, false, false, now).
		AddRow(2, 1, "BTCUSDT", "bybit,okx", entryTime, exitTime, 50.0, false, false, now)
	mock.ExpectQuery(`SELECT .+ FROM trades WHERE pair_id = \$1 ORDER BY exit_time DESC LIMIT \$2`).
		WithArgs(1, 10).
		WillReturnRows(rows)

	repo := NewStatsRepository(db)
	result, err := repo.GetTradesByPairID(1, 10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 trades, got %d", len(result))
	}
	if result[0].Symbol != "BTCUSDT" {
		t.Errorf("expected Symbol=BTCUSDT, got %s", result[0].Symbol)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStatsRepositoryGetTradesInTimeRange(t *testing.T) {
	now := time.Now()
	from := now.AddDate(0, 0, -7)
	to := now
	entryTime := now.Add(-time.Hour)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "pair_id", "symbol", "exchanges", "entry_time", "exit_time", "pnl", "was_stop_loss", "was_liquidation", "created_at"}).
		AddRow(1, 1, "BTCUSDT", "bybit,okx", entryTime, now, 100.0, false, false, now)
	mock.ExpectQuery(`SELECT .+ FROM trades WHERE exit_time >= \$1 AND exit_time <= \$2 ORDER BY exit_time DESC LIMIT \$3`).
		WithArgs(from, to, 10).
		WillReturnRows(rows)

	repo := NewStatsRepository(db)
	result, err := repo.GetTradesInTimeRange(from, to, 10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 trade, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStatsRepositoryCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(250)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM trades`).
		WillReturnRows(rows)

	repo := NewStatsRepository(db)
	count, err := repo.Count()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count != 250 {
		t.Errorf("expected count=250, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestStatsRepositoryGetPNLBySymbol(t *testing.T) {
	tests := []struct {
		name        string
		symbol      string
		expected    float64
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError bool
	}{
		{
			name:     "positive PNL",
			symbol:   "BTCUSDT",
			expected: 500.0,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"pnl"}).AddRow(500.0)
				mock.ExpectQuery(`SELECT COALESCE\(SUM\(pnl\), 0\) FROM trades WHERE symbol = \$1`).
					WithArgs("BTCUSDT").
					WillReturnRows(rows)
			},
			expectError: false,
		},
		{
			name:     "negative PNL",
			symbol:   "ETHUSDT",
			expected: -100.0,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"pnl"}).AddRow(-100.0)
				mock.ExpectQuery(`SELECT COALESCE\(SUM\(pnl\), 0\) FROM trades WHERE symbol = \$1`).
					WithArgs("ETHUSDT").
					WillReturnRows(rows)
			},
			expectError: false,
		},
		{
			name:     "no trades - zero PNL",
			symbol:   "UNKNOWN",
			expected: 0.0,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"pnl"}).AddRow(0.0)
				mock.ExpectQuery(`SELECT COALESCE\(SUM\(pnl\), 0\) FROM trades WHERE symbol = \$1`).
					WithArgs("UNKNOWN").
					WillReturnRows(rows)
			},
			expectError: false,
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

			repo := NewStatsRepository(db)
			result, err := repo.GetPNLBySymbol(tt.symbol)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result != tt.expected {
					t.Errorf("expected PNL=%f, got %f", tt.expected, result)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestStatsRepositoryGetTradesStats(t *testing.T) {
	now := time.Now()
	from := now.AddDate(0, 0, -7)
	to := now

	tests := []struct {
		name          string
		from          time.Time
		to            time.Time
		expectedCount int
		expectedPnl   float64
		mockSetup     func(mock sqlmock.Sqlmock)
		expectError   bool
	}{
		{
			name:          "with time range",
			from:          from,
			to:            to,
			expectedCount: 10,
			expectedPnl:   500.0,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count", "pnl"}).AddRow(10, 500.0)
				mock.ExpectQuery(`SELECT COUNT\(\*\), COALESCE\(SUM\(pnl\), 0\) FROM trades WHERE exit_time >= \$1 AND exit_time <= \$2`).
					WithArgs(from, to).
					WillReturnRows(rows)
			},
			expectError: false,
		},
		{
			name:          "all time (zero from)",
			from:          time.Time{},
			to:            time.Time{},
			expectedCount: 100,
			expectedPnl:   2500.0,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"count", "pnl"}).AddRow(100, 2500.0)
				mock.ExpectQuery(`SELECT COUNT\(\*\), COALESCE\(SUM\(pnl\), 0\) FROM trades`).
					WillReturnRows(rows)
			},
			expectError: false,
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

			repo := NewStatsRepository(db)
			count, pnl, err := repo.getTradesStats(tt.from, tt.to)

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if count != tt.expectedCount {
					t.Errorf("expected count=%d, got %d", tt.expectedCount, count)
				}
				if pnl != tt.expectedPnl {
					t.Errorf("expected pnl=%f, got %f", tt.expectedPnl, pnl)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}
