# 🔄 HSM Key Rotation - Docker & Production Compatibility

Скрипт `scripts/check-key-rotation.sh` поддерживает **оба окружения**: Docker и Production (Debian).

## 🔍 Автоматическое обнаружение окружения

Скрипт самостоятельно определяет, в каком окружении он запущен:

```bash
# Docker окружение
if [ -f "/.dockerenv" ] || docker info >/dev/null 2>&1; then
    ENVIRONMENT="docker"
    HSM_ADMIN_CMD="docker exec hsm-service /app/hsm-admin"
    LOG_FILE="/var/log/hsm-rotation-check.log"

# Production окружение
elif systemctl is-active --quiet hsm-service; then
    ENVIRONMENT="production"
    HSM_ADMIN_CMD="/opt/hsm-service/bin/hsm-admin -config /etc/hsm-service/config.yaml"
    LOG_FILE="/var/log/hsm-service/rotation.log"
fi
```

## 📊 Различия конфигурации

| Параметр | Docker | Production |
|----------|--------|-----------|
| **Окружение** | `docker-compose` контейнер | `systemd` сервис |
| **Проверка сервиса** | `docker ps \| grep` | `systemctl is-active` |
| **Команда hsm-admin** | `docker exec hsm-service /app/hsm-admin` | `/opt/hsm-service/bin/hsm-admin -config /etc/hsm-service/config.yaml` |
| **Файл логов** | `/var/log/hsm-rotation-check.log` | `/var/log/hsm-service/rotation.log` |
| **Переменные окружения** | Из `.env` файла Docker | Из `/etc/hsm-service/environment` |
| **Health check** | HTTPS к localhost:8443 (без сертификатов) | HTTPS к localhost:8443 с mTLS (с сертификатами) |

## ✅ Проверенные сценарии

### Docker (docker-compose)

```bash
# Выполнить проверку ротации вручную
./scripts/check-key-rotation.sh

# Вывод:
# HSM Key Rotation Status Check - Thu Jan 23 14:30:45 UTC 2026
# ========================================
# Environment: docker
# ...
```

**Переменные окружения** из `.env`:
```bash
AUTO_ROTATE=false  # Или true для автоматической ротации
SLACK_WEBHOOK=https://hooks.slack.com/...
ALERT_EMAIL=ops@example.com
```

### Production (Debian 13)

**1. Через systemd timer (рекомендуется):**

```bash
# Активировать timer
sudo systemctl enable hsm-rotation-check.timer
sudo systemctl start hsm-rotation-check.timer

# Проверить следующий запуск
sudo systemctl list-timers | grep hsm-rotation

# Посмотреть логи
sudo journalctl -u hsm-rotation-check.service -f
```

**2. Вручную:**

```bash
# Запустить проверку
sudo systemctl start hsm-rotation-check.service

# Или напрямую (с переменными из /etc/hsm-service/environment)
sudo su - hsm -c 'source /etc/hsm-service/environment && /opt/hsm-service/scripts/check-key-rotation.sh'
```

**Переменные окружения** из `/etc/hsm-service/environment`:
```bash
HSM_PIN=your-secret-pin
AUTO_ROTATE=true  # Включить автоматическую ротацию
SLACK_WEBHOOK=https://hooks.slack.com/...
ALERT_EMAIL=ops@company.com
```

## 🚀 Использование скрипта

### Режим 1: Только проверка (по умолчанию)

```bash
AUTO_ROTATE=false ./scripts/check-key-rotation.sh
```

**Результат:**
- ✅ Проверяет статус ключей
- ✅ Отправляет алерты через Slack/Email если нужна ротация
- ❌ НЕ выполняет ротацию автоматически

### Режим 2: Автоматическая ротация

```bash
AUTO_ROTATE=true ./scripts/check-key-rotation.sh
```

**Результат:**
- ✅ Проверяет статус ключей
- ✅ Выполняет ротацию просроченных ключей автоматически
- ✅ Отправляет алерты об успехе или ошибке

## 🔧 Конфигурирование уведомлений

### Email (опционально)

```bash
# Установить mailutils
apt install -y mailutils

# В /etc/hsm-service/environment добавить:
ALERT_EMAIL=ops@company.com
SEND_EMAIL=true
```

### Slack (рекомендуется)

```bash
# Создать Incoming Webhook: https://api.slack.com/messaging/webhooks

# В /etc/hsm-service/environment добавить:
SLACK_WEBHOOK=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
```

### Telegram (опционально)

```bash
# Создать Telegram бота и получить chat_id

# В /etc/hsm-service/environment добавить:
TELEGRAM_BOT_TOKEN=your-bot-token
TELEGRAM_CHAT_ID=your-chat-id
```

## 📋 Алерты и логирование

### Docker

```bash
# Посмотреть логи
docker logs hsm-service | grep rotation

# Или через journalctl (если включено logging to journal)
journalctl -u docker -f | grep rotation
```

### Production

```bash
# Systemd journal
sudo journalctl -u hsm-rotation-check.service -f

# Логи скрипта
sudo tail -100 /var/log/hsm-service/rotation.log

# Логи сервиса
sudo journalctl -u hsm-service -f
```

## ⚠️ Частые проблемы

### Problem: "Cannot detect HSM environment"

**Решение:**
- Docker: Убедитесь что контейнер `hsm-service` запущен: `docker ps`
- Production: Убедитесь что systemd сервис установлен: `systemctl is-active hsm-service`

### Problem: "Failed to get rotation status"

**Решение:**
- Docker: `docker exec hsm-service /app/hsm-admin rotation-status` должна работать
- Production: `/opt/hsm-service/bin/hsm-admin -config /etc/hsm-service/config.yaml rotation-status` должна работать

### Problem: Ротация не выполняется в Production

**Решение:**
```bash
# 1. Проверить что AUTO_ROTATE=true в /etc/hsm-service/environment
cat /etc/hsm-service/environment | grep AUTO_ROTATE

# 2. Проверить права доступа пользователя hsm
sudo -u hsm /opt/hsm-service/bin/hsm-admin rotation-status

# 3. Проверить логи
sudo journalctl -u hsm-rotation-check.service -n 50
```

## 📚 Дополнительные ресурсы

- [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) - Полное руководство по развертыванию
- [KEY_ROTATION.md](KEY_ROTATION.md) - Документация по ротации ключей
- [scripts/check-key-rotation.sh](scripts/check-key-rotation.sh) - Исходный код скрипта
