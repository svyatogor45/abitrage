package websocket

import (
	"log"
	"sync"

	// ОПТИМИЗАЦИЯ: jsoniter в 3-5x быстрее стандартного encoding/json
	// При 1000+ сообщений/сек экономит ~2-5ms CPU в секунду
	jsoniter "github.com/json-iterator/go"
)

// Быстрый JSON сериализатор (совместим со стандартным encoding/json API)
var json = jsoniter.ConfigCompatibleWithStandardLibrary

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
//
// ОПТИМИЗАЦИЯ: увеличен буфер broadcast до 4096
// При 1000+ сообщений/сек буфер 256 заполнялся за 256ms
// Теперь буфер на ~4 секунды - достаточно для пиковых нагрузок
func NewHub() *Hub {
	return &Hub{
		clients:    make(map[*Client]bool),
		broadcast:  make(chan []byte, 4096), // было 256, стало 4096
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
func (h *Hub) Broadcast(message interface{}) {
	data, err := json.Marshal(message)
	if err != nil {
		log.Printf("Error marshaling broadcast message: %v", err)
		return
	}

	h.broadcast <- data
}

// BroadcastPairUpdate отправляет обновление состояния пары
func (h *Hub) BroadcastPairUpdate(pairID int, data interface{}) {
	message := map[string]interface{}{
		"type":    "pairUpdate",
		"pair_id": pairID,
		"data":    data,
	}
	h.Broadcast(message)
}

// BroadcastNotification отправляет новое уведомление
func (h *Hub) BroadcastNotification(notification interface{}) {
	message := map[string]interface{}{
		"type": "notification",
		"data": notification,
	}
	h.Broadcast(message)
}

// BroadcastBalanceUpdate отправляет обновление баланса биржи
func (h *Hub) BroadcastBalanceUpdate(exchange string, balance float64) {
	message := map[string]interface{}{
		"type":     "balanceUpdate",
		"exchange": exchange,
		"balance":  balance,
	}
	h.Broadcast(message)
}

// BroadcastStatsUpdate отправляет обновление статистики
func (h *Hub) BroadcastStatsUpdate(stats interface{}) {
	message := map[string]interface{}{
		"type": "statsUpdate",
		"data": stats,
	}
	h.Broadcast(message)
}

// ClientCount возвращает количество подключенных клиентов
func (h *Hub) ClientCount() int {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return len(h.clients)
}
