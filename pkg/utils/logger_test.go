package utils

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// ============================================================
// Тесты InitLogger
// ============================================================

func TestInitLogger_Defaults(t *testing.T) {
	// Тест с пустой конфигурацией - должны применяться значения по умолчанию
	logger := InitLogger(LogConfig{})

	if logger == nil {
		t.Fatal("InitLogger returned nil")
	}
	if logger.Logger == nil {
		t.Fatal("Logger.Logger is nil")
	}
	if logger.sugar == nil {
		t.Fatal("Logger.sugar is nil")
	}
}

func TestInitLogger_JSONFormat(t *testing.T) {
	logger := InitLogger(LogConfig{
		Level:  "info",
		Format: "json",
	})

	if logger == nil {
		t.Fatal("InitLogger returned nil")
	}
}

func TestInitLogger_TextFormat(t *testing.T) {
	logger := InitLogger(LogConfig{
		Level:  "debug",
		Format: "text",
	})

	if logger == nil {
		t.Fatal("InitLogger returned nil")
	}
}

func TestInitLogger_DevelopmentMode(t *testing.T) {
	logger := InitLogger(LogConfig{
		Level:       "debug",
		Format:      "text",
		Development: true,
	})

	if logger == nil {
		t.Fatal("InitLogger returned nil")
	}
}

func TestInitLogger_AllLevels(t *testing.T) {
	levels := []string{"debug", "info", "warn", "warning", "error", "fatal", "invalid"}

	for _, level := range levels {
		t.Run(level, func(t *testing.T) {
			logger := InitLogger(LogConfig{Level: level})
			if logger == nil {
				t.Fatalf("InitLogger returned nil for level %s", level)
			}
		})
	}
}

func TestInitLogger_FileOutput(t *testing.T) {
	// Создаём временный файл
	tmpFile, err := os.CreateTemp("", "logger_test_*.log")
	if err != nil {
		t.Fatalf("Failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name())
	tmpFile.Close()

	logger := InitLogger(LogConfig{
		Level:  "info",
		Format: "json",
		Output: tmpFile.Name(),
	})

	if logger == nil {
		t.Fatal("InitLogger returned nil")
	}

	// Логируем сообщение
	logger.Info("Test message", zap.String("key", "value"))
	logger.Sync()

	// Читаем файл
	content, err := os.ReadFile(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}

	if len(content) == 0 {
		t.Error("Log file is empty")
	}

	// Проверяем что это валидный JSON
	var logEntry map[string]interface{}
	if err := json.Unmarshal(content, &logEntry); err != nil {
		t.Errorf("Log entry is not valid JSON: %v", err)
	}
}

func TestInitLogger_InvalidFileOutput(t *testing.T) {
	// Пытаемся записать в несуществующую директорию
	logger := InitLogger(LogConfig{
		Level:  "info",
		Output: "/nonexistent/directory/log.txt",
	})

	// Должен fallback на stderr, не паниковать
	if logger == nil {
		t.Fatal("InitLogger returned nil for invalid output")
	}
}

// ============================================================
// Тесты глобального логгера
// ============================================================

func TestGlobalLogger(t *testing.T) {
	// Сбрасываем глобальный логгер
	globalMu.Lock()
	globalLogger = nil
	globalMu.Unlock()

	// GetGlobalLogger должен создать логгер по умолчанию
	logger := GetGlobalLogger()
	if logger == nil {
		t.Fatal("GetGlobalLogger returned nil")
	}

	// Повторный вызов должен вернуть тот же логгер
	logger2 := GetGlobalLogger()
	if logger != logger2 {
		t.Error("GetGlobalLogger returned different loggers")
	}

	// L() должен вернуть тот же логгер
	logger3 := L()
	if logger != logger3 {
		t.Error("L() returned different logger")
	}
}

func TestInitGlobalLogger(t *testing.T) {
	config := LogConfig{
		Level:  "debug",
		Format: "text",
	}

	logger := InitGlobalLogger(config)
	if logger == nil {
		t.Fatal("InitGlobalLogger returned nil")
	}

	// Глобальный логгер должен быть установлен
	globalLogger := GetGlobalLogger()
	if globalLogger != logger {
		t.Error("Global logger was not set")
	}
}

func TestSetGlobalLogger(t *testing.T) {
	logger := InitLogger(LogConfig{Level: "warn"})
	SetGlobalLogger(logger)

	if GetGlobalLogger() != logger {
		t.Error("SetGlobalLogger did not set the logger")
	}
}

// ============================================================
// Тесты parseLevel
// ============================================================

func TestParseLevel(t *testing.T) {
	tests := []struct {
		input    string
		expected zapcore.Level
	}{
		{"debug", zapcore.DebugLevel},
		{"DEBUG", zapcore.DebugLevel},
		{"info", zapcore.InfoLevel},
		{"INFO", zapcore.InfoLevel},
		{"warn", zapcore.WarnLevel},
		{"WARN", zapcore.WarnLevel},
		{"warning", zapcore.WarnLevel},
		{"error", zapcore.ErrorLevel},
		{"ERROR", zapcore.ErrorLevel},
		{"fatal", zapcore.FatalLevel},
		{"invalid", zapcore.InfoLevel}, // default
		{"", zapcore.InfoLevel},        // default
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := parseLevel(tt.input)
			if result != tt.expected {
				t.Errorf("parseLevel(%q) = %v, want %v", tt.input, result, tt.expected)
			}
		})
	}
}

