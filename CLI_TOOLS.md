# 🛠️ HSM Admin CLI - Command Reference

> **Для DevOps/Operators**: Полная документация по hsm-admin CLI tool

## Оглавление

- [Введение](#введение)
- [Установка и настройка](#установка-и-настройка)
- [Команды](#команды)
- [Примеры использования](#примеры-использования)
- [Troubleshooting](#troubleshooting)

---

## Введение

`hsm-admin` - это CLI утилита для управления KEK (Key Encryption Keys) в HSM Service.

### Основные возможности

- ✅ Создание новых KEK
- ✅ Просмотр списка KEK
- ✅ Удаление KEK
- ✅ Ротация ключей
- ✅ Проверка статуса ротации
- ✅ Очистка старых версий
- ✅ Обновление checksums
- ✅ Экспорт metadata

---

## Установка и настройка

### Сборка

```bash
cd /opt/hsm-service
go build -o hsm-admin ./cmd/hsm-admin
```

### Перемещение в PATH (опционально)

```bash
sudo mv hsm-admin /usr/local/bin/
```

### Настройка environment

```bash
# HSM PIN (обязательно!)
export HSM_PIN=1234

# Config path (опционально, по умолчанию: config.yaml)
export CONFIG_PATH=/etc/hsm-service/config.yaml
```

### Файл конфигурации

Создать `~/.hsm-admin.env`:

```bash
HSM_PIN=your-secure-pin
CONFIG_PATH=/etc/hsm-service/config.yaml
```

Загрузка:
```bash
source ~/.hsm-admin.env
```

**Примечание**: Все остальные параметры (PKCS11 библиотека, slot label, metadata path) берутся из `config.yaml`.

**Примечание**: Все остальные параметры (PKCS11 библиотека, slot label, metadata path) берутся из `config.yaml`.

---

## Команды

### `create-kek` (отдельная утилита)

Создать новый KEK (Key Encryption Key) в HSM.

**⚠️ Примечание**: Это **отдельная утилита**, не команда `hsm-admin`. См. [cmd/create-kek/](cmd/create-kek/) для исходного кода.

**Синтаксис**:
```bash
create-kek <label> <pin> [version]
```

**Параметры**:
- `<label>` (обязательный) - уникальное имя KEK (например: `kek-exchange-key-v1`)
- `<pin>` (обязательный) - PIN токена HSM
- `[version]` (опционально) - версия ключа (по умолчанию: 1)

**Пример**:
```bash
# Create KEK for exchange key (version 1)
/opt/hsm-service/bin/create-kek "kek-exchange-key-v1" "1234" 1

# Create KEK for 2FA (version 1)
/opt/hsm-service/bin/create-kek "kek-2fa-v1" "1234" 1

# Using environment variable
export HSM_PIN=1234
/opt/hsm-service/bin/create-kek "kek-exchange-key-v2" "$HSM_PIN" 2
```

**Output**:
```
Generating 256-bit AES key...
Label: kek-exchange-key-v1
ID: 1a2b3c4d5e6f7g8h
Handle: 5
✓ KEK created successfully
```

**Когда использовать**:
- При первоначальной настройке HSM Service
- При создании новой версии KEK
- При инициализации нового HSM токена
- Вызывается автоматически во время `hsm-admin rotate`

---

### `list-kek`

Показать все KEK в HSM.

**Синтаксис**:
```bash
hsm-admin list-kek [--context <context>]
```

**Параметры**:
- `--context` (опционально) - фильтр по контексту

**Пример**:
```bash
# All KEKs
./hsm-admin list-kek

# Only exchange-key KEKs
./hsm-admin list-kek --context exchange-key
```

**Output**:
```
Total KEKs: 4

Label                 | Context      | Handle | Created At          | Status
----------------------|--------------|--------|---------------------|--------
kek-exchange-key-v1       | exchange-key | 5      | 2024-01-01 10:00:00 | active
kek-exchange-key-v2       | exchange-key | 8      | 2024-03-15 14:30:00 | active
kek-2fa-v1            | 2fa          | 6      | 2024-01-01 10:05:00 | active
kek-2fa-v2            | 2fa          | 9      | 2024-03-20 11:00:00 | active
```

**Когда использовать**:
- Проверка существующих ключей
- Аудит ключей
- Перед ротацией
- Перед созданием нового KEK

---

### `delete-kek`

Удалить KEK из HSM.

**⚠️ ОПАСНО**: Удаление KEK сделает невозможным расшифровку данных, зашифрованных этим ключом!

**Синтаксис**:
```bash
hsm-admin delete-kek --label <label> --confirm
```

**Параметры**:
- `--label` (обязательный) - имя KEK для удаления
- `--confirm` (обязательный) - подтверждение удаления

**Пример**:
```bash
# Удаление (требует флаг --confirm)
./hsm-admin delete-kek --label kek-old-v1 --confirm

# Без --confirm выдаст ошибку:
./hsm-admin delete-kek --label kek-old-v1
# Error: --confirm flag is required to delete KEK
# This operation is irreversible!
```

**Когда использовать**:
- После завершения `cleanup-old-versions` (автоматически вызывается)
- При удалении неиспользуемого контекста
- При ошибочном создании KEK

**⚠️ НЕ ИСПОЛЬЗУЙТЕ**, если:
- KEK все еще используется для расшифровки
- Не прошел период retention (по умолчанию 30 дней после ротации)

---

### `rotate`

Выполнить ротацию ключа для контекста.

**Синтаксис**:
```bash
hsm-admin rotate <context>
```

**Параметры**:
- `<context>` (обязательный) - контекст для ротации

**Пример**:
```bash
export HSM_PIN=1234

# Rotate exchange-key
./hsm-admin rotate exchange-key

# Rotate 2fa
./hsm-admin rotate 2fa
```

**Output**:
```
Starting rotation for context: exchange-key
Loaded metadata with 2 contexts
Creating new KEK: kek-exchange-key-v3
Executing: /app/create-kek kek-exchange-key-v3 1234 3
Created metadata backup: metadata.yaml.backup-20260115-143000
✓ Key rotation completed:
  Context: exchange-key
  Old key: kek-exchange-key-v2 (version 2)
  New key: kek-exchange-key-v3 (version 3)

⚠️  IMPORTANT:
  1. Restart the HSM service to load the new key
  2. Re-encrypt all data encrypted with the old key
  3. After 7 days overlap period, delete the old key:
     hsm-admin delete kek-exchange-key-v2
```

**Что происходит**:
1. Создается новая версия KEK (v3)
2. Metadata обновляется с датой ротации
3. Новый KEK становится активным для шифрования
4. Старые KEK остаются для расшифровки
5. Через `cleanup_after_days` старые версии удаляются (кроме `max_versions` последних)

**Когда использовать**:
- Плановая ротация (рекомендуется: каждые 90 дней)
- После security incident
- При подозрении на компрометацию ключа
- Перед major upgrades

---

### `rotation-status`

Показать статус ротации для всех контекстов.

**Синтаксис**:
```bash
hsm-admin rotation-status
```

**Параметры**:
- Нет параметров (команда показывает все контексты)

**Пример**:
```bash
# Показать статус всех контекстов
./hsm-admin rotation-status
```

**Output**:
```
Key Rotation Status:
====================

✓ Context: exchange-key
  Current:           kek-exchange-key-v2
  Version:           2
  Total Versions:    2
  Created:           2026-01-09 14:30:00
  Rotation Interval: 2160h0m0s
  Next Rotation:     2026-04-09
  Status:            OK (89 days remaining)

⚠️  Context: 2fa
  Current:           kek-2fa-v1
  Version:           1
  Total Versions:    1
  Created:           2025-10-10 10:30:00
  Rotation Interval: 2160h0m0s
  Next Rotation:     2026-01-08
  Status:            NEEDS ROTATION (7 days overdue)
```

**Когда использовать**:
- Регулярный аудит ключей
- Планирование ротаций
- Проверка compliance (например: ключи < 90 дней)

---

### `cleanup-old-versions`

Удалить старые версии KEK (кроме `max_versions` последних).

**Синтаксис**:
```bash
hsm-admin cleanup-old-versions [--dry-run]
```

**Параметры**:
- `--dry-run` (опционально) - показать что будет удалено, но не удалять

**Пример**:
```bash
# Dry run (preview)
./hsm-admin cleanup-old-versions --dry-run

# Вывод:
# === PCI DSS Key Cleanup ===
# Max versions to keep: 3
# Delete versions older than: 30 days
# DRY RUN MODE - No changes will be made
#
# Context: exchange-key (current: kek-exchange-key-v3)
#   ⚠ kek-exchange-key-v1 (v1) - created 2025-10-15 - TOO OLD
#   [DRY-RUN] Would delete kek-exchange-key-v1 (v1)
#   Summary: kept 2, deleted 1
#
# Context: 2fa (current: kek-2fa-v2)
#   ✓ No versions to delete
#
# DRY RUN COMPLETE - Would delete 1 versions

# Actual cleanup
./hsm-admin cleanup-old-versions

# Вывод:
# === PCI DSS Key Cleanup ===
# Max versions to keep: 3
# Delete versions older than: 30 days
#
# Context: exchange-key (current: kek-exchange-key-v3)
#   ⚠ kek-exchange-key-v1 (v1) - created 2025-10-15 - TOO OLD
#   Delete 1 versions? (yes/no): yes
#   ✓ Deleted kek-exchange-key-v1 (v1) from HSM
#   Summary: kept 2, deleted 1
#
# ✓ Old metadata backed up to: metadata.yaml.backup.20260115-143500
# ✓ Metadata updated: metadata.yaml
#
# CLEANUP COMPLETE - Deleted 1 versions
```

**Логика cleanup**:
- Сохраняются последние `max_versions` версий (по умолчанию: 3)
- Удаляются только KEK старше `cleanup_after_days` дней (по умолчанию: 30)
- Текущая (активная) версия никогда не удаляется

**Когда использовать**:
- Автоматически вызывается после ротации (рекомендуется)
- Manual cleanup если нужно освободить место в HSM
- После изменения `max_versions` в config

**⚠️ Осторожно**: убедитесь, что нет данных зашифрованных удаляемыми KEK!

---

### `update-checksums`

Пересчитать и обновить checksums для всех KEK.

**Синтаксис**:
```bash
hsm-admin update-checksums
```

**Пример**:
```bash
./hsm-admin update-checksums
```

**Output**:
```
Computing KEK checksums...

Context: exchange-key
  ✓ kek-exchange-key-v1 (v1): checksum already up-to-date (a1b2c3d4...)
  ✓ kek-exchange-key-v2 (v2): checksum already up-to-date (x1y2z3w4...)

Context: 2fa
  + kek-2fa-v1 (v1): NEW checksum m1n2o3p4...

Saving updated metadata to metadata.yaml...
✓ Updated 1 checksum(s) successfully

Next steps:
1. Restart HSM service to verify checksums on startup
2. Commit metadata.yaml to version control
```

**Когда использовать**:
- После восстановления из backup
- При подозрении на corrupted metadata
- После manual изменений в HSM

---

### `export-metadata`

Экспортировать metadata в JSON формате.

**Синтаксис**:
```bash
hsm-admin export-metadata [--output <file>]
```

**Параметры**:
- `--output` (опционально) - файл для сохранения (по умолчанию: stdout)

**Пример**:
```bash
# Output to stdout
./hsm-admin export-metadata

# Save to file
./hsm-admin export-metadata --output /tmp/metadata-export.json
```

**Output (JSON)**:
```json
{
  "exported_at": "2024-03-25T15:30:00Z",
  "contexts": [
    {
      "name": "exchange-key",
      "current_version": "v3",
      "versions": [
        {
          "version": "v3",
          "label": "kek-exchange-key-v3",
          "created_at": "2024-03-15T14:30:00.000000Z",
          "checksum": "a1b2c3d4e5f6...",
          "status": "active"
        },
        {
          "version": "v2",
          "label": "kek-exchange-key-v2",
          "created_at": "2024-02-01T10:00:00.000000Z",
          "checksum": "x1y2z3w4v5...",
          "status": "old"
        }
      ],
      "last_rotation": "2024-03-15T14:30:00Z",
      "next_cleanup": "2024-04-15T14:30:00Z"
    }
  ]
}
```

**Когда использовать**:
- Audit logging
- Integration с другими системами
- Backup metadata в JSON формате
- Отчетность

---

## Примеры использования

### Сценарий 1: Начальная настройка

```bash
# 1. Create initial KEKs using create-kek utility
/opt/hsm-service/bin/create-kek "kek-exchange-key-v1" "1234" 1
/opt/hsm-service/bin/create-kek "kek-2fa-v1" "1234" 1

# 2. Verify with hsm-admin
export HSM_PIN=1234
./hsm-admin list-kek

# 3. Update checksums
./hsm-admin update-checksums

# 4. Export metadata for backup
./hsm-admin export-metadata --output /backup/initial-metadata.json
```

---

### Сценарий 2: Плановая ротация

```bash
# 1. Check current status
./hsm-admin rotation-status

# Вывод показывает:
# exchange-key: last rotation 85 days ago → нужна ротация!

# 2. Perform rotation
export HSM_PIN=1234
./hsm-admin rotate exchange-key

# 3. Verify new version
./hsm-admin list-kek

# 4. Cleanup будет автоматически через 30 дней
# Или manual:
./hsm-admin cleanup-old-versions --dry-run
```

---

### Сценарий 3: Добавление нового контекста

```bash
# 1. Update config.yaml
nano /etc/hsm-service/config.yaml

# Добавить:
# hsm:
#   keys:
#     new-service:
#       type: aes
#
# acl:
#   mappings:
#     NewServiceGroup:
#       - new-service

# 2. Create KEK using create-kek utility
export HSM_PIN=1234
/opt/hsm-service/bin/create-kek "kek-new-service-v1" "$HSM_PIN" 1

# 3. Verify with hsm-admin
./hsm-admin list-kek

# 4. Restart service
sudo systemctl restart hsm-service

# 5. Verify
curl -k https://localhost:8443/health ... | jq .active_keys
# Should include "kek-new-service-v1"
```

---

### Сценарий 4: Disaster Recovery

```bash
# 1. Restore HSM tokens from backup
sudo tar -xzf tokens-backup.tar.gz -C /var/lib/softhsm/

# 2. Verify KEKs present
./hsm-admin list-kek

# 3. Update checksums (если нужно)
./hsm-admin update-checksums

# 4. Check rotation status
./hsm-admin rotation-status

# 5. Start service
sudo systemctl start hsm-service
```

---

### Сценарий 5: Security Incident Response

```bash
# Подозрение на компрометацию ключа exchange-key

# 1. IMMEDIATE: Rotate compromised key
export HSM_PIN=1234
./hsm-admin rotate exchange-key

# 2. Update ACL to revoke compromised clients
echo "  - compromised-client" | sudo tee -a /etc/hsm-service/pki/revoked.yaml

# 3. Verify new KEK active
./hsm-admin list-kek

# 4. Notify clients to re-encrypt with new key
# (Application-level procedure)

# 5. After retention period - cleanup old KEK
# (Automatic after 30 days)
```

---

### Сценарий 6: Аудит ключей

```bash
# Monthly audit script

#!/bin/bash
echo "=== HSM KEK Audit Report ==="
echo "Date: $(date)"
echo ""

# 1. List all KEKs
echo "1. All KEKs:"
./hsm-admin list-kek
echo ""

# 2. Rotation status
echo "2. Rotation Status:"
./hsm-admin rotation-status
echo ""

# 3. Check for old keys (> 90 days)
echo "3. Keys older than 90 days:"
./hsm-admin rotation-status | grep "days ago" | awk '$5 > 90'
echo ""

# 4. Export metadata
echo "4. Exporting metadata..."
./hsm-admin export-metadata --output "/audit/metadata-$(date +%Y%m%d).json"
echo "Saved to: /audit/metadata-$(date +%Y%m%d).json"
```

---

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HSM_PIN` | (required) | HSM token PIN |
| `CONFIG_PATH` | `config.yaml` | Path to config.yaml file |

**Примечание**: Параметры `max_versions` и `cleanup_after_days` настраиваются в `config.yaml`, а не через environment variables.

---

## Troubleshooting

### Problem: CKR_PIN_INCORRECT

```bash
# Check HSM_PIN
echo $HSM_PIN

# Set correct PIN
export HSM_PIN=correct-pin
```

### Problem: Token not found

```bash
# List available tokens
softhsm2-util --show-slots

# Use correct label
export SLOT_LABEL=your-token-label
```

### Problem: Permission denied on metadata

```bash
# Fix ownership
sudo chown hsm:hsm /var/lib/hsm-service/metadata.yaml
sudo chmod 644 /var/lib/hsm-service/metadata.yaml
```

### Problem: KEK already exists

```bash
# List existing KEKs
./hsm-admin list-kek

# Use different label
./hsm-admin create-kek --label kek-exchange-key-v2 --context exchange-key
```

---

## Best Practices

### ✅ DO

- **Always** set `HSM_PIN` environment variable (не hardcode!)
- **Backup** metadata после каждой ротации
- **Test** `--dry-run` перед cleanup
- **Monitor** rotation status регулярно
- **Audit** KEKs ежемесячно
- **Document** ротации в change log

### ❌ DON'T

- **Never** delete current (active) KEK
- **Never** cleanup без проверки `--dry-run`
- **Never** share HSM PIN
- **Don't** rotate слишком часто (< 30 дней)
- **Don't** delete KEK если есть данные зашифрованные им

---

## Automation Examples

### Cron job для automatic rotation

**Примечание**: Рекомендуется использовать systemd timer вместо cron (см. [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md)).

```cron
# Rotate exchange-key every 90 days
0 3 1 */3 * cd /opt/hsm-service && export HSM_PIN=$(cat /etc/hsm-service/.pin) && ./hsm-admin rotate exchange-key >> /var/log/hsm-service/rotation.log 2>&1

# Cleanup old versions monthly
0 4 1 * * cd /opt/hsm-service && export HSM_PIN=$(cat /etc/hsm-service/.pin) && ./hsm-admin cleanup-old-versions >> /var/log/hsm-service/cleanup.log 2>&1
```

### Prometheus exporter для KEK metrics

```bash
#!/bin/bash
# kek-metrics.sh

KEK_COUNT=$(./hsm-admin list-kek | grep -c "kek-")
echo "hsm_kek_total $KEK_COUNT"

# Age of oldest KEK
# ... (parse from rotation-status)
```

---

## Additional Resources

- [API.md](API.md) - API documentation
- [KEY_ROTATION.md](KEY_ROTATION.md) - Подробное описание ротации
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Troubleshooting guide
- [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) - Production setup
