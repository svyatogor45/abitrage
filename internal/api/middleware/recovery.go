package middleware

import (
	"fmt"
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
// - Логирование сообщения об ошибке
// - Логирование полного stack trace
// - Возврат 500 Internal Server Error клиенту
// - Предотвращение падения сервера
// - Продолжение обработки последующих запросов
//
// Важность:
// Критически важен для стабильности сервера в production.
// Даже если в коде есть необработанная ошибка, сервер продолжит работу.
// Помогает обнаружить и исправить баги через логи.
//
// Stack trace содержит:
// - Последовательность вызовов функций до panic
// - Номера строк кода
// - Имена файлов
// - Полезно для отладки
func Recovery(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				// TODO:
				// 1. Захватить panic
				// 2. Получить stack trace через debug.Stack()
				// 3. Залогировать ошибку и stack trace
				// 4. Вернуть 500 Internal Server Error клиенту
				// 5. Опционально: отправить уведомление (email, Slack, Sentry)

				// Логирование panic
				log.Printf("PANIC: %v\n", err)
				log.Printf("Stack trace:\n%s", debug.Stack())

				// Возврат ошибки клиенту
				http.Error(
					w,
					fmt.Sprintf("Internal Server Error: %v", err),
					http.StatusInternalServerError,
				)
			}
		}()

		next.ServeHTTP(w, r)
	})
}
