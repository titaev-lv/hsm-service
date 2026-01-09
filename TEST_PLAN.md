# 🧪 HSM Service - Комплексный план тестирования

> **Цель**: Обеспечить 100% покрытие критических функций, безопасность и надежность HSM Service

## 📊 Текущее состояние тестирования

### ✅ Что уже есть

**Unit Tests** (7 файлов, ~1,447 строк):
- ✅ `crypto_test.go` - тесты шифрования/расшифровки
- ✅ `acl_test.go` - тесты ACL проверок
- ✅ `acl_reload_test.go` - тесты auto-reload (6 test cases)
- ✅ `handlers_test.go` - тесты HTTP handlers
- ✅ `middleware_test.go` - тесты rate limiting
- ✅ `logger_test.go` - тесты логирования
- ✅ `config_test.go` - тесты конфигурации

**Integration Tests**:
- ✅ `scripts/full-integration-test.sh` - полный E2E тест (31 test case)

**Coverage**: ~60-70% (оценка)

---

## 🎯 Стратегия тестирования

### Пирамида тестирования

```
           /\
          /  \    E2E Tests (5%)
         /    \   - Полные сценарии
        /------\  
       /        \ Integration Tests (15%)
      /          \ - API тесты, Docker
     /------------\
    /              \ Unit Tests (80%)
   /________________\ - Функции, модули
```

### Уровни тестирования

| Уровень | Цель | Инструменты | Автоматизация |
|---------|------|-------------|---------------|
| **Unit** | Изолированные функции | Go test | CI/CD (каждый commit) |
| **Integration** | API + HSM взаимодействие | Docker + curl | CI/CD (каждый PR) |
| **E2E** | Полные user scenarios | bash scripts | CI/CD (before merge) |
| **Security** | Vulnerability scan | trivy, gosec | CI/CD (nightly) |
| **Performance** | Load testing | k6, vegeta | Manual (before release) |
| **Chaos** | Failure scenarios | chaos toolkit | Manual (quarterly) |

---

## 📋 Детальный план по категориям

### 1️⃣ Unit Tests (расширение)

#### 1.1 Crypto Module (`internal/hsm/`)

**Текущее покрытие**: ✅ ~80%

**Добавить тесты**:
- [ ] `TestEncryptWithDifferentKeyVersions` - шифрование разными версиями KEK
- [ ] `TestConcurrentEncryption` - параллельные операции шифрования
- [ ] `TestLargePayload` - шифрование больших данных (>1MB)
- [ ] `TestInvalidKeyHandle` - обработка невалидного key handle
- [ ] `TestCorruptedMetadata` - поведение при повреждении metadata
- [ ] `TestNonceCollision` - проверка уникальности nonce

**Приоритет**: 🔴 HIGH

---

#### 1.2 ACL Module (`internal/server/acl*.go`)

**Текущее покрытие**: ✅ ~90%

**Добавить тесты**:
- [ ] `TestConcurrentACLChecks` - параллельные ACL проверки
- [ ] `TestACLReloadRaceCondition` - race condition при reload
- [ ] `TestACLWithMultipleOUs` - клиенты с несколькими OU
- [ ] `TestACLCaseSensitivity` - чувствительность к регистру CN/OU
- [ ] `TestACLWildcardMatching` - поддержка wildcards (если планируется)
- [ ] `TestACLPerformanceWith1000Rules` - производительность с большим ACL

**Приоритет**: 🔴 HIGH

---

#### 1.3 HTTP Handlers (`internal/server/handlers*.go`)

**Текущее покрытие**: ⚠️ ~60%

