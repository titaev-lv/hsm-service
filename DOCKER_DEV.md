# 🐳 HSM Service - Запуск в Docker (Development)

> **Для разработчиков**: Как запустить HSM Service в Docker для локальной разработки

## Оглавление

- [Быстрый старт](#быстрый-старт)
- [Структура Docker setup](#структура-docker-setup)
- [Dockerfile объяснение](#dockerfile-объяснение)
- [docker-compose.yml объяснение](#docker-composeyml-объяснение)
- [Volumes и персистентность](#volumes-и-персистентность)
- [Переменные окружения](#переменные-окружения)
- [Разработка с hot-reload](#разработка-с-hot-reload)
- [Debugging в контейнере](#debugging-в-контейнере)
- [Troubleshooting](#troubleshooting)

---

## Быстрый старт

### Предварительные требования

```bash
# Docker
docker --version  # >= 20.10

# Docker Compose
docker-compose --version  # >= 1.29

# Git
git --version
```

### Шаг 1: Клонировать репозиторий

```bash
git clone <repository-url>
cd hsm-service
```

### Шаг 2: Генерация PKI

```bash
cd pki
./scripts/generate-all.sh
cd ..
```

**Что создается**:
- `pki/ca/` - CA сертификат
- `pki/server/` - Серверный сертификат для hsm-service.local
- `pki/client/` - Клиентские сертификаты для тестирования
- `pki/revoked.yaml` - Пустой список отозванных сертификатов

### Шаг 3: Настройка .env

```bash
cp .env.example .env
nano .env
```

**Минимальная конфигурация .env**:
```bash
# SoftHSM Configuration
HSM_PIN=1234                    # PIN для SoftHSM (измените!)
SLOT_LABEL=hsm-token           # Имя токена

# Logging
LOG_LEVEL=info                 # debug, info, warn, error
LOG_FORMAT=json                # json, text
```

### Шаг 4: Создание директорий

```bash
# Создать директорию для SoftHSM tokens
mkdir -p data/tokens
chmod 755 data/tokens

# Создать директорию для логов (опционально)
mkdir -p data/logs
chmod 755 data/logs
```

### Шаг 5: Запуск

```bash
docker-compose up -d
```

**Что происходит**:
1. Собирается Docker image `hsm-service:latest`
2. Запускается контейнер с именем `hsm-service`
3. Инициализируется SoftHSM token
4. Создаются KEK ключи из config.yaml
5. Запускается HTTPS сервер на :8443

### Шаг 6: Проверка

```bash
# Статус контейнера
docker-compose ps

# Логи
docker-compose logs hsm-service

# Health check
curl -k https://localhost:8443/health \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt
```

---

## Структура Docker setup

```
hsm-service/
├── Dockerfile                  # Multi-stage build
├── docker-compose.yml          # Оркестрация
├── .dockerignore              # Что не копировать в image
├── .env.example               # Шаблон переменных
├── .env                       # Локальные переменные (gitignored)
├── config.yaml                # Конфигурация сервиса (read-only)
├── metadata.yaml.example      # Пример метаданных
├── metadata.yaml              # Метаданные ротации (read-write)
├── softhsm2.conf             # Конфигурация SoftHSM
└── data/
    ├── tokens/                # Persistent SoftHSM tokens
    └── logs/                  # Опционально: логи
```

---

## Dockerfile объяснение

### Multi-stage Build

```dockerfile
# Stage 1: Builder
FROM golang:1.22-alpine AS builder

WORKDIR /build
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o hsm-service .
RUN CGO_ENABLED=0 GOOS=linux go build -o hsm-admin ./cmd/hsm-admin

# Stage 2: Runtime
FROM alpine:3.19

# Install SoftHSM
RUN apk add --no-cache softhsm opensc ca-certificates

# Copy binaries
COPY --from=builder /build/hsm-service /app/hsm-service
COPY --from=builder /build/hsm-admin /app/hsm-admin

# Copy config
COPY config.yaml /app/config.yaml
COPY softhsm2.conf /etc/softhsm2.conf

# Setup directories
RUN mkdir -p /var/lib/softhsm/tokens /var/log/hsm-service

EXPOSE 8443
CMD ["/app/hsm-service"]
```

**Преимущества multi-stage**:
- ✅ Builder image: ~800MB (с Go toolchain)
- ✅ Runtime image: ~50MB (только alpine + SoftHSM + binary)
- ✅ Быстрый деплой (маленький образ)
- ✅ Безопасность (нет build tools в production)

### Оптимизации

1. **Layer caching**:
   ```dockerfile
   COPY go.mod go.sum ./  # Cache dependencies
   RUN go mod download
   COPY . .                # Code changes don't invalidate deps
   ```

2. **Static binary**:
   ```bash
   CGO_ENABLED=0  # No C dependencies
   ```

3. **Small base image**:
   ```dockerfile
   FROM alpine:3.19  # ~5MB
   ```

---

## docker-compose.yml объяснение

```yaml
version: '3.8'

services:
  hsm-service:
    build:
      context: .
      dockerfile: Dockerfile
    image: hsm-service:latest
    container_name: hsm-service
    
    # Restart policy
    restart: unless-stopped
    
    # Ports
    ports:
      - "8443:8443"
    
    # Environment variables
    environment:
      - HSM_PIN=${HSM_PIN}
      - SLOT_LABEL=${SLOT_LABEL:-hsm-token}
      - LOG_LEVEL=${LOG_LEVEL:-info}
    
    # Volumes
    volumes:
      - ./data/tokens:/var/lib/softhsm/tokens  # Persistent HSM tokens
      - ./pki:/app/pki:ro                       # PKI (read-only)
      - ./config.yaml:/app/config.yaml:ro       # Config (read-only)
      - ./metadata.yaml:/app/metadata.yaml:rw   # Metadata (read-write!)
      - ./pki/revoked.yaml:/app/pki/revoked.yaml:ro  # Revocation list
    
    # Health check
    healthcheck:
      test: ["CMD", "wget", "--no-check-certificate", "--spider", "https://localhost:8443/health"]
      interval: 30s
      timeout: 10s
      retries: 3
      start_period: 10s
    
    # Limits
    deploy:
      resources:
        limits:
          cpus: '1.0'
          memory: 512M
        reservations:
          cpus: '0.5'
          memory: 256M
```

### Ключевые моменты

#### 1. Volumes (монтирование)

| Volume | Mode | Описание |
|--------|------|----------|
| `data/tokens` | rw | SoftHSM токены (persistent!) |
| `pki/` | ro | Сертификаты (read-only) |
| `config.yaml` | ro | Конфигурация (read-only) |
| `metadata.yaml` | **rw** | Метаданные ротации (read-write!) |
| `pki/revoked.yaml` | ro | Список отзыва |

**Важно**: `metadata.yaml` должен быть `:rw` потому что:
- Обновляется при ротации ключей (`hsm-admin rotate`)
- Содержит текущую версию KEK

#### 2. Environment Variables

```yaml
environment:
  - HSM_PIN=${HSM_PIN}  # From .env file
  - SLOT_LABEL=${SLOT_LABEL:-hsm-token}  # Default: hsm-token
```

**Приоритет**:
1. Docker Compose `environment:` section
2. `.env` file
3. System environment variables

#### 3. Health Check

```yaml
healthcheck:
  test: ["CMD", "wget", "--spider", "https://localhost:8443/health"]
  interval: 30s
```

**Statuses**:
- `starting` - First 10s (start_period)
- `healthy` - Health check passed
- `unhealthy` - 3 consecutive failures

Проверка:
```bash
docker inspect --format='{{.State.Health.Status}}' hsm-service
```

#### 4. Resource Limits

```yaml
limits:
  cpus: '1.0'      # Maximum 1 CPU core
  memory: 512M     # Maximum 512MB RAM
```

**Зачем**:
- Защита от runaway processes
- Predictable performance
- Multi-tenant environments

---

## Volumes и персистентность

### Что сохраняется между перезапусками?

✅ **Сохраняется**:
- `data/tokens/` - SoftHSM keys
- `metadata.yaml` - KEK versions

❌ **НЕ сохраняется**:
- Логи внутри контейнера (если не монтированы)
- Temporary files в /tmp

### Backup важных данных

```bash
# Backup SoftHSM tokens
tar -czf hsm-tokens-backup-$(date +%Y%m%d).tar.gz data/tokens/

# Backup metadata
cp metadata.yaml metadata.yaml.backup-$(date +%Y%m%d)

# Backup PKI
tar -czf pki-backup-$(date +%Y%m%d).tar.gz pki/
```

### Restore из backup

```bash
# Stop service
docker-compose down

# Restore tokens
tar -xzf hsm-tokens-backup-20260109.tar.gz

# Restore metadata
cp metadata.yaml.backup-20260109 metadata.yaml

# Start service
docker-compose up -d
```

---

## Переменные окружения

### Обязательные

| Variable | Example | Описание |
|----------|---------|----------|
| `HSM_PIN` | `1234` | PIN для SoftHSM token |

### Опциональные

| Variable | Default | Описание |
|----------|---------|----------|
| `SLOT_LABEL` | `hsm-token` | Имя SoftHSM slot |
| `LOG_LEVEL` | `info` | debug, info, warn, error |
| `LOG_FORMAT` | `json` | json, text |

### Установка переменных

**Метод 1: .env file** (рекомендуется)
```bash
# .env
HSM_PIN=my-secure-pin-12345
LOG_LEVEL=debug
```

**Метод 2: docker-compose.yml**
```yaml
environment:
  - HSM_PIN=1234
  - LOG_LEVEL=debug
```

**Метод 3: Command line**
```bash
HSM_PIN=1234 docker-compose up -d
```

---

## Разработка с hot-reload

### Вариант 1: Volume mount исходников

```yaml
# docker-compose.dev.yml
volumes:
  - .:/app/src:ro  # Mount source code
command: >
  sh -c "cd /app/src && 
         go run . || 
         /app/hsm-service"
```

**Проблема**: Нужно пересборку при изменении кода.

### Вариант 2: Air (hot reload tool)

```dockerfile
# Dockerfile.dev
FROM golang:1.22-alpine

RUN go install github.com/cosmtrek/air@latest

WORKDIR /app
CMD ["air", "-c", ".air.toml"]
```

```yaml
# docker-compose.dev.yml
build:
  dockerfile: Dockerfile.dev
volumes:
  - .:/app
```

**`.air.toml`**:
```toml
[build]
  cmd = "go build -o ./tmp/hsm-service ."
  bin = "tmp/hsm-service"
  include_ext = ["go"]
  exclude_dir = ["tmp", "vendor"]
```

**Использование**:
```bash
docker-compose -f docker-compose.dev.yml up
# Изменяйте код → автоматический rebuild
```

---

## Debugging в контейнере

### 1. Интерактивный shell

```bash
docker-compose exec hsm-service sh
```

### 2. Проверка SoftHSM

```bash
# Внутри контейнера
softhsm2-util --show-slots

# List objects
pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --slot-index 0 \
  --login --pin 1234 \
  --list-objects
```

### 3. Проверка сертификатов

```bash
# Проверка монтирования PKI
docker-compose exec hsm-service ls -la /app/pki

# Проверка CA cert
docker-compose exec hsm-service openssl x509 \
  -in /app/pki/ca/ca.crt \
  -noout -subject -dates
```

### 4. Логи с деталями

```bash
# Real-time logs
docker-compose logs -f hsm-service

# Last 100 lines
docker-compose logs --tail=100 hsm-service

# JSON formatted logs (если LOG_FORMAT=json)
docker-compose logs hsm-service | jq .
```

### 5. Delve debugger

```dockerfile
# Dockerfile.debug
FROM golang:1.22-alpine

RUN go install github.com/go-delve/delve/cmd/dlv@latest

WORKDIR /app
COPY . .

# Build with debug symbols
RUN go build -gcflags="all=-N -l" -o hsm-service .

EXPOSE 8443 2345
CMD ["dlv", "exec", "./hsm-service", "--headless", "--listen=:2345", "--api-version=2"]
```

**docker-compose.debug.yml**:
```yaml
build:
  dockerfile: Dockerfile.debug
ports:
  - "8443:8443"
  - "2345:2345"  # Delve port
security_opt:
  - "apparmor=unconfined"
cap_add:
  - SYS_PTRACE
```

**VS Code launch.json**:
```json
{
  "version": "0.2.0",
  "configurations": [
    {
      "name": "Connect to Docker",
      "type": "go",
      "request": "attach",
      "mode": "remote",
      "remotePath": "/app",
      "port": 2345,
      "host": "localhost"
    }
  ]
}
```

---

## Troubleshooting

### Problem: Permission denied на data/tokens

```bash
# Fix permissions
chmod 755 data/tokens

# Or use Docker user
docker-compose exec -u root hsm-service chmod 755 /var/lib/softhsm/tokens
```

### Problem: HSM_PIN неверный

```bash
# Проверить .env
cat .env | grep HSM_PIN

# Пересоздать токен
docker-compose down -v  # Delete volumes!
rm -rf data/tokens/*
docker-compose up -d
```

### Problem: Certificate не найден

```bash
# Проверить монтирование
docker-compose exec hsm-service ls -la /app/pki/ca
docker-compose exec hsm-service ls -la /app/pki/server
docker-compose exec hsm-service ls -la /app/pki/client
```

### Problem: Port 8443 уже занят

```bash
# Найти процесс
lsof -i :8443

# Или измените порт в docker-compose.yml
ports:
  - "9443:8443"  # Host:Container
```

### Problem: Container exits immediately

```bash
# Проверить логи
docker-compose logs hsm-service

# Запустить вручную для debugging
docker-compose run --rm hsm-service sh
```

### Problem: Health check failing

```bash
# Проверить health status
docker inspect --format='{{json .State.Health}}' hsm-service | jq .

# Manual health check
docker-compose exec hsm-service wget -O- --no-check-certificate https://localhost:8443/health
```

---

## Полезные команды

### Docker Compose

```bash
# Запуск в background
docker-compose up -d

# Пересборка образа
docker-compose up -d --build

# Остановка
docker-compose down

# Остановка + удаление volumes (осторожно!)
docker-compose down -v

# Рестарт одного сервиса
docker-compose restart hsm-service

# Посмотреть статус
docker-compose ps

# Логи
docker-compose logs -f hsm-service
```

### Docker

```bash
# Список образов
docker images

# Удалить образ
docker rmi hsm-service:latest

# Удалить все неиспользуемые образы
docker image prune -a

# Exec команда
docker exec -it hsm-service sh

# Копирование файлов
docker cp hsm-service:/app/metadata.yaml ./metadata-backup.yaml
docker cp ./new-config.yaml hsm-service:/app/config.yaml
```

### hsm-admin commands

```bash
# List KEKs
docker-compose exec hsm-service /app/hsm-admin list-kek

# Rotate key
docker-compose exec hsm-service /app/hsm-admin rotate exchange-key

# Rotation status
docker-compose exec hsm-service /app/hsm-admin rotation-status

# Cleanup old versions
docker-compose exec hsm-service /app/hsm-admin cleanup-old-versions --dry-run
```

---

## Production готовность

Перед деплоем в production:

- [ ] Измените `HSM_PIN` на сильный (не 1234!)
- [ ] Используйте настоящие сертификаты (не self-signed)
- [ ] Настройте мониторинг (Prometheus)
- [ ] Настройте алерты
- [ ] Настройте бэкапы (tokens + metadata)
- [ ] Протестируйте ротацию ключей
- [ ] Настройте log aggregation
- [ ] Review SECURITY_AUDIT.md
- [ ] Настройте firewall
- [ ] Enable resource limits

См. [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) для production setup.

---

## Ссылки

- [QUICKSTART.md](QUICKSTART.md) - Быстрый старт для новичков
- [API.md](API.md) - API документация
- [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) - Production deployment
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Решение проблем
