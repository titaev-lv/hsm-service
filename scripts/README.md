# HSM Key Rotation Automation Scripts

Набор скриптов для автоматизации ротации ключей в production окружении.

## Скрипты

### 1. check-key-rotation.sh

**Назначение:** Ежедневная проверка статуса ротации ключей с оповещениями (+ опциональная автоматическая ротация)

**Функции:**
- Проверяет ключи на необходимость ротации
- Отправляет предупреждения за 14 дней до истечения
- Критические оповещения за 7 дней до истечения
- **Может автоматически запустить ротацию (если AUTO_ROTATION_ENABLED=true)**
- Поддержка Email, Slack, Telegram
- Проверка здоровья HSM сервиса
- Логирование в syslog

**Использование:**

```bash
# Ручной запуск
sudo ./scripts/check-key-rotation.sh

# С настройкой Email оповещений
export HSM_SEND_EMAIL=true
export HSM_ALERT_EMAIL="ops@company.com"
sudo ./scripts/check-key-rotation.sh

# С Slack оповещениями
export HSM_SLACK_WEBHOOK="https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
sudo ./scripts/check-key-rotation.sh

# С Telegram оповещениями
export HSM_TELEGRAM_BOT_TOKEN="your_bot_token"
export HSM_TELEGRAM_CHAT_ID="your_chat_id"
sudo ./scripts/check-key-rotation.sh
```

**Настройка cron:**

```bash
# Ежедневная проверка в 9:00
0 9 * * * /path/to/hsm-service/scripts/check-key-rotation.sh

# С переменными окружения
0 9 * * * HSM_SLACK_WEBHOOK="https://..." /path/to/hsm-service/scripts/check-key-rotation.sh
```

**Пример crontab с полной настройкой:**

```bash
# HSM Key Rotation Monitoring
# Проверка ключей каждый день в 9:00
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin
HSM_SEND_EMAIL=true
HSM_ALERT_EMAIL=ops@company.com
HSM_SLACK_WEBHOOK=https://hooks.slack.com/services/YOUR/WEBHOOK

0 9 * * * cd /home/user/hsm-service && ./scripts/check-key-rotation.sh >> /var/log/hsm-cron.log 2>&1
```

### 2. rotate-key-interactive.sh

**Назначение:** Интерактивная ротация ключа с подтверждением на каждом шаге

**Функции:**
- Пошаговая ротация с подтверждением пользователя
- Автоматическое резервное копирование (config + HSM token)
- Создание нового ключа
- Обновление конфигурации для overlap period
- Проверка здоровья сервиса
- Откат при ошибках

**Использование:**

```bash
# Установка HSM PIN
export HSM_PIN=1234

# Запуск интерактивной ротации
sudo -E ./scripts/rotate-key-interactive.sh
```

**Пример сессии:**

```
==========================================
   HSM Key Rotation - Interactive Mode
==========================================

ℹ Step 1: Checking current rotation status...
Key Rotation Status:
====================
⚠️  Context: exchange-key
  Label:             kek-exchange-v1
  Status:            NEEDS ROTATION (2 days overdue)

❓ Continue with rotation? (yes/no): yes

ℹ Step 2: Select key to rotate
Enter the key label to rotate (e.g., kek-exchange-v1):
kek-exchange-v1

ℹ Old key: kek-exchange-v1 (version 1)
ℹ New key: kek-exchange-v2 (version 2)

❓ Proceed with rotation from v1 to v2? (yes/no): yes

ℹ Step 3: Creating backups...
✓ Config backed up to: /var/backups/hsm/config.yaml.20260109-120000
✓ HSM token backed up to: /var/backups/hsm/token.20260109-120000.tar.gz

ℹ Step 4: Creating new KEK...
✓ Created KEK: kek-exchange-v2 (handle: 3, ID: 02, version: 2)

ℹ Step 5: Updating configuration for overlap period...
✓ Config updated: exchange-key -> kek-exchange-v2 (v2)
✓ Old key preserved: exchange-key-old -> kek-exchange-v1

ℹ Step 6: Restarting HSM service...
✓ Service restarted successfully and is healthy

==========================================
✓ KEY ROTATION COMPLETED
==========================================

OVERLAP PERIOD (7 days) - Both keys are active:
  • New encryptions will use: kek-exchange-v2
  • Old data can be decrypted with: kek-exchange-v1

NEXT STEPS:
1. ✓ New key created and loaded
2. ⚠  Re-encrypt all data encrypted with kek-exchange-v1
3. ⏰ Wait 7 days for overlap period
4. ⚠  Delete old key after verification
```
rotate-key-auto.sh

