# 📡 HSM Service - API Reference

> **Простым языком**: Как отправлять запросы к HSM Service для шифрования и расшифрования данных

## Базовая информация

- **Базовый URL**: `https://localhost:8443` (dev) или `https://hsm.example.com` (prod)
- **Протокол**: HTTPS only (TLS 1.3)
- **Аутентификация**: mTLS (обязателен клиентский сертификат)
- **Формат данных**: JSON
- **Кодировка**: UTF-8
- **Бинарные данные**: Base64

**Request ID (корреляция):**
- Клиент может передать заголовок `X-Request-ID`.
- Если заголовок отсутствует, сервис сгенерирует его сам.
- Значение возвращается в ответе и пишется в audit/error логи.

---

## Endpoints

| Метод | Endpoint | Описание |
|-------|----------|----------|
| POST | `/encrypt` | Зашифровать данные |
| POST | `/decrypt` | Расшифровать данные |
| GET  | `/health` | Проверка здоровья сервиса |
| GET  | `/metrics` | Prometheus метрики |

---

## 1. POST /encrypt

Шифрует данные используя KEK из HSM.

### Request

```http
POST /encrypt HTTP/1.1
Host: localhost:8443
Content-Type: application/json

{
  "context": "exchange-key",
  "plaintext": "SGVsbG8gV29ybGQh"
}
```

### Параметры

| Поле | Тип | Обязательный | Описание |
|------|-----|--------------|----------|
| `context` | string | ✅ Да | Имя контекста KEK (exchange-key, 2fa) |
| `plaintext` | string | ✅ Да | Данные в base64 для шифрования |

**Важно**: 
- `context` должен быть разрешен для OU вашего сертификата (см. ACL в config.yaml)
- `plaintext` ОБЯЗАТЕЛЬНО в base64
- Максимальный размер данных: ~4KB (ограничение GCM)

### Response (Success 200)

```json
{
  "ciphertext": "AQIDBAgAAAAAAAAAAAAAAAAAAAAAAAA...",
  "key_id": "kek-exchange-key-v1"
}
```

| Поле | Тип | Описание |
|------|-----|----------|
| `ciphertext` | string | Зашифрованные данные в base64 |
| `key_id` | string | ID KEK которым зашифровано (для decrypt) |

**Формат ciphertext**:
```
[version:1 byte][nonce:12 bytes][tag:16 bytes][encrypted_data]
```
Все в base64.

### Errors

#### 400 Bad Request - Неверный JSON
```json
{
  "error": "invalid JSON in request"
}
```

#### 400 Bad Request - Неверный base64
```json
{
  "error": "invalid base64 plaintext"
}
```

#### 403 Forbidden - Нет доступа к context
```json
{
  "error": "access denied: insufficient permissions"
}
```

#### 403 Forbidden - Сертификат отозван
```json
{
  "error": "certificate revoked"
}
```

#### 429 Too Many Requests - Rate limit
```json
{
  "error": "rate limit exceeded"
}
```
**Headers**: `Retry-After: 1`

#### 500 Internal Server Error - HSM ошибка
```json
{
  "error": "encryption failed"
}
```

### Пример (curl)

```bash
curl -X POST https://localhost:8443/encrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{
    "context": "exchange-key",
    "plaintext": "SGVsbG8gV29ybGQh"
  }'
```

### Пример (Python)

```python
import requests
import base64

# Данные для шифрования
plaintext = b"Hello World!"
plaintext_b64 = base64.b64encode(plaintext).decode('utf-8')

response = requests.post(
    'https://localhost:8443/encrypt',
    cert=('pki/client/trading-service-1.crt', 
          'pki/client/trading-service-1.key'),
    verify='pki/ca/ca.crt',
    json={
        'context': 'exchange-key',
        'plaintext': plaintext_b64
    }
)

if response.status_code == 200:
    data = response.json()
    ciphertext = data['ciphertext']
    key_id = data['key_id']
    print(f"Encrypted with {key_id}")
    print(f"Ciphertext: {ciphertext[:50]}...")
else:
    print(f"Error {response.status_code}: {response.json()}")
```

### Пример (Go)

