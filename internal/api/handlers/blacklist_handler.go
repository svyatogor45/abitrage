package handlers

import (
	"encoding/json"
	"errors"
	"net/http"

	"arbitrage/internal/service"

	"github.com/gorilla/mux"
)

// BlacklistHandler отвечает за управление черным списком торговых пар
//
// Endpoints:
// - GET /api/v1/blacklist - получение черного списка
// - POST /api/v1/blacklist - добавление пары в черный список
// - DELETE /api/v1/blacklist/{symbol} - удаление из черного списка
//
// Назначение:
// Обрабатывает запросы для справочного черного списка пар.
// Черный список носит ИНФОРМАТИВНЫЙ характер - это заметки пользователя
// о нежелательных парах. Бот НЕ фильтрует автоматически на основе этого списка.
type BlacklistHandler struct {
	blacklistService *service.BlacklistService
}

// NewBlacklistHandler создает новый BlacklistHandler с внедрением зависимостей.
func NewBlacklistHandler(blacklistService *service.BlacklistService) *BlacklistHandler {
	return &BlacklistHandler{
		blacklistService: blacklistService,
	}
}

// addToBlacklistRequest - структура запроса для добавления в черный список
type addToBlacklistRequest struct {
	Symbol string `json:"symbol"` // Торговый символ (например, "BTCUSDT")
	Reason string `json:"reason"` // Причина добавления (опционально)
}

// blacklistResponse - структура ответа со списком записей
type blacklistResponse struct {
	Entries []blacklistEntryResponse `json:"entries"`
	Total   int                      `json:"total"`
}

// blacklistEntryResponse - структура одной записи черного списка
type blacklistEntryResponse struct {
	ID        int    `json:"id"`
	Symbol    string `json:"symbol"`
	Reason    string `json:"reason"`
	CreatedAt string `json:"created_at"`
}

// GetBlacklist возвращает весь черный список пар
//
// GET /api/v1/blacklist
//
// Response 200:
//
//	{
//	  "entries": [
//	    {"id": 1, "symbol": "BTCUSDT", "reason": "Высокая волатильность", "created_at": "2025-01-15T10:30:00Z"},
//	    {"id": 2, "symbol": "ETHUSDT", "reason": "Низкая ликвидность", "created_at": "2025-01-14T09:00:00Z"}
//	  ],
//	  "total": 2
//	}
//
// Response 500:
//
//	{"error": "internal server error"}
func (h *BlacklistHandler) GetBlacklist(w http.ResponseWriter, r *http.Request) {
	entries, err := h.blacklistService.GetBlacklist()
	if err != nil {
		respondError(w, http.StatusInternalServerError, "Failed to get blacklist")
		return
	}

	// Формируем ответ
	response := blacklistResponse{
		Entries: make([]blacklistEntryResponse, 0, len(entries)),
		Total:   len(entries),
	}

	for _, entry := range entries {
		response.Entries = append(response.Entries, blacklistEntryResponse{
			ID:        entry.ID,
			Symbol:    entry.Symbol,
			Reason:    entry.Reason,
			CreatedAt: entry.CreatedAt.Format("2006-01-02T15:04:05Z"),
		})
	}

	respondJSON(w, http.StatusOK, response)
}

// AddToBlacklist добавляет пару в черный список
//
// POST /api/v1/blacklist
//
// Request:
//
//	{
//	  "symbol": "BTCUSDT",
//	  "reason": "Высокая волатильность"
//	}
//
// Response 201:
//
//	{
//	  "id": 1,
//	  "symbol": "BTCUSDT",
//	  "reason": "Высокая волатильность",
//	  "created_at": "2025-01-15T10:30:00Z"
//	}
//
// Response 400:
//
//	{"error": "symbol is required"}
//
// Response 409:
//
//	{"error": "symbol already in blacklist"}
//
// Response 500:
//
//	{"error": "internal server error"}
func (h *BlacklistHandler) AddToBlacklist(w http.ResponseWriter, r *http.Request) {
	var req addToBlacklistRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		respondError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// Валидация
	if req.Symbol == "" {
		respondError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	// Добавляем в черный список
	entry, err := h.blacklistService.AddToBlacklist(req.Symbol, req.Reason)
	if err != nil {
		if errors.Is(err, service.ErrBlacklistSymbolEmpty) {
			respondError(w, http.StatusBadRequest, "symbol is required")
			return
		}
		if errors.Is(err, service.ErrBlacklistSymbolExists) {
			respondError(w, http.StatusConflict, "symbol already in blacklist")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to add to blacklist")
		return
	}

	// Формируем ответ
	response := blacklistEntryResponse{
		ID:        entry.ID,
		Symbol:    entry.Symbol,
		Reason:    entry.Reason,
		CreatedAt: entry.CreatedAt.Format("2006-01-02T15:04:05Z"),
	}

	respondJSON(w, http.StatusCreated, response)
}

// RemoveFromBlacklist удаляет пару из черного списка
//
// DELETE /api/v1/blacklist/{symbol}
//
// Response 204: No Content (успешное удаление)
//
// Response 400:
//
//	{"error": "symbol is required"}
//
// Response 404:
//
//	{"error": "symbol not found in blacklist"}
//
// Response 500:
//
//	{"error": "internal server error"}
func (h *BlacklistHandler) RemoveFromBlacklist(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	symbol := vars["symbol"]

	if symbol == "" {
		respondError(w, http.StatusBadRequest, "symbol is required")
		return
	}

	err := h.blacklistService.RemoveFromBlacklist(symbol)
	if err != nil {
		if errors.Is(err, service.ErrBlacklistSymbolEmpty) {
			respondError(w, http.StatusBadRequest, "symbol is required")
			return
		}
		if errors.Is(err, service.ErrBlacklistEntryNotFound) {
			respondError(w, http.StatusNotFound, "symbol not found in blacklist")
			return
		}
		respondError(w, http.StatusInternalServerError, "Failed to remove from blacklist")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// respondJSON отправляет JSON ответ
func respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// respondError отправляет JSON ответ с ошибкой
func respondError(w http.ResponseWriter, status int, message string) {
	respondJSON(w, status, map[string]string{"error": message})
}
