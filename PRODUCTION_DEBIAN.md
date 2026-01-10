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
  requests_per_second: 100
  burst: 50

logging:
  level: info
  format: json
```

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

# Limits
LimitNOFILE=65536
LimitNPROC=4096

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=hsm-service

[Install]
WantedBy=multi-user.target
```

### 2. Создание environment file (рекомендуется)

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

### 3. Перезагрузка systemd и запуск

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

### 4. Проверка работы

```bash
# Health check
curl -k https://localhost:8443/health \
  --cert /etc/hsm-service/pki/client/client1.crt \
  --key /etc/hsm-service/pki/client/client1.key \
  --cacert /etc/hsm-service/pki/ca/ca.crt

# Expected output:
# {"status":"healthy","active_keys":["kek-exchange-v1","kek-2fa-v1"]}
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
- [ ] Протестирована ротация ключей
- [ ] Настроен logrotate
- [ ] Включен fail2ban
- [ ] Проведен security audit
- [ ] Настроен disaster recovery plan
- [ ] Документированы operational procedures
- [ ] Обучены операторы

---

## Дополнительные ресурсы

- [QUICKSTART_DOCKER.md](QUICKSTART_DOCKER.md) - Быстрый старт (Docker)
- [QUICKSTART_NATIVE.md](QUICKSTART_NATIVE.md) - Быстрый старт (Native binary)
- [API.md](API.md) - API документация
- [MONITORING.md](MONITORING.md) - Мониторинг и алерты
- [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - Security audit
- [KEY_ROTATION.md](KEY_ROTATION.md) - Ротация ключей
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Решение проблем
