# 🚀 HSM Service — План усовершенствования 2026

> **Дата анализа**: 8 февраля 2026  
> **Текущая версия**: v1.1.2  
> **Целевая версия**: v2.0.0  
> **Статус**: В актуализации (этап test coverage завершен)

---

## 📊 Executive Summary

### Текущее состояние проекта

**Оценка зрелости: 8.5/10** ✅

| Категория | Оценка | Статус |
|-----------|--------|--------|
| **Безопасность** | 9.5/10 | ✅ Excellent |
| **Архитектура** | 8/10 | 🟡 Good, needs HA |
| **Тестирование** | 9.5/10 | ✅ Coverage stage completed |
| **Документация** | 9/10 | ✅ Comprehensive |
| **Compliance** | 8/10 | 🟡 PCI DSS partial |
| **Операционная готовность** | 7/10 | 🟡 Needs automation |

### Ключевые достижения

✅ **Безопасность (v1.0.1 - v1.1.2)**
- All OWASP Top 10 2021 vulnerabilities addressed
- 7 security fixes (G115, G204, G304, G302/G306, G104, PIN leak)
- TLS 1.3-only, mTLS enforced, AES-256-GCM
- Rate limiting, request size limits, timeout protection
- KEK integrity verification with SHA-256 checksums

✅ **Key Rotation (PCI DSS 3.6.4)**
- Zero-downtime rotation with hot reload
- Multi-version key support (overlap period)
- Automated rotation scripts
- Cleanup old versions (PCI DSS compliance)
- RFC3339Micro timestamp tracking

✅ **Access Control**
- OU-based ACL with context isolation
- Certificate revocation (revoked.yaml)
- Per-client rate limiting
- Detailed audit logging

✅ **Мониторинг**
- Prometheus metrics (8 categories)
- Structured logging with rotation
- Health checks (/health, /ready)

### Критические пробелы

🔴 **High Priority**
1. **High Availability**: Single point of failure
2. **Hardware HSM**: Only SoftHSM support
3. **Backup/Restore**: Manual, no automation
4. **PCI DSS Split Knowledge**: Single PIN для всех ключей

🟠 **Medium Priority**
6. **Audit API**: No structured audit queries
7. **CLI Consolidation**: Single hsm-admin CLI (DONE v2.0.0)
8. **Multi-Slot Isolation**: All KEKs in single HSM slot
9. **Certificate Automation**: No cert-manager integration
10. **Disaster Recovery**: No documented runbooks
11. **Metrics Isolation**: `/metrics` is served on main API listener instead of dedicated listener/port

---

## 🎯 Стратегические цели

## 📋 Детальный план по приоритетам

## PHASE 1: Foundation & Testing (Q1 2026)

### 1.1 Test Coverage Improvement 🔴 CRITICAL

**Статус на 2026-03-18:** 🟢 Выполнено с превышением целевых метрик

**Фактические результаты:**
- ✅ `internal/hsm`: **93.5%** (целевой порог >80% превышен)
- ✅ `cmd/hsm-admin`: **91.5%** (целевой порог >50% существенно превышен)
- ✅ Добавлены hermetic unit-тесты без зависимости от реального HSM для dry-run, error-path и edge-case веток
- ✅ Покрыты сложные ветвления (reload, nil-guards, multi-version, hot reload, checksum mismatch, error handling)
- ✅ Добавлены coverage-gates в CI (`make test-coverage-check`, `make ci`)

**Остаток после закрытия 1.1:**
- ⏳ Декомпозиция compliance-проверок в отдельные PCI DSS подпункты (текущий агрегированный `tests/compliance/pci-dss.sh` оставлен как базовый gate)

**Метрики качества:**
```bash
# Актуальные метрики (март 2026)
internal/hsm:     93.5%  ✅
cmd/hsm-admin:    91.5%  ✅
```

#### 1.1.3 Integration Test Suite Expansion (актуальный оставшийся блок)