**Назначение:** Полностью автоматическая ротация ключей без взаимодействия с пользователем

**Функции:**
- Полный цикл ротации без подтверждений пользователя
- Автоматическое резервное копирование
- Создание нового ключа
- Обновление конфигурации
- Перезапуск сервиса
- Проверка здоровья с автоматическим откатом при ошибках
- Отправка оповещений о начале и завершении
- Автоматическая очистка старых ключей (если AUTO_CLEANUP_ENABLED=true)

**Использование:**

```bash
# ВАЖНО: Установка флагов безопасности
export AUTO_ROTATION_ENABLED=true  # Обязательно!
export AUTO_CLEANUP_ENABLED=true   # Опционально (автоудаление старых ключей)
export HSM_PIN=1234

# Запуск автоматической ротации
sudo -E ./scripts/rotate-key-auto.sh
```

**Настройки безопасности:**

```bash
# Максимальный возраст ключа для автоматической ротации (защита от очень старых ключей)
export MAX_ROTATION_AGE_DAYS=100  # По умолчанию 100 дней (макс 10 дней просрочки)

# Период overlap для старых ключей
export OVERLAP_PERIOD_DAYS=7  # По умолчанию 7 дней
```

**Пример вывода:**

```
==========================================
HSM Automatic Key Rotation - Starting
==========================================

✓ Prerequisites check passed
ℹ Keys needing rotation:
kek-exchange-v1

ℹ Starting automatic rotation for key: kek-exchange-v1
ℹ Rotation plan: kek-exchange-v1 (v1) → kek-exchange-v2 (v2)
ℹ Key age: 92 days

🔄 [Slack/Email/Telegram] Starting automatic rotation: kek-exchange-v1 → kek-exchange-v2

ℹ Creating backups...
✓ Backups created: config=/var/backups/hsm/config.yaml.20260109-150000, token=...

ℹ Creating new KEK: kek-exchange-v2
✓ New KEK created: kek-exchange-v2

ℹ Updating configuration for overlap period...
✓ Configuration updated for overlap period

ℹ Restarting HSM service...
✓ Service is healthy
✓ Both keys verified in HSM

✅ [Slack/Email/Telegram] AUTO-ROTATION COMPLETED
    Key rotated: kek-exchange-v1 → kek-exchange-v2
    ...

✓ Rotation completed: kek-exchange-v1 → kek-exchange-v2

==========================================
Auto-rotation summary:
  Successful: 1
  Failed: 0
==========================================
```

### 4. cleanup-old-keys.sh

**Назначение:** Удаление старых ключей после завершения overlap period (интерактивный режим)
**Назначение:** Удаление старых ключей после завершения overlap period

**Функции:**
- Находит старые ключи в конфигурации
- Проверяет возраст ключа (минимум 7 дней)
- Удаляет ключ из HSM
- Удаляет из config.yaml
- Перезапускает сервис

**Использование:**

```bash
# Запуск очистки
sudo ./scripts/cleanup-old-keys.sh
```

**Пример сессии:**

```
==========================================
   HSM Old Key Cleanup
==========================================

Found old key contexts:
exchange-key-old

Checking key: kek-exchange-v1
  Context: exchange-key-old
  Created: 2025-10-09T10:30:00Z
  Age: 92 days
Режим 1: Полностью автоматическая ротация (рекомендуется)

```bash
# /etc/cron.d/hsm-rotation
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin
HSM_SLACK_WEBHOOK=https://hooks.slack.com/services/XXX
HSM_SEND_EMAIL=true
HSM_ALERT_EMAIL=ops@company.com

# АВТОМАТИЧЕСКАЯ РОТАЦИЯ: проверка + автоматическое выполнение ротации
AUTO_ROTATION_ENABLED=true
AUTO_CLEANUP_ENABLED=true
HSM_PIN=********  # Используйте secrets manager!

