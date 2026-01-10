# HSM Service - Architecture Documentation

## Оглавление
1. [Обзор системы](#обзор-системы)
2. [Компоненты архитектуры](#компоненты-архитектуры)
3. [Управление конфигурацией](#управление-конфигурацией)
4. [Криптографическая архитектура](#криптографическая-архитектура)
5. [Сетевая архитектура](#сетевая-архитектура)
6. [Управление доступом (ACL)](#управление-доступом-acl)
7. [PKI Инфраструктура](#pki-инфраструктура)
8. [Потоки данных](#потоки-данных)
9. [Безопасность](#безопасность)
10. [Масштабирование и отказоустойчивость](#масштабирование-и-отказоустойчивость)

---

## Обзор системы

### Назначение

Централизованный криптографический сервис для шифрования/расшифрования данных с использованием Hardware Security Module (SoftHSM v2). Сервис предоставляет безопасное хранилище для Key Encryption Keys (KEK) и выполняет криптографические операции для распределенных систем.

### Контекст использования

```mermaid
graph TB
    subgraph "Распределенные сервисы"
        TS1[Trading Service 1<br/>OU=Trading]
        TS2[Trading Service 2<br/>OU=Trading]
        WEB[Web 2FA Service<br/>OU=2FA]
        MOB[Mobile 2FA App<br/>OU=2FA]
    end
    
    subgraph "HSM Service"
        API[HTTPS API :8443<br/>mTLS Required]
        ACL[ACL Engine<br/>OU-based]
        CRYPTO[Crypto Engine<br/>AES-256-GCM]
        
        subgraph "SoftHSM v2"
            KEK1[kek-exchange-v1<br/>AES-256]
            KEK2[kek-2fa-v1<br/>AES-256]
        end
    end
    
    subgraph "Databases"
        DB1[(Trading DB<br/>encrypted DEKs)]
        DB2[(2FA DB<br/>encrypted secrets)]
    end
    
    TS1 -->|mTLS| API
    TS2 -->|mTLS| API
    WEB -->|mTLS| API
    MOB -->|mTLS| API
    
    API --> ACL
    ACL --> CRYPTO
    CRYPTO --> KEK1
    CRYPTO --> KEK2
    
    TS1 -.->|mTLS<br/>stores encrypted DEKs| DB1
    TS2 -.->|mTLS<br/>stores encrypted DEKs| DB1
    WEB -.->|mTLS<br/>stores encrypted secrets| DB2
```

### Ключевые преимущества

**Безопасность ключей:**
- ✅ KEK НИКОГДА не покидает HSM (CKA_EXTRACTABLE=false)
- ✅ Защита от компрометации при утечке конфигов
- ✅ Физическая изоляция криптографических операций
- ✅ Атрибуты ключей запрещают экспорт

**Централизованное управление:**
- ✅ Единая точка управления всеми KEK
- ✅ Простая ротация ключей без обновления клиентских сервисов
- ✅ Унифицированная политика безопасности
- ✅ Централизованный аудит криптографических операций

**Разграничение доступа:**
- ✅ mTLS с взаимной аутентификацией
- ✅ ACL на основе OU (Organizational Unit) в сертификатах
- ✅ Управление отозванными сертификатами (revoked.yaml)
- ✅ Контекстная изоляция (context binding через AAD)

**Масштабируемость:**
- ✅ Поддержка множества распределенных сервисов
- ✅ Независимая ротация ключей для разных контекстов
- ✅ Версионирование KEK с обратной совместимостью
- ✅ Возможность горизонтального масштабирования (Phase 2)

---

## Компоненты архитектуры

### High-Level Architecture

```mermaid
graph LR
    subgraph "HSM Service Process"
        subgraph "HTTP Layer"
            MTLS[mTLS Handler]
            RT[Rate Limiter]
            LOG[Audit Logger]
        end
        
        subgraph "Business Logic"
            ACL[ACL Checker<br/>OU-based]
            ENC[Encrypt Handler]
            DEC[Decrypt Handler]
        end
        
        subgraph "Crypto Layer"
            MGR[Key Manager]
            AES[AES-GCM Engine]
        end
        
        subgraph "PKCS#11 Interface"
            P11[crypto11 library]
        end
        
        subgraph "SoftHSM v2"
            TOKEN[HSM Token<br/>in-process]
        end
    end
    
    CLIENT[Client<br/>with mTLS cert] --> MTLS
    MTLS --> RT
    RT --> LOG
    LOG --> ACL
    ACL --> ENC
    ACL --> DEC
    ENC --> MGR
    DEC --> MGR
    MGR --> AES
    AES --> P11
    P11 --> TOKEN
```

### Структура проекта

```
hsm-service/
├── cmd/
│   ├── create-kek/             # CLI утилита для создания KEK
│   │   ├── main.go
│   │   └── README.md
│   │
│   └── hsm-admin/              # CLI утилита для управления KEK
│       ├── main.go
│       ├── rotate.go           # Ротация ключей
│       ├── cleanup.go          # Очистка старых ключей
│       ├── checksums.go        # Проверка целостности
│       └── README.md
│
├── internal/
│   ├── config/                 # Конфигурация
│   │   ├── config.go           # Загрузка config.yaml и metadata.yaml
│   │   ├── config_test.go      # Тесты конфигурации
│   │   └── types.go            # Типы конфигурации и метаданных
│   │
│   ├── hsm/                    # PKCS#11 и криптография
│   │   ├── pkcs11.go           # Инициализация SoftHSM
│   │   ├── crypto.go           # Encrypt/Decrypt логика
│   │   └── crypto_test.go      # Тесты криптографии
│   │
│   ├── server/                 # HTTP сервер
│   │   ├── server.go           # HTTP server setup
│   │   ├── handlers.go         # /encrypt, /decrypt endpoints
│   │   ├── handlers_test.go    # Тесты handlers
│   │   ├── acl.go              # ACL проверки по OU
│   │   ├── acl_test.go         # Тесты ACL
│   │   ├── acl_reload_test.go  # Тесты hot reload ACL
│   │   ├── middleware.go       # Rate limit, audit log
│   │   ├── middleware_test.go  # Тесты middleware
│   │   ├── logger.go           # Audit logging
│   │   ├── logger_test.go      # Тесты logger
│   │   └── metrics.go          # Prometheus метрики
│   │
│   └── revocation/             # Управление отозванными сертификатами (пусто)
│
├── pki/                        # PKI инфраструктура
│   ├── ca/                     # CA сертификаты
│   ├── server/                 # Серверные сертификаты
│   ├── client/                 # Клиентские сертификаты
│   ├── scripts/                # Скрипты для PKI
│   │   ├── issue-client-cert.sh
│   │   ├── issue-server-cert.sh
│   │   └── revoke-cert.sh
│   ├── inventory.yaml          # Список всех сертификатов
│   ├── revoked.yaml            # Список отозванных сертификатов
│   └── README.md               # PKI документация
│
├── scripts/
│   ├── init-hsm.sh             # Инициализация SoftHSM token
│   ├── rotate-key-auto.sh      # Автоматическая ротация ключей
│   ├── rotate-key-interactive.sh # Интерактивная ротация
│   ├── cleanup-old-keys.sh     # Очистка старых ключей
│   ├── check-key-rotation.sh   # Мониторинг статуса ротации
│   ├── full-integration-test.sh # Интеграционные тесты
│   └── README.md               # Описание скриптов
│
├── data/                       # Данные runtime
│   └── tokens/                 # SoftHSM токены (persist)
│
├── config.yaml                 # Статическая конфигурация (в Git)
├── metadata.yaml               # Динамические метаданные ротации (вне Git)
├── metadata.yaml.example       # Шаблон метаданных для Git
├── softhsm2.conf              # SoftHSM конфигурация
├── Dockerfile
├── docker-compose.yml
├── .dockerignore
├── .gitignore
├── .env.example
├── go.mod
├── go.sum
├── main.go                     # Entry point
│
├── ARCHITECTURE.md             # Этот файл - архитектурная документация
├── TECHNICAL_SPEC.md           # Техническое задание
├── DEVELOPMENT_PLAN.md         # План разработки
├── README.md                   # Основная документация
├── API.md                      # API документация
├── PKI_SETUP.md                # Настройка PKI
├── KEY_ROTATION.md             # Документация по ротации ключей
├── CLI_TOOLS.md                # Документация CLI утилит
├── MONITORING.md               # Мониторинг и метрики
├── SECURITY_AUDIT.md           # Аудит безопасности
├── BACKUP_RESTORE.md           # Backup и восстановление
├── REVOCATION_RELOAD.md        # Hot reload отозванных сертификатов
├── DOCKER.md                   # Docker документация
├── DOCKER_COMPOSE.md           # Docker Compose документация
├── DOCKER_DEV.md               # Docker для разработки
├── QUICKSTART_DOCKER.md        # Быстрый старт с Docker
├── QUICKSTART_NATIVE.md        # Быстрый старт нативно
├── PRODUCTION_DEBIAN.md        # Production на Debian
├── TROUBLESHOOTING.md          # Решение проблем
├── TEST_PLAN.md                # План тестирования
├── HOT_RELOAD_SUMMARY.md       # Hot reload функциональность
├── DOCS_INDEX.md               # Индекс документации
├── DOCUMENTATION_SUMMARY.md    # Сводка документации
└── LICENSE                     # Лицензия
```

---

## Управление конфигурацией

### Разделение статической конфигурации и динамических метаданных

Архитектура разделяет два типа данных для обеспечения совместимости с GitOps/IaC и принципами immutable infrastructure:

#### 1. Статическая конфигурация (config.yaml)

**Назначение:** Неизменяемая конфигурация сервиса, управляемая через Git/Ansible/Terraform

**Расположение:** В Git репозитории, монтируется в контейнер как read-only (`:ro`)

**Содержимое:**
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
  metadata_file: /app/metadata.yaml  # Путь к файлу метаданных
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
    Database: []
```

**Характеристики:**
- ✅ Управляется через систему контроля версий (Git)
- ✅ Изменяется только через pull request / code review
- ✅ Монтируется в контейнер как read-only
- ✅ Содержит только типы ключей и политики
- ✅ GitOps/IaC friendly (Ansible, Terraform)

#### 2. Динамические метаданные ротации (metadata.yaml)

**Назначение:** Текущее состояние ключей, обновляемое автоматическими скриптами при ротации

**Расположение:** Вне Git, на сервере, монтируется как read-write (`:rw`)

**Содержимое:**
```yaml
rotation:
  exchange-key:
    label: kek-exchange-v2
    version: 2
    created_at: '2025-10-11T12:00:00Z'
  
  2fa:
    label: kek-2fa-v1
    version: 1
    created_at: '2025-10-11T12:00:00Z'
```

**Характеристики:**
- ✅ НЕ коммитится в Git (в `.gitignore`)
- ✅ Обновляется автоматическими скриптами
- ✅ Монтируется в контейнер как read-write
- ✅ Содержит только runtime состояние
- ✅ Шаблон `metadata.yaml.example` в Git

#### Преимущества разделения

**1. GitOps совместимость:**
- config.yaml управляется через Git/Ansible
- Автоматическая ротация не создаёт конфликтов
- metadata.yaml не перезаписывается при deploy

**2. Immutable Infrastructure:**
- config.yaml неизменяемый (`:ro`)
- Только metadata.yaml изменяется в runtime
- Соответствие принципам 12-factor app

**3. Упрощённый Rollback:**
- Откат требует только metadata.yaml
- config.yaml остаётся стабильным
- Быстрое восстановление при сбоях

**4. Чистое разделение ответственности:**
- DevOps → config.yaml (статика)
- Automation → metadata.yaml (динамика)
- Понятные границы изменений

#### Docker конфигурация

```yaml
# docker-compose.yml
volumes:
  # Статическая конфигурация (read-only)
  - ./config.yaml:/app/config.yaml:ro
  
  # Динамические метаданные (read-write)
  - ./metadata.yaml:/app/metadata.yaml:rw
  
  # PKI (read-only)
  - ./pki:/app/pki:ro
```

---

## Криптографическая архитектура

### Иерархия ключей

```mermaid
graph TD
    subgraph "HSM (SoftHSM v2)"
        KEK1[KEK: kek-exchange-v1<br/>AES-256<br/>CKA_EXTRACTABLE=false]
        KEK2[KEK: kek-2fa-v1<br/>AES-256<br/>CKA_EXTRACTABLE=false]
    end
    
    subgraph "Trading Services"
        DEK1[DEK #1<br/>encrypted by kek-exchange-v1]
        DEK2[DEK #2<br/>encrypted by kek-exchange-v1]
        
        KEY1[Exchange API Key #1<br/>encrypted by DEK #1]
        KEY2[Exchange API Key #2<br/>encrypted by DEK #2]
    end
    
    subgraph "2FA Services"
        SECRET1[2FA Secret #1<br/>encrypted by kek-2fa-v1]
        SECRET2[2FA Secret #2<br/>encrypted by kek-2fa-v1]
    end
    
    KEK1 -.->|encrypts| DEK1
    KEK1 -.->|encrypts| DEK2
    DEK1 -.->|encrypts| KEY1
    DEK2 -.->|encrypts| KEY2
    
    KEK2 -.->|encrypts| SECRET1
    KEK2 -.->|encrypts| SECRET2
```

### Два типа использования

#### Вариант 1: Envelope Encryption (для биржевых ключей)

```
1. Trading Service генерирует DEK локально
2. HSM Service шифрует DEK с помощью kek-exchange-v1
3. Trading Service хранит encrypted_DEK в БД
4. Trading Service использует DEK для шифрования API ключей бирж
5. При необходимости Trading Service расшифровывает DEK через HSM
```

**Преимущества:**
- Минимальные обращения к HSM
- DEK кэшируется в памяти Trading Service
- Высокая производительность

#### Вариант 2: Direct Encryption (для 2FA секретов)

```
1. Web 2FA Service отправляет 2FA secret в HSM
2. HSM шифрует secret напрямую с помощью kek-2fa-v1
3. Web 2FA Service хранит encrypted_secret в БД
4. При необходимости Web 2FA Service расшифровывает через HSM
```

**Преимущества:**
- Простота (не нужно управлять DEK)
- Прямое шифрование

### Криптографические параметры

**Алгоритм:** AES-256-GCM

**Параметры:**
- Key Size: 256 бит
- Nonce Size: 12 байт (96 бит)
- Tag Size: 16 байт (128 бит)
- AAD (Additional Authenticated Data): `context || "|" || client_CN`

**Формат ciphertext:**

```
┌──────────┬────────────────────┬──────────┐
│  Nonce   │    Ciphertext      │   Tag    │
│ 12 bytes │   Variable length  │ 16 bytes │
└──────────┴────────────────────┴──────────┘
        ↓
    Base64 Encoded
```

**Пример AAD:**

```
Context: "exchange-key"
Client CN: "trading-service-1"
AAD: "exchange-key|trading-service-1"
```

**Защита AAD:**
- Привязка к контексту (нельзя использовать ciphertext из другого контекста)
- Привязка к клиенту (нельзя переиспользовать ciphertext другим клиентом)
- Защита от replay attacks между разными доменами

### Версионирование KEK

```yaml
# Поддержка нескольких версий KEK одновременно
KEK Lifecycle:

kek-exchange-v1:  [ACTIVE]    - используется для новых encrypt
kek-exchange-v2:  [PENDING]   - создан, но не активен
kek-2fa-v1:       [ACTIVE]
```

### Hot Reload для KEK и Metadata (Zero-Downtime Rotation)

**Проблема текущей реализации:**
- ❌ Restart сервиса требует downtime для всех 50+ клиентов
- ❌ Невозможность graceful rotation в production
- ❌ Прерывание активных операций
- ❌ Single point of failure при обновлении

**Требуемое решение - Zero-Downtime KEK Reload:**

```go
type KeyManager struct {
    ctx           crypto11.Context  // Persistent PKCS#11 session
    keys          map[string]*KeyHandle
    keysMutex     sync.RWMutex
    
    metadata      *MetadataConfig
    metadataMutex sync.RWMutex
    metadataFile  string
    lastModTime   time.Time
}

// Периодическая проверка metadata.yaml (каждые 30 секунд)
func (km *KeyManager) AutoReloadMetadata(interval time.Duration) {
    ticker := time.NewTicker(interval)
    defer ticker.Stop()
    
    for {
        select {
        case <-ticker.C:
            if km.metadataChanged() {
                km.ReloadKeysFromMetadata()
            }
        case <-km.stopChan:
            return
        }
    }
}

// Загрузка новых KEK без разрыва сессии
func (km *KeyManager) ReloadKeysFromMetadata() error {
    // 1. Read metadata.yaml
    newMetadata, err := loadMetadata(km.metadataFile)
    if err != nil {
        return err // Keep old data
    }
    
    // 2. Load NEW keys from HSM (не закрывая старые)
    newKeys := make(map[string]*KeyHandle)
    for context, meta := range newMetadata.Rotation {
        handle, err := km.ctx.FindKey(nil, []byte(meta.Label))
        if err != nil {
            return err // Rollback, keep old data
        }
        newKeys[context] = &KeyHandle{
            Handle:  handle,
            Label:   meta.Label,
            Version: meta.Version,
        }
    }
    
    // 3. Atomic swap (все или ничего)
    km.keysMutex.Lock()
    oldKeys := km.keys
    km.keys = newKeys
    km.keysMutex.Unlock()
    
    km.metadataMutex.Lock()
    km.metadata = newMetadata
    km.metadataMutex.Unlock()
    
    // 4. Cleanup old handles (опционально, можно оставить в памяти)
    // Old handles still valid for ongoing decrypt operations
    
    slog.Info("KEK hot reload successful", 
        "contexts", len(newKeys))
    return nil
}

// Encrypt использует текущую активную версию
func (km *KeyManager) Encrypt(context string, plaintext []byte) ([]byte, error) {
    km.keysMutex.RLock()
    keyHandle, exists := km.keys[context]
    km.keysMutex.RUnlock()
    
    if !exists {
        return nil, ErrKeyNotFound
    }
    
    // Use current active version from metadata
    return encryptAESGCM(keyHandle.Handle, plaintext)
}

// Decrypt работает с любой версией (по key_id из request)
func (km *KeyManager) Decrypt(keyID string, ciphertext []byte) ([]byte, error) {
    // Find key by label (может быть старая версия)
    handle, err := km.ctx.FindKey(nil, []byte(keyID))
    if err != nil {
        return nil, ErrKeyNotFound
    }
    
    return decryptAESGCM(handle, ciphertext)
}
```

**Процесс ротации с Hot Reload:**

```bash
# 1. Создать новую версию ключа
hsm-admin rotate exchange-key
# Output: Created kek-exchange-v2, updated metadata.yaml

# 2. Metadata.yaml обновлен автоматически:
# rotation:
#   exchange-key:
#     label: kek-exchange-v2
#     version: 2
#     created_at: '2026-01-10T15:30:00Z'

# 3. HSM Service автоматически перезагружает metadata.yaml в течение 30 сек
#    - Загружает kek-exchange-v2 из HSM
#    - Атомарно переключается на новую версию
#    - Старые ключи остаются доступны для decrypt
#    - ZERO DOWNTIME, клиенты не видят прерываний

# 4. Новые encrypt используют v2, старые decrypt работают с v1

# 5. Фоновое перешифрование в клиентских сервисах (недели/месяцы)

# 6. Удаление старого ключа после завершения миграции
hsm-admin cleanup exchange-key --version 1
# Проверяет, что нет активных decrypt с v1 (опционально)
```

**Преимущества:**
- ✅ **Zero Downtime** - клиенты не видят перезапуска
- ✅ **Graceful Rotation** - старые и новые ключи доступны одновременно
- ✅ **Atomic Switch** - переключение на новую версию мгновенное
- ✅ **Persistent PKCS#11 Session** - нет повторной инициализации
- ✅ **Production Ready** - для нагруженных систем с 50+ клиентами

**Примечание:** 
- Hot reload для `revoked.yaml` уже реализован (30 сек)
- Hot reload для `metadata.yaml` и KEK - **КРИТИЧНО для production**, требуется реализация

---

## Сетевая архитектура

### mTLS Configuration

```mermaid
sequenceDiagram
    participant Client
    participant TLS
    participant ACL
    participant HSM
    
    Client->>TLS: ClientHello + Client Cert
    TLS->>TLS: Verify Client Cert with CA
    alt Cert Invalid
        TLS-->>Client: TLS Alert: Bad Certificate
    end
    
    TLS->>TLS: Extract CN and OU from cert
    TLS->>ACL: Check(CN, OU, revoked.yaml)
    
    alt Cert Revoked
        ACL-->>Client: 403 Forbidden
    end
    
    ACL->>ACL: Check OU permissions
    alt OU not allowed
        ACL-->>Client: 403 Forbidden
    end
    
    ACL->>HSM: Forward request
    HSM->>HSM: Encrypt/Decrypt
    HSM-->>Client: Response
```

### TLS Parameters

```go
&tls.Config{
    // Требуем клиентский сертификат
    ClientAuth: tls.RequireAndVerifyClientCert,
    
    // CA для проверки клиентов
    ClientCAs: caCertPool,
    
    // Минимальная версия TLS
    MinVersion: tls.VersionTLS13,
    
    // Разрешенные cipher suites (TLS 1.3)
    CipherSuites: []uint16{
        tls.TLS_AES_256_GCM_SHA384,
        tls.TLS_CHACHA20_POLY1305_SHA256,
    },
    
    // Server certificates
    Certificates: []tls.Certificate{serverCert},
}
```

### Endpoints

```
Base URL: https://hsm-service.local:8443

POST /encrypt
POST /decrypt
GET  /health       (no auth required)
GET  /metrics      (prometheus, optional)
```

---

## Управление доступом (ACL)

### ACL на основе OU (Organizational Unit)

**Принцип:** Организационная единица в сертификате определяет разрешения

```yaml
# config.yaml (статическая конфигурация)
acl:
  mappings:
    Trading: [exchange-key]
    2FA: [2fa]
    Database: []  # нет доступа к ключам
```

**Certificate Subject Format:**

```
CA:      /C=RU/ST=Moscow/L=Moscow/O=Private/OU=Private/CN=Titaev CA
Server:  /C=RU/ST=Moscow/L=Moscow/O=Private/OU=Services/CN=hsm-service.local
Client:  /C=RU/ST=Moscow/L=Moscow/O=Private/OU=Trading/CN=trading-service-1
```

### Алгоритм проверки ACL

```go
func CheckAccess(cert *x509.Certificate, context string) error {
    // 1. Извлечь CN
    cn := cert.Subject.CommonName
    
    // 2. Проверить revoked.yaml
    if IsRevoked(cn) {
        return ErrCertificateRevoked
    }
    
    // 3. Извлечь OU
    if len(cert.Subject.OrganizationalUnit) == 0 {
        return ErrNoOU
    }
    ou := cert.Subject.OrganizationalUnit[0]
    
    // 4. Получить разрешенные contexts для OU
    allowedContexts := config.ACL.ByOU[ou]
    if len(allowedContexts) == 0 {
        return ErrUnknownOU
    }
    
    // 5. Проверить context
    if !contains(allowedContexts, context) {
        return ErrContextNotAllowed
    }
    
    return nil
}
```

### Управление отозванными сертификатами

**Файл: pki/revoked.yaml**

```yaml
# Список отозванных сертификатов
revoked:
  - cn: "trading-service-old"
    serial: "03"
    revoked_date: "2026-01-03T10:00:00Z"
    reason: "compromised"
  
  - cn: "test-service"
    serial: "15"
    revoked_date: "2026-01-04T15:30:00Z"
    reason: "decommissioned"
```

**Загрузка:**
- При старте сервиса
- **Hot reload каждые 30 секунд** (автоматическая проверка modTime файла)
- Проверка только при изменении файла (эффективно)
- Атомарное обновление с валидацией (старые данные сохраняются при ошибке)

**Процесс отзыва:**

```bash
# 1. Добавить запись в revoked.yaml
cat >> pki/revoked.yaml << EOF
  - cn: "compromised-service"
    serial: "1A:2B:3C:4D"
    revoked_date: "$(date -Iseconds)"
    reason: "compromised"
EOF

# 2. Сервис автоматически перезагрузит файл в течение 30 секунд
# (hot reload реализован, перезапуск НЕ требуется)

# 3. Сертификат блокируется и больше не может подключиться
```

**Примечание:** Hot reload работает только для `revoked.yaml`. Для изменений в `metadata.yaml` или конфигурации ключей требуется restart сервиса.

---

## PKI Инфраструктура

### Certificate Authority (CA)

**Существующий CA:**
```
Subject: /C=RU/ST=Moscow/L=Moscow/O=Private/OU=Private/CN=Titaev CA/emailAddress=titaev@gmail.com
Files:
  - ca.crt (публичный сертификат)
  - ca.key (приватный ключ, защищен паролем)
```

**Местоположение:** Отдельная защищенная VM

### Certificate Types

#### 1. Server Certificates

**Назначение:** TLS серверы (HSM service, MySQL, ClickHouse)

**Требования:**
- Subject: `/C=RU/ST=Moscow/L=Moscow/O=Private/OU=Services/CN=<service-name>`
- SAN (Subject Alternative Names):
  - DNS names (обязательно)
  - IP addresses (опционально)
- Key Usage: Digital Signature, Key Encipherment
- Extended Key Usage: TLS Web Server Authentication

**Пример:**
```
Subject: /C=RU/ST=Moscow/L=Moscow/O=Private/OU=Services/CN=hsm-service.local
SAN: DNS:hsm-service.local, DNS:localhost, IP:127.0.0.1
```

#### 2. Client Certificates

**Назначение:** mTLS клиенты

**Требования:**
- Subject: `/C=RU/ST=Moscow/L=Moscow/O=Private/OU=<OU>/CN=<client-name>`
- OU определяет группу доступа
- NO SAN required
- Key Usage: Digital Signature
- Extended Key Usage: TLS Web Client Authentication

**Примеры:**
```
# Trading service
Subject: /C=RU/ST=Moscow/L=Moscow/O=Private/OU=Trading/CN=trading-service-1

# 2FA service
Subject: /C=RU/ST=Moscow/L=Moscow/O=Private/OU=2FA/CN=web-2fa-service

# MySQL client
Subject: /C=RU/ST=Moscow/L=Moscow/O=Private/OU=Database/CN=app-backend-1
```

### PKI Scripts

**scripts/pki/issue-server-cert.sh**
```bash
Usage: ./issue-server-cert.sh <cn> <san-dns> [<san-ip>]
Example: ./issue-server-cert.sh hsm-service.local "hsm-service.local,localhost" "127.0.0.1"

Outputs:
  - pki/server/<cn>.crt
  - pki/server/<cn>.key
  - Updates pki/inventory.yaml
```

**scripts/pki/issue-client-cert.sh**
```bash
Usage: ./issue-client-cert.sh <cn> <ou>
Example: ./issue-client-cert.sh trading-service-1 Trading

Outputs:
  - pki/client/<cn>.crt
  - pki/client/<cn>.key
  - Updates pki/inventory.yaml
```

**scripts/pki/revoke-cert.sh**
```bash
Usage: ./revoke-cert.sh <cn> <reason>
Example: ./revoke-cert.sh old-service compromised

Actions:
  - Adds to pki/revoked.yaml
  - Optionally generates CRL (future)
```

### Certificate Inventory

**pki/inventory.yaml** - автоматически обновляется скриптами

```yaml
certificates:
  servers:
    - cn: hsm-service.local
      ou: Services
      issued: "2026-01-05"
      expires: "2027-01-05"
      serial: "01"
      san_dns: ["hsm-service.local", "localhost"]
      san_ip: ["127.0.0.1"]
      file: server/hsm-service.local
    
  clients:
    - cn: trading-service-1
      ou: Trading
      issued: "2026-01-05"
      expires: "2027-01-05"
      serial: "02"
      access_contexts: [exchange-key]
      file: client/trading-service-1
    
    - cn: web-2fa-service
      ou: 2FA
      issued: "2026-01-05"
      expires: "2027-01-05"
      serial: "03"
      access_contexts: [2fa]
      file: client/web-2fa-service
```

---

## Потоки данных

### Encrypt Flow

```mermaid
sequenceDiagram
    participant Client as Trading Service
    participant TLS as TLS Handler
    participant ACL as ACL Checker
    participant Handler as Encrypt Handler
    participant Crypto as Crypto Engine
    participant HSM as SoftHSM

    Client->>TLS: POST /encrypt (mTLS)
    Note over Client,TLS: {context:"exchange-key", plaintext:"base64"}
    
    TLS->>TLS: Verify client certificate
    TLS->>ACL: Extract CN & OU
    ACL->>ACL: Check OU="Trading" allowed for "exchange-key"
    ACL->>ACL: Check CN not in revoked.yaml
    
    ACL->>Handler: Request validated
    Handler->>Handler: Decode base64 plaintext
    Handler->>Handler: Build AAD = "exchange-key|trading-service-1"
    
    Handler->>Crypto: Encrypt(plaintext, AAD, key_id="kek-exchange-v1")
    Crypto->>HSM: Find key by label "kek-exchange-v1"
    Crypto->>HSM: Generate random nonce (12 bytes)
    Crypto->>HSM: AES-GCM Encrypt
    HSM-->>Crypto: nonce || ciphertext || tag
    
    Crypto->>Handler: Encrypted data
    Handler->>Handler: Base64 encode
    Handler-->>Client: {ciphertext:"base64", key_id:"kek-exchange-v1"}
    
    Note over Client: Store ciphertext in database
```

### Decrypt Flow

```mermaid
sequenceDiagram
    participant Client as Trading Service
    participant TLS as TLS Handler
    participant ACL as ACL Checker
    participant Handler as Decrypt Handler
    participant Crypto as Crypto Engine
    participant HSM as SoftHSM

    Client->>TLS: POST /decrypt (mTLS)
    Note over Client,TLS: {context:"exchange-key", ciphertext:"base64", key_id:"kek-exchange-v1"}
    
    TLS->>TLS: Verify client certificate
    TLS->>ACL: Extract CN & OU
    ACL->>ACL: Check permissions
    
    ACL->>Handler: Request validated
    Handler->>Handler: Decode base64 ciphertext
    Handler->>Handler: Build AAD = "exchange-key|trading-service-1"
    Handler->>Handler: Parse nonce || ciphertext || tag
    
    Handler->>Crypto: Decrypt(ciphertext, nonce, tag, AAD, key_id)
    Crypto->>HSM: Find key by label "kek-exchange-v1"
    Crypto->>HSM: AES-GCM Decrypt with AAD verification
    
    alt AAD Mismatch
        HSM-->>Client: 400 Bad Request: AAD verification failed
    end
    
    HSM-->>Crypto: plaintext
    Crypto->>Handler: Decrypted data
    Handler->>Handler: Base64 encode plaintext
    Handler-->>Client: {plaintext:"base64"}
    
    Note over Client: Use plaintext (e.g., DEK to decrypt exchange keys)
```

---

## Безопасность

### Threat Model

**Защищаемые активы:**
1. KEK (kek-exchange-v1, kek-2fa-v1) - КРИТИЧНО
2. Plaintext данные в транзите
3. Приватные ключи сертификатов

**Угрозы:**

| Угроза | Митигация |
|--------|-----------|
| Компрометация KEK | KEK НИКОГДА не покидает HSM (CKA_EXTRACTABLE=false) |
| Man-in-the-Middle | mTLS с mutual authentication |
| Unauthorized access | ACL по OU + revoked.yaml |
| Replay attacks | AAD включает context + client_CN, уникальные nonce |
| Context confusion | AAD привязан к context |
| Brute force | Rate limiting |
| Certificate theft | Certificate + Private Key хранятся отдельно |
| Insider threat | Audit logging всех операций |
| HSM compromise | Physical security VM, access controls |

### Security Controls

**1. Cryptographic:**
- AES-256-GCM (authenticated encryption)
- Unique nonce per encryption
- AAD для context binding
- KEK non-extractable

**2. Network:**
- TLS 1.3 only
- mTLS (mutual authentication)
- Strong cipher suites

**3. Access Control:**
- Certificate-based authentication
- OU-based authorization
- Revocation list (revoked.yaml)

**4. Operational:**
- Audit logging (не логируем plaintext/keys)
- Rate limiting
- Health checks

**5. Physical:**
- SoftHSM tokens на защищенной VM
- Backup tokens зашифрованы
- Access controls на VM

### Secrets Management

**Что НЕ логируется:**
- ❌ Plaintext
- ❌ KEK handles/IDs (только labels)
- ❌ Nonces, ciphertext (только metadata)
- ❌ HSM PIN

**Что логируется:**
- ✅ Client CN
- ✅ Context
- ✅ Operation (encrypt/decrypt)
- ✅ Timestamp
- ✅ Success/Failure
- ✅ Key ID (label)
- ✅ Client IP

**Environment Variables (секреты):**
```bash
HSM_PIN=<pin>          # Не храним в config.yaml
HSM_SO_PIN=<so-pin>    # Только для admin операций
```

---

## Масштабирование и отказоустойчивость

### Current Architecture (Phase 1)

```
Single instance deployment:
- One VM
- One HSM service process
- One SoftHSM token
```

**Limitations:**
- Single point of failure
- Limited throughput
- No geographic redundancy

### Future Scalability (Phase 2+)

**Option A: Active-Passive HA**

```mermaid
graph TB
    subgraph "Load Balancer"
        LB[HAProxy / Nginx Stream]
    end
    
    subgraph "Active Node"
        HSM1[HSM Service 1<br/>ACTIVE]
        TOKEN1[SoftHSM Token 1]
    end
    
    subgraph "Passive Node"
        HSM2[HSM Service 2<br/>STANDBY]
        TOKEN2[SoftHSM Token 2<br/>Replicated]
    end
    
    CLIENTS[Clients] --> LB
    LB --> HSM1
    LB -.->|failover| HSM2
    HSM1 --> TOKEN1
    HSM2 --> TOKEN2
    TOKEN1 -.->|backup/restore| TOKEN2
```

**Option B: Horizontal Scaling (Read Replicas)**

```
Multiple HSM service instances:
- Shared read-only KEK (via token replication)
- Load balanced requests
- Eventual consistency for KEK rotation
```

**Challenges:**
- SoftHSM не поддерживает clustering
- Нужна репликация токенов (backup/restore)
- KEK ротация требует координации

---

## Monitoring & Observability

### Metrics (Prometheus)

```
# Requests
hsm_requests_total{operation="encrypt", context="exchange-key", status="success"}
hsm_requests_total{operation="decrypt", context="2fa", status="error"}

# Latency
hsm_operation_duration_seconds{operation="encrypt"}

# Rate limiting
hsm_rate_limit_exceeded_total{client="trading-service-1"}

# Errors
hsm_errors_total{type="acl_denied", ou="Unknown"}
```

### Health Checks

```
GET /health

Response:
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

### Audit Log Format

```json
{
  "timestamp": "2026-01-05T12:34:56Z",
  "client_cn": "trading-service-1",
  "client_ou": "Trading",
  "client_ip": "10.0.0.5",
  "operation": "encrypt",
  "context": "exchange-key",
  "key_id": "kek-exchange-v1",
  "status": "success",
  "duration_ms": 5
}
```

---

## Deployment Architecture

### Development Environment

```yaml
# docker-compose.yml
services:
  hsm-service:
    build: .
    ports:
      - "8443:8443"
    volumes:
      - ./data/tokens:/var/lib/softhsm/tokens  # Persistence
      - ./pki:/app/pki:ro                       # Certificates
    environment:
      - HSM_PIN=${HSM_PIN}
      - CONFIG_PATH=/app/config.yaml
```

### Production Environment

```
VM Configuration:
- OS: Ubuntu 22.04 LTS
- RAM: 4GB minimum
- CPU: 2 cores
- Disk: 20GB (для token storage)
- Network: Internal VLAN only

Security:
- Firewall: только 8443 от известных IP
- SELinux / AppArmor enabled
- SSH hardened (key-only, no root)
- Audit logging to SIEM
```

---

## Appendix

### Glossary

- **KEK** - Key Encryption Key, основной ключ в HSM
- **DEK** - Data Encryption Key, ключ для шифрования данных
- **HSM** - Hardware Security Module
- **mTLS** - Mutual TLS, двусторонняя аутентификация
- **AAD** - Additional Authenticated Data, дополнительные данные для GCM
- **OU** - Organizational Unit, организационная единица в DN
- **CN** - Common Name, общее имя в DN
- **SAN** - Subject Alternative Name
- **CRL** - Certificate Revocation List

### References

- SoftHSM v2: https://www.opendnssec.org/softhsm/
- PKCS#11 Spec: https://docs.oasis-open.org/pkcs11/pkcs11-base/v2.40/
- crypto11 library: https://github.com/ThalesIgnite/crypto11
- AES-GCM: NIST SP 800-38D
