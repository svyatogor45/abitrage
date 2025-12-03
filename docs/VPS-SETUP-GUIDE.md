# Пошаговая инструкция: Запуск проекта на VPS

Инструкция для новичков. Выполняйте команды по одной.

---

## Шаг 1: Подключитесь к VPS

На вашем компьютере откройте терминал (или PowerShell на Windows) и выполните:

```bash
ssh root@ВАШ_IP_АДРЕС
```

Введите пароль от VPS, когда попросят.

---

## Шаг 2: Обновите систему

После подключения выполните:

```bash
apt update && apt upgrade -y
```

Это обновит список пакетов и установит обновления.

---

## Шаг 3: Установите Git

```bash
apt install git -y
```

Проверьте установку:

```bash
git --version
```

Должно показать что-то вроде: `git version 2.x.x`

---

## Шаг 4: Установите Docker

Выполните эти команды по очереди:

```bash
# Установка необходимых пакетов
apt install ca-certificates curl gnupg -y

# Добавление ключа Docker
install -m 0755 -d /etc/apt/keyrings
curl -fsSL https://download.docker.com/linux/ubuntu/gpg | gpg --dearmor -o /etc/apt/keyrings/docker.gpg
chmod a+r /etc/apt/keyrings/docker.gpg

# Добавление репозитория Docker
echo \
  "deb [arch=$(dpkg --print-architecture) signed-by=/etc/apt/keyrings/docker.gpg] https://download.docker.com/linux/ubuntu \
  $(. /etc/os-release && echo "$VERSION_CODENAME") stable" | \
  tee /etc/apt/sources.list.d/docker.list > /dev/null

# Обновление списка пакетов
apt update

# Установка Docker
apt install docker-ce docker-ce-cli containerd.io docker-buildx-plugin docker-compose-plugin -y
```

Проверьте, что Docker установлен:

```bash
docker --version
```

Должно показать: `Docker version 2x.x.x`

---

## Шаг 5: Склонируйте репозиторий

```bash
cd /root
git clone https://github.com/svyatogor45/abitrage.git
```

Перейдите в папку проекта:

```bash
cd abitrage
```

---

## Шаг 6: Создайте файл конфигурации

Скопируйте пример конфигурации:

```bash
cp .env.example .env
```

---

## Шаг 7: Настройте безопасность (ВАЖНО!)

Откройте файл для редактирования:

```bash
nano .env
```

Измените следующие строки:

1. **JWT_SECRET** - замените на длинную случайную строку (минимум 32 символа)
2. **ENCRYPTION_KEY** - должен быть РОВНО 32 символа
3. **DB_PASSWORD** - придумайте сложный пароль

Пример генерации случайного ключа:

```bash
# Для JWT_SECRET (запустите в другом терминале):
openssl rand -hex 32

# Для ENCRYPTION_KEY (32 символа):
openssl rand -hex 16
```

После редактирования:
- Нажмите `Ctrl + O` (сохранить)
- Нажмите `Enter` (подтвердить)
- Нажмите `Ctrl + X` (выйти)

---

## Шаг 8: Запустите проект

```bash
docker compose up -d
```

Первый запуск займёт 3-10 минут (скачивание образов и сборка).

---

## Шаг 9: Проверьте, что всё работает

Посмотрите статус контейнеров:

```bash
docker compose ps
```

Все контейнеры должны быть в статусе `Up` или `healthy`.

Ожидаемый вывод:
```
NAME                    STATUS
arbitrage-db            Up (healthy)
arbitrage-app           Up
arbitrage-frontend      Up
arbitrage-prometheus    Up
arbitrage-alertmanager  Up
arbitrage-grafana       Up
```

---

## Шаг 10: Откройте в браузере

На вашем компьютере откройте браузер и перейдите:

| Сервис | URL |
|--------|-----|
| **Frontend (основной)** | `http://ВАШ_IP:3000` |
| **Backend API** | `http://ВАШ_IP:8080` |
| **Grafana (мониторинг)** | `http://ВАШ_IP:3001` (логин: admin/admin) |
| **Prometheus** | `http://ВАШ_IP:9090` |

---

## Полезные команды

### Просмотр логов приложения:
```bash
docker compose logs -f app
```

### Просмотр логов всех сервисов:
```bash
docker compose logs -f
```

### Остановка проекта:
```bash
docker compose down
```

### Перезапуск проекта:
```bash
docker compose restart
```

### Полная переустановка (с удалением данных):
```bash
docker compose down -v
docker compose up -d
```

### Обновление проекта:
```bash
git pull
docker compose down
docker compose up -d --build
```

---

## Возможные проблемы

### Ошибка "port already in use"

Проверьте, что занимает порт:
```bash
lsof -i :8080
```

Убейте процесс или измените порт в `docker-compose.yml`.

### Контейнер app не запускается

Посмотрите логи:
```bash
docker compose logs app
```

### Ошибка с базой данных

Проверьте, что БД запустилась:
```bash
docker compose logs db
```

### Недостаточно памяти

Проверьте свободную память:
```bash
free -h
```

Рекомендуется минимум 2 GB RAM.

---

## Настройка firewall (для безопасности)

Установите ufw и откройте только нужные порты:

```bash
apt install ufw -y
ufw allow 22        # SSH
ufw allow 3000      # Frontend
ufw allow 8080      # Backend API
ufw enable
```

---

## Требования к VPS

- **ОС:** Ubuntu 22.04 или 24.04
- **RAM:** минимум 2 GB (рекомендуется 4 GB)
- **Диск:** минимум 20 GB
- **CPU:** 1-2 ядра

---

Готово! Ваш арбитражный терминал должен работать.
