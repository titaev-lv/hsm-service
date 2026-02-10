# HSM Service - Quick Start (Docker)

> **Цель**: Запустить HSM Service в Docker за 2 минуты с одной командой

## ⚡ Быстрый старт (рекомендуется)

```bash
# Клонировать репозиторий
git clone <repository-url>
cd hsm-service

# Запустить скрипт инициализации (делает всё автоматически!)
chmod +x init-pki-docker.sh
./init-pki-docker.sh
```

**Что произойдет автоматически:**
- ✅ Генерирует Root CA (центр сертификации)
- ✅ Генерирует серверный сертификат для HSM Service
- ✅ Генерирует клиентские сертификаты (Trading, 2FA)
- ✅ Копирует metadata.yaml из примера
- ✅ Собирает Docker образ
- ✅ Запускает контейнер
- ✅ Проверяет здоровье сервиса

**После выполнения:**
```
🎉 Initialization Complete!
✓ HSM Service is ready for development!
```

Дальше переходите к [Проверке API](#проверка-api)

---

## Расширенные опции

Если нужно переустановить с нуля:
```bash
./init-pki-docker.sh --force    # Переустановить всё (CA, сертификаты, Docker)
```

Если нужна только PKI без Docker:
```bash
./init-pki-docker.sh --skip-docker    # Только генерирует сертификаты
```

---

## 📖 Пошаговый старт (если что-то не сработало)

Если `init-pki-docker.sh` не сработал, выполняйте шаги вручную:

### Шаг 1: Подготовка проекта

```bash
# Клонировать репозиторий
git clone <repository-url>
cd hsm-service

# Проверить что все файлы на месте
ls -la config.yaml
ls -la metadata.yaml.example
ls -la docker-compose.yml
ls -la Dockerfile
```

### Шаг 2: Генерирование PKI инфраструктуры

```bash
# Создать директории для сертификатов
mkdir -p pki/ca pki/server pki/client

# Генерировать Root CA (самоподписанный)
openssl req -x509 -newkey rsa:4096 -keyout pki/ca/ca.key \
  -out pki/ca/ca.crt -days 3650 -nodes \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=HSM-Dev/CN=hsm-ca"

# Генерировать серверный сертификат для HSM Service
openssl genrsa -out pki/server/hsm-service.local.key 4096
openssl req -new -key pki/server/hsm-service.local.key \
  -out pki/server/hsm-service.local.csr \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=HSM-Dev/CN=hsm-service.local"
openssl x509 -req -in pki/server/hsm-service.local.csr \
  -CA pki/ca/ca.crt -CAkey pki/ca/ca.key -CAcreateserial \
  -out pki/server/hsm-service.local.crt -days 825 \
  -extfile <(echo "subjectAltName=DNS:localhost,DNS:hsm-service,DNS:hsm-service.local")

# Генерировать клиентские сертификаты
for CLIENT in trading-service-1 2fa-service-1; do
  openssl genrsa -out pki/client/$CLIENT.key 4096
  openssl req -new -key pki/client/$CLIENT.key \
    -out pki/client/$CLIENT.csr \
    -subj "/C=RU/ST=Moscow/L=Moscow/O=HSM-Dev/CN=$CLIENT"
  openssl x509 -req -in pki/client/$CLIENT.csr \
    -CA pki/ca/ca.crt -CAkey pki/ca/ca.key -CAcreateserial \
    -out pki/client/$CLIENT.crt -days 825
done

# Cleanup CSR файлы
rm -f pki/server/*.csr pki/client/*.csr pki/ca/*.srl
```

**Проверить что всё создалось:**
```bash
ls -la pki/ca/
ls -la pki/server/
ls -la pki/client/
```

### Шаг 3: Создание metadata.yaml

**Важно:** Перед запуском контейнера нужно создать файл `metadata.yaml` с базовой структурой, иначе Docker создаст **директорию** вместо файла.

```bash
# Создать metadata.yaml из примера
cp metadata.yaml.example metadata.yaml
```

### Шаг 4: Запуск Docker контейнера

```bash
# Собрать Docker образ и запустить
mkdir -p logs
docker compose up -d --build

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

---

## Проверка API

### Health Check

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

### Шифрование (Encrypt)

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

**Ожидаемый ответ**:
```json
{
  "ciphertext": "plpmmI0StauF6ZWGfEnrlxom23Zt8wS1yPkqTCxgQykMRAkYhgZfLKprYzM=",
  "key_id": "kek-exchange-key-v1"
}
```

### Расшифрование (Decrypt)

```bash
curl -k -X POST https://localhost:8443/decrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{
    "context": "exchange-key",
    "ciphertext": "плпммI0StauF6ZWGfEnrlxom23Zt8wS1yPkqTCxgQykMRAkYhgZfLKprYzM=",
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

# === Просмотр PKI ===
openssl x509 -in pki/ca/ca.crt -noout -text           # CA сертификат
openssl x509 -in pki/client/trading-service-1.crt -noout -subject -dates  # Клиентский сертификат
openssl x509 -in pki/server/hsm-service.local.crt -noout -subject -dates      # Серверный сертификат
```

---

## ❓ Troubleshooting

### ❌ Docker контейнер не запускается

```bash
# Проверить логи
docker compose logs hsm-service

# Пересобрать образ
docker compose up -d --build

# Проверить права на директории
ls -la data/tokens/
chmod 755 data/tokens/

# Проверить права на директорию логов
ls -la logs/
chmod 750 logs/
```

### ❌ Ошибка: "OU not authorized"

**Проблема**: Клиентский сертификат с неверным OU пытается получить доступ

**Решение**: Проверьте ACL в `config.yaml` и убедитесь, что сертификаты имеют правильные OU

### ❌ Ошибка: "Certificate verification failed"

```bash
# Проверить что CA cert существует
ls -la pki/ca/ca.crt

# Проверить что клиентский сертификат подписан CA
openssl verify -CAfile pki/ca/ca.crt pki/client/trading-service-1.crt
```

---

## 📚 Дополнительная документация

| Документ | Описание |
|----------|----------|
| [README.md](README.md) | Обзор проекта, use cases, compliance |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Архитектура и компоненты |
| [API.md](API.md) | Полная API reference |
| [MONITORING.md](MONITORING.md) | Prometheus + Grafana setup |
| [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) | Production deployment |
| [CLI_TOOLS.md](CLI_TOOLS.md) | hsm-admin command reference |

---

**Готово!** Ваш HSM Service запущен и готов к работе 🚀
