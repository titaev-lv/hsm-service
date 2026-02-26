# 📊 HSM Service - Мониторинг и Алерты

> **Для DevOps/SRE**: Полное руководство по мониторингу HSM Service с Prometheus, Grafana и настройкой алертов

## Оглавление

- [Обзор метрик](#обзор-метрик)
- [Prometheus setup](#prometheus-setup)
- [Grafana dashboards](#grafana-dashboards)
- [Alerting rules](#alerting-rules)
- [Логирование](#логирование)
- [Performance monitoring](#performance-monitoring)
- [Troubleshooting](#troubleshooting)

---

## Обзор метрик

HSM Service экспортирует метрики через endpoint `/metrics` (требуется mTLS).

### 8 групп метрик

#### 1. Request Metrics (Запросы)

| Метрика | Тип | Описание |
|---------|-----|----------|
| `hsm_requests_total` | Counter | Общее количество запросов |
| `hsm_request_duration_seconds` | Histogram | Время обработки запросов |
| `hsm_requests_in_flight` | Gauge | Количество текущих запросов |

**Labels**:
- `method` - HTTP метод (POST, GET)
- `endpoint` - путь (/encrypt, /decrypt, /health)
- `status_code` - HTTP код (200, 400, 500)

**Пример**:
```promql
# RPS для /encrypt
rate(hsm_requests_total{endpoint="/encrypt"}[1m])

# P95 latency
histogram_quantile(0.95, rate(hsm_request_duration_seconds_bucket[5m]))

# Текущая нагрузка
hsm_requests_in_flight
```

#### 2. Error Metrics (Ошибки)

| Метрика | Тип | Описание |
|---------|-----|----------|
| `hsm_errors_total` | Counter | Количество ошибок |

**Labels**:
- `type` - тип ошибки (hsm_error, validation_error, internal_error)
- `endpoint` - путь

**Пример**:
```promql
# Error rate (ошибки в секунду)
rate(hsm_errors_total[5m])

# Процент ошибок
rate(hsm_errors_total[5m]) / rate(hsm_requests_total[5m]) * 100
```

#### 3. ACL Metrics (Access Control)

| Метрика | Тип | Описание |
|---------|-----|----------|
| `hsm_acl_checks_total` | Counter | Количество ACL проверок |
| `hsm_acl_denials_total` | Counter | Количество отказов доступа |
| `hsm_acl_reload_total` | Counter | Количество перезагрузок revoked.yaml |
| `hsm_acl_reload_errors_total` | Counter | Ошибки при перезагрузке |

**Labels**:
- `client_cn` - Common Name клиента
- `context` - контекст (exchange-key, 2fa)
- `reason` - причина отказа (revoked, not_authorized)

**Пример**:
```promql
# ACL denials rate
rate(hsm_acl_denials_total[5m])

# Top blocked clients
topk(10, sum by (client_cn) (hsm_acl_denials_total))

# Reload errors
increase(hsm_acl_reload_errors_total[1h])
```

#### 4. Rate Limit Metrics

| Метрика | Тип | Описание |
|---------|-----|----------|
| `hsm_rate_limit_exceeded_total` | Counter | Превышения rate limit |

**Labels**:
- `client_cn` - клиент
- `endpoint` - путь

**Пример**:
```promql
# Rate limit abuse
rate(hsm_rate_limit_exceeded_total[5m])

# Самые агрессивные клиенты
topk(5, sum by (client_cn) (hsm_rate_limit_exceeded_total))
```

#### 5. HSM Metrics (Hardware Security Module)

| Метрика | Тип | Описание |
|---------|-----|----------|
| `hsm_operations_total` | Counter | Количество HSM операций |
| `hsm_operation_duration_seconds` | Histogram | Время HSM операций |
| `hsm_active_keys` | Gauge | Количество активных KEK |

**Labels**:
- `operation` - тип операции (encrypt, decrypt, create_key)
- `context` - контекст ключа

**Пример**:
```promql
# HSM operations per second
rate(hsm_operations_total[1m])

# HSM latency P99
histogram_quantile(0.99, rate(hsm_operation_duration_seconds_bucket[5m]))

# Количество активных ключей
hsm_active_keys
```

#### 6. Rotation Metrics (Ротация ключей)

| Метрика | Тип | Описание |
|---------|-----|----------|
| `hsm_rotation_total` | Counter | Количество ротаций |
| `hsm_rotation_errors_total` | Counter | Ошибки ротации |
| `hsm_key_age_seconds` | Gauge | Возраст ключа (секунды) |

**Labels**:
- `context` - контекст ключа
- `version` - версия ключа

**Пример**:
```promql
# Последняя ротация
time() - max(hsm_key_age_seconds)

# Rotation errors
increase(hsm_rotation_errors_total[24h])
```

#### 7. System Metrics (Система)

| Метрика | Тип | Описание |
|---------|-----|----------|
| `hsm_uptime_seconds` | Gauge | Время работы сервиса |
| `hsm_goroutines` | Gauge | Количество goroutines |
| `hsm_memory_usage_bytes` | Gauge | Использование памяти |

**Пример**:
```promql
# Uptime (дни)
hsm_uptime_seconds / 86400

# Goroutine leak detection
rate(hsm_goroutines[5m]) > 0
```

#### 8. TLS Metrics

| Метрика | Тип | Описание |
|---------|-----|----------|
| `hsm_tls_handshakes_total` | Counter | TLS handshake'и |
| `hsm_tls_errors_total` | Counter | TLS ошибки |

**Labels**:
- `error_type` - тип ошибки (certificate_expired, unknown_ca)

**Пример**:
```promql
# TLS error rate
rate(hsm_tls_errors_total[5m])

# TLS handshake success rate
rate(hsm_tls_handshakes_total[5m]) / (rate(hsm_tls_handshakes_total[5m]) + rate(hsm_tls_errors_total[5m]))
```

---

## Prometheus Setup

### 1. Создание сертификата для Prometheus

```bash
cd /opt/hsm-service/pki
./scripts/issue-client-cert.sh monitoring prometheus-server
```

### 2. Конфигурация Prometheus

**prometheus.yml**:
```yaml
global:
  scrape_interval: 15s
  evaluation_interval: 15s
  external_labels:
    cluster: 'production'
    environment: 'prod'

# Alertmanager configuration
alerting:
  alertmanagers:
    - static_configs:
        - targets:
            - 'localhost:9093'

# Load rules
rule_files:
  - '/etc/prometheus/rules/hsm-service.yml'

# Scrape configs
scrape_configs:
  - job_name: 'hsm-service'
    scheme: https
    scrape_interval: 10s
    scrape_timeout: 5s
    
    tls_config:
      ca_file: /etc/prometheus/certs/ca.crt
      cert_file: /etc/prometheus/certs/monitoring.crt
      key_file: /etc/prometheus/certs/monitoring.key
      insecure_skip_verify: false
    
    static_configs:
      - targets:
          - 'hsm-service-1.example.com:8443'
          - 'hsm-service-2.example.com:8443'
        labels:
          instance_group: 'hsm-primary'
          datacenter: 'dc1'
    
    metric_relabel_configs:
      # Drop some internal metrics (опционально)
      - source_labels: [__name__]
        regex: 'go_.*'
        action: drop

  - job_name: 'node-exporter'
    static_configs:
      - targets:
          - 'hsm-service-1.example.com:9100'
          - 'hsm-service-2.example.com:9100'
```

### 3. Проверка конфигурации

```bash
# Validate config
promtool check config /etc/prometheus/prometheus.yml

# Test scrape
curl -k https://hsm-service.example.com:8443/metrics \
  --cert /etc/prometheus/certs/monitoring.crt \
  --key /etc/prometheus/certs/monitoring.key \
  --cacert /etc/prometheus/certs/ca.crt
```

---

## Grafana Dashboards

### Dashboard 1: Overview

**Панели**:

1. **Request Rate** (Graph)
```promql
# QPS
sum(rate(hsm_requests_total[1m])) by (endpoint)
```

2. **Error Rate** (Graph)
```promql
# Errors per second
sum(rate(hsm_errors_total[1m])) by (type)
```

3. **Latency** (Graph)
```promql
# P50, P95, P99
histogram_quantile(0.50, sum(rate(hsm_request_duration_seconds_bucket[5m])) by (le))
histogram_quantile(0.95, sum(rate(hsm_request_duration_seconds_bucket[5m])) by (le))
histogram_quantile(0.99, sum(rate(hsm_request_duration_seconds_bucket[5m])) by (le))
```

4. **Active Keys** (Gauge)
```promql
hsm_active_keys
```

5. **ACL Denials** (Graph)
```promql
sum(rate(hsm_acl_denials_total[5m])) by (client_cn, reason)
```

6. **Rate Limit** (Graph)
```promql
sum(rate(hsm_rate_limit_exceeded_total[5m])) by (client_cn)
```

### Dashboard 2: HSM Operations

1. **Operations per Second**
```promql
sum(rate(hsm_operations_total[1m])) by (operation, context)
```

2. **HSM Latency**
```promql
histogram_quantile(0.95, sum(rate(hsm_operation_duration_seconds_bucket[5m])) by (le, operation))
```

3. **Key Age**
```promql
(time() - hsm_key_age_seconds) / 86400  # days
```

4. **Rotation Events**
```promql
increase(hsm_rotation_total[24h])
```

### Dashboard 3: Security

1. **TLS Errors**
```promql
sum(rate(hsm_tls_errors_total[5m])) by (error_type)
```

2. **ACL Violations**
```promql
topk(10, sum by (client_cn) (increase(hsm_acl_denials_total[1h])))
```

3. **Revoked Certificates**
```promql
# Custom metric from ACL reload
hsm_acl_revoked_count
```

4. **Failed Authentications**
```promql
sum(rate(hsm_tls_errors_total{error_type="certificate_required"}[5m]))
```

### Готовый JSON dashboard

```json
{
  "dashboard": {
    "title": "HSM Service - Overview",
    "panels": [
      {
        "title": "Request Rate",
        "targets": [
          {
            "expr": "sum(rate(hsm_requests_total[1m])) by (endpoint)"
          }
        ]
      }
    ]
  }
}
```

---

## Логирование

HSM Service ведет два независимых потока логов:

- **audit.log** — все запросы (audit trail)
- **error.log** — системные события и ошибки

**Особенности:**
- JSON формат, единое время: UTC, RFC3339 с микросекундами
- `request_id` добавляется во все события для корреляции
- Audit лог пишется в stdout (удобно для `docker logs`)
- В debug режиме audit дублируется в error лог
- Сервис не стартует без доступа на запись/ротацию

**Пути по умолчанию:**
- `/var/log/hsm-service/audit.log`
- `/var/log/hsm-service/error.log`

**Пример настройки в config.yaml:**
```yaml
logging:
  level: info
  format: json
  error_path: /var/log/hsm-service/error.log
  audit_path: /var/log/hsm-service/audit.log
  max_size_mb: 100
  max_backups: 10
  max_age_days: 30
  compress: true
  audit_to_stdout: true
```

---

## Alerting Rules

### Critical Alerts

**hsm-service-critical.yml**:
```yaml
groups:
  - name: hsm_critical
    interval: 30s
    rules:
      # Service down
      - alert: HSMServiceDown
        expr: up{job="hsm-service"} == 0
        for: 1m
        labels:
          severity: critical
          component: hsm-service
        annotations:
          summary: "HSM Service is down"
          description: "Instance {{ $labels.instance }} has been down for more than 1 minute"
      
      # High error rate
      - alert: HSMHighErrorRate
        expr: |
          (
            sum(rate(hsm_errors_total[5m])) 
            / 
            sum(rate(hsm_requests_total[5m]))
          ) > 0.05
        for: 5m
        labels:
          severity: critical
        annotations:
          summary: "High error rate detected"
          description: "Error rate is {{ $value | humanizePercentage }} (threshold: 5%)"
      
      # HSM unavailable
      - alert: HSMOperationsFailing
        expr: rate(hsm_errors_total{type="hsm_error"}[5m]) > 0
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "HSM operations failing"
          description: "HSM errors detected on {{ $labels.instance }}"
      
      # Key rotation failed
      - alert: KeyRotationFailed
        expr: increase(hsm_rotation_errors_total[1h]) > 0
        labels:
          severity: critical
        annotations:
          summary: "Key rotation failed"
          description: "Failed to rotate key for context {{ $labels.context }}"
```

### Warning Alerts

**hsm-service-warnings.yml**:
```yaml
groups:
  - name: hsm_warnings
    interval: 1m
    rules:
      # High latency
      - alert: HSMHighLatency
        expr: |
          histogram_quantile(0.95, 
            sum(rate(hsm_request_duration_seconds_bucket[5m])) by (le)
          ) > 0.5
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "High request latency"
          description: "P95 latency is {{ $value }}s (threshold: 0.5s)"
      
      # ACL denials spike
      - alert: HighACLDenials
        expr: rate(hsm_acl_denials_total[5m]) > 1
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Unusual number of ACL denials"
          description: "{{ $value }} denials/sec for client {{ $labels.client_cn }}"
      
      # Rate limit abuse
      - alert: RateLimitAbuse
        expr: rate(hsm_rate_limit_exceeded_total[5m]) > 0.1
        for: 10m
        labels:
          severity: warning
        annotations:
          summary: "Client hitting rate limits"
          description: "Client {{ $labels.client_cn }} exceeded rate limit {{ $value }} times/sec"
      
      # Old keys
      - alert: KeyTooOld
        expr: (time() - hsm_key_age_seconds) > (90 * 86400)
        labels:
          severity: warning
        annotations:
          summary: "Encryption key is very old"
          description: "Key {{ $labels.context }}-v{{ $labels.version }} is {{ $value | humanizeDuration }} old (threshold: 90 days)"
      
      # ACL reload errors
      - alert: ACLReloadErrors
        expr: increase(hsm_acl_reload_errors_total[1h]) > 3
        labels:
          severity: warning
        annotations:
          summary: "ACL reload failing"
          description: "{{ $value }} ACL reload errors in the last hour"
```

### Info Alerts

**hsm-service-info.yml**:
```yaml
groups:
  - name: hsm_info
    interval: 5m
    rules:
      # Key rotation completed
      - alert: KeyRotationCompleted
        expr: increase(hsm_rotation_total[5m]) > 0
        labels:
          severity: info
        annotations:
          summary: "Key rotation completed"
          description: "Successfully rotated key for {{ $labels.context }}"
      
      # ACL reloaded
      - alert: ACLReloaded
        expr: increase(hsm_acl_reload_total[5m]) > 0
        labels:
          severity: info
        annotations:
          summary: "ACL configuration reloaded"
          description: "revoked.yaml reloaded successfully"
```

### Alertmanager Configuration

**alertmanager.yml**:
```yaml
global:
  smtp_smarthost: 'smtp.example.com:587'
  smtp_from: 'alerts@example.com'
  smtp_auth_username: 'alerts@example.com'
  smtp_auth_password: 'password'

route:
  receiver: 'default'
  group_by: ['alertname', 'cluster', 'severity']
  group_wait: 10s
  group_interval: 10s
  repeat_interval: 12h
  
  routes:
    # Critical - сразу в PagerDuty + email
    - match:
        severity: critical
      receiver: 'pagerduty'
      continue: true
    
    # Warnings - email + Slack
    - match:
        severity: warning
      receiver: 'slack-warnings'
    
    # Info - только Slack
    - match:
        severity: info
      receiver: 'slack-info'

receivers:
  - name: 'default'
    email_configs:
      - to: 'devops@example.com'
  
  - name: 'pagerduty'
    pagerduty_configs:
      - service_key: 'YOUR_PAGERDUTY_KEY'
    email_configs:
      - to: 'oncall@example.com'
        send_resolved: true
  
  - name: 'slack-warnings'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
        channel: '#hsm-alerts'
        title: '{{ .GroupLabels.alertname }}'
        text: '{{ range .Alerts }}{{ .Annotations.description }}{{ end }}'
  
  - name: 'slack-info'
    slack_configs:
      - api_url: 'https://hooks.slack.com/services/YOUR/WEBHOOK/URL'
        channel: '#hsm-info'
        title: '{{ .GroupLabels.alertname }}'
```

---

## Логирование

### Structured Logging (JSON)

HSM Service логирует в JSON формате и разделяет потоки:
- audit.log (PCI DSS, крипто-операции)
- access.log (все HTTP запросы)
- error.log (системные ошибки и диагностика)

```json
{
  "time": "2024-01-15T10:30:45Z",
  "level": "info",
  "msg": "Request processed successfully",
  "endpoint": "/encrypt",
  "client_cn": "trading-service-1",
  "context": "exchange-key",
  "duration_ms": 15.3,
  "status_code": 200
}

Пример access log:
```json
{
  "time": "2024-01-15T10:30:45.000000Z",
  "request_id": "4f3b2b1c-6a7d-4b9a-9f2f-2a9b9b8f1c77",
  "method": "POST",
  "path": "/encrypt",
  "status": 200,
  "duration_ms": 15,
  "client_ip": "10.0.0.5:53412",
  "user_agent": "curl/8.4.0",
  "tls_version": "TLS1.3",
  "request_size_bytes": 128,
  "response_size_bytes": 256
}
```
```

### Log Levels

- **DEBUG**: Детальная информация для разработки
- **INFO**: Нормальные операции (запросы, ACL checks, ротация)
- **WARN**: Предупреждения (ACL reload errors, old keys)
- **ERROR**: Ошибки (HSM failures, TLS errors)

### Анализ логов

```bash
# Все ошибки за последний час
journalctl -u hsm-service --since "1 hour ago" | jq 'select(.level=="error")'

# Top clients по количеству запросов
journalctl -u hsm-service --since "1 hour ago" | jq -r '.client_cn' | sort | uniq -c | sort -rn

# ACL denials
journalctl -u hsm-service | jq 'select(.msg | contains("access denied"))'

# Latency distribution
journalctl -u hsm-service | jq -r '.duration_ms' | sort -n | tail -100
```

### ELK Stack Integration

**filebeat.yml**:
```yaml
filebeat.inputs:
  - type: journald
    id: hsm-service
    include_matches:
      - systemd.unit=hsm-service.service

processors:
  - decode_json_fields:
      fields: ["message"]
      target: "json"

output.elasticsearch:
  hosts: ["elasticsearch:9200"]
  index: "hsm-service-%{+yyyy.MM.dd}"
```

**Logstash pipeline**:
```ruby
filter {
  json {
    source => "message"
  }
  
  if [level] == "error" {
    mutate {
      add_tag => ["error"]
    }
  }
}
```

---

## Performance Monitoring

### Ключевые метрики производительности

#### 1. Latency (задержка)

**SLO**: P95 < 100ms, P99 < 500ms

```promql
# P95 latency
histogram_quantile(0.95, sum(rate(hsm_request_duration_seconds_bucket[5m])) by (le, endpoint))

# Alert if P95 > 100ms
alert: HighP95Latency
expr: histogram_quantile(0.95, ...) > 0.1
```

#### 2. Throughput (пропускная способность)

**SLO**: > 1000 req/sec

```promql
# Current RPS
sum(rate(hsm_requests_total[1m]))

# Peak RPS (last 24h)
max_over_time(sum(rate(hsm_requests_total[1m]))[24h:])
```

#### 3. Error Rate

**SLO**: < 0.1% (99.9% success rate)

```promql
# Error percentage
(sum(rate(hsm_errors_total[5m])) / sum(rate(hsm_requests_total[5m]))) * 100
```

#### 4. Availability

**SLO**: 99.95% uptime

```promql
# Uptime percentage
avg_over_time(up{job="hsm-service"}[30d]) * 100
```

### Resource Usage

```promql
# Memory usage
hsm_memory_usage_bytes / 1024 / 1024  # MB

# Goroutines
hsm_goroutines

# CPU (from node_exporter)
rate(process_cpu_seconds_total{job="hsm-service"}[5m]) * 100
```

---

## Troubleshooting

### Problem: Metrics не scrape'ятся

```bash
# Check TLS certificates
openssl s_client -connect hsm-service:8443 \
  -cert /etc/prometheus/certs/monitoring.crt \
  -key /etc/prometheus/certs/monitoring.key

# Check Prometheus logs
journalctl -u prometheus | grep hsm-service

# Test manual scrape
curl -k https://hsm-service:8443/metrics \
  --cert monitoring.crt \
  --key monitoring.key \
  --cacert ca.crt
```

### Problem: Alerts не срабатывают

```bash
# Check Prometheus rules
promtool check rules /etc/prometheus/rules/hsm-service.yml

# Check firing alerts
curl http://localhost:9090/api/v1/alerts

# Check Alertmanager
curl http://localhost:9093/api/v1/alerts
```

### Problem: Высокая latency

```bash
# Check HSM operations
curl -k https://localhost:8443/metrics | grep hsm_operation_duration

# Check system resources
htop
iotop
nethogs

# Check logs for slow operations
journalctl -u hsm-service | jq 'select(.duration_ms > 100)'
```

---

## SLI/SLO Tracking

### Service Level Indicators (SLI)

| Метрика | SLO | Период | Запрос |
|---------|-----|--------|--------|
| Availability | 99.95% | 30 дней | `avg_over_time(up[30d])` |
| Latency P95 | < 100ms | 5 минут | `histogram_quantile(0.95, ...)` |
| Error Rate | < 0.1% | 5 минут | `errors / requests` |
| Throughput | > 1000 req/s | 1 минута | `rate(requests[1m])` |

### Error Budget

Для 99.95% availability:
- Допустимый downtime: 21.6 минут/месяц
- Допустимые ошибки: 0.05% запросов

**Запрос для tracking error budget**:
```promql
# Оставшийся error budget (%)
100 - (
  (1 - avg_over_time(up{job="hsm-service"}[30d])) 
  / 
  (1 - 0.9995)
) * 100
```

---

## Next Steps

- [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) - Production deployment
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Решение проблем
- [BACKUP_RESTORE.md](BACKUP_RESTORE.md) - Backup и восстановление
