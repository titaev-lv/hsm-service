# HSM Service - Technical Specification

**Version:** 1.0  
**Date:** 2026-01-05  
**Author:** Development Team  
**Status:** Draft

---

## 1. Введение

### 1.1 Цель документа

Данный документ содержит детальное техническое задание на разработку централизованного криптографического сервиса (HSM Service) для шифрования и расшифрования данных с использованием SoftHSM v2.

### 1.2 Область применения

Сервис предназначен для:
- Шифрования/расшифрования Data Encryption Keys (DEK) для торговых сервисов
- Шифрования/расшифрования 2FA секретов для веб-сервисов
- Централизованного управления Key Encryption Keys (KEK)
- Предоставления безопасного криптографического API через mTLS

### 1.3 Целевая аудитория

- Backend разработчики
- DevOps инженеры
- Security engineers
- System architects

---

## 2. Требования

### 2.1 Функциональные требования

#### FR-1: Шифрование данных
**Приоритет:** CRITICAL  
**Описание:** Сервис ДОЛЖЕН предоставлять endpoint для шифрования произвольных данных.

**Спецификация:**
- Endpoint: `POST /encrypt`
- Метод: HTTPS (TLS 1.3) с mTLS
- Input:
  ```json
  {
    "context": "exchange-key | 2fa",
    "plaintext": "base64_encoded_data"
  }
  ```
- Output (успех):
  ```json
  {
    "ciphertext": "base64_encoded_encrypted_data",
    "key_id": "kek-exchange-v1 | kek-2fa-v1"
  }
  ```
- Output (ошибка):
  ```json
  {
    "error": "error_message"
  }
  ```
- HTTP Status Codes:
  - 200: Success
  - 400: Bad Request (invalid input)
  - 403: Forbidden (ACL denied)
  - 429: Too Many Requests (rate limit)
  - 500: Internal Server Error

**Acceptance Criteria:**
1. Plaintext корректно декодируется из base64
2. Шифрование выполняется через SoftHSM (AES-256-GCM)
3. AAD формируется как `context + "|" + client_CN`
4. Ciphertext включает nonce (12 байт) + encrypted data + tag (16 байт)
5. Ciphertext возвращается в base64

---

#### FR-2: Расшифрование данных
**Приоритет:** CRITICAL  
**Описание:** Сервис ДОЛЖЕН предоставлять endpoint для расшифрования данных.

**Спецификация:**
- Endpoint: `POST /decrypt`
- Input:
  ```json
  {
    "context": "exchange-key | 2fa",
    "ciphertext": "base64_encoded_encrypted_data",
    "key_id": "kek-exchange-v1 | kek-2fa-v1"
  }
  ```
- Output (успех):
  ```json
  {
    "plaintext": "base64_encoded_data"
  }
  ```
- AAD ДОЛЖЕН совпадать с тем, что использовался при шифровании
- При несовпадении AAD возвращается ошибка 400

**Acceptance Criteria:**
1. Ciphertext корректно парсится (nonce || data || tag)
2. AAD проверяется при расшифровании
3. Неверный AAD → 400 Bad Request
4. Неверный key_id → 404 Not Found
5. Plaintext возвращается в base64

---

#### FR-3: Аутентификация через mTLS
**Приоритет:** CRITICAL  
**Описание:** Сервис ДОЛЖЕН требовать валидный клиентский сертификат для всех операций.

**Спецификация:**
- TLS версия: 1.3 (минимум)
- Client Auth: RequireAndVerifyClientCert
- CA: файл `/app/certs/ca/ca.crt`
- Проверка:
  1. Сертификат подписан доверенным CA
  2. Сертификат не истек
  3. CN не в списке revoked.yaml

**Rejection scenarios:**
- Нет клиентского сертификата → TLS handshake failure
- Неподписанный CA → TLS handshake failure
- Истекший сертификат → TLS handshake failure
- CN в revoked.yaml → 403 Forbidden (после TLS)

---

#### FR-4: Авторизация по OU
**Приоритет:** CRITICAL  
**Описание:** Доступ к context определяется по Organizational Unit сертификата.

**Спецификация:**

