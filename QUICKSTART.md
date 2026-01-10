# 🚀 HSM Service - Быстрый старт

> **Цель**: Запустить HSM Service и выполнить первый тестовый запрос за 5 минут

## Предварительные требования

Перед началом у вас должно быть:
- ✅ Docker + Docker Compose установлены
- ✅ **Собственный CA (Certificate Authority)** - ключи и сертификат готовы
- ✅ CA файлы скопированы в `pki/ca/ca.key` и `pki/ca/ca.crt`

**Если у вас НЕТ CA**, создайте его:
```bash
# Создать приватный ключ CA (4096 бит, защищенный паролем)
openssl genrsa -aes256 -out ca.key 4096

# Создать самоподписанный сертификат CA (10 лет)
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=YourCompany/OU=Security/CN=HSM-CA"

# Скопировать в проект
cp ca.key pki/ca/
cp ca.crt pki/ca/
chmod 600 pki/ca/ca.key
chmod 644 pki/ca/ca.crt
```

---

## Шаг 1: Подготовка проекта

```bash
# Клонировать репозиторий
git clone <repository-url>
cd hsm-service

# Проверить что CA на месте
ls -la pki/ca/ca.key pki/ca/ca.crt
```

**Ожидаемый вывод**:
```
-rw------- 1 user user 3243 Jan 10 12:00 pki/ca/ca.key
-rw-r--r-- 1 user user 1891 Jan 10 12:00 pki/ca/ca.crt
```

---

## Шаг 2: Генерация сертификатов

### 2.1. Серверный сертификат для HSM Service

```bash
./pki/scripts/issue-server-cert.sh hsm-service.local
```

**Что происходит**:
- Создается приватный ключ сервера (`pki/server/hsm-service.local.key`)
- Генерируется CSR (Certificate Signing Request)
- CA подписывает сертификат → `pki/server/hsm-service.local.crt`

**Проверка**:
```bash
ls -la pki/server/hsm-service.local.*
openssl x509 -in pki/server/hsm-service.local.crt -noout -subject -dates
```

### 2.2. Клиентские сертификаты

```bash
# Для Trading сервиса
./pki/scripts/issue-client-cert.sh trading-service-1 Trading

# Для 2FA сервиса (опционально)
./pki/scripts/issue-client-cert.sh 2fa-service-1 2FA
```

**Что создается**:
- `pki/client/trading-service-1.key` - приватный ключ клиента
- `pki/client/trading-service-1.crt` - сертификат с OU=Trading

**⚠️ Важно**: OU (Organizational Unit) определяет ACL доступ!

**Проверка OU**:
```bash
openssl x509 -in pki/client/trading-service-1.crt -noout -subject
# Должно быть: subject=CN=trading-service-1,OU=Trading,O=...
```

---

## Шаг 3: Конфигурация metadata.yaml

```bash
# Создать metadata.yaml из шаблона
cp metadata.yaml.example metadata.yaml
```

**Файл metadata.yaml** содержит динамические метаданные ключей (версии, timestamps). Обновляется автоматически при ротации.

**Первоначальная структура** (создается автоматически при init-keys):
```yaml
rotation:
  exchange-key:
    current: kek-exchange-v1
    versions:
      - label: kek-exchange-v1
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

## Шаг 4: Запуск HSM Service

```bash
# Запустить Docker Compose
docker-compose up -d

# Проверить что контейнер запустился
docker-compose ps
```

**Ожидаемый вывод**:
```
NAME           IMAGE          STATUS         PORTS
hsm-service    hsm-service    Up 5 seconds   0.0.0.0:8443->8443/tcp
```

**Проверить логи**:
```bash
docker-compose logs hsm-service
```

**Ожидаемые логи**:
```
INFO  Initializing SoftHSM token...
INFO  Loading KEK from inventory...
INFO  Starting HSM service on port 8443
INFO  started revoked.yaml auto-reload interval=30s
INFO  Loaded 2 KEKs: [kek-exchange-v1 kek-2fa-v1]
```

---

## Шаг 5: Инициализация KEK (первый запуск)

```bash
# Создать KEK ключи в SoftHSM
docker exec hsm-service /app/hsm-admin init-keys
```

**Что происходит**:
- Читается `pki/inventory.yaml` (список всех KEK)
- Для каждого context создается AES-256 ключ в SoftHSM
- Ключи маркируются как non-extractable (не экспортируемые)

**Ожидаемый вывод**:
```
Initializing KEK keys from inventory...
✓ Created kek-exchange-v1 (AES-256, context: exchange-key)
✓ Created kek-2fa-v1 (AES-256, context: 2fa)
✓ Updated metadata.yaml
Done! Initialized 2 KEK keys.
```

---

## Шаг 6: Первый тестовый запрос

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

**❌ Если ошибка**:
- `curl: (60) SSL certificate problem` → проверьте что CA сертификат правильный
- `curl: (35) error:14094410:SSL` → проверьте mTLS сертификаты
- `Connection refused` → проверьте `docker-compose ps`

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
  "ciphertext": "base64_encrypted_data_here...",
  "key_id": "kek-exchange-v1"
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
    "key_id": "kek-exchange-v1"
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
- `/encrypt`, `/decrypt` - базовые операции
- `/rotate/:context` - ротация ключей
- `/revoke` - отзыв сертификатов
- `/metrics` - Prometheus метрики

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
./scripts/full-integration-test.sh

# Подробнее в TEST_PLAN.md
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
docker-compose logs hsm-service

# Пересобрать образ
docker-compose up -d --build

# Проверить права на директории
ls -la data/tokens/
chmod 755 data/tokens/
```

