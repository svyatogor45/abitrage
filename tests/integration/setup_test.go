// Package integration contains integration tests for the arbitrage trading terminal.
//
// These tests verify the correct interaction between components:
// - API integration tests: full HTTP request cycle
// - WebSocket tests: connection, broadcast messaging
// - Database tests: migrations, transactions
//
// Integration tests use build tag "integration" to separate from unit tests.
// Run with: go test -tags=integration ./tests/integration/...
package integration

import (
	"database/sql"
	"fmt"
	"log"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"arbitrage/internal/api"
	"arbitrage/internal/api/handlers"
	"arbitrage/internal/repository"
	"arbitrage/internal/service"
	"arbitrage/internal/websocket"

	"github.com/gorilla/mux"
	_ "github.com/lib/pq"
)

// TestConfig contains configuration for integration tests
type TestConfig struct {
	DBDriver   string
	DBHost     string
	DBPort     string
	DBName     string
	DBUser     string
	DBPassword string
	DBSSLMode  string
}

// TestServer encapsulates all components needed for integration testing
type TestServer struct {
	DB          *sql.DB
	Router      *mux.Router
	Server      *httptest.Server
	Hub         *websocket.Hub
	Repos       *TestRepositories
	Services    *TestServices
	Handlers    *TestHandlers
	Cleanup     func()
}

// TestRepositories contains all repository instances for testing
type TestRepositories struct {
	Exchange     *repository.ExchangeRepository
	Pair         *repository.PairRepository
	Order        *repository.OrderRepository
	Notification *repository.NotificationRepository
	Settings     *repository.SettingsRepository
	Blacklist    *repository.BlacklistRepository
	Stats        *repository.StatsRepository
}

// TestServices contains all service instances for testing
type TestServices struct {
	Exchange     *service.ExchangeService
	Pair         *service.PairService
	Stats        *service.StatsService
	Settings     *service.SettingsService
	Notification *service.NotificationService
	Blacklist    *service.BlacklistService
}

// TestHandlers contains all handler instances for testing
type TestHandlers struct {
	Exchange     *handlers.ExchangeHandler
	Pair         *handlers.PairHandler
	Stats        *handlers.StatsHandler
	Settings     *handlers.SettingsHandler
	Notification *handlers.NotificationHandler
	Blacklist    *handlers.BlacklistHandler
}

// getTestConfig returns configuration from environment variables or defaults
func getTestConfig() TestConfig {
	return TestConfig{
		DBDriver:   getEnv("TEST_DB_DRIVER", "postgres"),
		DBHost:     getEnv("TEST_DB_HOST", "localhost"),
		DBPort:     getEnv("TEST_DB_PORT", "5432"),
		DBName:     getEnv("TEST_DB_NAME", "arbitrage_test"),
		DBUser:     getEnv("TEST_DB_USER", "postgres"),
		DBPassword: getEnv("TEST_DB_PASSWORD", "postgres"),
		DBSSLMode:  getEnv("TEST_DB_SSLMODE", "disable"),
	}
}

// getEnv returns environment variable value or default
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// SetupTestDB creates a test database connection
func SetupTestDB(t *testing.T) (*sql.DB, func()) {
	config := getTestConfig()

	connStr := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		config.DBHost, config.DBPort, config.DBUser, config.DBPassword, config.DBName, config.DBSSLMode,
	)

	db, err := sql.Open(config.DBDriver, connStr)
	if err != nil {
		t.Skipf("Skipping integration test: cannot connect to database: %v", err)
		return nil, func() {}
	}

	// Test connection
	if err := db.Ping(); err != nil {
		t.Skipf("Skipping integration test: cannot ping database: %v", err)
		return nil, func() {}
	}

	// Set connection pool settings
	db.SetMaxOpenConns(5)
	db.SetMaxIdleConns(2)
	db.SetConnMaxLifetime(5 * time.Minute)

	cleanup := func() {
		if err := db.Close(); err != nil {
			log.Printf("Error closing database: %v", err)
		}
	}

	return db, cleanup
}

// SetupTestServer creates a complete test server with all components
func SetupTestServer(t *testing.T) *TestServer {
	db, dbCleanup := SetupTestDB(t)
	if db == nil {
		return nil
	}

	// Initialize tables
	if err := initTestTables(db); err != nil {
		t.Skipf("Skipping integration test: cannot initialize tables: %v", err)
		return nil
	}

	// Create WebSocket hub
	hub := websocket.NewHub()
	go hub.Run()

	// Create repositories
	repos := &TestRepositories{
		Exchange:     repository.NewExchangeRepository(db),
		Pair:         repository.NewPairRepository(db),
		Order:        repository.NewOrderRepository(db),
		Notification: repository.NewNotificationRepository(db),
		Settings:     repository.NewSettingsRepository(db),
		Blacklist:    repository.NewBlacklistRepository(db),
		Stats:        repository.NewStatsRepository(db),
	}

	// Create services
	// Note: Some services need additional dependencies, using minimal setup for testing
	exchangeSvc := service.NewExchangeService(repos.Exchange, repos.Pair, "test-encryption-key-32bytes!!")
	services := &TestServices{
		Exchange:     exchangeSvc,
		Pair:         service.NewPairService(repos.Pair, repos.Exchange, exchangeSvc),
		Stats:        service.NewStatsService(repos.Stats, repos.Pair),
		Settings:     service.NewSettingsService(repos.Settings),
		Notification: service.NewNotificationService(repos.Notification, repos.Settings),
		Blacklist:    service.NewBlacklistService(repos.Blacklist),
	}
	// Set WebSocket hub for notification service
	services.Notification.SetWebSocketHub(hub)

	// Create handlers
	testHandlers := &TestHandlers{
		Stats:        handlers.NewStatsHandler(services.Stats),
		Settings:     handlers.NewSettingsHandler(services.Settings),
		Notification: handlers.NewNotificationHandler(services.Notification),
		Blacklist:    handlers.NewBlacklistHandler(services.Blacklist),
	}

	// Setup router
	deps := &api.Dependencies{
		StatsService:        services.Stats,
		SettingsService:     services.Settings,
		NotificationService: services.Notification,
		BlacklistService:    services.Blacklist,
		Hub:                 hub,
	}
	router := api.SetupRoutes(deps)

	// Create test server
	server := httptest.NewServer(router)

	cleanup := func() {
		server.Close()
		cleanupTestTables(db)
		dbCleanup()
	}

	return &TestServer{
		DB:       db,
		Router:   router,
		Server:   server,
		Hub:      hub,
		Repos:    repos,
		Services: services,
		Handlers: testHandlers,
		Cleanup:  cleanup,
	}
}

