# Arbitrage Trading Terminal

Автоматизированный арбитражный торговый терминал для торговли фьючерсами между криптовалютными биржами.

## Описание

Система предназначена для хеджированной торговли фьючерсами между несколькими криптовалютными биржами в режиме реального времени. Терминал автоматически отслеживает ценовые расхождения (спреды) по выбранным торговым парам на разных биржах и совершает одновременные сделки (лонг/шорт) для извлечения прибыли из разницы цен.

## Поддерживаемые биржи

- Bybit
- Bitget
- OKX
- Gate.io
- HTX
- BingX

## Технологический стек

**Backend:**
- Go 1.21+
- PostgreSQL 15
- WebSocket для real-time обновлений
- Gorilla Mux для HTTP роутинга

**Frontend:**
- React 18+
- WebSocket для real-time данных

**Инфраструктура:**
- Docker & Docker Compose
- PostgreSQL для хранения данных

## Быстрый старт

### Предварительные требования

- Docker и Docker Compose установлены
- Порты 8080 (backend), 3000 (frontend), 5432 (postgres) свободны

### Установка и запуск

1. **Клонируйте репозиторий:**
```bash
git clone <repository-url>
cd abitrage
```

2. **Создайте файл .env на основе примера:**
```bash
cp .env.example .env
```

3. **ВАЖНО: Отредактируйте .env и измените:**
   - `JWT_SECRET` - длинный случайный ключ
   - `ENCRYPTION_KEY` - ровно 32 символа для AES-256 шифрования
   - Пароли базы данных

4. **Запустите все сервисы:**
```bash
docker-compose up -d
```

5. **Проверьте статус:**
```bash
docker-compose ps
```

6. **Просмотр логов:**
```bash
docker-compose logs -f app
```

## Разработка

### Запуск без Docker (для разработки)

1. **Установите зависимости:**
```bash
go mod download
```

2. **Запустите PostgreSQL:**
```bash
docker-compose up -d db
```

3. **Примените миграции:**
```bash
# TODO: Добавить команду для миграций
```

4. **Запустите сервер:**
```bash
go run cmd/server/main.go
```

### Структура проекта

Подробное описание архитектуры см. в [Архитектура.md](./Архитектура.md)

```
arbitrage/
├── cmd/server/          # Точка входа приложения
├── internal/            # Внутренняя логика
│   ├── api/            # REST API handlers
│   ├── bot/            # Логика арбитражного бота
│   ├── exchange/       # Интеграция с биржами
│   ├── models/         # Модели данных
│   ├── repository/     # Работа с БД
│   ├── service/        # Бизнес-логика
│   └── config/         # Конфигурация
├── pkg/                # Переиспользуемые библиотеки
├── migrations/         # SQL миграции
├── frontend/           # React приложение
└── docker-compose.yml  # Docker конфигурация
```

## Документация

- [Техническое задание](./Тз.md) - Полное описание требований
- [Архитектура](./Архитектура.md) - Детальное описание архитектуры системы

## API Endpoints

### Exchanges (Биржи)
- `POST /api/exchanges/{name}/connect` - Подключить биржу
- `DELETE /api/exchanges/{name}/connect` - Отключить биржу
- `GET /api/exchanges` - Получить список бирж и их статусы

### Pairs (Торговые пары)
- `POST /api/pairs` - Добавить торговую пару
- `GET /api/pairs` - Получить список пар
- `PATCH /api/pairs/{id}` - Редактировать пару
- `DELETE /api/pairs/{id}` - Удалить пару
- `POST /api/pairs/{id}/start` - Запустить мониторинг пары
- `POST /api/pairs/{id}/pause` - Приостановить пару

### Notifications (Уведомления)
- `GET /api/notifications` - Получить уведомления
- `DELETE /api/notifications` - Очистить журнал

### Stats (Статистика)
- `GET /api/stats` - Получить статистику

### WebSocket
- `WS /ws/stream` - Real-time обновления

## Безопасность

⚠️ **ВАЖНО для Production:**

1. **Измените все секретные ключи** в .env файле
2. **Используйте HTTPS** (установите `USE_HTTPS=true` и укажите сертификаты)
3. **Настройте firewall** - закройте порты, открытые только для локальной сети
4. **Используйте сложные пароли** для PostgreSQL
5. **Регулярно обновляйте** зависимости

## Мониторинг

Логи приложения доступны:
```bash
docker-compose logs -f app
```

База данных:
```bash
docker-compose exec db psql -U arbitrage_user -d arbitrage
```

## Остановка

Остановить все сервисы:
```bash
docker-compose down
```

Остановить и удалить данные:
```bash
docker-compose down -v
```

## Разработка и вклад

TODO: Добавить правила contributing

## Лицензия

TODO: Добавить лицензию

## Контакты

TODO: Добавить контакты
