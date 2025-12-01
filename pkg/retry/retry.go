package retry

import (
	"context"
	"errors"
	"math"
	"math/rand"
	"time"
)

// Config конфигурация для retry логики
//
// Экспоненциальный backoff с jitter:
// delay = min(InitialDelay * Multiplier^attempt + jitter, MaxDelay)
//
// Jitter добавляет случайность чтобы избежать "thundering herd"
// когда много клиентов retry'ят одновременно
type Config struct {
	// MaxRetries - максимальное количество попыток (включая первую)
	// 0 или отрицательное = бесконечные retry (не рекомендуется)
	MaxRetries int

	// InitialDelay - начальная задержка между попытками
	// По умолчанию: 100ms
	InitialDelay time.Duration

	// MaxDelay - максимальная задержка между попытками
	// По умолчанию: 30s
	MaxDelay time.Duration

	// Multiplier - множитель для экспоненциального роста
	// По умолчанию: 2.0 (удвоение после каждой попытки)
	Multiplier float64

	// JitterFactor - фактор случайности (0.0 - 1.0)
	// 0.0 = нет jitter, 1.0 = до 100% вариации
	// По умолчанию: 0.1 (10% вариации)
	JitterFactor float64

	// RetryIf - функция для определения нужно ли retry'ить ошибку
	// По умолчанию: retry все ошибки
	RetryIf func(error) bool

	// OnRetry - callback вызываемый перед каждым retry
	// Полезно для логирования
	OnRetry func(attempt int, err error, delay time.Duration)
}

// DefaultConfig возвращает конфигурацию по умолчанию
//
// Подходит для большинства API запросов:
// - 4 попытки
// - Задержки: 100ms, 200ms, 400ms, 800ms (+ jitter)
// - Максимум 30 секунд ожидания
func DefaultConfig() Config {
	return Config{
		MaxRetries:   4,
		InitialDelay: 100 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.1,
	}
}

// AggressiveConfig для критичных операций (например, закрытие позиций)
//
// Больше попыток, быстрее retry:
// - 6 попыток
// - Задержки: 50ms, 100ms, 200ms, 400ms, 800ms, 1600ms
func AggressiveConfig() Config {
	return Config{
		MaxRetries:   6,
		InitialDelay: 50 * time.Millisecond,
		MaxDelay:     10 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.1,
	}
}

// ConservativeConfig для некритичных операций (например, получение баланса)
//
// Меньше попыток, медленнее retry:
// - 3 попытки
// - Задержки: 500ms, 1s, 2s
func ConservativeConfig() Config {
	return Config{
		MaxRetries:   3,
		InitialDelay: 500 * time.Millisecond,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.2,
	}
}

// NetworkConfig для сетевых ошибок с более длинными задержками
//
// - 4 попытки
// - Задержки: 1s, 2s, 4s, 8s
func NetworkConfig() Config {
	return Config{
		MaxRetries:   4,
		InitialDelay: 1 * time.Second,
		MaxDelay:     30 * time.Second,
		Multiplier:   2.0,
		JitterFactor: 0.2,
	}
}

// validate проверяет и устанавливает значения по умолчанию
func (c *Config) validate() {
	if c.InitialDelay <= 0 {
		c.InitialDelay = 100 * time.Millisecond
	}
	if c.MaxDelay <= 0 {
		c.MaxDelay = 30 * time.Second
	}
	if c.Multiplier <= 0 {
		c.Multiplier = 2.0
	}
	if c.JitterFactor < 0 {
		c.JitterFactor = 0
	}
	if c.JitterFactor > 1 {
		c.JitterFactor = 1
	}
}

// calculateDelay вычисляет задержку для указанной попытки
func (c *Config) calculateDelay(attempt int) time.Duration {
	// Экспоненциальный рост: InitialDelay * Multiplier^attempt
	delay := float64(c.InitialDelay) * math.Pow(c.Multiplier, float64(attempt))

	// Ограничиваем максимальной задержкой
	if delay > float64(c.MaxDelay) {
		delay = float64(c.MaxDelay)
	}

	// Добавляем jitter
	if c.JitterFactor > 0 {
		jitter := delay * c.JitterFactor * (rand.Float64()*2 - 1) // -JitterFactor до +JitterFactor
		delay += jitter
	}

	// Не допускаем отрицательную задержку
	if delay < 0 {
		delay = 0
	}

	return time.Duration(delay)
}

