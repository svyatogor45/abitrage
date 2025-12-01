package ratelimit

import (
	"context"
	"sync"
	"time"
)

// RateLimiter - Token Bucket rate limiter для контроля частоты запросов к API бирж
//
// Алгоритм Token Bucket:
// - Ведро наполняется токенами с постоянной скоростью (rate токенов/сек)
// - Максимальная ёмкость ведра = burst (позволяет короткие всплески)
// - Каждый запрос потребляет 1 токен
// - Если токенов нет, запрос ждёт или отклоняется
//
// Преимущества:
// - Позволяет burst запросов (важно для параллельных ордеров)
// - Сглаживает нагрузку при постоянном потоке
// - Защищает от превышения лимитов биржи
//
// Использование:
//
//	limiter := NewRateLimiter(10, 20) // 10 req/sec, burst 20
//	err := limiter.Wait(ctx)          // блокирующее ожидание
//	if limiter.Allow() { ... }        // неблокирующая проверка
type RateLimiter struct {
	rate       float64   // токенов в секунду
	burst      float64   // максимальная ёмкость (burst capacity)
	tokens     float64   // текущее количество токенов
	lastRefill time.Time // время последнего пополнения
	mu         sync.Mutex
}

// NewRateLimiter создаёт новый rate limiter
//
// Параметры:
//   - rate: количество запросов в секунду (например, 10 для 10 req/sec)
//   - burst: максимальный burst (обычно 1.5-2x от rate)
//
// Примеры лимитов бирж:
//   - Bybit:  10 req/sec (burst 20)
//   - Bitget: 10 req/sec (burst 20)
//   - OKX:   20 req/sec (burst 40)
//   - Gate:  10 req/sec (burst 20)
//   - HTX:   10 req/sec (burst 20)
//   - BingX: 10 req/sec (burst 20)
func NewRateLimiter(rate, burst float64) *RateLimiter {
	if rate <= 0 {
		rate = 10 // дефолт 10 req/sec
	}
	if burst <= 0 {
		burst = rate * 2 // дефолт burst = 2x rate
	}
	if burst < rate {
		burst = rate
	}

	return &RateLimiter{
		rate:       rate,
		burst:      burst,
		tokens:     burst, // начинаем с полным ведром
		lastRefill: time.Now(),
	}
}

// refill пополняет токены на основе прошедшего времени
// ВАЖНО: вызывается под lock'ом
func (rl *RateLimiter) refill() {
	now := time.Now()
	elapsed := now.Sub(rl.lastRefill).Seconds()

	// Добавляем токены пропорционально прошедшему времени
	rl.tokens += elapsed * rl.rate

	// Не превышаем burst capacity
	if rl.tokens > rl.burst {
		rl.tokens = rl.burst
	}

	rl.lastRefill = now
}

// Wait блокирует до получения токена или отмены контекста
//
// Возвращает:
//   - nil: токен получен, можно выполнять запрос
//   - ctx.Err(): контекст отменён (timeout или cancel)
//
// Пример:
//
//	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
//	defer cancel()
//	if err := limiter.Wait(ctx); err != nil {
//	    return err // timeout
//	}
//	// выполняем запрос к бирже
func (rl *RateLimiter) Wait(ctx context.Context) error {
	for {
		rl.mu.Lock()
		rl.refill()

		if rl.tokens >= 1 {
			rl.tokens--
			rl.mu.Unlock()
			return nil
		}

		// Вычисляем время ожидания до следующего токена
		waitTime := time.Duration((1 - rl.tokens) / rl.rate * float64(time.Second))
		rl.mu.Unlock()

		// Ждём с возможностью отмены
		select {
		case <-time.After(waitTime):
			// Повторяем попытку получить токен
			continue
		case <-ctx.Done():
			return ctx.Err()
		}
	}
}

// WaitN блокирует до получения n токенов или отмены контекста
// Полезно для batch операций
func (rl *RateLimiter) WaitN(ctx context.Context, n int) error {
	if n <= 0 {
		return nil
	}

	for i := 0; i < n; i++ {
		if err := rl.Wait(ctx); err != nil {
			return err
		}
	}
	return nil
}

// Allow проверяет доступность токена без блокировки
//
// Возвращает:
//   - true: токен получен, можно выполнять запрос
//   - false: нет токенов, запрос нужно отложить
//
// Пример:
//
//	if limiter.Allow() {
//	    // выполняем запрос
//	} else {
//	    // отклоняем или откладываем
//	}
func (rl *RateLimiter) Allow() bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= 1 {
		rl.tokens--
		return true
	}

	return false
}

