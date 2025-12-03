# Multi-stage build для оптимизации размера образа
#
# Сборка:
#   docker build -t arbitrage:latest .
#
# Запуск:
#   docker run -p 8080:8080 --env-file .env arbitrage:latest

# =============================================================================
# Этап 1: Сборка
# =============================================================================
FROM golang:1.21-alpine AS builder

LABEL stage="builder"

# Установка необходимых инструментов
RUN apk add --no-cache git

WORKDIR /app

# Копирование go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка приложения с оптимизациями
# -ldflags="-s -w" уменьшает размер бинарника (strip debug info)
RUN CGO_ENABLED=0 GOOS=linux go build \
    -a -installsuffix cgo \
    -ldflags="-s -w" \
    -o arbitrage ./cmd/server

# =============================================================================
# Этап 2: Финальный образ
# =============================================================================
FROM alpine:3.19

# Метаданные образа
LABEL maintainer="Arbitrage Bot Team"
LABEL version="1.0.0"
LABEL description="Arbitrage trading bot - backend service"
LABEL org.opencontainers.image.source="https://github.com/svyatogor45/abitrage"

# Установка сертификатов для HTTPS и timezone data
RUN apk --no-cache add ca-certificates tzdata wget

# Создание непривилегированного пользователя для безопасности
# Это предотвращает эскалацию привилегий при компрометации контейнера
RUN addgroup -g 1000 appgroup && \
    adduser -D -u 1000 -G appgroup -h /app appuser

WORKDIR /app

# Копирование скомпилированного бинарника с правильными правами
COPY --from=builder --chown=appuser:appgroup /app/arbitrage .

# Копирование миграций
COPY --from=builder --chown=appuser:appgroup /app/migrations ./migrations

# Создание директории для логов
RUN mkdir -p /app/logs && chown appuser:appgroup /app/logs

# Переключение на непривилегированного пользователя
USER appuser

# Открытие порта
EXPOSE 8080

# Healthcheck для проверки готовности приложения
HEALTHCHECK --interval=30s --timeout=10s --start-period=10s --retries=3 \
    CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Запуск приложения
CMD ["./arbitrage"]
