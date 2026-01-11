# План оптимизации производительности HSM Service
**Дата:** 12 января 2026  
**Версия:** 1.0

---

## 📊 Сравнение результатов тестирования

### **Rate Limiter - важная деталь:**
**Тип:** PER-CLIENT (по Certificate CN), НЕ глобальный!  
**Код:** `internal/server/middleware.go:103-110`
```go
func (rl *RateLimiter) GetLimiter(clientCN string) *rate.Limiter {
    // Создает ОТДЕЛЬНЫЙ лимитер для КАЖДОГО clientCN
    limiter: rate.NewLimiter(rate.Limit(rl.rps), rl.burst)
}
```
**Вывод:** Каждый клиент (по mTLS сертификату) имеет свой лимит 50000 req/s + 5000 burst.  
В тестах vegeta использует **ОДИН** клиентский сертификат → один лимитер.

---

## 📈 Результаты: До vs После

### **Конфигурация:**

| Параметр | ТЕСТ 1 (старый) | ТЕСТ 2 (новый) |
|----------|----------------|----------------|
| `requests_per_second` | 50000 | 50000 |
| `burst` | **50** | **5000** |

---

### **1. Incremental Load Test**

#### **100 req/s:**
| Метрика | ТЕСТ 1 | ТЕСТ 2 | Изменение |
|---------|--------|--------|-----------|
| Success | 100% | 100% | ✅ Без изменений |
| Throughput | 100.03 req/s | 100.03 req/s | = |
| P95 Latency | 430 µs | 428 µs | -0.5% |

#### **500 req/s:**
| Метрика | ТЕСТ 1 | ТЕСТ 2 | Изменение |
|---------|--------|--------|-----------|
| Success | 100% | 100% | ✅ Без изменений |
| Throughput | 500.02 req/s | 500.02 req/s | = |
| P95 Latency | 343 µs | 342 µs | -0.3% |

#### **1000 req/s:**
| Метрика | ТЕСТ 1 | ТЕСТ 2 | Изменение |
|---------|--------|--------|-----------|
| Success | **50.13%** | **100%** | ✅ +99.7% |
| Throughput | 501 req/s | **1000 req/s** | ✅ +99.4% |
| P95 Latency | 337 µs | 373 µs | +10.7% |
| Errors | 429 (14962) | **0** | ✅ Устранены |

**Вывод:** Увеличение `burst` с 50 до 5000 **полностью устранило** проблему при 1000 req/s!

#### **2000 req/s (новый тест):**
| Метрика | ТЕСТ 2 |
|---------|--------|
| Success | **68.49%** |
| Throughput | **1195 req/s** |
| P95 Latency | **20.3 секунд** ⚠️ |
| Errors | 429 (4802), EOF (14102), stream errors |

**Breaking point переместился:** 500 req/s → **~1200 req/s** (рост в 2.4x)

---

### **2. Sustained High Load (60s @ 1000 req/s)**

| Метрика | ТЕСТ 1 | ТЕСТ 2 | Изменение |
|---------|--------|--------|-----------|
| Success | 50.04% | **Не запущен** | - |
| Throughput | 500.37 req/s | - | - |
| Errors | 429 (29978) | - | - |

---

### **3. Spike Test (10s @ 5000 req/s)**

| Метрика | ТЕСТ 1 | ТЕСТ 2 | Изменение |
|---------|--------|--------|-----------|
| Success | **5.49%** | **62.11%** | ✅ +1031% |
| Throughput | 68.83 req/s | **789 req/s** | ✅ +1046% |
| P95 Latency | 30 секунд | **15.9 секунд** | ✅ -47% |
| P99 Latency | 30 секунд | **23 секунды** | ✅ -23% |
| Errors (429) | 32291 | **1306** | ✅ -96% |
| Errors (timeout/0) | 14831 | **17627** | ⚠️ +18.8% |

**Вывод:** Кардинальное улучшение! Success вырос с 5.5% до 62%. Но остаются проблемы:
- EOF, connection reset
- bind: address already in use (port exhaustion на клиенте)
- HTTP/2 stream errors

---

### **4. Endurance Test (5 min @ 100 req/s)**

| Метрика | ТЕСТ 1 | ТЕСТ 2 | Изменение |
|---------|--------|--------|-----------|
| Success | **84.40%** | **83.48%** | -1.1% |
| Throughput | 84.40 req/s | 83.48 req/s | -1.1% |
| Errors (port exhaustion) | Да | **Да** | ⚠️ Не устранено |
| Errors (connection reset) | Да | Да | ⚠️ Не устранено |

**Вывод:** `burst` не влияет на длительные тесты. Проблема в **network stack**, не в rate limiter.

---

## 🎯 Выявленные проблемы