// Do выполняет операцию с повторными попытками
//
// Параметры:
//   - ctx: контекст для отмены (timeout, cancel)
//   - operation: функция для выполнения
//   - cfg: конфигурация retry
//
// Возвращает:
//   - nil: операция успешна
//   - error: все попытки неудачны, возвращает последнюю ошибку
//
// Пример:
//
//	err := retry.Do(ctx, func() error {
//	    return exchange.PlaceOrder(...)
//	}, retry.DefaultConfig())
func Do(ctx context.Context, operation func() error, cfg Config) error {
	cfg.validate()

	var lastErr error

	for attempt := 0; cfg.MaxRetries <= 0 || attempt < cfg.MaxRetries; attempt++ {
		// Проверяем контекст перед каждой попыткой
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return lastErr
			}
			return ctx.Err()
		default:
		}

		// Выполняем операцию
		err := operation()
		if err == nil {
			return nil // Успех!
		}

		lastErr = err

		// Проверяем нужно ли retry'ить эту ошибку
		if cfg.RetryIf != nil && !cfg.RetryIf(err) {
			return err // Не retry'им эту ошибку
		}

		// Последняя попытка - не ждём
		if cfg.MaxRetries > 0 && attempt >= cfg.MaxRetries-1 {
			break
		}

		// Вычисляем задержку
		delay := cfg.calculateDelay(attempt)

		// Callback перед retry
		if cfg.OnRetry != nil {
			cfg.OnRetry(attempt+1, err, delay)
		}

		// Ждём с возможностью отмены
		select {
		case <-time.After(delay):
			// Продолжаем к следующей попытке
		case <-ctx.Done():
			return lastErr
		}
	}

	return lastErr
}

// DoWithResult выполняет операцию с результатом и retry
//
// Полезно когда операция возвращает значение:
//
//	result, err := retry.DoWithResult(ctx, func() (*Order, error) {
//	    return exchange.PlaceOrder(...)
//	}, retry.DefaultConfig())
func DoWithResult[T any](ctx context.Context, operation func() (T, error), cfg Config) (T, error) {
	cfg.validate()

	var lastErr error
	var zero T

	for attempt := 0; cfg.MaxRetries <= 0 || attempt < cfg.MaxRetries; attempt++ {
		// Проверяем контекст перед каждой попыткой
		select {
		case <-ctx.Done():
			if lastErr != nil {
				return zero, lastErr
			}
			return zero, ctx.Err()
		default:
		}

		// Выполняем операцию
		result, err := operation()
		if err == nil {
			return result, nil // Успех!
		}

		lastErr = err

		// Проверяем нужно ли retry'ить эту ошибку
		if cfg.RetryIf != nil && !cfg.RetryIf(err) {
			return zero, err
		}

		// Последняя попытка - не ждём
		if cfg.MaxRetries > 0 && attempt >= cfg.MaxRetries-1 {
			break
		}

		// Вычисляем задержку
		delay := cfg.calculateDelay(attempt)

		// Callback перед retry
		if cfg.OnRetry != nil {
			cfg.OnRetry(attempt+1, err, delay)
		}

		// Ждём с возможностью отмены
		select {
		case <-time.After(delay):
			// Продолжаем к следующей попытке
		case <-ctx.Done():
			return zero, lastErr
		}
	}

	return zero, lastErr
}

// ============================================================
// Predefined RetryIf functions
// ============================================================

// RetryableError интерфейс для ошибок которые можно retry'ить
type RetryableError interface {
	error
	Retryable() bool
}

// IsRetryable проверяет можно ли retry'ить ошибку
//
// Возвращает true если:
// - Ошибка реализует RetryableError и Retryable() == true
// - Ошибка временная (Temporary() == true)
// - Ошибка содержит wrapped RetryableError
func IsRetryable(err error) bool {
	if err == nil {
		return false
	}

	// Проверяем RetryableError
	var retryable RetryableError
	if errors.As(err, &retryable) {
		return retryable.Retryable()
	}

	// Проверяем временные ошибки
	type temporary interface {
		Temporary() bool
	}
	var temp temporary
	if errors.As(err, &temp) {
		return temp.Temporary()
	}

	// По умолчанию - retry'им
	return true
}