| OU        | Allowed Contexts  |
|-----------|-------------------|
| Trading   | exchange-key      |
| 2FA       | 2fa               |
| Admin     | (future)          |

**Процесс проверки:**
1. Извлечь OU из `cert.Subject.OrganizationalUnit[0]`
2. Если OU отсутствует → 403 Forbidden
3. Получить разрешенные contexts для OU из config.yaml
4. Если запрошенный context не в списке → 403 Forbidden

**Acceptance Criteria:**
1. OU=Trading может использовать только context=exchange-key
2. OU=2FA может использовать только context=2fa
3. Неизвестный OU → 403 Forbidden
4. Запрос с неразрешенным context → 403 Forbidden

---

#### FR-5: Certificate Revocation
**Приоритет:** HIGH  
**Описание:** Сервис ДОЛЖЕН блокировать отозванные сертификаты.

**Спецификация:**
- Файл: `pki/revoked.yaml`
- Формат:
  ```yaml
  revoked:
    - cn: "compromised-service"
      serial: "05"
      revoked_date: "2026-01-03T10:00:00Z"
      reason: "compromised"
  ```
- Загрузка: при старте сервиса
- Hot reload: через SIGHUP (опционально)

**Acceptance Criteria:**
1. Сертификат с CN в revoked.yaml получает 403 Forbidden
2. Изменение revoked.yaml требует restart (или hot reload)
3. Лог содержит информацию о заблокированном CN

---

#### FR-6: Rate Limiting
**Приоритет:** MEDIUM  
**Описание:** Сервис ДОЛЖЕН ограничивать количество запросов от одного клиента.

**Спецификация:**
- Limit: 100 requests/second per client CN
- Burst: 50 additional requests
- Ответ при превышении: 429 Too Many Requests
- Header: `Retry-After: <seconds>`

**Acceptance Criteria:**
1. Клиент может сделать до 100 req/s стабильно
2. Burst позволяет до 150 req одновременно
3. Превышение → 429 с Retry-After заголовком

---

#### FR-7: Health Check
**Приоритет:** MEDIUM  
**Описание:** Сервис ДОЛЖЕН предоставлять endpoint для проверки состояния.

**Спецификация:**
- Endpoint: `GET /health`
- Аутентификация: НЕ требуется (public)
- Output:
  ```json
  {
    "status": "healthy | degraded | unhealthy",
    "hsm_available": true,
    "kek_status": {
      "kek-exchange-v1": "available",
      "kek-2fa-v1": "available"
    },
    "uptime_seconds": 3600
  }
  ```

**Health criteria:**
- healthy: HSM доступен, все KEK найдены
- degraded: HSM доступен, некоторые KEK отсутствуют
- unhealthy: HSM недоступен

---

#### FR-8: Audit Logging
**Приоритет:** HIGH  
**Описание:** Сервис ДОЛЖЕН логировать все криптографические операции.

**Спецификация:**
- Формат: JSON (structured logging)
- Поля:
  - timestamp
  - client_cn
  - client_ou
  - client_ip
  - operation (encrypt | decrypt)
  - context
  - key_id
  - status (success | error)
  - duration_ms
  - error_message (если есть)

**НЕ логируется:**
- ❌ plaintext
- ❌ ciphertext
- ❌ nonce
- ❌ KEK handles

**Destination:**
- stdout (для Docker)
- файл /var/log/hsm/audit.log (опционально)

---

### 2.2 Нефункциональные требования

#### NFR-1: Производительность
- **Throughput:** Минимум 1000 encrypt/decrypt операций в секунду на одном инстансе
- **Latency:** 
  - p50: < 10ms
  - p95: < 50ms
  - p99: < 100ms
- **Concurrent connections:** До 100 одновременных mTLS соединений

#### NFR-2: Безопасность
- **KEK Protection:** KEK НИКОГДА не покидает HSM (CKA_EXTRACTABLE=false)
- **TLS:** Только TLS 1.3, strong cipher suites
- **Secrets:** Никаких секретов в логах или на диске (кроме HSM token)
- **Memory:** Zeroing sensitive data after use (defer)

#### NFR-3: Доступность
- **Uptime:** 99.9% (phase 1: single instance)
- **Recovery Time:** < 5 минут при перезапуске
- **Backup:** SoftHSM token backup ежедневно

