package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"arbitrage/internal/api"
	"arbitrage/internal/config"
	"arbitrage/internal/repository"
	"arbitrage/internal/service"

	_ "github.com/lib/pq"
)

func main() {
	// Загрузка конфигурации
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	// Инициализация базы данных
	db, err := initDatabase(cfg)
	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	log.Println("Connected to database successfully")

	// Инициализация репозиториев
	exchangeRepo := repository.NewExchangeRepository(db)
	pairRepo := repository.NewPairRepository(db)
	// notificationRepo := repository.NewNotificationRepository(db)
	// statsRepo := repository.NewStatsRepository(db)
	// blacklistRepo := repository.NewBlacklistRepository(db)
	// settingsRepo := repository.NewSettingsRepository(db)

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

	// TODO: инициализировать другие сервисы когда они будут реализованы
	// notificationService := service.NewNotificationService(...)
	// statsService := service.NewStatsService(...)

	// TODO: Инициализация WebSocket hub
	// hub := websocket.NewHub()
	// go hub.Run()

	// TODO: Инициализация бота
	// botEngine := bot.NewEngine(db, hub)
	// go botEngine.Run()

	// Настройка зависимостей для API
	deps := &api.Dependencies{
		ExchangeService: exchangeService,
		PairService:     pairService,
		// NotificationService: notificationService,
		// StatsService:        statsService,
	}

	// Настройка HTTP роутера
	router := api.SetupRoutes(deps)

	// HTTP сервер
	server := &http.Server{
		Addr:         fmt.Sprintf("%s:%d", cfg.Server.Host, cfg.Server.Port),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Запуск сервера в отдельной горутине
	go func() {
		log.Printf("Starting server on %s", server.Addr)
		if cfg.Server.UseHTTPS {
			if err := server.ListenAndServeTLS(cfg.Server.CertFile, cfg.Server.KeyFile); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Server failed: %v", err)
			}
		} else {
			if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Fatalf("Server failed: %v", err)
			}
		}
	}()

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("Shutting down server...")

	// Закрываем соединения с биржами
	if err := exchangeService.Close(); err != nil {
		log.Printf("Error closing exchange connections: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}

// initDatabase создает подключение к базе данных
func initDatabase(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.Name,
		cfg.Database.SSLMode,
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
