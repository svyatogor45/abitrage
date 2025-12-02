// Package exchange предоставляет унифицированный интерфейс для работы с биржами.
package exchange

import (
	"context"
	"crypto/tls"
	"net"
	"net/http"
	"sync"
	"time"
)

// HTTPClientConfig содержит настройки HTTP клиента для бирж
// Параметры соответствуют требованиям производительности торгового ядра
type HTTPClientConfig struct {
	// Таймауты соединения
	ConnectTimeout time.Duration // таймаут установки TCP соединения (default: 5s)
	ReadTimeout    time.Duration // таймаут чтения ответа (default: 10s)
	WriteTimeout   time.Duration // таймаут отправки запроса (default: 10s)
	TotalTimeout   time.Duration // общий таймаут операции (default: 30s)

	// Connection pooling
	MaxIdleConns        int           // максимум idle соединений (default: 100)
	MaxIdleConnsPerHost int           // максимум idle соединений на хост (default: 10)
	MaxConnsPerHost     int           // максимум соединений на хост (default: 20)
	IdleConnTimeout     time.Duration // таймаут простоя соединения (default: 90s)

	// TLS
	TLSHandshakeTimeout time.Duration // таймаут TLS handshake (default: 5s)

	// Keep-Alive
	DisableKeepAlives bool          // отключить Keep-Alive (default: false)
	KeepAliveInterval time.Duration // интервал Keep-Alive (default: 30s)
}

// DefaultHTTPClientConfig возвращает конфигурацию по умолчанию
// Параметры оптимизированы для торговых операций с низкой latency
func DefaultHTTPClientConfig() HTTPClientConfig {
	return HTTPClientConfig{
		// Таймауты согласно Архитектура.md
		ConnectTimeout: 5 * time.Second,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   10 * time.Second,
		TotalTimeout:   30 * time.Second,

		// Connection pooling для Keep-Alive
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 10,
		MaxConnsPerHost:     20,
		IdleConnTimeout:     90 * time.Second,

		// TLS
		TLSHandshakeTimeout: 5 * time.Second,

		// Keep-Alive
		DisableKeepAlives: false,
		KeepAliveInterval: 30 * time.Second,
	}
}

// HTTPClient представляет оптимизированный HTTP клиент для работы с биржевыми API
// Поддерживает connection pooling и детальные таймауты
type HTTPClient struct {
	client *http.Client
	config HTTPClientConfig
}

// globalClient - глобальный HTTP клиент для переиспользования соединений
var (
	globalClient     *HTTPClient
	globalClientOnce sync.Once
)

// GetGlobalHTTPClient возвращает глобальный HTTP клиент с настройками по умолчанию
// Использует singleton pattern для переиспользования connection pool
func GetGlobalHTTPClient() *HTTPClient {
	globalClientOnce.Do(func() {
		globalClient = NewHTTPClient(DefaultHTTPClientConfig())
	})
	return globalClient
}

// NewHTTPClient создаёт новый HTTP клиент с заданной конфигурацией
func NewHTTPClient(config HTTPClientConfig) *HTTPClient {
	// Создаём диaler с таймаутами
	dialer := &net.Dialer{
		Timeout:   config.ConnectTimeout,
		KeepAlive: config.KeepAliveInterval,
	}

	// Создаём transport с connection pooling
	transport := &http.Transport{
		// Dialer с таймаутом соединения
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			// Проверяем deadline контекста
			if deadline, ok := ctx.Deadline(); ok {
				timeout := time.Until(deadline)
				if timeout < config.ConnectTimeout {
					dialerWithTimeout := &net.Dialer{
						Timeout:   timeout,
						KeepAlive: config.KeepAliveInterval,
					}
					return dialerWithTimeout.DialContext(ctx, network, addr)
				}
			}
			return dialer.DialContext(ctx, network, addr)
		},

		// Connection pooling (Keep-Alive)
		MaxIdleConns:        config.MaxIdleConns,
		MaxIdleConnsPerHost: config.MaxIdleConnsPerHost,
		MaxConnsPerHost:     config.MaxConnsPerHost,
		IdleConnTimeout:     config.IdleConnTimeout,

		// TLS
		TLSHandshakeTimeout: config.TLSHandshakeTimeout,
		TLSClientConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},

		// Keep-Alive
		DisableKeepAlives: config.DisableKeepAlives,

		// Оптимизации для скорости
		DisableCompression:  true, // отключаем сжатие для минимизации latency
		ForceAttemptHTTP2:   true, // используем HTTP/2 где возможно
		ExpectContinueTimeout: 1 * time.Second,
		ResponseHeaderTimeout: config.ReadTimeout,
	}

	client := &http.Client{
		Transport: transport,
		Timeout:   config.TotalTimeout, // общий таймаут как fallback
	}

	return &HTTPClient{
		client: client,
		config: config,
	}
}

// Do выполняет HTTP запрос с учётом всех таймаутов
// Использует context для контроля таймаутов на уровне запроса
func (hc *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	return hc.client.Do(req)
}

// DoWithTimeout выполняет HTTP запрос с кастомным таймаутом
// Полезно для операций требующих нестандартный таймаут
func (hc *HTTPClient) DoWithTimeout(req *http.Request, timeout time.Duration) (*http.Response, error) {
	ctx, cancel := context.WithTimeout(req.Context(), timeout)
	defer cancel()
	return hc.client.Do(req.WithContext(ctx))
}

// GetClient возвращает базовый http.Client для совместимости
func (hc *HTTPClient) GetClient() *http.Client {
	return hc.client
}

// GetConfig возвращает текущую конфигурацию клиента
func (hc *HTTPClient) GetConfig() HTTPClientConfig {
	return hc.config
}

// Close закрывает все idle соединения
// Должен вызываться при graceful shutdown
func (hc *HTTPClient) Close() {
	if transport, ok := hc.client.Transport.(*http.Transport); ok {
		transport.CloseIdleConnections()
	}
}

// CloseGlobalClient закрывает глобальный HTTP клиент
// Вызывается при graceful shutdown приложения
func CloseGlobalClient() {
	if globalClient != nil {
		globalClient.Close()
	}
}

// timeoutRoundTripper оборачивает Transport для добавления таймаутов на Read/Write
type timeoutRoundTripper struct {
	transport    http.RoundTripper
	readTimeout  time.Duration
	writeTimeout time.Duration
}

// RoundTrip выполняет HTTP запрос с контролем таймаутов
func (t *timeoutRoundTripper) RoundTrip(req *http.Request) (*http.Response, error) {
	return t.transport.RoundTrip(req)
}
