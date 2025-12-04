package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strings"

	"arbitrage/internal/exchange"
	"arbitrage/internal/service"

	"github.com/gorilla/mux"
)

// ConnectExchangeRequest - тело запроса для подключения биржи
type ConnectExchangeRequest struct {
	APIKey     string `json:"api_key"`
	SecretKey  string `json:"secret_key"`
	Passphrase string `json:"passphrase,omitempty"` // для OKX
}

// ExchangeResponse - ответ с информацией о бирже
type ExchangeResponse struct {
	Name      string  `json:"name"`
	Connected bool    `json:"connected"`
	Balance   float64 `json:"balance"`
	LastError string  `json:"last_error,omitempty"`
}

// BalanceResponse - ответ с балансом биржи
type BalanceResponse struct {
	Exchange string  `json:"exchange"`
	Balance  float64 `json:"balance"`
	Currency string  `json:"currency"`
}

// MaxRequestBodySize ограничение размера тела запроса (1 MB)
const MaxRequestBodySize = 1 << 20 // 1 MB

// ExchangeHandler отвечает за управление биржевыми аккаунтами
//
// Endpoints:
// - POST /api/v1/exchanges/{name}/connect - подключение биржи
// - DELETE /api/v1/exchanges/{name}/connect - отключение биржи
// - GET /api/v1/exchanges - получение списка бирж и их статусов
// - GET /api/v1/exchanges/{name}/balance - обновление баланса биржи
type ExchangeHandler struct {
	exchangeService service.ExchangeServiceInterface
}

// NewExchangeHandler создает новый ExchangeHandler
func NewExchangeHandler(exchangeService service.ExchangeServiceInterface) *ExchangeHandler {
	return &ExchangeHandler{
		exchangeService: exchangeService,
	}
}

// ConnectExchange подключает биржу с API ключами
// POST /api/v1/exchanges/{name}/connect
//
// Тело запроса:
//
//	{
//	  "api_key": "your-api-key",
//	  "secret_key": "your-secret-key",
//	  "passphrase": "optional-passphrase" // для OKX
//	}
//
// Ответы:
// - 200 OK: биржа успешно подключена
// - 400 Bad Request: некорректные данные
// - 401 Unauthorized: неверные API ключи
// - 409 Conflict: биржа уже подключена
func (h *ExchangeHandler) ConnectExchange(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchangeName := strings.ToLower(vars["name"])

	// 1. Проверяем поддержку биржи
	if !exchange.IsSupported(exchangeName) {
		h.respondWithError(w, http.StatusBadRequest, "Unsupported exchange", "Supported exchanges: "+strings.Join(exchange.SupportedExchanges, ", "))
		return
	}

	// 2. Ограничиваем размер тела запроса и декодируем
	r.Body = http.MaxBytesReader(w, r.Body, MaxRequestBodySize)
	var req ConnectExchangeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "Invalid request body", err.Error())
		return
	}

	// 3. Валидация входных данных
	if req.APIKey == "" {
		h.respondWithError(w, http.StatusBadRequest, "API key is required", "")
		return
	}
	if req.SecretKey == "" {
		h.respondWithError(w, http.StatusBadRequest, "Secret key is required", "")
		return
	}

	// OKX требует passphrase
	if exchangeName == "okx" && req.Passphrase == "" {
		h.respondWithError(w, http.StatusBadRequest, "Passphrase is required for OKX", "")
		return
	}

	// 4. Подключаем биржу через сервис
	ctx := r.Context()
	err := h.exchangeService.ConnectExchange(ctx, exchangeName, req.APIKey, req.SecretKey, req.Passphrase)
	if err != nil {
		// Определяем тип ошибки и возвращаем соответствующий HTTP код
		switch {
		case errors.Is(err, service.ErrExchangeNotSupported):
			h.respondWithError(w, http.StatusBadRequest, "Exchange not supported", err.Error())
		case errors.Is(err, service.ErrExchangeAlreadyConnected):
			h.respondWithError(w, http.StatusConflict, "Exchange is already connected", "Disconnect first to change credentials")
		case errors.Is(err, service.ErrInvalidCredentials):
			h.respondWithError(w, http.StatusUnauthorized, "Invalid API credentials", err.Error())
		case errors.Is(err, service.ErrConnectionFailed):
			h.respondWithError(w, http.StatusBadGateway, "Failed to connect to exchange", err.Error())
		default:
			h.respondWithError(w, http.StatusInternalServerError, "Internal server error", err.Error())
		}
		return
	}

	// 5. Получаем обновленную информацию о бирже
	account, err := h.exchangeService.GetExchangeByName(exchangeName)
	if err != nil {
		// Биржа подключена, но не можем получить данные - все равно возвращаем успех
		h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
			"message":   "Exchange connected successfully",
			"name":      exchangeName,
			"connected": true,
		})
		return
	}

	// 6. Возвращаем успешный ответ
	h.respondWithJSON(w, http.StatusOK, ExchangeResponse{
		Name:      account.Name,
		Connected: account.Connected,
		Balance:   account.Balance,
		LastError: account.LastError,
	})
}