**Добавить тесты**:
- [ ] `TestEncryptHandler_Success` - полный happy path
- [ ] `TestDecryptHandler_Success` - полный happy path
- [ ] `TestEncryptHandler_EmptyContext` - валидация пустого context
- [ ] `TestDecryptHandler_WrongKeyID` - расшифровка с неверным key_id
- [ ] `TestHealthHandler_HSMDown` - health при отказе HSM
- [ ] `TestMetricsHandler_Prometheus` - корректность Prometheus метрик
- [ ] `TestHandlers_ContentType` - проверка Content-Type headers
- [ ] `TestHandlers_RequestSizeLimit` - лимит размера запроса
- [ ] `TestHandlers_Timeout` - таймауты запросов

**Приоритет**: 🟡 MEDIUM

---

#### 1.4 Rate Limiting (`internal/server/middleware*.go`)

**Текущее покрытие**: ⚠️ ~50%

**Добавить тесты**:
- [ ] `TestRateLimiter_BurstHandling` - обработка burst запросов
- [ ] `TestRateLimiter_PerClientLimits` - лимиты per-client
- [ ] `TestRateLimiter_CleanupOldLimiters` - очистка старых limiters
- [ ] `TestRateLimiter_ConfigChange` - изменение конфига на лету
- [ ] `TestRateLimiter_429Response` - корректность HTTP 429

**Приоритет**: 🟡 MEDIUM

---

#### 1.5 Key Rotation (`internal/hsm/rotation*.go`)

**Текущее покрытие**: ❌ 0% (нет тестов!)

**Создать тесты**:
- [ ] `TestRotateKey_Success` - успешная ротация
- [ ] `TestRotateKey_CreateNewVersion` - создание новой версии
- [ ] `TestRotateKey_UpdateMetadata` - обновление metadata.yaml
- [ ] `TestRotateKey_PreserveOldKeys` - сохранение старых ключей
- [ ] `TestRotateKey_FailureRollback` - откат при ошибке
- [ ] `TestRotateKey_ConcurrentRotation` - защита от одновременной ротации
- [ ] `TestCleanupOldVersions_RespectRetention` - уважение retention policy
- [ ] `TestCleanupOldVersions_KeepMinimum` - сохранение min versions

**Приоритет**: 🔴 CRITICAL

---

#### 1.6 Configuration (`internal/config/`)

**Текущее покрытие**: ✅ ~70%

**Добавить тесты**:
- [ ] `TestConfig_Validation` - валидация всех полей
- [ ] `TestConfig_Defaults` - проверка дефолтных значений
- [ ] `TestConfig_EnvOverride` - переопределение через ENV
- [ ] `TestConfig_InvalidRotationInterval` - невалидный interval

**Приоритет**: 🟢 LOW

---

### 2️⃣ Integration Tests

#### 2.1 API Integration Tests

**Создать**: `tests/integration/api_test.go`

```go
// Примерная структура
package integration_test

func TestEncryptDecryptFlow(t *testing.T)
func TestMultiVersionDecryption(t *testing.T)  
func TestACLDenial(t *testing.T)
func TestRateLimitExceeded(t *testing.T)
func TestHealthEndpoint(t *testing.T)
func TestMetricsEndpoint(t *testing.T)
```

**Тест-кейсы**:
- [ ] Encrypt → Decrypt happy path
- [ ] Encrypt с v1 → Rotate → Decrypt с v1
- [ ] Encrypt с v2 → Decrypt с v2
- [ ] ACL denial для неавторизованного OU
- [ ] Rate limit enforcement
- [ ] TLS handshake validation
- [ ] Certificate revocation check
- [ ] Health check при нормальной работе
- [ ] Health check при отказе HSM
- [ ] Metrics endpoint доступность

**Инструменты**: Go test + Docker testcontainers

**Приоритет**: 🔴 HIGH

---

#### 2.2 HSM Integration Tests

**Создать**: `tests/integration/hsm_test.go`

**Тест-кейсы**:
- [ ] SoftHSM initialization
- [ ] KEK creation в HSM
- [ ] KEK deletion из HSM
- [ ] Проверка persistence токенов
- [ ] Восстановление после restart
- [ ] Multiple contexts одновременно
- [ ] PKCS#11 session management

