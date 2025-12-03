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
// NotificationRepository Tests
// ============================================================

func TestNewNotificationRepository(t *testing.T) {
	db, _, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	repo := NewNotificationRepository(db)
	if repo == nil {
		t.Fatal("NewNotificationRepository returned nil")
	}
	if repo.db != db {
		t.Error("db not set correctly")
	}
}

func TestNotificationRepositoryCreate(t *testing.T) {
	pairID := 1

	tests := []struct {
		name        string
		notif       *models.Notification
		mockSetup   func(mock sqlmock.Sqlmock)
		expectError bool
	}{
		{
			name: "success without meta",
			notif: &models.Notification{
				Type:     models.NotificationTypeOpen,
				Severity: models.SeverityInfo,
				PairID:   &pairID,
				Message:  "Position opened",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO notifications`).
					WithArgs(sqlmock.AnyArg(), models.NotificationTypeOpen, models.SeverityInfo, &pairID, "Position opened", []byte(nil)).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(1))
			},
			expectError: false,
		},
		{
			name: "success with meta",
			notif: &models.Notification{
				Type:     models.NotificationTypeError,
				Severity: models.SeverityError,
				PairID:   &pairID,
				Message:  "API error",
				Meta:     map[string]interface{}{"code": 400, "exchange": "bybit"},
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO notifications`).
					WithArgs(sqlmock.AnyArg(), models.NotificationTypeError, models.SeverityError, &pairID, "API error", sqlmock.AnyArg()).
					WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(2))
			},
			expectError: false,
		},
		{
			name: "database error",
			notif: &models.Notification{
				Type:     models.NotificationTypeSL,
				Severity: models.SeverityWarn,
				Message:  "Stop loss triggered",
			},
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`INSERT INTO notifications`).
					WithArgs(sqlmock.AnyArg(), models.NotificationTypeSL, models.SeverityWarn, (*int)(nil), "Stop loss triggered", []byte(nil)).
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

			repo := NewNotificationRepository(db)
			err = repo.Create(tt.notif)

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

func TestNotificationRepositoryGetByID(t *testing.T) {
	now := time.Now()
	pairID := 1

	tests := []struct {
		name        string
		id          int
		mockSetup   func(mock sqlmock.Sqlmock)
		expected    *models.Notification
		expectError error
	}{
		{
			name: "success without meta",
			id:   1,
			mockSetup: func(mock sqlmock.Sqlmock) {
				rows := sqlmock.NewRows([]string{"id", "timestamp", "type", "severity", "pair_id", "message", "meta"}).
					AddRow(1, now, models.NotificationTypeOpen, models.SeverityInfo, &pairID, "Position opened", nil)
				mock.ExpectQuery(`SELECT .+ FROM notifications WHERE id = \$1`).
					WithArgs(1).
					WillReturnRows(rows)
			},
			expected: &models.Notification{
				ID:       1,
				Type:     models.NotificationTypeOpen,
				Severity: models.SeverityInfo,
				Message:  "Position opened",
			},
			expectError: nil,
		},
		{
			name: "success with meta",
			id:   2,
			mockSetup: func(mock sqlmock.Sqlmock) {
				metaJSON, _ := json.Marshal(map[string]interface{}{"code": 400})
				rows := sqlmock.NewRows([]string{"id", "timestamp", "type", "severity", "pair_id", "message", "meta"}).
					AddRow(2, now, models.NotificationTypeError, models.SeverityError, &pairID, "API error", metaJSON)
				mock.ExpectQuery(`SELECT .+ FROM notifications WHERE id = \$1`).
					WithArgs(2).
					WillReturnRows(rows)
			},
			expected: &models.Notification{
				ID:       2,
				Type:     models.NotificationTypeError,
				Severity: models.SeverityError,
				Message:  "API error",
			},
			expectError: nil,
		},
		{
			name: "not found",
			id:   999,
			mockSetup: func(mock sqlmock.Sqlmock) {
				mock.ExpectQuery(`SELECT .+ FROM notifications WHERE id = \$1`).
					WithArgs(999).
					WillReturnError(sql.ErrNoRows)
			},
			expected:    nil,
			expectError: ErrNotificationNotFound,
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

			repo := NewNotificationRepository(db)
			result, err := repo.GetByID(tt.id)

			if tt.expectError != nil {
				if !errors.Is(err, tt.expectError) {
					t.Errorf("expected error %v, got %v", tt.expectError, err)
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
				if result.Type != tt.expected.Type {
					t.Errorf("expected Type=%s, got %s", tt.expected.Type, result.Type)
				}
			}

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("unfulfilled expectations: %v", err)
			}
		})
	}
}

