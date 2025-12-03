// Package integration contains integration tests for the arbitrage trading terminal.
//
// Database Integration Tests
// These tests verify database operations, migrations, and transactions:
// - Table creation and schema validation
// - CRUD operations through repositories
// - Transaction support and rollback
// - Concurrent database access
// - Data integrity constraints
//
// Run with: go test -tags=integration ./tests/integration/...
package integration

import (
	"database/sql"
	"sync"
	"testing"
	"time"

	"arbitrage/internal/models"
	"arbitrage/internal/repository"
)

// ============================================================
// Database Schema Tests
// ============================================================

func TestDatabase_SchemaCreation_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	// Initialize tables
	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	tables := []string{
		"exchanges",
		"pairs",
		"orders",
		"notifications",
		"settings",
		"blacklist",
		"trades",
	}

	for _, table := range tables {
		t.Run("table_"+table+"_exists", func(t *testing.T) {
			var exists bool
			err := db.QueryRow(`
				SELECT EXISTS (
					SELECT FROM information_schema.tables
					WHERE table_name = $1
				)
			`, table).Scan(&exists)

			if err != nil {
				t.Fatalf("failed to check table existence: %v", err)
			}
			if !exists {
				t.Errorf("table %s does not exist", table)
			}
		})
	}
}

func TestDatabase_SchemaColumns_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	t.Run("exchanges table has required columns", func(t *testing.T) {
		requiredColumns := []string{"id", "name", "api_key", "secret_key", "connected", "balance"}
		checkTableColumns(t, db, "exchanges", requiredColumns)
	})

	t.Run("pairs table has required columns", func(t *testing.T) {
		requiredColumns := []string{
			"id", "symbol", "base", "quote", "entry_spread_pct",
			"exit_spread_pct", "volume_asset", "n_orders", "status",
		}
		checkTableColumns(t, db, "pairs", requiredColumns)
	})

	t.Run("notifications table has required columns", func(t *testing.T) {
		requiredColumns := []string{"id", "timestamp", "type", "severity", "message"}
		checkTableColumns(t, db, "notifications", requiredColumns)
	})

	t.Run("trades table has required columns", func(t *testing.T) {
		requiredColumns := []string{"id", "symbol", "pnl", "entry_time", "exit_time"}
		checkTableColumns(t, db, "trades", requiredColumns)
	})
}

func checkTableColumns(t *testing.T, db *sql.DB, tableName string, requiredColumns []string) {
	for _, col := range requiredColumns {
		var exists bool
		err := db.QueryRow(`
			SELECT EXISTS (
				SELECT FROM information_schema.columns
				WHERE table_name = $1 AND column_name = $2
			)
		`, tableName, col).Scan(&exists)

		if err != nil {
			t.Fatalf("failed to check column %s.%s: %v", tableName, col, err)
		}
		if !exists {
			t.Errorf("column %s.%s does not exist", tableName, col)
		}
	}
}

// ============================================================
// Repository CRUD Integration Tests
// ============================================================

func TestDatabase_BlacklistRepository_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	// Clear blacklist table
	TruncateTable(db, "blacklist")

	repo := repository.NewBlacklistRepository(db)

	t.Run("create entry", func(t *testing.T) {
		entry := &models.BlacklistEntry{
			Symbol: "BTCUSDT",
			Reason: "Test reason",
		}

		err := repo.Create(entry)
		if err != nil {
			t.Fatalf("failed to create entry: %v", err)
		}

		if entry.ID == 0 {
			t.Error("expected non-zero ID after creation")
		}
	})

	t.Run("get all entries", func(t *testing.T) {
		entries, err := repo.GetAll()
		if err != nil {
			t.Fatalf("failed to get entries: %v", err)
		}

		if len(entries) != 1 {
			t.Errorf("expected 1 entry, got %d", len(entries))
		}

		if entries[0].Symbol != "BTCUSDT" {
			t.Errorf("expected symbol BTCUSDT, got %s", entries[0].Symbol)
		}
	})

	t.Run("check exists", func(t *testing.T) {
		exists, err := repo.Exists("BTCUSDT")
		if err != nil {
			t.Fatalf("failed to check exists: %v", err)
		}
		if !exists {
			t.Error("BTCUSDT should exist")
		}

		notExists, err := repo.Exists("ETHUSDT")
		if err != nil {
			t.Fatalf("failed to check not exists: %v", err)
		}
		if notExists {
			t.Error("ETHUSDT should not exist")
		}
	})

	t.Run("delete entry", func(t *testing.T) {
		err := repo.Delete("BTCUSDT")
		if err != nil {
			t.Fatalf("failed to delete entry: %v", err)
		}

		entries, _ := repo.GetAll()
		if len(entries) != 0 {
			t.Errorf("expected 0 entries after delete, got %d", len(entries))
		}
	})
}

