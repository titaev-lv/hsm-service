# Процедура ротации KEK

## Обзор

Этот документ описывает процедуру ротации KEK (Key Encryption Key) для HSM Service. Ротация ключей — критически важная практика безопасности, требуемая стандартом PCI DSS Requirement 3.6.4.

## Архитектура конфигурации

Конфигурация HSM Service разделена на два файла:

### config.yaml (статическая конфигурация)
- Хранится в Git (Infrastructure as Code)
- Содержит неизменяемые параметры: contexts, ACL правила, алгоритмы шифрования
- **НЕ содержит** версии ключей и даты создания

```yaml
hsm:
  keys:
    exchange-key:
      type: aes
      mode: shared
    2fa:
      type: aes
      mode: private
```

### metadata.yaml (динамические метаданные ротации)
- **НЕ хранится в Git** (.gitignore)
- Управляется автоматикой (hsm-admin, скрипты ротации)
- Содержит: текущую версию ключа, историю версий, даты создания
- **Обновляется при каждой ротации**

```yaml
rotation:
  exchange-key:
    current: kek-exchange-v2      # Текущий активный ключ
    rotation_interval_days: 90    # Период ротации
    versions:
      - label: kek-exchange-v2    # Новый ключ
        version: 2
        created_at: '2026-01-09T14:30:00Z'
      - label: kek-exchange-v1    # Старый ключ (период overlap)
        version: 1
        created_at: '2025-10-10T10:00:00Z'
```

## 🔥 Zero-Downtime Hot Reload

HSM service автоматически перезагружает метаданные без перезапуска:

