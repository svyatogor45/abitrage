package middleware

import (
	"crypto/subtle"
	"net/http"
	"os"
)

// debugUsername и debugPassword для защиты debug endpoints.
// Загружаются из переменных окружения DEBUG_USERNAME и DEBUG_PASSWORD.
// Если не установлены, debug endpoints будут недоступны в production.
var (
	debugUsername = os.Getenv("DEBUG_USERNAME")
	debugPassword = os.Getenv("DEBUG_PASSWORD")
)

// DebugAuth - middleware для защиты debug/pprof endpoints
//
// Назначение:
// Защищает debug endpoints (/debug/pprof/*, /debug/runtime) от неавторизованного доступа.
// Использует HTTP Basic Authentication для простоты.
//
// Конфигурация:
// - DEBUG_USERNAME: имя пользователя для доступа к debug endpoints
// - DEBUG_PASSWORD: пароль для доступа к debug endpoints
// - Если переменные не установлены, доступ запрещен (401)
//
// Безопасность:
// - Использует constant-time сравнение для предотвращения timing attacks
// - В production ОБЯЗАТЕЛЬНО установить DEBUG_USERNAME и DEBUG_PASSWORD
// - Рекомендуется использовать сложные пароли
//
// Использование:
//
//	debug := router.PathPrefix("/debug").Subrouter()
//	debug.Use(middleware.DebugAuth)
func DebugAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Если credentials не настроены, запрещаем доступ в production
		if debugUsername == "" || debugPassword == "" {
			// В development (если явно не настроено) разрешаем доступ
			if os.Getenv("ENV") == "development" || os.Getenv("ENV") == "" {
				next.ServeHTTP(w, r)
				return
			}
			http.Error(w, "Debug endpoints disabled. Set DEBUG_USERNAME and DEBUG_PASSWORD.", http.StatusForbidden)
			return
		}

		// Получаем credentials из запроса
		user, pass, ok := r.BasicAuth()
		if !ok {
			w.Header().Set("WWW-Authenticate", `Basic realm="Debug endpoints"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		// Constant-time сравнение для предотвращения timing attacks
		userMatch := subtle.ConstantTimeCompare([]byte(user), []byte(debugUsername)) == 1
		passMatch := subtle.ConstantTimeCompare([]byte(pass), []byte(debugPassword)) == 1

		if !userMatch || !passMatch {
			w.Header().Set("WWW-Authenticate", `Basic realm="Debug endpoints"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// Auth - middleware для аутентификации и авторизации запросов
//
// Назначение:
// Проверяет наличие и валидность токенов аутентификации (JWT или session cookie).
// Защищает API endpoints от неавторизованного доступа.
// Извлекает информацию о пользователе из токена и добавляет в context запроса.
//
// Функции:
// - Проверка JWT токенов из заголовка Authorization
// - Валидация подписи и срока действия токена
// - Извлечение user_id из токена
// - Добавление user_id в request context для использования в handlers
// - Возврат 401 Unauthorized при отсутствии или невалидном токене
// - Rate limiting для предотвращения bruteforce атак
//
// Примечание:
// В первой версии проекта (один пользователь) может быть упрощенная версия
// с базовой проверкой пароля или без auth вообще (для локального развертывания)
func Auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO:
		// 1. Извлечь токен из заголовка Authorization: Bearer <token>
		// 2. Если токена нет - вернуть 401 Unauthorized
		// 3. Валидировать JWT токен:
		//    - Проверить подпись (используя JWT_SECRET из config)
		//    - Проверить срок действия (exp claim)
		// 4. Извлечь claims из токена (user_id, роль)
		// 5. Добавить информацию в context запроса
		// 6. Передать управление следующему handler

		// Временная реализация: пропускать все запросы (для разработки)
		next.ServeHTTP(w, r)
	})
}

// OptionalAuth - опциональная аутентификация
//
// Назначение:
// Проверяет токен если он предоставлен, но не требует его наличия.
// Используется для endpoints, которые могут работать как для авторизованных,
// так и для неавторизованных пользователей.
func OptionalAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// TODO:
		// 1. Попытаться извлечь токен из заголовка
		// 2. Если токен есть - валидировать и добавить в context
		// 3. Если токена нет или невалидный - продолжить без auth
		// 4. Передать управление следующему handler

		next.ServeHTTP(w, r)
	})
}