# Проверка ротации каждый день в 2:00 AM (с автоматической ротацией при необходимости)
0 2 * * * root cd /opt/hsm-service && ./scripts/check-key-rotation.sh >> /var/log/hsm-rotation-check.log 2>&1

# Еженедельная проверка здоровья
0 10 * * 1 root docker exec hsm-service /app/hsm-admin rotation-status >> /var/log/hsm-weekly-status.log 2>&1
```

**Как это работает:**
1. Каждую ночь в 2:00 AM запускается `check-key-rotation.sh`
2. Если ключи просрочены и `AUTO_ROTATION_ENABLED=true`, автоматически запускается `rotate-key-auto.sh`
3. Ротация выполняется полностью автоматически:
   - Создаются бэкапы
   - Генерируется новый ключ
   - Обновляется конфигурация
   - Перезапускается сервис
   - Проверяется здоровье
4. После 7 дней (OVERLAP_PERIOD_DAYS), если `AUTO_CLEANUP_ENABLED=true`, старые ключи автоматически удаляются
5. Вы получаете оповещения в Slack/Email/Telegram о начале и завершении ротации

**Преимущества:**
- ✅ Нет необходимости в ручном вмешательстве
- ✅ Ротация происходит в нерабочее время (2:00 AM)
- ✅ Автоматический откат при ошибках
- ✅ Полное логирование и мониторинг
- ✅ Соответствие PCI DSS без человеческого фактора

**Требования:**
- HSM_PIN должен храниться в secrets manager (не в crontab!)
- Настроенные оповещения (Slack/Email/Telegram)
- Тестирование на dev/staging окружении

### Режим 2: Только мониторинг + ручная ротация

```bash
# /etc/cron.d/hsm-rotation
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin
HSM_SLACK_WEBHOOK=https://hooks.slack.com/services/XXX
HSM_SEND_EMAIL=true
HSM_ALERT_EMAIL=ops@company.com

# AUTO_ROTATION_ENABLED не установлен (или =false) - только мониторинг

