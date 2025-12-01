package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// BlacklistHandler отвечает за управление черным списком торговых пар
//
// Функции:
// - Получение черного списка (GET /api/blacklist)
// - Добавление пары в черный список (POST /api/blacklist)
// - Удаление из черного списка (DELETE /api/blacklist/{symbol})
//
// Назначение:
// Обрабатывает запросы для справочного черного списка пар.
// Черный список носит информативный характер - это заметки пользователя
// о нежелательных парах. Бот НЕ фильтрует автоматически на основе этого списка.
// Пользователь может добавить причину, почему пара в черном списке.
type BlacklistHandler struct {
	// TODO: добавить зависимости (service)
}

// NewBlacklistHandler создает новый BlacklistHandler
func NewBlacklistHandler() *BlacklistHandler {
	return &BlacklistHandler{}
}

// GetBlacklist возвращает весь черный список пар
// GET /api/blacklist
func (h *BlacklistHandler) GetBlacklist(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Получить все записи из БД через service
	// 2. Вернуть массив объектов {id, symbol, reason, created_at}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]map[string]interface{}{})
}

// AddToBlacklist добавляет пару в черный список
// POST /api/blacklist
func (h *BlacklistHandler) AddToBlacklist(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Декодировать JSON body {symbol, reason}
	// 2. Валидировать symbol (не пустой)
	// 3. Проверить что пара еще не в черном списке
	// 4. Добавить в БД через service
	// 5. Вернуть созданный объект

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Added to blacklist (not implemented)",
	})
}

// RemoveFromBlacklist удаляет пару из черного списка
// DELETE /api/blacklist/{symbol}
func (h *BlacklistHandler) RemoveFromBlacklist(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	// TODO:
	// 1. Получить symbol из URL
	// 2. Удалить из БД через service
	// 3. Вернуть 204 No Content

	_ = symbol
	w.WriteHeader(http.StatusNoContent)
}