#### NFR-4: Maintainability
- **Code structure:** Модульная структура (internal/config, internal/hsm, internal/server)
- **Configuration:** YAML config + ENV для секретов
- **Documentation:** Inline comments для всех публичных функций
- **Error handling:** Понятные error messages

#### NFR-5: Observability
- **Metrics:** Prometheus-compatible metrics (опционально)
- **Logging:** Structured logging (JSON)
- **Health checks:** /health endpoint
- **Tracing:** Request ID для корреляции логов (опционально)

---

## 3. Архитектура системы

### 3.1 Компоненты

```
┌─────────────────────────────────────────────────┐
│              HSM Service Process                │
├─────────────────────────────────────────────────┤
│                                                 │
│  ┌───────────────────────────────────────────┐  │
│  │         HTTP Server (mTLS)                │  │
│  │  - TLS 1.3                                │  │
│  │  - Client cert validation                 │  │
│  │  - Port 8443                              │  │
│  └───────────────────────────────────────────┘  │
│                      ↓                          │
│  ┌───────────────────────────────────────────┐  │
│  │           Middleware                      │  │
│  │  - Rate Limiter                           │  │
│  │  - Audit Logger                           │  │
│  │  - ACL Checker                            │  │
│  └───────────────────────────────────────────┘  │
│                      ↓                          │
│  ┌───────────────────────────────────────────┐  │
│  │           Handlers                        │  │
│  │  - /encrypt handler                       │  │ 
│  │  - /decrypt handler                       │  │
│  │  - /health handler                        │  │
│  └───────────────────────────────────────────┘  │
│                      ↓                          │
│  ┌───────────────────────────────────────────┐  │
│  │         Crypto Engine                     │  │
│  │  - Key manager                            │  │
│  │  - AES-GCM operations                     │  │
│  │  - AAD builder                            │  │
│  └───────────────────────────────────────────┘  │
│                      ↓                          │
│  ┌───────────────────────────────────────────┐  │
│  │       PKCS#11 Interface                   │  │
│  │  - crypto11 library                       │  │
│  │  - In-process only                        │  │
│  └───────────────────────────────────────────┘  │
│                      ↓                          │
│  ┌───────────────────────────────────────────┐  │
│  │          SoftHSM v2                       │  │
│  │  - Token: hsm-token                       │  │
│  │  - KEK: kek-exchange-v1                   │  │
│  │  - KEK: kek-2fa-v1                        │  │
│  │  - Library: libsofthsm2.so                │  │
│  └───────────────────────────────────────────┘  │
│                                                 │
└─────────────────────────────────────────────────┘
```

### 3.2 Структура проекта

```
hsm-service/
├── cmd/
│   └── hsm-admin/              # CLI для управления KEK
│       └── main.go
│
├── internal/
│   ├── config/                 # Конфигурация
│   │   ├── config.go
│   │   └── types.go
│   │
│   ├── hsm/                    # PKCS#11 и криптография
│   │   ├── pkcs11.go          # Инициализация SoftHSM
│   │   ├── crypto.go          # Encrypt/Decrypt
│   │   ├── kek.go             # Управление KEK (CLI)
│   │   └── types.go
│   │
│   ├── server/                 # HTTP сервер
│   │   ├── server.go          # Setup
│   │   ├── handlers.go        # Endpoints
│   │   ├── acl.go             # ACL по OU
│   │   ├── middleware.go      # Rate limit, logging
│   │   └── types.go
│   │
│   └── revocation/             # Управление отзывами
│       ├── loader.go          # Загрузка revoked.yaml
│       └── checker.go
│
├── pki/                        # PKI инфраструктура
│   ├── ca/
│   │   ├── ca.crt
│   │   └── ca.key
│   ├── server/
│   ├── client/
│   ├── scripts/
│   │   ├── issue-server-cert.sh
│   │   ├── issue-client-cert.sh
│   │   ├── revoke-cert.sh
│   │   └── update-inventory.sh
│   ├── inventory.yaml
│   └── revoked.yaml
│
├── scripts/
│   └── init-hsm.sh
│
├── config.yaml
├── softhsm2.conf
├── Dockerfile
├── docker-compose.yml
├── go.mod
├── go.sum
└── main.go
```