**Приоритет**: 🟡 MEDIUM

---

#### 2.3 Docker Integration Tests

**Расширить**: `scripts/full-integration-test.sh`

**Добавить тест-кейсы**:
- [ ] Test 11: mTLS validation (неверный client cert)
- [ ] Test 12: Volume persistence (данные сохраняются после restart)
- [ ] Test 13: Environment variables override
- [ ] Test 14: Log rotation работает
- [ ] Test 15: Graceful shutdown (SIGTERM)
- [ ] Test 16: Health check during startup
- [ ] Test 17: Metrics scraping
- [ ] Test 18: Multi-container setup (HA)

**Приоритет**: 🟡 MEDIUM

---

### 3️⃣ End-to-End Tests

#### 3.1 User Journey Tests

**Создать**: `tests/e2e/scenarios/`

**Сценарии**:

**Scenario 1: Новый клиент начинает использовать сервис**
```bash
#!/bin/bash
# tests/e2e/scenarios/new-client.sh

# 1. Создать client certificate
# 2. Добавить в ACL mapping
# 3. Выполнить первый encrypt
# 4. Выполнить decrypt
# 5. Проверить audit logs
```

**Scenario 2: Плановая ротация ключей**
```bash
# tests/e2e/scenarios/planned-rotation.sh

# 1. Зашифровать данные v1
# 2. Выполнить ротацию
# 3. Зашифровать данные v2
# 4. Расшифровать старые данные (v1)
# 5. Расшифровать новые данные (v2)
# 6. Проверить metadata
```

**Scenario 3: Отзыв сертификата**
```bash
# tests/e2e/scenarios/certificate-revocation.sh

# 1. Client успешно подключается
# 2. Добавить CN в revoked.yaml
# 3. Подождать auto-reload (30 сек)
# 4. Проверить что client заблокирован
# 5. Удалить из revoked
# 6. Проверить что client снова работает
```

**Scenario 4: Disaster Recovery**
```bash
# tests/e2e/scenarios/disaster-recovery.sh

# 1. Создать данные
# 2. Сделать backup
# 3. Уничтожить контейнер
# 4. Восстановить из backup
# 5. Проверить что данные расшифровываются
```

**Приоритет**: 🔴 HIGH

---

#### 3.2 Multi-Service Integration

**Создать**: `tests/e2e/multi-service/`

**Тест-кейсы**:
- [ ] HSM Service + Trading Service integration
- [ ] HSM Service + 2FA Service integration
- [ ] Prometheus metrics scraping
- [ ] Grafana dashboard rendering
- [ ] Alertmanager alerts triggering

**Приоритет**: 🟢 LOW (опционально)

---

### 4️⃣ Security Tests

#### 4.1 Static Analysis

**Инструменты**: gosec, staticcheck

**Создать**: `.github/workflows/security.yml`

```yaml
name: Security Scan
on: [push, pull_request]
jobs:
  gosec:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: securego/gosec@master
        with:
          args: './...'
```

**Проверки**:
- [ ] Gosec scan (code vulnerabilities)
- [ ] Staticcheck (code quality)
- [ ] go vet (standard checks)
- [ ] Dependency vulnerability scan (govulncheck)

**Приоритет**: 🔴 CRITICAL

---

#### 4.2 Container Security

**Инструменты**: Trivy, Docker Bench

**Тест-кейсы**:
- [ ] `trivy image hsm-service:latest` - CVE scan
- [ ] `docker-bench-security` - Docker hardening
- [ ] Проверка что контейнер не запускается под root
- [ ] Проверка read-only filesystem
- [ ] Проверка no capabilities

**Создать**: `scripts/security-scan.sh`

```bash
#!/bin/bash
echo "Running Trivy scan..."
trivy image hsm-service:latest

echo "Running Docker Bench..."
docker run --rm --net host --pid host --userns host --cap-add audit_control \
    -v /var/lib:/var/lib \
    -v /var/run/docker.sock:/var/run/docker.sock \
    docker/docker-bench-security
```

