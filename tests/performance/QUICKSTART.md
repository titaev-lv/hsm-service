# ⚡ Performance Testing - Quick Start

## ⚠️ ВАЖНО: ACL и Сертификаты

HSM Service использует ACL (Access Control Lists) для контроля доступа к ключам:

```yaml
# config.yaml
acl:
  mappings:
    Trading:
      - exchange-key  # OU=Trading может использовать только exchange-key
    2FA:
      - 2fa           # OU=2FA может использовать только 2fa
    Database: []
```

**Для тестов используется:**
- Сертификат: `pki/client/hsm-trading-client-1.crt`
- OU (Organizational Unit): `Trading`
- Доступные контексты: **только `exchange-key`**

❌ Если вы видите `403 - access denied: insufficient permissions` - проверьте:
1. Соответствует ли OU сертификата запрашиваемому контексту
2. Правильно ли настроены ACL mappings в config.yaml

---

```bash
cd /home/leon/docker/ct-system/hsm-service

# Запустите Docker container
docker compose up -d

# Проверьте что сервис запущен
docker ps | grep hsm-service
```

## ✅ Шаг 2: Smoke Test (ОБЯЗАТЕЛЬНО!)

```bash
# Быстрая проверка (занимает 2 секунды)
./tests/performance/smoke-test.sh
```

**Ожидаемый результат:**
```
1. Health check... ✓ OK
2. Encrypt endpoint... ✓ OK
3. Decrypt endpoint... ✓ OK

HSM Service is reachable!
```

❌ **Если smoke test падает** - НЕ запускайте другие тесты!

Возможные проблемы:
- Сервис не запущен → `docker compose up -d`
- Нет сертификатов → проверьте `pki/client/hsm-trading-client-1.{crt,key}`
- Неправильный порт → проверьте `HSM_URL` в скриптах

---

## 📊 Шаг 3: Выберите тип теста

### Option A: Quick Load Test (2 минуты) ✅ RECOMMENDED

```bash
# k6 quick test - минимальная нагрузка
k6 run tests/performance/load-test-quick.js
```

**Что тестирует:** 20 concurrent users, encrypt/decrypt операции

**Ожидаемые метрики:**
- ✅ P95 latency < 500ms
- ✅ P99 latency < 1000ms  
- ✅ Error rate < 0.1%

**Реальные результаты (SoftHSM на стандартном железе):**
```
Total Requests: 3755
Request Rate: 31.16 req/s
Failed Requests: 0.00% ✅
Avg Duration: 0.40ms ✅
P95 Duration: 0.63ms ✅ (в 800x лучше цели!)
Encrypt P95: 1.00ms ✅ (в 100x лучше цели!)
Decrypt P95: 1.00ms ✅ (в 100x лучше цели!)
Total Operations: 3572 (за 2 минуты)
```

💡 **Вывод**: HSM Service на SoftHSM работает **экстремально быстро**, превосходя все целевые метрики.

---

### Option B: Full Load Test (22 минуты)

```bash
# Полный сценарий с нарастающей нагрузкой
k6 run tests/performance/load-test.js
```

**Сценарий:**
1. Warm-up: 0 → 50 users (1 мин)
2. Ramp-up: 50 → 100 users (3 мин)
3. Steady: 100 users (5 мин)
4. Spike: 100 → 200 users (2 мин)
5. Peak: 200 users (5 мин)
6. Cool down: 200 → 50 users (3 мин)
7. Ramp down: 50 → 0 (1 мин)

---

### Option C: Stress Test (12 минут)

```bash
# Все stress сценарии с vegeta
./tests/performance/stress-test.sh
```

**Сценарии:**
1. **Incremental** - плавное увеличение 100 → 5000 req/s
2. **Sustained** - постоянная нагрузка 1000 req/s
3. **Spike** - резкий скачок до 5000 req/s
4. **Endurance** - длительная нагрузка 500 req/s

**Или отдельный тест:**
```bash
./tests/performance/stress-test.sh incremental
./tests/performance/stress-test.sh spike
```

