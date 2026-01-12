# 🧪 Тестирование HSM Service

Полное руководство по тестированию HSM Service: unit-тесты, интеграционные тесты, E2E сценарии, нагрузочное и security тестирование.

---

## 📋 Оглавление

1. [Быстрый старт](#-быстрый-старт)
2. [Типы тестов](#-типы-тестов)
3. [Требования и установка](#-требования-и-установка)
4. [Unit-тесты](#-unit-тесты)
5. [Интеграционные тесты](#-интеграционные-тесты)
6. [E2E тесты](#-e2e-тесты)
7. [Нагрузочное тестирование](#-нагрузочное-тестирование)
8. [Security тестирование](#-security-тестирование)
9. [Coverage и бенчмарки](#-coverage-и-бенчмарки)
10. [Troubleshooting](#-troubleshooting)

---

## 🚀 Быстрый старт

### Запуск всех тестов

```bash
# Полный test suite (все типы тестов)
./tests/run-all-tests.sh

# Только unit-тесты
go test ./...

# С race detector
go test -race ./...

# С coverage
go test -cover ./...
```

### Запуск по категориям

```bash
# Только интеграционные
./tests/integration/full-integration-test.sh

# Только E2E сценарии
./tests/e2e/run-all.sh

# Только security сканирование
./tests/security/security-scan.sh

# Нагрузочное тестирование
./tests/performance/stress-test.sh
```

---

## 📊 Типы тестов

### Обзор реализованных тестов

| Тип | Файлов | Тестов | Описание |
|-----|--------|--------|----------|
| **Unit Tests** | 14 файлов | ~70 тестов | Изолированное тестирование функций |
| **Integration** | 1 скрипт | 42 теста | API тестирование с Docker |
| **E2E Scenarios** | 3 сценария | 3 workflow | End-to-end бизнес-процессы |
| **Performance** | 4 скрипта | 8+ нагрузочных | Stress и load тестирование |
| **Security** | 2 скрипта | 8 проверок | Vulnerability scanning |

**Всего**: ~10,000 строк тестового кода

---

## 🛠 Требования и установка

### Базовые требования

**Обязательно**:
```bash
# Go 1.22+
go version  # должно быть >= 1.22

# Docker и Docker Compose
docker --version          # >= 20.10
docker compose version    # >= v2.0

# Базовые утилиты
curl --version
openssl version
bash --version  # >= 4.0
```

### Дополнительные инструменты

**Для нагрузочного тестирования**:
```bash
# Vegeta - HTTP load generator
go install github.com/tsenart/vegeta@latest

# Проверка установки
vegeta -version
```

**Для security сканирования**:

1. **gosec** - Go security checker:
```bash
go install github.com/securego/gosec/v2/cmd/gosec@latest
```

2. **staticcheck** - Go static analyzer:
```bash
go install honnef.co/go/tools/cmd/staticcheck@latest
```

3. **govulncheck** - Go vulnerability database:
```bash
go install golang.org/x/vuln/cmd/govulncheck@latest
```

4. **trivy** - Container scanner:

Ubuntu/Debian:
```bash
sudo apt-get install wget apt-transport-https gnupg lsb-release
wget -qO - https://aquasecurity.github.io/trivy-repo/deb/public.key | sudo apt-key add -
echo "deb https://aquasecurity.github.io/trivy-repo/deb $(lsb_release -sc) main" | sudo tee /etc/apt/sources.list.d/trivy.list
sudo apt-get update
sudo apt-get install trivy
```

Arch Linux:
```bash
sudo pacman -S trivy
```

macOS:
```bash
brew install trivy
```

**Проверка установки**:
```bash
# Добавить Go tools в PATH
export PATH="$PATH:$(go env GOPATH)/bin"

# Проверить все инструменты
gosec --version
staticcheck -version
govulncheck -version
trivy --version
vegeta -version
```

---

## 📦 Unit-тесты

### Структура unit-тестов

```
internal/
├── config/                      # 3 test файла
│   ├── config_test.go          # Базовые тесты конфигурации (3 теста)
│   ├── config_extended_test.go # Расширенные тесты (7 тестов)
│   └── http2_test.go           # HTTP/2 конфигурация (2 теста)
│
├── hsm/                         # 6 test файлов
│   ├── crypto_test.go          # Шифрование/расшифровка (6 тестов)
│   ├── crypto_extended_test.go # Расширенное крипто (11 тестов + 4 benchmark)
│   ├── key_manager_test.go     # KeyManager hot reload (5 тестов)
│   ├── key_manager_extended_test.go # Расширенный KeyManager (13 тестов + 3 benchmark)
│   ├── metadata_test.go        # Метаданные и ротация (6 тестов + 1 benchmark)
│   └── rotation_test.go        # Ротация ключей (5 тестов)
│
└── server/                      # 5 test файлов
    ├── handlers_test.go        # HTTP handlers (17 тестов)
    ├── acl_test.go             # ACL проверки (8 тестов)
    ├── acl_reload_test.go      # Auto-reload ACL (6 тестов)
    ├── middleware_test.go      # Rate limiting (5 тестов)
    └── logger_test.go          # Логирование (8 тестов)
```

### Запуск unit-тестов

```bash
# Все unit-тесты
go test ./...

# С подробным выводом
go test -v ./...

# С race detector (ВАЖНО!)
go test -race ./...

# Быстрый режим (пропускает long-running)
go test -short ./...

# Конкретный модуль
go test ./internal/hsm/
go test ./internal/server/
go test ./internal/config/

# Конкретный тест
go test -run TestEncryptDecrypt ./internal/hsm/
go test -run TestACL ./internal/server/
```

### Описание unit-тестов

#### internal/hsm/ - HSM модуль (46 тестов)

**crypto_test.go** (6 тестов):
- `TestEncryptDecrypt` - базовое шифрование/расшифровка
- `TestEncrypt_InvalidContext` - обработка невалидного context
- `TestDecrypt_InvalidCiphertext` - обработка битого ciphertext
- `TestEncrypt_EmptyPlaintext` - пустой plaintext
- `TestDecrypt_WrongKey` - расшифровка неправильным ключом
- `TestEncrypt_LargePlaintext` - большие данные (10KB)

**crypto_extended_test.go** (11 тестов + 4 benchmark):
- `TestEncryptDecrypt_MultipleContexts` - несколько contexts
- `TestEncrypt_ConcurrentOperations` - concurrent шифрование
- `TestDecrypt_DifferentVersions` - расшифровка разными версиями ключей
- `TestEncryptDecrypt_SpecialCharacters` - спецсимволы в данных
- `TestEncrypt_MaxPayloadSize` - максимальный размер payload
- `TestDecrypt_CorruptedMetadata` - поврежденные метаданные
- `TestEncryptDecrypt_BinaryData` - бинарные данные
- `TestEncrypt_EmptyKey` - пустой ключ
- `TestDecrypt_MissingKey` - отсутствующий ключ
- `TestEncryptDecrypt_UnicodeData` - Unicode данные
- `TestEncrypt_RapidSequential` - быстрая последовательность операций
- **Benchmarks**: BenchmarkEncrypt, BenchmarkDecrypt, BenchmarkEncryptLarge, BenchmarkDecryptLarge

**key_manager_test.go** (5 тестов):
- `TestKeyManager_LoadKeys` - загрузка ключей
- `TestKeyManager_GetKey` - получение ключа
- `TestKeyManager_Reload` - hot reload метаданных
- `TestKeyManager_ConcurrentAccess` - concurrent доступ
- `TestKeyManager_InvalidMetadata` - невалидные метаданные

**key_manager_extended_test.go** (13 тестов + 3 benchmark):
- `TestKeyManager_MultipleVersions` - множественные версии ключей
- `TestKeyManager_AutoReload` - автоматическая перезагрузка
- `TestKeyManager_ReloadFailure` - обработка ошибок reload
- `TestKeyManager_ConcurrentReload` - concurrent reload
- `TestKeyManager_GetKeyByVersion` - получение ключа по версии
- `TestKeyManager_GetCurrentKey` - получение текущего ключа
- `TestKeyManager_MissingContext` - отсутствующий context
- `TestKeyManager_EmptyMetadata` - пустые метаданные
- `TestKeyManager_CorruptedYAML` - поврежденный YAML
- `TestKeyManager_VersionRollback` - откат версии
- `TestKeyManager_ReloadPreservesState` - reload сохраняет состояние
- `TestKeyManager_GetAllKeys` - получение всех ключей
- `TestKeyManager_FileWatcher` - file watcher механизм
- **Benchmarks**: BenchmarkGetKey, BenchmarkReload, BenchmarkConcurrentGetKey

**metadata_test.go** (6 тестов + 1 benchmark):
- `TestLoadMetadata` - загрузка метаданных
- `TestSaveMetadata` - сохранение метаданных
- `TestUpdateMetadata` - обновление метаданных
- `TestValidateMetadata` - валидация метаданных
- `TestMetadata_Versioning` - версионирование
- `TestMetadata_Concurrent` - concurrent операции
- **Benchmark**: BenchmarkLoadMetadata

**rotation_test.go** (5 тестов):
- `TestRotateKey_CreateNewVersion` - создание новой версии ключа
- `TestRotateKey_UpdateMetadata` - обновление метаданных при ротации
- `TestRotateKey_PreserveOldKeys` - сохранение старых ключей
- `TestRotateKey_ConcurrentRotation` - concurrent ротация
- `TestRotateKey_ValidationErrors` - валидация ошибок

#### internal/server/ - Server модуль (44 теста)

**handlers_test.go** (17 тестов):
- `TestEncryptHandler` - тест /encrypt endpoint
- `TestDecryptHandler` - тест /decrypt endpoint
- `TestHealthHandler` - тест /health endpoint
- `TestMetricsHandler` - тест /metrics endpoint
- `TestEncryptHandler_InvalidJSON` - невалидный JSON
- `TestEncryptHandler_MissingFields` - отсутствующие поля
- `TestDecryptHandler_InvalidCiphertext` - невалидный ciphertext
- `TestEncryptHandler_Unauthorized` - неавторизованный доступ
- `TestDecryptHandler_WrongContext` - неправильный context
- `TestEncryptHandler_EmptyPlaintext` - пустой plaintext
- `TestDecryptHandler_EmptyCiphertext` - пустой ciphertext
- `TestHandlers_ConcurrentRequests` - concurrent запросы
- `TestHandlers_LargePayload` - большой payload
- `TestHandlers_RateLimiting` - rate limiting
- `TestHandlers_ErrorResponses` - error responses
- `TestHandlers_MetricsIncrement` - инкремент метрик
- `TestHandlers_AuditLogging` - audit logging

**acl_test.go** (8 тестов):
- `TestACL_CheckAccess` - проверка доступа
- `TestACL_LoadMapping` - загрузка ACL маппинга
- `TestACL_Unauthorized` - неавторизованный доступ
- `TestACL_MultipleOUs` - множественные OU
- `TestACL_CaseInsensitive` - регистронезависимая проверка
- `TestACL_EmptyOU` - пустой OU
- `TestACL_UnknownContext` - неизвестный context
- `TestACL_WildcardAccess` - wildcard доступ

**acl_reload_test.go** (6 тестов):
- `TestACL_AutoReload` - автоматическая перезагрузка
- `TestACL_ReloadOnFileChange` - reload при изменении файла
- `TestACL_ReloadValidation` - валидация при reload
- `TestACL_ReloadConcurrent` - concurrent reload
- `TestACL_ReloadPreservesState` - reload сохраняет состояние
- `TestACL_ReloadErrorHandling` - обработка ошибок reload

**middleware_test.go** (5 тестов):
- `TestRateLimiter` - базовый rate limiter
- `TestRateLimiter_Concurrent` - concurrent rate limiting
- `TestRateLimiter_PerClient` - per-client limiting
- `TestRateLimiter_Reset` - сброс лимитов
- `TestRateLimiter_Overflow` - превышение лимита

**logger_test.go** (8 тестов):
- `TestLogger_Info` - info логирование
- `TestLogger_Error` - error логирование
- `TestLogger_Warn` - warn логирование
- `TestLogger_AuditLog` - audit логирование
- `TestLogger_Structured` - structured logging
- `TestLogger_Concurrent` - concurrent logging
- `TestLogger_Rotation` - log rotation
- `TestLogger_Levels` - log levels

#### internal/config/ - Config модуль (12 тестов)

**config_test.go** (3 теста):
- `TestLoadConfig` - загрузка конфигурации
- `TestValidateConfig` - валидация конфигурации
- `TestConfig_Defaults` - значения по умолчанию

**config_extended_test.go** (7 тестов):
- `TestConfig_Environment` - переменные окружения
- `TestConfig_InvalidYAML` - невалидный YAML
- `TestConfig_MissingFile` - отсутствующий файл
- `TestConfig_Override` - переопределение значений
- `TestConfig_Validation` - расширенная валидация
- `TestConfig_TLSSettings` - TLS настройки
- `TestConfig_HSMSettings` - HSM настройки

**http2_test.go** (2 теста):
- `TestHTTP2Config` - HTTP/2 конфигурация
- `TestHTTP2_Negotiation` - HTTP/2 negotiation

---

## 🔗 Интеграционные тесты

### full-integration-test.sh

**Расположение**: `tests/integration/full-integration-test.sh`  
**Тестов**: 42 теста  
**Время выполнения**: ~2-3 минуты

**Этапы тестирования**:

**Phase 1: Environment Setup** (10 тестов)
- Docker контейнер запущен
- HSM инициализирован
- Сертификаты созданы
- Метаданные загружены
- ACL настроен
- Health check проходит
- Metrics доступны
- TLS работает
- mTLS проверяется
- Rate limiting активен

**Phase 2: Basic Operations** (8 тестов)
- Encrypt операция (Trading client)
- Decrypt операция (Trading client)
- Encrypt с 2FA client
- Decrypt с 2FA client
- Большой payload (10KB)
- Пустой plaintext (валидация)
- Невалидный context
- Несанкционированный доступ

**Phase 3: ACL & Authorization** (10 тестов)
- Trading → exchange-key (✅ разрешено)
- Trading → 2fa (❌ запрещено)
- 2FA → 2fa (✅ разрешено)
- 2FA → exchange-key (❌ запрещено)
- Revoked сертификат (❌ 403)
- Отсутствующий сертификат (❌ tls error)
- Неправильный CA (❌ tls error)
- Expired сертификат (❌ tls error)
- Множественные OU
- Wildcard context

**Phase 4: Key Rotation** (8 тестов)
- Проверка текущей версии
- Ротация ключа
- Новая версия создана
- Старая версия доступна
- Encrypt использует новую версию
- Decrypt старых данных работает
- Metadata обновлена
- Hot reload без перезапуска

**Phase 5: Advanced Scenarios** (6 тестов)
- Concurrent операции (100 запросов)
- Rate limiting (превышение лимита)
- Metrics корректные
- Audit logs записаны
- Error handling
- Graceful shutdown

**Запуск**:
```bash
# Запуск integration тестов
./tests/integration/full-integration-test.sh

# С debug выводом
DEBUG=1 ./tests/integration/full-integration-test.sh

# С cleanup после теста
CLEANUP=1 ./tests/integration/full-integration-test.sh
```

---

## 🎬 E2E тесты

### Сценарии E2E тестирования

**Расположение**: `tests/e2e/scenarios/`

#### 1. key-rotation-e2e.sh - Полный цикл ротации ключей

**Что тестирует**:
- Создание начальных ключей
- Шифрование данных текущим ключом
- Ротация ключа
- Проверка новой версии
- Расшифровка старых данных
- Шифрование новых данных новым ключом
- Проверка что обе версии работают
- Cleanup старых версий

**Запуск**:
```bash
./tests/e2e/scenarios/key-rotation-e2e.sh
```

#### 2. disaster-recovery.sh - Сценарий восстановления

**Что тестирует**:
- Создание backup HSM токена
- Создание backup метаданных
- Шифрование тестовых данных
- Симуляция потери данных
- Восстановление из backup
- Проверка что расшифровка работает
- Проверка целостности данных

**Запуск**:
```bash
./tests/e2e/scenarios/disaster-recovery.sh
```

#### 3. acl-realtime-reload.sh - Динамическое обновление ACL

**Что тестирует**:
- Начальный доступ клиента
- Добавление нового правила ACL
- Автоматическая перезагрузка (30s)
- Проверка нового доступа
- Revocation сертификата
- Проверка блокировки (403)
- Восстановление доступа

**Запуск**:
```bash
./tests/e2e/scenarios/acl-realtime-reload.sh
```

### Запуск всех E2E тестов

```bash
# Все E2E сценарии
./tests/e2e/run-all.sh

# Конкретный сценарий
./tests/e2e/scenarios/key-rotation-e2e.sh
./tests/e2e/scenarios/disaster-recovery.sh
./tests/e2e/scenarios/acl-realtime-reload.sh
```

---

## ⚡ Нагрузочное тестирование

### Performance тесты

**Расположение**: `tests/performance/`

#### 1. smoke-test.sh - Дымовой тест

**Цель**: Быстрая проверка базовой функциональности  
**Нагрузка**: 100 req/s на 10 секунд  
**Что тестирует**:
- Encrypt endpoint
- Decrypt endpoint
- Базовая производительность

**Запуск**:
```bash
./tests/performance/smoke-test.sh
```

#### 2. stress-test.sh - Стандартный нагрузочный тест

**Цель**: Найти breaking point при нормальной нагрузке  
**Нагрузка**: Постепенно от 1k до 30k req/s  
**Длительность**: 30s на каждый уровень  
**Что тестирует**:
- Encrypt: 1k, 5k, 10k, 15k, 20k, 25k, 30k req/s
- Decrypt: 1k, 5k, 10k, 15k, 20k req/s
- Mixed workload (encrypt + decrypt)
- Success rate на каждом уровне

**Запуск**:
```bash
./tests/performance/stress-test.sh

# Результаты
cat stress-results/*.txt
```

#### 3. stress-test-extreme.sh - Экстремальный тест

**Цель**: Найти абсолютный предел системы  
**Нагрузка**: До 100k req/s  
**Конфигурация**: 4 CPU cores, HTTP/2, keepalive  
**Что тестирует**:

**Test 1**: Encrypt Ultra High Load
- 20k, 25k, 30k, 40k, 50k, 75k, 100k req/s
- 30 секунд на каждый уровень
- Результат: 100% success до 100k req/s ✅

**Test 2**: Decrypt Ultra High Load
- 20k, 25k, 30k, 40k, 50k, 75k, 100k req/s
- 30 секунд на каждый уровень
- Результат: 100% success до 100k req/s ✅

**Test 3**: Massive Spike Attack
- 100k req/s на 20 секунд
- Симуляция DDoS атаки
- Результат: 100% success, 24.5k throughput ✅

**Test 4**: Mixed Workload
- 10k encrypt + 10k decrypt одновременно
- Результат: 100% success оба ✅

**Test 5**: Large Payload
- 512 bytes payload
- 5k req/s
- Результат: 100% success ✅

**Test 6**: Round-Trip Latency
- 1000 итераций encrypt→decrypt
- Результат: 40ms среднее время ✅

**Test 7**: Burst Recovery
- Baseline (5k) → Burst (50k) → Recovery (5k)
- Результат: Полное восстановление ✅

**Test 8**: Multi-Context
- Trading (7.5k) + 2FA (7.5k) параллельно
- Результат: 100% success оба ✅

**Результаты** (подробнее в [EXTREME_TEST_RESULTS.md](../EXTREME_TEST_RESULTS.md)):
- Breaking point: НЕ НАЙДЕН до 100k req/s
- Sustained throughput: 20-21k req/s
- P95 latency: 99ms
- Round-trip: 40ms

**Запуск**:
```bash
./tests/performance/stress-test-extreme.sh

# Просмотр результатов
cat stress-results-extreme/*.txt
```

#### 4. benchmark-test.sh - Go бенчмарки

**Цель**: Измерение производительности отдельных функций  
**Что тестирует**:
- BenchmarkEncrypt
- BenchmarkDecrypt
- BenchmarkEncryptLarge
- BenchmarkDecryptLarge
- BenchmarkGetKey
- BenchmarkReload
- BenchmarkConcurrentGetKey

**Запуск**:
```bash
./tests/performance/benchmark-test.sh

# Или напрямую через go test
go test -bench=. -benchmem ./internal/hsm/
```

---

## 🔒 Security тестирование

### Security сканирование

**Расположение**: `tests/security/security-scan.sh`

**Проверки** (8 security checks):

1. **gosec** - Go Security Checker
   - Поиск уязвимостей в коде
   - SQL injection, XSS, crypto misuse
   - Command injection, path traversal

2. **staticcheck** - Static Analysis
   - Неиспользуемый код
   - Потенциальные баги
   - Code smells

3. **govulncheck** - Vulnerability Database
   - Известные уязвимости в зависимостях
   - CVE проверки

4. **trivy** - Container Scanner (если доступен)
   - Сканирование Docker образа
   - OS vulnerabilities
   - Dependency vulnerabilities

5. **TLS Configuration Check**
   - Только TLS 1.3
   - Strong cipher suites
   - Certificate validation

6. **Secret Scanner**
   - Поиск hardcoded secrets
   - API keys, passwords
   - Private keys

7. **Dependency Check**
   - go mod verify
   - go list -m all

8. **File Permissions**
   - Проверка permissions
   - 600 для sensitive файлов
   - 700 для директорий

**Запуск**:
```bash
# Полное security сканирование
./tests/security/security-scan.sh

# Если tools не установлены - пропускаются
# Установка: см. раздел "Требования и установка"
```

### Compliance тесты

**Расположение**: `tests/compliance/`

#### pci-dss.sh - PCI DSS Compliance

**Проверки**:
- Requirement 3.5: Crypto keys protection
- Requirement 3.6: Key documentation
- Requirement 3.6.4: Key rotation (90 days)
- Requirement 10.2: Audit logging
- TLS 1.3 requirement
- Certificate revocation

**Запуск**:
```bash
./tests/compliance/pci-dss.sh
```

#### owasp-top10.sh - OWASP Top 10

**Проверки**:
- A01: Broken Access Control
- A02: Cryptographic Failures
- A03: Injection
- A05: Security Misconfiguration
- A07: Identification and Authentication Failures
- A09: Security Logging and Monitoring Failures

**Запуск**:
```bash
./tests/compliance/owasp-top10.sh
```

---

## 📈 Coverage и бенчмарки

### Coverage (Покрытие кода)

```bash
# Базовый coverage
go test -cover ./...

# Детальный coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html

# Просмотр в браузере
firefox coverage.html
```

**Текущее покрытие**:
- internal/hsm: ~85%
- internal/server: ~75%
- internal/config: ~70%
- **Overall**: ~80%

### Benchmarks

```bash
# Все бенчмарки
go test -bench=. ./...

# Конкретный модуль
go test -bench=. ./internal/hsm/

# С memory profiling
go test -bench=. -benchmem ./internal/hsm/

# CPU profiling
go test -bench=BenchmarkEncrypt -cpuprofile=cpu.prof ./internal/hsm/
go tool pprof cpu.prof
```

**Пример вывода**:
```
BenchmarkEncrypt-8           1000000    1234 ns/op    512 B/op    8 allocs/op
BenchmarkDecrypt-8           1000000    1156 ns/op    512 B/op    7 allocs/op
BenchmarkGetKey-8           10000000     123 ns/op      0 B/op    0 allocs/op
```

---

## 🔧 Troubleshooting

### Проблема: "too many open files"

**Решение**:
```bash
# Увеличить лимит file descriptors
ulimit -n 10000

# Проверить
ulimit -n
```

### Проблема: Race detector находит проблемы

**Пример**:
```
WARNING: DATA RACE
Read at 0x00c0001a2008 by goroutine 23
```

**Решение**:
1. Найти строку в коде
2. Добавить mutex protection
3. Перезапустить `go test -race`

### Проблема: Тесты timeout

**Решение**:
```bash
# Увеличить timeout
go test -timeout 30s ./internal/hsm/

# Для integration тестов
go test -timeout 15m ./...
```

### Проблема: Docker контейнер не запускается

**Проверка**:
```bash
# Статус контейнера
docker compose ps

# Логи
docker compose logs hsm-service

# Перезапуск
docker compose restart
```

### Проблема: Vegeta не установлен

**Решение**:
```bash
# Установка
go install github.com/tsenart/vegeta@latest

# Проверка
vegeta -version

# Добавить в PATH если нужно
export PATH="$PATH:$(go env GOPATH)/bin"
```

---

## 📋 Checklist перед commit

- [ ] `go test -short -race ./...` проходит
- [ ] Coverage ≥ 80% для новых файлов
- [ ] `./tests/integration/full-integration-test.sh` проходит
- [ ] `./tests/security/security-scan.sh` без критичных проблем
- [ ] Нет TODO в тестах для critical функций
- [ ] Race detector clean
- [ ] Benchmarks показывают приемлемую производительность

---

## 🚀 Полный test pipeline (CI/CD)

```bash
#!/bin/bash
# Рекомендуемый pipeline для CI/CD

echo "=== Phase 1: Unit Tests ==="
go test -short -race -cover ./...

echo "=== Phase 2: Integration Tests ==="
./tests/integration/full-integration-test.sh

echo "=== Phase 3: E2E Tests ==="
./tests/e2e/run-all.sh

echo "=== Phase 4: Security Scan ==="
./tests/security/security-scan.sh

echo "=== Phase 5: Performance Tests ==="
./tests/performance/smoke-test.sh

echo "=== Phase 6: Compliance ==="
./tests/compliance/pci-dss.sh
./tests/compliance/owasp-top10.sh

echo "✅ All tests passed!"
```

---

## 📊 Статистика тестирования

| Метрика | Значение |
|---------|----------|
| **Unit тестов** | ~70 тестов |
| **Integration тестов** | 42 теста |
| **E2E сценариев** | 3 workflow |
| **Performance тестов** | 8 нагрузочных |
| **Security проверок** | 8 checks |
| **Coverage** | ~80% |
| **Строк тестового кода** | ~10,000 строк |
| **Время выполнения (full)** | ~15-20 минут |

---

## 🔗 Дополнительные ресурсы

- [Go Testing](https://go.dev/doc/tutorial/add-a-test)
- [Race Detector](https://go.dev/doc/articles/race_detector)
- [Coverage Tool](https://go.dev/blog/cover)
- [Vegeta Documentation](https://github.com/tsenart/vegeta)
- [gosec](https://github.com/securego/gosec)
- [Trivy](https://github.com/aquasecurity/trivy)

---

**Документ обновлен**: 2026-01-13  
**Версия**: 2.0
