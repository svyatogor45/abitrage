package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config содержит всю конфигурацию приложения
type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Security SecurityConfig
	Bot      BotConfig
	Logging  LoggingConfig
}

// ServerConfig - настройки HTTP сервера
type ServerConfig struct {
	Port     int
	Host     string
	UseHTTPS bool
	CertFile string
	KeyFile  string
}

// DatabaseConfig - настройки подключения к БД
type DatabaseConfig struct {
	Driver   string
	Host     string
	Port     int
	Name     string
	User     string
	Password string
	SSLMode  string
}

// SecurityConfig - настройки безопасности
type SecurityConfig struct {
	JWTSecret      string
	EncryptionKey  string
	SessionTimeout int
}

// BotConfig - настройки бота
type BotConfig struct {
	// WebSocket настройки (event-driven, без polling)
	WSReconnectDelay  time.Duration // задержка перед переподключением WS
	WSPingInterval    time.Duration // интервал ping для поддержания соединения
	WSReadTimeout     time.Duration // таймаут чтения WS сообщений

	// Периодические задачи (не влияют на торговлю)
	BalanceUpdateFreq time.Duration // обновление балансов для UI
	StatsUpdateFreq   time.Duration // обновление статистики для UI

	// Retry логика для критических операций
	MaxRetries      int
	RetryBackoff    time.Duration
	OrderTimeout    time.Duration // таймаут ожидания исполнения ордера

	// Торговые параметры
	MaxConcurrentArbs int // максимум одновременных арбитражей (0 = без лимита)
}

// LoggingConfig - настройки логирования
type LoggingConfig struct {
	Level  string
	Format string
}

// Load загружает конфигурацию из переменных окружения
func Load() (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:     getEnvAsInt("SERVER_PORT", 8080),
			Host:     getEnv("SERVER_HOST", "0.0.0.0"),
			UseHTTPS: getEnvAsBool("USE_HTTPS", false),
			CertFile: getEnv("CERT_FILE", ""),
			KeyFile:  getEnv("KEY_FILE", ""),
		},
		Database: DatabaseConfig{
			Driver:   getEnv("DB_DRIVER", "postgres"),
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnvAsInt("DB_PORT", 5432),
			Name:     getEnv("DB_NAME", "arbitrage"),
			User:     getEnv("DB_USER", "user"),
			Password: getEnv("DB_PASSWORD", "password"),
			SSLMode:  getEnv("DB_SSL_MODE", "disable"),
		},
		Security: SecurityConfig{
			JWTSecret:      getEnv("JWT_SECRET", "change-me-in-production"),
			EncryptionKey:  getEnv("ENCRYPTION_KEY", ""),
			SessionTimeout: getEnvAsInt("SESSION_TIMEOUT", 3600),
		},
		Bot: BotConfig{
			// WebSocket - event-driven, без polling!
			WSReconnectDelay:  getEnvAsDuration("WS_RECONNECT_DELAY", 1*time.Second),
			WSPingInterval:    getEnvAsDuration("WS_PING_INTERVAL", 15*time.Second),
			WSReadTimeout:     getEnvAsDuration("WS_READ_TIMEOUT", 30*time.Second),

			// Периодические задачи для UI (не критичны для торговли)
			BalanceUpdateFreq: getEnvAsDuration("BALANCE_UPDATE_FREQ", 1*time.Minute),
			StatsUpdateFreq:   getEnvAsDuration("STATS_UPDATE_FREQ", 5*time.Second),

			// Retry для ордеров
			MaxRetries:   getEnvAsInt("MAX_RETRIES", 4),
			RetryBackoff: getEnvAsDuration("RETRY_BACKOFF", 500*time.Millisecond),
			OrderTimeout: getEnvAsDuration("ORDER_TIMEOUT", 5*time.Second),

			// Торговые лимиты
			MaxConcurrentArbs: getEnvAsInt("MAX_CONCURRENT_ARBS", 0), // 0 = без лимита
		},
		Logging: LoggingConfig{
			Level:  getEnv("LOG_LEVEL", "info"),
			Format: getEnv("LOG_FORMAT", "json"),
		},
	}

	// Валидация критичных параметров
	if cfg.Security.EncryptionKey == "" {
		return nil, fmt.Errorf("ENCRYPTION_KEY is required for encrypting API keys")
	}

	if len(cfg.Security.EncryptionKey) != 32 {
		return nil, fmt.Errorf("ENCRYPTION_KEY must be exactly 32 bytes for AES-256")
	}

	return cfg, nil
}

// Вспомогательные функции для чтения переменных окружения

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

func getEnvAsDuration(key string, defaultValue time.Duration) time.Duration {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := time.ParseDuration(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