```go
package main

import (
    "bytes"
    "crypto/tls"
    "crypto/x509"
    "encoding/base64"
    "encoding/json"
    "fmt"
    "io/ioutil"
    "net/http"
)

func main() {
    // Load CA cert
    caCert, _ := ioutil.ReadFile("pki/ca/ca.crt")
    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)

    // Load client cert
    clientCert, _ := tls.LoadX509KeyPair(
        "pki/client/trading-service-1.crt",
        "pki/client/trading-service-1.key",
    )

    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{clientCert},
        RootCAs:      caCertPool,
    }

    client := &http.Client{
        Transport: &http.Transport{TLSClientConfig: tlsConfig},
    }

    // Prepare request
    plaintext := base64.StdEncoding.EncodeToString([]byte("Hello World!"))
    reqBody, _ := json.Marshal(map[string]string{
        "context":   "exchange-key",
        "plaintext": plaintext,
    })

    resp, err := client.Post(
        "https://localhost:8443/encrypt",
        "application/json",
        bytes.NewBuffer(reqBody),
    )
    if err != nil {
        panic(err)
    }
    defer resp.Body.Close()

    var result map[string]string
    json.NewDecoder(resp.Body).Decode(&result)
    
    fmt.Printf("Ciphertext: %s\n", result["ciphertext"])
    fmt.Printf("Key ID: %s\n", result["key_id"])
}
```

---

## 2. POST /decrypt

Расшифровывает данные используя KEK из HSM.

### Request

```http
POST /decrypt HTTP/1.1
Host: localhost:8443
Content-Type: application/json

{
  "context": "exchange-key",
  "ciphertext": "AQIDBAgAAAAAAAAAAAAAAAAAAAAAAAA...",
  "key_id": "kek-exchange-key-v1"
}
```

### Параметры

| Поле | Тип | Обязательный | Описание |
|------|-----|--------------|----------|
| `context` | string | ✅ Да | Имя контекста KEK |
| `ciphertext` | string | ✅ Да | Зашифрованные данные в base64 |
| `key_id` | string | ✅ Да | ID KEK (из /encrypt response) |

**Важно**: 
- `context` должен совпадать с тем что использовался при encrypt
- `key_id` должен существовать в HSM (даже старые версии после ротации)

### Response (Success 200)

```json
{
  "plaintext": "SGVsbG8gV29ybGQh"
}
```

| Поле | Тип | Описание |
|------|-----|----------|
| `plaintext` | string | Расшифрованные данные в base64 |

### Errors

Аналогичны `/encrypt` + дополнительно:

#### 400 Bad Request - Неверный ciphertext
```json
{
  "error": "invalid base64 ciphertext"
}
```

#### 400 Bad Request - Key ID не найден
```json
{
  "error": "key not found"
}
```

#### 500 Internal Server Error - Расшифрование failed
```json
{
  "error": "decryption failed"
}
```

**Возможные причины**:
- Ciphertext поврежден
- Использован неправильный context
- AAD не совпадает (context или CN изменились)

### Пример (curl)

```bash
curl -X POST https://localhost:8443/decrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{
    "context": "exchange-key",
    "ciphertext": "AQIDBAgAAAAAAAAAAAAAAAAAAAAAAAA...",
    "key_id": "kek-exchange-key-v1"
  }'
```

### Пример (Python)

```python
response = requests.post(
    'https://localhost:8443/decrypt',
    cert=('pki/client/trading-service-1.crt', 
          'pki/client/trading-service-1.key'),
    verify='pki/ca/ca.crt',
    json={
        'context': 'exchange-key',
        'ciphertext': ciphertext,  # From encrypt response
        'key_id': 'kek-exchange-key-v1'
    }
)

if response.status_code == 200:
    plaintext_b64 = response.json()['plaintext']
    plaintext = base64.b64decode(plaintext_b64)
    print(f"Decrypted: {plaintext.decode('utf-8')}")
```

---

## 3. GET /health

Проверка здоровья сервиса.

### Request

```http
GET /health HTTP/1.1
Host: localhost:8443
```

**Требуется mTLS**: ✅ Да (любой валидный сертификат)

### Response (Success 200)

```json
{
  "status": "healthy",
  "hsm_available": true,
  "kek_status": {
    "kek-exchange-key-v1": "available",
    "kek-2fa-v1": "available"
  }
}
```

| Поле | Тип | Описание |
|------|-----|----------|
| `status` | string | Общий статус: `healthy` или `degraded` |
| `hsm_available` | boolean | HSM доступен (`true`) или нет (`false`) |
| `kek_status` | object | Статус каждого KEK: `"available"` или `"unavailable"` |

### Response (Degraded 503)

```json
{
  "status": "degraded",
  "hsm_available": false,
  "kek_status": {
    "kek-exchange-key-v1": "available",
    "kek-2fa-v1": "unavailable"
  }
}
```

**Примечание:** Если хотя бы один KEK недоступен, статус становится `degraded` и возвращается HTTP 503.