### **✅ РЕШЕНО:**
1. ~~Rate limiting при 1000 req/s~~ → увеличен `burst` до 5000
2. ~~Spike test провал (5.5% success)~~ → улучшение до 62%

### **⚠️ ЧАСТИЧНО РЕШЕНО:**
3. **Breaking point повысился:** 500 → 1200 req/s (+140%)
4. **Spike latency снизилась:** P95 30s → 15.9s (-47%)

### **🔴 НЕ РЕШЕНО:**
5. **HTTP/2 stream errors** при высокой нагрузке (2000+ req/s)
6. **Port exhaustion** на клиенте (vegeta): `bind: address already in use`
7. **Connection resets** при spike/endurance тестах
8. **EOF errors** при резких нагрузках
9. **P95 latency 20+ секунд** при 2000 req/s

---

## 📋 ПЛАН ДОРАБОТОК И НАСТРОЕК

### **ФАЗА 1: HTTP/2 Optimization (Высокий приоритет)**
**Цель:** Устранить stream errors, снизить latency при 2000+ req/s  
**Ожидаемый эффект:** Breaking point 1200 → 2500+ req/s

#### 1.1. Добавить HTTP/2 конфигурацию в config.yaml
```yaml
server:
  http2:
    max_concurrent_streams: 500        # Default ~100-250
    initial_window_size: 2097152       # 2 MB (default 64KB)
    max_frame_size: 1048576            # 1 MB (default 16KB)
    max_header_list_size: 1048576      # 1 MB
    idle_timeout_seconds: 120          # 2 минуты
    max_upload_buffer_per_conn: 2097152  # 2 MB
```

#### 1.2. Обновить internal/config/types.go
```go
type HTTP2Config struct {
    MaxConcurrentStreams     uint32 `yaml:"max_concurrent_streams"`
    InitialWindowSize        int32  `yaml:"initial_window_size"`
    MaxFrameSize             uint32 `yaml:"max_frame_size"`
    MaxHeaderListSize        uint32 `yaml:"max_header_list_size"`
    IdleTimeoutSeconds       int    `yaml:"idle_timeout_seconds"`
    MaxUploadBufferPerConn   int32  `yaml:"max_upload_buffer_per_conn"`
}

type ServerConfig struct {
    Port   string      `yaml:"port"`
    TLS    TLSConfig   `yaml:"tls"`
    HTTP2  HTTP2Config `yaml:"http2"`  // НОВОЕ
}
```

#### 1.3. Обновить internal/server/server.go
```go
import "golang.org/x/net/http2"

// В NewServer():
http2Cfg := &http2.Server{
    MaxConcurrentStreams:         cfg.HTTP2.MaxConcurrentStreams,
    InitialConnWindowSize:        cfg.HTTP2.InitialWindowSize,
    InitialStreamWindowSize:      cfg.HTTP2.InitialWindowSize,
    MaxReadFrameSize:             cfg.HTTP2.MaxFrameSize,
    MaxHeaderListSize:            cfg.HTTP2.MaxHeaderListSize,
    IdleTimeout:                  time.Duration(cfg.HTTP2.IdleTimeoutSeconds) * time.Second,
    MaxUploadBufferPerConnection: cfg.HTTP2.MaxUploadBufferPerConn,
    MaxUploadBufferPerStream:     cfg.HTTP2.MaxUploadBufferPerConn,
}

if err := http2.ConfigureServer(httpServer, http2Cfg); err != nil {
    return nil, fmt.Errorf("failed to configure HTTP/2: %w", err)
}
```

#### 1.4. Обновить go.mod
```bash
go get golang.org/x/net/http2
```

**Тестирование:**
- Запустить spike test (5000 req/s) → ожидаем success >80%
- Проверить отсутствие stream errors в логах
- Incremental test до 3000 req/s → найти новый breaking point

---

### **ФАЗА 2: Network Stack Tuning (Средний приоритет)**
**Цель:** Устранить port exhaustion, connection resets  
**Ожидаемый эффект:** Endurance test 83% → 95%+ success

#### 2.1. Kernel tuning на хосте (/etc/sysctl.conf)
```bash
# Connection handling
net.core.somaxconn = 8192
net.ipv4.tcp_max_syn_backlog = 8192
net.netfilter.nf_conntrack_max = 524288

# Port range и reuse
net.ipv4.ip_local_port_range = 1024 65535
net.ipv4.tcp_tw_reuse = 1              # Критично для port exhaustion!
net.ipv4.tcp_fin_timeout = 15

# Buffer sizes для HTTP/2
net.core.rmem_max = 16777216
net.core.wmem_max = 16777216
net.ipv4.tcp_rmem = 4096 87380 16777216
net.ipv4.tcp_wmem = 4096 65536 16777216

# Connection tracking
net.ipv4.tcp_keepalive_time = 600
net.ipv4.tcp_keepalive_intvl = 10
net.ipv4.tcp_keepalive_probes = 3
```

