# ⚡ HSM Service - Quick Start (Native Binary)

> **Для опытных разработчиков**: Запуск нативного Go бинарника HSM Service (без Docker)

## 📋 Предварительные требования

- ✅ Go 1.22+ установлен
- ✅ SoftHSM2 установлен (`softhsm2-util --version`)
- ✅ **PKI инфраструктура настроена** (CA, сертификаты)

**📖 Если PKI не настроена**, сначала выполните:
👉 **[PKI_SETUP.md](PKI_SETUP.md)** - создание CA и генерация всех сертификатов

---

## Шаг 1: Сборка

```bash
# Клонировать репозиторий
git clone <repository-url>
cd hsm-service

# Скачать зависимости
go mod download

# Собрать основной сервис
go build -o hsm-service .

# Собрать admin CLI
go build -o hsm-admin ./cmd/hsm-admin

# Проверка
./hsm-service --version
./hsm-admin --help
```

---

## Шаг 2: Инициализация SoftHSM

### 2.1. Настройка SoftHSM

```bash
# Создать директорию для токенов
mkdir -p data/tokens

# Инициализировать токен
softhsm2-util --init-token \
  --slot 0 \
  --label "hsm-token" \
  --so-pin 5678 \
  --pin 1234

# Проверка
softhsm2-util --show-slots
```

**Ожидаемый вывод**:
```
Slot 0
    Slot info:
        Description:      SoftHSM slot
        Manufacturer ID:  SoftHSM project
        Hardware version: 2.6
        Firmware version: 2.6
    Token info:
        Manufacturer ID:  SoftHSM project
        Model:            SoftHSM v2
        Label:            hsm-token
```

### 2.2. Установить HSM_PIN

```bash
export HSM_PIN="1234"
```

---

## Шаг 3: Настройка конфигурации

---

## Шаг 3: Настройка конфигурации

### 3.1. config.yaml

```bash
# Скопировать шаблон
cp config.yaml.example config.yaml
```

**config.yaml** (пример для development):

```yaml
server:
  port: "8443"
  tls:
    ca_path: /app/pki/ca/ca.crt
    cert_path: /app/pki/server/hsm-service.local.crt
    key_path: /app/pki/server/hsm-service.local.key

hsm:
  pkcs11_lib: /usr/lib/softhsm/libsofthsm2.so
  slot_id: hsm-token
  metadata_file: /app/metadata.yaml
  keys:
    exchange-key:
      type: aes
      rotation_interval: 2160h  # 90 days
    2fa:
      type: aes
      rotation_interval: 2160h

acl:
  revoked_file: /app/pki/revoked.yaml
  mappings:
    Trading: [exchange-key]
    2FA: [2fa]

rate_limit:
  requests_per_second: 100
  burst: 50

logging:
  level: info
  format: json
```

### 3.2. metadata.yaml

```bash
# Скопировать шаблон
cp metadata.yaml.example metadata.yaml
```

**metadata.yaml** (обновляется автоматически):

```yaml
rotation:
  exchange-key:
    label: kek-exchange-v1
    version: 1
    created_at: '2025-10-11T12:00:00Z'
  
  2fa:
    label: kek-2fa-v1
    version: 1
    created_at: '2025-10-11T12:00:00Z'
```

---

## Шаг 4: Инициализация KEK

```bash
# Создать KEK ключи в SoftHSM
./hsm-admin init-keys
```

**Что происходит**:
- Читается `pki/inventory.yaml` (список всех KEK)
- Для каждого context создается AES-256 ключ в SoftHSM
- Обновляется `metadata.yaml`

**Ожидаемый вывод**:
```
Initializing KEK keys from inventory...
✓ Created kek-exchange-v1 (AES-256, context: exchange-key)
✓ Created kek-2fa-v1 (AES-256, context: 2fa)
✓ Updated metadata.yaml
Done! Initialized 2 KEK keys.
```

---

## Шаг 5: Запуск сервиса

```bash
# Установить HSM_PIN (если еще не установлен)
export HSM_PIN="1234"

# Запустить сервис
./hsm-service
```