### Пример (curl)

```bash
curl https://localhost:8443/health \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt
```

**Использование**: Kubernetes liveness/readiness probes, мониторинг

---

## 4. GET /metrics

Prometheus метрики в формате OpenMetrics.

### Request

```http
GET /metrics HTTP/1.1
Host: localhost:8443
```

**Требуется mTLS**: ✅ Да

### Response (Success 200)

```prometheus
# HELP hsm_requests_total Total HTTP requests
# TYPE hsm_requests_total counter
hsm_requests_total{endpoint="/encrypt",client_cn="trading-service-1",status="200"} 1523

# HELP hsm_encrypt_ops_total Total encrypt operations
# TYPE hsm_encrypt_ops_total counter
hsm_encrypt_ops_total{context="exchange-key",status="success"} 1520

# HELP hsm_request_duration_seconds Request duration histogram
# TYPE hsm_request_duration_seconds histogram
hsm_request_duration_seconds_bucket{endpoint="/encrypt",le="0.005"} 1200
hsm_request_duration_seconds_bucket{endpoint="/encrypt",le="0.01"} 1500
...
```

### Ключевые метрики

| Метрика | Тип | Описание |
|---------|-----|----------|
| `hsm_requests_total` | Counter | Всего HTTP запросов |
| `hsm_acl_failures_total` | Counter | ACL отказы (security!) |
| `hsm_revocation_failures_total` | Counter | Попытки с отозванными сертификатами |
| `hsm_encrypt_ops_total` | Counter | Операции шифрования |
| `hsm_decrypt_ops_total` | Counter | Операции расшифрования |
| `hsm_request_duration_seconds` | Histogram | Длительность запросов |
| `hsm_rate_limit_hits_total` | Counter | Rate limit срабатывания |
| `hsm_errors_total` | Counter | HSM ошибки |

Подробнее: [MONITORING.md](MONITORING.md)

---

## ACL (Access Control List)

### Как работает ACL

1. Сервис извлекает **OU** (Organizational Unit) из клиентского сертификата
2. Проверяет маппинг `OU → contexts` в config.yaml
3. Если context разрешен для OU → OK
4. Иначе → 403 Forbidden

### Пример конфигурации

```yaml
acl:
  mappings:
    Trading:           # OU=Trading
      - exchange-key   # Разрешен access к exchange-key
    2FA:              # OU=2FA
      - 2fa            # Разрешен access к 2fa
```

### Проверка OU в сертификате

```bash
openssl x509 -in pki/client/trading-service-1.crt -noout -subject
# subject=CN=trading-service-1,OU=Trading,O=Example Corp
```

### Что если OU не найден?

```http
HTTP/1.1 403 Forbidden
Content-Type: application/json

{
  "error": "access denied: unknown organizational unit"
}
```

---

## Certificate Revocation

### Auto-reload

Сервис автоматически перезагружает `pki/revoked.yaml` каждые **30 секунд**.

### Формат revoked.yaml

```yaml
revoked:
  - cn: "trading-service-1"
    serial: "1A:2B:3C:4D"
    reason: "key-compromise"
    date: "2026-01-09"
```

### Поведение

Если CN в revoked.yaml:
```http
HTTP/1.1 403 Forbidden

{
  "error": "certificate revoked"
}
```

Подробнее: [REVOCATION_RELOAD.md](REVOCATION_RELOAD.md)

---

## Rate Limiting

### Лимиты (по умолчанию)

- **Requests per second**: 100 req/s
- **Burst**: 50

### Поведение

Если превышен лимит:
```http
HTTP/1.1 429 Too Many Requests
Retry-After: 1

{
  "error": "rate limit exceeded"
}
```

### Per-client лимиты

Rate limit применяется **per CN** (Common Name из сертификата).

Пример:
- `trading-service-1` может делать 100 req/s
- `trading-service-2` может делать 100 req/s
- Независимо друг от друга

---

## Best Practices

### 1. Кэшируйте ciphertext

❌ **Плохо**: Шифровать одни и те же данные каждый раз
```python
for i in range(1000):
    encrypt(same_data)  # Waste of resources!
```

✅ **Хорошо**: Зашифровать один раз, сохранить ciphertext
```python
ciphertext = encrypt(data)
db.save(ciphertext)  # Reuse ciphertext
```

### 2. Обрабатывайте ошибки

