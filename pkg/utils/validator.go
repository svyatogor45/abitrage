package utils

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// validator.go - валидация данных
//
// Назначение:
// Проверка корректности входных данных для арбитражного бота.
// Все функции возвращают error с описанием проблемы или nil при успехе.
//
// Функции:
// - ValidateSymbol: проверка формата символа (BTCUSDT)
// - ValidateSpread: проверка спреда (> 0)
// - ValidateVolume: проверка объема (> 0)
// - ValidateNOrders: проверка количества ордеров (≥ 1)
// - ValidateEmail: проверка email формата
// - ValidateAPIKey: базовая проверка API ключа
//
// Использование:
// - Валидация входящих HTTP запросов
// - Проверка параметров торговых пар
// - Валидация настроек пользователя

// ============================================================
// Ошибки валидации
// ============================================================

var (
	ErrEmptySymbol       = errors.New("symbol cannot be empty")
	ErrInvalidSymbol     = errors.New("invalid symbol format")
	ErrSymbolTooLong     = errors.New("symbol is too long")
	ErrInvalidSpread     = errors.New("spread must be greater than 0")
	ErrSpreadTooLarge    = errors.New("spread is too large")
	ErrInvalidVolume     = errors.New("volume must be greater than 0")
	ErrVolumeTooLarge    = errors.New("volume is too large")
	ErrInvalidNOrders    = errors.New("n_orders must be at least 1")
	ErrNOrdersTooLarge   = errors.New("n_orders is too large")
	ErrEmptyEmail        = errors.New("email cannot be empty")
	ErrInvalidEmail      = errors.New("invalid email format")
	ErrEmptyAPIKey       = errors.New("API key cannot be empty")
	ErrInvalidAPIKey     = errors.New("invalid API key format")
	ErrEmptyAPISecret    = errors.New("API secret cannot be empty")
	ErrInvalidAPISecret  = errors.New("invalid API secret format")
	ErrInvalidExchange   = errors.New("invalid exchange name")
	ErrInvalidStopLoss   = errors.New("stop loss must be greater than 0")
	ErrInvalidLeverage   = errors.New("leverage must be between 1 and 100")
	ErrInvalidPercentage = errors.New("percentage must be between 0 and 100")
)

// ============================================================
// Регулярные выражения
// ============================================================

var (
	// symbolRegex - допустимый формат символа: только буквы и цифры, минимум 2 символа
	// Примеры: BTCUSDT, ETHUSDT, BTC-USDT, BTC_USDT
	symbolRegex = regexp.MustCompile(`^[A-Z0-9]{2,20}$`)

	// symbolWithSeparatorRegex - символ с разделителем
	symbolWithSeparatorRegex = regexp.MustCompile(`^[A-Z0-9]{2,10}[-_/][A-Z0-9]{2,10}$`)

	// emailRegex - базовый паттерн для email
	emailRegex = regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)

	// apiKeyRegex - базовый паттерн для API ключа (буквы, цифры, дефисы)
	apiKeyRegex = regexp.MustCompile(`^[A-Za-z0-9\-_]{16,128}$`)
)

// ============================================================
// Константы
// ============================================================

const (
	MaxSymbolLength = 30
	MaxSpread       = 100.0  // Максимальный спред 100%
	MaxVolume       = 1e9    // Максимальный объем (1 млрд USDT)
	MinVolume       = 1e-8   // Минимальный объем
	MaxNOrders      = 100    // Максимальное количество ордеров для разбиения
	MinAPIKeyLength = 16     // Минимальная длина API ключа
	MaxAPIKeyLength = 128    // Максимальная длина API ключа
	MaxStopLoss     = 100.0  // Максимальный стоп-лосс (100%)
	MaxLeverage     = 100    // Максимальное плечо
)

// SupportedExchanges - список поддерживаемых бирж
var SupportedExchanges = []string{
	"bybit",
	"bitget",
	"okx",
	"gate",
	"htx",
	"bingx",
}

// ============================================================
// Валидация символов
// ============================================================

// ValidateSymbol проверяет формат торгового символа
//
// Допустимые форматы:
//   - BTCUSDT (без разделителя)
//   - BTC-USDT (с дефисом)
//   - BTC_USDT (с подчеркиванием)
//   - BTC/USDT (со слешем)
//
// Параметры:
//   - symbol: торговый символ
//
// Возвращает:
//   - nil: символ валиден
//   - error: описание проблемы
//
// Пример:
//
//	if err := ValidateSymbol("BTCUSDT"); err != nil {
//	    return fmt.Errorf("invalid symbol: %w", err)
//	}
func ValidateSymbol(symbol string) error {
	if symbol == "" {
		return ErrEmptySymbol
	}

	// Приводим к верхнему регистру
	symbol = strings.ToUpper(symbol)

	if len(symbol) > MaxSymbolLength {
		return ErrSymbolTooLong
	}

	// Проверяем формат без разделителя
	if symbolRegex.MatchString(symbol) {
		return nil
	}

	// Проверяем формат с разделителем
	if symbolWithSeparatorRegex.MatchString(symbol) {
		return nil
	}

	return ErrInvalidSymbol
}