# Проверка ротации каждый день в 9:00 (только оповещения)
==========================================
Cleanup completed
==========================================
```

## Установка

### 1. Сделать скрипты исполняемыми

```bash
cd /path/to/hsm-service
chmod +x scripts/*.sh
```

### 2. Создать директории для логов и бэкапов

```bash
sudo mkdir -p /var/log
sudo mkdir -p /var/backups/hsm
sudo chown -R $USER:$USER /var/backups/hsm
```

### 3. Настроить cron для автоматической проверки

```bash
# Редактировать crontab
sudo crontab -e

# Добавить задачу
0 9 * * * cd /home/user/hsm-service && ./scripts/check-key-rotation.sh >> /var/log/hsm-cron.log 2>&1
```

### 4. Настроить оповещения (опционально)

#### Email (требует postfix/sendmail)

```bash
# Установка mailutilв ручном режиме (когда получено оповещение)

1. **День 0 - Получение оповещения:**
   ```
   ⚠️ WARNING: HSM key expiring soon!
   Context: exchange-key
   Days remaining: 7
   ```

2. **День 1 - Планирование:**
   - Согласовать окно обслуживания с командой
   - Уведомить пользователей о maintenance window
   - Подготовить rollback plan

3. **День 2 - Выполнение ротации:**
   
   **Вариант A: Интерактивная ротация (рекомендуется для первого раза)**
   ```bash
   export HSM_PIN=your_pin
   sudo -E ./scripts/rotate-key-interactive.sh
   ```
   
   **Вариант B: Полностью автоматическая (для опытных администраторов)**
   ```bash
   export AUTO_ROTATION_ENABLED=true
   export HSM_PIN=your_pin
   sudo -E ./scripts/rotate-key-auto.sh
   ```

4. **День 2-9 - Overlap period:**
   - Мониторинг сервиса
   - Перешифрование данных приложениями
   - Проверка логов на ошибки

5. **День 9 - Очистка:**
   
   **Вариант A: Ручная очистка**
   ```bash
   sudo ./scripts/cleanup-old-keys.sh
   ```
   
   **Вариант B: Автоматическая очистка**
   ```bash
   # Если при ротации был установлен AUTO_CLEANUP_ENABLED=true,
   # очистка произойдет автоматически через 7 днейion

### Ежедневная автоматизация

```bash
# /etc/cron.d/hsm-rotation
SHELL=/bin/bash
PATH=/usr/local/sbin:/usr/local/bin:/sbin:/bin:/usr/sbin:/usr/bin
HSM_SLACK_WEBHOOK=https://hooks.slack.com/services/XXX
HSM_SEND_EMAIL=true
HSM_ALERT_EMAIL=ops@company.com

# Проверка ротации каждый день в 9:00
0 9 * * * root cd /opt/hsm-service && ./scripts/check-key-rotation.sh >> /var/log/hsm-rotation-check.log 2>&1

# Еженедельная проверка здоровья
0 10 * * 1 root docker exec hsm-service /app/hsm-admin rotation-status >> /var/log/hsm-weekly-status.log 2>&1
```

### Процесс ротации (когда получено оповещение)

1. **День 0 - Получение оповещения:**
   ```
   ⚠️ WARNING: HSM key expiring soon!
   Context: exchange-key
   Days remaining: 7
   ```

2. **День 1 - Планирование:**
   - Согласовать окно обслуживания с командой
   - Уведомить пользователей о maintenance window
   - Подготовить rollback plan

3. **День 2 - Выполнение ротации:**
   ```bash (КРИТИЧЕСКИ ВАЖНО!)

```bash
# НЕ записывайте PIN в crontab напрямую!
# Используйте secrets manager или systemd credentials

# Вариант 1: HashiCorp Vault
HSM_PIN=$(vault kv get -field=pin secret/hsm/pin)
export HSM_PIN

# Вариант 2: AWS Secrets Manager
HSM_PIN=$(aws secretsmanager get-secret-value --secret-id hsm-pin --query SecretString --output text)
export HSM_PIN

# Вариант 3: systemd credentials (рекомендуется для auto-rotation)
# См. секцию "Настройка systemd timer для автоматической ротации" ниже
```

### Использование systemd вместо cron (рекомендуется для auto-rotation)

Systemd позволяет безопасно хранить HSM_PIN и управлять автоматической ротацией.

**Создание systemd service:**

```bash
# /etc/systemd/system/hsm-rotation-check.service
[Unit]
Description=HSM Key Rotation Check
After=docker.service
Requires=docker.service

[Service]
Type=oneshot
User=root
WorkingDirectory=/opt/hsm-service
Environment="AUTO_ROTATION_ENABLED=true"
Environment="AUTO_CLEANUP_ENABLED=true"
Environment="HSM_SLACK_WEBHOOK=https://hooks.slack.com/services/XXX"
Environment="HSM_SEND_EMAIL=true"
Environment="HSM_ALERT_EMAIL=ops@company.com"
# HSM_PIN хранится в credentials (см. ниже)
ExecStart=/opt/hsm-service/scripts/check-key-rotation.sh
StandardOutput=append:/var/log/hsm-rotation-check.log
StandardError=append:/var/log/hsm-rotation-check.log

# Безопасное хранение HSM_PIN
LoadCredential=hsm_pin:/etc/hsm/credentials/pin
# В скрипте используйте: HSM_PIN=$(cat $CREDENTIALS_DIRECTORY/hsm_pin)
```

**Создание systemd timer:**

```bash
# /etc/systemd/system/hsm-rotation-check.timer
[Unit]
Description=HSM Key Rotation Check Timer
Requires=hsm-rotation-check.service

[Timer]
# Запуск каждый день в 2:00 AM
OnCalendar=*-*-* 02:00:00
Persistent=true
RandomizedDelaySec=10min

[Install]
WantedBy=timers.target
```

**Настройка credentials (безопасное хранение PIN):**

```bash
# Создание директории для credentials
sudo mkdir -p /etc/hsm/credentials
sudo chmod 700 /etc/hsm/credentials

# Сохранение PIN (зашифрованно)
echo -n "1234" | sudo tee /etc/hsm/credentials/pin > /dev/null
sudo chmod 400 /etc/hsm/credentials/pin
sudo chown root:root /etc/hsm/credentials/pin

# Опционально: использование systemd-creds для шифрования
echo -n "1234" | systemd-creds encrypt --name=hsm-pin - /etc/hsm/credentials/pin.enc
```

**Активация timer:**

```bash
# Перезагрузка systemd
sudo systemctl daemon-reload

# Включение и запуск timer
sudo systemctl enable hsm-rotation-check.timer
sudo systemctl start hsm-rotation-check.timer

# Проверка статуса
sudo systemctl status hsm-rotation-check.timer
sudo systemctl list-timers | grep hsm

# Просмотр логов
sudo journalctl -u hsm-rotation-check.service -f

# Ручной запуск для теста
sudo systemctl start hsm-rotation-check.service
```

**Преимущества systemd над cron:**
- ✅ Безопасное хранение HSM_PIN через credentials
- ✅ Автоматический перезапуск при ошибках
- ✅ Dependency management (запуск после Docker)
- ✅ Централизованное логирование через journalctl
- ✅ RandomizedDelaySec предотвращает одновременный запуск на нескольких серверах
- ✅ Persistent=true запускает пропущенные задачи после перезагрузки# После проверки, что все данные перешифрованы
   sudo ./scripts/cleanup-old-keys.sh
   ```

## Мониторинг и логи

### Просмотр логов

```bash
# Лог проверки ротации
tail -f /var/log/hsm-rotation-check.log

# Лог ротации
tail -f /var/log/hsm-rotation.log

# Cron лог
tail -f /var/log/hsm-cron.log

# Syslog
sudo journalctl -t hsm-rotation -f
```

### Метрики для мониторинга

Рекомендуется добавить в Prometheus/Grafana:

```yaml
# prometheus-alerts.yml
groups:
  - name: hsm_rotation
    rules:
      - alert: HSMKeyRotationOverdue
        expr: hsm_key_days_until_rotation < 0
        for: 1h
        labels:
          severity: critical
        annotations:
          summary: "HSM key rotation overdue"
          
      - alert: HSMKeyRotationWarning
        expr: hsm_key_days_until_rotation < 7
        for: 1h
        labels:
          severity: warning
        annotations:
          summary: "HSM key rotation needed soon"
```

## Безопасность

### Защита HSM_PIN

```bash
# НЕ записывайте PIN в crontab!
# Используйте secrets manager

# Пример с HashiCorp Vault
HSM_PIN=$(vault kv get -field=pin secret/hsm/pin)
export HSM_PIN

# Пример с AWS Secrets Manager
HSM_PIN=$(aws secretsmanager get-secret-value --secret-id hsm-pin --query SecretString --output text)
export HSM_PIN
```

### Права доступа

```bash
# Ограничить доступ к скриптам
chmod 750 scripts/*.sh
chown root:root scripts/*.sh

# Защитить логи
chmod 640 /var/log/hsm-*.log
chown root:adm /var/log/hsm-*.log

# Защитить бэкапы
chmod 700 /var/backups/hsm
chown root:root /var/backups/hsm
```

## Troubleshooting

### Проблема: Скрипт не может подключиться к Docker

```bash
# Проверка Docker
systemctl status docker

# Добавить пользователя в группу docker
sudo usermod -aG docker $USER

# Или запустить с sudo
sudo ./scripts/check-key-rotation.sh
```

### Проблема: Оповещения не приходят

```bash
# Проверка Email
echo "Test" | mail -s "Test" ops@company.com

# Проверка Slack webhook
curl -X POST "$HSM_SLACK_WEBHOOK" -d '{"text":"Test"}'

# Проверка Telegram
curl -X POST "https://api.telegram.org/bot$HSM_TELEGRAM_BOT_TOKEN/sendMessage" \
  -d "chat_id=$HSM_TELEGRAM_CHAT_ID" -d "text=Test"
```

### Проблема: Python не найден

```bash
# Установка Python
sudo apt-get install python3 python3-pip

# Установка PyYAML
pip3 install pyyaml
```

## См. также

- [KEY_ROTATION.md](../KEY_ROTATION.md) - Детальная документация по ротации
- [SECURITY_AUDIT.md](../SECURITY_AUDIT.md) - Аудит безопасности
- [DEVELOPMENT_PLAN.md](../DEVELOPMENT_PLAN.md) - План разработки