**Применить:**
```bash
sudo sysctl -p
```

#### 2.2. Docker resource limits (docker-compose.yml)
```yaml
services:
  hsm-service:
    deploy:
      resources:
        limits:
          cpus: '4.0'
          memory: 4G
        reservations:
          cpus: '2.0'
          memory: 2G
    ulimits:
      nofile:
        soft: 65536
        hard: 65536
      nproc:
        soft: 65536
        hard: 65536
```

**Тестирование:**
- Endurance test (5 min @ 100 req/s) → ожидаем 0 port exhaustion errors
- Spike test → проверить connection resets

---

### **ФАЗА 3: Rate Limiter Enhancements (Низкий приоритет)**
**Цель:** Добавить глобальный лимит + мониторинг  
**Ожидаемый эффект:** Защита от DDoS, видимость нагрузки

#### 3.1. Добавить глобальный rate limiter
**Текущее:** только per-client (по CN)  
**Нужно:** глобальный лимит для всех клиентов + per-client

```yaml
rate_limit:
  # Per-client limits (существующие)
  requests_per_second: 5000    # Реалистичное значение
  burst: 10000
  
  # Новые: глобальные лимиты
  global_requests_per_second: 10000
  global_burst: 20000
```

#### 3.2. Обновить middleware.go
```go
type RateLimiter struct {
    limiters      map[string]*limiterEntry
    globalLimiter *rate.Limiter  // НОВОЕ
    mu            sync.Mutex
    rps           int
    burst         int
}

func NewRateLimiter(rps, burst, globalRps, globalBurst int) *RateLimiter {
    return &RateLimiter{
        limiters:      make(map[string]*limiterEntry),
        globalLimiter: rate.NewLimiter(rate.Limit(globalRps), globalBurst),
        rps:           rps,
        burst:         burst,
    }
}

// В RateLimitMiddleware:
// Сначала проверить глобальный лимит
if !limiter.globalLimiter.Allow() {
    slog.Warn("global rate limit exceeded")
    respondError(w, http.StatusTooManyRequests, "service overloaded")
    return
}

// Затем per-client
clientLimiter := limiter.GetLimiter(clientCN)
if !clientLimiter.Allow() {
    // ... существующий код
}
```

**Тестирование:**
- 10 параллельных vegeta клиентов (каждый 1000 req/s) → глобальный лимит срабатывает

---

### **ФАЗА 4: Monitoring & Metrics (Низкий приоритет)**
**Цель:** Видимость производительности в реальном времени

#### 4.1. Добавить метрики в internal/server/metrics.go
```go
var (
    // HTTP/2 метрики
    http2StreamsActive = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "hsm_http2_streams_active",
        Help: "Active HTTP/2 streams",
    })
    
    http2StreamErrors = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "hsm_http2_stream_errors_total",
        Help: "Total HTTP/2 stream errors",
    })
    
    // Rate limiter метрики
    rateLimitHitsPerClient = prometheus.NewCounterVec(
        prometheus.CounterOpts{
            Name: "hsm_rate_limit_hits_total",
            Help: "Rate limit hits by client CN",
        },
        []string{"client_cn"},
    )
    
    rateLimitGlobalHits = prometheus.NewCounter(prometheus.CounterOpts{
        Name: "hsm_rate_limit_global_hits_total",
        Help: "Global rate limit hits",
    })
    
    // Connection метрики
    activeConnections = prometheus.NewGauge(prometheus.GaugeOpts{
        Name: "hsm_active_connections",
        Help: "Current active connections",
    })
    
    // Latency histogram
    requestDuration = prometheus.NewHistogramVec(
        prometheus.HistogramOpts{
            Name:    "hsm_request_duration_seconds",
            Help:    "Request duration in seconds",
            Buckets: []float64{0.0001, 0.0005, 0.001, 0.005, 0.01, 0.05, 0.1, 0.5, 1, 5, 10, 30},
        },
        []string{"endpoint"},
    )
)
```

#### 4.2. Endpoint /metrics
```go
// В server.go:
mux.Handle("/metrics", promhttp.Handler())
```

**Тестирование:**
- Grafana dashboard для мониторинга
- Алерты на spike latency >1s

---

### **ФАЗА 5: Load Balancing Preparation (Будущее)**
**Цель:** Горизонтальное масштабирование  
**Примечание:** Не для текущей версии, но важно учесть в архитектуре

#### 5.1. Требования для multi-instance:
- Shared metadata.yaml через volume (уже есть ✅)
- Session affinity в load balancer (mTLS CN-based)
- Health check endpoint (/health уже есть ✅)
- Graceful shutdown (нужно улучшить)