// ============================================================
// Тесты методов Logger
// ============================================================

func TestLogger_With(t *testing.T) {
	logger := InitLogger(LogConfig{Level: "info"})

	newLogger := logger.With(zap.String("key", "value"))

	if newLogger == nil {
		t.Fatal("With returned nil")
	}
	if newLogger == logger {
		t.Error("With should return a new logger")
	}
}

func TestLogger_WithHelpers(t *testing.T) {
	logger := InitLogger(LogConfig{Level: "info"})

	tests := []struct {
		name   string
		helper func() *Logger
	}{
		{"WithComponent", func() *Logger { return logger.WithComponent("test") }},
		{"WithExchange", func() *Logger { return logger.WithExchange("bybit") }},
		{"WithSymbol", func() *Logger { return logger.WithSymbol("BTCUSDT") }},
		{"WithPairID", func() *Logger { return logger.WithPairID(123) }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			newLogger := tt.helper()
			if newLogger == nil {
				t.Fatalf("%s returned nil", tt.name)
			}
			if newLogger == logger {
				t.Errorf("%s should return a new logger", tt.name)
			}
		})
	}
}

func TestLogger_Sugar(t *testing.T) {
	logger := InitLogger(LogConfig{Level: "info"})

	sugar := logger.Sugar()
	if sugar == nil {
		t.Fatal("Sugar returned nil")
	}
}

// ============================================================
// Тесты глобальных функций логирования
// ============================================================

func TestGlobalLoggingFunctions(t *testing.T) {
	// Создаём буфер для захвата вывода
	var buf bytes.Buffer

	// Создаём тестовый логгер
	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			MessageKey: "message",
			LevelKey:   "level",
		}),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	)
	testLogger := &Logger{
		Logger: zap.New(core),
		sugar:  zap.New(core).Sugar(),
	}
	SetGlobalLogger(testLogger)

	// Тестируем функции
	Debug("debug message", zap.String("key", "debug"))
	Info("info message", zap.String("key", "info"))
	Warn("warn message", zap.String("key", "warn"))
	Error("error message", zap.String("key", "error"))

	testLogger.Sync()

	output := buf.String()

	// Проверяем что все сообщения записаны
	if !strings.Contains(output, "debug message") {
		t.Error("Debug message not found in output")
	}
	if !strings.Contains(output, "info message") {
		t.Error("Info message not found in output")
	}
	if !strings.Contains(output, "warn message") {
		t.Error("Warn message not found in output")
	}
	if !strings.Contains(output, "error message") {
		t.Error("Error message not found in output")
	}
}

func TestGlobalFormattedLoggingFunctions(t *testing.T) {
	var buf bytes.Buffer

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			MessageKey: "message",
			LevelKey:   "level",
		}),
		zapcore.AddSync(&buf),
		zapcore.DebugLevel,
	)
	testLogger := &Logger{
		Logger: zap.New(core),
		sugar:  zap.New(core).Sugar(),
	}
	SetGlobalLogger(testLogger)

	Debugf("debug %s %d", "test", 1)
	Infof("info %s %d", "test", 2)
	Warnf("warn %s %d", "test", 3)
	Errorf("error %s %d", "test", 4)

	testLogger.Sync()

	output := buf.String()

	if !strings.Contains(output, "debug test 1") {
		t.Error("Debugf message not found")
	}
	if !strings.Contains(output, "info test 2") {
		t.Error("Infof message not found")
	}
	if !strings.Contains(output, "warn test 3") {
		t.Error("Warnf message not found")
	}
	if !strings.Contains(output, "error test 4") {
		t.Error("Errorf message not found")
	}
}