### 3.3 Конфигурация

**config.yaml:**

```yaml
server:
  # Порт для HTTPS
  port: 8443
  
  # TLS конфигурация
  tls:
    ca_cert: /app/certs/ca/ca.crt
    server_cert: /app/certs/server/hsm-service.local.crt
    server_key: /app/certs/server/hsm-service.local.key
  
  # Таймауты
  read_timeout: 10s
  write_timeout: 10s
  idle_timeout: 60s

# HSM конфигурация
hsm:
  # Путь к библиотеке SoftHSM
  library: /usr/lib/softhsm/libsofthsm2.so
  
  # Имя токена
  token_label: hsm-token
  
  # PIN загружается из ENV: HSM_PIN
  
  # Активные KEK
  keys:
    - label: kek-exchange-v1
      context: exchange-key
      active: true
    
    - label: kek-2fa-v1
      context: 2fa
      active: true

# ACL конфигурация
acl:
  # Файл с отозванными сертификатами
  revoked_file: /app/pki/revoked.yaml
  
  # Маппинг OU -> contexts
  by_ou:
    Trading: [exchange-key]
    2FA: [2fa]

# Rate limiting
rate_limit:
  requests_per_second: 100
  burst: 50

# Logging
logging:
  level: info  # debug | info | warn | error
  format: json # json | text
  audit_log_path: /var/log/hsm/audit.log  # опционально
```

**Environment Variables (секреты):**

```bash
# HSM PIN (обязательно)
HSM_PIN=1234

# HSM SO PIN (для admin операций)
HSM_SO_PIN=12345678

# Опционально
CONFIG_PATH=/app/config.yaml
LOG_LEVEL=info
```

---

## 4. API Specification

### 4.1 POST /encrypt

**Description:** Шифрует plaintext данные с использованием KEK из HSM.

**Request:**

```http
POST /encrypt HTTP/1.1
Host: hsm-service.local:8443
Content-Type: application/json

{
  "context": "exchange-key",
  "plaintext": "SGVsbG8gV29ybGQ="
}
```

**Request Fields:**

| Field     | Type   | Required | Description                           |
|-----------|--------|----------|---------------------------------------|
| context   | string | Yes      | "exchange-key" или "2fa"              |
| plaintext | string | Yes      | Base64-encoded данные для шифрования  |

**Response (Success):**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "ciphertext": "nonce+ciphertext+tag в base64",
  "key_id": "kek-exchange-v1"
}
```

**Response (Error):**

```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": "OU Trading not allowed for context 2fa"
}
```

**Error Codes:**

| HTTP Code | Error                                  |
|-----------|----------------------------------------|
| 400       | Invalid JSON, invalid base64, missing fields |
| 403       | ACL denied, revoked certificate        |
| 429       | Rate limit exceeded                    |
| 500       | HSM error, internal error              |

---

### 4.2 POST /decrypt

**Description:** Расшифровывает ciphertext с использованием KEK из HSM.

**Request:**

```http
POST /decrypt HTTP/1.1
Host: hsm-service.local:8443
Content-Type: application/json

{
  "context": "exchange-key",
  "ciphertext": "nonce+ciphertext+tag в base64",
  "key_id": "kek-exchange-v1"
}
```

**Request Fields:**

| Field      | Type   | Required | Description                              |
|------------|--------|----------|------------------------------------------|
| context    | string | Yes      | "exchange-key" или "2fa"                 |
| ciphertext | string | Yes      | Base64-encoded зашифрованные данные      |
| key_id     | string | Yes      | Идентификатор KEK ("kek-exchange-v1")    |

**Response (Success):**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "plaintext": "SGVsbG8gV29ybGQ="
}
```

**Response (Error - AAD Mismatch):**

```http
HTTP/1.1 400 Bad Request
Content-Type: application/json

{
  "error": "AAD verification failed"
}
```

---

### 4.3 GET /health

**Description:** Проверка состояния сервиса.

**Request:**

```http
GET /health HTTP/1.1
Host: hsm-service.local:8443
```

**Response:**