// NormalizeSymbol приводит символ к стандартному формату (без разделителей, uppercase)
//
// Примеры:
//   - "btc-usdt" -> "BTCUSDT"
//   - "BTC_USDT" -> "BTCUSDT"
//   - "btc/usdt" -> "BTCUSDT"
func NormalizeSymbol(symbol string) string {
	symbol = strings.ToUpper(symbol)
	symbol = strings.ReplaceAll(symbol, "-", "")
	symbol = strings.ReplaceAll(symbol, "_", "")
	symbol = strings.ReplaceAll(symbol, "/", "")
	return symbol
}

// ExtractBaseCurrency извлекает базовую валюту из символа
//
// Примеры:
//   - "BTCUSDT" -> "BTC"
//   - "ETHUSDT" -> "ETH"
//   - "BTC-USDT" -> "BTC"
func ExtractBaseCurrency(symbol string) string {
	symbol = strings.ToUpper(symbol)

	// Проверяем разделители
	for _, sep := range []string{"-", "_", "/"} {
		if idx := strings.Index(symbol, sep); idx > 0 {
			return symbol[:idx]
		}
	}

	// Ищем известные quote валюты
	quoteCurrencies := []string{"USDT", "USDC", "USD", "BUSD", "BTC", "ETH"}
	for _, quote := range quoteCurrencies {
		if strings.HasSuffix(symbol, quote) {
			return symbol[:len(symbol)-len(quote)]
		}
	}

	return symbol
}

// ExtractQuoteCurrency извлекает котируемую валюту из символа
//
// Примеры:
//   - "BTCUSDT" -> "USDT"
//   - "ETHBTC" -> "BTC"
func ExtractQuoteCurrency(symbol string) string {
	symbol = strings.ToUpper(symbol)

	// Проверяем разделители
	for _, sep := range []string{"-", "_", "/"} {
		if idx := strings.Index(symbol, sep); idx > 0 {
			return symbol[idx+1:]
		}
	}

	// Ищем известные quote валюты
	quoteCurrencies := []string{"USDT", "USDC", "USD", "BUSD", "BTC", "ETH"}
	for _, quote := range quoteCurrencies {
		if strings.HasSuffix(symbol, quote) {
			return quote
		}
	}

	return ""
}

// ============================================================
// Валидация числовых параметров
// ============================================================

// ValidateSpread проверяет значение спреда
//
// Спред должен быть:
//   - Больше 0
//   - Меньше MaxSpread (100%)
//
// Параметры:
//   - spread: значение спреда в процентах
//
// Возвращает:
//   - nil: спред валиден
//   - error: описание проблемы
func ValidateSpread(spread float64) error {
	if spread <= 0 {
		return ErrInvalidSpread
	}
	if spread > MaxSpread {
		return ErrSpreadTooLarge
	}
	return nil
}

// ValidateVolume проверяет значение объема
//
// Объем должен быть:
//   - Больше MinVolume (1e-8)
//   - Меньше MaxVolume (1e9)
//
// Параметры:
//   - volume: значение объема
//
// Возвращает:
//   - nil: объем валиден
//   - error: описание проблемы
func ValidateVolume(volume float64) error {
	if volume <= 0 || volume < MinVolume {
		return ErrInvalidVolume
	}
	if volume > MaxVolume {
		return ErrVolumeTooLarge
	}
	return nil
}

// ValidateNOrders проверяет количество ордеров для разбиения объема
//
// Количество должно быть:
//   - Не меньше 1
//   - Не больше MaxNOrders (100)
//
// Параметры:
//   - n: количество ордеров
//
// Возвращает:
//   - nil: значение валидно
//   - error: описание проблемы
func ValidateNOrders(n int) error {
	if n < 1 {
		return ErrInvalidNOrders
	}
	if n > MaxNOrders {
		return ErrNOrdersTooLarge
	}
	return nil
}

