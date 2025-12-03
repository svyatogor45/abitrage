package middleware

import (
	"net/http"
	"os"
	"strings"
)

// allowedOrigins содержит список разрешенных доменов для CORS.
// В production загружается из переменной окружения CORS_ALLOWED_ORIGINS.
var allowedOrigins = map[string]bool{
	"http://localhost:3000":   true,
	"http://127.0.0.1:3000":   true,
	"http://localhost:8080":   true,
	"http://127.0.0.1:8080":   true,
	"http://localhost:5173":   true, // Vite dev server
	"http://127.0.0.1:5173":   true,
}

func init() {
	// Загружаем дополнительные origins из переменной окружения
	if origins := os.Getenv("CORS_ALLOWED_ORIGINS"); origins != "" {
		for _, origin := range strings.Split(origins, ",") {
			origin = strings.TrimSpace(origin)
			if origin != "" {
				allowedOrigins[origin] = true
			}
		}
	}
}

// isOriginAllowed проверяет, разрешен ли origin
func isOriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	return allowedOrigins[origin]
}

// CORS - middleware для настройки Cross-Origin Resource Sharing
//
// Назначение:
// Настраивает CORS заголовки для безопасного взаимодействия frontend с backend API.
// Позволяет браузерным приложениям (React frontend) делать запросы к API на другом домене.
//
// Функции:
// - Установка Access-Control-Allow-Origin для разрешенных доменов
// - Обработка preflight запросов (OPTIONS)
// - Разрешение определенных HTTP методов (GET, POST, PUT, DELETE, PATCH)
// - Разрешение определенных заголовков (Content-Type, Authorization)
// - Поддержка credentials (cookies, authorization headers)
// - Установка времени жизни preflight кеша (24 часа)
//
// Конфигурация:
// - Разрешенные origins загружаются из CORS_ALLOWED_ORIGINS (через запятую)
// - По умолчанию разрешены localhost:3000, localhost:8080, localhost:5173
//
// Важные заголовки:
// - Access-Control-Allow-Origin: конкретный домен (не * при credentials)
// - Access-Control-Allow-Methods: GET, POST, PUT, DELETE, PATCH, OPTIONS
// - Access-Control-Allow-Headers: Content-Type, Authorization
// - Access-Control-Allow-Credentials: true
// - Access-Control-Max-Age: 86400 (24 часа)
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		origin := r.Header.Get("Origin")

		// Проверяем, разрешен ли origin
		if isOriginAllowed(origin) {
			// Для разрешенных origins с credentials устанавливаем конкретный origin
			w.Header().Set("Access-Control-Allow-Origin", origin)
			w.Header().Set("Access-Control-Allow-Credentials", "true")
		} else if origin == "" {
			// Запросы без Origin (не из браузера, например curl) - разрешаем
			w.Header().Set("Access-Control-Allow-Origin", "*")
		}
		// Для неразрешенных origins не устанавливаем заголовки - браузер заблокирует

		// Общие заголовки для всех ответов
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Requested-With")
		w.Header().Set("Access-Control-Max-Age", "86400") // 24 часа кеширования preflight

		// Обработка preflight запросов
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