```http
HTTP/1.1 200 OK
Content-Type: application/json

{
  "status": "healthy",
  "hsm_available": true,
  "kek_status": {
    "kek-exchange-v1": "available",
    "kek-2fa-v1": "available"
  },
  "uptime_seconds": 3600
}
```

---

## 5. Криптография

### 5.1 Алгоритм

**Cipher:** AES-256-GCM (Galois/Counter Mode)

**Параметры:**
- Key Size: 256 бит
- Nonce: 12 байт (96 бит), генерируется случайно для каждой операции
- Tag: 16 байт (128 бит)
- AAD: `context + "|" + client_CN`

### 5.2 Формат ciphertext

```
┌────────────┬─────────────────────┬────────────┐
│   Nonce    │    Ciphertext       │    Tag     │
│  12 bytes  │  Variable length    │  16 bytes  │
└────────────┴─────────────────────┴────────────┘
                       ↓
              Base64 Encode
                       ↓
            "bm9uY2U...dGFn"
```

**Пример:**

```
Input plaintext: "Hello World"
Context: "exchange-key"
Client CN: "trading-service-1"

1. AAD = "exchange-key|trading-service-1"
2. Nonce = random 12 bytes
3. AES-GCM Encrypt:
   - Key: kek-exchange-v1 (из HSM)
   - Plaintext: "Hello World"
   - AAD: "exchange-key|trading-service-1"
   - Nonce: <random>
   → Ciphertext + Tag

4. Output = Base64(nonce || ciphertext || tag)
```

### 5.3 KEK Properties

**PKCS#11 Attributes:**

```c
CKA_CLASS = CKO_SECRET_KEY
CKA_KEY_TYPE = CKK_AES
CKA_VALUE_LEN = 32  // 256 бит
CKA_LABEL = "kek-exchange-v1"
CKA_ID = <unique_id>

// Security attributes
CKA_TOKEN = CK_TRUE         // Persistent
CKA_PRIVATE = CK_TRUE       // Requires PIN
CKA_SENSITIVE = CK_TRUE     // Cannot read
CKA_EXTRACTABLE = CK_FALSE  // НИКОГДА не покидает HSM

// Usage attributes
CKA_ENCRYPT = CK_TRUE
CKA_DECRYPT = CK_TRUE
CKA_WRAP = CK_FALSE
CKA_UNWRAP = CK_FALSE
```

---

## 6. PKI Infrastructure

### 6.1 Certificate Authority

**Subject:**
```
/C=RU/ST=Moscow/L=Moscow/O=Private/OU=Private/CN=Titaev CA/emailAddress=titaev@gmail.com
```

**Местоположение:** Отдельная защищенная VM

**Файлы:**
- `ca.crt` - публичный сертификат (распространяется на все сервисы)
- `ca.key` - приватный ключ (защищен паролем, не покидает CA VM)

### 6.2 Server Certificates

**Template:**
```
Subject: /C=RU/ST=Moscow/L=Moscow/O=Private/OU=Services/CN=<service-name>
SAN: DNS:<dns-name>, DNS:<alt-name>, IP:<ip>
Key Usage: Digital Signature, Key Encipherment
Extended Key Usage: TLS Web Server Authentication
Validity: 365 days
```

**Примеры:**
```
# HSM Service
CN=hsm-service.local
SAN: DNS:hsm-service.local, DNS:localhost, IP:127.0.0.1

# MySQL Server
CN=mysql-server.local
SAN: DNS:mysql-server.local, DNS:mysql-master.local, IP:10.0.0.5

# ClickHouse Server
CN=clickhouse-server.local
SAN: DNS:clickhouse-server.local, IP:10.0.0.6
```

### 6.3 Client Certificates

**Template:**
```
Subject: /C=RU/ST=Moscow/L=Moscow/O=Private/OU=<OU>/CN=<client-name>
Key Usage: Digital Signature
Extended Key Usage: TLS Web Client Authentication
Validity: 365 days
```

**OU определяет группу доступа:**

| OU       | Purpose                  |
|----------|--------------------------|
| Trading  | Торговые сервисы         |
| 2FA      | 2FA сервисы              |
| Database | MySQL/ClickHouse клиенты |

