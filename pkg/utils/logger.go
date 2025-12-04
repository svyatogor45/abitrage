package utils

import (
	"os"
	"strings"
	"sync"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// logger.go - структурированное логирование на основе zap
//
// Назначение:
// Предоставляет единый интерфейс логирования для всего приложения.
// Использует uber-go/zap для высокопроизводительного структурированного логирования.
//
// Особенности:
// - Уровни логирования: DEBUG, INFO, WARN, ERROR
// - Форматы вывода: JSON (production), text/console (development)
// - Структурированные поля для контекста
// - Thread-safe глобальный логгер
// - Zero-allocation в hot path (для INFO и выше)
//
// Использование:
//   logger := utils.InitLogger(utils.LogConfig{Level: "info", Format: "json"})
//   logger.Info("Server started", zap.String("addr", ":8080"))
//   // Или через глобальный логгер:
//   utils.Info("Message", zap.Int("count", 42))

// LogConfig конфигурация логгера
type LogConfig struct {
	// Level уровень логирования: debug, info, warn, error
	Level string

	// Format формат вывода: json, text (console)
	Format string

	// Output путь к файлу или "stdout"/"stderr"
	// По умолчанию: stderr
	Output string

	// Development режим разработки (более читаемый вывод, stack traces)
	Development bool
}

// Logger обёртка над zap.Logger с дополнительными методами
type Logger struct {
	*zap.Logger
	sugar *zap.SugaredLogger
}

// глобальный логгер (инициализируется через InitLogger или InitGlobalLogger)
var (
	globalLogger *Logger
	globalMu     sync.RWMutex
)

// defaultConfig возвращает конфигурацию по умолчанию
func defaultConfig() LogConfig {
	return LogConfig{
		Level:       "info",
		Format:      "json",
		Output:      "stderr",
		Development: false,
	}
}

// InitLogger создаёт новый логгер с указанной конфигурацией
//
// Параметры:
//   - config: конфигурация логгера
//
// Возвращает:
//   - *Logger: инициализированный логгер
//
// Пример:
//
//	logger := InitLogger(LogConfig{Level: "debug", Format: "text"})
//	logger.Info("Application started")
func InitLogger(config LogConfig) *Logger {
	// Применяем значения по умолчанию
	if config.Level == "" {
		config.Level = "info"
	}
	if config.Format == "" {
		config.Format = "json"
	}
	if config.Output == "" {
		config.Output = "stderr"
	}

	// Парсим уровень логирования
	level := parseLevel(config.Level)

	// Создаём encoder config
	encoderConfig := zapcore.EncoderConfig{
		TimeKey:        "timestamp",
		LevelKey:       "level",
		NameKey:        "logger",
		CallerKey:      "caller",
		FunctionKey:    zapcore.OmitKey,
		MessageKey:     "message",
		StacktraceKey:  "stacktrace",
		LineEnding:     zapcore.DefaultLineEnding,
		EncodeLevel:    zapcore.LowercaseLevelEncoder,
		EncodeTime:     zapcore.ISO8601TimeEncoder,
		EncodeDuration: zapcore.MillisDurationEncoder,
		EncodeCaller:   zapcore.ShortCallerEncoder,
	}

	// Для development режима используем цветной вывод
	if config.Development || config.Format == "text" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoderConfig.EncodeTime = zapcore.TimeEncoderOfLayout("15:04:05.000")
	}

	// Выбираем encoder
	var encoder zapcore.Encoder
	if config.Format == "json" {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	}

	// Выбираем output
	var writeSyncer zapcore.WriteSyncer
	switch strings.ToLower(config.Output) {
	case "stdout":
		writeSyncer = zapcore.AddSync(os.Stdout)
	case "stderr", "":
		writeSyncer = zapcore.AddSync(os.Stderr)
	default:
		// Файл - открываем для записи
		file, err := os.OpenFile(config.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			// Fallback на stderr если не удалось открыть файл
			writeSyncer = zapcore.AddSync(os.Stderr)
		} else {
			writeSyncer = zapcore.AddSync(file)
		}
	}

	// Создаём core
	core := zapcore.NewCore(encoder, writeSyncer, level)

	// Опции логгера
	opts := []zap.Option{
		zap.AddCaller(),
		zap.AddCallerSkip(1), // Пропускаем обёртку Logger
	}

	if config.Development {
		opts = append(opts, zap.Development())
		opts = append(opts, zap.AddStacktrace(zapcore.WarnLevel))
	} else {
		opts = append(opts, zap.AddStacktrace(zapcore.ErrorLevel))
	}

	zapLogger := zap.New(core, opts...)

	return &Logger{
		Logger: zapLogger,
		sugar:  zapLogger.Sugar(),
	}
}