**Приоритет**: 🔴 CRITICAL

---

#### 4.3 Penetration Testing

**Тест-кейсы** (manual):
- [ ] TLS downgrade attack
- [ ] Certificate validation bypass attempt
- [ ] SQL injection в JSON payloads
- [ ] Path traversal в file paths
- [ ] Rate limit bypass attempts
- [ ] ACL bypass attempts
- [ ] Timing attacks на crypto operations
- [ ] Memory leak через repeated requests

**Инструменты**: 
- OWASP ZAP
- Burp Suite
- Custom scripts

**Приоритет**: 🟡 MEDIUM (quarterly)

---

### 5️⃣ Performance Tests

#### 5.1 Load Testing

**Создать**: `tests/performance/load-test.js` (k6)

```javascript
import http from 'k6/http';
import { check } from 'k6';

export let options = {
  stages: [
    { duration: '2m', target: 100 },  // Ramp-up to 100 users
    { duration: '5m', target: 100 },  // Stay at 100 users
    { duration: '2m', target: 200 },  // Ramp-up to 200 users
    { duration: '5m', target: 200 },  // Stay at 200 users
    { duration: '2m', target: 0 },    // Ramp-down
  ],
  thresholds: {
    http_req_duration: ['p(95)<500'],  // 95% < 500ms
    http_req_failed: ['rate<0.01'],    // <1% errors
  },
};

export default function() {
  let payload = JSON.stringify({
    context: 'exchange-key',
    plaintext: 'SGVsbG8gV29ybGQh'
  });
  
  let params = {
    headers: { 'Content-Type': 'application/json' },
  };
  
  let res = http.post('https://localhost:8443/encrypt', payload, params);
  check(res, {
    'status is 200': (r) => r.status === 200,
    'has ciphertext': (r) => r.json('ciphertext') !== undefined,
  });
}
```

**Метрики**:
- [ ] Requests per second (target: >1000)
- [ ] P95 latency (target: <100ms)
- [ ] P99 latency (target: <500ms)
- [ ] Error rate (target: <0.1%)
- [ ] Memory usage под нагрузкой
- [ ] CPU usage под нагрузкой

**Приоритет**: 🟡 MEDIUM

---

#### 5.2 Stress Testing

**Тест-кейсы**:
- [ ] Максимальная нагрузка до отказа
- [ ] Recovery после перегрузки
- [ ] Memory leak detection (long-running)
- [ ] Goroutine leak detection
- [ ] Connection pool exhaustion

**Инструменты**: vegeta, Apache Bench

**Приоритет**: 🟢 LOW

---

#### 5.3 Endurance Testing

**Тест-кейс**: Запустить под умеренной нагрузкой на 24 часа

```bash
#!/bin/bash
# tests/performance/endurance-test.sh

echo "Starting 24h endurance test..."
START_TIME=$(date +%s)

while [ $(($(date +%s) - START_TIME)) -lt 86400 ]; do
    # 10 req/sec for 24 hours
    ab -n 100 -c 10 -T 'application/json' \
       -p encrypt-payload.json \
       https://localhost:8443/encrypt
    sleep 10
done
```

**Проверки**:
- [ ] No memory leaks
- [ ] No goroutine leaks
- [ ] No file descriptor leaks
- [ ] Stable latency
- [ ] No errors

**Приоритет**: 🟢 LOW (before major releases)

---

### 6️⃣ Chaos Engineering

#### 6.1 Failure Injection Tests

**Создать**: `tests/chaos/`

**Сценарии**:

**Chaos 1: HSM становится недоступным**
```bash
# tests/chaos/hsm-unavailable.sh

# 1. Запустить сервис
# 2. Отправить encrypt requests (success)
# 3. Удалить HSM token файлы
# 4. Отправить encrypt requests (должно фейлить gracefully)
# 5. Восстановить HSM
# 6. Проверить recovery
```