func TestNotificationRepositoryGetRecent(t *testing.T) {
	now := time.Now()
	pairID := 1

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "timestamp", "type", "severity", "pair_id", "message", "meta"}).
		AddRow(2, now, models.NotificationTypeClose, models.SeverityInfo, &pairID, "Position closed", nil).
		AddRow(1, now.Add(-time.Hour), models.NotificationTypeOpen, models.SeverityInfo, &pairID, "Position opened", nil)
	mock.ExpectQuery(`SELECT .+ FROM notifications ORDER BY timestamp DESC LIMIT \$1`).
		WithArgs(10).
		WillReturnRows(rows)

	repo := NewNotificationRepository(db)
	result, err := repo.GetRecent(10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("expected 2 notifications, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestNotificationRepositoryGetByPairID(t *testing.T) {
	now := time.Now()
	pairID := 1

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "timestamp", "type", "severity", "pair_id", "message", "meta"}).
		AddRow(1, now, models.NotificationTypeOpen, models.SeverityInfo, &pairID, "Position opened", nil)
	mock.ExpectQuery(`SELECT .+ FROM notifications WHERE pair_id = \$1 ORDER BY timestamp DESC LIMIT \$2`).
		WithArgs(pairID, 10).
		WillReturnRows(rows)

	repo := NewNotificationRepository(db)
	result, err := repo.GetByPairID(pairID, 10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 notification, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestNotificationRepositoryGetBySeverity(t *testing.T) {
	now := time.Now()
	pairID := 1

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "timestamp", "type", "severity", "pair_id", "message", "meta"}).
		AddRow(1, now, models.NotificationTypeError, models.SeverityError, &pairID, "API error", nil)
	mock.ExpectQuery(`SELECT .+ FROM notifications WHERE severity = \$1 ORDER BY timestamp DESC LIMIT \$2`).
		WithArgs(models.SeverityError, 10).
		WillReturnRows(rows)

	repo := NewNotificationRepository(db)
	result, err := repo.GetBySeverity(models.SeverityError, 10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 notification, got %d", len(result))
	}
	if result[0].Severity != models.SeverityError {
		t.Errorf("expected Severity=error, got %s", result[0].Severity)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestNotificationRepositoryGetInTimeRange(t *testing.T) {
	now := time.Now()
	from := now.AddDate(0, 0, -1)
	to := now
	pairID := 1

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"id", "timestamp", "type", "severity", "pair_id", "message", "meta"}).
		AddRow(1, now, models.NotificationTypeOpen, models.SeverityInfo, &pairID, "Position opened", nil)
	mock.ExpectQuery(`SELECT .+ FROM notifications WHERE timestamp >= \$1 AND timestamp <= \$2`).
		WithArgs(from, to, 10).
		WillReturnRows(rows)

	repo := NewNotificationRepository(db)
	result, err := repo.GetInTimeRange(from, to, 10)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if len(result) != 1 {
		t.Errorf("expected 1 notification, got %d", len(result))
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestNotificationRepositoryDeleteAll(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM notifications`).
		WillReturnResult(sqlmock.NewResult(0, 100))

	repo := NewNotificationRepository(db)
	err = repo.DeleteAll()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestNotificationRepositoryDeleteOlderThan(t *testing.T) {
	threshold := time.Now().AddDate(0, 0, -30)

	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM notifications WHERE timestamp < \$1`).
		WithArgs(threshold).
		WillReturnResult(sqlmock.NewResult(0, 50))

	repo := NewNotificationRepository(db)
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

func TestNotificationRepositoryDeleteByPairID(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM notifications WHERE pair_id = \$1`).
		WithArgs(1).
		WillReturnResult(sqlmock.NewResult(0, 10))

	repo := NewNotificationRepository(db)
	err = repo.DeleteByPairID(1)

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestNotificationRepositoryCount(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(150)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM notifications`).
		WillReturnRows(rows)

	repo := NewNotificationRepository(db)
	count, err := repo.Count()

	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if count != 150 {
		t.Errorf("expected count=150, got %d", count)
	}

	if err := mock.ExpectationsWereMet(); err != nil {
		t.Errorf("unfulfilled expectations: %v", err)
	}
}

func TestNotificationRepositoryCountByType(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(25)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM notifications WHERE type = \$1`).
		WithArgs(models.NotificationTypeError).
		WillReturnRows(rows)

	repo := NewNotificationRepository(db)
	count, err := repo.CountByType(models.NotificationTypeError)

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

func TestNotificationRepositoryCountBySeverity(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	rows := sqlmock.NewRows([]string{"count"}).AddRow(10)
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM notifications WHERE severity = \$1`).
		WithArgs(models.SeverityError).
		WillReturnRows(rows)

	repo := NewNotificationRepository(db)
	count, err := repo.CountBySeverity(models.SeverityError)

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

func TestNotificationRepositoryKeepRecent(t *testing.T) {
	db, mock, err := sqlmock.New()
	if err != nil {
		t.Fatalf("failed to create mock: %v", err)
	}
	defer db.Close()

	mock.ExpectExec(`DELETE FROM notifications WHERE id NOT IN`).
		WithArgs(100).
		WillReturnResult(sqlmock.NewResult(0, 50))

	repo := NewNotificationRepository(db)
	deleted, err := repo.KeepRecent(100)

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