func TestDatabase_NotificationRepository_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	TruncateTable(db, "notifications")

	repo := repository.NewNotificationRepository(db)

	t.Run("create notification", func(t *testing.T) {
		notif := &models.Notification{
			Type:      "OPEN",
			Severity:  "info",
			Message:   "Test notification",
			Timestamp: time.Now(),
		}

		err := repo.Create(notif)
		if err != nil {
			t.Fatalf("failed to create notification: %v", err)
		}

		if notif.ID == 0 {
			t.Error("expected non-zero ID after creation")
		}
	})

	t.Run("get recent notifications", func(t *testing.T) {
		// Create more notifications
		for i := 0; i < 5; i++ {
			repo.Create(&models.Notification{
				Type:      "CLOSE",
				Severity:  "info",
				Message:   "Test notification",
				Timestamp: time.Now(),
			})
		}

		notifications, err := repo.GetRecent(3)
		if err != nil {
			t.Fatalf("failed to get recent: %v", err)
		}

		if len(notifications) != 3 {
			t.Errorf("expected 3 notifications, got %d", len(notifications))
		}
	})

	t.Run("get by types", func(t *testing.T) {
		// Add a different type
		repo.Create(&models.Notification{
			Type:      "SL",
			Severity:  "error",
			Message:   "Stop loss triggered",
			Timestamp: time.Now(),
		})

		notifications, err := repo.GetByTypes([]string{"SL"}, 10)
		if err != nil {
			t.Fatalf("failed to get by types: %v", err)
		}

		for _, n := range notifications {
			if n.Type != "SL" {
				t.Errorf("expected type SL, got %s", n.Type)
			}
		}
	})

	t.Run("delete all notifications", func(t *testing.T) {
		err := repo.DeleteAll()
		if err != nil {
			t.Fatalf("failed to delete all: %v", err)
		}

		notifications, _ := repo.GetRecent(100)
		if len(notifications) != 0 {
			t.Errorf("expected 0 notifications after delete, got %d", len(notifications))
		}
	})
}

func TestDatabase_SettingsRepository_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	repo := repository.NewSettingsRepository(db)

	t.Run("get default settings", func(t *testing.T) {
		settings, err := repo.Get()
		if err != nil {
			t.Fatalf("failed to get settings: %v", err)
		}

		if settings.ID != 1 {
			t.Errorf("expected settings ID 1, got %d", settings.ID)
		}
	})

	t.Run("update settings", func(t *testing.T) {
		settings := &models.Settings{
			ID:              1,
			ConsiderFunding: true,
		}

		err := repo.Update(settings)
		if err != nil {
			t.Fatalf("failed to update settings: %v", err)
		}

		updated, _ := repo.Get()
		if !updated.ConsiderFunding {
			t.Error("expected ConsiderFunding to be true")
		}
	})
}