**tests/security/** - Add missing security tests:
```bash
tests/
  security/
    sql_injection.sh        # ✅ Done (N/A - no SQL)
    command_injection.sh    # ✅ Done (fixed G204)
    path_traversal.sh       # ✅ Done (fixed G304)
    timing_attack.sh        # ✅ Done
    replay_attack.sh        # ✅ Done
    tls_downgrade.sh        # ✅ Done
```

**Примечание:** новые security-сценарии подключены в `tests/security/security-scan.sh` (раздел active attack simulations).

**Статус:** 🟢 Базовый этап завершен
- ✅ Security attack simulations реализованы и подключены
- ✅ Compliance phase интегрирована в общий `run-all-tests.sh`
- ⏳ Следующий шаг: декомпозиция PCI DSS сценариев на подпункты

**tests/compliance/** - PCI DSS validation:
```bash
tests/
  compliance/
    pci-dss.sh             # ✅ Exists (aggregated checks)
    owasp-top10.sh         # ✅ Exists
    pci-dss-3.6.4.sh       # ⏳ TODO (split from pci-dss.sh)
    pci-dss-3.6.6.sh       # ⏳ TODO (Phase 3)
    pci-dss-10.2.sh        # ⏳ TODO (split from pci-dss.sh)
```

**Где запускать 1.1.3:**
- Рекомендуется отдельная машина/runner (staging или dedicated CI host), а не dev-ноутбук
- Причина: интеграционные security/compliance тесты требуют стабильного сетевого окружения, Docker Compose стенда и PKCS#11/SoftHSM окружения
- Минимум окружения: Linux, Docker/Compose, SoftHSM2/PKCS#11 libs, тестовые сертификаты и изолированная сеть

**Операционный план запуска 1.1.3:**
1. Поднять отдельный стенд `docker-compose up -d` в staging-профиле.
2. Прогнать `tests/security/security-scan.sh` как обязательный pre-release gate.
3. Прогнать `tests/compliance/pci-dss.sh` и `tests/compliance/owasp-top10.sh`.
4. Декомпозировать `pci-dss.sh` на подпункты (`3.6.4`, `10.2`) и сделать их отдельными CI jobs.
5. Зафиксировать результаты в артефактах CI и в release checklist.

**Критерии успеха:**
- [x] internal/hsm coverage >80%
- [x] internal/hsm coverage >90%
- [x] cmd/* coverage >50%
- [x] All integration tests passing
- [x] CI/CD pipeline with coverage gates

---

### 1.2 CLI Consolidation 🟡 MEDIUM

**Статус:** DONE (v2.0.0)

**Текущее состояние:**
```
/app/
  hsm-admin            # 470 строк, основной CLI
  
# Проблемы (до v2.0.0):
# - Дублирование PKCS#11 инициализации
# - Разный UX (create-kek: args, hsm-admin: flags)
# - Документация разбита
# - Deployment complexity
```

### 1.3 Backup & Restore Automation 🟠 HIGH

**Текущее состояние:**
- Manual backup via `softhsm2-util --export`
- No encryption for backup files
- No restore testing
- No automation scripts

**Проблемы:**
- Compliance risk (no DR plan)
- Manual process = human error
- Unencrypted backup files
- No verification of backup integrity

**Решение:**

```bash
# cmd/hsm-admin/backup.go (new command)
hsm-admin backup \
  --output /backup/hsm-$(date +%Y%m%d).enc \
  --encryption-key-env BACKUP_ENCRYPTION_KEY \
  --include-metadata

# Что включает backup:
# 1. SoftHSM token export (encrypted)
# 2. metadata.yaml + checksums
# 3. config.yaml (for reference)
# 4. Manifest с версиями и датами

# cmd/hsm-admin/restore.go (new command)
hsm-admin restore \
  --input /backup/hsm-20260208.enc \
  --encryption-key-env BACKUP_ENCRYPTION_KEY \
  --verify-checksums \
  --dry-run  # Test restore without applying
```

**Архитектура backup:**

```yaml
# backup-manifest.yaml (generated during backup)
  version: "1.0"
  backup_date: "2026-02-08T10:30:00Z"
  hsm_service_version: "v1.1.2"
encryption:
  algorithm: AES-256-GCM
  key_derivation: Argon2id
contents:
  - type: softhsm_token
    path: tokens/hsm-token.db
    checksum: sha256:abc123...
  - type: metadata
    path: metadata.yaml
    checksum: sha256:def456...
  - type: config
    path: config.yaml
    checksum: sha256:ghi789...
keys:
  - label: kek-exchange-key-v1
    version: 1
    checksum: abc123...
  - label: kek-exchange-key-v2
    version: 2
    checksum: def456...
```

**Automated backup script:**

```bash
# scripts/backup-hsm.sh (run via cron)
#!/bin/bash
set -euo pipefail

BACKUP_DIR="${BACKUP_DIR:-/backup/hsm}"
RETENTION_DAYS="${RETENTION_DAYS:-30}"
BACKUP_ENCRYPTION_KEY="${BACKUP_ENCRYPTION_KEY:?Required}"

# Daily backup
TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_FILE="$BACKUP_DIR/hsm-backup-$TIMESTAMP.enc"

docker exec hsm-service /app/hsm-admin backup \
  --output "$BACKUP_FILE" \
  --encryption-key-env BACKUP_ENCRYPTION_KEY \
  --include-metadata

# Verify backup integrity
docker exec hsm-service /app/hsm-admin verify-backup \
  --input "$BACKUP_FILE" \
  --encryption-key-env BACKUP_ENCRYPTION_KEY

# Cleanup old backups (keep 30 days)
find "$BACKUP_DIR" -name "hsm-backup-*.enc" -mtime +$RETENTION_DAYS -delete

# Upload to S3/MinIO (optional)
if [ -n "${S3_BUCKET:-}" ]; then
  aws s3 cp "$BACKUP_FILE" "s3://$S3_BUCKET/hsm-backups/"
fi

echo "✅ Backup completed: $BACKUP_FILE"
```

**DR Runbook:**

```markdown
# Disaster Recovery Procedure

## Scenario 1: HSM token corruption

1. Stop HSM service
   ```bash
   docker stop hsm-service
   ```

2. List available backups
   ```bash
   ls -lh /backup/hsm/
   # Choose latest backup
   ```

3. Restore from backup
   ```bash
   docker run --rm \
     -v /backup/hsm:/backup:ro \
     -v hsm-tokens:/var/lib/softhsm/tokens \
     -e BACKUP_ENCRYPTION_KEY=$KEY \
    hsm-service:v1.1.2 \
     /app/hsm-admin restore \
       --input /backup/hsm-backup-20260208-103000.enc \
       --verify-checksums
   ```

4. Start service and verify
   ```bash
   docker start hsm-service
   docker logs -f hsm-service  # Check for "KEK integrity verified"
   curl -k https://localhost:8443/health
   ```

## Scenario 2: Complete system loss

[Full DR steps...]
```

**PCI DSS Compliance:**
- ✅ Requirement 3.5.1: Backup stored encrypted (AES-256-GCM)
- ✅ Requirement 3.6.1.3: Key storage separate from data
- ✅ Requirement 12.10.4: Backup restoration tested quarterly

**Effort:** 5 дней разработки + 2 дня тестирования

---

## PHASE 2: Enterprise Features (Q2 2026)

### 2.1 Multi-Slot Architecture 🟡 MEDIUM

**Текущая проблема:**
```
┌─────────────────────────────────┐
│  Single HSM Slot "hsm-token"    │
│  PIN: 1234 (shared)             │
│                                 │
│  ├── kek-exchange-key-v1        │
│  ├── kek-exchange-key-v2        │
│  ├── kek-2fa-v1                 │
│  └── kek-billing-v1             │
└─────────────────────────────────┘

⚠️  Проблема: Компрометация одного PIN = доступ ко ВСЕМ ключам
⚠️  PCI DSS 3.6.6: Рекомендует split knowledge
```

**Решение: Multi-Slot Isolation**

```
┌──────────────────────┐  ┌──────────────────────┐  ┌──────────────────────┐
│ Slot: slot-exchange  │  │ Slot: slot-2fa       │  │ Slot: slot-billing   │
│ PIN: $PIN_EXCHANGE   │  │ PIN: $PIN_2FA        │  │ PIN: $PIN_BILLING    │
│ Owner: Trading Team  │  │ Owner: Security Team │  │ Owner: Finance Team  │
│                      │  │                      │  │                      │
│ ├─ kek-exchange-v1   │  │ ├─ kek-2fa-v1        │  │ ├─ kek-billing-v1    │
│ └─ kek-exchange-v2   │  │ └─ kek-2fa-v2        │  │ └─ kek-payments-v1   │
└──────────────────────┘  └──────────────────────┘  └──────────────────────┘

✅ Separation of Duties
✅ Limited blast radius
✅ Different teams = different PINs
```

**Configuration:**

```yaml
# config.yaml (new schema)
hsm:
  pkcs11_lib: /usr/lib/softhsm/libsofthsm2.so
  
  # Legacy mode (backward compatible)
  # slot_id: hsm-token  # If specified, ignores slots section
  
  # Multi-slot mode (new)
  slots:
    trading:
      label: slot-exchange
      pin_env: HSM_PIN_TRADING
      contexts:
        - exchange-key
        - order-key
    
    security:
      label: slot-2fa
      pin_env: HSM_PIN_2FA
      contexts:
        - 2fa
        - mfa
    
    finance:
      label: slot-billing
      pin_env: HSM_PIN_BILLING
      contexts:
        - billing
        - payments
```

**Implementation:**

```go
// internal/hsm/slot_manager.go (new file)
package hsm

type SlotManager interface {
    GetKeyForContext(context string) (cipher.AEAD, error)
    GetAllContexts() []string
    ReloadKeys() error
    Close() error
}

// Single slot (legacy)
type SingleSlotManager struct {
    ctx  *crypto11.Context
    keys map[string][]cipher.AEAD
}

// Multi-slot (new)
type MultiSlotManager struct {
    slots   map[string]*SlotContext  // slot name -> context
    mapping map[string]string         // context -> slot name
}

type SlotContext struct {
    name     string
    ctx      *crypto11.Context
    keys     map[string][]cipher.AEAD
    contexts []string
}

// Factory function (auto-detects mode)
func NewSlotManager(cfg *config.HSMConfig, metadata *config.Metadata) (SlotManager, error) {
    if len(cfg.Slots) > 0 {
        log.Println("Using multi-slot mode")
        return NewMultiSlotManager(cfg, metadata)
    }
    log.Println("Using single-slot mode (legacy)")
    return NewSingleSlotManager(cfg, metadata)
}
```

**Migration path:**

```bash
# Step 1: Create new slots
softhsm2-util --init-token --slot 1 --label slot-exchange --pin $PIN_EXCHANGE --so-pin $SOPIN
softhsm2-util --init-token --slot 2 --label slot-2fa --pin $PIN_2FA --so-pin $SOPIN

# Step 2: Migrate keys to new slots
hsm-admin migrate-to-multisite \
  --from-slot hsm-token \
  --from-pin 1234 \
  --config config-multisite.yaml \
  --dry-run

# Step 3: Update config and restart
docker restart hsm-service

# Step 4: Verify
hsm-admin list --verbose
# Output:
# Slot: slot-exchange (contexts: exchange-key, order-key)
#   ├── kek-exchange-key-v1 (v1, created: 2026-01-09)
#   └── kek-exchange-key-v2 (v2, created: 2026-02-08)
# Slot: slot-2fa (contexts: 2fa, mfa)
#   └── kek-2fa-v1 (v1, created: 2026-01-15)
```

**Security benefits:**
- ✅ **Isolation**: Compromise of trading PIN ≠ access to 2FA keys
- ✅ **Separation of Duties**: Different teams manage different slots
- ✅ **Audit trail**: Per-slot logging
- ✅ **PCI DSS 3.6.6**: Split knowledge compliance
- ✅ **Principle of Least Privilege**: Services only get PINs they need

**Effort:** 7 дней разработки + 3 дня тестирования

---

### 2.2 Hardware HSM Support 🟠 HIGH

**Текущее ограничение:**
- Only SoftHSM2 (software emulation)
- No FIPS 140-2 Level 3 compliance
- No physical tamper protection

**Целевые устройства:**

1. **Thales Luna HSM** (Network HSM)
   - FIPS 140-2 Level 3 certified
   - Common in enterprise
   - PKCS#11 interface

2. **Yubico YubiHSM 2** (USB HSM)
   - Affordable (~$650)
   - FIPS 140-2 Level 3
   - Good for small/medium deployments

3. **AWS CloudHSM** (Cloud HSM)
   - FIPS 140-2 Level 3
   - Managed service
   - PKCS#11 interface

**Implementation strategy:**

```go
// internal/hsm/provider.go (new file)
type HSMProvider interface {
    Initialize(config *HSMConfig) error
    CreateKey(label string, keyType KeyType, size int) error
    LoadKey(label string) (cipher.AEAD, error)
    ListKeys() ([]KeyInfo, error)
    Close() error
}

// SoftHSM provider (existing)
type SoftHSMProvider struct {
    ctx *crypto11.Context
}

// Luna HSM provider (new)
type LunaHSMProvider struct {
    partition string
    password  string
    ctx       *crypto11.Context
}

// YubiHSM provider (new)
type YubiHSMProvider struct {
    connector string
    authKey   uint16
    password  string
    session   *yubihsm.Session
}

// CloudHSM provider (new)
type CloudHSMProvider struct {
    clusterID string
    ipAddress string
    ctx       *crypto11.Context  // Uses PKCS#11
}

// Factory function
func NewHSMProvider(cfg *HSMConfig) (HSMProvider, error) {
    switch cfg.Type {
    case "softhsm":
        return NewSoftHSMProvider(cfg)
    case "luna":
        return NewLunaHSMProvider(cfg)
    case "yubihsm":
        return NewYubiHSMProvider(cfg)
    case "cloudhsm":
        return NewCloudHSMProvider(cfg)
    default:
        return nil, fmt.Errorf("unknown HSM type: %s", cfg.Type)
    }
}
```

**Configuration:**

```yaml
# config.yaml
hsm:
  type: luna  # softhsm | luna | yubihsm | cloudhsm
  
  # SoftHSM (existing)
  softhsm:
    pkcs11_lib: /usr/lib/softhsm/libsofthsm2.so
    slot_id: hsm-token
    pin_env: HSM_PIN
  
  # Thales Luna HSM (new)
  luna:
    pkcs11_lib: /usr/safenet/lunaclient/lib/libCryptoki2_64.so
    partition: partition1
    password_env: LUNA_PASSWORD
    server: 10.0.1.100
  
  # YubiHSM (new)
  yubihsm:
    connector: http://127.0.0.1:12345
    auth_key_id: 1
    password_env: YUBIHSM_PASSWORD
  
  # AWS CloudHSM (new)
  cloudhsm:
    pkcs11_lib: /opt/cloudhsm/lib/libcloudhsm_pkcs11.so
    cluster_id: cluster-xyz123
    pin_env: CLOUDHSM_PIN
```

**Testing strategy:**

```bash
# Unit tests with mocks
go test ./internal/hsm/provider_test.go

# Integration tests per provider
# tests/integration/softhsm_test.sh    # Existing
# tests/integration/luna_test.sh       # New (requires hardware)
# tests/integration/yubihsm_test.sh    # New
# tests/integration/cloudhsm_test.sh   # New (requires AWS)
```

**Documentation:**

```markdown
# guides/hardware_hsm_setup.md

## Thales Luna HSM Setup

### Prerequisites
- Luna Network HSM appliance
- lunacm client installed
- Network connectivity to HSM

### Step 1: Initialize partition
```bash
lunacm
> partition create -partition partition1 -password MySecurePassword
> partition showInfo -partition partition1
```

### Step 2: Configure HSM service
```yaml
hsm:
  type: luna
  luna:
    partition: partition1
    password_env: LUNA_PASSWORD
```

[... detailed steps ...]
```

**Effort:** 10 дней разработки + 5 дней тестирования + 3 дня документации

---

### 2.3 Audit API v1 🟡 MEDIUM

**Проблема:**
- Audit logs only in text format
- No structured queries
- No compliance reports
- Manual log parsing required

**Решение: REST Audit API**

**API Endpoints:**

```bash
# 1. Query audit events
GET /audit/events?from=2026-01-01T00:00:00Z&to=2026-02-08T23:59:59Z&context=exchange-key&limit=100

Response:
{
  "total": 1543,
  "events": [
    {
      "timestamp": "2026-02-08T10:30:15.123456Z",
      "event_type": "encrypt",
      "client_cn": "cts-trading-service-1",
      "context": "exchange-key",
      "key_label": "kek-exchange-key-v2",
      "status": "success",
      "duration_ms": 2.5,
      "metadata": {
        "plaintext_size": 64,
        "ciphertext_size": 92
      }
    },
    // ... more events
  ]
}

# 2. Generate compliance reports
GET /audit/reports/pci-dss-3.6.4?period=90d

Response:
{
  "report_type": "PCI-DSS-3.6.4",
  "title": "Key Rotation Compliance Report",
  "period": {
    "from": "2025-11-09T00:00:00Z",
    "to": "2026-02-08T23:59:59Z"
  },
  "generated_at": "2026-02-08T12:00:00Z",
  "compliance_status": "COMPLIANT",
  "summary": {
    "total_contexts": 3,
    "rotations_required": 3,
    "rotations_completed": 3,
    "overdue_rotations": 0
  },
  "details": [
    {
      "context": "exchange-key",
      "current_version": "kek-exchange-key-v2",
      "last_rotation": "2026-01-16T10:30:00Z",
      "days_since_rotation": 23,
      "rotation_interval_days": 90,
      "next_rotation_due": "2026-04-16T10:30:00Z",
      "status": "COMPLIANT"
    },
    // ...
  ]
}

# 3. Export for SIEM
GET /audit/export?format=json&from=2026-02-01&to=2026-02-08

Response: NDJSON stream
{"timestamp":"2026-02-08T10:30:15Z","event":"encrypt","client":"..."}
{"timestamp":"2026-02-08T10:30:16Z","event":"decrypt","client":"..."}
...

# 4. Statistics
GET /audit/stats?period=24h

Response:
{
  "period": "24h",
  "total_requests": 15234,
  "by_operation": {
    "encrypt": 8123,
    "decrypt": 7089,
    "health": 22
  },
  "by_context": {
    "exchange-key": 12456,
    "2fa": 2756
  },
  "by_client": {
    "cts-trading-service-1": 5678,
    "web-ui-2fa-service": 2756,
    // ...
  },
  "errors": {
    "total": 23,
    "rate": 0.15  // %
  }
}
```

**Storage backend:**

```go
// internal/audit/store.go
type AuditStore interface {
    LogEvent(event *AuditEvent) error
    QueryEvents(query *Query) ([]*AuditEvent, error)
    GenerateReport(reportType string, params map[string]string) (*Report, error)
    Export(format string, from, to time.Time) (io.Reader, error)
}

// SQLite implementation (for small deployments)
type SQLiteAuditStore struct {
    db *sql.DB
}

// PostgreSQL implementation (for large deployments)
type PostgreSQLAuditStore struct {
    db *sql.DB
}

// Schema
CREATE TABLE audit_events (
    id BIGSERIAL PRIMARY KEY,
    timestamp TIMESTAMP NOT NULL,
    event_type VARCHAR(50) NOT NULL,
    client_cn VARCHAR(255) NOT NULL,
    context VARCHAR(100),
    key_label VARCHAR(255),
    status VARCHAR(20),
    duration_ms DECIMAL(10,3),
    error_message TEXT,
    metadata JSONB,
    INDEX idx_timestamp (timestamp),
    INDEX idx_context (context),
    INDEX idx_client (client_cn)
);
```

**Configuration:**

```yaml
# config.yaml
audit:
  enabled: true
  
  # Storage backend
  store:
    type: sqlite  # sqlite | postgresql | file
    
    # SQLite (good for <10k events/day)
    sqlite:
      path: /data/audit.db
      max_size_mb: 1000
      retention_days: 365
    
    # PostgreSQL (for high volume)
    # postgresql:
    #   host: postgres.internal
    #   port: 5432
    #   database: hsm_audit
    #   user: hsm_service
    #   password_env: DB_PASSWORD
  
  # Sampling (optional, for high load)
  sampling:
    enabled: false
    rate: 0.1  # Log 10% of successful requests
    always_log_errors: true
```

**PCI DSS Compliance Reports:**

```bash
# Generate standard reports
GET /audit/reports/pci-dss-3.6.4  # Key rotation
GET /audit/reports/pci-dss-10.2   # User activity tracking
GET /audit/reports/pci-dss-10.3   # Record retention

# Custom report with filters
GET /audit/reports/custom?from=2026-01-01&to=2026-02-08&context=exchange-key
```

**Access control for audit API:**

```yaml
# config.yaml
audit:
  api:
    enabled: true
    require_mtls: true
    
    # ACL for audit endpoints
    allowed_ou:
      - Audit        # Compliance officers
      - Security     # Security team
      - Admin        # Admins
```

**Effort:** 8 дней разработки + 3 дня тестирования + 2 дня документации

---

### 2.4 Metrics Listener Separation 🟡 MEDIUM

**Проблема:**
- `/metrics` обслуживается на основном API listener (`:8443`)
- Метрики зависят от runtime-пути основного API и его middleware цепочки
- Сложнее изолировать scrape-трафик сетевыми политиками

**Решение:**
- Вынести метрики на отдельный listener и порт (например `:9090`)
- Добавить отдельную секцию конфигурации:

```yaml
metrics:
  enabled: true
  port: 9090
  path: /metrics
```

- Оставить бизнес API на `server.port`, а Prometheus endpoint на `metrics.port`
- Ограничить доступ к `metrics.port` только из monitoring-сети/compose network

**Критерии успеха:**
- [ ] Метрики доступны на отдельном порту без влияния на основной API
- [ ] Prometheus scrape работает после перезапуска сервиса
- [ ] Политики сети блокируют внешний доступ к `metrics.port`

**Effort:** 2 дня разработки + 1 день тестирования + 0.5 дня документации

---

## PHASE 3: High Availability & Compliance (Q3 2026)

### 3.1 High Availability (Active-Active) 🔴 CRITICAL

**Текущее состояние:**
- Single instance = Single Point of Failure
- No failover
- No load balancing
- Downtime during updates

**Целевая архитектура:**

```
                    ┌─────────────────┐
                    │  Load Balancer  │
                    │  (HAProxy/Nginx)│
                    └────────┬────────┘
                             │
         ┌───────────────────┼───────────────────┐
         │                   │                   │
    ┌────▼────┐         ┌────▼────┐         ┌────▼────┐
    │ HSM-1   │◄────────►│ HSM-2   │◄────────►│ HSM-3   │
    │ Node    │  State   │ Node    │  State   │ Node    │
    │         │  Sync    │         │  Sync    │         │
    └────┬────┘         └────┬────┘         └────┬────┘
         │                   │                   │
         └───────────────────┼───────────────────┘
                             │
                    ┌────────▼────────┐
                    │  etcd Cluster   │
                    │  (Coordination) │
                    └─────────────────┘
```

**Ключевые компоненты:**

#### 3.1.1 Distributed Configuration

```yaml
# config.yaml
cluster:
  enabled: true
  mode: active-active
  node_id: hsm-node-1  # Unique per node
  
  # Leader election & coordination
  coordination:
    type: etcd
    endpoints:
      - etcd1.internal:2379
      - etcd2.internal:2379
      - etcd3.internal:2379
    tls:
      ca_cert: /pki/ca.crt
      client_cert: /pki/etcd-client.crt
      client_key: /pki/etcd-client.key
  
  # Token/key replication
  replication:
    enabled: true
    # Options:
    # 1. Share same HSM network device (Luna Network HSM)
    # 2. Replicate SoftHSM tokens between nodes (encrypted)
    method: shared_hsm  # shared_hsm | replicated_tokens
    
    # If replicated_tokens:
    encryption_key_env: TOKEN_REPLICATION_KEY
    sync_interval_seconds: 60
```

#### 3.1.2 State Synchronization

```go
// internal/cluster/coordinator.go
type Coordinator interface {
    // Registration
    RegisterNode(nodeID string, address string) error
    DeregisterNode(nodeID string) error
    
    // Health
    Heartbeat(nodeID string) error
    GetHealthyNodes() ([]NodeInfo, error)
    
    // Metadata sync
    WatchMetadataChanges() (<-chan *Metadata, error)
    PublishMetadataChange(metadata *Metadata) error
    
    // Leader election (for maintenance tasks)
    ElectLeader() (string, error)
    IsLeader() bool
}

// etcd implementation
type EtcdCoordinator struct {
    client *clientv3.Client
    nodeID string
    ttl    time.Duration
}
```

#### 3.1.3 Hot Reload Synchronization

```go
// When rotation happens on one node:
func (km *KeyManager) RotateKey(context string) error {
    // 1. Rotate locally
    if err := km.performRotation(context); err != nil {
        return err
    }
    
    // 2. Publish to cluster
    if km.coordinator != nil {
        if err := km.coordinator.PublishMetadataChange(km.metadata); err != nil {
            log.Printf("ERROR: Failed to publish rotation to cluster: %v", err)
            // Continue - local rotation succeeded
        }
    }
    
    return nil
}

// All nodes listen for changes:
func (km *KeyManager) watchClusterChanges() {
    changes, err := km.coordinator.WatchMetadataChanges()
    if err != nil {
        log.Printf("ERROR: Failed to watch metadata changes: %v", err)
        return
    }
    
    for metadata := range changes {
        log.Printf("Received metadata update from cluster")
        if err := km.ReloadKeys(); err != nil {
            log.Printf("ERROR: Failed to reload after cluster update: %v", err)
        }
    }
}
```

#### 3.1.4 Load Balancer Configuration

```nginx
# nginx.conf
upstream hsm_cluster {
    least_conn;  # Route to node with fewest connections
    
    server hsm-node-1:8443 max_fails=3 fail_timeout=30s;
    server hsm-node-2:8443 max_fails=3 fail_timeout=30s;
    server hsm-node-3:8443 max_fails=3 fail_timeout=30s;
}

server {
    listen 443 ssl http2;
    server_name hsm.internal;
    
    # mTLS configuration
    ssl_certificate /pki/lb.crt;
    ssl_certificate_key /pki/lb.key;
    ssl_client_certificate /pki/ca.crt;
    ssl_verify_client on;
    
    location / {
        proxy_pass https://hsm_cluster;
        proxy_ssl_verify on;
        proxy_ssl_trusted_certificate /pki/ca.crt;
        
        # Health checks
        proxy_next_upstream error timeout http_503;
        proxy_connect_timeout 2s;
        proxy_send_timeout 5s;
        proxy_read_timeout 10s;
    }
    
    location /health {
        proxy_pass https://hsm_cluster;
        access_log off;  # Don't log health checks
    }
}
```

#### 3.1.5 Deployment

```yaml
# docker-compose-ha.yml
version: '3.8'

services:
  etcd-1:
    image: quay.io/coreos/etcd:v3.5
    # ... etcd config
  
  etcd-2:
    image: quay.io/coreos/etcd:v3.5
  
  etcd-3:
    image: quay.io/coreos/etcd:v3.5
  
  hsm-node-1:
    image: hsm-service:v1.3.0
    environment:
      - CLUSTER_ENABLED=true
      - CLUSTER_NODE_ID=hsm-node-1
      - ETCD_ENDPOINTS=etcd-1:2379,etcd-2:2379,etcd-3:2379
    volumes:
      - hsm-tokens-shared:/var/lib/softhsm/tokens:rw  # Shared volume
    
  hsm-node-2:
    image: hsm-service:v1.3.0
    environment:
      - CLUSTER_ENABLED=true
      - CLUSTER_NODE_ID=hsm-node-2
    volumes:
      - hsm-tokens-shared:/var/lib/softhsm/tokens:rw
  
  hsm-node-3:
    image: hsm-service:v1.3.0
    environment:
      - CLUSTER_ENABLED=true
      - CLUSTER_NODE_ID=hsm-node-3
    volumes:
      - hsm-tokens-shared:/var/lib/softhsm/tokens:rw
  
  nginx-lb:
    image: nginx:alpine
    ports:
      - "8443:443"
    volumes:
      - ./nginx.conf:/etc/nginx/nginx.conf:ro

volumes:
  hsm-tokens-shared:  # Shared NFS/GlusterFS volume
```

**Benefits:**
- ✅ Zero downtime updates (rolling restart)
- ✅ Automatic failover (nginx detects failed nodes)
- ✅ Horizontal scaling (add more nodes)
- ✅ Disaster recovery (multi-node survival)

**Effort:** 15 дней разработки + 5 дней тестирования + 3 дня документации

---

### 3.2 Split Knowledge (Shamir Secret Sharing) 🟠 HIGH

**PCI DSS 3.6.6 Requirement:**
> "Cryptographic keys shall be stored in the fewest possible locations and forms.
> Key-encryption keys shall be protected by using at least one of the following:
> - **Split knowledge** (requires two or more people)"

**Текущая проблема:**
- Single administrator knows HSM PIN
- No protection against insider threat
- Single point of compromise

**Решение: M-of-N Key Ceremony**

```
┌─────────────────────────────────────────────────────────────┐
│               Shamir Secret Sharing (3-of-5)                │
│                                                             │
│  Master PIN = "MySecureP1n2023!"                           │
│                                                             │
│  Split into 5 shares:                                      │
│  ├─ Share 1 → Admin Alice     (Trading Team)              │
│  ├─ Share 2 → Admin Bob       (Security Team)             │
│  ├─ Share 3 → Admin Charlie   (Finance Team)              │
│  ├─ Share 4 → Admin Diana     (Compliance)                │
│  └─ Share 5 → Admin Eve       (IT Operations)             │
│                                                             │
│  To unlock HSM: Need ANY 3 out of 5 shares                │
│                                                             │
│  Example combinations:                                     │
│  ✅ Alice + Bob + Charlie     → PIN reconstructed          │
│  ✅ Bob + Diana + Eve         → PIN reconstructed          │
│  ❌ Alice + Bob               → Insufficient (need 3)      │
│  ❌ Single admin              → Impossible                 │
└─────────────────────────────────────────────────────────────┘
```

**Implementation:**

```go
// cmd/hsm-admin/shamir.go (new command)
package main

import (
    "github.com/hashicorp/vault/shamir"
)

// Generate shares from master PIN
func generateSharesCommand() {
    fs := flag.NewFlagSet("generate-shares", flag.ExitOnError)
    numShares := fs.Int("shares", 5, "Total number of shares")
    threshold := fs.Int("threshold", 3, "Minimum shares to reconstruct")
    outputDir := fs.String("output", "/secure/shares", "Output directory")
    
    fs.Parse(os.Args[2:])
    
    // Read master PIN securely
    fmt.Print("Enter master PIN: ")
    masterPIN, err := term.ReadPassword(int(os.Stdin.Fd()))
    if err != nil {
        log.Fatalf("Failed to read PIN: %v", err)
    }
    
    // Generate shares using Shamir's algorithm
    shares, err := shamir.Split(masterPIN, *numShares, *threshold)
    if err != nil {
        log.Fatalf("Failed to split secret: %v", err)
    }
    
    // Encrypt each share with admin's public key
    for i, share := range shares {
        adminKey := fmt.Sprintf("admin-%d-public.pem", i+1)
        encryptedShare, err := encryptWithPublicKey(share, adminKey)
        if err != nil {
            log.Fatalf("Failed to encrypt share %d: %v", i+1, err)
        }
        
        // Save to file
        filename := filepath.Join(*outputDir, fmt.Sprintf("share-%d.enc", i+1))
        if err := os.WriteFile(filename, encryptedShare, 0600); err != nil {
            log.Fatalf("Failed to write share %d: %v", i+1, err)
        }
        
        fmt.Printf("✅ Share %d generated: %s\n", i+1, filename)
    }
    
    // Zero master PIN from memory
    for i := range masterPIN {
        masterPIN[i] = 0
    }
    
    fmt.Printf("\n✅ Generated %d shares (threshold: %d)\n", *numShares, *threshold)
    fmt.Println("Distribute shares to different administrators securely.")
}

// Reconstruct PIN from shares
func reconstructPINCommand() {
    fs := flag.NewFlagSet("reconstruct-pin", flag.ExitOnError)
    shareFiles := fs.String("shares", "", "Comma-separated share files")
    adminKeys := fs.String("keys", "", "Comma-separated admin private keys")
    
    fs.Parse(os.Args[2:])
    
    // Parse inputs
    shareList := strings.Split(*shareFiles, ",")
    keyList := strings.Split(*adminKeys, ",")
    
    if len(shareList) != len(keyList) {
        log.Fatal("Number of shares must match number of keys")
    }
    
    // Decrypt shares
    var decryptedShares [][]byte
    for i, shareFile := range shareList {
        // Read encrypted share
        encShare, err := os.ReadFile(shareFile)
        if err != nil {
            log.Fatalf("Failed to read share %s: %v", shareFile, err)
        }
        
        // Decrypt with admin's private key
        fmt.Printf("Admin %d, enter your private key password: ", i+1)
        password, _ := term.ReadPassword(int(os.Stdin.Fd()))
        fmt.Println()
        
        share, err := decryptWithPrivateKey(encShare, keyList[i], password)
        if err != nil {
            log.Fatalf("Failed to decrypt share %d: %v", i+1, err)
        }
        
        decryptedShares = append(decryptedShares, share)
    }
    
    // Reconstruct PIN using Shamir's algorithm
    masterPIN, err := shamir.Combine(decryptedShares)
    if err != nil {
        log.Fatalf("Failed to reconstruct PIN: %v", err)
    }
    
    // Use PIN to unlock HSM
    if err := unlockHSM(string(masterPIN)); err != nil {
        log.Fatalf("Failed to unlock HSM: %v", err)
    }
    
    // Zero sensitive data
    for i := range masterPIN {
        masterPIN[i] = 0
    }
    for _, share := range decryptedShares {
        for i := range share {
            share[i] = 0
        }
    }
    
    fmt.Println("✅ HSM unlocked successfully")
}
```

**Key Ceremony Procedure:**

```bash
# Step 1: Generate master PIN shares (one-time setup)
hsm-admin generate-shares \
  --shares 5 \
  --threshold 3 \
  --output /secure/shares

# Output:
# ✅ Share 1 generated: /secure/shares/share-1.enc (for Alice)
# ✅ Share 2 generated: /secure/shares/share-2.enc (for Bob)
# ✅ Share 3 generated: /secure/shares/share-3.enc (for Charlie)
# ✅ Share 4 generated: /secure/shares/share-4.enc (for Diana)
# ✅ Share 5 generated: /secure/shares/share-5.enc (for Eve)

# Step 2: Distribute shares securely
# - Hand-deliver USB drives to each admin
# - Require physical access to 3 admins to unlock HSM

# Step 3: Unlock HSM (requires 3 admins present)
# Alice, Bob, and Charlie must be physically present:
hsm-admin reconstruct-pin \
  --shares share-1.enc,share-2.enc,share-3.enc \
  --keys alice.key,bob.key,charlie.key

# Prompts each admin for their private key password
# Reconstructs PIN and unlocks HSM
```

**Audit trail:**

```json
{
  "timestamp": "2026-08-15T10:30:00Z",
  "event": "hsm_unlock",
  "method": "shamir_secret_sharing",
  "participants": ["alice", "bob", "charlie"],
  "threshold": 3,
  "total_shares": 5,
  "result": "success"
}
```

**PCI DSS Compliance:**
- ✅ Requirement 3.6.6: Split knowledge implemented
- ✅ Requirement 12.3.8: Security awareness (key ceremony training)
- ✅ Dual control: Multiple people required for critical operations

**Effort:** 7 дней разработки + 3 дня тестирования + 2 дня процедурной документации

---

### 3.3 PCI DSS Full Compliance Certification 🟡 MEDIUM

**Текущий статус:**

| Requirement | Status | Gap |
|-------------|--------|-----|
| 3.5.1 - Encrypted storage | ✅ | None |
| 3.6.1 - KEK management | ✅ | None |
| 3.6.4 - Key rotation | ✅ | Automated |
| 3.6.6 - Split knowledge | 🟡 | Phase 3.2 |
| 3.7 - Key access restriction | ✅ | mTLS + ACL |
| 10.2 - User activity tracking | ✅ | Full audit |
| 10.3 - Tamper protection | 🟡 | Add checksums |
| 12.3 - Security policies | 🟠 | Need documentation |

**Action items:**

#### 3.3.1 Documentation Package

```
docs/
  compliance/
    PCI_DSS_v4.0_Compliance_Matrix.md     # Requirement mapping
    Security_Policies.md                   # Formal policies
    Key_Management_Policy.md               # KEK lifecycle
    Access_Control_Policy.md               # Who can do what
    Incident_Response_Plan.md              # Breach procedures
    Business_Continuity_Plan.md            # DR procedures
    
  audit/
    Quarterly_Review_Template.md           # For auditors
    Key_Rotation_Report_Template.md        # Compliance evidence
    Access_Log_Analysis.md                 # Usage patterns
```

#### 3.3.2 Automated Compliance Checks

```bash
# scripts/compliance-check.sh
#!/bin/bash
set -euo pipefail

echo "=== PCI DSS Compliance Check ==="

# Check 1: Key rotation (3.6.4) - must rotate within 90 days
echo "Checking key rotation compliance..."
OLDEST_KEY_DAYS=$(hsm-admin list --format json | jq '.[] | .days_since_creation' | sort -rn | head -1)
if [ "$OLDEST_KEY_DAYS" -gt 90 ]; then
    echo "❌ FAIL: Key rotation overdue ($OLDEST_KEY_DAYS days)"
    exit 1
fi
echo "✅ PASS: All keys rotated within 90 days"

# Check 2: Audit logs (10.2) - must be enabled
echo "Checking audit logging..."
if ! grep -q "audit.*enabled.*true" /app/config.yaml; then
    echo "❌ FAIL: Audit logging not enabled"
    exit 1
fi
echo "✅ PASS: Audit logging enabled"

# Check 3: TLS 1.3 (4.1) - no weak TLS
echo "Checking TLS configuration..."
if grep -q "MinVersion.*VersionTLS12" /app/*.go; then
    echo "❌ FAIL: TLS 1.2 allowed (must use TLS 1.3)"
    exit 1
fi
echo "✅ PASS: TLS 1.3 enforced"

# Check 4: Split knowledge (3.6.6) - if configured
if grep -q "shamir.*enabled.*true" /app/config.yaml; then
    THRESHOLD=$(yq '.shamir.threshold' /app/config.yaml)
    if [ "$THRESHOLD" -lt 2 ]; then
        echo "❌ FAIL: Shamir threshold too low (must be ≥2)"
        exit 1
    fi
    echo "✅ PASS: Split knowledge enabled (threshold: $THRESHOLD)"
else
    echo "⚠️  WARNING: Split knowledge not enabled (optional for PCI DSS 3.6.6)"
fi

# Check 5: Access control (7.1) - ACL enabled
echo "Checking access control..."
if ! grep -q "acl:" /app/config.yaml; then
    echo "❌ FAIL: ACL not configured"
    exit 1
fi
echo "✅ PASS: ACL configured"

echo ""
echo "=== Compliance Check Complete ==="
echo "✅ All critical checks passed"
```

**Run compliance check daily:**
```yaml
# kubernetes CronJob
apiVersion: batch/v1
kind: CronJob
metadata:
  name: pci-dss-compliance-check
spec:
  schedule: "0 6 * * *"  # Daily at 6am
  jobTemplate:
    spec:
      template:
        spec:
          containers:
          - name: compliance-check
            image: hsm-service:v1.3.0
            command: ["/scripts/compliance-check.sh"]
            volumeMounts:
            - name: config
              mountPath: /app/config.yaml
```

**Effort:** 5 дней документации + 2 дня автоматизации

---

## PHASE 4: Advanced Features (Q4 2026)

### 4.1 Web Admin UI (Optional) 🟢 LOW

**Justification:**
- CLI is primary interface (for automation)
- Web UI for convenience and visualization
- Reduces onboarding time for new operators

**Features:**

```
┌────────────────────────────────────────────────────────────┐
│  HSM Service Admin Dashboard                    [Logout]   │
├────────────────────────────────────────────────────────────┤
│                                                            │
│  📊 Overview                                               │
│  ┌──────────────┐  ┌──────────────┐  ┌──────────────┐    │
│  │   15,234     │  │      23      │  │     99.97%   │    │
│  │   Requests   │  │    Errors    │  │   Uptime     │    │
│  │   (24h)      │  │    (24h)     │  │   (30d)      │    │
│  └──────────────┘  └──────────────┘  └──────────────┘    │
│                                                            │
│  🔑 Keys                                                   │
│  ┌────────────────────────────────────────────────────┐   │
│  │ Context      │ Current Version │ Last Rotation     │   │
│  ├────────────────────────────────────────────────────┤   │
│  │ exchange-key │ kek-...-v2      │ 23 days ago   ✅  │   │
│  │ 2fa          │ kek-2fa-v1      │ 15 days ago   ✅  │   │
│  │ billing      │ kek-bill-v3     │ 85 days ago   ⚠️   │   │
│  └────────────────────────────────────────────────────┘   │
│                                                            │
│  📈 Request Rate (1h)                                     │
│  [Line chart showing RPS over time]                       │
│                                                            │
│  📝 Recent Events                                         │
│  ┌────────────────────────────────────────────────────┐   │
│  │ 10:30:15  encrypt   cts-trading-1   success  2.5ms │   │
│  │ 10:30:16  decrypt   web-2fa         success  1.8ms │   │
│  │ 10:30:17  encrypt   cts-trading-2   success  2.1ms │   │
│  └────────────────────────────────────────────────────┘   │
└────────────────────────────────────────────────────────────┘
```

**Tech stack:**
- Frontend: React + TypeScript
- Backend: Go (REST API)
- Auth: mTLS (same PKI as main service)
- Charts: Recharts or Chart.js

**Effort:** 10 дней frontend + 3 дня backend + 2 дня integration

---

## 📊 Prioritization Matrix

### Критерии приоритезации

| Feature | Security Impact | PCI DSS | Complexity | ROI | Priority |
|---------|----------------|---------|------------|-----|----------|
| **Test Coverage** | ✅ Closed | ✅ Yes | ✅ Done | ✅ Delivered | ✅ Completed |
| **Backup Automation** | 🔴 High | ✅ Yes | 🟡 Medium | 🔴 High | 🔴 P0 |
| **Hardware HSM** | 🟠 Medium | ✅ Yes | 🔴 High | 🟠 Medium | 🟠 P1 |
| **HA/Clustering** | 🟠 Medium | 🟡 Partial | 🔴 High | 🔴 High | 🟠 P1 |
| **Multi-Slot** | 🟡 Low | ✅ Yes | 🟡 Medium | 🟡 Low | 🟡 P2 |
| **Split Knowledge** | 🟡 Low | ✅ Yes | 🟡 Medium | 🟡 Low | 🟡 P2 |
| **Audit API** | 🟡 Low | ✅ Yes | 🟡 Medium | 🟠 Medium | 🟡 P2 |
| **CLI Consolidation** | 🟢 None | ❌ No | 🟢 Low | 🟢 Low | 🟢 P3 |
| **Web UI** | 🟢 None | ❌ No | 🔴 High | 🟢 Low | 🟢 P4 |

---

## 📅 Рекомендованная последовательность

### Sprint 1-2 (Февраль 2026) - CRITICAL (завершено)
- ✅ Этап test coverage закрыт (internal/hsm + cmd/hsm-admin выше целевых порогов)
- ✅ Базовый security/compliance pipeline внедрен
- ✅ Документация и quality gates синхронизированы

### Sprint 3-4 (Март 2026) - HIGH
- Hardware HSM support (Luna/YubiHSM)
- Multi-slot architecture (foundation)
- Audit API v1 (basic queries)

### Sprint 5-6 (Апрель-Май 2026) - HIGH
- High Availability (2-node cluster)
- Load balancer configuration
- Integration tests for HA

### Sprint 7-8 (Июнь-Июль 2026) - MEDIUM
- Split knowledge (Shamir)
- PCI DSS full compliance audit
- Performance optimization

### Sprint 9-10 (Август-Октябрь 2026) - LOW
- CLI consolidation
- Web Admin UI (optional)
- Advanced metrics & dashboards

---

## ✅ Критерии готовности (Definition of Done)

Для каждой фичи:

- [ ] **Code complete**: Implementation done, code review passed
- [ ] **Tests**: Unit tests >80% coverage, integration tests passing
- [ ] **Documentation**: User guide + API docs + runbook updated
- [ ] **Security review**: No new vulnerabilities introduced
- [ ] **Performance**: No degradation (benchmark comparison)
- [ ] **Backward compatibility**: Existing deployments not broken
- [ ] **CHANGELOG**: Updated with release notes
- [ ] **Demo**: Working demo for stakeholders

---

## 📚 Appendix: Compliance Mapping

### PCI DSS v4.0 Requirements

| Requirement | Current Status | Target (v2.0) | Implementation |
|-------------|---------------|---------------|----------------|
| **3.5.1** - Encrypted storage | ✅ Compliant | ✅ | AES-256-GCM |
| **3.6.1** - KEK protection | ✅ Compliant | ✅ | HSM non-extractable |
| **3.6.4** - Key rotation | ✅ Compliant | ✅ | 90-day rotation |
| **3.6.6** - Split knowledge | 🟡 Optional | ✅ | Shamir (Phase 3.2) |
| **3.7** - Access restriction | ✅ Compliant | ✅ | mTLS + ACL |
| **4.1** - Strong crypto | ✅ Compliant | ✅ | TLS 1.3 only |
| **10.2** - User tracking | ✅ Compliant | ✅ | Full audit logs |
| **10.3** - Tamper protection | 🟡 Partial | ✅ | KEK checksums |
| **12.3** - Security policies | 🟠 Needs work | ✅ | Documentation (Phase 3.3) |
| **12.10** - BC/DR plan | 🟠 Needs work | ✅ | HA + Backup (Phase 1.3, 3.1) |

### NIST Cybersecurity Framework

| Function | Category | Current | Target |
|----------|----------|---------|--------|
| **Identify** | Asset Management | 🟡 Partial | ✅ |
| **Protect** | Access Control | ✅ Strong | ✅ |
| **Protect** | Data Security | ✅ Strong | ✅ |
| **Detect** | Anomalies & Events | 🟡 Partial | ✅ |
| **Respond** | Incident Response | 🟠 Needs work | ✅ |
| **Recover** | Recovery Planning | 🟠 Needs work | ✅ |

---

## 🎯 Success Metrics

### Technical KPIs

- **Availability**: 99.95% → 99.99% (with HA)
- **Test Coverage**: 40% → 85%
- **Security Score**: 9.5/10 → 10/10
- **MTTR**: 30min → 5min (automated recovery)
- **Audit Compliance**: 80% → 100%

### Business KPIs

- **Deployment Time**: 2h → 15min (automation)
- **Onboarding Time**: 1 week → 1 day (Web UI)
- **Audit Preparation**: 2 weeks → 2 days (automated reports)
- **DR Test Time**: 4h → 30min (automated procedures)

---

> **Next Steps:**
> 1. ✅ Review and approve this plan
> 2. ✅ Create GitHub issues/milestones
> 3. ✅ Assign resources to Sprint 1-2
> 4. ✅ Phase 1 (Test Coverage) завершен, перейти к Phase 3/4 задачам по HA и compliance decomposition
> 5. ✅ Schedule quarterly reviews

**Prepared by**: AI Assistant  
**Date**: February 8, 2026  
**Version**: 1.0
