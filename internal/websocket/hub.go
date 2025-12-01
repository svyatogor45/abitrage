package websocket

import (
	"bytes"
	"encoding/json"
	"log"
	"sync"
)

// ============ ОПТИМИЗАЦИЯ: sync.Pool для JSON буферов ============
// Убирает аллокации при каждом Broadcast (было ~1000+/сек)

var jsonBufferPool = sync.Pool{
	New: func() interface{} {
		return bytes.NewBuffer(make([]byte, 0, 512)) // начальный размер 512 байт
	},
}

// ============ Типизированные сообщения (без map[string]interface{}) ============
// Избегаем рефлексии при сериализации - Go оптимизирует для известных типов

// PairUpdateMessage - сообщение об обновлении пары
type PairUpdateMessage struct {
	Type   string      `json:"type"`
	PairID int         `json:"pair_id"`
	Data   interface{} `json:"data"`
}

// NotificationMessage - сообщение с уведомлением
type NotificationMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// BalanceUpdateMessage - сообщение об обновлении баланса
type BalanceUpdateMessage struct {
	Type     string  `json:"type"`
	Exchange string  `json:"exchange"`
	Balance  float64 `json:"balance"`
}

// StatsUpdateMessage - сообщение со статистикой
type StatsUpdateMessage struct {
	Type string      `json:"type"`
	Data interface{} `json:"data"`
}

// Hub управляет всеми активными WebSocket соединениями
//
// Назначение:
// Центральный менеджер для broadcast сообщений всем подключенным клиентам.
// Обеспечивает real-time обновления данных на frontend без необходимости polling.
//
// Функции:
// - Регистрация новых WebSocket клиентов
// - Отмена регистрации отключенных клиентов
// - Broadcast сообщений всем активным клиентам
// - Маршрутизация сообщений по типам (pairUpdate, notification, balanceUpdate)
// - Обработка переподключений
// - Очистка отключенных соединений
// - Потокобезопасная работа с клиентами (sync.RWMutex)
//
// Типы сообщений:
// - pairUpdate: обновление состояния пары (цены, PNL, спред)
// - notification: новое уведомление
// - balanceUpdate: обновление баланса биржи
// - statsUpdate: обновление статистики
//
// Использование:
// 1. Создать hub: hub := NewHub()
// 2. Запустить в горутине: go hub.Run()
// 3. Отправлять сообщения: hub.Broadcast(message)
type Hub struct {
	// Зарегистрированные клиенты
	clients map[*Client]bool

	// Broadcast канал для отправки сообщений всем клиентам
	broadcast chan []byte

	// Регистрация нового клиента
	register chan *Client

	// Отмена регистрации клиента
	unregister chan *Client

	// Mutex для потокобезопасного доступа к clients
	mu sync.RWMutex
}

// NewHub создает новый Hub
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *Client),
		unregister: make(chan *Client),
	}
}

// Run запускает главный цикл Hub
//
// Должен запускаться в отдельной горутине: go hub.Run()
// Обрабатывает регистрацию, отмену регистрации и broadcast
//
// ОПТИМИЗАЦИЯ: исправлен race condition при удалении клиентов под RLock
// Теперь: копируем список → отправляем без Lock → удаляем под Write Lock
func (h *Hub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("Client connected. Total clients: %d", len(h.clients))

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("Client disconnected. Total clients: %d", len(h.clients))

		case message := <-h.broadcast:
			// ОПТИМИЗАЦИЯ: копируем список клиентов под коротким RLock
			h.mu.RLock()
			clients := make([]*Client, 0, len(h.clients))
			for client := range h.clients {
				clients = append(clients, client)
			}
			h.mu.RUnlock()

			// Отправляем сообщения БЕЗ блокировки (не блокируем register/unregister)
			var toRemove []*Client
			for _, client := range clients {
				select {
				case client.send <- message:
					// Сообщение отправлено успешно
				default:
					// Клиент не успевает обрабатывать сообщения - помечаем для удаления
					toRemove = append(toRemove, client)
				}
			}

			// Удаляем медленных клиентов под Write Lock
			if len(toRemove) > 0 {
				h.mu.Lock()
				for _, client := range toRemove {
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						close(client.send)
					}
				}
				h.mu.Unlock()
				log.Printf("Removed %d slow clients. Total clients: %d", len(toRemove), len(h.clients))
			}
		}
	}
}

// Broadcast отправляет сообщение всем подключенным клиентам
// ОПТИМИЗАЦИЯ: использует sync.Pool для буферов (убирает аллокации)
func (h *Hub) Broadcast(message interface{}) {
	// Получаем буфер из пула
	buf := jsonBufferPool.Get().(*bytes.Buffer)
	buf.Reset()

	// Сериализуем в буфер
	if err := json.NewEncoder(buf).Encode(message); err != nil {
		log.Printf("Error marshaling broadcast message: %v", err)
		jsonBufferPool.Put(buf)
		return
	}

	// Убираем trailing newline от Encode
	data := buf.Bytes()
	if len(data) > 0 && data[len(data)-1] == '\n' {
		data = data[:len(data)-1]
	}

	// Копируем данные (буфер вернётся в пул)
	msgCopy := make([]byte, len(data))
	copy(msgCopy, data)

	// Возвращаем буфер в пул
	jsonBufferPool.Put(buf)

	h.broadcast <- msgCopy
}

// BroadcastPairUpdate отправляет обновление состояния пары
// ОПТИМИЗАЦИЯ: использует типизированную структуру вместо map[string]interface{}
func (h *Hub) BroadcastPairUpdate(pairID int, data interface{}) {
	h.Broadcast(&PairUpdateMessage{
		Type:   "pairUpdate",
		PairID: pairID,
		Data:   data,
	})
}

// BroadcastNotification отправляет новое уведомление
// ОПТИМИЗАЦИЯ: использует типизированную структуру
func (h *Hub) BroadcastNotification(notification interface{}) {
	h.Broadcast(&NotificationMessage{
		Type: "notification",
		Data: notification,
	})
}

// BroadcastBalanceUpdate отправляет обновление баланса биржи
// ОПТИМИЗАЦИЯ: использует типизированную структуру (все поля примитивные - быстрая сериализация)
func (h *Hub) BroadcastBalanceUpdate(exchange string, balance float64) {
	h.Broadcast(&BalanceUpdateMessage{
		Type:     "balanceUpdate",
		Exchange: exchange,
		Balance:  balance,
	})
}

// BroadcastStatsUpdate отправляет обновление статистики
// ОПТИМИЗАЦИЯ: использует типизированную структуру
func (h *Hub) BroadcastStatsUpdate(stats interface{}) {
	h.Broadcast(&StatsUpdateMessage{
		Type: "statsUpdate",
		Data: stats,
	})
}

// ClientCount возвращает количество подключенных клиентов
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
