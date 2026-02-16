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
- [Нагрузочное тестирование перед внедрением](#нагрузочное-тестирование-перед-внедрением)
- [Troubleshooting](#troubleshooting)

---

## Системные требования

### Минимальные

- **OS**: Debian 13 (Trixie) или Debian 12 (Bookworm)
- **CPU**: 1 cores
- **RAM**: 1 GB
- **Disk**: 20 GB
- **Network**: Статический IP

### Рекомендуемые

- **OS**: Debian 13 (Trixie)
- **CPU**: 4 cores
- **RAM**: 2 GB
- **Disk**: 50 GB
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
timedatectl set-timezone UTC

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

dpkg -s opensc | grep Version
# Version: 0.26.1-2
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
sudo chown -R hsm:hsm /var/lib/softhsm
sudo chown -R hsm:hsm /var/log/hsm-service
sudo chown -R hsm:hsm /etc/hsm-service
sudo chown -R hsm:hsm /etc/softhsm

# Set permissions
sudo chmod 755 /opt/hsm-service
sudo chmod 700 /var/lib/softhsm/tokens
sudo chmod 750 /var/log/hsm-service
sudo chmod 755 /etc/hsm-service
```

### 2. Копирование бинарников

```bash
# Скопировать с build-сервера (с вашего CI/CD или локально)
scp hsm-service hsm@production-server:/opt/hsm-service/bin/
scp hsm-admin hsm@production-server:/opt/hsm-service/bin/

# Установить права выполнения
ssh hsm@production-server "chmod +x /opt/hsm-service/bin/hsm-service /opt/hsm-service/bin/hsm-admin"

# Проверка бинарников
ssh hsm@production-server "ls -lh /opt/hsm-service/bin/"
# -rwxr-xr-x 1 hsm hsm 12M Jan 19 10:00 hsm-service
# -rwxr-xr-x 1 hsm hsm 10M Jan 19 10:01 hsm-admin
```

**Примечание**: `create-kek` больше не требуется для инициализации. Используйте `hsm-admin create-kek`.

---

## Настройка PKI

> **📖 Детальная инструкция**: См. [PKI_SETUP.md](PKI_SETUP.md) для создания CA и генерации сертификатов

### Копирование существующих сертификатов

```bash
# Создать директории
sudo mkdir -p /etc/hsm-service/pki/{ca,server}

# Скопировать сертификаты с CA-сервера или локально
sudo cp /path/to/ca.crt /etc/hsm-service/pki/ca/
sudo cp /path/to/hsm-service.crt /etc/hsm-service/pki/server/
sudo cp /path/to/hsm-service.key /etc/hsm-service/pki/server/

# Set ownership
sudo chown -R hsm:hsm /etc/hsm-service/pki

# Set permissions (КРИТИЧЕСКИ ВАЖНО!)
sudo chmod 600 /etc/hsm-service/pki/server/*.key
sudo chmod 644 /etc/hsm-service/pki/ca/*.crt
sudo chmod 644 /etc/hsm-service/pki/server/*.crt
```

**Проверка**:
```bash
# Проверить серверный сертификат
openssl verify -CAfile /etc/hsm-service/pki/ca/ca.crt /etc/hsm-service/pki/server/hsm-service.crt
# /etc/hsm-service/pki/server/hsm-service.crt: OK
```

```

## Конфигурация сервиса

### 1. Конфигурация SoftHSM (опционально)

SoftHSM по умолчанию ищет конфиг в `/etc/softhsm/softhsm2.conf`. Если вы используете стандартные пути, этот шаг можно пропустить.

**Если нужно использовать custom пути:**

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

**Альтернатива**: Указать путь через переменную окружения (в systemd service):
```ini
Environment="SOFTHSM2_CONF=/etc/softhsm/softhsm2.conf"
```

### 2. Инициализация HSM токена

```bash
# Initialize token (от root)
sudo softhsm2-util --init-token \
  --slot 0 \
  --label "hsm-token" \
  --so-pin 5678 \
  --pin 1234

# ВАЖНО: Используйте сильные PIN'ы на production!
# Пример генерации для Prod: openssl rand -hex 32
# Запишите PIN в безопасное место (KMS, Vault)

# ⚠️ КРИТИЧЕСКИ ВАЖНО: Исправить права доступа после инициализации
# SoftHSM создает файлы токена от root, но hsm пользователь должен иметь доступ
sudo chown -R hsm:hsm /var/lib/softhsm/tokens/
sudo chmod 700 /var/lib/softhsm/tokens/
sudo find /var/lib/softhsm/tokens/ -type f -exec chmod 600 {} \;
sudo chown hsm:hsm /etc/softhsm/softhsm2.conf

# Verify (от root)
sudo softhsm2-util --show-slots

# Verify (от пользователя hsm - должно работать)
sudo -u hsm softhsm2-util --show-slots
```

**⚠️ Проверка отказа в доступе при использовании от hsm:**

Если пользователь `hsm` получает ошибку:
```
ERROR: Could not load the SoftHSM configuration.
ERROR: Please verify that the SoftHSM configuration is correct.
```

**Решение:**

```bash
# 1. Убедиться что конфиг доступен для чтения
sudo -u hsm cat /etc/softhsm/softhsm2.conf
# Если файл не существует или не доступен, проверить права

# 2. Если конфиг в custom пути, установить переменную окружения
export SOFTHSM2_CONF=/etc/softhsm/softhsm2.conf
sudo -u hsm sh -c 'export SOFTHSM2_CONF=/etc/softhsm/softhsm2.conf && softhsm2-util --show-slots'

# 3. Проверить права на директорию токенов
ls -la /var/lib/softhsm/
ls -la /var/lib/softhsm/tokens/

# 4. Если права неправильные, исправить
sudo chown -R hsm:hsm /var/lib/softhsm/tokens/
sudo chmod 700 /var/lib/softhsm/tokens/
sudo chown hsm:hsm /var/lib/softhsm
sudo chown -R hsm:hsm /etc/softhsm
```

### 3. Конфигурация HSM Service

```bash
# Copy config template
sudo cp /path/to/config.yaml.example /etc/hsm-service/config.yaml

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
  # HTTP/2 optimization for high-load scenarios  
  http2:
    max_concurrent_streams: "2000"       # Default: ~250, increase for high throughput
    initial_window_size: "4M"            # Default: 64KB, larger for better flow control
    max_frame_size: "1M"                 # Default: 16KB, reduce syscalls
    max_header_list_size: "2M"           # Support large mTLS certificates
    idle_timeout_seconds: 120            # Connection reuse
    max_upload_buffer_per_conn: "4M"     # Memory budget per connection
    max_upload_buffer_per_stream: "4M"   # Memory budget per stream

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
  error_path: /var/log/hsm-service/error.log
  audit_path: /var/log/hsm-service/audit.log
  access_path: /var/log/hsm-service/access.log
  max_size_mb: 100
  max_backups: 10
  max_age_days: 30
  compress: true
  audit_to_stdout: true
  access_to_stdout: true
  audit_mirror_to_error_on_debug: true
```

**Примечание:** Значения в `http2` секции можно указывать в килобайтах (k/K) или мегабайтах (m/M), например: `"4M"`, `"512k"`, или просто байтами `"1048576"`.

### 4. Создание metadata.yaml

```bash
sudo mkdir -p /var/lib/hsm-service

# Create initial metadata
sudo touch /var/lib/hsm-service/metadata.yaml
```

Установка прав:
```bash
sudo chown -R hsm:hsm /var/lib/hsm-service
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
```bash
sudo chown hsm:hsm /etc/hsm-service/pki/revoked.yaml
```

### 6. Создание начальных KEK

**Важно**: KEK должны быть связаны с контекстами через metadata.yaml. Процесс инициализации:

```bash
# Switch to hsm user
sudo su - hsm

# Set HSM_PIN environment variable
export HSM_PIN=1234  # Ваш PIN!

# Шаг 1: Создать ключи в HSM с помощью hsm-admin
/opt/hsm-service/bin/hsm-admin create-kek --label kek-exchange-key-v1 --context exchange-key --version 1
/opt/hsm-service/bin/hsm-admin create-kek --label kek-2fa-v1 --context 2fa --version 1

# Шаг 2: Инициализировать metadata.yaml с контекстами
# Это связывает физические ключи с логическими контекстами
# CURRENT_DATE подставляется автоматически в ISO8601 формат
CURRENT_DATE=$(date -u +'%Y-%m-%dT%H:%M:%SZ')

cat > /var/lib/hsm-service/metadata.yaml << EOF
rotation:
  exchange-key:
    current: kek-exchange-key-v1
    rotation_interval_days: 90
    versions:
      - label: kek-exchange-key-v1
        version: 1
        created_at: "$CURRENT_DATE"
  2fa:
    current: kek-2fa-v1
    rotation_interval_days: 90
    versions:
      - label: kek-2fa-v1
        version: 1
        created_at: "$CURRENT_DATE"
EOF

# Шаг 3: Обновить checksums для проверки целостности
/opt/hsm-service/bin/hsm-admin update-checksums

# Шаг 4: Проверить что всё настроено правильно
echo ""
echo "Checking KEKs in HSM:"
/opt/hsm-service/bin/hsm-admin list-kek

echo ""
echo "Checking rotation status:"
/opt/hsm-service/bin/hsm-admin rotation-status
```

**Как это работает:**

1. **`hsm-admin create-kek`** - создает физический ключ в HSM (PKCS#11 операция)
  - Параметры: `hsm-admin create-kek --label <label> --context <context> [--version <n>] [--size <bits>]`
  - Создает ключ с меткой `kek-exchange-key-v1`

2. **metadata.yaml** - описывает логическую структуру ключей
   - Связывает контекст (например `exchange-key`) с физическим ключом
   - Хранит историю версий ключей
   - Используется `hsm-admin` для управления ротацией

3. **`hsm-admin update-checksums`** - вычисляет и сохраняет checksums
   - Используется для проверки целостности ключей

**Параметры `hsm-admin create-kek`:**
- `--label` - Уникальное имя ключа (например: `kek-exchange-key-v1`)
- `--context` - Контекст (например: `exchange-key`)
- `--version` - Номер версии (опционально, по умолчанию: 1)
- `--size` - Размер ключа (128, 192, 256; по умолчанию: 256)

**Примечание о PIN'ах:**
- **`HSM_PIN`** (флаг `--pin` при инициализации токена) - обычный PIN пользователя для доступа к ключам
- **`SO_PIN`** (флаг `--so-pin` при инициализации токена) - PIN администратора, нужен только для управления самим токеном

**Доступные hsm-admin команды:**

> ⚠️ ВАЖНО о sudo и HSM_PIN: `sudo` по умолчанию очищает окружение и теряет `HSM_PIN`!

```bash
# hsm-admin автоматически ищет config.yaml в этом порядке:
# 1. Переменная окружения CONFIG_PATH
# 2. Текущая директория (./config.yaml)
# 3. /etc/hsm-service/config.yaml

# ✓ СПОСОБ 1: Запуск от пользователя hsm БЕЗ sudo (рекомендуется)
# HSM_PIN загружается автоматически из окружения
sudo -u hsm bash -c 'source /etc/hsm-service/environment && /opt/hsm-service/bin/hsm-admin list-kek'
sudo -u hsm bash -c 'source /etc/hsm-service/environment && /opt/hsm-service/bin/hsm-admin update-checksums'
sudo -u hsm bash -c 'source /etc/hsm-service/environment && /opt/hsm-service/bin/hsm-admin rotation-status'

# ✓ СПОСОБ 2: Передать PIN явно через sudo env
sudo env HSM_PIN=$HSM_PIN /opt/hsm-service/bin/hsm-admin list-kek
sudo env HSM_PIN=$HSM_PIN /opt/hsm-service/bin/hsm-admin update-checksums

# ✓ СПОСОБ 3: Использовать sudo -E (сохранить окружение, небезопасно!)
# ⚠️ Используйте только если HSM_PIN уже в окружении и вы уверены
sudo -E /opt/hsm-service/bin/hsm-admin list-kek

# ✓ СПОСОБ 4: Через переменную окружения (если запуск от root)
export CONFIG_PATH=/etc/hsm-service/config.yaml
export HSM_PIN=$(grep HSM_PIN /etc/hsm-service/environment | cut -d= -f2)
/opt/hsm-service/bin/hsm-admin list-kek

# ✓ СПОСОБ 5: По умолчанию (если запуск прямо от пользователя hsm)
# При запуске от пользователя hsm, конфиг найдется автоматически
# но HSM_PIN должен быть в окружении
source /etc/hsm-service/environment
hsm-admin list-kek
hsm-admin update-checksums
```

**Рекомендуемый вариант для production:**
```bash
# Прочитать PIN из защищённого файла и передать явно
HSM_PIN=$(grep '^HSM_PIN=' /etc/hsm-service/environment | cut -d= -f2-)
sudo -u hsm bash -c "HSM_PIN='$HSM_PIN' /opt/hsm-service/bin/hsm-admin list-kek"

# Или одной командой (если вы уже в консоли hsm пользователя):
sudo -u hsm bash -c "HSM_PIN=\$(grep '^HSM_PIN=' /etc/hsm-service/environment | cut -d= -f2-) && /opt/hsm-service/bin/hsm-admin list-kek"
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

# Environment - НЕ ХРАНИТЬ PIN здесь! Используйте EnvironmentFile!
Environment="SLOT_LABEL=hsm-token"
Environment="CONFIG_PATH=/etc/hsm-service/config.yaml"
# PIN загружается из защищённого файла /etc/hsm-service/environment
EnvironmentFile=/etc/hsm-service/environment

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
ReadWritePaths=/var/lib/hsm-service /var/log/hsm-service /var/lib/softhsm

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
echo "nf_conntrack" | sudo tee /etc/modules-load.d/nf_conntrack.conf
```

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
sudo sysctl net.core.somaxconn
sudo sysctl net.ipv4.tcp_tw_reuse
```

### 3. Создание environment file (ОБЯЗАТЕЛЬНО для production!)

**⚠️ КРИТИЧЕСКИ ВАЖНО**: НЕ ХРАНИТЬ PIN в systemd unit файле! Используйте отдельный защищённый файл.

```bash
sudo nano /etc/hsm-service/environment
```

**Содержимое** (с вашим реальным PIN):
```bash
# ⚠️ БЕЗОПАСНОСТЬ: Этот файл содержит secrets!
# Не коммитить в git, не выкладывать в публику!

# HSM PIN (используется для доступа к ключам в HSM)
HSM_PIN=your-secret-pin-here-use-strong-pin

# Логирование
LOG_LEVEL=info
```

**Установка безопасных прав** (ОБЯЗАТЕЛЬНО!):
```bash
# Владелец: root, доступ только для чтения пользователем hsm
sudo chown root:hsm /etc/hsm-service/environment
sudo chmod 640 /etc/hsm-service/environment

# Проверка (должно быть -rw-r-----)
ls -la /etc/hsm-service/environment
# -rw-r----- 1 root hsm 256 Jan 22 10:00 /etc/hsm-service/environment
```

**Генерация сильного PIN для production:**

```bash
# Вместо слабого "1234", используйте криптографически стойкий PIN:
openssl rand -hex 32
# Пример вывода: 125a1bf04387ed172eda63b3c6a341a84e23eb2b78a39efd7c23b0d2340ae02d


# Или используйте KMS/Vault для управления PIN'ами:
# - AWS Secrets Manager
# - HashiCorp Vault
# - Azure Key Vault
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
sudo apt install -y nftables

# Enable service
sudo systemctl enable nftables
sudo systemctl start nftables
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

        # SSH from trusted networks only (ПЕРЕД rate limiting!)
        ip saddr $TRUSTED_NETWORKS tcp dport $SSH_PORT ct state new accept

        # HSM Service (mTLS) from allowed clients only (ПЕРЕД rate limiting!)
        ip saddr $ALLOWED_CLIENTS tcp dport $HSM_PORT ct state new accept

        # Prometheus metrics (optional, from monitoring server)
        # ip saddr 10.30.30.10 tcp dport $METRICS_PORT ct state new accept

        # ICMP (ping) from trusted networks
        ip saddr $TRUSTED_NETWORKS icmp type echo-request limit rate 5/second accept

        # Log dropped packets (для debugging блокировок firewall)
        # log prefix "nftables-drop: " level debug
        tcp flags syn ct state new log prefix "nftables-drop: " level debug drop

        # Drop everything else (запрещены все остальные соединения)
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

Потоки логов:
- `/var/log/hsm-service/audit.log` (PCI DSS аудит)
- `/var/log/hsm-service/access.log` (все HTTP запросы)
- `/var/log/hsm-service/error.log` (системные ошибки)

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
Group=hsm
WorkingDirectory=/opt/hsm-service

# Load environment variables (EnvironmentFile с минусом игнорирует ошибку если файла нет)
EnvironmentFile=-/etc/hsm-service/environment

# Ensure variables are exported to child processes
Environment="AUTO_ROTATE=true"

# Shell interpreter to ensure proper variable expansion
ExecStart=/bin/bash /opt/hsm-service/scripts/check-key-rotation.sh

StandardOutput=journal
StandardError=journal
SyslogIdentifier=hsm-rotation-check

[Install]
WantedBy=multi-user.target
```

**Примечание:**
- `EnvironmentFile=-/etc/hsm-service/environment` загружает переменные из файла (минус = игнорировать ошибки)
- Переменные из файла автоматически доступны скрипту
- `Environment="AUTO_ROTATE=true"` включает автоматическую ротацию и cleanup
- `ExecStart=/bin/bash` гарантирует что переменные правильно передаются shell скрипту

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
OnCalendar=*-*-* 03:00:00
# Запустить через 5 минут после boot
OnBootSec=5min
Persistent=true

[Install]
WantedBy=timers.target
```

**3. Установить права:**

```bash
sudo chmod +x /opt/hsm-service/scripts/check-key-rotation.sh
```

**4. Активировать timer:**

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

# Изменить created_at на дату 91 день назад (RFC3339 с микросекундами)
# created_at: 2025-10-15T00:00:00.000000Z

# Запустить проверку
sudo systemctl start hsm-rotation-check.service

# Проверить, что ротация сработала
/opt/hsm-service/bin/hsm-admin rotation-status
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

# Добавить в environment файл
sudo nano /etc/hsm-service/environment

# Добавить переменные:
SLACK_WEBHOOK=https://hooks.slack.com/services/YOUR/WEBHOOK/URL
ALERT_EMAIL=ops@company.com
APP_WEBHOOK=https://your-app.com/api/webhooks/key-rotation

# Убедиться в правильных правах
sudo chown root:hsm /etc/hsm-service/environment
sudo chmod 640 /etc/hsm-service/environment
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
- 👤 Требуется ручная ротация оператором: `hsm-admin rotate <context>`
- ❌ Старые ключи НЕ удаляются

**2. Автоматическая ротация и cleanup:**

```ini
# /etc/systemd/system/hsm-rotation-check.service
Environment="AUTO_ROTATE=true"
```

При обнаружении просроченных ключей скрипт выполняет:
1. ✅ Автоматически создает новый ключ (`hsm-admin rotate`)
2. ✅ **Автоматически удаляет старые ключи** (`hsm-admin cleanup-old-versions --force`)
3. ✅ Отправляет webhook приложениям для re-encryption (zero-downtime)
4. ✅ Отправляет уведомление об успехе/ошибке
5. ⚡ Zero-downtime через hot reload HSM service

**⚠️ ВАЖНО: Почему cleanup нужен?**

Команда `hsm-admin rotate` только создает новый ключ, но **не удаляет старый**. Для PCI DSS compliance и безопасности нужно запустить `cleanup-old-versions` отдельно. Начиная с AUTO_ROTATE=true это происходит автоматически после успешной ротации.

### Мониторинг ротации

**Проверить статус ключей:**

```bash
sudo /opt/hsm-service/bin/hsm-admin rotation-status
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

```bash
sudo chown hsm:hsm /opt/hsm-service/scripts/backup.sh
sudo chmod u+x /opt/hsm-service/scripts/backup.sh
```

### 2. Cron job для автоматических бэкапов

```bash
sudo crontab -e -u hsm
```

**Добавить**:
```cron
# Daily backup at 2 AM
0 2 * * * sudo /opt/hsm-service/scripts/backup.sh >> /var/log/hsm-service/backup.log 2>&1
```

### 3. Restore из backup

**Создать restore скрипт:**

```bash
sudo nano /opt/hsm-service/scripts/restore.sh
```

**Содержимое** (полный restore процесс):
```bash
#!/bin/bash
set -e

BACKUP_DIR="/var/backups/hsm-service"
BACKUP_FILE="${1:-}"

if [ -z "$BACKUP_FILE" ]; then
    echo "Usage: $0 <backup-file> [metadata-file]"
    echo ""
    echo "Available backups:"
    ls -lh "$BACKUP_DIR"/tokens-*.tar.gz 2>/dev/null || echo "No backups found"
    exit 1
fi

METADATA_FILE="${2:-}"

if [ ! -f "$BACKUP_FILE" ]; then
    echo "Error: Backup file not found: $BACKUP_FILE"
    exit 1
fi

echo "=========================================="
echo "HSM Service - Restore from Backup"
echo "=========================================="
echo ""
echo "⚠️  WARNING: This will stop HSM service and restore from backup"
echo "Backup file: $BACKUP_FILE"
echo ""
read -p "Continue? (yes/no): " confirm
if [ "$confirm" != "yes" ]; then
    echo "Restore cancelled"
    exit 0
fi

echo ""
echo "Step 1: Stopping HSM service..."
sudo systemctl stop hsm-service
echo "✓ HSM service stopped"

echo ""
echo "Step 2: Restoring SoftHSM tokens..."
sudo tar -xzf "$BACKUP_FILE" -C / 2>&1 | grep -E "^var/lib/softhsm" || true
echo "✓ Tokens restored"

echo ""
echo "Step 3: Fixing permissions on tokens..."
sudo chown -R hsm:hsm /var/lib/softhsm/tokens/
sudo chmod 700 /var/lib/softhsm/tokens/
sudo find /var/lib/softhsm/tokens/ -type f -exec chmod 600 {} \;
echo "✓ Permissions fixed"

# Restore metadata if provided
if [ -n "$METADATA_FILE" ] && [ -f "$METADATA_FILE" ]; then
    echo ""
    echo "Step 4: Restoring metadata.yaml..."
    sudo cp "$METADATA_FILE" /var/lib/hsm-service/metadata.yaml
    sudo chown hsm:hsm /var/lib/hsm-service/metadata.yaml
    echo "✓ Metadata restored"
fi

echo ""
echo "Step 5: Starting HSM service..."
sudo systemctl start hsm-service
sleep 2

if sudo systemctl is-active --quiet hsm-service; then
    echo "✓ HSM service started successfully"
else
    echo "✗ ERROR: HSM service failed to start"
    echo "Check logs: sudo journalctl -u hsm-service -n 50"
    exit 1
fi

echo ""
echo "Step 6: Verifying restored keys..."
# Wait for service to be ready
sleep 2

# Read HSM_PIN from environment file
HSM_PIN_VALUE=$(grep '^HSM_PIN=' /etc/hsm-service/environment | cut -d= -f2-)
if [ -z "$HSM_PIN_VALUE" ]; then
    echo "✗ ERROR: Could not read HSM_PIN from /etc/hsm-service/environment"
    exit 1
fi

# Check if keys are available
# Run as hsm user with HSM_PIN passed explicitly
if sudo -u hsm bash -c "HSM_PIN='$HSM_PIN_VALUE' /opt/hsm-service/bin/hsm-admin list-kek" >/dev/null 2>&1; then
    echo "✓ Keys verified"
    sudo -u hsm bash -c "HSM_PIN='$HSM_PIN_VALUE' /opt/hsm-service/bin/hsm-admin list-kek"
else
    echo "✗ ERROR: Could not verify keys"
    echo "Try running manually:"
    echo "  HSM_PIN='your-pin' sudo -u hsm /opt/hsm-service/bin/hsm-admin list-kek"
    exit 1
fi

echo ""
echo "Step 7: Updating checksums..."
# This is CRITICAL - update checksums after restore
if sudo -u hsm bash -c "HSM_PIN='$HSM_PIN_VALUE' /opt/hsm-service/bin/hsm-admin update-checksums"; then
    echo "✓ Checksums updated successfully"
else
    echo "⚠️  Warning: Could not update checksums (non-critical)"
    echo "You may need to run manually:"
    echo "  HSM_PIN='your-pin' sudo -u hsm /opt/hsm-service/bin/hsm-admin update-checksums"
fi

echo ""
echo "=========================================="
echo "✓ Restore completed successfully!"
echo "=========================================="
echo ""
echo "Next steps:"
echo "  1. Verify rotation status: /opt/hsm-service/bin/hsm-admin rotation-status"
echo "  2. Test encryption: curl -k https://localhost:8443/health ..."
echo "  3. Check logs: sudo journalctl -u hsm-service -n 50"
```

**Установить права:**

```bash
sudo chown hsm:hsm /opt/hsm-service/scripts/restore.sh
sudo chmod u+x /opt/hsm-service/scripts/restore.sh
```

**Использование:**

```bash
# Список доступных бэкапов
ls -lh /var/backups/hsm-service/tokens-*.tar.gz

# Restore с указанием файлов
sudo /opt/hsm-service/scripts/restore.sh \
  /var/backups/hsm-service/tokens-20260124-020000.tar.gz \
  /var/backups/hsm-service/metadata-20260124-020000.yaml

# Или интерактивно (скрипт спросит подтверждение)
sudo /opt/hsm-service/scripts/restore.sh /path/to/tokens-YYYYMMDD-HHMMSS.tar.gz
```

**Что делает restore скрипт:**

1. ✓ Останавливает HSM service
2. ✓ Распаковывает tokens из tar.gz backup
3. ✓ Исправляет права доступа на tokens (критично!)
4. ✓ Восстанавливает metadata.yaml (если указан)
5. ✓ Запускает HSM service
6. ✓ Проверяет что сервис запустился
7. ✓ Проверяет что ключи загружены (list-kek)
8. ✓ **Запускает update-checksums** (важно!)
9. ✓ Выводит статус

### Важные замечания про restore

**⚠️ КРИТИЧНО: update-checksums**

После restore ОБЯЗАТЕЛЬНО нужно запустить:
```bash
HSM_PIN=$(grep '^HSM_PIN=' /etc/hsm-service/environment | cut -d= -f2-)
sudo -u hsm bash -c "HSM_PIN='$HSM_PIN' /opt/hsm-service/bin/hsm-admin update-checksums"
```

Это пересчитывает контрольные суммы ключей. Если этого не сделать, `hsm-admin` может выдать ошибку при ротации.

**⚠️ Проверка целостности**

```bash
# После restore проверить целостность
/opt/hsm-service/bin/hsm-admin list-kek
# Expected:
# Key: kek-exchange-key-v1 (Handle: 3, ID: 02, version: 1)
# Key: kek-2fa-v1 (Handle: 4, ID: 03, version: 1)

# Проверить статус ротации
/opt/hsm-service/bin/hsm-admin rotation-status
```

**⚠️ Права доступа на tokens**

После распаковки tar.gz **ОБЯЗАТЕЛЬНО** исправить права, потому что:
- tar распаковывает с правами, которые были в архиве
- Часто это root:root с неправильными пермиссиями
- Пользователь hsm не сможет получить доступ к ключам

```bash
# Проверить права (должны быть 600 на файлы, 700 на директории)
ls -la /var/lib/softhsm/tokens/
# -rw------- 1 hsm hsm ...

# Если права неправильные (например 644), исправить
sudo chmod 600 /var/lib/softhsm/tokens/*
sudo chmod 700 /var/lib/softhsm/tokens
```

### Disaster Recovery (сценарий полной потери данных)

**Если потеряны ВСЕ ключи (полный disaster):**

```bash
# 1. Остановить сервис
sudo systemctl stop hsm-service

# 2. Удалить поврежденный токен
sudo rm -rf /var/lib/softhsm/tokens/hsm-token/

# 3. Пересоздать новый токен
sudo softhsm2-util --init-token \
  --slot 0 \
  --label hsm-token \
  --so-pin 0000 \
  --pin 1234

# 4. Исправить права
sudo chown -R hsm:hsm /var/lib/softhsm/tokens/

# 5. Восстановить из backup (как выше)
sudo /opt/hsm-service/scripts/restore.sh /var/backups/hsm-service/tokens-*.tar.gz

# 6. Проверить что всё восстановилось
/opt/hsm-service/bin/hsm-admin rotation-status
```

**Если backup тоже потерян (полная потеря):**

⚠️ **КЛЮЧИ НЕ ВОССТАНОВИТЬ, ДАННЫЕ ПОТЕРЯНЫ!**

Это критическая ситуация. Нужно:
1. Задекларировать incident
2. Перегенерировать ВСЕ ключи
3. Re-encrypt ВСЕ данные новыми ключами
4. Обновить все зависимые системы
5. Провести security audit

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

### 2.1 Перезапуск Fail2ban

После создания конфигов нужно перезагрузить Fail2ban:

```bash
# Перезагрузить Fail2ban
sudo systemctl restart fail2ban

# Проверить статус
sudo systemctl status fail2ban

# Проверить что фильтр и jail загружены
sudo fail2ban-client status hsm-service

# Ожидаемый вывод:
# Status for the jail: hsm-service
# |- Filter
# |  |- Currently failed: 0
# |  |- Total failed: 0
# |  `- Journal matches: 0
# `- Actions
#    |- Currently banned: 0
#    |- Total banned: 0
#    `- Banned IP list:

# Если появляется ошибка "Jail does not exist", проверить конфиг
sudo fail2ban-client get-errors
```

### 3. SELinux/AppArmor (опционально)

Debian по умолчанию не использует SELinux, но можно настроить AppArmor.

**⚠️ Важно для Debian 13**: Debian 13 использует `systemd journal` вместо классического `syslog`, поэтому `aa-genprof` может выдать ошибку:
```
ERROR: Can't find system log "/var/log/syslog". Please check permissions.
```

**Решение - вариант 1 (быстро, рекомендуется):**

Включить логирование в syslog через rsyslog:

```bash
# Установить rsyslog
sudo apt install -y rsyslog

# Включить и запустить
sudo systemctl enable rsyslog
sudo systemctl start rsyslog

# После этого aa-genprof будет работать
sudo aa-genprof /opt/hsm-service
sudo aa-enforce /opt/hsm-service
```

**Решение - вариант 2 (минимально):**

Если не хотите устанавливать rsyslog, можно создать пустой файл syslog:

```bash
# Создать файл syslog (для совместимости с aa-genprof)
sudo touch /var/log/syslog
sudo chmod 640 /var/log/syslog

# Теперь aa-genprof должен работать
sudo aa-genprof /opt/hsm-service
sudo aa-enforce /opt/hsm-service
```

**Решение - вариант 3 (без AppArmor):**

AppArmor опционален для production. Если не планируете его использовать, просто пропустите этот шаг. systemd уже предоставляет Security hardening через ProtectSystem, ProtectHome и другие параметры в unit файле.

```bash
# Проверить текущие Security параметры systemd
systemctl show -p ProtectSystem hsm-service
systemctl show -p ProtectHome hsm-service
# systemd Security достаточно для большинства сценариев
```

**Проверка что AppArmor работает (если установлен):**

```bash
# Проверить статус AppArmor
sudo systemctl status apparmor

# Проверить профиль HSM service
sudo aa-status | grep hsm-service

# Проверить логи AppArmor в journal
sudo journalctl -u apparmor -f
```

---

## Нагрузочное тестирование перед внедрением

Перед использованием в production требуется провести stress тестирование. Это требует **временного отключения** некоторых систем безопасности и ограничений.

### Подготовка к stress тесту

**1. Остановить firewall (nftables)**

```bash
sudo systemctl stop nftables
sudo systemctl mask nftables  # Чтобы не запустился при reboot

# Проверка
sudo nft list ruleset
# (должно быть пусто или "No such file or directory")
```

**2. Остановить fail2ban**

```bash
sudo systemctl stop fail2ban
sudo systemctl mask fail2ban
```

**3. Отключить rate limiting в config.yaml**

```bash
sudo nano /etc/hsm-service/config.yaml
```

Изменить секцию `rate_limit`:
```yaml
rate_limit:
  requests_per_second: 999999  # Очень высокий лимит для тестирования
  burst: 999999
```

Перезагрузить сервис:
```bash
sudo systemctl restart hsm-service
```

**4. Увеличить kernel limits временно**

```bash
sudo sysctl -w net.core.somaxconn=32768
sudo sysctl -w net.ipv4.tcp_max_syn_backlog=32768
sudo sysctl -w net.netfilter.nf_conntrack_max=2097152
```

**5. Отключить logrotate (опционально)**

```bash
sudo systemctl stop logrotate
sudo systemctl mask logrotate
```

**6. Отключить systemd-journald rate limit (опционально)**

```bash
sudo systemctl stop systemd-journal-flush

# Или уменьшить уровень логирования в config.yaml:
# logging:
#   level: warn  # Вместо info
```

### Запуск stress тестов

**Убедиться что всё отключено:**

```bash
sudo systemctl status nftables    # должно быть masked/inactive
sudo systemctl status fail2ban    # должно быть masked/inactive
grep "requests_per_second" /etc/hsm-service/config.yaml  # должно быть 999999
```

**Запустить тесты (выбрать один вариант):**

```bash
# Переменные окружения для клиентского сертификата
export HSM_URL=https://localhost:8443
export CLIENT_CERT=/etc/hsm-service/pki/client/client1.crt
export CLIENT_KEY=/etc/hsm-service/pki/client/client1.key
```

**Вариант 1: Smoke Test (безопасный, ~1 минута)**

```bash
cd /opt/hsm-service
./tests/performance/smoke-test.sh

# Expected output:
# ✓ Health check passed
# ✓ Encryption test passed
# ✓ Decryption test passed
```

**Вариант 2: Quick Load Test (умеренная нагрузка, ~2 минуты)**

```bash
cd /opt/hsm-service
k6 run tests/performance/load-test-quick.js

# Expected: ~3500 запросов за 2 минуты
# P95 latency < 100ms
```

**Вариант 3: Full Stress Test (интенсивная нагрузка, ~22 минуты)**

```bash
cd /opt/hsm-service
./tests/performance/stress-test.sh incremental

# ⚠️ Внимание: долгий тест, создает значительную нагрузку
```

### Мониторинг во время теста

Запустить в отдельных окнах для наблюдения:

**Логи сервиса:**
```bash
sudo journalctl -u hsm-service -f
```

**Метрики системы:**
```bash
watch -n 1 'free -h && echo "---" && top -b -n1 | head -20'
```

**Активные соединения:**
```bash
watch -n 1 'netstat -tan | grep :8443 | wc -l'  # Количество соединений к HSM
```

**Prometheus метрики (если доступны):**
```bash
curl -k https://localhost:8443/metrics \
  --cert /etc/hsm-service/pki/client/client1.crt \
  --key /etc/hsm-service/pki/client/client1.key \
  --cacert /etc/hsm-service/pki/ca/ca.crt
```

### Восстановление после stress теста

**Восстановить firewall:**

```bash
sudo systemctl unmask nftables
sudo systemctl start nftables
```

**Восстановить fail2ban:**

```bash
sudo systemctl unmask fail2ban
sudo systemctl start fail2ban
```

**Восстановить rate limiting:**

```bash
sudo nano /etc/hsm-service/config.yaml
```

Вернуть production значения:
```yaml
rate_limit:
  requests_per_second: 50000  # Исходное значение
  burst: 5000
```

Перезагрузить сервис:
```bash
sudo systemctl restart hsm-service
```

**Восстановить kernel limits:**

```bash
sudo sysctl -p /etc/sysctl.d/99-hsm-service.conf
```

**Восстановить logrotate:**

```bash
sudo systemctl unmask logrotate
sudo systemctl start logrotate
```

**Проверить что всё восстановилось:**

```bash
sudo systemctl status hsm-service
sudo systemctl status nftables
sudo systemctl status fail2ban

# Verify rate limit в config
grep "requests_per_second" /etc/hsm-service/config.yaml
```

### Критические моменты при stress тестировании

⚠️ **Обязательно отключить эти 3 вещи:**

1. **Firewall (nftables)** — иначе будет dropping пакеты при высокой нагрузке
   ```bash
   sudo systemctl stop nftables
   ```

2. **Rate limiting в config.yaml** — иначе тест будет throttled
   ```yaml
   requests_per_second: 999999
   ```

3. **fail2ban** — иначе может забанить сам тестовый клиент
   ```bash
   sudo systemctl stop fail2ban
   ```

### Рекомендуемый процесс тестирования

1. **Перед каждым тестом:** Уведомите команду
2. **Первый раз:** smoke → quick (в нерабочее время)
3. **Регулярно:** smoke test (еженедельно для контроля деградации)
4. **Периодически:** full load (ежеквартально, в maintenance window)
5. **Редко:** stress test (для capacity planning, в maintenance window)

**Документация:** Полная информация о performance тестировании см. в [tests/performance/README.md](tests/performance/README.md)

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

### Problem: hsm-admin ошибка "no such file or directory: config.yaml"

Если при запуске `hsm-admin` получаете ошибку:
```
Failed to update checksums: failed to load config: read config file: open config.yaml: no such file or directory
```

**Решение:** `hsm-admin` автоматически ищет конфиг в следующем порядке:
1. Переменная окружения `CONFIG_PATH`
2. Текущая директория (`./config.yaml`)
3. `/etc/hsm-service/config.yaml`

Если конфиг в `/etc/hsm-service/config.yaml`, то команда должна работать без флагов:

```bash
# Способ 1: Автоматическое обнаружение (конфиг в /etc/hsm-service/)
/opt/hsm-service/bin/hsm-admin update-checksums
/opt/hsm-service/bin/hsm-admin rotation-status
/opt/hsm-service/bin/hsm-admin list-kek

# Способ 2: Явно указать флагом -config (перед командой)
/opt/hsm-service/bin/hsm-admin -config /etc/hsm-service/config.yaml update-checksums
/opt/hsm-service/bin/hsm-admin -config /etc/hsm-service/config.yaml rotation-status

# Способ 3: Короткая форма -c
/opt/hsm-service/bin/hsm-admin -c /etc/hsm-service/config.yaml update-checksums

# Способ 4: Через переменную окружения CONFIG_PATH
export CONFIG_PATH=/etc/hsm-service/config.yaml
/opt/hsm-service/bin/hsm-admin update-checksums
/opt/hsm-service/bin/hsm-admin rotation-status
```

**Важно:** Флаг конфига должен быть указан ПЕРЕД командой:
```bash
# ✓ Правильно
hsm-admin -config /path/to/config.yaml update-checksums

# ✗ Неправильно
hsm-admin update-checksums -config /path/to/config.yaml
```

### Problem: Permission denied на tokens или "Could not load the SoftHSM configuration"

Если пользователь `hsm` получает ошибку при `softhsm2-util --show-slots`:
```
ERROR: Could not load the SoftHSM configuration.
ERROR: Please verify that the SoftHSM configuration is correct.
```

**Решение:**

```bash
# 1. Проверить права на директорию токенов
ls -la /var/lib/softhsm/
# Должно быть: drwx------ hsm hsm

# 2. Исправить владельца и права
sudo chown -R hsm:hsm /var/lib/softhsm/tokens/
sudo chmod 700 /var/lib/softhsm/tokens/
sudo find /var/lib/softhsm/tokens/ -type f -exec chmod 600 {} \;

# 3. Если конфиг SoftHSM не доступен
sudo cat /etc/softhsm/softhsm2.conf
ls -la /etc/softhsm/softhsm2.conf

# 4. Если нужен доступ через переменную окружения
export SOFTHSM2_CONF=/etc/softhsm/softhsm2.conf
sudo -u hsm sh -c 'SOFTHSM2_CONF=/etc/softhsm/softhsm2.conf softhsm2-util --show-slots'

# 5. Проверить что всё работает
sudo -u hsm softhsm2-util --show-slots
# Expected:
# Slot 0
#     Slot info:
#         Description      : SoftHSM slot ID 0x0
#         Manufacturer ID  : SoftHSM project
#         Hardware version : 2.6
#         Firmware version : 2.6
#         Serial number    : gaSJbNtm
#         Initialized      : yes
#     Token info:
#         Manufacturer ID  : SoftHSM project
#         Model            : SoftHSM v2
#         Hardware version : 2.6
#         Firmware version : 2.6
#         Serial number    : 123456789ABCDEF0
#         Initialized      : yes
#         User PIN init.   : yes
#         Label            : hsm-token
```

**Ключевой момент:** Инициализация токена должна быть от `root`, но после этого нужно **обязательно** исправить права так, чтобы пользователь `hsm` мог читать файлы токена.

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

### Problem: PIN видим в процессах (утечка безопасности)

Если PIN хранится в systemd unit файле, его можно увидеть через `ps` или `systemctl show`:

```bash
# ПЛОХО - PIN видим!
systemctl show hsm-service | grep HSM_PIN
# Environment=HSM_PIN=1234 ...

# Или через процесс
ps aux | grep hsm-service
# hsm  1234  ...  /opt/hsm-service/bin/hsm-service
# Переменные окружения видны в /proc/1234/environ
```

**Решение:**

```bash
# 1. Никогда не используйте Environment="HSM_PIN=..." в systemd файле!
# 2. Всегда используйте отдельный файл с ограниченными правами:
sudo nano /etc/hsm-service/environment
# HSM_PIN=your-strong-pin-here

# 3. Установите правильные права:
sudo chown root:hsm /etc/hsm-service/environment
sudo chmod 640 /etc/hsm-service/environment  # Только root и группа hsm

# 4. В systemd файле используйте:
# EnvironmentFile=/etc/hsm-service/environment  # Не EnvironmentFile=-

# 5. Для production рекомендуется использовать KMS/Vault:
#    - HashiCorp Vault (локально)
#    - AWS Secrets Manager
#    - Azure Key Vault
#    - Kubernetes Secrets (если на k8s)
```

**Проверка что PIN не утекает:**

```bash
# PIN НЕ должен быть видим
systemctl show hsm-service | grep HSM_PIN
# (пусто - правильно!)

# PIN НЕ должен быть в конфиге
cat /etc/systemd/system/hsm-service.service | grep HSM_PIN
# (пусто - правильно!)

# PIN только в защищённом файле
ls -la /etc/hsm-service/environment
# -rw-r----- 1 root hsm 256 ...
```

### Problem: Production PIN Management

Для серьезной production рекомендуются эти методы:

**1. HashiCorp Vault (самый надежный):**

```bash
# Установить Vault на отдельном хосте/контейнере
# Записать PIN в Vault:
vault kv put secret/hsm-service hsm_pin="your-pin"

# В systemd скрипте, получить PIN из Vault:
ExecStartPre=/opt/hsm-service/scripts/get-pin-from-vault.sh
EnvironmentFile=/tmp/hsm-pin.env
```

**2. AWS Secrets Manager (если на AWS):**

```bash
# Сохранить PIN:
aws secretsmanager create-secret \
  --name hsm-service/pin \
  --secret-string "your-pin"

# В systemd скрипте:
ExecStartPre=/opt/hsm-service/scripts/get-pin-from-aws.sh
EnvironmentFile=/tmp/hsm-pin.env
```

**3. Kubernetes Secrets (если на k8s):**

```yaml
apiVersion: v1
kind: Secret
metadata:
  name: hsm-service-pin
type: Opaque
stringData:
  HSM_PIN: "your-pin"
---
apiVersion: v1
kind: Pod
metadata:
  name: hsm-service
spec:
  containers:
  - name: hsm-service
    env:
    - name: HSM_PIN
      valueFrom:
        secretKeyRef:
          name: hsm-service-pin
          key: HSM_PIN
```

**4. Локальный защищённый файл (минимум для production):**

```bash
# /etc/hsm-service/environment
# Права: 640 (root:hsm)
# Содержимое никогда не выводить, не логировать, не коммитить

HSM_PIN=your-strong-random-pin-generated-with-openssl-rand
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

- [ ] **БЕЗОПАСНОСТЬ PIN**: HSM_PIN в файле `/etc/hsm-service/environment` с правами 640, НЕ в systemd unit!
- [ ] PIN - криптографически сильный (openssl rand -hex 16), НЕ "1234"
- [ ] Рассмотрен KMS/Vault для управления PIN'ами (HashiCorp Vault, AWS Secrets Manager, Azure Key Vault)
- [ ] Изменены default SO_PIN и HSM_PIN для токена
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
- [ ] Проверено что PIN не видим в процессах: `systemctl show hsm-service | grep PIN`
- [ ] Проверено что конфиг-файл не попал в git: `.gitignore` содержит `/etc/hsm-service/`

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