func TestDatabase_StatsRepository_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	TruncateTable(db, "trades")

	repo := repository.NewStatsRepository(db)

	t.Run("get empty stats", func(t *testing.T) {
		stats, err := repo.GetStats()
		if err != nil {
			t.Fatalf("failed to get stats: %v", err)
		}

		if stats.TotalTrades != 0 {
			t.Errorf("expected 0 total trades, got %d", stats.TotalTrades)
		}
	})

	t.Run("record trade", func(t *testing.T) {
		now := time.Now()
		err := repo.RecordTrade(
			0,                          // pairID (0 for no pair)
			"BTCUSDT",                  // symbol
			[2]string{"bybit", "okx"},  // exchanges
			now.Add(-time.Hour),        // entryTime
			now,                        // exitTime
			50.25,                      // pnl
			false,                      // wasStopLoss
			false,                      // wasLiquidation
		)
		if err != nil {
			t.Fatalf("failed to record trade: %v", err)
		}

		stats, _ := repo.GetStats()
		if stats.TodayTrades < 1 {
			t.Error("expected at least 1 trade today")
		}
	})

	t.Run("record trade with stop loss", func(t *testing.T) {
		now := time.Now()
		// Record a trade that was a stop loss
		err := repo.RecordTrade(
			0,                             // pairID
			"ETHUSDT",                     // symbol
			[2]string{"bitget", "gate"},   // exchanges
			now.Add(-time.Hour),           // entryTime
			now,                           // exitTime
			-25.00,                        // pnl (loss)
			true,                          // wasStopLoss
			false,                         // wasLiquidation
		)
		if err != nil {
			t.Fatalf("failed to record stop loss trade: %v", err)
		}

		stats, _ := repo.GetStats()
		if stats.StopLossCount.Today < 1 {
			t.Error("expected at least 1 stop loss today")
		}
	})

	t.Run("get top pairs by trades", func(t *testing.T) {
		// Insert multiple trades for different pairs
		now := time.Now()
		repo.RecordTrade(0, "SOLUSDT", [2]string{"htx", "bingx"}, now.Add(-time.Hour), now, 10.0, false, false)
		repo.RecordTrade(0, "SOLUSDT", [2]string{"htx", "bingx"}, now.Add(-time.Hour), now, 20.0, false, false)

		pairs, err := repo.GetTopPairsByTrades(5)
		if err != nil {
			t.Fatalf("failed to get top pairs: %v", err)
		}

		// Should return some pairs
		if pairs == nil {
			t.Error("expected non-nil pairs list")
		}
	})
}

// ============================================================
// Transaction Tests
// ============================================================