#### 5.2. Архитектура:
```
               ┌─────────────┐
               │   HAProxy   │
               │  (mTLS LB)  │
               └──────┬──────┘
                      │
        ┌─────────────┼─────────────┐
        │             │             │
   ┌────▼────┐   ┌────▼────┐   ┌────▼────┐
   │  HSM 1  │   │  HSM 2  │   │  HSM 3  │
   └────┬────┘   └────┬────┘   └────┬────┘
        │             │             │
        └─────────────┼─────────────┘
                      │
              ┌───────▼────────┐
              │ Shared Volume  │
              │ metadata.yaml  │
              └────────────────┘
```

---

## 🧪 План тестирования после доработок

### **Критерии успеха:**

| Тест | Нагрузка | Целевой Success Rate | Целевая Latency P95 |
|------|----------|----------------------|---------------------|
| Incremental | 100 req/s | 100% | <500 µs |
| Incremental | 500 req/s | 100% | <500 µs |
| Incremental | 1000 req/s | 100% | <1 ms |
| Incremental | 2000 req/s | **>95%** | **<5 ms** |
| Incremental | 3000 req/s | **>90%** | **<10 ms** |
| Sustained | 1000 req/s (60s) | **>99%** | <2 ms |
| Sustained | 2000 req/s (60s) | **>95%** | <5 ms |
| Spike | 5000 req/s (10s) | **>80%** | **<10 s** |
| Spike | 10000 req/s (10s) | **>50%** | <20 s |
| Endurance | 100 req/s (5 min) | **>98%** | <1 ms |

### **Метрики для мониторинга:**

1. **Success Rate** (основная метрика)
2. **Throughput** (реальная vs запрашиваемая)
3. **Latency** (P50, P95, P99, Max)
4. **Errors:**
   - 429 Too Many Requests
   - HTTP/2 stream errors
   - Connection resets
   - Port exhaustion
   - Timeouts
5. **Resource usage:**
   - CPU %
   - Memory MB
   - Open file descriptors
   - Goroutines count

---

## 📅 Roadmap

### **Sprint 1 (Неделя 1-2):**
- ✅ Фаза 1: HTTP/2 Optimization
- Тестирование incremental + spike
- Анализ результатов

### **Sprint 2 (Неделя 3):**
- ✅ Фаза 2: Network Stack Tuning
- Тестирование endurance + sustained
- Документирование изменений

### **Sprint 3 (Неделя 4):**
- ✅ Фаза 3: Rate Limiter Enhancements (опционально)
- ✅ Фаза 4: Monitoring & Metrics
- Финальное регрессионное тестирование

### **Будущее:**
- Фаза 5: Load Balancing (при необходимости масштабирования >10K req/s)

---

## 💡 Рекомендации по эксплуатации

### **Текущие настройки (PROD-ready):**
```yaml
rate_limit:
  requests_per_second: 5000    # Консервативно для per-client
  burst: 10000                 # 2x запас для всплесков
```

### **После Фазы 1 (HTTP/2):**
```yaml
rate_limit:
  requests_per_second: 10000   # Увеличить после тестов
  burst: 20000
  
server:
  http2:
    max_concurrent_streams: 500
    initial_window_size: 2097152  # 2 MB
```

### **Production checklist:**
- [ ] Kernel tuning применен (`sysctl -p`)
- [ ] Docker ulimits настроены (nofile=65536)
- [ ] HTTP/2 конфигурация активна
- [ ] Prometheus метрики экспортируются
- [ ] Grafana dashboards настроены
- [ ] Алерты на latency >1s и success <95%
- [ ] Regression tests проходят (все 42 integration tests)
- [ ] Performance tests показывают улучшение

---

## 📌 Ключевые выводы

### **Достижения:**
1. ✅ Увеличение `burst: 50 → 5000` устранило проблему при 1000 req/s
2. ✅ Breaking point вырос: **500 → 1200 req/s** (+140%)
3. ✅ Spike test success: **5.5% → 62%** (+1031%)
4. ✅ Spike throughput: **68 → 789 req/s** (+1046%)

### **Оставшиеся проблемы:**
1. ⚠️ HTTP/2 stream errors при 2000+ req/s
2. ⚠️ Port exhaustion на клиенте vegeta
3. ⚠️ Connection resets при spike нагрузке
4. ⚠️ P95 latency 20+ секунд при 2000 req/s

### **Следующий шаг:**
**Внедрить Фазу 1 (HTTP/2 Optimization)** - ожидаем breaking point 1200 → 2500+ req/s.

---

**Автор:** HSM Service Performance Team  
**Версия документа:** 1.0  
**Последнее обновление:** 12 января 2026