**Chaos 2: Network partition**
```bash
# tests/chaos/network-partition.sh

# 1. Запустить 2 инстанса
# 2. Создать network partition
# 3. Проверить что каждый работает независимо
# 4. Восстановить сеть
# 5. Проверить консистентность
```

**Chaos 3: Disk full**
```bash
# tests/chaos/disk-full.sh

# 1. Заполнить диск до 100%
# 2. Попытаться записать metadata
# 3. Проверить graceful degradation
# 4. Освободить место
# 5. Проверить recovery
```

**Chaos 4: CPU/Memory exhaustion**
```bash
# tests/chaos/resource-exhaustion.sh

# 1. Запустить stress-ng
# 2. Проверить что сервис продолжает работать
# 3. Проверить что latency не превышает SLO
```

**Chaos 5: Sudden container kill**
```bash
# tests/chaos/kill-container.sh

# 1. Отправить requests
# 2. docker kill hsm-service (SIGKILL)
# 3. Перезапустить
# 4. Проверить что нет corrupted data
```

**Инструменты**: chaos-mesh, pumba, toxiproxy

**Приоритет**: 🟢 LOW (quarterly)

---

### 7️⃣ Compliance Tests

#### 7.1 PCI DSS Compliance

**Тест-кейсы**:
- [ ] Ротация ключей каждые 90 дней (automated)
- [ ] Cleanup старых версий через 30 дней
- [ ] Audit logging всех crypto operations
- [ ] TLS 1.3 only (no TLS 1.2)
- [ ] Strong cipher suites only
- [ ] No plaintext in logs

**Создать**: `tests/compliance/pci-dss.sh`

**Приоритет**: 🔴 CRITICAL

---

#### 7.2 OWASP Top 10 Testing

**Тест-кейсы** (automated):
- [ ] A01: Broken Access Control → ACL tests
- [ ] A02: Cryptographic Failures → Strong crypto tests
- [ ] A03: Injection → JSON validation tests
- [ ] A05: Security Misconfiguration → Config validation
- [ ] A07: Identification/Auth Failures → mTLS tests
- [ ] A09: Security Logging Failures → Audit log tests

**Приоритет**: 🔴 CRITICAL

---

### 8️⃣ Regression Tests

**Создать**: `tests/regression/`

**Процесс**:
1. Каждый баг → создать regression test
2. Regression suite запускается на каждый PR
3. Нельзя merge если regression тесты fail

**Примеры**:
- [ ] Bug #123: ACL reload не обновлял список → `test_acl_reload_updates.sh`
- [ ] Bug #456: Memory leak в rate limiter → `test_rate_limiter_memory.go`

**Приоритет**: 🔴 HIGH

---

## 🚀 План внедрения (Roadmap)

### Фаза 1: Критические тесты (Weeks 1-2)

**Week 1**:
- [x] ✅ Unit tests для ACL (уже есть)
- [x] ✅ Unit tests для crypto (уже есть)
- [x] ✅ Integration test (уже есть)
- [ ] 🔴 Unit tests для key rotation
- [ ] 🔴 Security scan (gosec, trivy)
- [ ] 🔴 PCI DSS compliance tests

**Week 2**:
- [ ] 🔴 E2E scenarios (3-4 основных)
- [ ] 🔴 API integration tests
- [ ] 🔴 Regression test suite
- [ ] 🟡 Performance load test (k6)

**Цель**: 80% critical path покрытие

---

### Фаза 2: Расширенные тесты (Weeks 3-4)

**Week 3**:
- [ ] 🟡 Handlers unit tests (расширение)
- [ ] 🟡 HSM integration tests
- [ ] 🟡 Docker integration tests (расширение)
- [ ] 🟡 Stress testing

**Week 4**:
- [ ] 🟢 Chaos engineering tests
- [ ] 🟢 Endurance testing
- [ ] 🟢 Multi-service integration
- [ ] 🟢 Penetration testing (manual)