1. **Background monitor** проверяет `metadata.yaml` каждые 30 секунд
2. **Обнаружено изменение?** → Загрузить и валидировать новые метаданные
3. **Загрузить новые KEK** из HSM (через существующую PKCS#11 сессию)
4. **Atomic swap** - обновление ключей одновременно
5. **Старые версии** остаются доступными для расшифровки

**Мониторинг reload (Docker):**
```bash
docker compose logs -f hsm-service | grep "reload"
# {"level":"INFO","msg":"KEK hot reload successful","contexts":2,"total_keys":3}
```

**Мониторинг reload (Production):**
```bash
journalctl -u hsm-service -f | grep "reload"
# {"level":"INFO","msg":"metadata file changed","path":"/app/metadata.yaml"}
# {"level":"INFO","msg":"KEK hot reload successful","contexts":2,"total_keys":3}
```

## Политика ротации

- **Интервал по умолчанию**: 90 дней (PCI DSS Requirement 3.6.4)
- **Период перекрытия**: 7-30 дней (оба ключа работают одновременно)
- **Версионирование**: kek-exchange-v1 → kek-exchange-v2 → kek-exchange-v3
- **Автоматическая очистка**: Старые версии удаляются через `cleanup_after_days` (config.yaml)

## Проверка статуса ротации

**Docker:**
```bash
docker exec hsm-service /app/hsm-admin rotation-status
```

**Production (systemd):**
```bash
sudo /usr/local/bin/hsm-admin rotation-status
```

**Пример вывода:**
```
Key Rotation Status:
====================

✓ Context: exchange-key
  Label:             kek-exchange-v2
  Version:           2
  Created:           2026-01-09 14:30:00
  Rotation Interval: 90 days
  Next Rotation:     2026-04-09
  Status:            OK (89 days remaining)

⚠️  Context: 2fa
  Label:             kek-2fa-v1
  Version:           1
  Created:           2025-10-10 10:30:00
  Rotation Interval: 90 days
  Next Rotation:     2026-01-08
  Status:            NEEDS ROTATION (7 days overdue)
```

## Процедура ручной ротации

### Шаг 1: Проверка ключей, требующих ротации

См. команду выше для проверки статуса.

### Шаг 2: Ротация ключа

**Docker:**
```bash
docker exec hsm-service /app/hsm-admin rotate exchange-key
```

**Production:**
```bash
sudo -E /usr/local/bin/hsm-admin rotate exchange-key
```

**Что происходит:**
1. Генерация нового номера версии (v1 → v2)
2. Создание нового KEK в HSM с меткой `kek-exchange-v2`
3. **Обновление metadata.yaml** (добавление новой версии в список)
4. Создание резервной копии `metadata.yaml.backup-TIMESTAMP`
5. **config.yaml НЕ изменяется** (он статический)

**Пример вывода:**
```
Creating new KEK: kek-exchange-v2
✓ Created KEK: kek-exchange-v2 (handle: 3, ID: 02, version: 2)
✓ Updated metadata.yaml with new version
Created backup: metadata.yaml.backup-20260109-143000

⚠️  NEXT STEPS:
  1. Wait 30 seconds for automatic hot reload (NO RESTART NEEDED)
  2. Monitor /health endpoint to verify new key is loaded
  3. Application will automatically use new key for encryption
  4. Old key remains available for decryption (overlap period)
  5. After overlap period: clean up old versions
```

### Шаг 3: Автоматическая Hot Reload (Zero Downtime)

**Не требуется перезагрузка!** Сервис автоматически обнаружит изменения metadata.yaml в течение 30 секунд.

**Проверка reload (Docker):**
```bash
# Подождать 35 секунд для гарантии
sleep 35

# Проверить логи
docker compose logs --since 40s hsm-service | grep "KEK hot reload"
```

**Проверка reload (Production):**
```bash
sleep 35
journalctl -u hsm-service --since "40 seconds ago" | grep "KEK hot reload"
```

**Ожидаемый вывод:**
```
{"time":"2026-01-09T14:30:45Z","level":"INFO","msg":"metadata file changed"}
{"time":"2026-01-09T14:30:45Z","level":"INFO","msg":"KEK hot reload successful","contexts":2,"total_keys":3}
```

**Проверка через /health:**
```bash
# Docker
curl -sk https://localhost:8443/health \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt | jq '.kek_status'

# Production
curl -sk https://hsm-service.local:8443/health \
  --cert /etc/hsm/pki/client.crt \
  --key /etc/hsm/pki/client.key \
  --cacert /etc/hsm/pki/ca.crt | jq '.kek_status'
```

**Ожидаемый результат:**
```json
{
  "kek-2fa-v1": "available",
  "kek-exchange-v1": "available",
  "kek-exchange-v2": "available"
}
```

**Примечание:** Если требуется немедленная перезагрузка (< 30 сек):
- Docker: `docker compose restart hsm-service`
- Production: `sudo systemctl restart hsm-service`

### Шаг 4: Перешифрование данных приложениями

**ВАЖНО:** После ротации ключа приложения должны перешифровать старые данные новым ключом.

#### Как приложение узнает о необходимости перешифрования?

**Вариант 1: Мониторинг metadata.yaml (рекомендуется для production)**

Приложение периодически проверяет API endpoint или metadata.yaml:

```bash
# Приложение делает периодический запрос
curl -sk https://hsm-service.local:8443/health | jq -r '.kek_status | keys[]'

# Вывод показывает все доступные версии:
# kek-exchange-v1
# kek-exchange-v2  ← новая версия обнаружена!
# kek-2fa-v1
```

Приложение сравнивает с известной версией из БД:
```sql
-- Проверить, есть ли данные с устаревшей версией
SELECT COUNT(*) FROM encrypted_data 
WHERE key_version = 1 AND context = 'exchange-key';
-- Если > 0, запустить перешифрование
```

**Вариант 2: Event-driven (webhook/message queue)**

Скрипт ротации отправляет событие после успешной ротации:

```bash
# В auto-rotate-keys.sh после успешной ротации
if [[ $ROTATION_SUCCESS == true ]]; then
    # Отправить событие в Kafka/RabbitMQ
    echo '{"event":"key_rotated","context":"exchange-key","old_version":1,"new_version":2}' | \
      kafkacat -b kafka:9092 -t hsm-rotation-events
    
    # Или вызвать webhook
    curl -X POST https://trading-app/api/webhooks/key-rotation \
      -H "Content-Type: application/json" \
      -d '{"context":"exchange-key","new_version":2}'
fi
```

Приложение подписывается на событие и запускает перешифрование.

**Вариант 3: Планировщик (простой подход)**

Приложение запускает проверку по расписанию (cron):

```bash
# Ежедневная проверка в 02:00
0 2 * * * /app/scripts/check-and-reencrypt.sh
```

#### Процесс перешифрования

**Стратегия 1: Batch re-encryption (большие объемы)**

```python
#!/usr/bin/env python3
# Пример скрипта перешифрования для trading application

import requests
import psycopg2
from datetime import datetime

HSM_URL = "https://hsm-service.local:8443"
CERT = "/etc/app/certs/client.crt"
KEY = "/etc/app/certs/client.key"
CA = "/etc/app/certs/ca.crt"
CONTEXT = "exchange-key"
OLD_VERSION = 1
BATCH_SIZE = 1000

def get_current_version():
    """Узнать текущую версию ключа из HSM"""
    r = requests.get(f"{HSM_URL}/health", cert=(CERT, KEY), verify=CA)
    keys = r.json()['kek_status'].keys()
    # Найти максимальную версию для контекста
    versions = [int(k.split('-v')[1]) for k in keys if k.startswith(f'kek-{CONTEXT}')]
    return max(versions)

def reencrypt_batch(records):
    """Перешифровать пакет записей"""
    for record in records:
        # 1. Расшифровать старым ключом
        decrypt_payload = {
            "context": CONTEXT,
            "ciphertext": record['ciphertext'],
            "key_id": f"kek-{CONTEXT}-v{OLD_VERSION}"
        }
        r = requests.post(f"{HSM_URL}/decrypt", 
                         json=decrypt_payload,
                         cert=(CERT, KEY), 
                         verify=CA)
        plaintext = r.json()['plaintext']
        
        # 2. Зашифровать новым ключом (HSM автоматически использует current version)
        encrypt_payload = {
            "context": CONTEXT,
            "plaintext": plaintext
        }
        r = requests.post(f"{HSM_URL}/encrypt",
                         json=encrypt_payload,
                         cert=(CERT, KEY),
                         verify=CA)
        new_ciphertext = r.json()['ciphertext']
        new_key_id = r.json()['key_id']  # kek-exchange-v2
        
        # 3. Обновить БД
        update_record(record['id'], new_ciphertext, new_key_id)

def update_record(record_id, ciphertext, key_id):
    """Обновить запись в БД"""
    conn = psycopg2.connect("dbname=trading user=app")
    cur = conn.cursor()
    cur.execute("""
        UPDATE encrypted_deks 
        SET ciphertext = %s,
            key_id = %s,
            updated_at = %s
        WHERE id = %s
    """, (ciphertext, key_id, datetime.now(), record_id))
    conn.commit()
    conn.close()

def main():
    current_version = get_current_version()
    print(f"Current key version: {current_version}")
    
    if current_version == OLD_VERSION:
        print("No rotation detected, exiting")
        return
    
    # Получить записи для перешифрования
    conn = psycopg2.connect("dbname=trading user=app")
    cur = conn.cursor()
    cur.execute("""
        SELECT id, ciphertext 
        FROM encrypted_deks 
        WHERE key_id LIKE %s
        ORDER BY id
    """, (f'%v{OLD_VERSION}',))
    
    total = 0
    while True:
        batch = cur.fetchmany(BATCH_SIZE)
        if not batch:
            break
        
        reencrypt_batch(batch)
        total += len(batch)
        print(f"Re-encrypted {total} records...")
    
    conn.close()
    print(f"✓ Re-encryption completed: {total} records")

if __name__ == '__main__':
    main()
```

**Стратегия 2: Lazy re-encryption (малые объемы)**

Перешифрование "на лету" при обращении к данным:

```python
def get_decrypted_dek(dek_id):
    """Получить DEK и перешифровать, если устарел"""
    record = db.query("SELECT * FROM encrypted_deks WHERE id = ?", dek_id)
    
    # Расшифровать
    plaintext = hsm_decrypt(record['ciphertext'], record['key_id'])
    
    # Если используется старая версия ключа - перешифровать
    current_version = get_current_key_version()
    if record['key_id'] != f"kek-exchange-v{current_version}":
        # Перешифровать новым ключом
        new_ciphertext, new_key_id = hsm_encrypt(plaintext, "exchange-key")
        db.update("UPDATE encrypted_deks SET ciphertext=?, key_id=? WHERE id=?",
                 new_ciphertext, new_key_id, dek_id)
        print(f"Lazy re-encrypted DEK {dek_id}: {record['key_id']} → {new_key_id}")
    
    return plaintext
```

#### Мониторинг прогресса перешифрования

```sql
-- Статистика по версиям ключей
SELECT 
    key_id,
    COUNT(*) as records,
    MIN(updated_at) as oldest_update,
    MAX(updated_at) as newest_update
FROM encrypted_deks
GROUP BY key_id
ORDER BY key_id;

-- Результат показывает прогресс:
-- key_id            | records | oldest_update       | newest_update
-- kek-exchange-v1   | 1200    | 2025-10-10 10:00:00 | 2026-01-08 15:30:00
-- kek-exchange-v2   | 8800    | 2026-01-09 14:45:00 | 2026-01-09 18:20:00
-- ↑ 88% перешифровано
```

### Шаг 5: Период overlap и очистка старых ключей

**Период overlap (7-30 дней):**
- Оба ключа доступны (старый и новый)
- Новые шифрования используют новый ключ автоматически
- Старые данные расшифровываются старым ключом
- Приложения перешифровывают данные в фоне

**Проверка готовности к удалению:**

```sql
-- Убедиться, что все данные перешифрованы
SELECT COUNT(*) FROM encrypted_deks WHERE key_id = 'kek-exchange-v1';
-- Должно быть 0
```

**Удаление старого ключа:**

```bash
# Docker
docker exec hsm-service /app/hsm-admin cleanup exchange-key --version 1 --confirm

# Production
sudo /usr/local/bin/hsm-admin cleanup exchange-key --version 1 --confirm
```

**Автоматическая очистка:**

Старые версии удаляются автоматически через `cleanup_after_days` из config.yaml:

```yaml
hsm:
  max_versions: 3            # Максимум версий на контекст
  cleanup_after_days: 30     # Удалить версии старше 30 дней
```

Скрипт `/scripts/cleanup-old-keys.sh` запускается по расписанию (cron):

```bash
# /etc/cron.daily/hsm-cleanup
0 3 * * * /opt/hsm-service/scripts/cleanup-old-keys.sh
```

## Автоматическая ротация

### Настройка автоматической ротации (Production)

**1. Установить интервал ротации в metadata.yaml:**

```yaml
rotation:
  exchange-key:
    current: kek-exchange-v1
    rotation_interval_days: 90  # Автоматическая ротация каждые 90 дней
    versions:
      - label: kek-exchange-v1
        version: 1
        created_at: '2026-01-09T00:00:00Z'
```

**2. Создать systemd timer для автоматической проверки:**

```bash
# /etc/systemd/system/hsm-rotation-check.service
[Unit]
Description=HSM Key Rotation Check
After=network.target

[Service]
Type=oneshot
User=hsm
Environment="HSM_PIN_FILE=/etc/hsm/pin.txt"
ExecStart=/opt/hsm-service/scripts/check-key-rotation.sh
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

```bash
# /etc/systemd/system/hsm-rotation-check.timer
[Unit]
Description=HSM Key Rotation Check Timer
Requires=hsm-rotation-check.service

[Timer]
OnCalendar=daily
OnBootSec=5min
Persistent=true

[Install]
WantedBy=timers.target
```

**Активация:**
```bash
sudo systemctl enable hsm-rotation-check.timer
sudo systemctl start hsm-rotation-check.timer
sudo systemctl status hsm-rotation-check.timer
```

**3. Скрипт проверки и автоматической ротации:**

```bash
#!/bin/bash
# /opt/hsm-service/scripts/check-key-rotation.sh

set -euo pipefail

LOG_FILE="/var/log/hsm/rotation.log"
ALERT_EMAIL="titaev@gmail.com"
AUTO_ROTATE=${AUTO_ROTATE:-false}  # false = только алерты, true = автоматическая ротация

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

check_rotation_status() {
    /usr/local/bin/hsm-admin rotation-status | tee /tmp/rotation-status.txt
}

send_alert() {
    local subject=$1
    local body=$2
    
    # Email alert
    echo "$body" | mail -s "$subject" "$ALERT_EMAIL"
    
    # Slack/PagerDuty webhook
    curl -X POST https://hooks.slack.com/services/YOUR/WEBHOOK/URL \
      -H 'Content-Type: application/json' \
      -d "{\"text\":\"$subject\n\n$body\"}"
}

perform_rotation() {
    local context=$1
    
    log "Starting automatic rotation for context: $context"
    
    if /usr/local/bin/hsm-admin rotate "$context"; then
        log "✓ Rotation completed for $context"
        
        # Отправить событие приложениям
        curl -X POST https://trading-app/api/webhooks/key-rotation \
          -H "Content-Type: application/json" \
          -d "{\"context\":\"$context\",\"timestamp\":\"$(date -Iseconds)\"}"
        
        return 0
    else
        log "✗ Rotation failed for $context"
        send_alert "HSM Rotation FAILED: $context" "Automatic rotation failed. Manual intervention required."
        return 1
    fi
}

main() {
    log "Starting key rotation check..."
    
    check_rotation_status
    
    # Найти ключи, требующие ротации
    OVERDUE_KEYS=$(grep "NEEDS ROTATION" /tmp/rotation-status.txt | awk '{print $3}' | tr -d ':' || true)
    
    if [[ -z "$OVERDUE_KEYS" ]]; then
        log "All keys are up to date"
        exit 0
    fi
    
    log "Keys requiring rotation: $OVERDUE_KEYS"
    
    if [[ "$AUTO_ROTATE" == "true" ]]; then
        # Автоматическая ротация
        for context in $OVERDUE_KEYS; do
            perform_rotation "$context"
        done
    else
        # Только алерты
        send_alert "HSM Keys Need Rotation" "$(cat /tmp/rotation-status.txt)"
        log "Alerts sent. Manual rotation required (AUTO_ROTATE=false)"
    fi
}

main
```

**Включение автоматической ротации:**

```bash
# Добавить в /etc/systemd/system/hsm-rotation-check.service
Environment="AUTO_ROTATE=true"

sudo systemctl daemon-reload
sudo systemctl restart hsm-rotation-check.service
```

### Docker автоматическая ротация

**docker-compose.yml с автоматической ротацией:**

```yaml
services:
  hsm-service:
    image: hsm-service:latest
    # ... остальная конфигурация ...
  
  hsm-rotation-scheduler:
    image: hsm-service:latest
    entrypoint: /bin/sh
    command: >
      -c "
      while true; do
        sleep 86400;  # Проверка раз в день
        /app/scripts/check-key-rotation.sh;
      done
      "
    environment:
      - AUTO_ROTATE=true
      - HSM_PIN=${HSM_PIN}
    volumes:
      - ./metadata.yaml:/app/metadata.yaml
      - ./scripts:/app/scripts
    depends_on:
      - hsm-service
```

## Экстренная ротация при компрометации

**ВАЖНО:** При компрометации ключа порядок действий критичен.

### Шаг 1: Немедленная ротация

```bash
# Создать новый ключ
sudo /usr/local/bin/hsm-admin rotate exchange-key

# Проверить, что новый ключ загружен (подождать 30 сек или перезапустить)
sudo systemctl restart hsm-service  # Для немедленного применения
```

### Шаг 2: Блокировка скомпрометированного ключа

**НЕЛЬЗЯ удалять старый ключ сразу!** Нужен для расшифровки существующих данных.

Временно заблокировать доступ к старому ключу через ACL или отключить context:

```yaml
# Временно закомментировать context в config.yaml (если это приемлемо)
# hsm:
#   keys:
#     exchange-key-OLD:  # Временно отключено
#       type: aes
```

### Шаг 3: Срочное перешифрование

Запустить перешифрование в **приоритетном режиме**:

```bash
# Запуск с максимальной параллельностью
./reencrypt.py --context exchange-key --parallel 10 --priority high
```

**Мониторинг прогресса:**

```sql
-- Проверка каждые 5 минут
SELECT 
    key_id,
    COUNT(*) as remaining,
    COUNT(*) * 100.0 / (SELECT COUNT(*) FROM encrypted_deks) as percent_remaining
FROM encrypted_deks
WHERE key_id = 'kek-exchange-v1'  -- Скомпрометированная версия
GROUP BY key_id;
```

### Шаг 4: Удаление скомпрометированного ключа

**Только после 100% перешифрования:**

```bash
# Финальная проверка
if [[ $(psql -t -c "SELECT COUNT(*) FROM encrypted_deks WHERE key_id='kek-exchange-v1'") -eq 0 ]]; then
    echo "✓ All data re-encrypted, safe to delete old key"
    sudo /usr/local/bin/hsm-admin cleanup exchange-key --version 1 --confirm
else
    echo "✗ WARNING: Data still encrypted with old key! Cannot delete."
    exit 1
fi
```

### Шаг 5: Аудит и уведомления

```bash
# 1. Просмотр логов доступа к скомпрометированному ключу
journalctl -u hsm-service --since "7 days ago" | \
  grep "kek-exchange-v1" | \
  grep -E "decrypt|encrypt" > /tmp/compromised-key-audit.log

# 2. Уведомление security team
mail -s "URGENT: KEK Compromised - exchange-key-v1" security@company.com < /tmp/incident-report.txt

# 3. Обновление incident response документации
```

**Временная шкала экстренной ротации:**
- T+0 мин: Обнаружение компрометации
- T+5 мин: Новый ключ создан и загружен
- T+10 мин: Перешифрование запущено (параллельно)
- T+30 мин - 4 часа: Перешифрование завершено (зависит от объема)
- T+4 часа: Старый ключ удален
- T+24 часа: Аудит завершен

## Соглашение об именовании ключей

Формат: `kek-<context>-v<version>`

**Примеры:**
- kek-exchange-v1, kek-exchange-v2, kek-exchange-v3
- kek-2fa-v1, kek-2fa-v2
- kek-payment-v1

## Соответствие PCI DSS

Эта процедура ротации удовлетворяет требованию PCI DSS Requirement 3.6.4:

- ✅ **3.6.4.a**: Ключи ротируются через определенные интервалы (90 дней)
- ✅ **3.6.4.b**: Ключи ротируются при компрометации (экстренная ротация)
- ✅ **3.6.4.c**: Старые ключи выводятся из эксплуатации после overlap периода
- ✅ **3.6.4.d**: Новые ключи заменяют старые для всех новых шифрований

## Устранение неполадок

### Проблема: Hot reload не происходит

**Причина:** metadata.yaml не изменился или сервис не мониторит файл

**Решение:**
```bash
# Проверить, что файл изменился
stat /app/metadata.yaml  # Docker
stat /opt/hsm-service/metadata.yaml  # Production

# Проверить логи мониторинга
journalctl -u hsm-service | grep "metadata" | tail -20

# Принудительная перезагрузка
sudo systemctl restart hsm-service
```

### Проблема: Новый ключ не появляется в /health

**Причина:** metadata.yaml поврежден или содержит некорректный YAML

**Решение:**
```bash
# Валидация YAML
python3 -c 'import yaml; yaml.safe_load(open("/app/metadata.yaml"))'

# Проверить права доступа
ls -la /app/metadata.yaml
# Должно быть: -rw-r--r-- 1 hsm hsm

# Восстановить из backup
cp metadata.yaml.backup-TIMESTAMP metadata.yaml
```

### Проблема: Старые данные не расшифровываются после удаления ключа

**Причина:** Ключ удален до завершения перешифрования

**Решение:**
1. **НЕ ПАНИКОВАТЬ** - у вас должна быть резервная копия токена
2. Восстановить старый ключ из backup токена:

```bash
# Остановить сервис
sudo systemctl stop hsm-service

# Восстановить токен из backup
sudo cp -r /backup/hsm/tokens/* /var/lib/softhsm/tokens/

# Добавить старую версию обратно в metadata.yaml
# (из metadata.yaml.backup)

# Запустить сервис
sudo systemctl start hsm-service

# Завершить перешифрование
./reencrypt.py --context exchange-key

# Только после 100% - удалить старый ключ
```

### Проблема: Ротация завершается ошибкой "HSM_PIN not set"

**Причина:** Переменная окружения HSM_PIN не установлена

**Решение:**
```bash
# Docker
docker exec -e HSM_PIN=1234 hsm-service /app/hsm-admin rotate exchange-key

# Production (systemd)
sudo -E /usr/local/bin/hsm-admin rotate exchange-key
# Убедитесь, что HSM_PIN_FILE=/etc/hsm/pin.txt в systemd unit

# Или экспортируйте переменную
export HSM_PIN=$(cat /etc/hsm/pin.txt)
sudo -E /usr/local/bin/hsm-admin rotate exchange-key
```

### Проблема: Приложение не перешифровывает данные

**Причина:** Приложение не получило событие о ротации

**Решение:**
```bash
# 1. Проверить, что webhook работает
curl -X POST https://trading-app/api/webhooks/key-rotation \
  -H "Content-Type: application/json" \
  -d '{"context":"exchange-key","new_version":2,"test":true}'

# 2. Проверить логи приложения
kubectl logs -f deployment/trading-app | grep "key_rotation"

# 3. Запустить перешифрование вручную
kubectl exec -it deployment/trading-app -- /app/scripts/reencrypt.py --context exchange-key
```

## Лучшие практики

### Планирование

1. **Ротируйте в окна обслуживания** (low-traffic periods)
2. **Тестируйте в staging** перед production
3. **Уведомляйте команды** за 24 часа до ротации
4. **Мониторьте приложения** во время overlap периода

### Резервное копирование

1. **Backup токена HSM** перед каждой ротацией:
```bash
sudo /opt/hsm-service/scripts/backup-hsm-token.sh
```

2. **Backup metadata.yaml** (автоматически создается hsm-admin)
3. **Версионирование в Git** (metadata.yaml.backup-*)
4. **Offsite backup** токенов (encrypted, раз в неделю)

### Мониторинг

1. **Dashboards:**
   - Prometheus метрики: `hsm_key_version`, `hsm_key_rotation_due_days`
   - Grafana алерты при `rotation_due_days < 7`

2. **Логирование:**
   - Централизованные логи (ELK/Loki)
   - Аудит всех операций ротации
   - Retention: минимум 1 год (compliance)

3. **Alerts:**
   - PagerDuty/Opsgenie для критичных событий
   - Email для плановых уведомлений
   - Slack для успешных ротаций

### Документирование

1. **Changelog** всех ротаций:
```markdown
## 2026-01-09: exchange-key rotation
- Old: kek-exchange-v1 (created 2025-10-10)
- New: kek-exchange-v2 (created 2026-01-09)
- Reason: Scheduled 90-day rotation
- Completed by: ops-team
- Re-encryption: 10,500 records (4 hours)
```

2. **Runbook** для on-call инженеров
3. **Incident reports** для экстренных ротаций

### Автоматизация

1. **CI/CD интеграция:**
   - Automated testing после ротации
   - Rollback процедуры
   - Canary deployments

2. **Monitoring integration:**
   - Auto-alerts при приближении rotation deadline
   - Dashboard с countdown до следующей ротации

3. **GitOps workflow:**
   - metadata.yaml в отдельном Git repo (encrypted)
   - Automated PR для ротаций
   - Approval workflow

## См. также

- [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - Отчет по аудиту безопасности
- [API.md](API.md) - API документация
- PCI DSS v4.0 Requirement 3.6 - Управление криптографическими ключами
