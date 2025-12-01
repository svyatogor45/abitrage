package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"

	"arbitrage/internal/models"
	"arbitrage/internal/service"

	"github.com/gorilla/mux"
)

// PairHandler отвечает за управление торговыми парами
//
// Endpoints:
// - POST /api/v1/pairs           - добавление новой торговой пары
// - GET /api/v1/pairs            - получение списка всех пар
// - GET /api/v1/pairs/{id}       - получение конкретной пары
// - PATCH /api/v1/pairs/{id}     - редактирование параметров пары
// - DELETE /api/v1/pairs/{id}    - удаление пары
// - POST /api/v1/pairs/{id}/start - запуск мониторинга пары
// - POST /api/v1/pairs/{id}/pause - приостановка пары
type PairHandler struct {
	pairService *service.PairService
}

// NewPairHandler создает новый PairHandler с внедрением зависимостей
func NewPairHandler(pairService *service.PairService) *PairHandler {
	return &PairHandler{
		pairService: pairService,
	}
}

// CreatePairRequest структура запроса на создание пары
type CreatePairRequest struct {
	Symbol         string  `json:"symbol"`       // BTCUSDT
	Base           string  `json:"base"`         // BTC
	Quote          string  `json:"quote"`        // USDT
	EntrySpreadPct float64 `json:"entry_spread"` // % для входа
	ExitSpreadPct  float64 `json:"exit_spread"`  // % для выхода
	VolumeAsset    float64 `json:"volume"`       // объем в монетах
	NOrders        int     `json:"n_orders"`     // количество частей (default: 1)
	StopLoss       float64 `json:"stop_loss"`    // в USDT (опционально)
}

// UpdatePairRequest структура запроса на обновление пары
type UpdatePairRequest struct {
	EntrySpreadPct *float64 `json:"entry_spread,omitempty"`
	ExitSpreadPct  *float64 `json:"exit_spread,omitempty"`
	VolumeAsset    *float64 `json:"volume,omitempty"`
	NOrders        *int     `json:"n_orders,omitempty"`
	StopLoss       *float64 `json:"stop_loss,omitempty"`
}

// PairResponse структура ответа с данными пары
type PairResponse struct {
	ID             int                    `json:"id"`
	Symbol         string                 `json:"symbol"`
	Base           string                 `json:"base"`
	Quote          string                 `json:"quote"`
	EntrySpreadPct float64                `json:"entry_spread"`
	ExitSpreadPct  float64                `json:"exit_spread"`
	VolumeAsset    float64                `json:"volume"`
	NOrders        int                    `json:"n_orders"`
	StopLoss       float64                `json:"stop_loss"`
	Status         string                 `json:"status"`
	Stats          *PairStatsResponse     `json:"stats"`
	Runtime        *PairRuntimeResponse   `json:"runtime,omitempty"`
	PendingConfig  *PendingConfigResponse `json:"pending_config,omitempty"`
}

// PairStatsResponse статистика пары
type PairStatsResponse struct {
	TradesCount int     `json:"trades_count"`
	TotalPnl    float64 `json:"total_pnl"`
}

// PairRuntimeResponse runtime состояние пары
type PairRuntimeResponse struct {
	State          string        `json:"state"`
	Legs           []LegResponse `json:"legs,omitempty"`
	CurrentSpread  float64       `json:"current_spread"`
	UnrealizedPnl  float64       `json:"unrealized_pnl"`
	RealizedPnl    float64       `json:"realized_pnl"`
	FilledParts    int           `json:"filled_parts"`
}

// LegResponse данные об одной ноге позиции
type LegResponse struct {
	Exchange      string  `json:"exchange"`
	Side          string  `json:"side"`
	EntryPrice    float64 `json:"entry_price"`
	CurrentPrice  float64 `json:"current_price"`
	Quantity      float64 `json:"quantity"`
	UnrealizedPnl float64 `json:"unrealized_pnl"`
}

// PendingConfigResponse отложенные изменения конфигурации
type PendingConfigResponse struct {
	EntrySpreadPct float64 `json:"entry_spread"`
	ExitSpreadPct  float64 `json:"exit_spread"`
	VolumeAsset    float64 `json:"volume"`
	NOrders        int     `json:"n_orders"`
	StopLoss       float64 `json:"stop_loss"`
}

