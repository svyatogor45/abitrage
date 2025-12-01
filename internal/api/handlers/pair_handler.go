package handlers

import (
	"encoding/json"
	"net/http"

	"github.com/gorilla/mux"
)

// PairHandler отвечает за управление торговыми парами
//
// Функции:
// - Добавление новой торговой пары (POST /api/pairs)
// - Получение списка всех пар (GET /api/pairs)
// - Получение конкретной пары (GET /api/pairs/{id})
// - Редактирование параметров пары (PATCH /api/pairs/{id})
// - Удаление пары (DELETE /api/pairs/{id})
// - Запуск мониторинга пары (POST /api/pairs/{id}/start)
// - Приостановка пары (POST /api/pairs/{id}/pause)
//
// Назначение:
// Обрабатывает CRUD операции для торговых пар,
// валидирует параметры (спреды, объемы, лимиты),
// проверяет доступность актива на минимум 2 биржах,
// управляет запуском и остановкой мониторинга пар ботом
type PairHandler struct {
	// TODO: добавить зависимости (service)
}

// NewPairHandler создает новый PairHandler
func NewPairHandler() *PairHandler {
	return &PairHandler{}
}

// CreatePair добавляет новую торговую пару
// POST /api/pairs
func (h *PairHandler) CreatePair(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Декодировать JSON body (symbol, entrySpread, exitSpread, volume, nOrders, stopLoss)
	// 2. Валидировать параметры:
	//    - volume > 0
	//    - entrySpread > 0 && exitSpread > 0
	//    - entrySpread > exitSpread
	//    - nOrders >= 1
	// 3. Проверить что актив доступен минимум на 2 подключенных биржах
	// 4. Создать пару через service (статус: paused)
	// 5. Вернуть созданный объект

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(map[string]string{
		"message": "Pair created (not implemented)",
	})
}

// GetPairs возвращает список всех торговых пар
// GET /api/pairs
func (h *PairHandler) GetPairs(w http.ResponseWriter, r *http.Request) {
	// TODO:
	// 1. Получить все пары из БД через service
	// 2. Для каждой пары получить runtime данные (если активна)
	// 3. Вернуть массив пар со статистикой

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode([]map[string]interface{}{})
}

// GetPair возвращает конкретную пару по ID
// GET /api/pairs/{id}
func (h *PairHandler) GetPair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pairID := vars["id"]

	// TODO:
	// 1. Получить пару из БД по ID
	// 2. Если не найдена - 404
	// 3. Получить runtime данные если активна
	// 4. Вернуть объект пары

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id": pairID,
	})
}

// UpdatePair обновляет параметры торговой пары
// PATCH /api/pairs/{id}
func (h *PairHandler) UpdatePair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pairID := vars["id"]

	// TODO:
	// 1. Декодировать JSON body с изменениями
	// 2. Валидировать новые параметры
	// 3. Проверить состояние пары (открыта ли позиция)
	// 4. Если позиция открыта - отложенное применение
	// 5. Если нет - применить немедленно
	// 6. Обновить в БД через service
	// 7. Вернуть обновленный объект

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":      pairID,
		"message": "Pair updated (not implemented)",
	})
}

// DeletePair удаляет торговую пару
// DELETE /api/pairs/{id}
func (h *PairHandler) DeletePair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pairID := vars["id"]

	// TODO:
	// 1. Проверить что пара на паузе
	// 2. Проверить что нет открытых позиций
	// 3. Если есть позиции - 409 Conflict
	// 4. Удалить из БД
	// 5. Вернуть 204 No Content

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"message": "Pair " + pairID + " deleted (not implemented)",
	})
}

// StartPair запускает мониторинг торговой пары
// POST /api/pairs/{id}/start
func (h *PairHandler) StartPair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pairID := vars["id"]

	// TODO:
	// 1. Получить пару из БД
	// 2. Проверить что минимум 2 биржи подключены
	// 3. Изменить статус на "active"
	// 4. Сигнализировать боту для начала мониторинга
	// 5. Вернуть обновленный статус

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     pairID,
		"status": "active",
	})
}

// PausePair приостанавливает торговую пару
// POST /api/pairs/{id}/pause
func (h *PairHandler) PausePair(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	pairID := vars["id"]

	// TODO:
	// 1. Получить пару из БД
	// 2. Проверить наличие открытых позиций
	// 3. Если позиции открыты:
	//    - Опционально: принудительно закрыть (query param ?force=true)
	//    - Или вернуть ошибку 409 Conflict
	// 4. Изменить статус на "paused"
	// 5. Остановить мониторинг ботом
	// 6. Вернуть обновленный статус

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     pairID,
		"status": "paused",
	})
}
