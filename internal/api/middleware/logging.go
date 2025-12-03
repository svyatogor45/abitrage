package middleware

import (
	"log"
	"net/http"
	"time"
)

// Logging - middleware для логирования HTTP запросов
//
// Назначение:
// Логирует все входящие HTTP запросы для мониторинга и отладки.
// Записывает важную информацию о каждом запросе в структурированном формате.
//
// Функции:
// - Логирование метода HTTP (GET, POST, PUT, DELETE, etc.)
// - Логирование пути запроса (URL path)
// - Логирование IP адреса клиента
// - Измерение времени обработки запроса (latency)
// - Логирование статус кода ответа
// - Логирование размера ответа (в байтах)
// - Структурированное логирование в JSON формате (для production)
//
// Формат лога:
// [timestamp] METHOD /path - status_code - duration - client_ip
// Пример: [2025-12-01 12:00:00] GET /api/pairs - 200 - 45ms - 192.168.1.1
type responseWriter struct {
	http.ResponseWriter
	statusCode int
	written    int64
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Write(b []byte) (int, error) {
	n, err := rw.ResponseWriter.Write(b)
	rw.written += int64(n)
	return n, err
}

func Logging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Wrap ResponseWriter чтобы захватить status code
		wrapped := &responseWriter{
			ResponseWriter: w,
			statusCode:     http.StatusOK,
		}

		// TODO:
		// 1. Захватить начальное время
		// 2. Вызвать следующий handler
		// 3. После обработки:
		//    - Вычислить duration
		//    - Получить status code
		//    - Извлечь IP адрес из r.RemoteAddr
		//    - Залогировать в структурированном формате

		next.ServeHTTP(wrapped, r)

		duration := time.Since(start)

		// Формат лога: METHOD /path - status - duration - client_ip - response_size
		log.Printf(
			"%s %s - %d - %v - %s - %d bytes",
			r.Method,
			r.URL.Path,
			wrapped.statusCode,
			duration,
			r.RemoteAddr,
			wrapped.written,
		)
	})
}