// CreatePair добавляет новую торговую пару
// POST /api/v1/pairs
//
// Request Body:
//
//	{
//	  "symbol": "BTCUSDT",
//	  "base": "BTC",
//	  "quote": "USDT",
//	  "entry_spread": 1.0,
//	  "exit_spread": 0.2,
//	  "volume": 0.5,
//	  "n_orders": 4,
//	  "stop_loss": 100
//	}
//
// Response:
// - 201 Created: пара создана
// - 400 Bad Request: невалидные параметры
// - 409 Conflict: пара уже существует или достигнут лимит
func (h *PairHandler) CreatePair(w http.ResponseWriter, r *http.Request) {
	var req CreatePairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body", err.Error())
		return
	}

	// Валидация обязательных полей
	if req.Symbol == "" {
		h.respondWithError(w, http.StatusBadRequest, "missing_symbol", "Symbol is required", "")
		return
	}

	// Создаем конфигурацию пары
	pairConfig := &models.PairConfig{
		Symbol:         req.Symbol,
		Base:           req.Base,
		Quote:          req.Quote,
		EntrySpreadPct: req.EntrySpreadPct,
		ExitSpreadPct:  req.ExitSpreadPct,
		VolumeAsset:    req.VolumeAsset,
		NOrders:        req.NOrders,
		StopLoss:       req.StopLoss,
	}

	// Вызываем сервис для создания пары
	err := h.pairService.CreatePair(r.Context(), pairConfig)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Возвращаем созданную пару
	h.respondWithJSON(w, http.StatusCreated, h.pairToResponse(pairConfig, nil, nil))
}

// GetPairs возвращает список всех торговых пар
// GET /api/v1/pairs
//
// Query Parameters:
// - status: фильтр по статусу (paused, active)
//
// Response:
// - 200 OK: массив пар
func (h *PairHandler) GetPairs(w http.ResponseWriter, r *http.Request) {
	// Получаем все пары
	pairs, err := h.pairService.GetAllPairs(r.Context())
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "internal_error", "Failed to get pairs", err.Error())
		return
	}

	// Опциональный фильтр по статусу
	statusFilter := r.URL.Query().Get("status")

	response := make([]PairResponse, 0, len(pairs))
	for _, pair := range pairs {
		// Применяем фильтр по статусу
		if statusFilter != "" && pair.Status != statusFilter {
			continue
		}

		// Получаем runtime данные
		runtime := h.pairService.GetPairRuntime(pair.ID)
		pending := h.pairService.GetPendingConfig(pair.ID)

		response = append(response, h.pairToResponse(pair, runtime, pending))
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

// GetPair возвращает конкретную пару по ID
// GET /api/v1/pairs/{id}
//
// Response:
// - 200 OK: данные пары
// - 404 Not Found: пара не найдена
func (h *PairHandler) GetPair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_id", "Invalid pair ID", "ID must be a number")
		return
	}

	// Получаем пару с runtime данными
	pairWithRuntime, err := h.pairService.GetPairWithRuntime(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	response := h.pairToResponse(pairWithRuntime.Config, pairWithRuntime.Runtime, pairWithRuntime.PendingConfig)
	h.respondWithJSON(w, http.StatusOK, response)
}

// UpdatePair обновляет параметры торговой пары
// PATCH /api/v1/pairs/{id}
//
// Request Body (все поля опциональны):
//
//	{
//	  "entry_spread": 1.5,
//	  "exit_spread": 0.3,
//	  "volume": 1.0,
//	  "n_orders": 2,
//	  "stop_loss": 150
//	}
//
// Response:
// - 200 OK: обновленная пара
// - 400 Bad Request: невалидные параметры
// - 404 Not Found: пара не найдена
//
// Note: если позиция открыта, изменения применятся после её закрытия
func (h *PairHandler) UpdatePair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_id", "Invalid pair ID", "ID must be a number")
		return
	}

	var req UpdatePairRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_request", "Invalid JSON body", err.Error())
		return
	}

	// Конвертируем в параметры сервиса
	params := service.UpdatePairParams{
		EntrySpreadPct: req.EntrySpreadPct,
		ExitSpreadPct:  req.ExitSpreadPct,
		VolumeAsset:    req.VolumeAsset,
		NOrders:        req.NOrders,
		StopLoss:       req.StopLoss,
	}

	// Обновляем пару
	updatedPair, err := h.pairService.UpdatePair(r.Context(), id, params)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Получаем runtime и pending данные
	runtime := h.pairService.GetPairRuntime(id)
	pending := h.pairService.GetPendingConfig(id)

	// Формируем ответ
	response := h.pairToResponse(updatedPair, runtime, pending)

	// Добавляем информацию об отложенных изменениях
	if pending != nil {
		response.PendingConfig = &PendingConfigResponse{
			EntrySpreadPct: pending.EntrySpreadPct,
			ExitSpreadPct:  pending.ExitSpreadPct,
			VolumeAsset:    pending.VolumeAsset,
			NOrders:        pending.NOrders,
			StopLoss:       pending.StopLoss,
		}
	}

	h.respondWithJSON(w, http.StatusOK, response)
}

