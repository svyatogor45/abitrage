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

	// Валидация критичных параметров безопасности
	if err := cfg.validateSecurity(); err != nil {
		return nil, err
	}

	// Валидация числовых диапазонов
	if err := cfg.validateRanges(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// validateSecurity проверяет параметры безопасности
func (c *Config) validateSecurity() error {
	// ENCRYPTION_KEY обязателен для шифрования API ключей бирж
	if c.Security.EncryptionKey == "" {
		return fmt.Errorf("ENCRYPTION_KEY is required for encrypting API keys")
	}

	if len(c.Security.EncryptionKey) != 32 {
		return fmt.Errorf("ENCRYPTION_KEY must be exactly 32 bytes for AES-256")
	}

	// JWT_SECRET обязателен и не должен быть default значением
	if c.Security.JWTSecret == "" {
		return fmt.Errorf("JWT_SECRET is required for authentication")
	}

	if c.Security.JWTSecret == "change-me-in-production" {
		return fmt.Errorf("JWT_SECRET must be changed from default value in production")
	}

	if len(c.Security.JWTSecret) < 32 {
		return fmt.Errorf("JWT_SECRET must be at least 32 characters for security")
	}

	return nil
}

// validateRanges проверяет числовые диапазоны параметров
func (c *Config) validateRanges() error {
	// Валидация портов
	if c.Server.Port < 1 || c.Server.Port > 65535 {
		return fmt.Errorf("SERVER_PORT must be between 1 and 65535, got %d", c.Server.Port)
	}

	if c.Database.Port < 1 || c.Database.Port > 65535 {
		return fmt.Errorf("DB_PORT must be between 1 and 65535, got %d", c.Database.Port)
	}

	// Валидация retry параметров
	if c.Bot.MaxRetries < 0 {
		return fmt.Errorf("MAX_RETRIES cannot be negative, got %d", c.Bot.MaxRetries)
	}

	if c.Bot.MaxRetries > 10 {
		return fmt.Errorf("MAX_RETRIES should not exceed 10, got %d", c.Bot.MaxRetries)
	}

	// Валидация таймаутов (должны быть положительными)
	if c.Bot.OrderTimeout <= 0 {
		return fmt.Errorf("ORDER_TIMEOUT must be positive, got %v", c.Bot.OrderTimeout)
	}

	if c.Bot.WSReadTimeout <= 0 {
		return fmt.Errorf("WS_READ_TIMEOUT must be positive, got %v", c.Bot.WSReadTimeout)
	}

	// Валидация MaxConcurrentArbs (0 = без лимита, иначе > 0)
	if c.Bot.MaxConcurrentArbs < 0 {
		return fmt.Errorf("MAX_CONCURRENT_ARBS cannot be negative, got %d", c.Bot.MaxConcurrentArbs)
	}

	// Валидация SessionTimeout
	if c.Security.SessionTimeout < 60 {
		return fmt.Errorf("SESSION_TIMEOUT must be at least 60 seconds, got %d", c.Security.SessionTimeout)
	}

	return nil
}

// DSN возвращает строку подключения к базе данных
func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Password, d.Name, d.SSLMode)
}

// DSNWithoutPassword возвращает строку подключения без пароля (для логирования)
func (d DatabaseConfig) DSNWithoutPassword() string {
	return fmt.Sprintf("host=%s port=%d user=%s dbname=%s sslmode=%s",
		d.Host, d.Port, d.User, d.Name, d.SSLMode)
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