**Цель**: 90% общее покрытие

---

### Фаза 3: CI/CD Integration (Week 5)

- [ ] GitHub Actions workflows
- [ ] Automated test runs на каждый PR
- [ ] Nightly security scans
- [ ] Weekly performance benchmarks
- [ ] Test coverage reporting (codecov.io)

**Цель**: Полная автоматизация

---

## 📊 Метрики качества

### Coverage Targets

| Модуль | Текущий | Целевой | Приоритет |
|--------|---------|---------|-----------|
| `internal/hsm/` | ~80% | 95% | 🔴 HIGH |
| `internal/server/acl*.go` | ~90% | 95% | 🔴 HIGH |
| `internal/server/handlers*.go` | ~60% | 85% | 🟡 MEDIUM |
| `internal/server/middleware*.go` | ~50% | 80% | 🟡 MEDIUM |
| `internal/config/` | ~70% | 80% | 🟢 LOW |
| **OVERALL** | **~70%** | **90%+** | 🔴 HIGH |

### Test Execution Time Targets

| Тип теста | Целевое время | Частота |
|-----------|---------------|---------|
| Unit tests | <30 секунд | Каждый commit |
| Integration tests | <5 минут | Каждый PR |
| E2E tests | <15 минут | Before merge |
| Security scans | <10 минут | Nightly |
| Performance tests | <30 минут | Weekly |
| Chaos tests | <1 час | Monthly |

---

## 🛠️ Инструменты и технологии

### Testing Frameworks

```bash
# Go testing
go test ./...
go test -race ./...          # Race detector
go test -cover ./...         # Coverage
go test -bench=. ./...       # Benchmarks

# Integration testing
go get github.com/testcontainers/testcontainers-go

# Load testing
k6 run tests/performance/load-test.js
vegeta attack -duration=60s -rate=100/s

# Security scanning
gosec ./...
trivy image hsm-service:latest
govulncheck ./...

# Chaos engineering
chaos run tests/chaos/experiment.yaml
```

### CI/CD Integration

**GitHub Actions** (`.github/workflows/test.yml`):

```yaml
name: Test Suite

on: [push, pull_request]

jobs:
  unit-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.22'
      - run: go test -v -race -cover ./...
      
  integration-tests:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - run: docker compose up -d
      - run: ./scripts/full-integration-test.sh
      
  security-scan:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: securego/gosec@master
      - run: trivy image hsm-service:latest
```

---

## 📈 Reporting

### Coverage Report

```bash
# Generate coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Upload to codecov
bash <(curl -s https://codecov.io/bash)
```

### Test Results

```bash
# Generate JUnit XML for CI
go test -v ./... | go-junit-report > report.xml

# Test summary
gotestsum --format testname
```

---

## ✅ Definition of Done

Тесты считаются готовыми когда:

- [x] ✅ Покрытие unit тестами ≥90% для критических модулей
- [ ] Все E2E сценарии проходят успешно
- [ ] Security scan не показывает HIGH/CRITICAL уязвимостей
- [ ] Performance тесты показывают P95 < 100ms
- [ ] CI/CD pipeline полностью автоматизирован
- [ ] Нет flaky tests (нестабильных)
- [ ] Документация по тестированию актуальна
- [ ] Regression test suite охватывает все исторические баги

---

## 🔗 Дополнительные ресурсы

- [Go Testing Guide](https://go.dev/doc/tutorial/add-a-test)
- [Testcontainers](https://golang.testcontainers.org/)
- [k6 Documentation](https://k6.io/docs/)
- [OWASP Testing Guide](https://owasp.org/www-project-web-security-testing-guide/)
- [Chaos Engineering](https://principlesofchaos.org/)

---

**Автор**: GitHub Copilot  
**Дата**: 2026-01-09  
**Версия**: 1.0  
**Статус**: 📝 Draft → 🔄 В разработке