**Примеры:**
```
# Trading service
CN=trading-service-1, OU=Trading

# 2FA service
CN=web-2fa-service, OU=2FA

# MySQL client
CN=backend-app-1, OU=Database
```

### 6.4 Revocation Process

**Файл: pki/revoked.yaml**

```yaml
revoked:
  - cn: "compromised-service"
    serial: "05"
    revoked_date: "2026-01-03T10:00:00Z"
    reason: "compromised | decommissioned | superseded"
```

**Процесс отзыва:**

1. Добавить CN в `pki/revoked.yaml`
2. Перезапустить HSM service (или SIGHUP для hot reload)
3. Сертификат больше не может подключиться (403 Forbidden)

**Скрипт отзыва:**

```bash
./pki/scripts/revoke-cert.sh <cn> <reason>
# Автоматически обновляет revoked.yaml
```

---

## 7. CLI Utility (hsm-admin)

### 7.1 Назначение

CLI утилита для управления KEK в SoftHSM (локальное выполнение на HSM VM).

### 7.2 Команды

#### create-kek

Создает новый KEK в HSM.

```bash
hsm-admin create-kek --label <label> --context <context>

Flags:
  --label    KEK label (e.g., "kek-exchange-v2")
  --context  Context для ACL (e.g., "exchange-key")
  --pin      HSM PIN (или через ENV HSM_PIN)

Example:
  hsm-admin create-kek --label kek-exchange-v2 --context exchange-key
```

**Output:**
```
✓ KEK created successfully
  Label: kek-exchange-v2
  Context: exchange-key
  Handle: 0x12345678
  
Next steps:
1. Update config.yaml to activate this KEK
2. Restart HSM service
```

#### list-kek

Список всех KEK в токене.

```bash
hsm-admin list-kek

Example output:
┌──────────────────┬──────────────┬────────┬───────────┐
│ Label            │ Context      │ Active │ Handle    │
├──────────────────┼──────────────┼────────┼───────────┤
│ kek-exchange-v1  │ exchange-key │ Yes    │ 0x1234... │
│ kek-exchange-v2  │ exchange-key │ No     │ 0x5678... │
│ kek-2fa-v1       │ 2fa          │ Yes    │ 0xABCD... │
└──────────────────┴──────────────┴────────┴───────────┘
```

#### delete-kek

Удаляет KEK из токена (только если нет зависимостей).

```bash
hsm-admin delete-kek --label <label> --confirm

Flags:
  --label    KEK label to delete
  --confirm  Required flag для подтверждения

Example:
  hsm-admin delete-kek --label kek-exchange-v1 --confirm
```

**Validation:**
- Проверить, что KEK не активен в config.yaml
- Требовать `--confirm` для безопасности

#### export-metadata

Экспорт метаданных KEK для аудита (без приватных данных).

```bash
hsm-admin export-metadata --output <file>

Example:
  hsm-admin export-metadata --output kek-inventory.json
```

**Output (JSON):**
```json
{
  "timestamp": "2026-01-05T12:00:00Z",
  "token": "hsm-token",
  "keys": [
    {
      "label": "kek-exchange-v1",
      "context": "exchange-key",
      "algorithm": "AES-256-GCM",
      "extractable": false,
      "created": "2026-01-01T00:00:00Z"
    }
  ]
}
```

---

## 8. Deployment

### 8.1 Docker Setup

**Dockerfile:**

```dockerfile
FROM golang:1.21-alpine AS builder
RUN apk add --no-cache softhsm openssl build-base
WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN go build -o hsm-service .

FROM alpine:latest
RUN apk add --no-cache softhsm openssl
WORKDIR /app
COPY --from=builder /app/hsm-service .
COPY softhsm2.conf /etc/softhsm/softhsm2.conf
COPY scripts/init-hsm.sh /app/init-hsm.sh
RUN chmod +x /app/init-hsm.sh
ENTRYPOINT ["/app/init-hsm.sh"]
```

**docker-compose.yml:**

```yaml
version: '3.8'
services:
  hsm-service:
    build: .
    ports:
      - "8443:8443"
    environment:
      - HSM_PIN=${HSM_PIN}
      - HSM_SO_PIN=${HSM_SO_PIN}
    volumes:
      - ./data/tokens:/var/lib/softhsm/tokens
      - ./pki:/app/pki:ro
      - ./config.yaml:/app/config.yaml:ro
    restart: unless-stopped
```

