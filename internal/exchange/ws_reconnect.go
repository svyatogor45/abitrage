package exchange

import (
	"context"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
)

// WSReconnectConfig конфигурация переподключения WebSocket
type WSReconnectConfig struct {
	// Начальная задержка перед переподключением
	InitialDelay time.Duration
	// Максимальная задержка (после exponential backoff)
	MaxDelay time.Duration
	// Максимальное количество попыток (0 = бесконечно)
	MaxRetries int
	// Таймаут подключения
	ConnectTimeout time.Duration
	// Интервал ping для проверки соединения
	PingInterval time.Duration
	// Таймаут ожидания pong
	PongTimeout time.Duration
}

// DefaultWSReconnectConfig возвращает конфигурацию по умолчанию
// Соответствует требованиям из Разработка.md: 2s, 4s, 8s, 16s
func DefaultWSReconnectConfig() WSReconnectConfig {
	return WSReconnectConfig{
		InitialDelay:   2 * time.Second,
		MaxDelay:       16 * time.Second,
		MaxRetries:     10,
		ConnectTimeout: 10 * time.Second,
		PingInterval:   30 * time.Second,
		PongTimeout:    10 * time.Second,
	}
}

// WSConnectionState состояние WebSocket соединения
type WSConnectionState int32

const (
	WSStateDisconnected WSConnectionState = iota
	WSStateConnecting
	WSStateConnected
	WSStateReconnecting
	WSStateClosed
)

func (s WSConnectionState) String() string {
	switch s {
	case WSStateDisconnected:
		return "disconnected"
	case WSStateConnecting:
		return "connecting"
	case WSStateConnected:
		return "connected"
	case WSStateReconnecting:
		return "reconnecting"
	case WSStateClosed:
		return "closed"
	default:
		return "unknown"
	}
}

// WSReconnectManager управляет WebSocket соединением с автоматическим переподключением
//
// Назначение:
// Обеспечивает надёжное WebSocket соединение с биржей, автоматически
// переподключаясь при разрывах с exponential backoff.
//
// Функции:
// - Автоматическое переподключение с exponential backoff
// - Повторная подписка на каналы после переподключения
// - Ping/Pong для проверки живости соединения
// - Thread-safe операции
// - Callbacks для уведомления о событиях (connect, disconnect, message)
//
// Использование:
// 1. Создать manager: NewWSReconnectManager(...)
// 2. Установить handlers: SetOnMessage, SetOnConnect, SetOnDisconnect
// 3. Подключиться: Connect()
// 4. Отправлять сообщения: Send(msg)
// 5. Закрыть: Close()
type WSReconnectManager struct {
	// Имя биржи (для логирования)
	exchangeName string

	// URL для подключения
	wsURL string

	// Конфигурация
	config WSReconnectConfig

	// WebSocket соединение
	conn   *websocket.Conn
	connMu sync.RWMutex

	// Состояние
	state int32 // atomic WSConnectionState

	// Счётчик попыток переподключения
	retryCount int32 // atomic

	// Каналы управления
	closeChan   chan struct{}
	messageChan chan []byte

	// Callbacks
	onMessage    func([]byte)
	onConnect    func()
	onDisconnect func(error)
	callbackMu   sync.RWMutex

	// Подписки для восстановления после переподключения
	subscriptions   []interface{}
	subscriptionsMu sync.RWMutex

	// Аутентификация (для приватных каналов)
	authFunc func(*websocket.Conn) error
}

// NewWSReconnectManager создаёт новый менеджер переподключений
func NewWSReconnectManager(exchangeName, wsURL string, config WSReconnectConfig) *WSReconnectManager {
	return &WSReconnectManager{
		exchangeName: exchangeName,
		wsURL:        wsURL,
		config:       config,
		closeChan:    make(chan struct{}),
		messageChan:  make(chan []byte, 1000),
		subscriptions: make([]interface{}, 0),
	}
}

// SetOnMessage устанавливает callback для входящих сообщений
func (m *WSReconnectManager) SetOnMessage(handler func([]byte)) {
	m.callbackMu.Lock()
	m.onMessage = handler
	m.callbackMu.Unlock()
}

// SetOnConnect устанавливает callback для события подключения
func (m *WSReconnectManager) SetOnConnect(handler func()) {
	m.callbackMu.Lock()
	m.onConnect = handler
	m.callbackMu.Unlock()
}

// SetOnDisconnect устанавливает callback для события отключения
func (m *WSReconnectManager) SetOnDisconnect(handler func(error)) {
	m.callbackMu.Lock()
	m.onDisconnect = handler
	m.callbackMu.Unlock()
}

