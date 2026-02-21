# 🔧 HSM Service - Troubleshooting Guide

> **Для DevOps/Support**: Решение типичных проблем, debug процедуры, incident response

## Оглавление

- [Быстрая диагностика](#быстрая-диагностика)
- [Проблемы запуска](#проблемы-запуска)
- [Проблемы с сертификатами](#проблемы-с-сертификатами)
- [Проблемы с HSM](#проблемы-с-hsm)
- [Проблемы с производительностью](#проблемы-с-производительностью)
- [Проблемы с ACL](#проблемы-с-acl)
- [Проблемы с ротацией ключей](#проблемы-с-ротацией-ключей)
- [Network проблемы](#network-проблемы)
- [Debug процедуры](#debug-процедуры)
- [Incident Response](#incident-response)

---

## Быстрая диагностика

### Health Check Script

```bash
#!/bin/bash
# hsm-health-check.sh

echo "=== HSM Service Health Check ==="
echo

# 1. Check service status
echo "1. Service Status:"
systemctl is-active hsm-service
systemctl status hsm-service --no-pager | head -10
echo

# 2. Check process
echo "2. Process:"
ps aux | grep hsm-service | grep -v grep
echo

# 3. Check port
echo "3. Port 8443:"
ss -tlnp | grep 8443
echo

# 4. Check certificates
echo "4. Certificates:"
openssl x509 -in /etc/hsm-service/pki/server/server.crt -noout -dates
echo

# 5. Check HSM token
echo "5. HSM Token:"
softhsm2-util --show-slots | grep -A 5 "hsm-token"
echo

# 6. Test API
echo "6. Health Endpoint:"
curl -k -s https://localhost:8443/health \
  --cert /etc/hsm-service/pki/client/client1.crt \
  --key /etc/hsm-service/pki/client/client1.key \
  --cacert /etc/hsm-service/pki/ca/ca.crt | jq .
echo

# 7. Check logs
echo "7. Recent Errors:"
journalctl -u hsm-service --since "5 minutes ago" | grep -i error | tail -5
echo

# 7.1 Access log (file)
tail -n 20 /var/log/hsm-service/access.log

# 8. Check resources
echo "8. Resources:"
echo "Memory: $(free -h | grep Mem | awk '{print $3 "/" $2}')"
echo "CPU: $(top -bn1 | grep "Cpu(s)" | awk '{print $2}')%"
echo "Disk: $(df -h /var/lib/softhsm | tail -1 | awk '{print $5}')"
echo

echo "=== Health Check Complete ==="
```

Запуск:
```bash
chmod +x hsm-health-check.sh
./hsm-health-check.sh
```

---

## Проблемы запуска

### Problem 1: Service не запускается

**Симптомы**:
```bash
$ systemctl status hsm-service
● hsm-service.service - HSM Service
   Active: failed (Result: exit-code)
```

**Диагностика**:
```bash
# Проверить логи
journalctl -u hsm-service -n 50 --no-pager

# Попробовать запустить вручную
sudo -u hsm /opt/hsm-service/hsm-service

# Проверить конфигурацию
cat /etc/hsm-service/config.yaml
```

**Типичные причины**:

#### 1.1 Config файл не найден

**Ошибка**:
```
Error: config.yaml not found
```

**Решение**:
```bash
# Проверить путь
ls -la /etc/hsm-service/config.yaml

# Создать если отсутствует
sudo cp /opt/hsm-service/config.yaml /etc/hsm-service/

# Проверить права
sudo chown hsm:hsm /etc/hsm-service/config.yaml
```

#### 1.2 Invalid YAML syntax

**Ошибка**:
```
Error: yaml: unmarshal error
```

**Решение**:
```bash
# Проверить синтаксис
yamllint /etc/hsm-service/config.yaml

# Или через Python
python3 -c "import yaml; yaml.safe_load(open('/etc/hsm-service/config.yaml'))"
```

#### 1.3 HSM_PIN не установлен

**Ошибка**:
```
Error: HSM PIN not set
```

**Решение**:
```bash
# Добавить в environment file
echo "HSM_PIN=1234" | sudo tee -a /etc/hsm-service/environment

# Или в systemd unit
sudo systemctl edit hsm-service
# Добавить: Environment="HSM_PIN=1234"

sudo systemctl daemon-reload
sudo systemctl restart hsm-service
```

#### 1.4 Недоступна директория логов

**Ошибка**:
```
Log path validation failed
```

**Решение**:
```bash
# Проверить директорию логов
ls -la /var/log/hsm-service

# Исправить владельца и права
sudo chown -R hsm:hsm /var/log/hsm-service
sudo chmod 750 /var/log/hsm-service
```

### Problem 2: Port already in use

**Симптомы**:
```
Error: bind: address already in use
```

**Диагностика**:
```bash
# Проверить кто использует порт 8443
sudo ss -tlnp | grep 8443
sudo lsof -i :8443
```

**Решение**:
```bash
# Остановить конфликтующий процесс
sudo kill <PID>

# Или изменить порт в config.yaml
server:
  port: 8444
  timeouts:
    read: 10s
    write: 30s
    idle: 120s
    read_header: 5s
    shutdown_grace: 15s
  limits:
    max_header_bytes: 1048576
```

### Problem 3: Permission denied

**Симптомы**:
```
Error: permission denied: /var/lib/softhsm/tokens
```

**Диагностика**:
```bash
# Проверить права
ls -la /var/lib/softhsm/tokens
ls -la /var/log/hsm-service
```

**Решение**:
```bash
# Исправить ownership
sudo chown -R hsm:hsm /var/lib/softhsm/tokens
sudo chown -R hsm:hsm /var/log/hsm-service

# Исправить permissions
sudo chmod 700 /var/lib/softhsm/tokens
sudo chmod 750 /var/log/hsm-service
```

---

## Проблемы с сертификатами

### Problem 1: Certificate expired

**Симптомы**:
```
Error: x509: certificate has expired
```

**Диагностика**:
```bash
# Проверить срок действия
openssl x509 -in /etc/hsm-service/pki/server/server.crt -noout -dates

# Вывод:
# notBefore=Jan  1 00:00:00 2024 GMT
# notAfter=Jan  1 00:00:00 2025 GMT  # <- Expired!
```

**Решение**:
```bash
# Перевыпустить сертификат
cd /opt/hsm-service/pki
./scripts/issue-server-cert.sh hsm-service.example.com

# Или с SAN
./scripts/issue-server-cert.sh hsm-service.example.com "DNS:hsm.example.com,IP:10.0.0.1"

# Перезапустить сервис
sudo systemctl restart hsm-service
```

### Problem 2: Unknown CA

**Симптомы**:
```
Error: x509: certificate signed by unknown authority
```

**Диагностика**:
```bash
# Проверить CA certificate
openssl x509 -in /etc/hsm-service/pki/ca/ca.crt -noout -text

# Проверить цепочку
openssl verify -CAfile /etc/hsm-service/pki/ca/ca.crt \
  /etc/hsm-service/pki/server/server.crt
```

**Решение**:
```bash
# Убедиться что CA правильный
# Server cert должен быть подписан этим CA

# Проверить config.yaml
cat /etc/hsm-service/config.yaml | grep ca_path
# Должен указывать на правильный ca.crt
```

### Problem 3: Certificate revoked

**Симптомы**:
```json
{"error": "certificate revoked", "client_cn": "old-client"}
```

**Диагностика**:
```bash
# Проверить revoked list
cat /etc/hsm-service/pki/revoked.yaml
```

**Решение**:
```bash
# Если ошибочно отозван - удалить из списка
sudo nano /etc/hsm-service/pki/revoked.yaml

# Удалить CN из списка revoked:
# revoked:
#   - "old-client"  # <- удалить эту строку

# Сохранить файл - auto-reload произойдет через 30 секунд
# Или перезапустить сервис для немедленного применения
sudo systemctl restart hsm-service
```

### Problem 4: Certificate CN mismatch

**Симптомы**:
```
Error: x509: certificate is valid for server1.example.com, not server2.example.com
```

**Диагностика**:
```bash
# Проверить CN и SAN
openssl x509 -in /etc/hsm-service/pki/server/server.crt -noout -text | grep -A 5 "Subject:"
```

**Решение**:
```bash
# Перевыпустить с правильным CN/SAN
cd /opt/hsm-service/pki
./scripts/issue-server-cert.sh correct-hostname.example.com
```

---

## Проблемы с HSM

### Problem 1: Token not found

**Симптомы**:
```
Error: CKR_TOKEN_NOT_PRESENT: Token not present
```

**Диагностика**:
```bash
# Проверить доступные токены
softhsm2-util --show-slots

# Проверить конфигурацию SoftHSM
cat /etc/softhsm2.conf

# Проверить директорию токенов
ls -la /var/lib/softhsm/tokens/
```

**Решение**:
```bash
# Инициализировать токен
softhsm2-util --init-token \
  --slot 0 \
  --label "hsm-token" \
  --so-pin 5678 \
  --pin 1234

# Проверить
softhsm2-util --show-slots | grep "hsm-token"
```

### Problem 2: Wrong PIN

**Симптомы**:
```
Error: CKR_PIN_INCORRECT: PIN incorrect
```

**Диагностика**:
```bash
# Проверить environment variable
echo $HSM_PIN

# Проверить systemd environment
systemctl show hsm-service | grep HSM_PIN
```

**Решение**:
```bash
# Установить правильный PIN
sudo nano /etc/hsm-service/environment
# HSM_PIN=correct-pin

sudo systemctl daemon-reload
sudo systemctl restart hsm-service
```

### Problem 3: Session locked

**Симптомы**:
```
Error: CKR_SESSION_HANDLE_INVALID: Session handle invalid
```

**Решение**:
```bash
# Перезапустить сервис
sudo systemctl restart hsm-service

# Если не помогает - удалить lock files
sudo rm -f /var/lib/softhsm/tokens/*.lock

sudo systemctl start hsm-service
```

### Problem 4: Key not found

**Симптомы**:
```json
{"error": "key not found", "context": "exchange-key"}
```

**Диагностика**:
```bash
# Проверить существующие ключи
sudo -u hsm /opt/hsm-service/hsm-admin list-kek

# Проверить metadata
cat /var/lib/hsm-service/metadata.yaml
```

**Решение**:
```bash
# Создать недостающий ключ
export HSM_PIN=1234
./hsm-admin create-kek --label kek-exchange-key-v1 --context exchange-key

# Проверить
./hsm-admin list-kek
```

---

## Проблемы с производительностью

### Problem 1: High latency

**Симптомы**:
- Slow response times (> 500ms)
- Request timeouts

**Диагностика**:
```bash
# Проверить метрики
curl -k https://localhost:8443/metrics ... | grep duration

# Проверить CPU/Memory
htop
top -p $(pgrep hsm-service)

# Проверить I/O
iotop -p $(pgrep hsm-service)

# Проверить логи
journalctl -u hsm-service | jq 'select(.duration_ms > 100)'
```

**Решение**:

#### 1.1 High CPU
```bash
# Проверить количество goroutines
curl -k https://localhost:8443/metrics ... | grep hsm_goroutines

# Если > 10000 - возможна goroutine leak
# Перезапустить сервис
sudo systemctl restart hsm-service
```

#### 1.2 Slow HSM operations
```bash
# Проверить HSM latency
curl -k https://localhost:8443/metrics ... | grep hsm_operation_duration

# Если проблема в HSM - рассмотреть hardware HSM
```

#### 1.3 Network latency
```bash
# Проверить network
ping -c 10 hsm-service.example.com

# Проверить TLS handshake
time openssl s_client -connect hsm-service:8443 < /dev/null
```

### Problem 2: Memory leak

**Симптомы**:
- Memory usage постоянно растет
- OOM kills

**Диагностика**:
```bash
# Проверить память
ps aux | grep hsm-service
curl -k https://localhost:8443/metrics ... | grep hsm_memory_usage_bytes

# Мониторинг в реальном времени
watch -n 5 'ps aux | grep hsm-service'
```

**Решение**:
```bash
# Перезапуск (временное решение)
sudo systemctl restart hsm-service

# Долгосрочное решение - отчет о баге
# Собрать memory profile:
curl http://localhost:6060/debug/pprof/heap > heap.prof

# (требует включить pprof в коде)
```

### Problem 3: Too many connections

**Симптомы**:
```
Error: too many open files
```

**Диагностика**:
```bash
# Проверить limits
cat /proc/$(pgrep hsm-service)/limits | grep "Max open files"

# Проверить текущие connections
ss -tn | grep 8443 | wc -l
```

**Решение**:
```bash
# Увеличить limits в systemd
sudo systemctl edit hsm-service

# Добавить:
[Service]
LimitNOFILE=65536

sudo systemctl daemon-reload
sudo systemctl restart hsm-service
```

---

## Проблемы с ACL

### Problem 1: Access denied

**Симптомы**:
```json
{"error": "access denied", "client_cn": "my-client", "context": "exchange-key"}
```

**Диагностика**:
```bash
# 1. Проверить ACL mappings
cat /etc/hsm-service/config.yaml | grep -A 20 "acl:"

# 2. Проверить revoked list
cat /etc/hsm-service/pki/revoked.yaml

# 3. Проверить CN сертификата клиента
openssl x509 -in client.crt -noout -subject
# Subject: CN=my-client
```

**Решение**:

#### 1.1 Client не в mapping
```yaml
# Добавить в config.yaml
acl:
  mappings:
    MyClientGroup:        # Группа клиента
      - exchange-key      # Доступ к этому контексту
```

```bash
# Добавить CN в inventory.yaml
sudo nano /opt/hsm-service/pki/inventory.yaml

clients:
  - cn: "my-client"
    group: "MyClientGroup"
    issued_at: "2024-01-15"

# Перезапустить сервис
sudo systemctl restart hsm-service
```

#### 1.2 Client revoked
```bash
# Проверить
cat /etc/hsm-service/pki/revoked.yaml

# Если ошибочно отозван - удалить
sudo nano /etc/hsm-service/pki/revoked.yaml
# Удалить CN из списка

# Auto-reload произойдет через 30 сек
# Или перезапустить
sudo systemctl restart hsm-service
```

### Problem 2: ACL reload errors

**Симптомы**:
```
WARN: Failed to reload revoked list: yaml: unmarshal error
```

**Диагностика**:
```bash
# Проверить синтаксис
yamllint /etc/hsm-service/pki/revoked.yaml

# Проверить содержимое
cat /etc/hsm-service/pki/revoked.yaml
```

**Типичные ошибки**:

```yaml
# WRONG: Empty CN
revoked:
  - ""  # <- Пустая строка

# WRONG: Дубликаты
revoked:
  - "client1"
  - "client1"  # <- Дубликат

# CORRECT:
revoked:
  - "client1"
  - "client2"
```

**Решение**:
```bash
# Исправить файл
sudo nano /etc/hsm-service/pki/revoked.yaml

# Проверить
yamllint /etc/hsm-service/pki/revoked.yaml

# Wait for auto-reload (30 sec)
# Или перезапустить
sudo systemctl restart hsm-service
```

---

## Проблемы с ротацией ключей

### Problem 1: Rotation failed

**Симптомы**:
```
ERROR: Failed to rotate key: context=exchange-key error=...
```

**Диагностика**:
```bash
# Проверить rotation status
./hsm-admin rotation-status

# Проверить HSM token
softhsm2-util --show-slots

# Проверить metadata
cat /var/lib/hsm-service/metadata.yaml
```

**Решение**:
```bash
# Попробовать еще раз
export HSM_PIN=1234
./hsm-admin rotate --context exchange-key

# Если не помогает - проверить HSM PIN
echo $HSM_PIN

# Проверить права на metadata.yaml
ls -la /var/lib/hsm-service/metadata.yaml
sudo chown hsm:hsm /var/lib/hsm-service/metadata.yaml
```

### Problem 2: Old versions not cleaned up

**Симптомы**:
- Много старых KEK versions
- Большой размер HSM token

**Диагностика**:
```bash
# Проверить количество версий
./hsm-admin list-kek

# Проверить конфигурацию
cat /etc/hsm-service/config.yaml | grep -A 3 "hsm:"
# max_versions: 3
# cleanup_after_days: 30
```

**Решение**:
```bash
# Manual cleanup
export HSM_PIN=1234
./hsm-admin cleanup-old-versions --context exchange-key

# Проверить
./hsm-admin list-kek
```

---

## Network Проблемы

### Problem 1: Connection refused

**Симптомы**:
```
curl: (7) Failed to connect to hsm-service port 8443: Connection refused
```

**Диагностика**:
```bash
# 1. Проверить что сервис запущен
systemctl status hsm-service

# 2. Проверить что порт слушается
ss -tlnp | grep 8443

# 3. Проверить firewall
sudo nft list ruleset | grep 8443
```

**Решение**:

#### 1.1 Service down
```bash
sudo systemctl start hsm-service
```

#### 1.2 Firewall блокирует
```bash
# Проверить правила nftables
sudo nft list ruleset

# Добавить правило
sudo nft add rule inet filter input tcp dport 8443 accept

# Или отредактировать /etc/nftables.conf
```

### Problem 2: Timeout

**Симптомы**:
```
curl: (28) Connection timed out
```

**Диагностика**:
```bash
# Проверить сетевую связность
ping hsm-service.example.com

# Проверить traceroute
traceroute hsm-service.example.com

# Проверить порт
telnet hsm-service.example.com 8443
nc -zv hsm-service.example.com 8443
```

**Решение**:
- Проверить network routing
- Проверить firewall rules на всем пути
- Проверить security groups (если cloud)

### Problem 3: TLS handshake failure

**Симптомы**:
```
Error: tls: handshake failure
```

**Диагностика**:
```bash
# Подробная информация о TLS handshake
openssl s_client -connect hsm-service:8443 -showcerts \
  -cert client.crt \
  -key client.key \
  -CAfile ca.crt
```

**Типичные причины**:
- Wrong CA
- Expired certificate
- Certificate CN mismatch
- Wrong client certificate

---

## Debug Процедуры

### Enable Debug Logging

```yaml
# config.yaml
logging:
  level: debug  # было: info
  format: json
```

```bash
sudo systemctl restart hsm-service
journalctl -u hsm-service -f
```

### Packet Capture

```bash
# Capture TLS traffic на порту 8443
sudo tcpdump -i any -s 0 -w /tmp/hsm-traffic.pcap port 8443

# Анализ в Wireshark
wireshark /tmp/hsm-traffic.pcap
```

### Strace

```bash
# Trace system calls
sudo strace -p $(pgrep hsm-service) -f -t -e trace=network,file

# Trace file operations
sudo strace -p $(pgrep hsm-service) -e trace=open,read,write
```

### Core Dump

Если сервис падает:

```bash
# Enable core dumps
sudo systemctl edit hsm-service

[Service]
LimitCORE=infinity

sudo systemctl daemon-reload

# После crash
ls -la /var/lib/systemd/coredump/

# Analyze with gdb (если скомпилировано с debug symbols)
gdb /opt/hsm-service/hsm-service /path/to/core
```

---

## Incident Response

### Severity Levels

#### SEV-1 (Critical): Service Down

**Immediate actions**:
1. Check service status: `systemctl status hsm-service`
2. Check recent logs: `journalctl -u hsm-service -n 100`
3. Attempt restart: `sudo systemctl restart hsm-service`
4. If fails - rollback to previous version
5. Notify oncall team

**Investigation**:
- Collect logs
- Check metrics (CPU, memory, disk)
- Check for config changes
- Review recent deployments

#### SEV-2 (High): Degraded Performance

**Immediate actions**:
1. Check metrics: latency, error rate, throughput
2. Check resources: CPU, memory, disk I/O
3. Check for unusual traffic patterns
4. Consider scaling if needed

**Investigation**:
- Review slow query logs
- Check HSM performance
- Profile application (if possible)

#### SEV-3 (Medium): Isolated Errors

**Immediate actions**:
1. Identify affected clients
2. Check ACL configuration
3. Review certificate status
4. Monitor error rate

**Investigation**:
- Analyze error logs
- Check client certificates
- Review ACL mappings

### Incident Communication Template

```
Subject: [SEV-X] HSM Service Incident - <brief description>

SUMMARY:
HSM Service is experiencing <issue description>

IMPACT:
- Affected endpoints: <list>
- Affected clients: <list>
- Error rate: X%
- Started at: <timestamp>

STATUS: <Investigating/Identified/Monitoring/Resolved>

ACTIONS TAKEN:
1. <action 1>
2. <action 2>

NEXT STEPS:
- <step 1>
- <step 2>

ETA: <time>

ONCALL: <name>
```

---

## Полезные команды

### Quick Checks

```bash
# Service status
systemctl status hsm-service

# Recent logs
journalctl -u hsm-service -n 50 --no-pager

# Test health endpoint
curl -k https://localhost:8443/health --cert client.crt --key client.key --cacert ca.crt

# Check metrics
curl -k https://localhost:8443/metrics --cert client.crt --key client.key --cacert ca.crt

# List KEKs
./hsm-admin list-kek

# Rotation status
./hsm-admin rotation-status
```

### Log Analysis

```bash
# Errors only
journalctl -u hsm-service -p err

# Last hour
journalctl -u hsm-service --since "1 hour ago"

# Follow logs
journalctl -u hsm-service -f

# JSON parsing
journalctl -u hsm-service -o json-pretty | jq 'select(.level=="error")'

# Access denied
journalctl -u hsm-service | grep "access denied"

# Top clients
journalctl -u hsm-service | jq -r '.client_cn' | sort | uniq -c | sort -rn
```

---

## Escalation Path

1. **L1 Support**: Basic checks, restarts, known issues
2. **L2 DevOps**: Configuration, deployment, infrastructure
3. **L3 Developers**: Code bugs, performance issues
4. **Vendor Support**: HSM hardware/firmware issues

---

## Additional Resources

- [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) - Production deployment
- [BUILD.md](BUILD.md) - Build instructions
- [PKI_SETUP.md](PKI_SETUP.md) - PKI setup guide
- [API.md](API.md) - API documentation
- [MONITORING.md](MONITORING.md) - Monitoring and alerting
- [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - Security guidelines
- [BACKUP_RESTORE.md](BACKUP_RESTORE.md) - Backup и disaster recovery
- [KEY_ROTATION.md](KEY_ROTATION.md) - Ротация ключей