// DeletePair удаляет торговую пару
// DELETE /api/v1/pairs/{id}
//
// Response:
// - 204 No Content: пара удалена
// - 404 Not Found: пара не найдена
// - 409 Conflict: пара активна или есть открытая позиция
func (h *PairHandler) DeletePair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_id", "Invalid pair ID", "ID must be a number")
		return
	}

	// Удаляем пару
	err = h.pairService.DeletePair(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// StartPair запускает мониторинг торговой пары
// POST /api/v1/pairs/{id}/start
//
// Response:
// - 200 OK: пара запущена
// - 404 Not Found: пара не найдена
// - 409 Conflict: пара уже активна или недостаточно бирж
func (h *PairHandler) StartPair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_id", "Invalid pair ID", "ID must be a number")
		return
	}

	// Запускаем пару
	err = h.pairService.StartPair(r.Context(), id)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Получаем обновленные данные пары
	pairWithRuntime, err := h.pairService.GetPairWithRuntime(r.Context(), id)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "internal_error", "Pair started but failed to fetch updated data", err.Error())
		return
	}

	response := h.pairToResponse(pairWithRuntime.Config, pairWithRuntime.Runtime, pairWithRuntime.PendingConfig)
	h.respondWithJSON(w, http.StatusOK, response)
}

// PausePair приостанавливает торговую пару
// POST /api/v1/pairs/{id}/pause
//
// Query Parameters:
// - force: если true, принудительно закрывает позицию (default: false)
//
// Response:
// - 200 OK: пара приостановлена
// - 404 Not Found: пара не найдена
// - 409 Conflict: есть открытая позиция и force=false
func (h *PairHandler) PausePair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	id, err := strconv.Atoi(vars["id"])
	if err != nil {
		h.respondWithError(w, http.StatusBadRequest, "invalid_id", "Invalid pair ID", "ID must be a number")
		return
	}

	// Проверяем параметр force
	forceClose := r.URL.Query().Get("force") == "true"

	// Останавливаем пару
	err = h.pairService.PausePair(r.Context(), id, forceClose)
	if err != nil {
		h.handleServiceError(w, err)
		return
	}

	// Получаем обновленные данные пары
	pairWithRuntime, err := h.pairService.GetPairWithRuntime(r.Context(), id)
	if err != nil {
		h.respondWithError(w, http.StatusInternalServerError, "internal_error", "Pair paused but failed to fetch updated data", err.Error())
		return
	}

	response := h.pairToResponse(pairWithRuntime.Config, pairWithRuntime.Runtime, pairWithRuntime.PendingConfig)
	h.respondWithJSON(w, http.StatusOK, response)
}

// ============ Helper методы ============

// pairToResponse конвертирует модель пары в ответ API
func (h *PairHandler) pairToResponse(pair *models.PairConfig, runtime *models.PairRuntime, pending *service.PendingConfig) PairResponse {
	response := PairResponse{
		ID:             pair.ID,
		Symbol:         pair.Symbol,
		Base:           pair.Base,
		Quote:          pair.Quote,
		EntrySpreadPct: pair.EntrySpreadPct,
		ExitSpreadPct:  pair.ExitSpreadPct,
		VolumeAsset:    pair.VolumeAsset,
		NOrders:        pair.NOrders,
		StopLoss:       pair.StopLoss,
		Status:         pair.Status,
		Stats: &PairStatsResponse{
			TradesCount: pair.TradesCount,
			TotalPnl:    pair.TotalPnl,
		},
	}

	// Добавляем runtime данные если есть
	if runtime != nil {
		runtimeResp := &PairRuntimeResponse{
			State:         runtime.State,
			CurrentSpread: runtime.CurrentSpread,
			UnrealizedPnl: runtime.UnrealizedPnl,
			RealizedPnl:   runtime.RealizedPnl,
			FilledParts:   runtime.FilledParts,
			Legs:          make([]LegResponse, 0, len(runtime.Legs)),
		}

		for _, leg := range runtime.Legs {
			runtimeResp.Legs = append(runtimeResp.Legs, LegResponse{
				Exchange:      leg.Exchange,
				Side:          leg.Side,
				EntryPrice:    leg.EntryPrice,
				CurrentPrice:  leg.CurrentPrice,
				Quantity:      leg.Quantity,
				UnrealizedPnl: leg.UnrealizedPnl,
			})
		}

		response.Runtime = runtimeResp
	}

	// Добавляем pending конфигурацию если есть
	if pending != nil {
		response.PendingConfig = &PendingConfigResponse{
			EntrySpreadPct: pending.EntrySpreadPct,
			ExitSpreadPct:  pending.ExitSpreadPct,
			VolumeAsset:    pending.VolumeAsset,
			NOrders:        pending.NOrders,
			StopLoss:       pending.StopLoss,
		}
	}

	return response
}

