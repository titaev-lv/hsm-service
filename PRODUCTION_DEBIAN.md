# 🏭 HSM Service - Production Deployment (Debian 13)

> **Для DevOps**: Развертывание HSM Service на Debian 13 Trixie с nftables firewall

## Оглавление

- [Системные требования](#системные-требования)
- [Подготовка сервера](#подготовка-сервера)
- [Установка зависимостей](#установка-зависимостей)
- [Установка SoftHSM](#установка-softhsm)
- [Развертывание бинарников](#развертывание-бинарников)
- [Настройка PKI](#настройка-pki)
- [Конфигурация сервиса](#конфигурация-сервиса)
- [Systemd service setup](#systemd-service-setup)
- [Настройка nftables firewall](#настройка-nftables-firewall)
- [Мониторинг и логирование](#мониторинг-и-логирование)
- [Бэкапы](#бэкапы)
- [Безопасность](#безопасность)
- [Troubleshooting](#troubleshooting)

---

## Системные требования

### Минимальные

- **OS**: Debian 13 (Trixie) или Debian 12 (Bookworm)
- **CPU**: 2 cores
- **RAM**: 2 GB
- **Disk**: 20 GB
- **Network**: Статический IP

### Рекомендуемые

- **OS**: Debian 13 (Trixie)
- **CPU**: 4 cores
- **RAM**: 4 GB
- **Disk**: 50 GB (SSD)
- **Network**: Dedicated network interface

---

## Подготовка сервера

### 1. Обновление системы

```bash
# Update package lists
apt update

# Upgrade all packages
apt upgrade -y

# Install basic tools
apt install -y curl wget git vim sudo
```

### 2. Создание пользователя для сервиса

```bash
# Create system user
useradd -r -m -s /bin/bash -d /opt/hsm-service hsm

# Add to sudo group (опционально, для setup)
usermod -aG sudo hsm

# Set password
passwd hsm
```

### 3. Настройка hostname и timezone

```bash
# Set hostname
hostnamectl set-hostname hsm-service.example.com

# Set timezone
timedatectl set-timezone Europe/Moscow

# Verify
hostnamectl
timedatectl
```

---

## Установка зависимостей

### 1. Установка SoftHSM2

```bash
# Install SoftHSM2 and PKCS#11 tools
apt install -y softhsm2 opensc openssl

# Verify installation
softhsm2-util --version
# SoftHSM 2.6.1

pkcs11-tool --version
# pkcs11-tool 0.23.0
```

### 2. Установка дополнительных утилит

```bash
# Prometheus node_exporter (опционально)
apt install -y prometheus-node-exporter

# Logrotate
apt install -y logrotate

# Monitoring tools
apt install -y htop iotop nethogs

# Security tools
apt install -y fail2ban
```

---

## Развертывание бинарников

> **⚠️ ВАЖНО**: Предполагается что бинарники уже собраны на build-сервере. На production сервере НЕ устанавливаем Go и НЕ компилируем код.

### 1. Создание директорий

```bash
# Create directories
sudo mkdir -p /opt/hsm-service/bin
sudo mkdir -p /var/lib/softhsm/tokens
sudo mkdir -p /var/log/hsm-service
sudo mkdir -p /etc/hsm-service

# Set ownership
sudo chown -R hsm:hsm /opt/hsm-service
sudo chown -R hsm:hsm /var/lib/softhsm/tokens
sudo chown -R hsm:hsm /var/log/hsm-service
sudo chown -R hsm:hsm /etc/hsm-service

# Set permissions
sudo chmod 755 /opt/hsm-service
sudo chmod 700 /var/lib/softhsm/tokens
sudo chmod 755 /var/log/hsm-service
sudo chmod 755 /etc/hsm-service
```

### 2. Копирование бинарников

```bash
# Скопировать с build-сервера (с вашего CI/CD или локально)
scp hsm-service hsm@production-server:/opt/hsm-service/bin/
scp hsm-admin hsm@production-server:/opt/hsm-service/bin/

# Установить права выполнения
ssh hsm@production-server "chmod +x /opt/hsm-service/bin/hsm-service /opt/hsm-service/bin/hsm-admin"

# Проверка
ssh hsm@production-server "/opt/hsm-service/bin/hsm-service --version"
```

---

## Настройка PKI

> **📖 Детальная инструкция**: См. [PKI_SETUP.md](PKI_SETUP.md) для создания CA и генерации сертификатов

### Копирование существующих сертификатов

```bash
# Создать директории
sudo mkdir -p /etc/hsm-service/pki/{ca,server,client}

# Скопировать сертификаты с CA-сервера или локально
sudo cp /path/to/ca.crt /etc/hsm-service/pki/ca/
sudo cp /path/to/hsm-service.crt /etc/hsm-service/pki/server/
sudo cp /path/to/hsm-service.key /etc/hsm-service/pki/server/
sudo cp /path/to/client*.crt /etc/hsm-service/pki/client/  # для тестирования

# Set ownership
sudo chown -R hsm:hsm /etc/hsm-service/pki

# Set permissions (КРИТИЧЕСКИ ВАЖНО!)
sudo chmod 600 /etc/hsm-service/pki/server/*.key
sudo chmod 600 /etc/hsm-service/pki/client/*.key
sudo chmod 644 /etc/hsm-service/pki/ca/*.crt
sudo chmod 644 /etc/hsm-service/pki/server/*.crt
sudo chmod 644 /etc/hsm-service/pki/client/*.crt
```

**Проверка**:
```bash
# Проверить серверный сертификат
openssl verify -CAfile /etc/hsm-service/pki/ca/ca.crt /etc/hsm-service/pki/server/hsm-service.crt
# /etc/hsm-service/pki/server/hsm-service.crt: OK

# Проверить клиентский сертификат
openssl verify -CAfile /etc/hsm-service/pki/ca/ca.crt /etc/hsm-service/pki/client/trading-service-1.crt
# /etc/hsm-service/pki/client/trading-service-1.crt: OK
```

```

---

## Конфигурация сервиса

### 1. Конфигурация SoftHSM

```bash
# Edit SoftHSM config
sudo nano /etc/softhsm/softhsm2.conf
```

**Содержимое `/etc/softhsm/softhsm2.conf`**:
```ini
# SoftHSM v2 configuration file

directories.tokendir = /var/lib/softhsm/tokens/
objectstore.backend = file

# Logging
log.level = INFO
```

### 2. Инициализация HSM токена

```bash
# Initialize token
softhsm2-util --init-token \
  --slot 0 \
  --label "hsm-token" \
  --so-pin 5678 \
  --pin 1234

# ВАЖНО: Используйте сильные PIN'ы на production!
# Запишите PIN в безопасное место (KMS, Vault)

# Verify
softhsm2-util --show-slots
```

### 3. Конфигурация HSM Service

```bash
# Copy config template
sudo cp /opt/hsm-service/config.yaml.example /etc/hsm-service/config.yaml

# Edit configuration
sudo nano /etc/hsm-service/config.yaml
```

**Production config.yaml**:
```yaml
server:
  port: "8443"
  tls:
    ca_path: /etc/hsm-service/pki/ca/ca.crt
    cert_path: /etc/hsm-service/pki/server/hsm-service.crt
    key_path: /etc/hsm-service/pki/server/hsm-service.key

hsm:
  pkcs11_lib: /usr/lib/softhsm/libsofthsm2.so
  slot_id: hsm-token
  metadata_file: /var/lib/hsm-service/metadata.yaml
  max_versions: 3
  cleanup_after_days: 30
  keys:
    exchange-key:
      type: aes
    2fa:
      type: aes

acl:
  revoked_file: /etc/hsm-service/pki/revoked.yaml
  mappings:
    Trading:
      - exchange-key
    2FA:
      - 2fa

rate_limit:
  requests_per_second: 50000  # Per-client limit (by mTLS CN)
  burst: 5000                  # Burst capacity

logging:
  level: info
  format: json

# HTTP/2 optimization for high-load scenarios
server:
  http2:
    max_concurrent_streams: "2000"       # Default: ~250, increase for high throughput
    initial_window_size: "4M"            # Default: 64KB, larger for better flow control
    max_frame_size: "1M"                 # Default: 16KB, reduce syscalls
    max_header_list_size: "2M"           # Support large mTLS certificates
    idle_timeout_seconds: 120            # Connection reuse
    max_upload_buffer_per_conn: "4M"     # Memory budget per connection
    max_upload_buffer_per_stream: "4M"   # Memory budget per stream
```

**Примечание:** Значения в `http2` секции можно указывать в килобайтах (k/K) или мегабайтах (m/M), например: `"4M"`, `"512k"`, или просто байтами `"1048576"`.

### 4. Создание metadata.yaml

```bash
sudo mkdir -p /var/lib/hsm-service

# Create initial metadata
sudo nano /var/lib/hsm-service/metadata.yaml
```

**Содержимое**:
```yaml
rotation: {}
```

Установка прав:
```bash
sudo chown hsm:hsm /var/lib/hsm-service/metadata.yaml
sudo chmod 644 /var/lib/hsm-service/metadata.yaml
```

### 5. Создание revoked.yaml

```bash
# Create empty revocation list
sudo nano /etc/hsm-service/pki/revoked.yaml
```

**Содержимое**:
```yaml
revoked: []
```

### 6. Создание начальных KEK

```bash
# Switch to hsm user
sudo su - hsm

cd /opt/hsm-service

# Set HSM_PIN environment variable
export HSM_PIN=1234  # Ваш PIN!

# Create KEKs
/opt/hsm-service/bin/hsm-admin init-keys

# Verify
/opt/hsm-service/bin/hsm-admin list-kek
```

---

## Systemd Service Setup

### 1. Создание systemd unit file

```bash
sudo nano /etc/systemd/system/hsm-service.service
```

**Содержимое `/etc/systemd/system/hsm-service.service`**:
```ini
[Unit]
Description=HSM Service - Cryptographic Key Management
After=network.target
Wants=network-online.target

[Service]
Type=simple
User=hsm
Group=hsm
WorkingDirectory=/opt/hsm-service

# Environment
Environment="HSM_PIN=1234"
Environment="SLOT_LABEL=hsm-token"
EnvironmentFile=-/etc/hsm-service/environment

# Binary
ExecStart=/opt/hsm-service/bin/hsm-service

# Restart policy
Restart=on-failure
RestartSec=5s

# Security
NoNewPrivileges=true
PrivateTmp=true
ProtectSystem=strict
ProtectHome=true
ReadWritePaths=/var/lib/hsm-service /var/log/hsm-service /var/lib/softhsm/tokens

# Limits (Performance optimized for high load)
LimitNOFILE=65536
LimitNPROC=4096
LimitMEMLOCK=infinity

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=hsm-service

[Install]
WantedBy=multi-user.target
```

**Примечание:** `LimitNOFILE=65536` критично для обработки высоких нагрузок (>5000 req/s).

### 2. Kernel Network Tuning

Для максимальной производительности настройте параметры ядра:

```bash
# Edit sysctl configuration
sudo nano /etc/sysctl.d/99-hsm-service.conf
```

**Содержимое `/etc/sysctl.d/99-hsm-service.conf`**:
```ini
# Connection handling
net.core.somaxconn = 8192
net.ipv4.tcp_max_syn_backlog = 8192

# Port management (prevents port exhaustion)
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_tw_reuse = 1
net.ipv4.tcp_fin_timeout = 15

# Buffer sizes for HTTP/2
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216

# Connection tracking (for high concurrent connections)
net.netfilter.nf_conntrack_max = 524288
```

**Применить настройки:**
```bash
sudo sysctl -p /etc/sysctl.d/99-hsm-service.conf

# Verify
sysctl net.core.somaxconn
sysctl net.ipv4.tcp_tw_reuse
```

### 3. Создание environment file (рекомендуется)

```bash
sudo nano /etc/hsm-service/environment
```

**Содержимое**:
```bash
HSM_PIN=your-secure-pin-here
SLOT_LABEL=hsm-token
LOG_LEVEL=info
```

**Установка прав** (важно!):
```bash
sudo chown root:hsm /etc/hsm-service/environment
sudo chmod 640 /etc/hsm-service/environment
```

### 4. Перезагрузка systemd и запуск

```bash
# Reload systemd
sudo systemctl daemon-reload

# Enable service (auto-start on boot)
sudo systemctl enable hsm-service

# Start service
sudo systemctl start hsm-service

# Check status
sudo systemctl status hsm-service

# View logs
sudo journalctl -u hsm-service -f
```

### 5. Проверка работы

```bash
# Health check
curl -k https://localhost:8443/health \
  --cert /etc/hsm-service/pki/client/client1.crt \
  --key /etc/hsm-service/pki/client/client1.key \
  --cacert /etc/hsm-service/pki/ca/ca.crt

# Expected output:
# {"status":"healthy","active_keys":["kek-exchange-key-v1","kek-2fa-v1"]}
```

---

## Настройка nftables Firewall

### 1. Установка nftables

```bash
# Install nftables
apt install -y nftables

# Enable service
systemctl enable nftables
systemctl start nftables
```

### 2. Базовая конфигурация

```bash
sudo nano /etc/nftables.conf
```

**Полная конфигурация nftables**:
```nft
#!/usr/sbin/nft -f

# Flush existing rules
flush ruleset

# Define variables
define WAN_IF = "eth0"
define HSM_PORT = 8443
define SSH_PORT = 22
define METRICS_PORT = 9100

# Define trusted networks
define TRUSTED_NETWORKS = { 
    10.0.0.0/8,      # Internal network
    172.16.0.0/12,   # Private network
    192.168.0.0/16   # Local network
}

# Define allowed client IPs
define ALLOWED_CLIENTS = {
    10.10.10.0/24,   # Trading services subnet
    10.20.20.0/24    # 2FA services subnet
}

table inet filter {
    chain input {
        type filter hook input priority filter; policy drop;

        # Accept loopback
        iif "lo" accept

        # Accept established/related connections
        ct state established,related accept

        # Drop invalid connections
        ct state invalid drop

        # Rate limiting for new connections
        ct state new limit rate 100/second burst 200 packets accept

        # SSH from trusted networks only
        ip saddr $TRUSTED_NETWORKS tcp dport $SSH_PORT ct state new accept

        # HSM Service (mTLS) from allowed clients only
        ip saddr $ALLOWED_CLIENTS tcp dport $HSM_PORT ct state new accept

        # Prometheus metrics (optional, from monitoring server)
        # ip saddr 10.30.30.10 tcp dport $METRICS_PORT ct state new accept

        # ICMP (ping) from trusted networks
        ip saddr $TRUSTED_NETWORKS icmp type echo-request limit rate 5/second accept

        # Log dropped packets (optional, для debugging)
        # log prefix "nftables-drop: " drop

        # Drop everything else
        drop
    }

    chain forward {
        type filter hook forward priority filter; policy drop;
        # No forwarding needed for HSM service
    }

    chain output {
        type filter hook output priority filter; policy accept;
        # Allow all outbound traffic
    }
}

# Rate limiting для защиты от DDoS
table inet ratelimit {
    set ratelimit_set {
        type ipv4_addr
        size 65536
        flags dynamic,timeout
        timeout 10m
    }

    chain prerouting {
        type filter hook prerouting priority mangle; policy accept;

        # Track connection attempts per IP
        tcp dport $HSM_PORT add @ratelimit_set { ip saddr limit rate 10/second }
    }
}
```

### 3. Применение правил

```bash
# Check syntax
sudo nft -c -f /etc/nftables.conf

# Apply rules
sudo nft -f /etc/nftables.conf

# Verify rules
sudo nft list ruleset

# Save rules (persistent)
sudo systemctl enable nftables
```

### 4. Тестирование firewall

```bash
# From trusted network - should work
curl -k https://<server-ip>:8443/health \
  --cert client.crt \
  --key client.key \
  --cacert ca.crt

# From untrusted network - should fail (connection refused)

# Check logs (если включен logging)
sudo dmesg | grep nftables-drop
```

---

## Мониторинг и логирование

### 1. Логирование

```bash
# View logs
sudo journalctl -u hsm-service -f

# JSON formatted logs
sudo journalctl -u hsm-service -o json-pretty

# Filter by level
sudo journalctl -u hsm-service -p err

# Last hour
sudo journalctl -u hsm-service --since "1 hour ago"
```

### 2. Настройка logrotate

```bash
sudo nano /etc/logrotate.d/hsm-service
```

**Содержимое**:
```
/var/log/hsm-service/*.log {
    daily
    rotate 30
    compress
    delaycompress
    notifempty
    missingok
    create 0640 hsm hsm
    sharedscripts
    postrotate
        systemctl reload hsm-service > /dev/null 2>&1 || true
    endscript
}
```

### 3. Prometheus metrics

```bash
# Scrape metrics
curl -k https://localhost:8443/metrics \
  --cert /etc/hsm-service/pki/client/monitoring.crt \
  --key /etc/hsm-service/pki/client/monitoring.key \
  --cacert /etc/hsm-service/pki/ca/ca.crt
```

**Prometheus scrape config**:
```yaml
scrape_configs:
  - job_name: 'hsm-service'
    scheme: https
    tls_config:
      ca_file: /etc/prometheus/certs/ca.crt
      cert_file: /etc/prometheus/certs/client.crt
      key_file: /etc/prometheus/certs/client.key
    static_configs:
      - targets: ['hsm-service.example.com:8443']
```

---

## Автоматическая ротация KEK

### Настройка systemd timer для автоматической ротации

**1. Создать systemd service:**

```bash
sudo nano /etc/systemd/system/hsm-rotation-check.service
```

**Содержимое:**
```ini
[Unit]
Description=HSM Key Rotation Check
After=network.target hsm-service.service

[Service]
Type=oneshot
User=hsm
WorkingDirectory=/opt/hsm-service
Environment="HSM_PIN_FILE=/etc/hsm-service/pin.txt"
Environment="AUTO_ROTATE=true"
ExecStart=/opt/hsm-service/scripts/check-key-rotation.sh
StandardOutput=journal
StandardError=journal

[Install]
WantedBy=multi-user.target
```

**2. Создать systemd timer:**

```bash
sudo nano /etc/systemd/system/hsm-rotation-check.timer
```

**Содержимое:**
```ini
[Unit]
Description=HSM Key Rotation Check Timer
Requires=hsm-rotation-check.service

[Timer]
# Проверять каждый день в 3:00
OnCalendar=daily
OnCalendar=03:00
# Запустить через 5 минут после boot
OnBootSec=5min
Persistent=true

[Install]
WantedBy=timers.target
```

**3. Создать скрипт проверки:**

```bash
sudo nano /opt/hsm-service/scripts/check-key-rotation.sh
```

**Содержимое:**
```bash
#!/bin/bash
set -euo pipefail

LOG_FILE="/var/log/hsm-service/rotation.log"
ALERT_EMAIL="${ALERT_EMAIL:-ops@company.com}"
AUTO_ROTATE="${AUTO_ROTATE:-false}"
SLACK_WEBHOOK="${SLACK_WEBHOOK:-}"

log() {
    echo "[$(date +'%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

check_rotation_status() {
    /usr/local/bin/hsm-admin rotation-status | tee /tmp/rotation-status.txt
}

send_alert() {
    local subject=$1
    local body=$2
    
    # Email alert (опционально)
    if command -v mail >/dev/null 2>&1; then
        echo "$body" | mail -s "$subject" "$ALERT_EMAIL"
    fi
    
    # Slack webhook (опционально)
    if [[ -n "$SLACK_WEBHOOK" ]]; then
        curl -X POST "$SLACK_WEBHOOK" \
          -H 'Content-Type: application/json' \
          -d "{\"text\":\"⚠️  $subject\n\n\`\`\`$body\`\`\`\"}"
    fi
}

perform_rotation() {
    local context=$1
    
    log "Starting automatic rotation for context: $context"
    
    if /usr/local/bin/hsm-admin rotate "$context"; then
        log "✓ Rotation completed for $context"
        
        # Отправить webhook приложениям (опционально)
        if [[ -n "${APP_WEBHOOK:-}" ]]; then
            curl -X POST "$APP_WEBHOOK" \
              -H "Content-Type: application/json" \
              -d "{\"event\":\"key_rotation\",\"context\":\"$context\",\"timestamp\":\"$(date -Iseconds)\"}"
        fi
        
        return 0
    else
        log "✗ Rotation failed for $context"
        send_alert "HSM Rotation FAILED: $context" \
                   "Automatic rotation failed. Manual intervention required.\n\nCheck logs: sudo journalctl -u hsm-service -n 50"
        return 1
    fi
}

main() {
    log "Starting key rotation check..."
    
    check_rotation_status
    
    # Найти ключи, требующие ротации (поиск "NEEDS ROTATION")
    OVERDUE_KEYS=$(grep "NEEDS ROTATION" /tmp/rotation-status.txt | awk '{print $3}' | tr -d ':' || true)
    
    if [[ -z "$OVERDUE_KEYS" ]]; then
        log "All keys are up to date"
        exit 0
    fi
    
    log "Keys requiring rotation: $OVERDUE_KEYS"
    
    if [[ "$AUTO_ROTATE" == "true" ]]; then
        # Автоматическая ротация
        log "AUTO_ROTATE enabled, performing automatic rotation"
        for context in $OVERDUE_KEYS; do
            perform_rotation "$context"
        done
    else
        # Только алерты
        log "AUTO_ROTATE disabled, sending alerts only"
        send_alert "HSM Keys Need Rotation" "$(cat /tmp/rotation-status.txt)"
    fi
}

main
```

**4. Установить права:**

```bash
sudo chmod +x /opt/hsm-service/scripts/check-key-rotation.sh
sudo mkdir -p /var/log/hsm-service
sudo chown hsm:hsm /var/log/hsm-service
```

**5. Активировать timer:**

```bash
# Перезагрузить systemd
sudo systemctl daemon-reload

# Включить и запустить timer
sudo systemctl enable hsm-rotation-check.timer
sudo systemctl start hsm-rotation-check.timer

# Проверить статус
sudo systemctl status hsm-rotation-check.timer

# Посмотреть следующий запуск
sudo systemctl list-timers | grep hsm-rotation
```

**Вывод:**
```
NEXT                         LEFT          LAST  PASSED  UNIT                        ACTIVATES
Thu 2026-01-16 03:00:00 UTC  11h left      n/a   n/a     hsm-rotation-check.timer    hsm-rotation-check.service
```

### Тестирование автоматической ротации

**Запустить проверку вручную:**

```bash
# Запустить service вручную
sudo systemctl start hsm-rotation-check.service

# Посмотреть результат
sudo journalctl -u hsm-rotation-check.service -n 50

# Проверить лог
sudo tail -f /var/log/hsm-service/rotation.log
```

**Симуляция просроченного ключа:**

```bash
# Изменить дату создания ключа в metadata.yaml
sudo nano /var/lib/hsm-service/metadata.yaml

# Изменить created_at на дату 91 день назад
# created_at: '2025-10-15T00:00:00Z'

# Запустить проверку
sudo systemctl start hsm-rotation-check.service

# Проверить, что ротация сработала
sudo /usr/local/bin/hsm-admin rotation-status
```

### Настройка уведомлений

**Email уведомления (опционально):**

```bash
# Установить mailutils
sudo apt install -y mailutils

# Настроить SMTP (например, через Gmail)
sudo nano /etc/ssmtp/ssmtp.conf
```

**Slack webhook (рекомендуется):**

```bash
# Создать Incoming Webhook в Slack
# https://api.slack.com/messaging/webhooks

# Добавить в environment
sudo nano /etc/systemd/system/hsm-rotation-check.service

# В секцию [Service] добавить:
Environment="SLACK_WEBHOOK=https://hooks.slack.com/services/YOUR/WEBHOOK/URL"
Environment="ALERT_EMAIL=ops@company.com"
Environment="APP_WEBHOOK=https://your-app.com/api/webhooks/key-rotation"

# Применить
sudo systemctl daemon-reload
```

### Режимы работы

**1. Только алерты (по умолчанию):**

```ini
# /etc/systemd/system/hsm-rotation-check.service
Environment="AUTO_ROTATE=false"
```

При обнаружении просроченных ключей:
- ✅ Отправляет email/Slack уведомление
- ❌ НЕ выполняет автоматическую ротацию
- 👤 Требуется ручная ротация оператором

**2. Автоматическая ротация:**

```ini
# /etc/systemd/system/hsm-rotation-check.service
Environment="AUTO_ROTATE=true"
```

При обнаружении просроченных ключей:
- ✅ Автоматически выполняет ротацию
- ✅ Отправляет уведомление об успехе/ошибке
- ✅ Отправляет webhook приложениям для re-encryption
- ⚡ Zero-downtime через hot reload

### Мониторинг ротации

**Проверить статус ключей:**

```bash
sudo /usr/local/bin/hsm-admin rotation-status
```

**Проверить историю ротаций:**

```bash
# Логи ротации
sudo tail -50 /var/log/hsm-service/rotation.log

# Systemd journal
sudo journalctl -u hsm-rotation-check.service --since "7 days ago"
```

**Метрики Prometheus (если настроен):**

```promql
# Дни до следующей ротации
hsm_key_rotation_days_remaining{context="exchange-key"}

# Количество успешных ротаций
hsm_key_rotation_success_total

# Количество ошибок ротации
hsm_key_rotation_failed_total
```

---

## Бэкапы

### 1. Backup script

```bash
sudo nano /opt/hsm-service/scripts/backup.sh
```

**Содержимое**:
```bash
#!/bin/bash
set -e

BACKUP_DIR="/var/backups/hsm-service"
DATE=$(date +%Y%m%d-%H%M%S)

mkdir -p "$BACKUP_DIR"

# Backup SoftHSM tokens
tar -czf "$BACKUP_DIR/tokens-$DATE.tar.gz" /var/lib/softhsm/tokens/

# Backup metadata
cp /var/lib/hsm-service/metadata.yaml "$BACKUP_DIR/metadata-$DATE.yaml"

# Backup config (без sensitive data!)
cp /etc/hsm-service/config.yaml "$BACKUP_DIR/config-$DATE.yaml"

# Backup PKI (опционально)
tar -czf "$BACKUP_DIR/pki-$DATE.tar.gz" /etc/hsm-service/pki/

# Keep only last 30 days
find "$BACKUP_DIR" -type f -mtime +30 -delete

echo "Backup completed: $DATE"
```

### 2. Cron job для автоматических бэкапов

```bash
sudo crontab -e -u hsm
```

**Добавить**:
```cron
# Daily backup at 2 AM
0 2 * * * /opt/hsm-service/scripts/backup.sh >> /var/log/hsm-service/backup.log 2>&1
```

### 3. Restore из backup

```bash
#!/bin/bash
BACKUP_FILE=$1

# Stop service
sudo systemctl stop hsm-service

# Restore tokens
sudo tar -xzf "$BACKUP_FILE" -C /

# Restore metadata
sudo cp metadata-YYYYMMDD-HHMMSS.yaml /var/lib/hsm-service/metadata.yaml

# Start service
sudo systemctl start hsm-service
```

---

## Безопасность

### 1. Hardening systemd service

```ini
# Add to /etc/systemd/system/hsm-service.service

[Service]
# Security hardening
PrivateDevices=true
ProtectKernelTunables=true
ProtectKernelModules=true
ProtectControlGroups=true
RestrictAddressFamilies=AF_INET AF_INET6
RestrictNamespaces=true
LockPersonality=true
MemoryDenyWriteExecute=true
RestrictRealtime=true
RestrictSUIDSGID=true
RemoveIPC=true
CapabilityBoundingSet=CAP_NET_BIND_SERVICE
```

### 2. Fail2ban для брутфорса

```bash
sudo nano /etc/fail2ban/filter.d/hsm-service.conf
```

**Содержимое**:
```ini
[Definition]
failregex = ^.*access denied.*client_cn=<HOST>.*$
            ^.*certificate revoked.*client_cn=<HOST>.*$
            ^.*rate limit exceeded.*client_cn=<HOST>.*$
ignoreregex =
```

```bash
sudo nano /etc/fail2ban/jail.d/hsm-service.conf
```

**Содержимое**:
```ini
[hsm-service]
enabled = true
filter = hsm-service
logpath = /var/log/hsm-service/*.log
maxretry = 5
findtime = 600
bantime = 3600
action = nftables-multiport[name=hsm, port="8443", protocol=tcp]
```

### 3. SELinux/AppArmor (опционально)

Debian по умолчанию не использует SELinux, но можно настроить AppArmor:

```bash
# Install AppArmor
apt install -y apparmor apparmor-utils

# Create profile
sudo aa-genprof /opt/hsm-service/hsm-service

# Enable profile
sudo aa-enforce /opt/hsm-service/hsm-service
```

---

## Troubleshooting

### Problem: Service не запускается

```bash
# Check logs
sudo journalctl -u hsm-service -n 100

# Check configuration
sudo -u hsm /opt/hsm-service/hsm-service --help

# Test manually
sudo -u hsm sh -c 'export HSM_PIN=1234 && /opt/hsm-service/hsm-service'
```

### Problem: Permission denied на tokens

```bash
# Fix ownership
sudo chown -R hsm:hsm /var/lib/softhsm/tokens

# Fix permissions
sudo chmod 700 /var/lib/softhsm/tokens
```

### Problem: Certificate errors

```bash
# Verify certificates
openssl x509 -in /etc/hsm-service/pki/server/server.crt -noout -text

# Test TLS handshake
openssl s_client -connect localhost:8443 \
  -cert /etc/hsm-service/pki/client/client1.crt \
  -key /etc/hsm-service/pki/client/client1.key \
  -CAfile /etc/hsm-service/pki/ca/ca.crt
```

### Problem: Firewall блокирует запросы

```bash
# Temporarily flush rules
sudo nft flush ruleset

# Test connectivity
curl -k https://localhost:8443/health ...

# Restore rules
sudo nft -f /etc/nftables.conf

# Check logs
sudo dmesg | grep nft
```

---

## Мониторинг производительности

### System metrics

```bash
# CPU usage
htop

# Memory usage
free -h

# Disk I/O
iotop

# Network
nethogs
```

### Service metrics

```bash
# Request count
curl -k https://localhost:8443/metrics ... | grep hsm_requests_total

# Error rate
curl -k https://localhost:8443/metrics ... | grep hsm_errors_total

# Latency
curl -k https://localhost:8443/metrics ... | grep hsm_request_duration
```

---

## Обновление сервиса

```bash
# Stop service
sudo systemctl stop hsm-service

# Backup current version
sudo cp /opt/hsm-service/hsm-service /opt/hsm-service/hsm-service.backup

# Update code
cd /opt/hsm-service
git pull

# Rebuild
go build -o hsm-service .

# Start service
sudo systemctl start hsm-service

# Check logs
sudo journalctl -u hsm-service -f
```

---

## Production Checklist

Перед запуском в production:

- [ ] Изменены default PIN'ы (не 1234!)
- [ ] Настроены сильные сертификаты (не self-signed)
- [ ] Настроен nftables firewall
- [ ] Настроен Prometheus мониторинг
- [ ] Настроены алерты
- [ ] Настроены автоматические бэкапы
- [ ] Настроена автоматическая ротация KEK (systemd timer)
- [ ] Протестирована ротация ключей
- [ ] Настроены уведомления о ротации (email/Slack)
- [ ] Настроен logrotate
- [ ] Включен fail2ban
- [ ] Проведен security audit
- [ ] Настроен disaster recovery plan
- [ ] Документированы operational procedures
- [ ] Обучены операторы

---

## Дополнительные ресурсы

- [QUICKSTART_DOCKER.md](QUICKSTART_DOCKER.md) - Быстрый старт (Docker)
- [BUILD.md](BUILD.md) - Сборка бинарников
- [API.md](API.md) - API документация
- [MONITORING.md](MONITORING.md) - Мониторинг и алерты
- [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - Security audit
- [KEY_ROTATION.md](KEY_ROTATION.md) - Ротация ключей
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Решение проблем
- [tests/performance/README.md](tests/performance/README.md) - Performance тестирование

---

## Performance Testing Production

### Базовая проверка (безопасно)

```bash
# 1. Smoke test - минимальная нагрузка
HSM_URL=https://your-prod-server.com:8443 \
CLIENT_CERT=/path/to/prod-client.crt \
CLIENT_KEY=/path/to/prod-client.key \
./tests/performance/smoke-test.sh
```

**Результат**: Проверяет health, encrypt, decrypt. Занимает ~5 секунд.

---

### Quick Load Test (умеренная нагрузка)

```bash
# 2. Quick test - 20 concurrent users, 2 минуты
HSM_URL=https://your-prod-server.com:8443 \
CLIENT_CERT=/path/to/prod-client.crt \
CLIENT_KEY=/path/to/prod-client.key \
k6 run tests/performance/load-test-quick.js
```

**Результат**: ~3500 запросов за 2 минуты. Безопасно для production.

---

### Full Load Test (⚠️ требует согласования)

```bash
# 3. Full test - 22 минуты, до 200 concurrent users
HSM_URL=https://your-prod-server.com:8443 \
CLIENT_CERT=/path/to/prod-client.crt \
CLIENT_KEY=/path/to/prod-client.key \
k6 run tests/performance/load-test.js
```

**⚠️ Внимание**: Выполнять только в maintenance window или согласовать с командой.

---

### Stress Testing (⚠️ ТОЛЬКО в maintenance window)

```bash
# 4. Stress test - поиск breaking point
HSM_URL=https://your-prod-server.com:8443 \
CLIENT_CERT=/path/to/prod-client.crt \
CLIENT_KEY=/path/to/prod-client.key \
./tests/performance/stress-test.sh incremental
```

**⚠️ Внимание**: Может создать значительную нагрузку. Только с разрешения!

---

### Рекомендуемая последовательность

1. **Перед каждым тестом**: Уведомите команду
2. **Первый раз**: smoke → quick (в нерабочее время)
3. **Регулярно**: smoke test (мониторинг деградации)
4. **Периодически**: full load (quarterly, в maintenance window)
5. **Редко**: stress test (capacity planning, в maintenance window)

**Документация**: См. [tests/performance/README.md](tests/performance/README.md)