**Ожидаемый вывод**:
```
INFO  Initializing SoftHSM token...
INFO  Loading KEK from inventory...
INFO  Starting HSM service on port 8443
INFO  started revoked.yaml auto-reload interval=30s
INFO  Loaded 2 KEKs: [kek-exchange-v1 kek-2fa-v1]
```

✅ Сервис запущен на `https://localhost:8443`

---

## Шаг 6: Тестирование

### 6.1. Health Check

```bash
curl -k https://localhost:8443/health \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt
```

**Ожидаемый ответ**:
```json
{
  "status": "healthy",
  "active_keys": 2,
  "version": "1.0.0"
}
```

### 6.2. Encrypt/Decrypt

```bash
# Шифрование
curl -k -X POST https://localhost:8443/encrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{
    "context": "exchange-key",
    "plaintext": "SGVsbG8gV29ybGQh"
  }'
```

Подробнее: [API.md](API.md)

---

## Graceful Shutdown

**Способ 1**: Нажмите `Ctrl+C`

**Способ 2**: Отправьте SIGTERM

```bash
kill -TERM $(pgrep hsm-service)
```

**Ожидаемый вывод**:
```
2026/01/07 00:31:00 Received signal interrupt, shutting down gracefully...
2026/01/07 00:31:00 Stopping ACL auto-reload...
2026/01/07 00:31:00 Stopping HTTP server...
2026/01/07 00:31:00 Closing HSM context...
2026/01/07 00:31:00 HSM service stopped
```

**Graceful shutdown включает**:
- Остановку auto-reload для revoked.yaml (timeout 15s)
- Закрытие HTTP server (graceful shutdown timeout 30s)
- Освобождение HSM context и ресурсов

---

## 📊 Мониторинг и метрики

```bash
# Prometheus метрики
curl -k https://localhost:8443/metrics \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt | grep hsm_

# Health check
curl -k https://localhost:8443/health \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt | jq .
```

Подробнее: [MONITORING.md](MONITORING.md)

---

## 🔧 hsm-admin CLI утилиты

```bash
# Список KEK
./hsm-admin list-kek

# Ротация ключа
./hsm-admin rotate exchange-key

# Отзыв сертификата
./hsm-admin revoke-cert trading-service-1

# Проверка сертификата
./hsm-admin check-cert pki/client/trading-service-1.crt
```

Подробнее: [CLI_TOOLS.md](CLI_TOOLS.md)

---

## ✅ Unit тесты

```bash
# Запустить все тесты
go test ./...

# С покрытием
go test -cover ./...

# Интеграционные тесты
./tests/integration/full-integration-test.sh
```

Подробнее: [TESTING_GUIDE.md](TESTING_GUIDE.md)

---

## ❓ Troubleshooting

### ❌ "softhsm2-util: command not found"

**Решение**: Установите SoftHSM2
```bash
# Ubuntu/Debian
sudo apt install softhsm2

# macOS
brew install softhsm

# Проверка
softhsm2-util --version
```

### ❌ "Failed to load PKCS#11 library"

**Решение**: Проверьте путь к библиотеке в `config.yaml`
```bash
# Найти библиотеку
find /usr -name libsofthsm2.so 2>/dev/null

# Обновить config.yaml
# pkcs11_lib: /usr/lib/softhsm/libsofthsm2.so  # или путь из find
```

### ❌ "Token not found: hsm-token"

**Решение**: Инициализируйте токен
```bash
softhsm2-util --init-token --slot 0 --label "hsm-token" --so-pin 5678 --pin 1234
softhsm2-util --show-slots
```

### ❌ "OU not authorized for context"

**Решение**: Проверьте ACL в `config.yaml` и OU в сертификате
```bash
# Проверить OU
openssl x509 -in pki/client/trading-service-1.crt -noout -subject

# Проверить ACL
cat config.yaml | grep -A5 "acl:"
```

---

## 📚 Что дальше?

После успешного запуска native binary:

### Для разработчиков:
- 📖 **API Reference**: [API.md](API.md)
- 🏗️ **Архитектура**: [ARCHITECTURE.md](ARCHITECTURE.md)
- 🔧 **CLI утилиты**: [CLI_TOOLS.md](CLI_TOOLS.md)

