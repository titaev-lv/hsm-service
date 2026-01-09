# Automatic Revocation List Reload

## Overview

HSM Service автоматически перезагружает `revoked.yaml` каждые **30 секунд** без перезапуска сервиса.

## Features

### ✅ Automatic Reload
- Проверка файла каждые 30 секунд
- Перезагрузка только если файл изменился (проверка modTime)
- Нет перезагрузки если файл не менялся (эффективно)

### ✅ Validation Protection
- **YAML Parsing**: битый YAML не загружается
- **Empty CN Check**: записи с пустым CN отклоняются
- **Duplicate Detection**: дубликаты CN обнаруживаются
- **Atomic Update**: либо загружаются все записи, либо ни одна
- **Old Data Preserved**: при ошибке валидации старые данные сохраняются

### ✅ File Deletion Handling
- Если `revoked.yaml` удален → список очищается (никто не заблокирован)
- Логируется событие удаления файла
- Безопасное восстановление при воссоздании файла

### ✅ Graceful Shutdown
- Auto-reload останавливается при shutdown
- Timeout 15 секунд для graceful stop
- Goroutine корректно завершается

## Configuration

```yaml
acl:
  revoked_file: /app/pki/revoked.yaml  # Path to revocation list
  mappings:
    operations:
      - user-keys
      - service-keys
    finance:
      - payment-keys
```

## File Format

```yaml
revoked:
  - cn: "client1.example.com"
    serial: "1A:2B:3C:4D"
    reason: "key-compromise"
    date: "2024-01-15"
  
  - cn: "old-service.example.com"
    serial: "5E:6F:7A:8B"
    reason: "cessation-of-operation"
    date: "2024-02-01"
```

## Validation Rules

### ✅ Valid Entry
```yaml
revoked:
  - cn: "test.example.com"     # Non-empty CN required
    serial: "1234"              # Any string
    reason: "compromised"       # Any string
    date: "2024-01-01"          # Any string
```

### ❌ Invalid Entry (Empty CN)
```yaml
revoked:
  - cn: ""                      # ERROR: empty CN
    serial: "1234"
    reason: "test"
    date: "2024-01-01"
```

### ❌ Invalid Entry (Duplicate CN)
```yaml
revoked:
  - cn: "test.example.com"
    serial: "1234"
    reason: "compromised"
    date: "2024-01-01"
  
  - cn: "test.example.com"      # ERROR: duplicate CN
    serial: "5678"
    reason: "duplicate"
    date: "2024-01-02"
```

### ❌ Invalid YAML
```yaml
revoked:
  - cn: "test.example.com"
    reason: "unclosed quote      # ERROR: syntax error
    date: invalid [
```

## Behavior

### Successful Reload
```
INFO  revoked.yaml reloaded successfully path=/app/pki/revoked.yaml count=5
```

### Failed Reload (Validation Error)
```
WARN  revoked.yaml reload skipped due to validation error path=/app/pki/revoked.yaml
```
- **Old data preserved**: сервис продолжает работать с последней валидной версией
- **No downtime**: клиенты не затронуты
- **Security**: битые данные не применяются

### File Deleted
```
INFO  revoked.yaml deleted, cleared revocation list
```

## Performance

- **Minimal overhead**: проверка только modTime файла
- **No reload if unchanged**: экономия CPU
- **Lock-free reads**: RWMutex для concurrent access
- **Fast validation**: O(n) complexity

## Security Guarantees

1. **Atomic Updates**: старые данные не смешиваются с новыми
2. **Validation First**: битый файл НЕ применяется
3. **No Partial State**: либо весь файл валидный, либо игнорируется
4. **Thread-Safe**: concurrent reads во время reload
5. **Information Disclosure Prevention**: ошибки не содержат sensitive data

## Monitoring

### Prometheus Metrics
```
# Reload attempts
acl_reload_attempts_total{status="success"} 150
acl_reload_attempts_total{status="validation_error"} 2
acl_reload_attempts_total{status="io_error"} 0

# Current revoked count
acl_revoked_count 12

# Revocation check failures
acl_revocation_failures_total 45
```

### Logs to Monitor
```bash
# Successful reloads
grep "revoked.yaml reloaded successfully" /var/log/hsm-service/hsm-service.log

# Validation errors
grep "reload skipped due to validation error" /var/log/hsm-service/hsm-service.log

# File deletion events
grep "revoked.yaml deleted" /var/log/hsm-service/hsm-service.log
```

## Testing

### Test Reload Manually

1. Start service:
```bash
docker-compose up -d
```

2. Add revoked cert:
```bash
cd pki
./scripts/revoke-cert.sh client1.example.com "key-compromise"
```

3. Wait 30 seconds, check logs:
```bash
docker-compose logs hsm-service | grep reload
```

4. Test access (should get 403):
```bash
curl --cert pki/client/client1.crt \
     --key pki/client/client1.key \
     --cacert pki/ca/ca.crt \
     -H "Content-Type: application/json" \
     -d '{"context":"user-keys","plaintext":"dGVzdA=="}' \
     https://localhost:8443/encrypt
```

### Test Invalid YAML Protection

1. Break YAML syntax:
```bash
echo 'revoked: [invalid syntax' > pki/revoked.yaml
```

2. Wait 30 seconds, check logs:
```bash
docker-compose logs hsm-service | grep "validation error"
```

3. Service should still work with old data
4. Fix YAML → reload succeeds

## Comparison with SIGHUP

| Feature | Auto-Reload (Current) | SIGHUP Manual |
|---------|---------------------|---------------|
| **Human intervention** | Not required | Required |
| **Automation** | Fully automatic | Manual signal |
| **Reload delay** | 30 seconds max | Immediate |
| **Error handling** | Built-in validation | Custom logic needed |
| **DevOps friendly** | Yes (just edit file) | No (need signal) |
| **Kubernetes/Docker** | Works everywhere | Signal routing needed |

## Trade-offs

### ✅ Advantages
- Zero human intervention
- Automatic validation
- Safe error handling
- Works in containers
- Predictable behavior

### ⚠️ Considerations
- 30-second delay (not instant)
- Small background overhead
- File watching instead of instant notification

## Future Enhancements

### Possible Improvements
1. **Configurable interval**: allow setting reload frequency in config
2. **File watcher**: use inotify/fsnotify for instant reload
3. **Metrics**: add reload success/failure counters
4. **Health check**: expose reload status in /health endpoint
5. **Manual trigger**: add HTTP endpoint `/admin/reload-revocations`

### Alternative: CRL Support
Instead of `revoked.yaml`, implement proper CRL (Certificate Revocation List):
- Standard X.509 format
- OCSP support
- Automatic distribution
- Industry standard

## Related Documentation

- [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - Security analysis
- [TECHNICAL_SPEC.md](TECHNICAL_SPEC.md) - Technical specification
- [ARCHITECTURE.md](ARCHITECTURE.md) - System architecture
- [pki/README.md](pki/README.md) - PKI setup and certificate management

## Support

For questions or issues:
1. Check logs: `/var/log/hsm-service/hsm-service.log`
2. Verify file permissions: `ls -la pki/revoked.yaml`
3. Test YAML syntax: `yamllint pki/revoked.yaml`
4. Review security audit: [SECURITY_AUDIT.md](SECURITY_AUDIT.md)
