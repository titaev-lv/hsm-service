# HSM Service - Quick Start

## Сборка

```bash
go build -o hsm-service .
```

## Запуск

### 1. Убедитесь что config.yaml и metadata.yaml настроены

**config.yaml:**

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

**metadata.yaml:**

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

### 2. Установите переменную окружения HSM_PIN

```bash
export HSM_PIN="your-hsm-pin"
```

### 3. Запустите сервис

```bash
./hsm-service
```

Вывод:
```
2026/01/07 00:30:00 Starting HSM service on port 8443
```

## Graceful Shutdown

Нажмите `Ctrl+C` или отправьте SIGTERM:

```bash
kill -TERM <pid>
```

Вывод:
```
2026/01/07 00:31:00 Received signal interrupt, shutting down gracefully...
2026/01/07 00:31:00 HSM service stopped
```

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
