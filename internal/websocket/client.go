package websocket

import (
	"log"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

const (
	// Время ожидания записи сообщения
	writeWait = 10 * time.Second

	// Время ожидания между pong сообщениями
	pongWait = 60 * time.Second

	// Интервал отправки ping сообщений (должен быть меньше pongWait)
	pingPeriod = (pongWait * 9) / 10

	// Максимальный размер сообщения
	// ОПТИМИЗАЦИЯ: увеличено с 512 до 65536 (64KB)
	// Причина: JSON с состоянием пары (Legs, PNL, цены) легко превышает 512 байт
	// Типичный размер сообщения pairUpdate: 1-4KB
	maxMessageSize = 65536
)

var upgrader = websocket.Upgrader{
	ReadBufferSize:  4096,  // увеличено для больших сообщений
	WriteBufferSize: 4096,  // увеличено для больших сообщений
	CheckOrigin: func(r *http.Request) bool {
		// TODO: проверять origin в production
		return true
	},
}

// Client представляет одно WebSocket соединение
//
// Назначение:
// Управляет индивидуальным WebSocket соединением клиента.
// Обрабатывает чтение и запись сообщений для конкретного клиента.
//
// Функции:
// - Отправка сообщений конкретному клиенту
// - Обработка входящих сообщений от клиента (если нужно)
// - Ping/Pong для проверки живости соединения
// - Буферизация исходящих сообщений
// - Graceful закрытие соединения
// - Обработка ошибок соединения
//
// Архитектура:
// Каждый клиент имеет две горутины:
// 1. readPump - читает сообщения от клиента
// 2. writePump - пишет сообщения клиенту
//
// Использование:
// 1. ServeWS создает нового клиента при подключении
// 2. Клиент регистрируется в Hub
// 3. Запускаются readPump и writePump горутины
// 4. При отключении клиент удаляется из Hub
type Client struct {
	// WebSocket соединение
	conn *websocket.Conn

	// Hub которому принадлежит клиент
	hub *Hub

	// Буферизованный канал исходящих сообщений
	send chan []byte
}

// readPump читает сообщения от клиента
//
// Запускается в отдельной горутине для каждого клиента.
// Обрабатывает входящие сообщения и контролирует соединение.
func (c *Client) readPump() {
	defer func() {
		c.hub.unregister <- c
		c.conn.Close()
	}()

	c.conn.SetReadLimit(maxMessageSize)
	c.conn.SetReadDeadline(time.Now().Add(pongWait))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	for {
		_, message, err := c.conn.ReadMessage()
		if err != nil {
			if websocket.IsUnexpectedCloseError(err, websocket.CloseGoingAway, websocket.CloseAbnormalClosure) {
				log.Printf("WebSocket error: %v", err)
			}
			break
		}

		// TODO: обработка входящих сообщений от клиента если нужно
		_ = message
		// Обычно WebSocket используется только для отправки данных от сервера к клиенту
		// Но можно добавить обработку команд от клиента здесь
	}
}

// writePump отправляет сообщения клиенту
//
// Запускается в отдельной горутине для каждого клиента.
// Читает из канала send и отправляет через WebSocket.
func (c *Client) writePump() {
	ticker := time.NewTicker(pingPeriod)
	defer func() {
		ticker.Stop()
		c.conn.Close()
	}()

	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if !ok {
				// Hub закрыл канал
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}

			w, err := c.conn.NextWriter(websocket.TextMessage)
			if err != nil {
				return
			}
			w.Write(message)

			// ОПТИМИЗАЦИЯ: безопасное чтение из буфера без race condition
			// Было: n := len(c.send); for i := 0; i < n; i++ { <-c.send }
			// Проблема: между len() и <- канал мог измениться
			// Решение: non-blocking select в цикле
		drainLoop:
			for {
				select {
				case msg, ok := <-c.send:
					if !ok {
						// Канал закрыт
						break drainLoop
					}
					w.Write([]byte{'\n'})
					w.Write(msg)
				default:
					// Буфер пуст - выходим
					break drainLoop
				}
			}

			if err := w.Close(); err != nil {
				return
			}

		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(writeWait))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		}
	}
}

// ServeWS обрабатывает WebSocket запросы от клиента
//
// HTTP handler для WebSocket endpoint.
// Апгрейдит HTTP соединение до WebSocket.
// Создает нового клиента и запускает его горутины.
//
// Использование в routes:
// router.HandleFunc("/ws/stream", hub.ServeWS)
func ServeWS(hub *Hub, w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade error: %v", err)
		return
	}

	client := &Client{
		conn: conn,
		hub:  hub,
		send: make(chan []byte, 256),
	}

	client.hub.register <- client

	// Запускаем горутины клиента
	go client.writePump()
	go client.readPump()
}