// initTestTables creates or truncates tables for testing
func initTestTables(db *sql.DB) error {
	// Create tables if not exist
	tables := []string{
		`CREATE TABLE IF NOT EXISTS exchanges (
			id SERIAL PRIMARY KEY,
			name VARCHAR(50) UNIQUE NOT NULL,
			api_key TEXT NOT NULL DEFAULT '',
			secret_key TEXT NOT NULL DEFAULT '',
			passphrase TEXT DEFAULT '',
			connected BOOLEAN DEFAULT false,
			balance DECIMAL(20, 8) DEFAULT 0,
			last_error TEXT DEFAULT '',
			updated_at TIMESTAMP DEFAULT NOW(),
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS pairs (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) NOT NULL,
			base VARCHAR(10) NOT NULL,
			quote VARCHAR(10) NOT NULL,
			entry_spread_pct DECIMAL(10, 4) NOT NULL,
			exit_spread_pct DECIMAL(10, 4) NOT NULL,
			volume_asset DECIMAL(20, 8) NOT NULL,
			n_orders INT DEFAULT 1,
			stop_loss DECIMAL(20, 2),
			status VARCHAR(20) DEFAULT 'paused',
			trades_count INT DEFAULT 0,
			total_pnl DECIMAL(20, 2) DEFAULT 0,
			created_at TIMESTAMP DEFAULT NOW(),
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS orders (
			id SERIAL PRIMARY KEY,
			pair_id INT REFERENCES pairs(id) ON DELETE CASCADE,
			exchange VARCHAR(50) NOT NULL,
			side VARCHAR(10) NOT NULL,
			type VARCHAR(20) DEFAULT 'market',
			part_index INT DEFAULT 0,
			quantity DECIMAL(20, 8) NOT NULL,
			price_avg DECIMAL(20, 8),
			status VARCHAR(20) NOT NULL,
			error_message TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT NOW(),
			filled_at TIMESTAMP
		)`,
		`CREATE TABLE IF NOT EXISTS notifications (
			id SERIAL PRIMARY KEY,
			timestamp TIMESTAMP DEFAULT NOW(),
			type VARCHAR(50) NOT NULL,
			severity VARCHAR(10) DEFAULT 'info',
			pair_id INT,
			message TEXT NOT NULL,
			meta JSONB DEFAULT '{}'
		)`,
		`CREATE TABLE IF NOT EXISTS settings (
			id INT PRIMARY KEY DEFAULT 1,
			consider_funding BOOLEAN DEFAULT false,
			max_concurrent_trades INT,
			notification_prefs JSONB DEFAULT '{"open":true,"close":true,"stop_loss":true,"liquidation":true,"api_error":true,"margin":true,"pause":true,"second_leg_fail":true}',
			updated_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS blacklist (
			id SERIAL PRIMARY KEY,
			symbol VARCHAR(20) UNIQUE NOT NULL,
			reason TEXT DEFAULT '',
			created_at TIMESTAMP DEFAULT NOW()
		)`,
		`CREATE TABLE IF NOT EXISTS trades (
			id SERIAL PRIMARY KEY,
			pair_id INT,
			symbol VARCHAR(20) NOT NULL,
			exchanges VARCHAR(100) DEFAULT '',
			entry_time TIMESTAMP NOT NULL DEFAULT NOW(),
			exit_time TIMESTAMP NOT NULL DEFAULT NOW(),
			pnl DECIMAL(20, 2) NOT NULL DEFAULT 0,
			was_stop_loss BOOLEAN DEFAULT false,
			was_liquidation BOOLEAN DEFAULT false,
			created_at TIMESTAMP DEFAULT NOW()
		)`,
	}

	for _, table := range tables {
		if _, err := db.Exec(table); err != nil {
			return fmt.Errorf("failed to create table: %w", err)
		}
	}

	// Insert default settings if not exists
	_, err := db.Exec(`INSERT INTO settings (id) VALUES (1) ON CONFLICT (id) DO NOTHING`)
	if err != nil {
		return fmt.Errorf("failed to insert default settings: %w", err)
	}

	return nil
}

// cleanupTestTables truncates all test tables
func cleanupTestTables(db *sql.DB) {
	tables := []string{
		"trades",
		"orders",
		"notifications",
		"blacklist",
		"pairs",
		"exchanges",
	}

	for _, table := range tables {
		db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", table))
	}
}

// TruncateTable truncates a specific table for testing
func TruncateTable(db *sql.DB, tableName string) error {
	_, err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s CASCADE", tableName))
	return err
}
