# 🧪 Руководство по тестированию HSM Service

## 📚 Оглавление

1. [Быстрый старт](#быстрый-старт)
2. [Типы тестов](#типы-тестов)
3. [Запуск тестов](#запуск-тестов)
4. [Анализ результатов](#анализ-результатов)
5. [Coverage](#coverage)
6. [Benchmarks](#benchmarks)
7. [Troubleshooting](#troubleshooting)

---

## 🚀 Быстрый старт

### Запуск всех unit тестов

```bash
# Простой запуск всех тестов
go test ./...

# С подробным выводом
go test -v ./...

# С проверкой race conditions
go test -race ./...

# Быстрый режим (пропускает long-running тесты)
go test -short ./...
```

### Проверка coverage

```bash
# Coverage для всего проекта
go test -cover ./...

# Детальный coverage report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
firefox coverage.html  # или ваш браузер
```

---

## 📋 Типы тестов

### 1. Unit Tests

**Что тестируют**: Изолированные функции и модули

**Файлы**:
- `internal/hsm/*_test.go` - тесты HSM модуля
- `internal/server/*_test.go` - тесты HTTP сервера
- `internal/config/*_test.go` - тесты конфигурации

**Запуск**:
```bash
# Все unit тесты
go test ./internal/...

# Конкретный пакет
go test ./internal/hsm/
go test ./internal/server/
go test ./internal/config/

# Конкретный тест
go test -run TestRotateKey_CreateNewVersion ./internal/hsm/
```

### 2. Integration Tests

**Что тестируют**: Взаимодействие компонентов, API endpoints

**Файлы**:
- `scripts/full-integration-test.sh` - E2E тесты с Docker

**Запуск**:
```bash
# Полный integration test (34 теста)
./scripts/full-integration-test.sh

# С детальным выводом
DEBUG=1 ./scripts/full-integration-test.sh
```

### 3. HSM-dependent Tests

**Что тестируют**: Реальные операции с HSM (требуют инициализации)

**Пометка**: `t.Skip("Requires HSM initialization")`

**Запуск**:
```bash
# Сначала инициализируем HSM
./scripts/init-hsm.sh

# Запускаем Docker контейнер
docker compose up -d

# Запускаем integration тесты
./scripts/full-integration-test.sh
```

---

## ⚙️ Запуск тестов

### По модулям

```bash
# 1. Тесты ключевой ротации (НОВЫЕ)
go test -v ./internal/hsm/ -run Rotation

# 2. Тесты криптографии
go test -v ./internal/hsm/ -run Crypto

# 3. Тесты KeyManager + Hot Reload
go test -v ./internal/hsm/ -run KeyManager

# 4. Тесты ACL
go test -v ./internal/server/ -run ACL

# 5. Тесты HTTP handlers
go test -v ./internal/server/ -run Handler

# 6. Тесты rate limiting
go test -v ./internal/server/ -run RateLimiter

# 7. Тесты конфигурации
go test -v ./internal/config/
```

### С race detector

```bash
# Проверка на race conditions (КРИТИЧНО!)
go test -race ./...

# Только для конкретного пакета
go test -race ./internal/hsm/
go test -race ./internal/server/
```

### С timeout

```bash
# Установить timeout для тестов (по умолчанию 10m)
go test -timeout 30s ./internal/hsm/

# Для долгих integration тестов
go test -timeout 15m ./...
```

### Пропуск длинных тестов

```bash
# Быстрый режим (пропускает тесты с t.Skip и long-running)
go test -short ./...

# Проверка только критичных тестов
go test -short -race ./...
```

---

## 📊 Анализ результатов

### Интерпретация вывода

```bash
$ go test -v ./internal/hsm/

=== RUN   TestRotateKey_CreateNewVersion
    rotation_test.go:45: ✓ Successfully created new version: kek-test-v2 (v2)
--- PASS: TestRotateKey_CreateNewVersion (0.01s)

=== RUN   TestRotateKey_UpdateMetadata
    rotation_test.go:88: ✓ Metadata successfully updated with new version
--- PASS: TestRotateKey_UpdateMetadata (0.00s)

PASS
ok      github.com/titaev-lv/hsm-service/internal/hsm   0.234s
```

**Расшифровка**:
- `=== RUN` - тест запускается
- `---PASS` - тест прошёл успешно
- `(0.01s)` - время выполнения теста
- `ok ... 0.234s` - весь пакет прошёл за 0.234 секунды

### Ошибки и их анализ

```bash
# Пример ошибки
=== RUN   TestRotateKey_PreserveOldKeys
    rotation_test.go:145: Version kek-test-v1 was not preserved
--- FAIL: TestRotateKey_PreserveOldKeys (0.00s)
```

**Действия**:
1. Смотрим строку с ошибкой: `rotation_test.go:145`
2. Читаем сообщение: "Version kek-test-v1 was not preserved"
3. Открываем файл и анализируем проблему
4. Исправляем и запускаем снова

---

## 📈 Coverage (Покрытие кода)

### Базовый coverage

```bash
# Простой coverage report
go test -cover ./...

# Вывод:
ok      github.com/titaev-lv/hsm-service/internal/hsm      0.234s  coverage: 85.2% of statements
ok      github.com/titaev-lv/hsm-service/internal/server   0.156s  coverage: 75.8% of statements
ok      github.com/titaev-lv/hsm-service/internal/config   0.045s  coverage: 70.3% of statements
```

### Детальный coverage report

```bash
# 1. Генерируем coverage profile
go test -coverprofile=coverage.out ./...

# 2. Просмотр в терминале (по функциям)
go tool cover -func=coverage.out

# Вывод:
github.com/titaev-lv/hsm-service/internal/hsm/crypto.go:45:   Encrypt         85.7%
github.com/titaev-lv/hsm-service/internal/hsm/crypto.go:78:   Decrypt         92.3%
github.com/titaev-lv/hsm-service/internal/hsm/key_manager.go:120:  loadKeys   78.9%
total:                                                                (statements)  82.5%

# 3. HTML report (визуализация)
go tool cover -html=coverage.out -o coverage.html
```

### Coverage для конкретного пакета

```bash
# HSM модуль
go test -coverprofile=hsm_coverage.out ./internal/hsm/
go tool cover -html=hsm_coverage.out

# Server модуль
go test -coverprofile=server_coverage.out ./internal/server/
go tool cover -html=server_coverage.out
```

### Целевые показатели coverage

| Модуль | Текущий | Целевой | Приоритет |
|--------|---------|---------|-----------|
| `internal/hsm/` | ~85% | 95% | 🟡 MEDIUM |
| `internal/server/acl*.go` | ~95% | 95% | ✅ DONE |
| `internal/server/handlers*.go` | ~75% | 85% | 🟡 MEDIUM |
| `internal/server/middleware*.go` | ~50% | 80% | 🟡 MEDIUM |
| `internal/config/` | ~70% | 80% | 🟢 LOW |

---

## 🏃 Benchmarks (Производительность)

### Запуск бенчмарков

```bash
# Все бенчмарки
go test -bench=. ./...

# Конкретный модуль
go test -bench=. ./internal/hsm/

# С memory profiling
go test -bench=. -benchmem ./internal/hsm/

# Вывод:
BenchmarkEncryption-8           1000000    1234 ns/op    512 B/op    8 allocs/op
BenchmarkDecryption-8           1000000    1156 ns/op    512 B/op    7 allocs/op
BenchmarkRateLimiter-8         10000000     123 ns/op      0 B/op    0 allocs/op
```

**Расшифровка**:
- `BenchmarkEncryption-8` - название бенчмарка, `-8` = 8 CPU cores
- `1000000` - количество итераций
- `1234 ns/op` - время на одну операцию (наносекунды)
- `512 B/op` - память выделенная на операцию
- `8 allocs/op` - количество аллокаций памяти

### CPU профилирование

```bash
# Генерируем CPU profile
go test -bench=BenchmarkEncryption -cpuprofile=cpu.prof ./internal/hsm/

# Анализируем
go tool pprof cpu.prof

# В интерактивном режиме:
(pprof) top10          # топ 10 функций по CPU
(pprof) list Encrypt   # детальный листинг функции
(pprof) web            # визуализация (требует graphviz)
```

### Memory профилирование

```bash
# Генерируем memory profile
go test -bench=BenchmarkEncryption -memprofile=mem.prof ./internal/hsm/

# Анализируем
go tool pprof mem.prof

(pprof) top10
(pprof) list Encrypt
```

### Сравнение бенчмарков

```bash
# Запускаем бенчмарк ДО изменений
go test -bench=. ./internal/hsm/ > old.txt

# Вносим изменения...

# Запускаем бенчмарк ПОСЛЕ изменений
go test -bench=. ./internal/hsm/ > new.txt

# Сравниваем (требует установки: go install golang.org/x/perf/cmd/benchstat@latest)
benchstat old.txt new.txt

# Вывод:
name           old time/op  new time/op  delta
Encryption-8   1234ns ± 2%  1156ns ± 1%  -6.32%  (p=0.000 n=10+10)
```

---

## 🔍 Troubleshooting

### Проблема: Тесты падают с "too many open files"

**Решение**:
```bash
# Увеличить лимит file descriptors
ulimit -n 10000

# Проверить текущий лимит
ulimit -n
```

### Проблема: Race detector находит race condition

**Пример**:
```
==================
WARNING: DATA RACE
Read at 0x00c0001a2008 by goroutine 23:
  github.com/titaev-lv/hsm-service/internal/server.(*ACL).TryReload()
      /path/to/acl.go:145 +0x234
```

**Решение**:
1. Находим строку `acl.go:145`
2. Проверяем что переменная защищена mutex
3. Добавляем `mu.Lock()` / `mu.RLock()` если нужно
4. Перезапускаем тесты с `-race`

### Проблема: Тесты висят (timeout)

**Диагностика**:
```bash
# Запуск с коротким timeout
go test -timeout 10s ./internal/hsm/

# Если timeout, значит есть deadlock или бесконечный цикл
# Добавить debug логи:
t.Logf("Starting test...")
defer t.Logf("Test finished")
```

### Проблема: Coverage слишком низкий

**Анализ**:
```bash
# 1. Смотрим какие функции не покрыты
go tool cover -func=coverage.out | grep "0.0%"

# 2. Смотрим HTML report - красным показаны непокрытые строки
go tool cover -html=coverage.out

# 3. Добавляем недостающие тесты
```

### Проблема: Флакинущие (нестабильные) тесты

**Признаки**:
- Тест иногда проходит, иногда падает
- Зависит от timing/concurrency

**Решение**:
```go
// Добавить sync в тест
var wg sync.WaitGroup
wg.Add(1)
go func() {
    defer wg.Done()
    // test code
}()
wg.Wait()

// Или увеличить timeout/sleep
time.Sleep(100 * time.Millisecond)
```

---

## 🎯 Best Practices

### 1. Всегда запускайте с `-race`

```bash
# Перед каждым commit
go test -race ./...
```

### 2. Проверяйте coverage перед PR

```bash
# Минимум 80% coverage для нового кода
go test -cover ./...
```

### 3. Используйте `-short` для быстрой проверки

```bash
# Во время разработки
go test -short -race ./...
```

### 4. Запускайте полный integration тест перед merge

```bash
# Финальная проверка
./scripts/full-integration-test.sh
```

### 5. Профилируйте critical path

```bash
# Регулярно запускайте бенчмарки
go test -bench=. -benchmem ./internal/hsm/
```

---

## 📝 Примеры использования

### Пример 1: Разработка нового функционала

```bash
# 1. Написали новую функцию в crypto.go
vim internal/hsm/crypto.go

# 2. Написали тест
vim internal/hsm/crypto_test.go

# 3. Быстрая проверка
go test -short -run TestMyNewFunction ./internal/hsm/

# 4. Проверка race conditions
go test -race -run TestMyNewFunction ./internal/hsm/

# 5. Проверка coverage
go test -cover -run TestMyNewFunction ./internal/hsm/

# 6. Полная проверка
go test -v -race ./...

# 7. Integration тест
./scripts/full-integration-test.sh
```

### Пример 2: Отладка проблемы

```bash
# 1. Запускаем конкретный упавший тест с verbose
go test -v -run TestRotateKey_PreserveOldKeys ./internal/hsm/

# 2. Смотрим детали ошибки
# 3. Добавляем debug логи в тест:
t.Logf("Current keys: %+v", metadata.Keys)

# 4. Перезапускаем
go test -v -run TestRotateKey_PreserveOldKeys ./internal/hsm/

# 5. После исправления - проверка
go test -race -run TestRotateKey ./internal/hsm/
```

### Пример 3: Pre-commit проверка

```bash
#!/bin/bash
# save as .git/hooks/pre-commit

echo "Running tests..."

# Unit tests с race detector
if ! go test -short -race ./...; then
    echo "❌ Tests failed"
    exit 1
fi

# Coverage проверка
COVERAGE=$(go test -cover ./... | grep "coverage:" | awk '{sum+=$5; count++} END {print sum/count}')
if (( $(echo "$COVERAGE < 80" | bc -l) )); then
    echo "❌ Coverage too low: $COVERAGE%"
    exit 1
fi

echo "✅ All checks passed"
```

---

## 🔗 Дополнительные ресурсы

- [Go Testing Documentation](https://go.dev/doc/tutorial/add-a-test)
- [Race Detector](https://go.dev/doc/articles/race_detector)
- [Coverage Tool](https://go.dev/blog/cover)
- [Benchmarking](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- [pprof Tutorial](https://go.dev/blog/pprof)

---

## ✅ Checklist перед commit

- [ ] `go test -short -race ./...` проходит
- [ ] Coverage ≥ 80% для новых файлов
- [ ] Нет TODO тестов для critical функций
- [ ] Все новые функции имеют тесты
- [ ] Race detector clean
- [ ] Бенчмарки показывают приемлемую производительность

---

**Автор**: GitHub Copilot  
**Дата**: 2026-01-10  
**Версия**: 1.0