---

### Option D: Go Benchmarks (1 минута)

```bash
# Микробенчмарки (не требует запущенного сервиса)
./tests/performance/benchmark-test.sh
```

**Что тестирует:**
- Encrypt/Decrypt производительность
- Concurrent операции
- KeyManager методы
- Rotation logic

---

## 📈 Шаг 4: Анализ результатов

### k6 Load Test

```bash
# Смотрим на ключевые метрики
grep -E "http_req_duration|error_rate" /tmp/k6-results.json

# Детальный анализ
cat results/load-test-$(date +%Y%m%d).json | jq '.metrics'
```

**Нормальные значения:**
- ✅ http_req_duration (P95): < 500ms
- ✅ http_req_duration (P99): < 1000ms
- ✅ encrypt_duration (P95): < 100ms
- ✅ decrypt_duration (P95): < 100ms
- ✅ error_rate: < 0.1%

---

### vegeta Stress Test

```bash
# Результаты сохраняются в results/
cat results/stress-incremental-*.txt

# Быстрый анализ
grep "Success" results/stress-*.txt
```

**Цели:**
- ✅ Success rate > 99%
- ✅ P99 latency < 1s
- ✅ Throughput > 1000 req/s

---

### Go Benchmarks

```bash
# Анализ производительности
cat results/benchmark-latest.txt | grep "BenchmarkEncryption"

# Сравнение с предыдущим запуском
benchstat results/benchmark-previous.txt results/benchmark-latest.txt
```

**Baseline (SoftHSM):**
- Encryption: ~290 ns/op
- Decryption: ~220 ns/op
- Concurrent: ~50 ns/op

---

## 🔧 Troubleshooting

### ❌ "HSM Service not reachable"

```bash
# 1. Проверьте что контейнер запущен
docker ps | grep hsm-service

# 2. Проверьте логи
docker logs hsm-service

# 3. Перезапустите сервис
docker compose restart hsm-service

# 4. Проверьте порт
netstat -tlnp | grep 8443
```

---

### ❌ "TLS handshake error"

```bash
# Проверьте сертификаты
ls -l pki/client/hsm-trading-client-1.crt
ls -l pki/client/hsm-trading-client-1.key

# Если нет - сгенерируйте новые
cd pki
./scripts/issue-client-cert.sh hsm-trading-client-1
```

---

### ❌ "certificate signed by unknown authority"

Это нормально для self-signed сертификатов. Скрипты используют:
- k6: `insecureSkipTLSVerify: true`
- curl: `-k` flag
- vegeta: `-insecure` flag

---

### ❌ "too many open files"

```bash
# Увеличьте лимиты
ulimit -n 65536

# Или добавьте в /etc/security/limits.conf
* soft nofile 65536
* hard nofile 65536
```

---

## 🎯 Рекомендованный workflow

**1. Перед каждым тестом:**
```bash
./tests/performance/smoke-test.sh
```

**2. Базовая проверка (быстро):**
```bash
k6 run tests/performance/load-test-quick.js
```

**3. Полная проверка (когда есть время):**
```bash
# Запустите параллельно в tmux/screen
k6 run tests/performance/load-test.js &
./tests/performance/stress-test.sh &
```

**4. Мониторинг во время теста:**
```bash
# Другой терминал
watch -n 2 docker stats hsm-service
watch -n 5 'curl -sk --cert pki/client/hsm-trading-client-1.crt --key pki/client/hsm-trading-client-1.key https://localhost:8443/metrics | grep -E "hsm_|go_"'
```

**5. После теста:**
```bash
# Проверьте что сервис всё ещё здоров
./tests/performance/smoke-test.sh

# Соберите результаты
ls -lh results/
```

---

## 📚 Полная документация

См. [README.md](README.md) для детальной информации о:
- Метриках и порогах
- Профилировании
- CI/CD интеграции
- Endurance тестировании (24h)
- Chaos engineering