// RetryIfTemporary retry'ит только временные ошибки
func RetryIfTemporary(err error) bool {
	type temporary interface {
		Temporary() bool
	}
	var temp temporary
	if errors.As(err, &temp) {
		return temp.Temporary()
	}
	return false
}

// RetryIfNotContext не retry'ит ошибки контекста (cancel, timeout)
func RetryIfNotContext(err error) bool {
	return !errors.Is(err, context.Canceled) && !errors.Is(err, context.DeadlineExceeded)
}

// ============================================================
// Wrapper errors
// ============================================================

// PermanentError оборачивает ошибку которую не нужно retry'ить
type PermanentError struct {
	Err error
}

func (e *PermanentError) Error() string {
	return e.Err.Error()
}

func (e *PermanentError) Unwrap() error {
	return e.Err
}

func (e *PermanentError) Retryable() bool {
	return false
}

// Permanent оборачивает ошибку в PermanentError
//
// Пример:
//
//	if validationError {
//	    return retry.Permanent(errors.New("invalid input"))
//	}
func Permanent(err error) error {
	if err == nil {
		return nil
	}
	return &PermanentError{Err: err}
}

// TemporaryError оборачивает ошибку которую нужно retry'ить
type TemporaryError struct {
	Err error
}

func (e *TemporaryError) Error() string {
	return e.Err.Error()
}

func (e *TemporaryError) Unwrap() error {
	return e.Err
}

func (e *TemporaryError) Retryable() bool {
	return true
}

func (e *TemporaryError) Temporary() bool {
	return true
}

// Temporary оборачивает ошибку в TemporaryError
//
// Пример:
//
//	if networkError {
//	    return retry.Temporary(err)
//	}
func Temporary(err error) error {
	if err == nil {
		return nil
	}
	return &TemporaryError{Err: err}
}

// ============================================================
// Retryer - объект для многократного использования
// ============================================================

// Retryer предоставляет методы для retry с сохранённой конфигурацией
//
// Полезно когда нужно использовать одну конфигурацию много раз:
//
//	r := retry.NewRetryer(retry.DefaultConfig())
//	err := r.Do(ctx, operation1)
//	err = r.Do(ctx, operation2)
type Retryer struct {
	cfg Config
}

// NewRetryer создаёт новый Retryer с указанной конфигурацией
func NewRetryer(cfg Config) *Retryer {
	cfg.validate()
	return &Retryer{cfg: cfg}
}

// Do выполняет операцию с retry
func (r *Retryer) Do(ctx context.Context, operation func() error) error {
	return Do(ctx, operation, r.cfg)
}

// DoWithResult выполняет операцию с результатом и retry
func (r *Retryer) DoWithResult(ctx context.Context, operation func() (interface{}, error)) (interface{}, error) {
	return DoWithResult(ctx, operation, r.cfg)
}

// WithOnRetry возвращает копию Retryer с callback'ом
func (r *Retryer) WithOnRetry(onRetry func(attempt int, err error, delay time.Duration)) *Retryer {
	newCfg := r.cfg
	newCfg.OnRetry = onRetry
	return &Retryer{cfg: newCfg}
}

// WithRetryIf возвращает копию Retryer с функцией фильтрации ошибок
func (r *Retryer) WithRetryIf(retryIf func(error) bool) *Retryer {
	newCfg := r.cfg
	newCfg.RetryIf = retryIf
	return &Retryer{cfg: newCfg}
}

// ============================================================
// Простые функции-хелперы
// ============================================================

// Once выполняет операцию один раз (без retry)
// Полезно для унификации API
func Once(ctx context.Context, operation func() error) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
	}
	return operation()
}

// Retry выполняет операцию с дефолтной конфигурацией
//
// Сокращённая форма:
//
//	retry.Retry(ctx, operation) == retry.Do(ctx, operation, retry.DefaultConfig())
func Retry(ctx context.Context, operation func() error) error {
	return Do(ctx, operation, DefaultConfig())
}

// RetryN выполняет операцию с указанным количеством попыток
//
// Сокращённая форма для простых случаев:
//
//	retry.RetryN(ctx, operation, 3) // 3 попытки с дефолтными задержками
func RetryN(ctx context.Context, operation func() error, maxRetries int) error {
	cfg := DefaultConfig()
	cfg.MaxRetries = maxRetries
	return Do(ctx, operation, cfg)
}
