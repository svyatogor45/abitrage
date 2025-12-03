package repository

import (
	"database/sql"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"

	"arbitrage/internal/models"
)

// ============================================================
// SettingsRepository Tests
// ============================================================

func TestNewSettingsRepository(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewSettingsRepository(db)
	if repo == nil {
		t.Fatal("NewSettingsRepository returned nil")
	}
	if repo.db != db {
		t.Error("db not set correctly")
	}
}

func TestSettingsRepositoryGet(t *testing.T) {
	now := time.Now()
	maxTrades := 5

	tests := []struct {
		name        string
		mockSetup   func(mock sqlmock.Sqlmock)
		expected    *models.Settings
		expectError bool
	}{
		{
			name: "success",
			mockSetup: func(mock sqlmock.Sqlmock) {
				prefsJSON, _ := json.Marshal(models.NotificationPreferences{
					Open:          true,
					Close:         true,
					StopLoss:      true,
					Liquidation:   true,
					APIError:      true,
					Margin:        true,
					Pause:         true,
					SecondLegFail: true,
				})
				rows := sqlmock.NewRows([]string{"id", "consider_funding", "max_concurrent_trades", "notification_prefs", "updated_at"}).
					AddRow(1, true, &maxTrades, prefsJSON, now)
				mock.ExpectQuery(`SELECT .+ FROM settings WHERE id = 1`).
					WillReturnRows(rows)
			},
			expected: &models.Settings{
				ID:                  1,
				ConsiderFunding:     true,
				MaxConcurrentTrades: &maxTrades,
			},
			expectError: false,
		},
		{
			name: "not found - creates default",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM settings WHERE id = 1`).
					WillReturnError(sql.ErrNoRows)
				// createDefault is called
				prefsJSON, _ := json.Marshal(defaultNotificationPrefs())
				mock.ExpectExec(`INSERT INTO settings`).
					WithArgs(false, (*int)(nil), prefsJSON, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(1, 1))
			},
			expected: &models.Settings{
				ID:              1,
				ConsiderFunding: false,
			},
			expectError: false,
		},
		{
			name: "empty notification prefs",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "consider_funding", "max_concurrent_trades", "notification_prefs", "updated_at"}).
					AddRow(1, false, nil, nil, now)
				mock.ExpectQuery(`SELECT .+ FROM settings WHERE id = 1`).
					WillReturnRows(rows)
			},
			expected: &models.Settings{
				ID:              1,
				ConsiderFunding: false,
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

			repo := NewSettingsRepository(db)
			result, err := repo.Get()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.ConsiderFunding != tt.expected.ConsiderFunding {
					t.Errorf("expected ConsiderFunding=%v, got %v", tt.expected.ConsiderFunding, result.ConsiderFunding)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestSettingsRepositoryUpdate(t *testing.T) {
	maxTrades := 10

	tests := []struct {
		name        string
		settings    *models.Settings
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError error
	}{
		{
			name: "success",
			settings: &models.Settings{
				ID:                  1,
				ConsiderFunding:     true,
				MaxConcurrentTrades: &maxTrades,
				NotificationPrefs: models.NotificationPreferences{
					Open:  true,
					Close: true,
				},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE settings SET`).
					WithArgs(true, &maxTrades, sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: nil,
		},
		{
			name: "not found",
			settings: &models.Settings{
				ID:              1,
				ConsiderFunding: false,
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE settings SET`).
					WithArgs(false, (*int)(nil), sqlmock.AnyArg(), sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 0))
			},
			expectError: ErrSettingsNotFound,
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

			repo := NewSettingsRepository(db)
			err = repo.Update(tt.settings)

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

func TestSettingsRepositoryUpdateNotificationPrefs(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	prefs := models.NotificationPreferences{
		Open:          true,
		Close:         false,
		StopLoss:      true,
		Liquidation:   true,
		APIError:      false,
		Margin:        true,
		Pause:         false,
		SecondLegFail: true,
	}

	mock.ExpectExec(`UPDATE settings SET notification_prefs = \$1, updated_at = \$2 WHERE id = 1`).
		WithArgs(sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewSettingsRepository(db)
	err = repo.UpdateNotificationPrefs(prefs)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSettingsRepositoryUpdateConsiderFunding(t *testing.T) {
	tests := []struct {
		name        string
		consider    bool
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError bool
	}{
		{
			name:     "set true",
			consider: true,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE settings SET consider_funding = \$1, updated_at = \$2 WHERE id = 1`).
					WithArgs(true, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
			},
			expectError: false,
		},
		{
			name:     "set false",
			consider: false,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectExec(`UPDATE settings SET consider_funding = \$1, updated_at = \$2 WHERE id = 1`).
					WithArgs(false, sqlmock.AnyArg()).
					WillReturnResult(sqlmock.NewResult(0, 1))
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

			repo := NewSettingsRepository(db)
			err = repo.UpdateConsiderFunding(tt.consider)

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

func TestSettingsRepositoryUpdateMaxConcurrentTrades(t *testing.T) {
	maxTrades := 5

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE settings SET max_concurrent_trades = \$1, updated_at = \$2 WHERE id = 1`).
		WithArgs(&maxTrades, sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewSettingsRepository(db)
	err = repo.UpdateMaxConcurrentTrades(&maxTrades)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestSettingsRepositoryGetNotificationPrefs(t *testing.T) {
	tests := []struct {
		name        string
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError bool
	}{
		{
			name: "success",
			mockSetup: func(mock sqlmock.Sqlmock) {
				prefsJSON, _ := json.Marshal(models.NotificationPreferences{
					Open:  true,
					Close: true,
				})
				rows := sqlmock.NewRows([]string{"notification_prefs"}).AddRow(prefsJSON)
				mock.ExpectQuery(`SELECT notification_prefs FROM settings WHERE id = 1`).
					WillReturnRows(rows)
			},
			expectError: false,
		},
		{
			name: "not found - returns default",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT notification_prefs FROM settings WHERE id = 1`).
					WillReturnError(sql.ErrNoRows)
			},
			expectError: false,
		},
		{
			name: "empty prefs - returns default",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"notification_prefs"}).AddRow(nil)
				mock.ExpectQuery(`SELECT notification_prefs FROM settings WHERE id = 1`).
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

			repo := NewSettingsRepository(db)
			result, err := repo.GetNotificationPrefs()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result == nil {
					t.Error("expected non-nil result")
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestSettingsRepositoryGetMaxConcurrentTrades(t *testing.T) {
	maxTrades := 10

	tests := []struct {
		name        string
		mockSetup   func(mock sqlmock.Sqlmock)
		expected    *int
		expectError bool
	}{
		{
			name: "success with value",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"max_concurrent_trades"}).AddRow(&maxTrades)
				mock.ExpectQuery(`SELECT max_concurrent_trades FROM settings WHERE id = 1`).
					WillReturnRows(rows)
			},
			expected:    &maxTrades,
			expectError: false,
		},
		{
			name: "success with null",
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"max_concurrent_trades"}).AddRow(nil)
				mock.ExpectQuery(`SELECT max_concurrent_trades FROM settings WHERE id = 1`).
					WillReturnRows(rows)
			},
			expected:    nil,
			expectError: false,
		},
		{
			name: "not found - returns nil",
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT max_concurrent_trades FROM settings WHERE id = 1`).
					WillReturnError(sql.ErrNoRows)
			},
			expected:    nil,
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

			repo := NewSettingsRepository(db)
			result, err := repo.GetMaxConcurrentTrades()

			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if tt.expected == nil && result != nil {
					t.Errorf("expected nil, got %v", *result)
				}
				if tt.expected != nil && (result == nil || *result != *tt.expected) {
					t.Errorf("expected %v, got %v", *tt.expected, result)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestSettingsRepositoryResetToDefaults(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`UPDATE settings SET`).
		WithArgs(false, (*int)(nil), sqlmock.AnyArg(), sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(0, 1))

	repo := NewSettingsRepository(db)
	err = repo.ResetToDefaults()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestDefaultNotificationPrefs(t *testing.T) {
	prefs := defaultNotificationPrefs()

	if !prefs.Open {
		t.Error("expected Open=true")
	}
	if !prefs.Close {
		t.Error("expected Close=true")
	}
	if !prefs.StopLoss {
		t.Error("expected StopLoss=true")
	}
	if !prefs.Liquidation {
		t.Error("expected Liquidation=true")
	}
	if !prefs.APIError {
		t.Error("expected APIError=true")
	}
	if !prefs.Margin {
		t.Error("expected Margin=true")
	}
	if !prefs.Pause {
		t.Error("expected Pause=true")
	}
	if !prefs.SecondLegFail {
		t.Error("expected SecondLegFail=true")
	}
}
