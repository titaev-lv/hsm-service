# HSM Service

Enterprise-grade HSM (Hardware Security Module) Key Encryption Key (KEK) management service с поддержкой mTLS, автоматической ротации ключей и ACL.

## 🔐 Основные возможности

- **PKCS#11 Integration** - Работа с HSM через стандартный PKCS#11 интерфейс
- **Mutual TLS (mTLS)** - Двусторонняя аутентификация по клиентским сертификатам
- **ACL на основе OU** - Гранулярный контроль доступа через Organizational Unit
- **Автоматическая ротация KEK** - Политики ротации с метриками PCI DSS
- **Certificate Revocation** - Поддержка отзыва сертификатов
- **Audit Logging** - Полное логирование криптографических операций
- **Health Monitoring** - Готовые эндпоинты для мониторинга
- **CLI утилита** - hsm-admin для управления ключами

## 🏗️ Архитектура

### Разделение конфигурации

Сервис использует двухфайловую архитектуру конфигурации для совместимости с GitOps/IaC:

**config.yaml** (статическая конфигурация, в Git)
- Типы ключей и политики ротации
- ACL правила и маппинг OU → contexts
- Настройки сервера и HSM
- Монтируется read-only (`:ro`)

**metadata.yaml** (динамические метаданные, вне Git)
- **Текущая активная версия** (`current`) для каждого контекста
- **Массив всех версий** (`versions`) - поддержка overlap period
- Временные метки создания и номера версий
- Обновляется автоматически при ротации
- Монтируется read-write (`:rw`)

Пример структуры metadata.yaml:
```yaml
rotation:
  exchange-key:
    current: kek-exchange-v2      # Активная версия для новых операций
    versions:
      - label: kek-exchange-v1    # Старая версия (для расшифровки)
        version: 1
        created_at: '2026-01-09T00:00:00Z'
      - label: kek-exchange-v2    # Новая версия
        version: 2
        created_at: '2026-01-16T10:30:00Z'
```

Это обеспечивает:
- ✅ **GitOps совместимость** (Ansible/Terraform не конфликтует с автоматической ротацией)
- ✅ **Immutable Infrastructure** (config.yaml read-only)
- ✅ **Key Overlap Period** (множественные версии ключей доступны одновременно)
- ✅ **Zero-downtime rotation** (старые данные расшифровываются v1, новые шифруются v2)
- ✅ **Простой rollback** (изменяется только metadata.yaml)

## 📦 Быстрый старт

### Требования

- Docker + Docker Compose
- OpenSSL (для генерации PKI)
- Go 1.21+ (для сборки из исходников)

### Развертывание

```bash
# 1. Клонировать репозиторий
git clone <repo-url>
cd hsm-service

# 2. Сгенерировать PKI инфраструктуру
./pki/scripts/issue-server-cert.sh hsm-service.local
./pki/scripts/issue-client-cert.sh hsm-trading-client-1 Trading

# 3. Создать metadata.yaml из шаблона
cp metadata.yaml.example metadata.yaml

# 4. Запустить сервис
docker-compose up -d

# 5. Инициализировать KEK (первый раз)
docker exec hsm-service /app/hsm-admin init-keys
```

### Проверка работы

```bash
# Health check
curl --cacert pki/ca/ca.crt \
     --cert pki/client/hsm-trading-client-1.crt \
     --key pki/client/hsm-trading-client-1.key \
     https://hsm-service.local:8443/health

# Encrypt данные
curl -X POST https://hsm-service.local:8443/encrypt \
     --cacert pki/ca/ca.crt \
     --cert pki/client/hsm-trading-client-1.crt \
     --key pki/client/hsm-trading-client-1.key \
     -H "Content-Type: application/json" \
     -d '{"plaintext":"sensitive-data","context":"exchange-key"}'
```

## 📚 Документация

