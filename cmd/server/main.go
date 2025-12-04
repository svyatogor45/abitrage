package main

import (
	"context"
	"database/sql"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"arbitrage/internal/api"
	"arbitrage/internal/config"
	"arbitrage/internal/repository"
	"arbitrage/internal/service"
	"arbitrage/internal/websocket"
	"arbitrage/pkg/utils"

	_ "github.com/lib/pq"
)

func main() {
	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		// Используем стандартный log до инициализации логгера
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// Инициализация структурированного логгера
	logger := utils.InitGlobalLogger(utils.LogConfig{
		Level:       cfg.Logging.Level,
		Format:      cfg.Logging.Format,
		Development: cfg.Logging.Level == "debug",
	})
	defer logger.Sync()

	utils.Info("Configuration loaded successfully",
		utils.String("log_level", cfg.Logging.Level),
		utils.String("log_format", cfg.Logging.Format),
	)

	// Инициализация базы данных
	db, err := initDatabase(cfg, logger)
	if err != nil {
		utils.Fatal("Failed to connect to database", utils.Err(err))
	}
	defer db.Close()

	utils.Info("Connected to database successfully",
		utils.String("driver", cfg.Database.Driver),
	)

	// Инициализация репозиториев
	exchangeRepo := repository.NewExchangeRepository(db)
	pairRepo := repository.NewPairRepository(db)
	statsRepo := repository.NewStatsRepository(db)
	settingsRepo := repository.NewSettingsRepository(db)
	notificationRepo := repository.NewNotificationRepository(db)
	blacklistRepo := repository.NewBlacklistRepository(db)

	// Инициализация сервисов
	exchangeService := service.NewExchangeService(
		exchangeRepo,
		pairRepo,
		cfg.Security.EncryptionKey,
	)

	// Инициализация PairService
	pairService := service.NewPairService(
		pairRepo,
		exchangeRepo,
		exchangeService,
	)

	// Инициализация StatsService
	statsService := service.NewStatsService(
		statsRepo,
		pairRepo,
	)

	// Инициализация SettingsService
	settingsService := service.NewSettingsService(settingsRepo)

	// Инициализация NotificationService
	notificationService := service.NewNotificationService(
		notificationRepo,
		settingsRepo,
	)

	// Инициализация BlacklistService
	blacklistService := service.NewBlacklistService(blacklistRepo)

	// Инициализация WebSocket hub
	wsHub := websocket.NewHub()
	go wsHub.Run()
	utils.Info("WebSocket hub started")

	// Подключение WebSocket hub к сервисам для real-time обновлений
	notificationService.SetWebSocketHub(wsHub)
	exchangeService.SetWebSocketHub(wsHub)
	statsService.SetWebSocketHub(wsHub)

	// TODO: Инициализация бота
	// botEngine := bot.NewEngine(db, hub)
	// go botEngine.Run()

	// Настройка зависимостей для API
	deps := &api.Dependencies{
		ExchangeService:     exchangeService,
		PairService:         pairService,
		StatsService:        statsService,
		SettingsService:     settingsService,
		NotificationService: notificationService,
		BlacklistService:    blacklistService,
		Hub:                 wsHub,
	}

	// Настройка HTTP роутера
	router := api.SetupRoutes(deps)

	// HTTP сервер с конфигурируемыми таймаутами
	serverAddr := fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port)
	server := &http.Server{
		Addr:         serverAddr,
		Handler:      router,
		ReadTimeout:  cfg.Server.ReadTimeout,
		WriteTimeout: cfg.Server.WriteTimeout,
		IdleTimeout:  cfg.Server.IdleTimeout,
	}

	// Запуск сервера в отдельной горутине
	go func() {
		utils.Info("Starting HTTP server",
			utils.String("addr", serverAddr),
			utils.Bool("https", cfg.Server.UseHTTPS),
		)

		var serverErr error
		if cfg.Server.UseHTTPS {
			serverErr = server.ListenAndServeTLS(cfg.Server.CertFile, cfg.Server.KeyFile)
		} else {
			serverErr = server.ListenAndServe()
		}

		if serverErr != nil && serverErr != http.ErrServerClosed {
			utils.Fatal("Server failed", utils.Err(serverErr))
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	sig := <-quit

	utils.Info("Received shutdown signal",
		utils.String("signal", sig.String()),
	)

	// Останавливаем WebSocket hub (graceful shutdown)
	wsHub.Stop()
	utils.Info("WebSocket hub stopped")

	// Закрываем соединения с биржами
	if err := exchangeService.Close(); err != nil {
		utils.Error("Error closing exchange connections", utils.Err(err))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	utils.Info("Shutting down server gracefully",
		utils.Duration("timeout", 30*time.Second),
	)

	if err := server.Shutdown(ctx); err != nil {
		utils.Fatal("Server forced to shutdown", utils.Err(err))
	}

	utils.Info("Server exited successfully")
}

// initDatabase создает подключение к базе данных
func initDatabase(cfg *config.Config, logger *utils.Logger) (*sql.DB, error) {
	dsn := cfg.Database.DSN()

	utils.Debug("Connecting to database",
		utils.String("dsn", cfg.Database.DSNWithoutPassword()),
	)

	db, err := sql.Open(cfg.Database.Driver, dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Настройка пула соединений
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	// Проверка подключения
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	return db, nil
}