// SetAuthFunc устанавливает функцию аутентификации для приватных каналов
func (m *WSReconnectManager) SetAuthFunc(authFunc func(*websocket.Conn) error) {
	m.authFunc = authFunc
}

// AddSubscription добавляет подписку для восстановления после переподключения
func (m *WSReconnectManager) AddSubscription(sub interface{}) {
	m.subscriptionsMu.Lock()
	m.subscriptions = append(m.subscriptions, sub)
	m.subscriptionsMu.Unlock()
}

// ClearSubscriptions очищает список подписок
func (m *WSReconnectManager) ClearSubscriptions() {
	m.subscriptionsMu.Lock()
	m.subscriptions = make([]interface{}, 0)
	m.subscriptionsMu.Unlock()
}

// GetState возвращает текущее состояние соединения
func (m *WSReconnectManager) GetState() WSConnectionState {
	return WSConnectionState(atomic.LoadInt32(&m.state))
}

// IsConnected проверяет, установлено ли соединение
func (m *WSReconnectManager) IsConnected() bool {
	return m.GetState() == WSStateConnected
}

// Connect устанавливает WebSocket соединение
func (m *WSReconnectManager) Connect() error {
	// Проверяем, не закрыт ли менеджер
	select {
	case <-m.closeChan:
		return fmt.Errorf("manager is closed")
	default:
	}

	atomic.StoreInt32(&m.state, int32(WSStateConnecting))

	if err := m.dial(); err != nil {
		atomic.StoreInt32(&m.state, int32(WSStateDisconnected))
		return err
	}

	atomic.StoreInt32(&m.state, int32(WSStateConnected))
	atomic.StoreInt32(&m.retryCount, 0)

	// Вызываем callback подключения
	m.callbackMu.RLock()
	onConnect := m.onConnect
	m.callbackMu.RUnlock()

	if onConnect != nil {
		onConnect()
	}

	// Запускаем горутины чтения и ping
	go m.readPump()
	go m.pingPump()

	log.Printf("[%s] WebSocket connected to %s", m.exchangeName, m.wsURL)

	return nil
}

// dial выполняет подключение к WebSocket
func (m *WSReconnectManager) dial() error {
	ctx, cancel := context.WithTimeout(context.Background(), m.config.ConnectTimeout)
	defer cancel()

	dialer := websocket.Dialer{
		HandshakeTimeout: m.config.ConnectTimeout,
	}

	conn, _, err := dialer.DialContext(ctx, m.wsURL, nil)
	if err != nil {
		return fmt.Errorf("dial error: %w", err)
	}

	m.connMu.Lock()
	m.conn = conn
	m.connMu.Unlock()

	// Аутентификация если требуется
	if m.authFunc != nil {
		if err := m.authFunc(conn); err != nil {
			conn.Close()
			m.connMu.Lock()
			m.conn = nil
			m.connMu.Unlock()
			return fmt.Errorf("auth error: %w", err)
		}
	}

	// Восстанавливаем подписки
	if err := m.resubscribe(); err != nil {
		log.Printf("[%s] Warning: resubscribe error: %v", m.exchangeName, err)
		// Не возвращаем ошибку, подписки могут быть восстановлены позже
	}

	return nil
}

// resubscribe восстанавливает подписки после переподключения
func (m *WSReconnectManager) resubscribe() error {
	m.subscriptionsMu.RLock()
	subs := make([]interface{}, len(m.subscriptions))
	copy(subs, m.subscriptions)
	m.subscriptionsMu.RUnlock()

	m.connMu.RLock()
	conn := m.conn
	m.connMu.RUnlock()

	if conn == nil {
		return fmt.Errorf("no connection")
	}

	for _, sub := range subs {
		if err := conn.WriteJSON(sub); err != nil {
			return fmt.Errorf("resubscribe error: %w", err)
		}
	}

	if len(subs) > 0 {
		log.Printf("[%s] Resubscribed to %d channels", m.exchangeName, len(subs))
	}

	return nil
}

// readPump читает сообщения из WebSocket
func (m *WSReconnectManager) readPump() {
	defer m.handleDisconnect(nil)

	for {
		select {
		case <-m.closeChan:
			return
		default:
		}

		m.connMu.RLock()
		conn := m.conn
		m.connMu.RUnlock()

		if conn == nil {
			return
		}

		_, message, err := conn.ReadMessage()
		if err != nil {
			m.handleDisconnect(err)
			return
		}

		// Отправляем сообщение в callback
		m.callbackMu.RLock()
		onMessage := m.onMessage
		m.callbackMu.RUnlock()

		if onMessage != nil {
			onMessage(message)
		}
	}
}