### 8.2 Production Deployment

**VM Requirements:**
- OS: Ubuntu 22.04 LTS
- RAM: 4GB minimum
- CPU: 2 cores
- Disk: 20GB
- Network: Internal VLAN (не публичный интернет)

**Security Hardening:**
- Firewall: только порт 8443 от известных IP
- SELinux / AppArmor enabled
- SSH: только ключи, no root login
- Audit logging → SIEM
- Регулярные backups SoftHSM token

**Systemd Service:**

```ini
[Unit]
Description=HSM Service
After=network.target

[Service]
Type=simple
User=hsm
Group=hsm
WorkingDirectory=/opt/hsm-service
Environment="HSM_PIN=<from_vault>"
ExecStart=/opt/hsm-service/hsm-service
Restart=on-failure
RestartSec=5s

[Install]
WantedBy=multi-user.target
```

---

## 9. Testing Strategy

### 9.1 Unit Tests

**Coverage targets:**
- config/: 80%
- hsm/crypto.go: 90%
- server/acl.go: 95%
- server/handlers.go: 85%

**Mocking:**
- PKCS#11: использовать mock HSM для unit tests
- TLS certificates: генерировать временные тестовые сертификаты

### 9.2 Integration Tests

**Scenarios:**
1. Успешный encrypt → decrypt round-trip
2. ACL блокировка неправильного OU
3. Revoked certificate rejection
4. Rate limiting enforcement
5. AAD mismatch detection

### 9.3 Security Tests

**Penetration testing:**
- Invalid certificates
- Expired certificates
- Certificate spoofing
- Replay attacks
- Context confusion attacks

---

## 10. Acceptance Criteria

### 10.1 Phase 1: MVP

- [ ] POST /encrypt работает с двумя contexts (exchange-key, 2fa)
- [ ] POST /decrypt корректно расшифровывает
- [ ] mTLS аутентификация обязательна
- [ ] ACL по OU работает
- [ ] revoked.yaml блокирует отозванные сертификаты
- [ ] Rate limiting защищает от abuse
- [ ] GET /health возвращает статус
- [ ] Audit logging всех операций
- [ ] CLI утилита: create-kek, list-kek, delete-kek
- [ ] Docker setup для разработки
- [ ] PKI скрипты для выпуска сертификатов
- [ ] Документация: README, ARCHITECTURE, TECHNICAL_SPEC

### 10.2 Phase 2: Production Readiness

- [ ] Prometheus metrics
- [ ] Hot reload для revoked.yaml (SIGHUP)
- [ ] Graceful shutdown
- [ ] SoftHSM token backup/restore процедуры
- [ ] KEK rotation playbook
- [ ] HA deployment guide (active-passive)
- [ ] Load testing (1000+ req/s)
- [ ] Security audit

---

## 11. Glossary

- **KEK** - Key Encryption Key
- **DEK** - Data Encryption Key
- **HSM** - Hardware Security Module
- **mTLS** - Mutual TLS
- **AAD** - Additional Authenticated Data
- **OU** - Organizational Unit
- **CN** - Common Name
- **SAN** - Subject Alternative Name
- **GCM** - Galois/Counter Mode
- **PKCS#11** - Cryptographic Token Interface Standard

---

## 12. Appendix

### 12.1 Example Request/Response

**Encrypt Request (trading-service-1):**

```bash
curl -X POST https://hsm-service.local:8443/encrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{
    "context": "exchange-key",
    "plaintext": "bXktc2VjcmV0LWRlaw=="
  }'

Response:
{
  "ciphertext": "nonce+encrypted+tag в base64...",
  "key_id": "kek-exchange-v1"
}
```

**Decrypt Request:**

```bash
curl -X POST https://hsm-service.local:8443/decrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{
    "context": "exchange-key",
    "ciphertext": "nonce+encrypted+tag в base64...",
    "key_id": "kek-exchange-v1"
  }'

Response:
{
  "plaintext": "bXktc2VjcmV0LWRlaw=="
}
```

---

**END OF TECHNICAL SPECIFICATION**
