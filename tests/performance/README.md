# 🚀 Performance Testing Suite

Комплексное performance тестирование HSM Service.

## ⚠️ ВАЖНО: mTLS Authentication

**Все performance тесты требуют mTLS** (mutual TLS). HSM Service использует клиентские сертификаты для аутентификации.

### Настройка сертификатов

```bash
# Используемые сертификаты (по умолчанию)
CLIENT_CERT=pki/client/hsm-trading-client-1.crt
CLIENT_KEY=pki/client/hsm-trading-client-1.key

# Проверка что сертификаты существуют
ls -l pki/client/hsm-trading-client-1.{crt,key}

# Если нужно сгенерировать новые
cd pki
./scripts/issue-client-cert.sh my-client
```

**Перед любым тестом:**
1. ✅ Запустите сервис: `docker compose up -d`
2. ✅ Проверьте доступность: `./tests/performance/smoke-test.sh`
3. ✅ Убедитесь что сертификаты валидны

---

## 📋 Содержание

0. **Smoke Test** - Проверка работоспособности сервиса
1. **Go Benchmarks** - Микробенчмарки функций
2. **Load Testing** - k6 нагрузочное тестирование
3. **Stress Testing** - vegeta тесты на пределе возможностей
4. **Endurance Testing** - долгосрочная стабильность

---

## 🔍 0. Smoke Test (Обязательная проверка)

### Быстрая проверка работоспособности

```bash
# Простейшая проверка (3 теста: health, encrypt, decrypt)
./tests/performance/smoke-test.sh

# С кастомными сертификатами
CLIENT_CERT=pki/client/custom.crt CLIENT_KEY=pki/client/custom.key ./tests/performance/smoke-test.sh
```

**Что проверяется:**
- ✓ Health endpoint доступен
- ✓ Encrypt endpoint работает
- ✓ Decrypt правильно расшифровывает данные
- Docker stats (CPU, memory usage)

**Пример вывода:**
```
1. Health check... ✓ OK
2. Encrypt endpoint... ✓ OK
3. Decrypt endpoint... ✓ OK

HSM Service is reachable!

HSM Service Docker Stats:
CONTAINER     CPU %     MEM USAGE / LIMIT     MEM %
hsm-service   0.00%     12.11MiB / 512MiB     2.37%
```

⚠️ **Если smoke test падает** - не запускайте другие тесты, сначала исправьте проблему!

---

## 🔬 1. Go Benchmarks

### Запуск

```bash
# Базовый запуск
go test ./internal/hsm/... -bench=. -benchmem

# С помощью скрипта (рекомендуется)
./tests/performance/benchmark-test.sh

# С кастомными параметрами
BENCH_TIME=5s BENCH_COUNT=10 ./tests/performance/benchmark-test.sh
```

**Примечание**: Go benchmarks не требуют запущенного HSM service (используют моки).

### Текущие бенчмарки

| Benchmark | ns/op | B/op | allocs/op |
|-----------|-------|------|-----------|
| BenchmarkEncryption | ~296 ns | 144 B | 3 |
| BenchmarkDecryption | ~218 ns | 96 B | 2 |
| BenchmarkConcurrentEncryption | ~49 ns | 112 B | 3 |
| BenchmarkBuildAAD | ~95 ns | 32 B | 1 |
| BenchmarkKeyManagerEncrypt | ~339 ns | 160 B | 3 |
| BenchmarkKeyManagerDecrypt | ~248 ns | 112 B | 2 |
| BenchmarkKeyManagerConcurrent | ~117 ns | 176 B | 5 |
| BenchmarkNeedsRotation | ~42 ns | 0 B | 0 |

### Профилирование

```bash
# CPU профиль
go test ./internal/hsm/... -bench=BenchmarkEncryption -cpuprofile=cpu.prof
go tool pprof cpu.prof

# Memory профиль
go test ./internal/hsm/... -bench=BenchmarkEncryption -memprofile=mem.prof
go tool pprof mem.prof

# Trace профиль
go test ./internal/hsm/... -bench=BenchmarkEncryption -trace=trace.out
go tool trace trace.out
```

