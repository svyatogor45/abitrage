# Multi-stage build для оптимизации размера образа

# Этап 1: Сборка
FROM golang:1.21-alpine AS builder

# Установка необходимых инструментов
RUN apk add --no-cache git

WORKDIR /app

# Копирование go.mod и go.sum для кэширования зависимостей
COPY go.mod go.sum ./
RUN go mod download

# Копирование исходного кода
COPY . .

# Сборка приложения
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o arbitrage ./cmd/server

# Этап 2: Финальный образ
FROM alpine:latest

# Установка сертификатов для HTTPS и timezone data
RUN apk --no-cache add ca-certificates tzdata

WORKDIR /root/

# Копирование скомпилированного бинарника из builder
COPY --from=builder /app/arbitrage .

# Копирование миграций
COPY --from=builder /app/migrations ./migrations

# Открытие порта
EXPOSE 8080

# Запуск приложения
CMD ["./arbitrage"]