// ValidateStopLoss проверяет значение стоп-лосса
//
// StopLoss должен быть:
//   - Больше 0
//   - Меньше или равен MaxStopLoss (100%)
//
// Параметры:
//   - sl: значение стоп-лосса в процентах
func ValidateStopLoss(sl float64) error {
	if sl <= 0 {
		return ErrInvalidStopLoss
	}
	if sl > MaxStopLoss {
		return fmt.Errorf("stop loss cannot exceed %.0f%%", MaxStopLoss)
	}
	return nil
}

// ValidateLeverage проверяет значение плеча
//
// Leverage должен быть:
//   - Не меньше 1
//   - Не больше MaxLeverage (100)
func ValidateLeverage(leverage int) error {
	if leverage < 1 || leverage > MaxLeverage {
		return ErrInvalidLeverage
	}
	return nil
}

// ValidatePercentage проверяет процентное значение
//
// Процент должен быть:
//   - От 0 до 100 включительно
func ValidatePercentage(pct float64) error {
	if pct < 0 || pct > 100 {
		return ErrInvalidPercentage
	}
	return nil
}

// ============================================================
// Валидация email
// ============================================================

// ValidateEmail проверяет формат email адреса
//
// Параметры:
//   - email: email адрес
//
// Возвращает:
//   - nil: email валиден
//   - error: описание проблемы
//
// Пример:
//
//	if err := ValidateEmail("user@example.com"); err != nil {
//	    return fmt.Errorf("invalid email: %w", err)
//	}
func ValidateEmail(email string) error {
	if email == "" {
		return ErrEmptyEmail
	}

	email = strings.TrimSpace(email)

	if !emailRegex.MatchString(email) {
		return ErrInvalidEmail
	}

	return nil
}

// ============================================================
// Валидация API ключей
// ============================================================

// ValidateAPIKey проверяет формат API ключа
//
// API ключ должен:
//   - Не быть пустым
//   - Содержать только буквы, цифры, дефисы и подчеркивания
//   - Иметь длину от MinAPIKeyLength до MaxAPIKeyLength
//
// Параметры:
//   - apiKey: API ключ биржи
//
// Возвращает:
//   - nil: API ключ валиден
//   - error: описание проблемы
func ValidateAPIKey(apiKey string) error {
	if apiKey == "" {
		return ErrEmptyAPIKey
	}

	apiKey = strings.TrimSpace(apiKey)

	if len(apiKey) < MinAPIKeyLength {
		return fmt.Errorf("API key is too short (minimum %d characters)", MinAPIKeyLength)
	}

	if len(apiKey) > MaxAPIKeyLength {
		return fmt.Errorf("API key is too long (maximum %d characters)", MaxAPIKeyLength)
	}

	if !apiKeyRegex.MatchString(apiKey) {
		return ErrInvalidAPIKey
	}

	return nil
}

// ValidateAPISecret проверяет формат API secret
//
// API secret должен:
//   - Не быть пустым
//   - Содержать только печатаемые ASCII символы
//   - Иметь разумную длину
//
// Параметры:
//   - secret: API secret биржи
//
// Возвращает:
//   - nil: API secret валиден
//   - error: описание проблемы
func ValidateAPISecret(secret string) error {
	if secret == "" {
		return ErrEmptyAPISecret
	}

	if len(secret) < MinAPIKeyLength {
		return fmt.Errorf("API secret is too short (minimum %d characters)", MinAPIKeyLength)
	}

	if len(secret) > MaxAPIKeyLength*2 { // Secret может быть длиннее ключа
		return fmt.Errorf("API secret is too long")
	}

	// Проверяем что все символы печатаемые
	for _, r := range secret {
		if !unicode.IsPrint(r) {
			return ErrInvalidAPISecret
		}
	}

	return nil
}

// ValidateAPIPassphrase проверяет формат API passphrase (для бирж типа OKX)
//
// Passphrase может быть пустым (не все биржи требуют)
// Если не пустой, должен содержать только печатаемые символы
func ValidateAPIPassphrase(passphrase string) error {
	// Passphrase опционален
	if passphrase == "" {
		return nil
	}

	// Проверяем разумную длину
	if len(passphrase) > 64 {
		return fmt.Errorf("API passphrase is too long")
	}

	// Проверяем что все символы печатаемые
	for _, r := range passphrase {
		if !unicode.IsPrint(r) {
			return fmt.Errorf("API passphrase contains invalid characters")
		}
	}

	return nil
}

// ============================================================
// Валидация биржи
// ============================================================

// ValidateExchange проверяет название биржи
//
// Параметры:
//   - exchange: название биржи
//
// Возвращает:
//   - nil: биржа поддерживается
//   - error: биржа не поддерживается
func ValidateExchange(exchange string) error {
	if exchange == "" {
		return ErrInvalidExchange
	}

	exchange = strings.ToLower(strings.TrimSpace(exchange))

	for _, supported := range SupportedExchanges {
		if exchange == supported {
			return nil
		}
	}

	return fmt.Errorf("%w: %s (supported: %s)", ErrInvalidExchange, exchange, strings.Join(SupportedExchanges, ", "))
}