```python
try:
    response = requests.post(url, json=payload, cert=cert)
    response.raise_for_status()
    return response.json()
except requests.exceptions.HTTPError as e:
    if e.response.status_code == 403:
        log.error("Access denied - check ACL configuration")
    elif e.response.status_code == 429:
        time.sleep(int(e.response.headers.get('Retry-After', 1)))
        # Retry
    else:
        raise
```

### 3. Используйте connection pooling

```python
session = requests.Session()
session.cert = ('client.crt', 'client.key')
session.verify = 'ca.crt'

# Reuse session for multiple requests
for data in batch:
    response = session.post(url, json={'plaintext': data})
```

### 4. Проверяйте key_id при decrypt

```python
# Save key_id with ciphertext
db.save({
    'ciphertext': ciphertext,
    'key_id': key_id,  # Important!
    'created_at': now()
})

# Use correct key_id when decrypting
decrypt_request = {
    'context': 'exchange-key',
    'ciphertext': record['ciphertext'],
    'key_id': record['key_id']  # Must match!
}
```

### 5. Мониторьте метрики

```python
# Check error rate
if error_rate > 0.01:  # 1% errors
    alert("HSM service degraded")

# Check latency
if p99_latency > 100ms:
    alert("HSM service slow")
```

---

## Security Considerations

### 1. TLS 1.3 Only

Сервис принимает **только TLS 1.3**. TLS 1.2 и ниже отклоняются.

### 2. mTLS Required

Каждый запрос **ОБЯЗАТЕЛЬНО** требует клиентский сертификат.

### 3. Data in Transit

Все данные шифруются TLS 1.3 при передаче.

### 4. Data at Rest

`ciphertext` безопасно хранить в базе данных:
- Шифрован KEK из HSM
- Включает authenticated encryption (GCM)
- AAD гарантирует context binding

### 5. Audit Logging

Все операции логируются:
```json
{
  "time": "2026-01-09T12:34:56Z",
  "level": "INFO",
  "path": "/encrypt",
  "client_cn": "trading-service-1",
  "client_ou": "Trading",
  "duration_ms": 5.2
}
```

---

## FAQ

### Q: Можно ли шифровать большие файлы?

**A**: Нет. HSM Service предназначен для шифрования **ключей** (DEK), не файлов.

Правильный паттерн:
```
1. Generate DEK locally (random 256-bit key)
2. Encrypt file with DEK (AES-GCM locally)
3. Encrypt DEK with HSM Service → ciphertext
4. Store: encrypted_file + ciphertext_dek
```

### Q: Что такое context?

**A**: `context` - это имя KEK в HSM. Примеры:
- `exchange-key` - для торговых систем
- `2fa` - для 2FA секретов
- `payment-keys` - для платежных данных

Настраивается в config.yaml.

### Q: Почему plaintext в base64?

**A**: JSON не поддерживает бинарные данные. Base64 - стандартный способ передачи бинарных данных через JSON.

### Q: Могу ли я расшифровать данные после ротации KEK?

**A**: Да! HSM хранит старые версии KEK. Используйте правильный `key_id`.

Подробнее: [KEY_ROTATION.md](KEY_ROTATION.md)

### Q: Что если HSM недоступен?

**A**: Сервис вернет 500 Internal Server Error. Используйте retry logic с exponential backoff.

### Q: Rate limit слишком низкий для меня

**A**: Измените в config.yaml:
```yaml
rate_limit:
  requests_per_second: 1000  # Increase
  burst: 200                 # Increase
```

---

## Примеры интеграции

### Node.js

```javascript
const axios = require('axios');
const https = require('https');
const fs = require('fs');

const agent = new https.Agent({
  cert: fs.readFileSync('pki/client/trading-service-1.crt'),
  key: fs.readFileSync('pki/client/trading-service-1.key'),
  ca: fs.readFileSync('pki/ca/ca.crt')
});

async function encrypt(plaintext) {
  const response = await axios.post('https://localhost:8443/encrypt', {
    context: 'exchange-key',
    plaintext: Buffer.from(plaintext).toString('base64')
  }, { httpsAgent: agent });
  
  return response.data;
}

// Usage
encrypt('Hello World!').then(data => {
  console.log('Ciphertext:', data.ciphertext);
  console.log('Key ID:', data.key_id);
});
```

### Java

```java
// TODO: Add Java example
```

---

## Changelog

| Версия | Дата | Изменения |
|--------|------|-----------|
| 1.0 | 2026-01-09 | Initial API documentation |

---

## Поддержка

Проблемы с API? 

1. Проверьте [TROUBLESHOOTING.md](TROUBLESHOOTING.md)
2. Email: titaev@.com