// InitGlobalLogger инициализирует глобальный логгер
// Должен вызываться один раз при старте приложения
func InitGlobalLogger(config LogConfig) *Logger {
	logger := InitLogger(config)
	SetGlobalLogger(logger)
	return logger
}

// SetGlobalLogger устанавливает глобальный логгер
func SetGlobalLogger(logger *Logger) {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalLogger = logger
}

// GetGlobalLogger возвращает глобальный логгер
// Если не инициализирован, создаёт логгер с настройками по умолчанию
func GetGlobalLogger() *Logger {
	globalMu.RLock()
	if globalLogger != nil {
		defer globalMu.RUnlock()
		return globalLogger
	}
	globalMu.RUnlock()

	// Инициализируем с настройками по умолчанию
	return InitGlobalLogger(defaultConfig())
}

// L возвращает глобальный логгер (короткий алиас)
func L() *Logger {
	return GetGlobalLogger()
}

// parseLevel преобразует строку в zapcore.Level
func parseLevel(level string) zapcore.Level {
	switch strings.ToLower(level) {
	case "debug":
		return zapcore.DebugLevel
	case "info":
		return zapcore.InfoLevel
	case "warn", "warning":
		return zapcore.WarnLevel
	case "error":
		return zapcore.ErrorLevel
	case "fatal":
		return zapcore.FatalLevel
	default:
		return zapcore.InfoLevel
	}
}

// ============================================================
// Методы Logger для удобного использования
// ============================================================

// Sugar возвращает SugaredLogger для более простого API
func (l *Logger) Sugar() *zap.SugaredLogger {
	return l.sugar
}

// With создаёт новый логгер с дополнительными полями
func (l *Logger) With(fields ...zap.Field) *Logger {
	return &Logger{
		Logger: l.Logger.With(fields...),
		sugar:  l.sugar.With(fieldsToInterface(fields)...),
	}
}

// WithComponent создаёт логгер с указанным компонентом
func (l *Logger) WithComponent(component string) *Logger {
	return l.With(zap.String("component", component))
}

// WithExchange создаёт логгер с указанной биржей
func (l *Logger) WithExchange(exchange string) *Logger {
	return l.With(zap.String("exchange", exchange))
}

// WithSymbol создаёт логгер с указанным символом
func (l *Logger) WithSymbol(symbol string) *Logger {
	return l.With(zap.String("symbol", symbol))
}

// WithPairID создаёт логгер с указанным ID пары
func (l *Logger) WithPairID(pairID int) *Logger {
	return l.With(zap.Int("pair_id", pairID))
}

// Sync сбрасывает буферы логгера
func (l *Logger) Sync() error {
	return l.Logger.Sync()
}

// ============================================================
// Глобальные функции-шорткаты
// ============================================================

// Debug логирует сообщение уровня DEBUG
func Debug(msg string, fields ...zap.Field) {
	GetGlobalLogger().Debug(msg, fields...)
}

// Info логирует сообщение уровня INFO
func Info(msg string, fields ...zap.Field) {
	GetGlobalLogger().Info(msg, fields...)
}

// Warn логирует сообщение уровня WARN
func Warn(msg string, fields ...zap.Field) {
	GetGlobalLogger().Warn(msg, fields...)
}