// NormalizeExchange приводит название биржи к стандартному виду (lowercase)
func NormalizeExchange(exchange string) string {
	return strings.ToLower(strings.TrimSpace(exchange))
}

// ============================================================
// Комплексная валидация
// ============================================================

// ValidationError содержит информацию об ошибке валидации поля
type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("%s: %s", e.Field, e.Message)
}

// ValidationErrors - коллекция ошибок валидации
type ValidationErrors []ValidationError

func (ve ValidationErrors) Error() string {
	if len(ve) == 0 {
		return ""
	}

	var messages []string
	for _, e := range ve {
		messages = append(messages, e.Error())
	}
	return strings.Join(messages, "; ")
}

// HasErrors возвращает true если есть ошибки
func (ve ValidationErrors) HasErrors() bool {
	return len(ve) > 0
}

// Add добавляет ошибку валидации
func (ve *ValidationErrors) Add(field, message string) {
	*ve = append(*ve, ValidationError{Field: field, Message: message})
}

// AddError добавляет ошибку если она не nil
func (ve *ValidationErrors) AddError(field string, err error) {
	if err != nil {
		ve.Add(field, err.Error())
	}
}

// PairConfigValidation содержит параметры для валидации конфигурации пары
type PairConfigValidation struct {
	Symbol      string
	EntrySpread float64
	ExitSpread  float64
	Volume      float64
	NOrders     int
	StopLoss    float64
	ExchangeA   string
	ExchangeB   string
}

// ValidatePairConfig выполняет комплексную валидацию конфигурации торговой пары
//
// Возвращает:
//   - nil: все параметры валидны
//   - ValidationErrors: список ошибок валидации
func ValidatePairConfig(cfg PairConfigValidation) error {
	var errs ValidationErrors

	errs.AddError("symbol", ValidateSymbol(cfg.Symbol))
	errs.AddError("entry_spread", ValidateSpread(cfg.EntrySpread))
	errs.AddError("exit_spread", ValidateSpread(cfg.ExitSpread))
	errs.AddError("volume", ValidateVolume(cfg.Volume))
	errs.AddError("n_orders", ValidateNOrders(cfg.NOrders))

	if cfg.StopLoss > 0 {
		errs.AddError("stop_loss", ValidateStopLoss(cfg.StopLoss))
	}

	if cfg.ExchangeA != "" {
		errs.AddError("exchange_a", ValidateExchange(cfg.ExchangeA))
	}

	if cfg.ExchangeB != "" {
		errs.AddError("exchange_b", ValidateExchange(cfg.ExchangeB))
	}

	// Проверяем что биржи разные
	if cfg.ExchangeA != "" && cfg.ExchangeB != "" {
		if NormalizeExchange(cfg.ExchangeA) == NormalizeExchange(cfg.ExchangeB) {
			errs.Add("exchanges", "exchange_a and exchange_b must be different")
		}
	}

	// Проверяем логику спредов: entry_spread должен быть >= exit_spread
	if cfg.EntrySpread > 0 && cfg.ExitSpread > 0 && cfg.EntrySpread < cfg.ExitSpread {
		errs.Add("spreads", "entry_spread must be greater than or equal to exit_spread")
	}

	if errs.HasErrors() {
		return errs
	}
	return nil
}

// ============================================================
// Утилиты для валидации
// ============================================================

// IsValidSymbol возвращает true если символ валиден
func IsValidSymbol(symbol string) bool {
	return ValidateSymbol(symbol) == nil
}

// IsValidEmail возвращает true если email валиден
func IsValidEmail(email string) bool {
	return ValidateEmail(email) == nil
}

// IsValidAPIKey возвращает true если API ключ валиден
func IsValidAPIKey(apiKey string) bool {
	return ValidateAPIKey(apiKey) == nil
}

// IsValidExchange возвращает true если биржа поддерживается
func IsValidExchange(exchange string) bool {
	return ValidateExchange(exchange) == nil
}

// IsSupportedExchange проверяет поддерживается ли биржа
// Алиас для IsValidExchange
func IsSupportedExchange(exchange string) bool {
	return IsValidExchange(exchange)
}

// GetSupportedExchanges возвращает список поддерживаемых бирж
func GetSupportedExchanges() []string {
	result := make([]string, len(SupportedExchanges))
	copy(result, SupportedExchanges)
	return result
}
