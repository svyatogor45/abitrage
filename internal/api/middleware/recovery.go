package middleware

import (
	"log"
	"net/http"
	"runtime/debug"
)

// Recovery - middleware для восстановления после паники в handlers
//
// Назначение:
// Перехватывает panic в HTTP handlers и предотвращает падение всего сервера.
// Логирует информацию об ошибке и stack trace для отладки.
// Возвращает клиенту корректный HTTP ответ 500 Internal Server Error.
//
// Функции:
// - Перехват panic в любом handler
// - Логирование сообщения об ошибке (только в логи сервера)
// - Логирование полного stack trace (только в логи сервера)
// - Возврат 500 Internal Server Error клиенту (без деталей для безопасности)
// - Предотвращение падения сервера
// - Продолжение обработки последующих запросов
//
// Важность:
// Критически важен для стабильности сервера в production.
// Даже если в коде есть необработанная ошибка, сервер продолжит работу.
// Помогает обнаружить и исправить баги через логи.
//
// Безопасность:
// Клиенту возвращается только общее сообщение "Internal Server Error"
// без раскрытия внутренних деталей ошибки (предотвращение утечки информации).
//
// Stack trace содержит:
// - Последовательность вызовов функций до panic
// - Номера строк кода
// - Имена файлов
// - Полезно для отладки (только в серверных логах)
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// Логирование panic (только на сервере, не отправляется клиенту)
				log.Printf("PANIC recovered: %v", err)
				log.Printf("Request: %s %s from %s", r.Method, r.URL.Path, r.RemoteAddr)
				log.Printf("Stack trace:\n%s", debug.Stack())

				// Возврат ошибки клиенту БЕЗ раскрытия деталей (для безопасности)
				// Детали ошибки остаются только в логах сервера
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