// Error логирует сообщение уровня ERROR
func Error(msg string, fields ...zap.Field) {
	GetGlobalLogger().Error(msg, fields...)
}

// Fatal логирует сообщение уровня FATAL и завершает программу
func Fatal(msg string, fields ...zap.Field) {
	GetGlobalLogger().Fatal(msg, fields...)
}

// Debugf логирует форматированное сообщение уровня DEBUG
func Debugf(template string, args ...interface{}) {
	GetGlobalLogger().sugar.Debugf(template, args...)
}

// Infof логирует форматированное сообщение уровня INFO
func Infof(template string, args ...interface{}) {
	GetGlobalLogger().sugar.Infof(template, args...)
}

// Warnf логирует форматированное сообщение уровня WARN
func Warnf(template string, args ...interface{}) {
	GetGlobalLogger().sugar.Warnf(template, args...)
}

// Errorf логирует форматированное сообщение уровня ERROR
func Errorf(template string, args ...interface{}) {
	GetGlobalLogger().sugar.Errorf(template, args...)
}

// Fatalf логирует форматированное сообщение уровня FATAL и завершает программу
func Fatalf(template string, args ...interface{}) {
	GetGlobalLogger().sugar.Fatalf(template, args...)
}

// ============================================================
// Вспомогательные функции и типы полей
// ============================================================

// fieldsToInterface конвертирует zap.Field в []interface{} для SugaredLogger
func fieldsToInterface(fields []zap.Field) []interface{} {
	result := make([]interface{}, 0, len(fields)*2)
	for _, f := range fields {
		result = append(result, f.Key, f.Interface)
	}
	return result
}

// ============================================================
// Переэкспорт типов и функций zap для удобства импорта
// ============================================================

// Field алиас для zap.Field
type Field = zap.Field

// Переэкспорт конструкторов полей из zap
var (
	String     = zap.String
	Int        = zap.Int
	Int64      = zap.Int64
	Float64    = zap.Float64
	Bool       = zap.Bool
	Duration   = zap.Duration
	Time       = zap.Time
	Err        = zap.Error
	Any        = zap.Any
	Stringer   = zap.Stringer
	ByteString = zap.ByteString
)

// ============================================================
// Дополнительные конструкторы полей для арбитражного бота
// ============================================================

// Exchange создаёт поле с именем биржи
func Exchange(name string) Field {
	return zap.String("exchange", name)
}

// Symbol создаёт поле с символом торговой пары
func Symbol(symbol string) Field {
	return zap.String("symbol", symbol)
}

// PairID создаёт поле с ID пары
func PairID(id int) Field {
	return zap.Int("pair_id", id)
}

// OrderID создаёт поле с ID ордера
func OrderID(id string) Field {
	return zap.String("order_id", id)
}

// Price создаёт поле с ценой
func Price(price float64) Field {
	return zap.Float64("price", price)
}

// Volume создаёт поле с объёмом
func Volume(volume float64) Field {
	return zap.Float64("volume", volume)
}

// Spread создаёт поле со спредом
func Spread(spread float64) Field {
	return zap.Float64("spread", spread)
}

// PNL создаёт поле с прибылью/убытком
func PNL(pnl float64) Field {
	return zap.Float64("pnl", pnl)
}

// Side создаёт поле со стороной сделки (long/short)
func Side(side string) Field {
	return zap.String("side", side)
}

// State создаёт поле с состоянием
func State(state string) Field {
	return zap.String("state", state)
}

// Latency создаёт поле с задержкой в миллисекундах
func Latency(ms float64) Field {
	return zap.Float64("latency_ms", ms)
}

// RequestID создаёт поле с ID запроса
func RequestID(id string) Field {
	return zap.String("request_id", id)
}

// UserID создаёт поле с ID пользователя
func UserID(id int) Field {
	return zap.Int("user_id", id)
}

// Component создаёт поле с именем компонента
func Component(name string) Field {
	return zap.String("component", name)
}