- [ARCHITECTURE.md](ARCHITECTURE.md) - Детальная архитектура системы
- [TECHNICAL_SPEC.md](TECHNICAL_SPEC.md) - Техническая спецификация и API
- [DEVELOPMENT_PLAN.md](DEVELOPMENT_PLAN.md) - План разработки по дням
- [KEY_ROTATION.md](KEY_ROTATION.md) - Процедуры ротации ключей
- [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - Аудит безопасности
- [RUN.md](RUN.md) - Инструкции по запуску
- [cmd/hsm-admin/README.md](cmd/hsm-admin/README.md) - Документация CLI утилиты
- [pki/README.md](pki/README.md) - Управление PKI инфраструктурой
- [scripts/README.md](scripts/README.md) - Скрипты автоматизации

## 🔑 Управление ключами

### Проверка статуса ротации

```bash
docker exec hsm-service /app/hsm-admin rotation-status
```

### Ротация ключа

```bash
# 1. Проверить что ключи готовы к ротации
docker exec hsm-service /app/hsm-admin rotation-status

# 2. Выполнить ротацию (создаёт новую версию ключа)
docker exec hsm-service /app/hsm-admin rotate exchange-key

# 3. Перезапустить сервис для загрузки новой версии
docker restart hsm-service

# 4. Проверить что обе версии доступны
docker exec hsm-service /app/hsm-admin rotation-status

# 5. Очистить старые версии (опционально, через 30+ дней)
docker exec hsm-service /app/hsm-admin cleanup-old-versions --dry-run
docker exec hsm-service /app/hsm-admin cleanup-old-versions
```

**Важно:**
- После ротации доступны **обе версии** ключа (overlap period)
- Новые операции encrypt используют v2
- Старые данные можно расшифровать ключом v1
- Автоматический cleanup удалит версии старше 30 дней (или при превышении max_versions=3)
- Используйте `--dry-run` для предварительного просмотра

## 🛡️ Безопасность

### ACL Маппинг

| Organizational Unit | Разрешенные Contexts |
|---------------------|---------------------|
| Trading             | exchange-key        |
| 2FA                 | 2fa                 |
| Database            | (нет доступа)       |

### Требования к клиентским сертификатам

- Должны быть выданы доверенным CA (указан в `config.yaml`)
- CN должен быть уникальным
- OU должен быть определен в ACL маппинге
- Сертификат не должен быть отозван (проверка по `revoked.yaml`)

### Ротация ключей

- **Интервал по умолчанию:** 90 дней (PCI DSS Requirement 3.6.4)
- **Период перекрытия (overlap):** Безлимитный - все версии ключей доступны одновременно
- **Retention Policy:** 
  - Max версий: 3 (настраивается через `max_versions`)
  - Auto-cleanup: версии старше 30 дней (настраивается через `cleanup_after_days`)
  - PCI DSS compliant - автоматическая очистка устаревших ключей
- **Версионирование:** kek-exchange-v1 → kek-exchange-v2 → kek-exchange-v3...
- **Динамические ID:** Каждая версия получает уникальный 16-значный hex ID на основе timestamp
- **Zero-downtime:** Старые данные расшифровываются v1, новые шифруются v2
- **Автоматические проверки:** При старте сервиса проверяются просроченные ключи и избыточные версии

## 🔧 Конфигурация

### config.yaml

```yaml
hsm:
  pkcs11_lib: /usr/lib/softhsm/libsofthsm2.so
  slot_id: hsm-token
  metadata_file: /app/metadata.yaml
  max_versions: 3           # Maximum key versions to keep (PCI DSS)
  cleanup_after_days: 30    # Auto-delete versions older than N days
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
```

### metadata.yaml

```yaml
rotation:
  exchange-key:
    current: kek-exchange-v2     # Текущая активная версия
    versions:
      - label: kek-exchange-v1   # Старая версия (доступна для decrypt)
        version: 1
        created_at: '2026-01-09T00:00:00Z'
      - label: kek-exchange-v2   # Новая версия (используется для encrypt)
        version: 2
        created_at: '2026-01-16T10:30:00Z'
  
  2fa:
    current: kek-2fa-v1
    versions:
      - label: kek-2fa-v1
        version: 1
        created_at: '2026-01-09T00:00:00Z'
```

### Environment Variables

```bash
# Обязательно
HSM_PIN=1234              # PIN для доступа к HSM токену

# Опционально
CONFIG_PATH=/app/config.yaml
LOG_LEVEL=info
```

## 📊 Мониторинг

### Health Check

```bash
curl https://hsm-service.local:8443/health
```

**Response:**
```json
{
  "status": "healthy",
  "hsm_initialized": true,
  "active_keys": 2,
  "timestamp": "2025-01-10T10:30:00Z"
}
```

### Метрики (если включено)

- `hsm_encrypt_total` - Количество операций encrypt
- `hsm_decrypt_total` - Количество операций decrypt
- `hsm_encrypt_duration_seconds` - Latency encrypt операций
- `hsm_decrypt_duration_seconds` - Latency decrypt операций
- `hsm_acl_denied_total` - Количество отказов ACL

## 🐳 Docker Compose

```yaml
services:
  hsm-service:
    build: .
    ports:
      - "8443:8443"
    environment:
      - HSM_PIN=${HSM_PIN}
    volumes:
      - ./config.yaml:/app/config.yaml:ro          # Статическая конфигурация
      - ./metadata.yaml:/app/metadata.yaml:rw      # Динамические метаданные
      - ./pki:/app/pki:ro                          # PKI certificates
      - ./data/tokens:/var/lib/softhsm/tokens      # HSM storage
    restart: unless-stopped
```

## 🧪 Тестирование

```bash
# Unit tests
go test ./internal/...

# Integration tests
./scripts/test-integration.sh

# Security audit
./scripts/security-scan.sh
```

## 📝 Логирование

Все криптографические операции логируются:

```json
{
  "level": "info",
  "time": "2025-01-10T10:30:00Z",
  "msg": "Encrypt operation",
  "client_cn": "hsm-trading-client-1",
  "client_ou": "Trading",
  "context": "exchange-key",
  "key_id": "kek-exchange-v2",
  "operation": "encrypt",
  "duration_ms": 12
}
```

## 🤝 Поддержка

- [Issues](https://github.com/your-org/hsm-service/issues)
- Email: ops@company.com

## 📄 Лицензия

См. [LICENSE](LICENSE)

## 🔗 Полезные ссылки

- [PKCS#11 Specification](http://docs.oasis-open.org/pkcs11/pkcs11-base/v2.40/pkcs11-base-v2.40.html)
- [PCI DSS Requirements](https://www.pcisecuritystandards.org/)
- [SoftHSM Documentation](https://www.opendnssec.org/softhsm/)
