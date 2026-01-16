# HSM Service - Quick Start (Docker)

> **Цель**: Запустить HSM Service в Docker и выполнить первый тестовый запрос за 5 минут

## 📋 Предварительные требования

- ✅ Docker + Docker Compose установлены
- ✅ **PKI инфраструктура настроена** (CA, сертификаты)

**📖 Если PKI не настроена**, сначала выполните:
👉 **[PKI_SETUP.md](PKI_SETUP.md)** - создание CA и генерация всех сертификатов

---

## Шаг 1: Подготовка проекта

```bash
# Клонировать репозиторий
git clone <repository-url>
cd hsm-service

# Проверить что PKI готова
ls -la pki/ca/ca.crt
ls -la pki/server/hsm-service.local.*
ls -la pki/client/trading-service-1.*
```

**Ожидаемый вывод**:
```
pki/ca/ca.crt                      ✓
pki/server/hsm-service.local.crt   ✓
pki/server/hsm-service.local.key   ✓
pki/client/trading-service-1.crt   ✓
pki/client/trading-service-1.key   ✓
```

**❌ Если файлы отсутствуют** → см. [PKI_SETUP.md](PKI_SETUP.md)

---

## Шаг 2: Конфигурация metadata.yaml

**Файл metadata.yaml** создается автоматически при первом запуске контейнера скриптом `init-hsm.sh`. Он содержит динамические метаданные ключей (версии, timestamps) и обновляется автоматически при ротации.

**Структура metadata.yaml** (создается автоматически):
```yaml
rotation:
  exchange-key:
    current: kek-exchange-key-v1
    versions:
      - label: kek-exchange-key-v1
        version: 1
        created_at: '2026-01-10T12:00:00Z'
  2fa:
    current: kek-2fa-v1
    versions:
      - label: kek-2fa-v1
        version: 1
        created_at: '2026-01-10T12:00:00Z'
```

---

## Шаг 3: Запуск HSM Service

```bash
# Собрать Docker образ
docker build -t hsm-service:latest .
# Запустить 
docker compose up -d

# Проверить что контейнер запустился
docker compose ps
```

**Ожидаемый вывод**:
```
NAME           IMAGE          STATUS         PORTS
hsm-service    hsm-service    Up 5 seconds   0.0.0.0:8443->8443/tcp
```

**Проверить логи**:
```bash
docker compose logs hsm-service
```

**Ожидаемые логи**:
```
INFO  Initializing SoftHSM token...
INFO  Loading KEK from inventory...
INFO  Starting HSM service on port 8443
INFO  started revoked.yaml auto-reload interval=30s
INFO  Loaded 2 KEKs: [kek-exchange-key-v1 kek-2fa-v1]
```

---

## Шаг 4: Проверка автоматической инициализации

**Контейнер автоматически инициализирует KEK при первом запуске** через скрипт `init-hsm.sh`.

**Проверьте что KEK созданы**:
```bash
docker exec hsm-service /app/hsm-admin list-kek
```

**Ожидаемый вывод**:
```
KEK objects in HSM:
  Label: kek-exchange-key-v1, ID: 01, Type: AES (256 bits)
  Label: kek-2fa-v1, ID: 01, Type: AES (256 bits)

Total: 2 KEK(s)
```

**Что произошло при запуске**:
- Скрипт `init-hsm.sh` инициализировал SoftHSM токен
- Создал KEK ключи: `kek-exchange-key-v1` и `kek-2fa-v1`
- Обновил `metadata.yaml` с метаданными ключей
- Запустил HSM Service

---

## Шаг 5: Первый тестовый запрос

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
  "hsm_available": true,
  "kek_status": {
    "kek-2fa-v1": "available",
    "kek-exchange-key-v1": "available"
  }
}
```

**❌ Если ошибка**:
- `curl: (60) SSL certificate problem` → проверьте что CA сертификат правильный
- `curl: (35) error:14094410:SSL` → проверьте mTLS сертификаты
- `Connection refused` → проверьте `docker compose ps`

### 6.2. Шифрование (Encrypt)

```bash
# Подготовить plaintext (Hello World! в base64)
echo -n "Hello World!" | base64
# Вывод: SGVsbG8gV29ybGQh

# Encrypt запрос
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

**Параметры**:
- `context: "exchange-key"` - какой KEK использовать (из config.yaml)
- `plaintext` - данные в base64

**Ожидаемый ответ**:
```json
{
  "ciphertext": "plpmmI0StauF6ZWGfEnrlxom23Zt8wS1yPkqTCxgQykMRAkYhgZfLKprYzM=",
  "key_id": "kek-exchange-key-v1"
}
```