// handleServiceError обрабатывает ошибки от сервиса и возвращает соответствующий HTTP статус
func (h *PairHandler) handleServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, service.ErrPairNotFound):
		h.respondWithError(w, http.StatusNotFound, "pair_not_found", "Pair not found", "")

	case errors.Is(err, service.ErrPairAlreadyExists):
		h.respondWithError(w, http.StatusConflict, "pair_exists", "Pair with this symbol already exists", "")

	case errors.Is(err, service.ErrMaxPairsReached):
		h.respondWithError(w, http.StatusConflict, "max_pairs_reached", "Maximum number of pairs (30) reached", "")

	case errors.Is(err, service.ErrPairAlreadyActive):
		h.respondWithError(w, http.StatusConflict, "pair_already_active", "Pair is already active", "")

	case errors.Is(err, service.ErrPairAlreadyPaused):
		h.respondWithError(w, http.StatusConflict, "pair_already_paused", "Pair is already paused", "")

	case errors.Is(err, service.ErrPairNotPaused):
		h.respondWithError(w, http.StatusConflict, "pair_not_paused", "Pair must be paused to delete", "")

	case errors.Is(err, service.ErrPairHasOpenPosition):
		h.respondWithError(w, http.StatusConflict, "position_open", "Pair has open position. Use ?force=true to close it", "")

	case errors.Is(err, service.ErrNotEnoughExchanges):
		h.respondWithError(w, http.StatusConflict, "not_enough_exchanges", "At least 2 exchanges must be connected for arbitrage", "")

	case errors.Is(err, service.ErrSymbolNotAvailable):
		h.respondWithError(w, http.StatusBadRequest, "symbol_not_available", "Symbol must be available on at least 2 connected exchanges", "")

	case errors.Is(err, service.ErrInvalidEntrySpread):
		h.respondWithError(w, http.StatusBadRequest, "invalid_entry_spread", "Entry spread must be greater than 0", "")

	case errors.Is(err, service.ErrInvalidExitSpread):
		h.respondWithError(w, http.StatusBadRequest, "invalid_exit_spread", "Exit spread must be greater than 0", "")

	case errors.Is(err, service.ErrExitSpreadTooHigh):
		h.respondWithError(w, http.StatusBadRequest, "exit_spread_too_high", "Exit spread must be less than entry spread", "")

	case errors.Is(err, service.ErrInvalidVolume):
		h.respondWithError(w, http.StatusBadRequest, "invalid_volume", "Volume must be greater than 0", "")

	case errors.Is(err, service.ErrInvalidNOrders):
		h.respondWithError(w, http.StatusBadRequest, "invalid_n_orders", "Number of orders must be at least 1", "")

	case errors.Is(err, service.ErrInvalidStopLoss):
		h.respondWithError(w, http.StatusBadRequest, "invalid_stop_loss", "Stop loss must be non-negative", "")

	case errors.Is(err, service.ErrInvalidSymbol):
		h.respondWithError(w, http.StatusBadRequest, "invalid_symbol", "Invalid symbol format", "")

	default:
		h.respondWithError(w, http.StatusInternalServerError, "internal_error", "Internal server error", err.Error())
	}
}

// respondWithJSON отправляет JSON ответ
func (h *PairHandler) respondWithJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if data != nil {
		json.NewEncoder(w).Encode(data)
	}
}

// respondWithError отправляет JSON ответ с ошибкой
func (h *PairHandler) respondWithError(w http.ResponseWriter, statusCode int, code, message, details string) {
	response := ErrorResponse{
		Error:   message,
		Code:    code,
		Details: details,
	}
	h.respondWithJSON(w, statusCode, response)
}