---

## 📊 2. Load Testing (k6)

### Установка k6

```bash
# Ubuntu/Debian
sudo apt install k6

# macOS
brew install k6

# Or from source
go install go.k6.io/k6@latest
```

### ⚠️ ВАЖНО: Требования

**HSM Service требует mTLS (mutual TLS)**. Скрипты настроены на использование клиентских сертификатов:

```bash
# Сертификаты по умолчанию
CLIENT_CERT=pki/client/hsm-trading-client-1.crt
CLIENT_KEY=pki/client/hsm-trading-client-1.key

# Если нужны другие сертификаты
CLIENT_CERT=pki/client/custom.crt CLIENT_KEY=pki/client/custom.key k6 run tests/performance/load-test.js
```

### Запуск

```bash
# 1. Проверьте что HSM service запущен
./tests/performance/smoke-test.sh

# 2. Quick smoke test (2 минуты вместо 22)
k6 run tests/performance/load-test-quick.js

# 3. Полный load test (22 минуты)
k6 run tests/performance/load-test.js

# С кастомными параметрами
HSM_URL=https://localhost:8443 k6 run tests/performance/load-test.js

# С выводом в InfluxDB/Grafana
k6 run --out influxdb=http://localhost:8086/k6 tests/performance/load-test.js
```

**ВАЖНО**: Перед запуском убедитесь что:
1. HSM service запущен: `docker compose up -d`
2. Клиентские сертификаты существуют в `pki/client/`
3. Сервис доступен: `./tests/performance/smoke-test.sh`

### Сценарий нагрузки

1. **Warm-up** (1 min): 0 → 50 пользователей
2. **Ramp-up** (3 min): 50 → 100 пользователей
3. **Steady state** (5 min): 100 пользователей
4. **Spike** (2 min): 100 → 200 пользователей
5. **Peak load** (5 min): 200 пользователей
6. **Cool down** (3 min): 200 → 50 пользователей
7. **Ramp down** (1 min): 50 → 0

**Total duration**: ~22 минуты

### Метрики и пороги

| Метрика | Цель | Критично |
|---------|------|----------|
| P95 latency | < 500ms | < 1000ms |
| P99 latency | < 1000ms | < 2000ms |
| Error rate | < 0.1% | < 1% |
| Encrypt P95 | < 100ms | < 200ms |
| Decrypt P95 | < 100ms | < 200ms |

---

## 💪 3. Stress Testing (vegeta)

### Установка vegeta

```bash
# Если есть Go
go install github.com/tsenart/vegeta@latest

# Или скачать бинарник (Ubuntu/Debian)
wget https://github.com/tsenart/vegeta/releases/download/v12.11.1/vegeta_12.11.1_linux_amd64.tar.gz
tar xzf vegeta_12.11.1_linux_amd64.tar.gz
sudo mv vegeta /usr/local/bin/

# Из репозитория (может быть устаревшая версия)
sudo apt install vegeta

# macOS
brew install vegeta

# Проверка установки
vegeta -version
```

### ⚠️ ВАЖНО: Требования

**Vegeta также требует mTLS**. Скрипты используют клиентские сертификаты:

```bash
# Сертификаты по умолчанию
CLIENT_CERT=pki/client/hsm-trading-client-1.crt
CLIENT_KEY=pki/client/hsm-trading-client-1.key

# Если нужны другие сертификаты
CLIENT_CERT=pki/client/custom.crt CLIENT_KEY=pki/client/custom.key ./tests/performance/stress-test.sh
```

### Запуск

```bash
# 1. Проверьте что HSM service запущен
./tests/performance/smoke-test.sh

# 2. Полный стресс-тест (все 4 сценария, ~12 минут)
./tests/performance/stress-test.sh

# Запуск отдельного сценария
./tests/performance/stress-test.sh incremental
./tests/performance/stress-test.sh sustained
./tests/performance/stress-test.sh spike
./tests/performance/stress-test.sh endurance

# С кастомным URL
HSM_URL=https://localhost:8443 ./tests/performance/stress-test.sh
```

