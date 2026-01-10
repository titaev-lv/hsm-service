# 🚀 Шпаргалка по тестированию - Quick Reference

## Основные команды

```bash
# Быстрая проверка (перед commit)
go test -short -race ./...

# Полный прогон всех тестов
go test -v -race ./...

# Coverage с HTML отчётом
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out

# Запуск по модулям
go test -v ./internal/hsm/        # HSM + crypto + rotation
go test -v ./internal/server/     # HTTP handlers + ACL + rate limiting
go test -v ./internal/config/     # Configuration

# Конкретный тест
go test -run TestRotateKey_CreateNewVersion ./internal/hsm/
go test -run TestRateLimiter_BurstHandling ./internal/server/

# Benchmarks
go test -bench=. -benchmem ./internal/hsm/

# Integration тесты
./scripts/full-integration-test.sh
```

## По категориям

### Key Rotation (новые тесты!)
```bash
go test -v -run Rotation ./internal/hsm/
# TestRotateKey_CreateNewVersion
# TestRotateKey_UpdateMetadata
# TestRotateKey_PreserveOldKeys
# TestCleanupOldVersions_RespectRetention
# TestCleanupOldVersions_KeepMinimum
```

### Crypto + Nonce
```bash
go test -v -run Nonce ./internal/hsm/
# TestNonceCollision (10,000 nonces)
# TestNonceUniquenessUnderConcurrency (100 goroutines)
```

### HTTP Handlers
```bash
go test -v -run Handler ./internal/server/
# TestEncryptHandler_Success
# TestDecryptHandler_Success
# TestHandlers_ContentType
# TestHandlers_RequestSizeLimit
# TestEncryptHandler_ConcurrentRequests
```

### Rate Limiting
```bash
go test -v -run RateLimiter ./internal/server/
# TestRateLimiter_BurstHandling
# TestRateLimiter_PerClientLimits
# TestRateLimiter_429Response
# TestRateLimiter_Concurrency
```

### Configuration
```bash
go test -v ./internal/config/
# TestConfig_Validation
# TestConfig_Defaults
# TestConfig_EnvOverride
# TestMetadata_SaveAndLoad
```

## Флаги

```bash
-v              # Verbose (детальный вывод)
-race           # Race detector (ВАЖНО!)
-short          # Пропустить long-running тесты
-cover          # Показать coverage
-coverprofile   # Сохранить coverage profile
-bench=.        # Запустить benchmarks
-benchmem       # Показать memory allocations в benchmarks
-run <pattern>  # Запустить только тесты по шаблону
-timeout 30s    # Установить timeout
```

## Coverage

```bash
# Базовый coverage
go test -cover ./...

# Детальный (по функциям)
go test -coverprofile=coverage.out ./...
go tool cover -func=coverage.out

# HTML отчёт
go tool cover -html=coverage.out -o coverage.html
firefox coverage.html

# Coverage конкретного модуля
go test -coverprofile=hsm.out ./internal/hsm/
go tool cover -html=hsm.out
```

## Benchmarks

```bash
# Запуск всех бенчмарков
go test -bench=. ./internal/hsm/

# С memory profiling
go test -bench=. -benchmem ./internal/hsm/

# CPU profiling
go test -bench=BenchmarkEncryption -cpuprofile=cpu.prof ./internal/hsm/
go tool pprof cpu.prof

# Memory profiling
go test -bench=BenchmarkEncryption -memprofile=mem.prof ./internal/hsm/
go tool pprof mem.prof

# Сравнение до/после
go test -bench=. ./internal/hsm/ > old.txt
# ... внести изменения ...
go test -bench=. ./internal/hsm/ > new.txt
benchstat old.txt new.txt
```

## Troubleshooting

```bash
# Найти race conditions
go test -race ./...

# Только fast тесты
go test -short ./...

# С timeout (если тесты висят)
go test -timeout 10s ./internal/hsm/

# Детальный вывод ошибок
go test -v -run TestFailingTest ./internal/hsm/
```

## Pre-commit Checklist

```bash
# 1. Быстрая проверка
go test -short -race ./...

# 2. Проверка coverage
go test -cover ./... | grep coverage

# 3. Форматирование
go fmt ./...

# 4. Vet (статический анализ)
go vet ./...

# 5. Сборка
go build
```

## Интеграция

```bash
# Полный E2E тест (34 теста)
./scripts/full-integration-test.sh

# С debug выводом
DEBUG=1 ./scripts/full-integration-test.sh

# Только hot reload тест (Phase 9.5)
# (смотри full-integration-test.sh строки 493-548)
```

## Целевые показатели

| Модуль | Coverage | Статус |
|--------|----------|--------|
| internal/hsm/ | 87% | ✅ |
| internal/server/ | 85% | ✅ |
| internal/config/ | 85% | ✅ |
| **Overall** | **86%** | **🎉** |

## Полная документация

📖 **TESTING_GUIDE.md** - подробное руководство (350+ строк)
📋 **TEST_PLAN.md** - план тестирования со статусами

---

**Быстрая проверка перед commit:**
```bash
go test -short -race ./... && echo "✅ Ready to commit"
```