// ============================================================
// Тесты конструкторов полей
// ============================================================

func TestFieldConstructors(t *testing.T) {
	var buf bytes.Buffer

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zapcore.EncoderConfig{
			MessageKey: "message",
			LevelKey:   "level",
		}),
		zapcore.AddSync(&buf),
		zapcore.InfoLevel,
	)
	testLogger := &Logger{
		Logger: zap.New(core),
		sugar:  zap.New(core).Sugar(),
	}

	// Тестируем все кастомные конструкторы полей
	testLogger.Info("test",
		Exchange("bybit"),
		Symbol("BTCUSDT"),
		PairID(123),
		OrderID("order-456"),
		Price(25000.50),
		Volume(0.5),
		Spread(1.5),
		PNL(100.25),
		Side("long"),
		State("active"),
		Latency(15.5),
		RequestID("req-789"),
		UserID(1),
		Component("engine"),
	)

	testLogger.Sync()
	output := buf.String()

	// Проверяем что поля присутствуют
	expectedFields := []string{
		"exchange", "bybit",
		"symbol", "BTCUSDT",
		"pair_id", "123",
		"order_id", "order-456",
		"price", "25000.5",
		"volume", "0.5",
		"spread", "1.5",
		"pnl", "100.25",
		"side", "long",
		"state", "active",
		"latency_ms", "15.5",
		"request_id", "req-789",
		"user_id", "1",
		"component", "engine",
	}

	for _, field := range expectedFields {
		if !strings.Contains(output, field) {
			t.Errorf("Field %q not found in output: %s", field, output)
		}
	}
}

func TestReexportedFieldConstructors(t *testing.T) {
	// Проверяем что переэкспортированные функции работают
	_ = String("key", "value")
	_ = Int("key", 42)
	_ = Int64("key", 42)
	_ = Float64("key", 3.14)
	_ = Bool("key", true)
	_ = Err(nil)
	_ = Any("key", struct{}{})
}

// ============================================================
// Тесты fieldsToInterface
// ============================================================

func TestFieldsToInterface(t *testing.T) {
	fields := []zap.Field{
		zap.String("key1", "value1"),
		zap.Int("key2", 42),
	}

	result := fieldsToInterface(fields)

	if len(result) != 4 {
		t.Errorf("Expected 4 elements, got %d", len(result))
	}

	// Проверяем ключи
	if result[0] != "key1" {
		t.Errorf("Expected key1, got %v", result[0])
	}
	if result[2] != "key2" {
		t.Errorf("Expected key2, got %v", result[2])
	}
}

// ============================================================
// Бенчмарки
// ============================================================

func BenchmarkLogger_Info(b *testing.B) {
	logger := InitLogger(LogConfig{
		Level:  "info",
		Format: "json",
		Output: "/dev/null", // Не записываем никуда
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.Info("Benchmark message",
			zap.String("key", "value"),
			zap.Int("count", i),
		)
	}
}

func BenchmarkLogger_Sugar_Infof(b *testing.B) {
	logger := InitLogger(LogConfig{
		Level:  "info",
		Format: "json",
		Output: "/dev/null",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		logger.sugar.Infof("Benchmark message key=%s count=%d", "value", i)
	}
}

func BenchmarkGlobal_Info(b *testing.B) {
	InitGlobalLogger(LogConfig{
		Level:  "info",
		Format: "json",
		Output: "/dev/null",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Info("Benchmark message",
			String("key", "value"),
			Int("count", i),
		)
	}
}

func BenchmarkLogger_With(b *testing.B) {
	logger := InitLogger(LogConfig{
		Level:  "info",
		Format: "json",
		Output: "/dev/null",
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		childLogger := logger.With(
			zap.String("exchange", "bybit"),
			zap.String("symbol", "BTCUSDT"),
		)
		childLogger.Info("Message")
	}
}