**Сценарии:**
1. **Incremental** (2 мин): Плавное увеличение нагрузки 100 → 5000 req/s
2. **Sustained** (2 мин): Постоянная нагрузка 1000 req/s
3. **Spike** (3 мин): Резкий скачок до 5000 req/s
4. **Endurance** (5 мин): Длительная нагрузка 500 req/s

### Анализ результатов

```bash
# Просмотр текстового отчета
cat stress-results/sustained-high.txt

# HTML график
open stress-results/sustained-high.html

# Детальный анализ
vegeta report stress-results/sustained-high.bin

# Histogram
vegeta report -type=hist[0,100ms,200ms,300ms] stress-results/sustained-high.bin
```

---

## ⏱️ 4. Endurance Testing

Долгосрочный тест для выявления утечек памяти, goroutine leaks и деградации производительности.

### Запуск

```bash
# 24-часовой тест
DURATION=24h ./tests/performance/stress-test.sh

# Или вручную с Apache Bench
ab -n 864000 -c 10 -t 86400 \
   -T 'application/json' \
   -p encrypt-payload.json \
   https://localhost:8443/encrypt
```

### Мониторинг во время теста

```bash
# Docker stats (каждые 5 секунд)
watch -n 5 docker stats hsm-service

# Memory tracking
watch -n 10 'docker exec hsm-service ps aux | grep hsm-service'

# Goroutine count
watch -n 30 'curl -s http://localhost:8443/metrics | grep go_goroutines'
```

### Проверки

- ✅ Стабильное использование памяти (no growth)
- ✅ Стабильное количество goroutines (no leaks)
- ✅ Стабильная latency (no degradation)
- ✅ Zero errors
- ✅ No file descriptor leaks

---

## 🎯 Performance Targets

### Latency Targets

| Operation | P50 | P95 | P99 |
|-----------|-----|-----|-----|
| Encrypt | < 50ms | < 100ms | < 200ms |
| Decrypt | < 50ms | < 100ms | < 200ms |
| Health check | < 5ms | < 10ms | < 20ms |

### Throughput Targets

| Metric | Target | Stretch Goal |
|--------|--------|--------------|
| Requests/sec | > 1,000 | > 5,000 |
| Concurrent users | 200 | 500 |
| Error rate | < 0.1% | < 0.01% |

### Resource Usage Targets

| Resource | Normal Load | Peak Load |
|----------|-------------|-----------|
| CPU | < 50% | < 80% |
| Memory | < 256MB | < 512MB |
| Goroutines | < 100 | < 500 |

---

## 📈 Continuous Performance Testing

### CI/CD Integration

```yaml
# .github/workflows/performance.yml
name: Performance Tests

on:
  push:
    branches: [main]
  schedule:
    - cron: '0 2 * * *'  # Daily at 2 AM

jobs:
  benchmark:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
      - name: Run benchmarks
        run: ./tests/performance/benchmark-test.sh
      - name: Upload results
        uses: actions/upload-artifact@v3
        with:
          name: benchmark-results
          path: benchmark-results/
```

---

## 🔍 Troubleshooting

### Low throughput

1. Проверьте rate limiter настройки
2. Увеличьте workers/goroutines
3. Профилируйте CPU: `go tool pprof cpu.prof`

### High latency

1. Проверьте HSM performance
2. Профилируйте memory: `go tool pprof mem.prof`
3. Проверьте network latency

### Memory leaks

1. Запустите с `-memprofile`
2. Используйте `go tool pprof -alloc_space`
3. Проверьте goroutine leaks: `curl /debug/pprof/goroutine`

### Error rate spikes

1. Проверьте logs: `docker logs hsm-service`
2. Проверьте HSM health
3. Проверьте сертификаты и ACL

---

## 📚 Resources

- [k6 Documentation](https://k6.io/docs/)
- [vegeta Documentation](https://github.com/tsenart/vegeta)
- [Go Benchmarking](https://dave.cheney.net/2013/06/30/how-to-write-benchmarks-in-go)
- [pprof Guide](https://blog.golang.org/pprof)

---

**Last Updated**: 2026-01-11  
**Version**: 1.0
