# 🚀 HSM Service - Быстрый старт

> **Цель**: Запустить HSM Service и сделать первый тестовый запрос за 5 минут

## Что вы получите

После выполнения этих шагов у вас будет:
- ✅ Запущенный HSM Service на https://localhost:8443
- ✅ PKI с тестовыми сертификатами
- ✅ Два KEK ключа (exchange-key и 2fa)
- ✅ Рабочий пример шифрования/расшифрования

---

## Быстрый старт (Docker)

### Шаг 1: Клонировать репозиторий

```bash
git clone <repository-url>
cd hsm-service
```

### Шаг 2: Запустить PKI генерацию

```bash
cd pki
./scripts/generate-all.sh
cd ..
```

**Что происходит**: Создаются CA сертификат, серверный сертификат и клиентские сертификаты для тестирования.

### Шаг 3: Создать .env файл

```bash
cp .env.example .env
```

**Файл .env содержит**:
```bash
HSM_PIN=1234                    # PIN для SoftHSM (измените для продакшена!)
SLOT_LABEL=hsm-token           # Имя токена
```

### Шаг 4: Запустить Docker Compose

```bash
docker-compose up -d
```

**Что происходит**:
1. Собирается Docker образ с Go приложением
2. Инициализируется SoftHSM токен
3. Создаются KEK ключи (exchange-key, 2fa)
4. Запускается HTTPS сервер на порту 8443

### Шаг 5: Проверить что запустилось

```bash
docker-compose ps
docker-compose logs hsm-service
```

**Ожидаемый вывод**:
```
INFO  Starting HSM service on port 8443
INFO  started revoked.yaml auto-reload interval=30s
INFO  Loaded 2 KEKs: [kek-exchange-v1 kek-2fa-v1]
```

---

## Первый тестовый запрос

### 1. Health Check (проверка что сервис живой)

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
  "active_keys": ["kek-exchange-v1", "kek-2fa-v1"]
}
```

### 2. Шифрование (Encrypt)

```bash
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
