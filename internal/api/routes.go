package api

import (
	"net/http"
	"net/http/pprof"
	"runtime"

	"arbitrage/internal/api/handlers"
	"arbitrage/internal/api/middleware"
	"arbitrage/internal/service"
	"arbitrage/internal/websocket"

	"github.com/gorilla/mux"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// Dependencies содержит все зависимости для API handlers
type Dependencies struct {
	ExchangeService     *service.ExchangeService
	PairService         *service.PairService
	StatsService        *service.StatsService
	SettingsService     *service.SettingsService
	NotificationService *service.NotificationService
	BlacklistService    *service.BlacklistService
	Hub                 *websocket.Hub
}

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
//
//	├── /exchanges/
//	│   ├── GET / - список бирж
//	│   ├── POST /{name}/connect - подключить биржу
//	│   ├── DELETE /{name}/connect - отключить биржу
//	│   └── GET /{name}/balance - получить баланс
//	├── /pairs/
//	│   ├── GET / - список пар
//	│   ├── POST / - создать пару
//	│   ├── GET /{id} - получить пару
//	│   ├── PATCH /{id} - обновить пару
//	│   ├── DELETE /{id} - удалить пару
//	│   ├── POST /{id}/start - запустить пару
//	│   └── POST /{id}/pause - приостановить пару
//	├── /notifications/
//	│   ├── GET / - получить уведомления
//	│   └── DELETE / - очистить журнал
//	├── /stats/
//	│   ├── GET / - получить статистику
//	│   ├── GET /top-pairs - топ-5 пар
//	│   └── POST /reset - сбросить счетчики
//	├── /blacklist/
//	│   ├── GET / - получить черный список
//	│   ├── POST / - добавить в черный список
//	│   └── DELETE /{symbol} - удалить из черного списка
//	└── /settings/
//	    ├── GET / - получить настройки
//	    └── PATCH / - обновить настройки
//
// /ws/
//
//	└── /stream - WebSocket для real-time обновлений
//
// Middleware применяется в следующем порядке:
// 1. Recovery (для всех маршрутов)
// 2. Logging (для всех маршрутов)
// 3. CORS (для всех маршрутов)
// 4. Auth (только для защищенных маршрутов)
func SetupRoutes(deps *Dependencies) *mux.Router {
	router := mux.NewRouter()

	// Глобальные middleware (применяются ко всем маршрутам)
	router.Use(middleware.Recovery)
	router.Use(middleware.Logging)
	router.Use(middleware.CORS)

	// Создание handlers с внедрением зависимостей
	var exchangeHandler *handlers.ExchangeHandler
	if deps != nil && deps.ExchangeService != nil {
		exchangeHandler = handlers.NewExchangeHandler(deps.ExchangeService)
	}

	var pairHandler *handlers.PairHandler
	if deps != nil && deps.PairService != nil {
		pairHandler = handlers.NewPairHandler(deps.PairService)
	}

	// Stats handler с внедрением зависимости
	var statsHandler *handlers.StatsHandler
	if deps != nil && deps.StatsService != nil {
		statsHandler = handlers.NewStatsHandler(deps.StatsService)
	}

	// Settings handler с внедрением зависимости
	var settingsHandler *handlers.SettingsHandler
	if deps != nil && deps.SettingsService != nil {
		settingsHandler = handlers.NewSettingsHandler(deps.SettingsService)
	}

	// Notification handler с внедрением зависимости
	var notificationHandler *handlers.NotificationHandler
	if deps != nil && deps.NotificationService != nil {
		notificationHandler = handlers.NewNotificationHandler(deps.NotificationService)
	}

	// Blacklist handler с внедрением зависимости
	var blacklistHandler *handlers.BlacklistHandler
	if deps != nil && deps.BlacklistService != nil {
		blacklistHandler = handlers.NewBlacklistHandler(deps.BlacklistService)
	}

	// API v1 routes
	api := router.PathPrefix("/api/v1").Subrouter()

	// Применяем auth middleware ко всему API
	// TODO: раскомментировать когда auth будет реализован
	// api.Use(middleware.Auth)

	// Exchange routes
	if exchangeHandler != nil {
		api.HandleFunc("/exchanges", exchangeHandler.GetExchanges).Methods("GET")
		api.HandleFunc("/exchanges/{name}/connect", exchangeHandler.ConnectExchange).Methods("POST")
		api.HandleFunc("/exchanges/{name}/connect", exchangeHandler.DisconnectExchange).Methods("DELETE")
		api.HandleFunc("/exchanges/{name}/balance", exchangeHandler.GetExchangeBalance).Methods("GET")
	}

	// Pair routes
	if pairHandler != nil {
		api.HandleFunc("/pairs", pairHandler.GetPairs).Methods("GET")
		api.HandleFunc("/pairs", pairHandler.CreatePair).Methods("POST")
		api.HandleFunc("/pairs/{id}", pairHandler.GetPair).Methods("GET")
		api.HandleFunc("/pairs/{id}", pairHandler.UpdatePair).Methods("PATCH")
		api.HandleFunc("/pairs/{id}", pairHandler.DeletePair).Methods("DELETE")
		api.HandleFunc("/pairs/{id}/start", pairHandler.StartPair).Methods("POST")
		api.HandleFunc("/pairs/{id}/pause", pairHandler.PausePair).Methods("POST")
	}

	// Notification routes
	if notificationHandler != nil {
		api.HandleFunc("/notifications", notificationHandler.GetNotifications).Methods("GET")
		api.HandleFunc("/notifications", notificationHandler.ClearNotifications).Methods("DELETE")
	}

	// Stats routes
	if statsHandler != nil {
		api.HandleFunc("/stats", statsHandler.GetStats).Methods("GET")
		api.HandleFunc("/stats/top-pairs", statsHandler.GetTopPairs).Methods("GET")
		api.HandleFunc("/stats/reset", statsHandler.ResetStats).Methods("POST")
	}

	// Blacklist routes
	if blacklistHandler != nil {
		api.HandleFunc("/blacklist", blacklistHandler.GetBlacklist).Methods("GET")
		api.HandleFunc("/blacklist", blacklistHandler.AddToBlacklist).Methods("POST")
		api.HandleFunc("/blacklist/{symbol}", blacklistHandler.RemoveFromBlacklist).Methods("DELETE")
	}

	// Settings routes
	if settingsHandler != nil {
		api.HandleFunc("/settings", settingsHandler.GetSettings).Methods("GET")
		api.HandleFunc("/settings", settingsHandler.UpdateSettings).Methods("PATCH")
	}

	// WebSocket route для real-time обновлений
	if deps != nil && deps.Hub != nil {
		router.HandleFunc("/ws/stream", func(w http.ResponseWriter, r *http.Request) {
			websocket.ServeWS(deps.Hub, w, r)
		}).Methods("GET")
	}

	// Health check endpoint
	router.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	// ============================================================
	// Prometheus metrics endpoint
	// ============================================================
	// GET /metrics - экспорт метрик для Prometheus
	// Используется для мониторинга производительности торгового ядра
	router.Handle("/metrics", promhttp.Handler()).Methods("GET")

	// ============================================================
	// pprof endpoints для профилирования
	// ============================================================
	// ВАЖНО: В production должны быть защищены авторизацией!
	// Используются для анализа производительности и отладки:
	// - /debug/pprof/         - индекс всех профилей
	// - /debug/pprof/profile  - CPU профиль (30 сек по умолчанию)
	// - /debug/pprof/heap     - профиль памяти
	// - /debug/pprof/goroutine - список горутин
	// - /debug/pprof/trace    - execution trace
	//
	// Пример использования:
	// go tool pprof http://localhost:8080/debug/pprof/profile
	// go tool pprof http://localhost:8080/debug/pprof/heap

	debug := router.PathPrefix("/debug/pprof").Subrouter()

	debug.HandleFunc("/", pprof.Index)
	debug.HandleFunc("/cmdline", pprof.Cmdline)
	debug.HandleFunc("/profile", pprof.Profile)
	debug.HandleFunc("/symbol", pprof.Symbol)
	debug.HandleFunc("/trace", pprof.Trace)

	// Handlers для специфичных профилей
	debug.HandleFunc("/heap", func(w http.ResponseWriter, r *http.Request) {
		pprof.Handler("heap").ServeHTTP(w, r)
	})
	debug.HandleFunc("/goroutine", func(w http.ResponseWriter, r *http.Request) {
		pprof.Handler("goroutine").ServeHTTP(w, r)
	})
	debug.HandleFunc("/block", func(w http.ResponseWriter, r *http.Request) {
		pprof.Handler("block").ServeHTTP(w, r)
	})
	debug.HandleFunc("/threadcreate", func(w http.ResponseWriter, r *http.Request) {
		pprof.Handler("threadcreate").ServeHTTP(w, r)
	})
	debug.HandleFunc("/mutex", func(w http.ResponseWriter, r *http.Request) {
		pprof.Handler("mutex").ServeHTTP(w, r)
	})
	debug.HandleFunc("/allocs", func(w http.ResponseWriter, r *http.Request) {
		pprof.Handler("allocs").ServeHTTP(w, r)
	})

	// Runtime stats endpoint (дополнительно)
	router.HandleFunc("/debug/runtime", func(w http.ResponseWriter, r *http.Request) {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{`))
		w.Write([]byte(`"goroutines":` + itoa(runtime.NumGoroutine()) + `,`))
		w.Write([]byte(`"heap_alloc_mb":` + ftoa(float64(m.HeapAlloc)/1024/1024) + `,`))
		w.Write([]byte(`"heap_sys_mb":` + ftoa(float64(m.HeapSys)/1024/1024) + `,`))
		w.Write([]byte(`"num_gc":` + itoa(int(m.NumGC)) + `,`))
		w.Write([]byte(`"gc_pause_total_ms":` + ftoa(float64(m.PauseTotalNs)/1e6)))
		w.Write([]byte(`}`))
	}).Methods("GET")

	return router
}

// Вспомогательные функции для JSON без fmt
func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		b[pos] = '-'
	}
	return string(b[pos:])
}

func ftoa(f float64) string {
	// Простое форматирование с 2 знаками после запятой
	i := int(f * 100)
	whole := i / 100
	frac := i % 100
	if frac < 0 {
		frac = -frac
	}
	fracStr := itoa(frac)
	if len(fracStr) == 1 {
		fracStr = "0" + fracStr
	}
	return itoa(whole) + "." + fracStr
}