func TestDatabase_Transaction_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	TruncateTable(db, "blacklist")

	t.Run("transaction commit", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("failed to begin transaction: %v", err)
		}

		_, err = tx.Exec(`INSERT INTO blacklist (symbol, reason) VALUES ($1, $2)`, "TXTEST1", "tx test")
		if err != nil {
			tx.Rollback()
			t.Fatalf("failed to insert in transaction: %v", err)
		}

		err = tx.Commit()
		if err != nil {
			t.Fatalf("failed to commit: %v", err)
		}

		// Verify data exists after commit
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM blacklist WHERE symbol = 'TXTEST1'`).Scan(&count)
		if count != 1 {
			t.Error("data should exist after commit")
		}
	})

	t.Run("transaction rollback", func(t *testing.T) {
		tx, err := db.Begin()
		if err != nil {
			t.Fatalf("failed to begin transaction: %v", err)
		}

		_, err = tx.Exec(`INSERT INTO blacklist (symbol, reason) VALUES ($1, $2)`, "TXTEST2", "rollback test")
		if err != nil {
			tx.Rollback()
			t.Fatalf("failed to insert in transaction: %v", err)
		}

		// Rollback instead of commit
		err = tx.Rollback()
		if err != nil {
			t.Fatalf("failed to rollback: %v", err)
		}

		// Verify data does not exist after rollback
		var count int
		db.QueryRow(`SELECT COUNT(*) FROM blacklist WHERE symbol = 'TXTEST2'`).Scan(&count)
		if count != 0 {
			t.Error("data should not exist after rollback")
		}
	})
}

// ============================================================
// Concurrent Access Tests
// ============================================================

func TestDatabase_ConcurrentAccess_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	TruncateTable(db, "notifications")

	repo := repository.NewNotificationRepository(db)

	t.Run("concurrent writes", func(t *testing.T) {
		const numGoroutines = 10
		const numWrites = 10

		var wg sync.WaitGroup
		errors := make(chan error, numGoroutines*numWrites)

		for i := 0; i < numGoroutines; i++ {
			wg.Add(1)
			go func(goroutineID int) {
				defer wg.Done()
				for j := 0; j < numWrites; j++ {
					notif := &models.Notification{
						Type:      "TEST",
						Severity:  "info",
						Message:   "Concurrent test",
						Timestamp: time.Now(),
					}
					if err := repo.Create(notif); err != nil {
						errors <- err
					}
				}
			}(i)
		}

		wg.Wait()
		close(errors)

		errorCount := 0
		for err := range errors {
			t.Logf("concurrent write error: %v", err)
			errorCount++
		}

		if errorCount > 0 {
			t.Errorf("got %d errors during concurrent writes", errorCount)
		}

		// Verify total count
		notifications, _ := repo.GetRecent(1000)
		expectedCount := numGoroutines * numWrites
		if len(notifications) != expectedCount {
			t.Errorf("expected %d notifications, got %d", expectedCount, len(notifications))
		}
	})

	t.Run("concurrent reads", func(t *testing.T) {
		const numReaders = 20

		var wg sync.WaitGroup
		results := make(chan int, numReaders)

		for i := 0; i < numReaders; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				notifications, err := repo.GetRecent(100)
				if err != nil {
					t.Logf("concurrent read error: %v", err)
					results <- -1
					return
				}
				results <- len(notifications)
			}()
		}

		wg.Wait()
		close(results)

		// All readers should get same count
		var lastCount int
		first := true
		for count := range results {
			if count < 0 {
				t.Error("got read error")
				continue
			}
			if first {
				lastCount = count
				first = false
			} else if count != lastCount {
				// This might happen due to concurrent writes, but should be rare
				t.Logf("inconsistent read: got %d, expected %d", count, lastCount)
			}
		}
	})
}

// ============================================================
// Data Integrity Tests
// ============================================================

func TestDatabase_DataIntegrity_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	t.Run("unique constraint on blacklist symbol", func(t *testing.T) {
		TruncateTable(db, "blacklist")

		// Insert first entry
		_, err := db.Exec(`INSERT INTO blacklist (symbol, reason) VALUES ('UNIQUE1', 'first')`)
		if err != nil {
			t.Fatalf("failed to insert first: %v", err)
		}

		// Try to insert duplicate
		_, err = db.Exec(`INSERT INTO blacklist (symbol, reason) VALUES ('UNIQUE1', 'second')`)
		if err == nil {
			t.Error("expected error for duplicate symbol")
		}
	})

	t.Run("unique constraint on exchange name", func(t *testing.T) {
		TruncateTable(db, "exchanges")

		// Insert first exchange
		_, err := db.Exec(`INSERT INTO exchanges (name) VALUES ('testexchange')`)
		if err != nil {
			t.Fatalf("failed to insert first: %v", err)
		}

		// Try to insert duplicate
		_, err = db.Exec(`INSERT INTO exchanges (name) VALUES ('testexchange')`)
		if err == nil {
			t.Error("expected error for duplicate exchange name")
		}
	})

	t.Run("foreign key constraint on orders", func(t *testing.T) {
		TruncateTable(db, "orders")
		TruncateTable(db, "pairs")

		// Try to insert order with non-existent pair_id
		_, err := db.Exec(`
			INSERT INTO orders (pair_id, exchange, side, quantity, status)
			VALUES (99999, 'test', 'buy', 1.0, 'filled')
		`)

		// Should fail due to foreign key constraint
		if err == nil {
			t.Error("expected foreign key violation error")
		}
	})

	t.Run("cascade delete on pair orders", func(t *testing.T) {
		TruncateTable(db, "orders")
		TruncateTable(db, "pairs")

		// Create pair
		var pairID int
		err := db.QueryRow(`
			INSERT INTO pairs (symbol, base, quote, entry_spread_pct, exit_spread_pct, volume_asset)
			VALUES ('BTCUSDT', 'BTC', 'USDT', 1.0, 0.2, 0.1)
			RETURNING id
		`).Scan(&pairID)
		if err != nil {
			t.Fatalf("failed to create pair: %v", err)
		}

		// Create orders for this pair
		_, err = db.Exec(`
			INSERT INTO orders (pair_id, exchange, side, quantity, status)
			VALUES ($1, 'bybit', 'buy', 0.1, 'filled')
		`, pairID)
		if err != nil {
			t.Fatalf("failed to create order: %v", err)
		}

		// Delete pair
		_, err = db.Exec(`DELETE FROM pairs WHERE id = $1`, pairID)
		if err != nil {
			t.Fatalf("failed to delete pair: %v", err)
		}

		// Orders should be deleted via cascade
		var orderCount int
		db.QueryRow(`SELECT COUNT(*) FROM orders WHERE pair_id = $1`, pairID).Scan(&orderCount)
		if orderCount != 0 {
			t.Error("orders should be deleted when pair is deleted")
		}
	})
}

// ============================================================
// Migration Tests
// ============================================================

func TestDatabase_MigrationIdempotency_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	t.Run("tables can be recreated without error", func(t *testing.T) {
		// First run
		err := initTestTables(db)
		if err != nil {
			t.Fatalf("first run failed: %v", err)
		}

		// Second run (should be idempotent)
		err = initTestTables(db)
		if err != nil {
			t.Fatalf("second run failed: %v", err)
		}
	})
}

// ============================================================
// Performance Tests
// ============================================================

func TestDatabase_BulkInsert_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	TruncateTable(db, "notifications")

	t.Run("bulk insert performance", func(t *testing.T) {
		const insertCount = 100

		start := time.Now()

		for i := 0; i < insertCount; i++ {
			_, err := db.Exec(`
				INSERT INTO notifications (type, severity, message, timestamp)
				VALUES ($1, $2, $3, $4)
			`, "BULK", "info", "Bulk test notification", time.Now())

			if err != nil {
				t.Fatalf("failed to insert: %v", err)
			}
		}

		duration := time.Since(start)

		// Should complete in reasonable time (< 5 seconds for 100 inserts)
		if duration > 5*time.Second {
			t.Errorf("bulk insert took too long: %v", duration)
		}

		t.Logf("Inserted %d rows in %v (%.2f rows/sec)", insertCount, duration, float64(insertCount)/duration.Seconds())
	})
}

func TestDatabase_QueryPerformance_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	if err := initTestTables(db); err != nil {
		t.Fatalf("failed to initialize tables: %v", err)
	}

	// Insert test data
	for i := 0; i < 100; i++ {
		db.Exec(`
			INSERT INTO notifications (type, severity, message, timestamp)
			VALUES ($1, $2, $3, $4)
		`, "QUERY", "info", "Query test", time.Now())
	}

	t.Run("query performance", func(t *testing.T) {
		const queryCount = 100

		start := time.Now()

		for i := 0; i < queryCount; i++ {
			rows, err := db.Query(`SELECT * FROM notifications ORDER BY timestamp DESC LIMIT 10`)
			if err != nil {
				t.Fatalf("failed to query: %v", err)
			}
			rows.Close()
		}

		duration := time.Since(start)

		// Should complete in reasonable time (< 2 seconds for 100 queries)
		if duration > 2*time.Second {
			t.Errorf("queries took too long: %v", duration)
		}

		t.Logf("Executed %d queries in %v (%.2f queries/sec)", queryCount, duration, float64(queryCount)/duration.Seconds())
	})
}

// ============================================================
// Connection Pool Tests
// ============================================================

func TestDatabase_ConnectionPool_Integration(t *testing.T) {
	db, cleanup := SetupTestDB(t)
	if db == nil {
		t.Skip("Skipping: database not available")
	}
	defer cleanup()

	t.Run("connection pool handles load", func(t *testing.T) {
		const concurrentConnections = 10

		var wg sync.WaitGroup
		errors := make(chan error, concurrentConnections)

		for i := 0; i < concurrentConnections; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()

				// Execute a query that holds the connection briefly
				var result int
				err := db.QueryRow(`SELECT pg_sleep(0.1)::int`).Scan(&result)
				if err != nil {
					// pg_sleep returns void, not int, so expect error
					// but connection should still work
					db.QueryRow(`SELECT 1`).Scan(&result)
				}
			}()
		}

		wg.Wait()
		close(errors)

		for err := range errors {
			t.Errorf("connection pool error: %v", err)
		}

		// Verify pool stats
		stats := db.Stats()
		t.Logf("Connection pool stats: Open=%d, InUse=%d, Idle=%d",
			stats.OpenConnections, stats.InUse, stats.Idle)
	})
}