// DisconnectExchange отключает биржу (удаляет API ключи)
// DELETE /api/v1/exchanges/{name}/connect
//
// Ответы:
// - 200 OK: биржа отключена
// - 400 Bad Request: биржа не поддерживается
// - 404 Not Found: биржа не подключена
func (h *ExchangeHandler) DisconnectExchange(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchangeName := strings.ToLower(vars["name"])

	// 1. Проверяем поддержку биржи
	if !exchange.IsSupported(exchangeName) {
		h.respondWithError(w, http.StatusBadRequest, "Unsupported exchange", "Supported exchanges: "+strings.Join(exchange.SupportedExchanges, ", "))
		return
	}

	// 2. Отключаем биржу через сервис
	ctx := r.Context()
	err := h.exchangeService.DisconnectExchange(ctx, exchangeName)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrExchangeNotConnected):
			h.respondWithError(w, http.StatusNotFound, "Exchange is not connected", "")
		case errors.Is(err, service.ErrHasActivePositions):
			h.respondWithError(w, http.StatusConflict, "Cannot disconnect: exchange has active positions", "Close all positions first")
		default:
			h.respondWithError(w, http.StatusInternalServerError, "Internal server error", err.Error())
		}
		return
	}

	// 3. Возвращаем успешный ответ
	h.respondWithJSON(w, http.StatusOK, map[string]interface{}{
		"message":   "Exchange disconnected successfully",
		"name":      exchangeName,
		"connected": false,
	})
}

// GetExchanges возвращает список всех бирж с их статусами
// GET /api/v1/exchanges
//
// Ответ:
//
//	[
//	  {
//	    "name": "bybit",
//	    "connected": true,
//	    "balance": 1500.00,
//	    "last_error": ""
//	  },
//	  ...
//	]
func (h *ExchangeHandler) GetExchanges(w http.ResponseWriter, r *http.Request) {
	// 1. Получаем список всех бирж через сервис
	accounts, err := h.exchangeService.GetAllExchanges()
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "Failed to get exchanges", err.Error())
		return
	}

	// 2. Формируем ответ
	response := make([]ExchangeResponse, 0, len(accounts))
	for _, account := range accounts {
		response = append(response, ExchangeResponse{
			Name:      account.Name,
			Connected: account.Connected,
			Balance:   account.Balance,
			LastError: account.LastError,
		})
	}

	// 3. Возвращаем список
	h.respondWithJSON(w, http.StatusOK, response)
}

// GetExchangeBalance обновляет и возвращает баланс конкретной биржи
// GET /api/v1/exchanges/{name}/balance
//
// Ответ:
//
//	{
//	  "exchange": "bybit",
//	  "balance": 1500.00,
//	  "currency": "USDT"
//	}
func (h *ExchangeHandler) GetExchangeBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	exchangeName := strings.ToLower(vars["name"])

	// 1. Проверяем поддержку биржи
	if !exchange.IsSupported(exchangeName) {
		h.respondWithError(w, http.StatusBadRequest, "Unsupported exchange", "Supported exchanges: "+strings.Join(exchange.SupportedExchanges, ", "))
		return
	}

	// 2. Обновляем баланс через сервис
	ctx := r.Context()
	balance, err := h.exchangeService.UpdateBalance(ctx, exchangeName)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrExchangeNotConnected):
			h.respondWithError(w, http.StatusNotFound, "Exchange is not connected", "Connect the exchange first")
		default:
			h.respondWithError(w, http.StatusBadGateway, "Failed to get balance from exchange", err.Error())
		}
		return
	}

	// 3. Возвращаем баланс
	h.respondWithJSON(w, http.StatusOK, BalanceResponse{
		Exchange: exchangeName,
		Balance:  balance,
		Currency: "USDT",
	})
}

// respondWithJSON отправляет JSON ответ
func (h *ExchangeHandler) respondWithJSON(w http.ResponseWriter, code int, payload interface{}) {
	response, err := json.Marshal(payload)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`{"error":"Failed to marshal response"}`))
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_, _ = w.Write(response)
}

// respondWithError отправляет JSON ответ с ошибкой
func (h *ExchangeHandler) respondWithError(w http.ResponseWriter, code int, message string, details string) {
	h.respondWithJSON(w, code, ErrorResponse{
		Error:   message,
		Details: details,
	})
}
