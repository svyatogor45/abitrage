package middleware

import (
	"net/http"
)

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
// - Установка времени жизни preflight кеша
//
// Конфигурация:
// - Development: разрешить все источники (*)
// - Production: разрешить только домен frontend приложения
//
// Важные заголовки:
// - Access-Control-Allow-Origin: домен frontend
// - Access-Control-Allow-Methods: GET, POST, PUT, DELETE, PATCH, OPTIONS
// - Access-Control-Allow-Headers: Content-Type, Authorization
// - Access-Control-Allow-Credentials: true
func CORS(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO:
		// 1. Получить origin из заголовка запроса
		// 2. Проверить origin в списке разрешенных
		// 3. Установить CORS заголовки:
		//    - Access-Control-Allow-Origin
		//    - Access-Control-Allow-Methods
		//    - Access-Control-Allow-Headers
		//    - Access-Control-Allow-Credentials
		//    - Access-Control-Max-Age
		// 4. Если это preflight запрос (OPTIONS):
		//    - Вернуть 200 OK без дальнейшей обработки
		// 5. Иначе передать управление следующему handler

		// Временная реализация: разрешить все (для разработки)
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, PATCH, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.Header().Set("Access-Control-Allow-Credentials", "true")

		// Обработка preflight запросов
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}