// AllowN проверяет доступность n токенов без блокировки
func (rl *RateLimiter) AllowN(n int) bool {
	if n <= 0 {
		return true
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	if rl.tokens >= float64(n) {
		rl.tokens -= float64(n)
		return true
	}

	return false
}

// Reserve резервирует токен и возвращает время ожидания
//
// Возвращает:
//   - Reservation с информацией о времени ожидания
//   - Вызывающий код должен сам реализовать ожидание
//
// Полезно когда нужно знать время ожидания заранее
func (rl *RateLimiter) Reserve() *Reservation {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill()

	res := &Reservation{
		limiter: rl,
		tokens:  1,
	}

	if rl.tokens >= 1 {
		rl.tokens--
		res.ok = true
		res.delay = 0
	} else {
		// Резервируем будущий токен
		res.ok = true
		res.delay = time.Duration((1 - rl.tokens) / rl.rate * float64(time.Second))
		rl.tokens-- // уходим в минус, refill восполнит
	}

	return res
}

// Tokens возвращает текущее количество доступных токенов
// Полезно для мониторинга и отладки
func (rl *RateLimiter) Tokens() float64 {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	rl.refill()
	return rl.tokens
}

// Rate возвращает скорость пополнения токенов (токенов/сек)
func (rl *RateLimiter) Rate() float64 {
	return rl.rate
}

// Burst возвращает максимальную ёмкость (burst capacity)
func (rl *RateLimiter) Burst() float64 {
	return rl.burst
}

// SetRate изменяет скорость пополнения токенов
// Потокобезопасно
func (rl *RateLimiter) SetRate(rate float64) {
	if rate <= 0 {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.refill() // фиксируем текущие токены перед изменением rate
	rl.rate = rate
}

// SetBurst изменяет максимальную ёмкость
// Потокобезопасно
func (rl *RateLimiter) SetBurst(burst float64) {
	if burst <= 0 {
		return
	}

	rl.mu.Lock()
	defer rl.mu.Unlock()

	rl.burst = burst
	if rl.tokens > rl.burst {
		rl.tokens = rl.burst
	}
}

// Reservation представляет зарезервированный токен
type Reservation struct {
	limiter *RateLimiter
	tokens  float64
	ok      bool
	delay   time.Duration
}

// OK возвращает true если резервация успешна
func (r *Reservation) OK() bool {
	return r.ok
}

// Delay возвращает время ожидания до использования токена
func (r *Reservation) Delay() time.Duration {
	return r.delay
}

// Cancel отменяет резервацию и возвращает токен
func (r *Reservation) Cancel() {
	if !r.ok || r.limiter == nil {
		return
	}

	r.limiter.mu.Lock()
	defer r.limiter.mu.Unlock()

	r.limiter.tokens += r.tokens
	if r.limiter.tokens > r.limiter.burst {
		r.limiter.tokens = r.limiter.burst
	}

	r.ok = false
}

// ============================================================
// MultiLimiter - комбинированный rate limiter для нескольких эндпоинтов
// ============================================================

// MultiLimiter управляет несколькими rate limiters
// Полезно когда у биржи разные лимиты для разных типов запросов
//
// Пример:
//   - order endpoints: 5 req/sec
//   - market data: 50 req/sec
//   - account info: 10 req/sec
type MultiLimiter struct {
	limiters map[string]*RateLimiter
	mu       sync.RWMutex
}

// NewMultiLimiter создаёт новый MultiLimiter
func NewMultiLimiter() *MultiLimiter {
	return &MultiLimiter{
		limiters: make(map[string]*RateLimiter),
	}
}

// Add добавляет rate limiter для категории запросов
func (ml *MultiLimiter) Add(category string, rate, burst float64) {
	ml.mu.Lock()
	defer ml.mu.Unlock()
	ml.limiters[category] = NewRateLimiter(rate, burst)
}

// Wait ожидает токен для указанной категории
func (ml *MultiLimiter) Wait(ctx context.Context, category string) error {
	ml.mu.RLock()
	limiter, ok := ml.limiters[category]
	ml.mu.RUnlock()

	if !ok {
		return nil // нет лимита для этой категории
	}

	return limiter.Wait(ctx)
}

// Allow проверяет доступность токена для категории
func (ml *MultiLimiter) Allow(category string) bool {
	ml.mu.RLock()
	limiter, ok := ml.limiters[category]
	ml.mu.RUnlock()

	if !ok {
		return true // нет лимита для этой категории
	}

	return limiter.Allow()
}

// Get возвращает limiter для категории
func (ml *MultiLimiter) Get(category string) *RateLimiter {
	ml.mu.RLock()
	defer ml.mu.RUnlock()
	return ml.limiters[category]
}