**💾 Сохраните ciphertext** - он нужен для расшифрования!

### 6.3. Расшифрование (Decrypt)

```bash
curl -k -X POST https://localhost:8443/decrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{
    "context": "exchange-key",
    "ciphertext": "base64_encrypted_data_here...",
    "key_id": "kek-exchange-key-v1"
  }'
```

**Ожидаемый ответ**:
```json
{
  "plaintext": "SGVsbG8gV29ybGQh"
}
```

**Проверка**:
```bash
echo "SGVsbG8gV29ybGQh" | base64 -d
# Вывод: Hello World!
```

✅ **Если plaintext совпадает с оригиналом - всё работает!**

---

## 🎉 Поздравляем!

Вы успешно:
- ✅ Настроили PKI с собственным CA
- ✅ Запустили HSM Service в Docker
- ✅ Инициализировали KEK ключи в SoftHSM
- ✅ Зашифровали и расшифровали данные через mTLS API

---

## Что дальше?

### 📖 Изучить API
Читайте [API.md](API.md) - полная документация всех эндпоинтов:
- `/encrypt`, `/decrypt` - базовые операции шифрования/расшифрования
- `/health` - проверка состояния сервиса и KEK
- `/metrics` - Prometheus метрики для мониторинга

### 🔧 Настроить мониторинг
Читайте [MONITORING.md](MONITORING.md):
- Prometheus + Grafana интеграция
- 8 групп метрик (операции, ротации, ошибки, latency)
- Готовые dashboards и алерты

### 🏭 Развернуть на production
Читайте [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md):
- Установка на Debian 13
- nftables firewall конфигурация
- systemd service
- Hardware HSM интеграция (опционально)

### 🧪 Запустить тесты
```bash
# Unit тесты
go test ./...

# Integration тесты
./tests/integration/full-integration-test.sh

# Подробнее в tests/README.md
```

---

## ❓ Troubleshooting

### ❌ Ошибка: "OU not authorized"

**Проблема**: Клиентский сертификат с OU=Trading пытается получить доступ к context=2fa

**Решение**: Проверьте ACL в `config.yaml`:
```yaml
acl:
  mappings:
    Trading:           # OU в сертификате
      - exchange-key   # Разрешенные contexts
    2FA:
      - 2fa            # 2FA OU может только 2fa context
```

**Проверка OU в сертификате**:
```bash
openssl x509 -in pki/client/trading-service-1.crt -noout -subject
```

### ❌ Ошибка: "Certificate revoked"

**Проблема**: Сертификат был отозван и находится в `pki/revoked.yaml`

**Решение**: Проверьте revoked.yaml:
```bash
cat pki/revoked.yaml | grep trading-service-1
```

Если сертификат отозван по ошибке - удалите запись из revoked.yaml (сервис перезагрузит файл автоматически через 30 секунд).

### ❌ Ошибка: "KEK not found for context"

**Проблема**: Запрашиваемый context не существует

**Решение**: Проверьте доступные contexts:
```bash
curl -k https://localhost:8443/health \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt | jq .
```

Список contexts определяется в `config.yaml` → `key_types`.

### ❌ Docker контейнер не запускается

```bash
# Проверить логи
docker compose logs hsm-service

# Пересобрать образ
docker compose up -d --build

# Проверить права на директории
ls -la data/tokens/
chmod 755 data/tokens/
```

---

## 💡 Полезные команды

```bash
# === Docker управление ===
docker compose up -d              # Запустить
docker compose down               # Остановить
docker compose logs -f            # Логи в реальном времени
docker compose restart            # Перезапустить

# === hsm-admin CLI ===
docker exec hsm-service /app/hsm-admin list-kek              # Список KEK
docker exec hsm-service /app/hsm-admin rotate exchange-key   # Ротация ключа
docker exec hsm-service /app/hsm-admin rotation-status       # Статус ротации
docker exec hsm-service /app/hsm-admin cleanup-old-versions exchange-key  # Очистка старых версий

# === Просмотр PKI ===
openssl x509 -in pki/ca/ca.crt -noout -text           # CA сертификат
openssl x509 -in pki/client/hsm-trading-client-1.crt -noout -subject -dates  # Клиентский сертификат
openssl x509 -in pki/server/hsm-service.local.crt -noout -subject -dates      # Серверный сертификат

# === Метрики ===
curl -k https://localhost:8443/metrics \
  --cert pki/client/hsm-trading-client-1.crt \
  --key pki/client/hsm-trading-client-1.key \
  --cacert pki/ca/ca.crt | grep hsm_
```

