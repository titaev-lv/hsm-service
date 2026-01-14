# 💾 HSM Service - Backup & Disaster Recovery

> **Для DevOps/SRE**: Полное руководство по бэкапам, восстановлению и disaster recovery

## Оглавление

- [Что нужно бэкапить](#что-нужно-бэкапить)
- [Backup стратегия](#backup-стратегия)
- [Автоматические бэкапы](#автоматические-бэкапы)
- [Manual backup](#manual-backup)
- [Восстановление](#восстановление)
- [Disaster Recovery](#disaster-recovery)
- [Тестирование восстановления](#тестирование-восстановления)
- [Retention policy](#retention-policy)

---

## Что нужно бэкапить

### 1. SoftHSM Tokens (КРИТИЧНО!)

**Путь**: `/var/lib/softhsm/tokens/`

**Содержит**: 
- Все KEK ключи (AES-256)
- Metadata ключей
- PKCS#11 objects

**Важность**: ⚠️ **CRITICAL** - без этого невозможно расшифровать данные!

**Размер**: ~10-50 MB

**Частота бэкапов**: Ежедневно + после каждой ротации ключей

---

### 2. Metadata файл

**Путь**: `/var/lib/hsm-service/metadata.yaml`

**Содержит**:
- Информация о ротациях ключей
- Даты создания версий KEK
- Checksums

**Важность**: 🟡 **HIGH** - без этого сложно определить, какой KEK использовать

**Размер**: ~1-5 KB

**Частота бэкапов**: После каждой ротации

---

### 3. Configuration files

**Пути**:
- `/etc/hsm-service/config.yaml`
- `/etc/hsm-service/environment`
- `/etc/systemd/system/hsm-service.service`

**Содержит**:
- Конфигурация сервиса
- Environment variables (включая HSM_PIN - **осторожно!**)
- Systemd unit file

**Важность**: 🟢 **MEDIUM** - можно пересоздать вручную

**Размер**: ~10 KB

**Частота бэкапов**: При изменении

---

### 4. PKI Certificates

**Пути**:
- `/etc/hsm-service/pki/ca/ca.crt`
- `/etc/hsm-service/pki/server/`
- `/etc/hsm-service/pki/client/`
- `/opt/hsm-service/pki/inventory.yaml`
- `/opt/hsm-service/pki/revoked.yaml`

**Содержит**:
- CA сертификаты
- Server/client сертификаты
- Private keys
- Список выданных сертификатов
- Список отозванных сертификатов

**Важность**: 🟡 **HIGH** - можно перевыпустить, но трудозатратно

**Размер**: ~100-500 KB

**Частота бэкапов**: При изменении

---

### 5. Application logs (опционально)

**Пути**:
- `/var/log/hsm-service/`
- Systemd journal

**Важность**: 🔵 **LOW** - для audit trail

**Размер**: зависит от нагрузки (10MB-1GB/день)

**Частота бэкапов**: Ежедневно (если нужен audit trail)

---

## Backup Стратегия

### 3-2-1 Rule

- **3** копии данных
- **2** разных типа хранилища
- **1** копия offsite (за пределами data center)

### Типы бэкапов

| Тип | Когда | Retention | Что бэкапим |
|-----|-------|-----------|-------------|
| **Hot backup** | После каждой ротации | 7 дней | Tokens + metadata |
| **Daily backup** | Каждый день в 2 AM | 30 дней | Все |
| **Weekly backup** | Воскресенье | 12 недель | Все |
| **Monthly backup** | 1-е число месяца | 12 месяцев | Все |

---

## Автоматические бэкапы

### Script для полного бэкапа

Создать `/opt/hsm-service/scripts/backup.sh`:

```bash
#!/bin/bash
set -euo pipefail

# === Configuration ===
BACKUP_ROOT="/var/backups/hsm-service"
BACKUP_DIR="$BACKUP_ROOT/$(date +%Y-%m-%d)"
DATE=$(date +%Y%m%d-%H%M%S)
RETENTION_DAYS=30

# === Colors ===
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

log() {
    echo -e "${GREEN}[$(date +'%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

warn() {
    echo -e "${YELLOW}[WARN]${NC} $1"
}

# === Pre-flight checks ===
if [[ $EUID -ne 0 ]]; then
   error "This script must be run as root"
   exit 1
fi

# Create backup directory
mkdir -p "$BACKUP_DIR"
cd "$BACKUP_DIR"

log "Starting HSM Service backup..."

# === 1. Backup SoftHSM Tokens (MOST IMPORTANT) ===
log "Backing up SoftHSM tokens..."
if [[ -d /var/lib/softhsm/tokens ]]; then
    tar -czf "tokens-${DATE}.tar.gz" -C /var/lib/softhsm tokens/ 2>/dev/null
    chmod 600 "tokens-${DATE}.tar.gz"
    log "✓ Tokens backed up: $(du -h tokens-${DATE}.tar.gz | cut -f1)"
else
    error "SoftHSM tokens directory not found!"
    exit 1
fi

# === 2. Backup metadata ===
log "Backing up metadata..."
if [[ -f /var/lib/hsm-service/metadata.yaml ]]; then
    cp /var/lib/hsm-service/metadata.yaml "metadata-${DATE}.yaml"
    chmod 600 "metadata-${DATE}.yaml"
    log "✓ Metadata backed up"
else
    warn "Metadata file not found (may be first run)"
fi

# === 3. Backup configuration ===
log "Backing up configuration..."
mkdir -p config
if [[ -f /etc/hsm-service/config.yaml ]]; then
    cp /etc/hsm-service/config.yaml "config/config-${DATE}.yaml"
fi
if [[ -f /etc/hsm-service/environment ]]; then
    cp /etc/hsm-service/environment "config/environment-${DATE}"
    chmod 600 "config/environment-${DATE}"  # Contains HSM_PIN!
fi
if [[ -f /etc/systemd/system/hsm-service.service ]]; then
    cp /etc/systemd/system/hsm-service.service "config/hsm-service-${DATE}.service"
fi
log "✓ Configuration backed up"

# === 4. Backup PKI ===
log "Backing up PKI..."
if [[ -d /etc/hsm-service/pki ]]; then
    tar -czf "pki-${DATE}.tar.gz" -C /etc/hsm-service pki/ 2>/dev/null
    chmod 600 "pki-${DATE}.tar.gz"  # Contains private keys!
    log "✓ PKI backed up: $(du -h pki-${DATE}.tar.gz | cut -f1)"
fi
if [[ -f /opt/hsm-service/pki/inventory.yaml ]]; then
    cp /opt/hsm-service/pki/inventory.yaml "pki-inventory-${DATE}.yaml"
fi
if [[ -f /opt/hsm-service/pki/revoked.yaml ]]; then
    cp /opt/hsm-service/pki/revoked.yaml "pki-revoked-${DATE}.yaml"
fi

# === 5. Backup logs (опционально) ===
log "Backing up logs..."
if [[ -d /var/log/hsm-service ]]; then
    tar -czf "logs-${DATE}.tar.gz" /var/log/hsm-service/ 2>/dev/null || true
    log "✓ Logs backed up: $(du -h logs-${DATE}.tar.gz 2>/dev/null | cut -f1 || echo 'N/A')"
fi

# === 6. Create manifest ===
log "Creating backup manifest..."
cat > "MANIFEST-${DATE}.txt" <<EOF
HSM Service Backup Manifest
===========================
Date: $(date)
Hostname: $(hostname)
Backup Directory: $BACKUP_DIR

Files:
$(ls -lh)

MD5 Checksums:
$(md5sum * 2>/dev/null | grep -v "MANIFEST")

System Info:
- Service version: $(cd /opt/hsm-service && git describe --tags 2>/dev/null || echo "unknown")
- SoftHSM version: $(softhsm2-util --version 2>/dev/null | head -1)
- OS: $(cat /etc/os-release | grep PRETTY_NAME | cut -d= -f2)
EOF

log "✓ Manifest created"

# === 7. Encrypt backup (опционально) ===
# Uncomment if you want GPG encryption
# GPG_KEY="backup@example.com"
# log "Encrypting backup..."
# tar -czf - * | gpg --encrypt --recipient "$GPG_KEY" > "../backup-${DATE}.tar.gz.gpg"
# log "✓ Backup encrypted"

# === 8. Cleanup old backups ===
log "Cleaning up old backups (retention: ${RETENTION_DAYS} days)..."
find "$BACKUP_ROOT" -type d -mtime +${RETENTION_DAYS} -exec rm -rf {} + 2>/dev/null || true
REMOVED=$(find "$BACKUP_ROOT" -type d -mtime +${RETENTION_DAYS} 2>/dev/null | wc -l)
log "✓ Removed $REMOVED old backups"

# === 9. Verify backup ===
log "Verifying backup..."
if tar -tzf "tokens-${DATE}.tar.gz" > /dev/null 2>&1; then
    log "✓ Backup verification passed"
else
    error "Backup verification FAILED!"
    exit 1
fi

# === Summary ===
BACKUP_SIZE=$(du -sh "$BACKUP_DIR" | cut -f1)
log "Backup completed successfully!"
log "Location: $BACKUP_DIR"
log "Total size: $BACKUP_SIZE"

# === Optional: Upload to remote storage ===
# Uncomment and configure for S3/rsync/etc
# log "Uploading to remote storage..."
# aws s3 cp "$BACKUP_DIR" s3://my-bucket/hsm-backups/ --recursive
# rsync -avz "$BACKUP_DIR" backup-server:/backups/hsm-service/

exit 0
```

### Сделать executable

```bash
sudo chmod +x /opt/hsm-service/scripts/backup.sh
```

### Тестовый запуск

```bash
sudo /opt/hsm-service/scripts/backup.sh
```

---

### Настройка cron для автоматических бэкапов

```bash
sudo crontab -e
```

**Добавить**:
```cron
# Daily backup at 2 AM
0 2 * * * /opt/hsm-service/scripts/backup.sh >> /var/log/hsm-service/backup.log 2>&1

# Weekly backup at Sunday 3 AM (с тегом)
0 3 * * 0 /opt/hsm-service/scripts/backup.sh && cp -r /var/backups/hsm-service/$(date +\%Y-\%m-\%d) /var/backups/hsm-service/weekly-$(date +\%Y-\%m-\%d) >> /var/log/hsm-service/backup.log 2>&1

# Monthly backup on 1st day of month at 4 AM
0 4 1 * * /opt/hsm-service/scripts/backup.sh && cp -r /var/backups/hsm-service/$(date +\%Y-\%m-\%d) /var/backups/hsm-service/monthly-$(date +\%Y-\%m-\%d) >> /var/log/hsm-service/backup.log 2>&1
```

### Проверка cron jobs

```bash
# View cron jobs
sudo crontab -l

# Check cron logs
sudo journalctl -u cron | grep backup
```

---

## Manual Backup

### Quick backup before changes

```bash
#!/bin/bash
# quick-backup.sh

DATE=$(date +%Y%m%d-%H%M%S)
BACKUP_DIR="/tmp/hsm-backup-$DATE"

mkdir -p "$BACKUP_DIR"

# Tokens
sudo tar -czf "$BACKUP_DIR/tokens.tar.gz" /var/lib/softhsm/tokens/

# Metadata
sudo cp /var/lib/hsm-service/metadata.yaml "$BACKUP_DIR/"

# Config
sudo cp /etc/hsm-service/config.yaml "$BACKUP_DIR/"

echo "Backup saved to: $BACKUP_DIR"
ls -lh "$BACKUP_DIR"
```

---

## Восстановление

### Full Restore Procedure

```bash
#!/bin/bash
# restore.sh

set -e

BACKUP_DATE=$1

if [[ -z "$BACKUP_DATE" ]]; then
    echo "Usage: $0 <backup-date>"
    echo "Example: $0 2024-01-15"
    echo ""
    echo "Available backups:"
    ls -1 /var/backups/hsm-service/
    exit 1
fi

BACKUP_DIR="/var/backups/hsm-service/$BACKUP_DATE"

if [[ ! -d "$BACKUP_DIR" ]]; then
    echo "ERROR: Backup not found: $BACKUP_DIR"
    exit 1
fi

echo "=== HSM Service Restore ==="
echo "Backup date: $BACKUP_DATE"
echo "Backup location: $BACKUP_DIR"
echo ""
read -p "This will OVERWRITE current data. Continue? (yes/no): " confirm

if [[ "$confirm" != "yes" ]]; then
    echo "Restore cancelled."
    exit 0
fi

# === 1. Stop service ===
echo "Stopping HSM service..."
sudo systemctl stop hsm-service

# === 2. Restore tokens ===
echo "Restoring SoftHSM tokens..."
TOKENS_BACKUP=$(ls -t "$BACKUP_DIR"/tokens-*.tar.gz | head -1)
if [[ -f "$TOKENS_BACKUP" ]]; then
    sudo rm -rf /var/lib/softhsm/tokens/*
    sudo tar -xzf "$TOKENS_BACKUP" -C /var/lib/softhsm/
    sudo chown -R hsm:hsm /var/lib/softhsm/tokens
    sudo chmod 700 /var/lib/softhsm/tokens
    echo "✓ Tokens restored"
else
    echo "ERROR: Tokens backup not found!"
    exit 1
fi

# === 3. Restore metadata ===
echo "Restoring metadata..."
METADATA_BACKUP=$(ls -t "$BACKUP_DIR"/metadata-*.yaml | head -1)
if [[ -f "$METADATA_BACKUP" ]]; then
    sudo cp "$METADATA_BACKUP" /var/lib/hsm-service/metadata.yaml
    sudo chown hsm:hsm /var/lib/hsm-service/metadata.yaml
    sudo chmod 644 /var/lib/hsm-service/metadata.yaml
    echo "✓ Metadata restored"
fi

# === 4. Restore configuration ===
echo "Restoring configuration..."
if [[ -d "$BACKUP_DIR/config" ]]; then
    CONFIG_BACKUP=$(ls -t "$BACKUP_DIR"/config/config-*.yaml | head -1)
    if [[ -f "$CONFIG_BACKUP" ]]; then
        sudo cp "$CONFIG_BACKUP" /etc/hsm-service/config.yaml
        sudo chown hsm:hsm /etc/hsm-service/config.yaml
    fi
    
    ENV_BACKUP=$(ls -t "$BACKUP_DIR"/config/environment-* | head -1)
    if [[ -f "$ENV_BACKUP" ]]; then
        sudo cp "$ENV_BACKUP" /etc/hsm-service/environment
        sudo chown root:hsm /etc/hsm-service/environment
        sudo chmod 640 /etc/hsm-service/environment
    fi
    
    echo "✓ Configuration restored"
fi

# === 5. Restore PKI ===
echo "Restoring PKI..."
PKI_BACKUP=$(ls -t "$BACKUP_DIR"/pki-*.tar.gz | head -1)
if [[ -f "$PKI_BACKUP" ]]; then
    sudo rm -rf /etc/hsm-service/pki/*
    sudo tar -xzf "$PKI_BACKUP" -C /etc/hsm-service/
    sudo chown -R hsm:hsm /etc/hsm-service/pki
    sudo chmod 600 /etc/hsm-service/pki/server/*.key
    sudo chmod 600 /etc/hsm-service/pki/client/*.key
    echo "✓ PKI restored"
fi

# === 6. Verify HSM token ===
echo "Verifying HSM token..."
if sudo -u hsm softhsm2-util --show-slots | grep -q "hsm-token"; then
    echo "✓ HSM token verified"
else
    echo "ERROR: HSM token not found after restore!"
    exit 1
fi

# === 7. Start service ===
echo "Starting HSM service..."
sudo systemctl start hsm-service

# === 8. Wait for service ===
echo "Waiting for service to start..."
sleep 5

# === 9. Verify service ===
echo "Verifying service..."
if sudo systemctl is-active --quiet hsm-service; then
    echo "✓ Service is running"
    
    # Test health endpoint
    if curl -k -s https://localhost:8443/health \
        --cert /etc/hsm-service/pki/client/client1.crt \
        --key /etc/hsm-service/pki/client/client1.key \
        --cacert /etc/hsm-service/pki/ca/ca.crt | grep -q "healthy"; then
        echo "✓ Health check passed"
    else
        echo "WARN: Health check failed"
    fi
else
    echo "ERROR: Service failed to start!"
    echo "Check logs: journalctl -u hsm-service -n 50"
    exit 1
fi

echo ""
echo "=== Restore Complete ==="
echo "Service restored from backup: $BACKUP_DATE"
echo ""
echo "Next steps:"
echo "1. Test encrypt/decrypt operations"
echo "2. Check logs: journalctl -u hsm-service -f"
echo "3. Verify all clients can connect"
```

### Сделать executable

```bash
sudo chmod +x /opt/hsm-service/scripts/restore.sh
```

### Запуск restore

```bash
# Список доступных бэкапов
ls -1 /var/backups/hsm-service/

# Восстановление
sudo /opt/hsm-service/scripts/restore.sh 2024-01-15
```

---

## Disaster Recovery

### DR Plan

**RTO (Recovery Time Objective)**: 4 часа  
**RPO (Recovery Point Objective)**: 24 часа (daily backups)

### DR Scenarios

#### Scenario 1: Полный отказ сервера

**Recovery steps**:

1. Подготовить новый сервер (см. [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md))
2. Установить HSM Service
3. Восстановить из последнего бэкапа
4. Обновить DNS/load balancer
5. Тестирование

**Estimated time**: 2-3 часа

#### Scenario 2: Потеря SoftHSM tokens

**Impact**: 🔴 **CRITICAL** - невозможно расшифровать данные!

**Recovery**:
```bash
# Восстановить из последнего бэкапа
sudo /opt/hsm-service/scripts/restore.sh <latest-backup>
```

**Prevention**:
- Ежедневные бэкапы
- Offsite backups
- 3-2-1 rule
- Тестирование восстановления

#### Scenario 3: Corrupted metadata.yaml

**Impact**: 🟡 **MEDIUM** - сложно определить актуальный KEK

**Recovery**:
```bash
# Restore metadata from backup
sudo cp /var/backups/hsm-service/latest/metadata-*.yaml \
  /var/lib/hsm-service/metadata.yaml

sudo systemctl restart hsm-service
```

#### Scenario 4: Expired certificates

**Recovery**:
```bash
# Перевыпустить сертификаты
cd /opt/hsm-service/pki
./scripts/issue-server-cert.sh hsm-service.example.com
./scripts/issue-client-cert.sh client-name GroupName

# Restart service
sudo systemctl restart hsm-service
```

---

## Тестирование восстановления

### DR Test Procedure

**Частота**: Ежеквартально (каждые 3 месяца)

**Checklist**:

```markdown
# DR Test Checklist

## Preparation
- [ ] Schedule test during maintenance window
- [ ] Notify stakeholders
- [ ] Prepare test server (VM or bare metal)
- [ ] Verify latest backup exists

## Test Steps
- [ ] Deploy clean server
- [ ] Install HSM Service
- [ ] Restore from backup
- [ ] Verify service starts
- [ ] Test /health endpoint
- [ ] Test encrypt operation
- [ ] Test decrypt operation (старых данных)
- [ ] Verify KEK versions
- [ ] Test ACL checks
- [ ] Verify metrics endpoint

## Verification
- [ ] All KEKs present
- [ ] Metadata correct
- [ ] Configuration correct
- [ ] Certificates valid
- [ ] Logs working
- [ ] Monitoring working

## Measurements
- Restore time: ___ minutes
- Time to first request: ___ minutes
- Total DR time: ___ hours

## Issues Found
- Issue 1: ___
- Issue 2: ___

## Actions
- Action 1: ___
- Action 2: ___

## Sign-off
- Tested by: ___
- Date: ___
- Result: PASS / FAIL
```

---

## Retention Policy

### Backup Types

| Type | Frequency | Retention | Storage |
|------|-----------|-----------|---------|
| Hot | After rotation | 7 days | Local SSD |
| Daily | 2 AM | 30 days | Local HDD |
| Weekly | Sunday | 12 weeks | Network storage |
| Monthly | 1st of month | 12 months | S3/Object storage |
| Yearly | Jan 1st | 7 years | Glacier/Tape |

### Storage Requirements

**Calculation**:
- Single backup: ~50 MB
- Daily for 30 days: 1.5 GB
- Weekly for 12 weeks: 600 MB
- Monthly for 12 months: 600 MB
- **Total**: ~3 GB

**Recommendation**: 20 GB storage для бэкапов

---

## Offsite Backups

### Option 1: S3/Object Storage

```bash
# Install AWS CLI
apt install -y awscli

# Configure
aws configure

# Upload backup
aws s3 sync /var/backups/hsm-service/ \
  s3://my-bucket/hsm-backups/ \
  --storage-class STANDARD_IA \
  --sse AES256
```

**Add to backup.sh**:
```bash
# After backup creation
log "Uploading to S3..."
aws s3 sync "$BACKUP_DIR" \
  "s3://my-bucket/hsm-backups/$(date +%Y-%m-%d)/" \
  --storage-class STANDARD_IA
```

### Option 2: Rsync to remote server

```bash
# Setup SSH key
ssh-keygen -t ed25519 -f /root/.ssh/backup_key -N ""
ssh-copy-id -i /root/.ssh/backup_key backup@backup-server

# Rsync backup
rsync -avz -e "ssh -i /root/.ssh/backup_key" \
  /var/backups/hsm-service/ \
  backup@backup-server:/backups/hsm-service/
```

### Option 3: Encrypted upload

```bash
# Encrypt and upload
tar -czf - /var/backups/hsm-service/ | \
  gpg --encrypt --recipient backup@example.com | \
  curl -T - https://backup-server/upload/hsm-backup-$(date +%Y%m%d).tar.gz.gpg
```

---

## Monitoring Backups

### Check backup status

```bash
#!/bin/bash
# check-backups.sh

BACKUP_DIR="/var/backups/hsm-service"
WARN_AGE=2  # days

# Find latest backup
LATEST=$(find "$BACKUP_DIR" -maxdepth 1 -type d -name "20*" | sort -r | head -1)
LATEST_DATE=$(basename "$LATEST")

# Calculate age
LATEST_EPOCH=$(date -d "$LATEST_DATE" +%s)
NOW_EPOCH=$(date +%s)
AGE_DAYS=$(( ($NOW_EPOCH - $LATEST_EPOCH) / 86400 ))

echo "Latest backup: $LATEST_DATE ($AGE_DAYS days old)"

if [[ $AGE_DAYS -gt $WARN_AGE ]]; then
    echo "WARNING: Backup is old!"
    exit 1
else
    echo "OK: Backup is recent"
    exit 0
fi
```

### Prometheus metric для backup age

```promql
# Age of latest backup (days)
(time() - hsm_last_backup_timestamp) / 86400

# Alert if > 2 days
alert: BackupTooOld
expr: (time() - hsm_last_backup_timestamp) / 86400 > 2
```

---

## Security Considerations

### Backup Encryption

```bash
# Encrypt backup with GPG
tar -czf - /var/lib/softhsm/tokens/ | \
  gpg --encrypt \
      --recipient backup-key@example.com \
      --output tokens-encrypted.tar.gz.gpg

# Decrypt
gpg --decrypt tokens-encrypted.tar.gz.gpg | tar -xzf -
```

### Backup Access Control

```bash
# Only root can read backups
chmod 700 /var/backups/hsm-service
chown root:root /var/backups/hsm-service

# Individual files
chmod 600 /var/backups/hsm-service/*/tokens-*.tar.gz
chmod 600 /var/backups/hsm-service/*/config/environment-*
```

### Audit Logging

```bash
# Log all backup/restore operations
logger -t hsm-backup "Backup started by $(whoami)"
logger -t hsm-restore "Restore started by $(whoami) from $BACKUP_DATE"
```

---

## Troubleshooting

### Problem: Backup fails

```bash
# Check disk space
df -h /var/backups

# Check permissions
ls -la /var/backups/hsm-service

# Check cron logs
journalctl -u cron | grep backup
```

### Problem: Restore fails

```bash
# Check backup integrity
tar -tzf tokens-*.tar.gz

# Check permissions
ls -la /var/lib/softhsm/tokens

# Check service logs
journalctl -u hsm-service -n 100
```

---

## Checklist перед Production

- [ ] Автоматические daily backups настроены
- [ ] Offsite backups настроены
- [ ] Backup retention policy определена
- [ ] DR plan документирован
- [ ] DR test проведен успешно
- [ ] Monitoring для backup age настроен
- [ ] Alerts для failed backups настроены
- [ ] Restore procedure протестирована
- [ ] Team обучен процедурам DR

---

## Additional Resources

- [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) - Production deployment
- [MONITORING.md](MONITORING.md) - Backup monitoring
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Troubleshooting restore issues