### Для DevOps:
- 🏭 **Production**: [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md)
- 🐳 **Docker**: [QUICKSTART_DOCKER.md](QUICKSTART_DOCKER.md)
- 📊 **Мониторинг**: [MONITORING.md](MONITORING.md)

### Для security:
- 🔒 **PKI управление**: [pki/README.md](pki/README.md)
- 🔄 **Ротация ключей**: [KEY_ROTATION.md](KEY_ROTATION.md)

---

**Готово!** Ваш HSM Service запущен в native режиме 🚀
- Завершение HTTP сервера (existing connections)
- Закрытие PKCS#11 сессии
- Cleanup rate limiter goroutines

## Auto-Reload Revoked Certificates

Сервис автоматически перезагружает `revoked.yaml` каждые **30 секунд** без перезапуска.

### Features

✅ **Automatic validation**: битые YAML файлы не применяются
✅ **Old data preserved**: при ошибке валидации старые данные сохраняются  
✅ **No downtime**: перезагрузка происходит в фоне
✅ **File deletion handling**: если файл удален → список очищается

### Validation Rules

```yaml
# ✅ Valid
revoked:
  - cn: "test.example.com"
    serial: "1234"
    reason: "key-compromise"
    date: "2024-01-15"

# ❌ Invalid - empty CN
revoked:
  - cn: ""
    serial: "1234"

# ❌ Invalid - duplicate CN
revoked:
  - cn: "test.example.com"
    serial: "1234"
  - cn: "test.example.com"  # ERROR: duplicate
    serial: "5678"

# ❌ Invalid - syntax error
revoked:
  - cn: "test
    reason: unclosed quote
```

### Logs

```
INFO  started revoked.yaml auto-reload interval=30s file=/app/pki/revoked.yaml
INFO  revoked.yaml reloaded successfully path=/app/pki/revoked.yaml count=5
WARN  revoked.yaml reload skipped due to validation error path=/app/pki/revoked.yaml
INFO  revoked.yaml deleted, cleared revocation list
```

**Подробная документация**: [REVOCATION_RELOAD.md](REVOCATION_RELOAD.md)

## Проверка работы

### Health check

```bash
curl --cacert pki/ca/ca.crt \
     --cert pki/client/trading-service-1.crt \
     --key pki/client/trading-service-1.key \
     https://localhost:8443/health
```

Ответ:
```json
{
  "status": "ok",
  "hsm_initialized": true,
  "active_keys": ["kek-exchange-v1"]
}
```

### Encrypt

```bash
curl --cacert pki/ca/ca.crt \
     --cert pki/client/trading-service-1.crt \
     --key pki/client/trading-service-1.key \
     -X POST https://localhost:8443/encrypt \
     -H "Content-Type: application/json" \
     -d '{
       "context": "exchange-key",
       "plaintext": "SGVsbG8gV29ybGQh"
     }'
```

Ответ:
```json
{
  "ciphertext": "base64-encrypted-data...",
  "key_id": "kek-exchange-v1"
}
```

### Decrypt

```bash
curl --cacert pki/ca/ca.crt \
     --cert pki/client/trading-service-1.crt \
     --key pki/client/trading-service-1.key \
     -X POST https://localhost:8443/decrypt \
     -H "Content-Type: application/json" \
     -d '{
       "context": "exchange-key",
       "ciphertext": "base64-encrypted-data...",
       "key_id": "kek-exchange-v1"
     }'
```

Ответ:
```json
{
  "plaintext": "SGVsbG8gV29ybGQh"
}
```

## Компоненты

- **Config**: Загрузка из config.yaml + env overrides
- **HSM**: PKCS#11 инициализация с PIN из ENV
- **ACL**: Проверка OU + revocation list
- **Rate Limiter**: Per-client ограничение (100 req/s, burst 50)
- **Server**: TLS 1.3 + mTLS на порту 8443
- **Middleware**: Rate Limit → Audit → Recovery → Request Log

## Тесты

```bash
# Все тесты
go test ./... -v

# Только config
go test ./internal/config -v

# Только server
go test ./internal/server -v

# Только HSM
go test ./internal/hsm -v
```

Всего: **30 unit tests**
