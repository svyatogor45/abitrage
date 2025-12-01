package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// ExchangeHandler отвечает за управление биржевыми аккаунтами
//
// Функции:
// - Подключение биржи (POST /api/exchanges/{name}/connect)
// - Отключение биржи (DELETE /api/exchanges/{name}/connect)
// - Получение списка бирж и их статусов (GET /api/exchanges)
// - Обновление баланса биржи (GET /api/exchanges/{name}/balance)
//
// Назначение:
// Обрабатывает HTTP запросы для управления API ключами бирж,
// проверяет валидность ключей через тестовые запросы к биржам,
// шифрует и сохраняет ключи в базу данных
type ExchangeHandler struct {
	// TODO: добавить зависимости (service, repository)
}

// NewExchangeHandler создает новый ExchangeHandler
func NewExchangeHandler() *ExchangeHandler {
	return &ExchangeHandler{}
}

// ConnectExchange подключает биржу с API ключами
// POST /api/exchanges/{name}/connect
func (h *ExchangeHandler) ConnectExchange(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchangeName := vars["name"]

	// TODO:
	// 1. Декодировать JSON body (apiKey, secretKey, passphrase)
	// 2. Валидировать входные данные
	// 3. Проверить поддержку биржи
	// 4. Создать тестовое подключение к бирже
	// 5. Шифровать ключи
	// 6. Сохранить в БД через service
	// 7. Вернуть результат

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Exchange " + exchangeName + " connected (not implemented)",
	})
}

// DisconnectExchange отключает биржу (удаляет API ключи)
// DELETE /api/exchanges/{name}/connect
func (h *ExchangeHandler) DisconnectExchange(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchangeName := vars["name"]

	// TODO:
	// 1. Проверить, нет ли активных пар на этой бирже
	// 2. Удалить ключи из БД
	// 3. Остановить WebSocket соединения
	// 4. Вернуть результат

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Exchange " + exchangeName + " disconnected (not implemented)",
	})
}

// GetExchanges возвращает список всех бирж с их статусами
// GET /api/exchanges
func (h *ExchangeHandler) GetExchanges(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Получить список всех поддерживаемых бирж
	// 2. Для каждой получить статус подключения из БД
	// 3. Получить текущий баланс (если подключена)
	// 4. Вернуть массив объектов с информацией о биржах

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]map[string]interface{}{
		{
			"name":      "bybit",
			"connected": false,
			"balance":   0,
		},
	})
}

// GetExchangeBalance обновляет и возвращает баланс конкретной биржи
// GET /api/exchanges/{name}/balance
func (h *ExchangeHandler) GetExchangeBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchangeName := vars["name"]

	// TODO:
	// 1. Проверить подключена ли биржа
	// 2. Запросить баланс через exchange API
	// 3. Обновить баланс в БД
	// 4. Вернуть актуальный баланс

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"exchange": exchangeName,
		"balance":  0,
	})
}