---

## 💡 Полезные команды

```bash
# === Docker управление ===
docker-compose up -d              # Запустить
docker-compose down               # Остановить
docker-compose logs -f            # Логи в реальном времени
docker-compose restart            # Перезапустить

# === hsm-admin CLI ===
docker exec hsm-service /app/hsm-admin list-kek       # Список KEK
docker exec hsm-service /app/hsm-admin rotate exchange-key  # Ротация
docker exec hsm-service /app/hsm-admin revoke-cert trading-service-1  # Отзыв

# === Просмотр PKI ===
openssl x509 -in pki/ca/ca.crt -noout -text           # CA сертификат
openssl x509 -in pki/client/trading-service-1.crt -noout -subject -dates
openssl x509 -in pki/server/hsm-service.local.crt -noout -subject -dates

# === Метрики ===
curl -k https://localhost:8443/metrics \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
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
| [TEST_PLAN.md](TEST_PLAN.md) | Тестирование (Unit, E2E, Security) |

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
  "key_id": "kek-exchange-v1"
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
    "key_id": "kek-exchange-v1"
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

---

## 🎉 Поздравляю!

Вы успешно:
- ✅ Запустили HSM Service
- ✅ Зашифровали данные
- ✅ Расшифровали данные

---

## Что дальше?

### Для backend разработчиков
📖 Читайте [API.md](API.md) - полная документация API

### Для DevOps инженеров
🔧 Читайте [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) - развертывание на production

### Для security инженеров
🔒 Читайте [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - аудит безопасности

---

## ❓ Часто задаваемые вопросы

### Q: Почему используется `-k` в curl?
A: Это отключает проверку сертификата сервера (для тестирования). На production используйте валидный сертификат и уберите `-k`.

### Q: Что такое base64 в plaintext?
A: HSM Service принимает данные в base64 для безопасной передачи бинарных данных через JSON.

Конвертация:
```bash
# Encode
echo -n "Hello World!" | base64
# SGVsbG8gV29ybGQh

# Decode
echo "SGVsbG8gV29ybGQh" | base64 -d
# Hello World!
```

### Q: Сертификат недействителен, ошибка SSL
A: Убедитесь что:
1. PKI сгенерирован: `ls pki/ca/ca.crt` должен существовать
2. Docker монтирует pki корректно: `docker-compose exec hsm-service ls /app/pki`
3. Сертификат не истек: `openssl x509 -in pki/client/trading-service-1.crt -noout -dates`

### Q: Permission denied на data/tokens
A: Исправьте права:
```bash
chmod 755 data/tokens
```

### Q: HSM_PIN неверный
A: Проверьте .env файл и перезапустите:
```bash
cat .env | grep HSM_PIN
docker-compose down
docker-compose up -d
```

---

## 🐛 Troubleshooting

### Проблема: Container не запускается

```bash
# Проверить логи
docker-compose logs hsm-service

# Проверить что образ собрался
docker images | grep hsm-service

# Пересобрать образ
docker-compose up -d --build
```

### Проблема: 403 Forbidden при запросе

**Причина**: OU в сертификате не имеет доступа к context

**Решение**: Проверьте ACL в config.yaml
```yaml
acl:
  mappings:
    Trading:           # OU в сертификате
      - exchange-key   # Разрешенные contexts
```

Проверьте OU в сертификате:
```bash
openssl x509 -in pki/client/trading-service-1.crt -noout -subject
# subject=CN=trading-service-1,OU=Trading,O=Example Corp
```

### Проблема: Connection refused

```bash
# Проверьте что контейнер запущен
docker-compose ps

# Проверьте порты
docker-compose ps | grep 8443

# Проверьте логи сервера
docker-compose logs hsm-service | grep "Starting"
```

---

## 📚 Следующие шаги

1. **API Reference**: [API.md](API.md)
2. **Архитектура**: [ARCHITECTURE.md](ARCHITECTURE.md)  
3. **Ротация ключей**: [KEY_ROTATION.md](KEY_ROTATION.md)
4. **Production setup**: [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md)

---

## 💡 Полезные команды

```bash
# Остановить сервис
docker-compose down

# Перезапустить с пересборкой
docker-compose up -d --build

# Посмотреть логи в реальном времени
docker-compose logs -f hsm-service

# Зайти в контейнер
docker-compose exec hsm-service sh

# Список KEK ключей
docker-compose exec hsm-service /app/hsm-admin list-kek

# Ротация ключа
docker-compose exec hsm-service /app/hsm-admin rotate exchange-key

# Посмотреть метрики Prometheus
curl -k https://localhost:8443/metrics \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt
```

**Готово!** Теперь у вас работающий HSM Service 🎊
