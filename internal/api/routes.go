package api

import (
	"net/http"

	"arbitrage/internal/api/handlers"
	"arbitrage/internal/api/middleware"

	"github.com/gorilla/mux"
)

// SetupRoutes настраивает все HTTP маршруты приложения
//
// Назначение:
// Центральное место для определения всех API endpoints.
// Регистрирует handlers для каждого маршрута.
// Применяет middleware к группам маршрутов.
// Организует версионирование API (v1).
//
// Структура маршрутов:
//
// /api/v1/
//   ├── /exchanges/
//   │   ├── GET / - список бирж
//   │   ├── POST /{name}/connect - подключить биржу
//   │   ├── DELETE /{name}/connect - отключить биржу
//   │   └── GET /{name}/balance - получить баланс
//   ├── /pairs/
//   │   ├── GET / - список пар
//   │   ├── POST / - создать пару
//   │   ├── GET /{id} - получить пару
//   │   ├── PATCH /{id} - обновить пару
//   │   ├── DELETE /{id} - удалить пару
//   │   ├── POST /{id}/start - запустить пару
//   │   └── POST /{id}/pause - приостановить пару
//   ├── /notifications/
//   │   ├── GET / - получить уведомления
//   │   └── DELETE / - очистить журнал
//   ├── /stats/
//   │   ├── GET / - получить статистику
//   │   ├── GET /top-pairs - топ-5 пар
//   │   └── POST /reset - сбросить счетчики
//   ├── /blacklist/
//   │   ├── GET / - получить черный список
//   │   ├── POST / - добавить в черный список
//   │   └── DELETE /{symbol} - удалить из черного списка
//   └── /settings/
//       ├── GET / - получить настройки
//       └── PATCH / - обновить настройки
//
// /ws/
//   └── /stream - WebSocket для real-time обновлений
//
// Middleware применяется в следующем порядке:
// 1. Recovery (для всех маршрутов)
// 2. Logging (для всех маршрутов)
// 3. CORS (для всех маршрутов)
// 4. Auth (только для защищенных маршрутов)
func SetupRoutes() *mux.Router {
	router := mux.NewRouter()

	// Глобальные middleware (применяются ко всем маршрутам)
	router.Use(middleware.Recovery)
	router.Use(middleware.Logging)
	router.Use(middleware.CORS)

	// Создание handlers
	exchangeHandler := handlers.NewExchangeHandler()
	pairHandler := handlers.NewPairHandler()
	notificationHandler := handlers.NewNotificationHandler()
	statsHandler := handlers.NewStatsHandler()
	blacklistHandler := handlers.NewBlacklistHandler()
	settingsHandler := handlers.NewSettingsHandler()

	// API v1 routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Применяем auth middleware ко всему API
	// TODO: раскомментировать когда auth будет реализован
	// api.Use(middleware.Auth)

	// Exchange routes
	api.HandleFunc("/exchanges", exchangeHandler.GetExchanges).Methods("GET")
	api.HandleFunc("/exchanges/{name}/connect", exchangeHandler.ConnectExchange).Methods("POST")
	api.HandleFunc("/exchanges/{name}/connect", exchangeHandler.DisconnectExchange).Methods("DELETE")
	api.HandleFunc("/exchanges/{name}/balance", exchangeHandler.GetExchangeBalance).Methods("GET")

	// Pair routes
	api.HandleFunc("/pairs", pairHandler.GetPairs).Methods("GET")
	api.HandleFunc("/pairs", pairHandler.CreatePair).Methods("POST")
	api.HandleFunc("/pairs/{id}", pairHandler.GetPair).Methods("GET")
	api.HandleFunc("/pairs/{id}", pairHandler.UpdatePair).Methods("PATCH")
	api.HandleFunc("/pairs/{id}", pairHandler.DeletePair).Methods("DELETE")
	api.HandleFunc("/pairs/{id}/start", pairHandler.StartPair).Methods("POST")
	api.HandleFunc("/pairs/{id}/pause", pairHandler.PausePair).Methods("POST")

	// Notification routes
	api.HandleFunc("/notifications", notificationHandler.GetNotifications).Methods("GET")
	api.HandleFunc("/notifications", notificationHandler.ClearNotifications).Methods("DELETE")

	// Stats routes
	api.HandleFunc("/stats", statsHandler.GetStats).Methods("GET")
	api.HandleFunc("/stats/top-pairs", statsHandler.GetTopPairs).Methods("GET")
	api.HandleFunc("/stats/reset", statsHandler.ResetStats).Methods("POST")

	// Blacklist routes
	api.HandleFunc("/blacklist", blacklistHandler.GetBlacklist).Methods("GET")
	api.HandleFunc("/blacklist", blacklistHandler.AddToBlacklist).Methods("POST")
	api.HandleFunc("/blacklist/{symbol}", blacklistHandler.RemoveFromBlacklist).Methods("DELETE")

	// Settings routes
	api.HandleFunc("/settings", settingsHandler.GetSettings).Methods("GET")
	api.HandleFunc("/settings", settingsHandler.UpdateSettings).Methods("PATCH")

	// WebSocket route
	// TODO: добавить WebSocket handler когда будет реализован
	// router.HandleFunc("/ws/stream", wsHandler.ServeWS)

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	return router
}