// pingPump отправляет ping для проверки соединения
func (m *WSReconnectManager) pingPump() {
	ticker := time.NewTicker(m.config.PingInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.closeChan:
			return
		case <-ticker.C:
			m.connMu.RLock()
			conn := m.conn
			m.connMu.RUnlock()

			if conn == nil {
				return
			}

			if m.GetState() != WSStateConnected {
				return
			}

			conn.SetWriteDeadline(time.Now().Add(m.config.PongTimeout))
			if err := conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				log.Printf("[%s] Ping error: %v", m.exchangeName, err)
				m.handleDisconnect(err)
				return
			}
		}
	}
}

// handleDisconnect обрабатывает разрыв соединения
func (m *WSReconnectManager) handleDisconnect(err error) {
	// Проверяем, не закрыт ли менеджер
	select {
	case <-m.closeChan:
		return
	default:
	}

	// Избегаем повторной обработки
	state := m.GetState()
	if state == WSStateReconnecting || state == WSStateClosed {
		return
	}

	atomic.StoreInt32(&m.state, int32(WSStateReconnecting))

	// Закрываем текущее соединение
	m.connMu.Lock()
	if m.conn != nil {
		m.conn.Close()
		m.conn = nil
	}
	m.connMu.Unlock()

	// Вызываем callback отключения
	m.callbackMu.RLock()
	onDisconnect := m.onDisconnect
	m.callbackMu.RUnlock()

	if onDisconnect != nil {
		onDisconnect(err)
	}

	if err != nil {
		log.Printf("[%s] WebSocket disconnected: %v", m.exchangeName, err)
	}

	// Запускаем переподключение
	go m.reconnectLoop()
}

// reconnectLoop выполняет переподключение с exponential backoff
func (m *WSReconnectManager) reconnectLoop() {
	delay := m.config.InitialDelay

	for {
		select {
		case <-m.closeChan:
			return
		default:
		}

		retryCount := atomic.AddInt32(&m.retryCount, 1)

		// Проверяем лимит попыток
		if m.config.MaxRetries > 0 && int(retryCount) > m.config.MaxRetries {
			log.Printf("[%s] Max reconnect attempts (%d) reached", m.exchangeName, m.config.MaxRetries)
			atomic.StoreInt32(&m.state, int32(WSStateDisconnected))
			return
		}

		log.Printf("[%s] Reconnecting in %v (attempt %d/%d)...",
			m.exchangeName, delay, retryCount, m.config.MaxRetries)

		// Ждём перед попыткой
		select {
		case <-m.closeChan:
			return
		case <-time.After(delay):
		}

		// Пытаемся подключиться
		if err := m.dial(); err != nil {
			log.Printf("[%s] Reconnect failed: %v", m.exchangeName, err)

			// Exponential backoff
			delay = delay * 2
			if delay > m.config.MaxDelay {
				delay = m.config.MaxDelay
			}
			continue
		}

		// Успешное подключение
		atomic.StoreInt32(&m.state, int32(WSStateConnected))
		atomic.StoreInt32(&m.retryCount, 0)

		// Вызываем callback подключения
		m.callbackMu.RLock()
		onConnect := m.onConnect
		m.callbackMu.RUnlock()

		if onConnect != nil {
			onConnect()
		}

		log.Printf("[%s] WebSocket reconnected successfully", m.exchangeName)

		// Запускаем горутины чтения и ping
		go m.readPump()
		go m.pingPump()

		return
	}
}

// Send отправляет сообщение через WebSocket
func (m *WSReconnectManager) Send(msg interface{}) error {
	if m.GetState() != WSStateConnected {
		return fmt.Errorf("not connected (state: %s)", m.GetState())
	}

	m.connMu.RLock()
	conn := m.conn
	m.connMu.RUnlock()

	if conn == nil {
		return fmt.Errorf("no connection")
	}

	return conn.WriteJSON(msg)
}

// Close закрывает WebSocket соединение и останавливает переподключение
func (m *WSReconnectManager) Close() error {
	// Проверяем, не закрыт ли уже
	select {
	case <-m.closeChan:
		return nil
	default:
		close(m.closeChan)
	}

	atomic.StoreInt32(&m.state, int32(WSStateClosed))

	m.connMu.Lock()
	defer m.connMu.Unlock()

	if m.conn != nil {
		err := m.conn.Close()
		m.conn = nil
		return err
	}

	return nil
}

// GetRetryCount возвращает текущее количество попыток переподключения
func (m *WSReconnectManager) GetRetryCount() int {
	return int(atomic.LoadInt32(&m.retryCount))
}