---

## 📚 Дополнительная документация

| Документ | Описание |
|----------|----------|
| [README.md](README.md) | Обзор проекта, use cases, PCI DSS compliance |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Архитектура, компоненты, data flow |
| [API.md](API.md) | Полная API reference |
| [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) | Production deployment |
| [MONITORING.md](MONITORING.md) | Prometheus + Grafana setup |
| [TROUBLESHOOTING.md](TROUBLESHOOTING.md) | Решение проблем |
| [CLI_TOOLS.md](CLI_TOOLS.md) | hsm-admin command reference |
| [tests/README.md](tests/README.md) | Руководство по тестированию |

**Готово!** Ваш HSM Service запущен и готов к работе 🚀
  -d '{
    "context": "exchange-key",
    "plaintext": "SGVsbG8gV29ybGQh"
  }'
```

**Объяснение**:
- `context: "exchange-key"` - какой KEK использовать
- `plaintext: "SGVsbG8gV29ybGQh"` - это "Hello World!" в base64

**Ожидаемый ответ**:
```json
{
  "ciphertext": "AQIDBHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8...",
  "key_id": "kek-exchange-key-v1"
}
```

**Сохраните ciphertext** - он нужен для расшифрования!

### 3. Расшифрование (Decrypt)

```bash
curl -k -X POST https://localhost:8443/decrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{
    "context": "exchange-key",
    "ciphertext": "AQIDBHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8fHx8...",
    "key_id": "kek-exchange-key-v1"
  }'
```

**Ожидаемый ответ**:
```json
{
  "plaintext": "SGVsbG8gV29ybGQh"
}
```

**Проверка**: `plaintext` должен совпадать с оригиналом!

```bash
echo "SGVsbG8gV29ybGQh" | base64 -d
# Output: Hello World!
```

✅ **Отлично!** Вы успешно зашифровали и расшифровали данные через HSM Service.

---

---

## ❓ Troubleshooting

### ❌ Ошибка: "OU not authorized"

**Проблема**: Клиентский сертификат с OU=Trading пытается получить доступ к context=2fa

**Решение**: Проверьте ACL в `config.yaml`:
```yaml
acl:
  mappings:
    Trading:           # OU в сертификате
      - exchange-key   # Разрешенные contexts
    2FA:
      - 2fa            # 2FA OU может только 2fa context
```

**Проверка OU в сертификате**:
```bash
openssl x509 -in pki/client/trading-service-1.crt -noout -subject
```

### ❌ Ошибка: "Certificate revoked"

**Проблема**: Сертификат был отозван и находится в `pki/revoked.yaml`

**Решение**: Проверьте revoked.yaml:
```bash
cat pki/revoked.yaml | grep trading-service-1
```

Если сертификат отозван по ошибке - удалите запись из revoked.yaml (сервис перезагрузит файл автоматически через 30 секунд).

### ❌ Ошибка: "KEK not found for context"

**Проблема**: Запрашиваемый context не существует

**Решение**: Проверьте доступные contexts:
```bash
curl -k https://localhost:8443/health \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt | jq .
```

Список contexts определяется в `config.yaml` → `hsm.keys`.

### ❌ Docker контейнер не запускается

```bash
# Проверить логи
docker compose logs hsm-service

# Пересобрать образ
docker compose up -d --build

# Проверить права на директории
ls -la data/tokens/
chmod 755 data/tokens/
```

### ❌ Permission denied на data/tokens

```bash
# Исправить права
chmod 755 data/tokens
```

### ❌ HSM_PIN неверный

```bash
# Проверить .env файл
cat .env | grep HSM_PIN

# Перезапустить
docker compose down
docker compose up -d
```

---

## 📚 Что дальше?

После успешного запуска Docker версии:

### Для backend разработчиков:
- 📖 **API Reference**: [API.md](API.md) - полная документация API
- 🔧 **CLI утилиты**: [CLI_TOOLS.md](CLI_TOOLS.md) - hsm-admin команды

### Для DevOps инженеров:
- 🏭 **Production**: [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) - развертывание на Debian
- 📊 **Мониторинг**: [MONITORING.md](MONITORING.md) - Prometheus + Grafana
- 🔄 **Ротация ключей**: [KEY_ROTATION.md](KEY_ROTATION.md)

### Для security инженеров:
- 🔒 **PKI управление**: [PKI_SETUP.md](PKI_SETUP.md) - полное руководство по сертификатам
- 🛡️ **Безопасность**: [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - аудит безопасности

---

**Готово!** Ваш HSM Service запущен в Docker и готов к работе 🚀
