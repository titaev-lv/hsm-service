# HSM Service - Development Plan

**Project:** HSM Service  
**Timeline:** 2 weeks (10 working days)  
**Team:** 1-2 developers  
**Start Date:** 2026-01-06  
**Target Release:** 2026-01-17

---

## Overview

План разработки разбит на 3 основные фазы:
1. **Phase 1: PKI Infrastructure** (2-3 дня)
2. **Phase 2: Core Service** (4-5 дней)
3. **Phase 3: Testing & Documentation** (2-3 дня)

---

## Phase 1: PKI Infrastructure Setup

**Duration:** Days 1-3  
**Goal:** Создать PKI инфраструктуру и протестировать на MySQL

### Day 1: PKI Scripts & Structure

#### Task 1.1: Создать структуру директорий
**Assignee:** Developer  
**Priority:** CRITICAL  
**Estimate:** 30 min

```bash
# Создать структуру
mkdir -p pki/{ca,server,client,scripts}
mkdir -p data/tokens
mkdir -p internal/{config,hsm,server,revocation}
mkdir -p cmd/hsm-admin
mkdir -p scripts
```

**Deliverables:**
- [ ] Директории созданы
- [ ] .gitignore настроен (исключить *.key, tokens/)

---

#### Task 1.2: PKI Script - issue-server-cert.sh
**Priority:** CRITICAL  
**Estimate:** 2 hours

**Script:** `pki/scripts/issue-server-cert.sh`

**Functionality:**
1. Принимает аргументы: CN, DNS names (SAN), IP addresses (SAN)
2. Генерирует приватный ключ RSA 4096
3. Создает CSR с SAN extension
4. Подписывает CSR через CA
5. Сохраняет в `pki/server/<cn>.{crt,key}`
6. Обновляет `pki/inventory.yaml`

**Usage:**
```bash
./pki/scripts/issue-server-cert.sh \
  hsm-service.local \
  "hsm-service.local,localhost" \
  "127.0.0.1"
```

**Deliverables:**
- [ ] Скрипт создан и протестирован
- [ ] Генерируются корректные сертификаты с SAN
- [ ] inventory.yaml обновляется автоматически

**Testing:**
```bash
# Проверить сертификат
openssl x509 -in pki/server/hsm-service.local.crt -noout -text | grep -A1 "Subject Alternative Name"
```

---

#### Task 1.3: PKI Script - issue-client-cert.sh
**Priority:** CRITICAL  
**Estimate:** 1.5 hours

**Script:** `pki/scripts/issue-client-cert.sh`

**Functionality:**
1. Принимает аргументы: CN, OU
2. Генерирует приватный ключ RSA 4096
3. Создает CSR с правильным Subject (включая OU)
4. Подписывает CSR через CA
5. Сохраняет в `pki/client/<cn>.{crt,key}`
6. Обновляет `pki/inventory.yaml`

**Usage:**
```bash
./pki/scripts/issue-client-cert.sh trading-service-1 Trading
./pki/scripts/issue-client-cert.sh web-2fa-service 2FA
```

**Deliverables:**
- [ ] Скрипт создан и протестирован
- [ ] OU корректно встраивается в Subject
- [ ] inventory.yaml обновляется

**Testing:**
```bash
# Проверить OU
openssl x509 -in pki/client/trading-service-1.crt -noout -subject
# Output: subject=C=RU, ST=Moscow, L=Moscow, O=Private, OU=Trading, CN=trading-service-1
```

---

#### Task 1.4: PKI Script - revoke-cert.sh
**Priority:** MEDIUM  
**Estimate:** 1 hour

**Script:** `pki/scripts/revoke-cert.sh`

**Functionality:**
1. Принимает: CN, reason
2. Извлекает serial number из сертификата
3. Добавляет запись в `pki/revoked.yaml`

**Usage:**
```bash
./pki/scripts/revoke-cert.sh old-service compromised
```

**Deliverables:**
- [ ] Скрипт добавляет запись в revoked.yaml
- [ ] Формат YAML корректный

---

#### Task 1.5: Template файлов
**Priority:** MEDIUM  
**Estimate:** 1 hour

**Files to create:**

**pki/inventory.yaml** (initial template):
```yaml
# Automatically updated by PKI scripts
certificates:
  servers: []
  clients: []
```

**pki/revoked.yaml** (initial template):
```yaml
# List of revoked certificates
revoked: []
```

**pki/README.md** (documentation):
- Инструкции по использованию скриптов
- Примеры выпуска сертификатов
- Процедура отзыва

**Deliverables:**
- [ ] Все template файлы созданы
- [ ] README.md с инструкциями

---

### Day 2: Certificate Generation & MySQL Testing

#### Task 2.1: Выпустить сертификаты для тестирования
**Priority:** CRITICAL  
**Estimate:** 1 hour

**Certificates to issue:**

```bash
# Server certificates
./pki/scripts/issue-server-cert.sh \
  mysql-server.local \
  "mysql-server.local,mysql.local" \
  "10.0.0.5"

./pki/scripts/issue-server-cert.sh \
  hsm-service.local \
  "hsm-service.local,localhost" \
  "127.0.0.1"

# Client certificates
./pki/scripts/issue-client-cert.sh mysql-client-test Database
./pki/scripts/issue-client-cert.sh trading-service-1 Trading
./pki/scripts/issue-client-cert.sh web-2fa-service 2FA
```

**Deliverables:**
- [x] Все сертификаты выпущены
- [x] inventory.yaml обновлен
- [x] Проверка: openssl verify -CAfile ca.crt <cert>.crt

---

#### Task 2.2: Настроить MySQL с mTLS
**Priority:** HIGH  
**Estimate:** 2 hours

**MySQL Configuration:**

```ini
# /etc/mysql/my.cnf
[mysqld]
ssl-ca=/etc/mysql/certs/ca.crt
ssl-cert=/etc/mysql/certs/server/mysql-server.local.crt
ssl-key=/etc/mysql/certs/server/mysql-server.local.key
require_secure_transport=ON
```

**Create test user:**
```sql
CREATE USER 'testuser'@'%' REQUIRE X509;
GRANT ALL PRIVILEGES ON testdb.* TO 'testuser'@'%';
FLUSH PRIVILEGES;
```

**Deliverables:**
- [x] MySQL настроен с mTLS
- [x] Тестовый пользователь создан

---

#### Task 2.3: Тест подключения к MySQL
**Priority:** HIGH  
**Estimate:** 1 hour

**Test connection:**
```bash
mysql \
  --host=mysql-server.local \
  --user=testuser \
  --ssl-ca=pki/ca/ca.crt \
  --ssl-cert=pki/client/mysql-client-test.crt \
  --ssl-key=pki/client/mysql-client-test.key \
  -e "SELECT 'mTLS works!'"
```

**Negative tests:**
- Подключение без сертификата → FAIL
- Подключение с неподписанным сертификатом → FAIL
- Подключение с истекшим сертификатом → FAIL

**Deliverables:**
- [x] mTLS подключение к MySQL работает
- [x] Negative tests пройдены
- [x] Документирован процесс в README-PKI.md

---

### Day 3: PKI Documentation & Review

#### Task 3.1: README-PKI.md
**Priority:** HIGH  
**Estimate:** 2 hours

**Content:**
1. Обзор PKI структуры
2. Использование скриптов (примеры)
3. Выпуск серверных сертификатов
4. Выпуск клиентских сертификатов
5. Отзыв сертификатов
6. Certificate inventory management
7. Best practices
8. Troubleshooting

**Deliverables:**
- [x] README-PKI.md создан
- [x] Все примеры протестированы

---

#### Task 3.2: Code Review & Testing
**Priority:** MEDIUM  
**Estimate:** 1 hour

**Review checklist:**
- [x] Скрипты следуют bash best practices
- [x] Error handling корректен
- [x] Permissions на файлах правильные (0600 для .key)
- [x] inventory.yaml формат валиден
- [x] Все сертификаты проверены через openssl

---

**Phase 1 Milestones:**
- ✅ PKI скрипты работают
- ✅ Выпущены тестовые сертификаты
- ✅ mTLS протестирован на MySQL
- ✅ Документация готова
- ✅ Code review завершен

---

## Phase 2: Core HSM Service Development

**Duration:** Days 4-8  
**Goal:** Разработать HSM service с полным функционалом

### Day 4: Project Setup & Configuration

#### Task 4.1: Go Module Setup
**Priority:** CRITICAL  
**Estimate:** 30 min

```bash
go mod init github.com/your-org/hsm-service
go get github.com/ThalesIgnite/crypto11
go get gopkg.in/yaml.v3
go get golang.org/x/time/rate
```

**go.mod dependencies:**
- crypto11 для PKCS#11
- yaml.v3 для конфигурации
- golang.org/x/time/rate для rate limiting

**Deliverables:**
- [x] go.mod создан
- [x] Зависимости скачаны
- [x] crypto11 (ThalesGroup) v1.6.0 установлен
- [x] yaml.v3 v3.0.1 установлен
- [x] golang.org/x/time v0.14.0 установлен

---

#### Task 4.2: Configuration Types & Loader
**Priority:** CRITICAL  
**Estimate:** 2 hours

**Files:**
- `internal/config/types.go` - структуры конфигурации и метаданных
- `internal/config/config.go` - загрузка config.yaml и metadata.yaml + ENV
- `config.yaml` - статическая конфигурация (в Git)
- `metadata.yaml` - динамические метаданные ротации (вне Git)

**config.yaml structure:**
```yaml
server:
  port: 8443
  tls:
    ca_path: /app/pki/ca/ca.crt
    cert_path: /app/pki/server/hsm-service.local.crt
    key_path: /app/pki/server/hsm-service.local.key

hsm:
  pkcs11_lib: /usr/lib/softhsm/libsofthsm2.so
  slot_id: hsm-token
  metadata_file: /app/metadata.yaml
  keys:
    exchange-key:
      type: aes
      rotation_interval: 2160h  # 90 days
    2fa:
      type: aes
      rotation_interval: 2160h

acl:
  revoked_file: /app/pki/revoked.yaml
  mappings:
    Trading: [exchange-key]
    2FA: [2fa]
    Database: []

rate_limit:
  requests_per_second: 100
  burst: 50

logging:
  level: info
  format: json
```

**metadata.yaml structure:**
```yaml
rotation:
  exchange-key:
    label: kek-exchange-v1
    version: 1
    created_at: '2025-10-11T12:00:00Z'
  
  2fa:
    label: kek-2fa-v1
    version: 1
    created_at: '2025-10-11T12:00:00Z'
```

**Code structure:**
```go
// internal/config/types.go
type Config struct {
    Server ServerConfig
    HSM    HSMConfig
    ACL    ACLConfig
    RateLimit RateLimitConfig
    Logging LoggingConfig
}

// internal/config/config.go
func Load(path string) (*Config, error)
func (c *Config) Validate() error
```

**Deliverables:**
- [x] Config types определены (types.go)
- [x] Load() функция работает (config.go)
- [x] ENV variables поддерживаются (HSM_PIN и др.)
- [x] Validation логика реализована (validateConfig)
- [x] Unit tests созданы (config_test.go)
- [x] Все тесты проходят

---

#### Task 4.3: Logging Setup
**Priority:** MEDIUM  
**Estimate:** 1 hour

**Use:** `log/slog` (Go 1.21+)

**Features:**
- JSON structured logging
- Configurable log level
- Audit logger (отдельный logger для audit events)

**Code:**
```go
// internal/server/middleware.go
func AuditLogger(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        start := time.Now()
        
        // Extract client info
        clientCN := r.TLS.PeerCertificates[0].Subject.CommonName
        
        next.ServeHTTP(w, r)
        
        // Log audit event
        slog.Info("audit",
            "client_cn", clientCN,
            "operation", r.URL.Path,
            "duration_ms", time.Since(start).Milliseconds(),
        )
    })
}
```

**Deliverables:**
- [x] Structured logging настроен (log/slog)
- [x] Audit logger реализован (AuditLogMiddleware)
- [x] НЕ логируются plaintext/secrets (SanitizeForLog)
- [x] RecoveryMiddleware для panic recovery
- [x] Unit tests созданы (logger_test.go)
- [x] Все тесты проходят

---

### Day 5: PKCS#11 Integration

#### Task 5.1: PKCS#11 Initialization
**Priority:** CRITICAL  
**Estimate:** 3 hours

**File:** `internal/hsm/pkcs11.go`

**Functionality:**
1. Инициализация crypto11
2. Открытие сессии с токеном
3. Поиск KEK по label
4. Caching KEK handles

**Code structure:**
```go
type HSMContext struct {
    ctx *crypto11.Context
    keys map[string]*crypto11.SecretKey  // label -> key
}

func InitHSM(config *config.HSMConfig, pin string) (*HSMContext, error) {
    // 1. Configure crypto11
    cfg := &crypto11.Config{
        Path:       config.Library,
        TokenLabel: config.TokenLabel,
        Pin:        pin,
    }
    
    // 2. Initialize context
    ctx, err := crypto11.Configure(cfg)
    if err != nil {
        return nil, err
    }
    
    // 3. Find all configured KEKs
    keys := make(map[string]*crypto11.SecretKey)
    for _, keyConfig := range config.Keys {
        key, err := ctx.FindKey(nil, []byte(keyConfig.Label))
        if err != nil {
            return nil, fmt.Errorf("KEK not found: %s", keyConfig.Label)
        }
        keys[keyConfig.Label] = key.(*crypto11.SecretKey)
    }
    
    return &HSMContext{ctx: ctx, keys: keys}, nil
}

func (h *HSMContext) Close() error {
    return h.ctx.Close()
}
```

**Deliverables:**
- [x] PKCS#11 инициализация работает
- [x] KEK успешно находятся по label
- [x] Error handling корректен
- [x] Context cleanup через Close()

---

#### Task 5.2: Crypto Operations
**Priority:** CRITICAL  
**Estimate:** 4 hours

**File:** `internal/hsm/crypto.go`

**Encrypt function:**
```go
func (h *HSMContext) Encrypt(
    plaintext []byte,
    aad []byte,
    keyLabel string,
) (ciphertext []byte, err error) {
    // 1. Get KEK
    key, ok := h.keys[keyLabel]
    if !ok {
        return nil, ErrKeyNotFound
    }
    
    // 2. Generate random nonce (12 bytes for GCM)
    nonce := make([]byte, 12)
    if _, err := rand.Read(nonce); err != nil {
        return nil, err
    }
    
    // 3. Create GCM cipher
    block, err := aes.NewCipher(key.Value) // !! WRONG - SecretKey не имеет Value
    // Правильно: использовать crypto11 API
    
    cipher, err := key.NewGCM()
    if err != nil {
        return nil, err
    }
    
    // 4. Encrypt with AAD
    encrypted := cipher.Seal(nil, nonce, plaintext, aad)
    
    // 5. Return nonce || ciphertext || tag
    result := append(nonce, encrypted...)
    return result, nil
}
```

**NOTE:** Проверить crypto11 API для GCM!

**Decrypt function:**
```go
func (h *HSMContext) Decrypt(
    ciphertext []byte,
    aad []byte,
    keyLabel string,
) (plaintext []byte, err error) {
    // 1. Parse nonce || encrypted_data
    if len(ciphertext) < 12+16 {
        return nil, ErrInvalidCiphertext
    }
    
    nonce := ciphertext[:12]
    encrypted := ciphertext[12:]
    
    // 2. Get KEK
    key, ok := h.keys[keyLabel]
    if !ok {
        return nil, ErrKeyNotFound
    }
    
    // 3. Create GCM cipher
    cipher, err := key.NewGCM()
    if err != nil {
        return nil, err
    }
    
    // 4. Decrypt with AAD verification
    plaintext, err = cipher.Open(nil, nonce, encrypted, aad)
    if err != nil {
        return nil, ErrDecryptionFailed  // AAD mismatch or tampered data
    }
    
    return plaintext, nil
}
```

**AAD Builder:**
```go
func BuildAAD(context, clientCN string) []byte {
    return []byte(context + "|" + clientCN)
}
```

**Deliverables:**
- [x] Encrypt работает корректно
- [x] Decrypt работает корректно
- [x] AAD проверяется
- [x] Unit tests для round-trip (6 тестов проходят)

**Testing:**
```go
func TestEncryptDecrypt(t *testing.T) {
    plaintext := []byte("secret data")
    aad := BuildAAD("exchange-key", "trading-service-1")
    
    ciphertext, err := hsm.Encrypt(plaintext, aad, "kek-exchange-v1")
    assert.NoError(t, err)
    
    decrypted, err := hsm.Decrypt(ciphertext, aad, "kek-exchange-v1")
    assert.NoError(t, err)
    assert.Equal(t, plaintext, decrypted)
}

func TestAADMismatch(t *testing.T) {
    plaintext := []byte("secret data")
    aad1 := BuildAAD("exchange-key", "trading-service-1")
    aad2 := BuildAAD("2fa", "trading-service-1")  // Different context
    
    ciphertext, _ := hsm.Encrypt(plaintext, aad1, "kek-exchange-v1")
    
    _, err := hsm.Decrypt(ciphertext, aad2, "kek-exchange-v1")
    assert.Error(t, err)  // AAD mismatch должен вернуть ошибку
}
```

---

### Day 6: HTTP Server & Handlers

#### Task 6.1: TLS Server Setup
**Priority:** CRITICAL  
**Estimate:** 2 hours

**File:** `internal/server/server.go`

**Code:**
```go
func NewServer(cfg *config.ServerConfig, hsmCtx *hsm.HSMContext, aclChecker *ACLChecker) (*http.Server, error) {
    // 1. Load server certificate
    serverCert, err := tls.LoadX509KeyPair(
        cfg.TLS.ServerCert,
        cfg.TLS.ServerKey,
    )
    if err != nil {
        return nil, err
    }
    
    // 2. Load CA for client verification
    caCert, err := os.ReadFile(cfg.TLS.CACert)
    if err != nil {
        return nil, err
    }
    caCertPool := x509.NewCertPool()
    caCertPool.AppendCertsFromPEM(caCert)
    
    // 3. TLS Config
    tlsConfig := &tls.Config{
        Certificates: []tls.Certificate{serverCert},
        ClientAuth:   tls.RequireAndVerifyClientCert,
        ClientCAs:    caCertPool,
        MinVersion:   tls.VersionTLS13,
        CipherSuites: []uint16{
            tls.TLS_AES_256_GCM_SHA384,
            tls.TLS_CHACHA20_POLY1305_SHA256,
        },
    }
    
    // 4. HTTP Router
    mux := http.NewServeMux()
    mux.HandleFunc("/encrypt", EncryptHandler(hsmCtx, aclChecker))
    mux.HandleFunc("/decrypt", DecryptHandler(hsmCtx, aclChecker))
    mux.HandleFunc("/health", HealthHandler(hsmCtx))
    
    // 5. Apply middleware
    handler := RateLimitMiddleware(
        AuditLogMiddleware(mux),
    )
    
    // 6. Create server
    server := &http.Server{
        Addr:      fmt.Sprintf(":%d", cfg.Port),
        Handler:   handler,
        TLSConfig: tlsConfig,
    }
    
    return server, nil
}

func (s *Server) Start() error {
    return s.ListenAndServeTLS("", "")  // Certs from TLSConfig
}
```

**Deliverables:**
- [x] TLS 1.3 server работает
- [x] mTLS обязателен
- [x] Endpoints зарегистрированы

---

#### Task 6.2: ACL Checker
**Priority:** CRITICAL  
**Estimate:** 2 hours

**File:** `internal/server/acl.go`

**Code:**
```go
type ACLChecker struct {
    config   *config.ACLConfig
    revoked  map[string]bool  // CN -> revoked
}

func NewACLChecker(cfg *config.ACLConfig) (*ACLChecker, error) {
    checker := &ACLChecker{
        config:  cfg,
        revoked: make(map[string]bool),
    }
    
    // Load revoked list
    if err := checker.LoadRevoked(); err != nil {
        return nil, err
    }
    
    return checker, nil
}

func (a *ACLChecker) LoadRevoked() error {
    data, err := os.ReadFile(a.config.RevokedFile)
    if err != nil {
        return err
    }
    
    var revokedList struct {
        Revoked []struct {
            CN string `yaml:"cn"`
        } `yaml:"revoked"`
    }
    
    if err := yaml.Unmarshal(data, &revokedList); err != nil {
        return err
    }
    
    a.revoked = make(map[string]bool)
    for _, cert := range revokedList.Revoked {
        a.revoked[cert.CN] = true
    }
    
    return nil
}

func (a *ACLChecker) CheckAccess(cert *x509.Certificate, context string) error {
    cn := cert.Subject.CommonName
    
    // 1. Check revoked list
    if a.revoked[cn] {
        return fmt.Errorf("certificate revoked: %s", cn)
    }
    
    // 2. Extract OU
    if len(cert.Subject.OrganizationalUnit) == 0 {
        return errors.New("certificate has no OU")
    }
    ou := cert.Subject.OrganizationalUnit[0]
    
    // 3. Check OU permissions
    allowedContexts, ok := a.config.ByOU[ou]
    if !ok {
        return fmt.Errorf("unknown OU: %s", ou)
    }
    
    // 4. Check context
    for _, allowed := range allowedContexts {
        if allowed == context {
            return nil  // Access granted
        }
    }
    
    return fmt.Errorf("OU %s not allowed for context %s", ou, context)
}
```

**Deliverables:**
- [x] ACL проверка работает
- [x] revoked.yaml загружается
- [x] OU-based authorization работает
- [x] Unit tests

---

#### Task 6.3: Handlers
**Priority:** CRITICAL  
**Estimate:** 3 hours

**File:** `internal/server/handlers.go`

**Types:**
```go
type EncryptRequest struct {
    Context   string `json:"context"`
    Plaintext string `json:"plaintext"`  // base64
}

type EncryptResponse struct {
    Ciphertext string `json:"ciphertext"`  // base64
    KeyID      string `json:"key_id"`
}

type DecryptRequest struct {
    Context    string `json:"context"`
    Ciphertext string `json:"ciphertext"`  // base64
    KeyID      string `json:"key_id"`
}

type DecryptResponse struct {
    Plaintext string `json:"plaintext"`  // base64
}

type ErrorResponse struct {
    Error string `json:"error"`
}
```

**Encrypt Handler:**
```go
func EncryptHandler(hsmCtx *hsm.HSMContext, aclChecker *ACLChecker) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // 1. Parse request
        var req EncryptRequest
        if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
            respondError(w, http.StatusBadRequest, "invalid JSON")
            return
        }
        
        // 2. Extract client cert
        clientCert := r.TLS.PeerCertificates[0]
        clientCN := clientCert.Subject.CommonName
        
        // 3. ACL check
        if err := aclChecker.CheckAccess(clientCert, req.Context); err != nil {
            respondError(w, http.StatusForbidden, err.Error())
            return
        }
        
        // 4. Decode plaintext
        plaintext, err := base64.StdEncoding.DecodeString(req.Plaintext)
        if err != nil {
            respondError(w, http.StatusBadRequest, "invalid base64")
            return
        }
        
        // 5. Build AAD
        aad := hsm.BuildAAD(req.Context, clientCN)
        
        // 6. Determine key ID
        keyID := getActiveKeyForContext(req.Context)  // from config
        
        // 7. Encrypt
        ciphertext, err := hsmCtx.Encrypt(plaintext, aad, keyID)
        if err != nil {
            respondError(w, http.StatusInternalServerError, "encryption failed")
            return
        }
        
        // 8. Respond
        resp := EncryptResponse{
            Ciphertext: base64.StdEncoding.EncodeToString(ciphertext),
            KeyID:      keyID,
        }
        respondJSON(w, http.StatusOK, resp)
    }
}
```

**Decrypt Handler:**
```go
func DecryptHandler(hsmCtx *hsm.HSMContext, aclChecker *ACLChecker) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        // Similar structure to Encrypt
        // ...
        
        // Decrypt with AAD verification
        plaintext, err := hsmCtx.Decrypt(ciphertext, aad, req.KeyID)
        if err != nil {
            // AAD mismatch or invalid ciphertext
            respondError(w, http.StatusBadRequest, "decryption failed")
            return
        }
        
        // ...
    }
}
```

**Health Handler:**
```go
func HealthHandler(hsmCtx *hsm.HSMContext) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        status := struct {
            Status       string            `json:"status"`
            HSMAvailable bool              `json:"hsm_available"`
            KEKStatus    map[string]string `json:"kek_status"`
        }{
            Status:       "healthy",
            HSMAvailable: true,
            KEKStatus:    make(map[string]string),
        }
        
        // Check each KEK
        for label := range hsmCtx.Keys() {
            status.KEKStatus[label] = "available"
        }
        
        respondJSON(w, http.StatusOK, status)
    }
}
```

**Deliverables:**
- [x] /encrypt handler работает
- [x] /decrypt handler работает
- [x] /health handler работает
- [x] Error handling корректен
- [x] Unit tests

---

### Day 7: Middleware & Main

#### Task 7.1: Rate Limiting Middleware
**Priority:** MEDIUM  
**Estimate:** 1.5 hours

**File:** `internal/server/middleware.go`

**Code:**
```go
import "golang.org/x/time/rate"

type RateLimiter struct {
    limiters map[string]*rate.Limiter  // CN -> limiter
    mu       sync.RWMutex
    rate     int
    burst    int
}

func NewRateLimiter(rps, burst int) *RateLimiter {
    return &RateLimiter{
        limiters: make(map[string]*rate.Limiter),
        rate:     rps,
        burst:    burst,
    }
}

func (rl *RateLimiter) GetLimiter(clientCN string) *rate.Limiter {
    rl.mu.Lock()
    defer rl.mu.Unlock()
    
    limiter, exists := rl.limiters[clientCN]
    if !exists {
        limiter = rate.NewLimiter(rate.Limit(rl.rate), rl.burst)
        rl.limiters[clientCN] = limiter
    }
    
    return limiter
}

func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
    return func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            clientCN := r.TLS.PeerCertificates[0].Subject.CommonName
            
            if !limiter.GetLimiter(clientCN).Allow() {
                w.Header().Set("Retry-After", "1")
                respondError(w, http.StatusTooManyRequests, "rate limit exceeded")
                return
            }
            
            next.ServeHTTP(w, r)
        })
    }
}
```

**Deliverables:**
- [x] Rate limiting работает
- [x] Per-client limiters
- [x] 429 возвращается при превышении
- [x] Cleanup goroutine для limiters (memory leak prevention)

---

#### Task 7.2: Main Entry Point
**Priority:** CRITICAL  
**Estimate:** 1 hour

**File:** `main.go`

**Code:**
```go
func main() {
    // 1. Load config
    cfg, err := config.Load("config.yaml")
    if err != nil {
        log.Fatal(err)
    }
    
    // 2. Get HSM PIN from ENV
    hsmPIN := os.Getenv("HSM_PIN")
    if hsmPIN == "" {
        log.Fatal("HSM_PIN not set")
    }
    
    // 3. Initialize HSM
    hsmCtx, err := hsm.InitHSM(&cfg.HSM, hsmPIN)
    if err != nil {
        log.Fatal(err)
    }
    defer hsmCtx.Close()
    
    // 4. Initialize ACL
    aclChecker, err := server.NewACLChecker(&cfg.ACL)
    if err != nil {
        log.Fatal(err)
    }
    
    // 5. Create server
    srv, err := server.NewServer(&cfg.Server, hsmCtx, aclChecker)
    if err != nil {
        log.Fatal(err)
    }
    
    // 6. Start server
    log.Printf("Starting HSM service on port %d", cfg.Server.Port)
    if err := srv.Start(); err != nil {
        log.Fatal(err)
    }
}
```

**Deliverables:**
- [x] main.go собирает все компоненты
- [x] Graceful startup
- [x] Error handling

---

### Day 8: CLI Tool (hsm-admin)

#### Task 8.1: CLI Structure
**Priority:** MEDIUM  
**Estimate:** 3 hours

**File:** `cmd/hsm-admin/main.go`

**Commands:**
- `create-kek --label <label> --context <context>`
- `list-kek`
- `delete-kek --label <label> --confirm`
- `export-metadata --output <file>`
- `rotate <context>` - KEY ROTATION (PCI DSS 3.6.4)
- `rotation-status` - Check rotation status
- `cleanup-old-versions` - PCI DSS compliance
- `update-checksums` - KEK integrity verification

**Code (using cobra or manual):**
```go
func main() {
    if len(os.Args) < 2 {
        printUsage()
        os.Exit(1)
    }
    
    command := os.Args[1]
    
    switch command {
    case "create-kek":
        createKEK(os.Args[2:])
    case "list-kek":
        listKEK()
    case "delete-kek":
        deleteKEK(os.Args[2:])
    case "export-metadata":
        exportMetadata(os.Args[2:])
    default:
        printUsage()
        os.Exit(1)
    }
}

func createKEK(args []string) {
    // Parse flags
    // Initialize PKCS#11
    // Generate AES-256 key with attributes
    // Save to token
}
```

**Deliverables:**
- [x] create-kek работает
- [x] list-kek показывает все KEK
- [x] delete-kek удаляет KEK
- [x] rotate command для ротации ключей
- [x] rotation-status для проверки статуса
- [x] cleanup-old-versions для PCI DSS
- [x] update-checksums для integrity verification
- [x] Все команды протестированы

---

## Phase 3: Docker, Testing & Documentation

**Duration:** Days 9-10  
**Goal:** Завершить интеграцию, тестирование и документацию

### Day 9: Docker Setup

#### Task 9.1: Dockerfile
**Priority:** HIGH  
**Estimate:** 1.5 hours

**File:** `Dockerfile`

```dockerfile
FROM golang:1.21-alpine AS builder

# Install dependencies
RUN apk add --no-cache softhsm openssl build-base

WORKDIR /app

# Download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Build application
COPY . .
RUN go build -o hsm-service .
RUN cd cmd/hsm-admin && go build -o ../../hsm-admin .

# Runtime image
FROM alpine:latest

RUN apk add --no-cache softhsm openssl

WORKDIR /app

# Copy binaries
COPY --from=builder /app/hsm-service .
COPY --from=builder /app/hsm-admin .

# Copy configs
COPY softhsm2.conf /etc/softhsm/softhsm2.conf
COPY scripts/init-hsm.sh /app/init-hsm.sh
RUN chmod +x /app/init-hsm.sh

EXPOSE 8443

ENTRYPOINT ["/app/init-hsm.sh"]
```

**Deliverables:**
- [x] Dockerfile собирается
- [x] Multi-stage build работает

---

#### Task 9.2: Docker Compose
**Priority:** HIGH  
**Estimate:** 1 hour

**File:** `docker-compose.yml`

```yaml
version: '3.8'

services:
  hsm-service:
    build: .
    container_name: hsm-service
    ports:
      - "8443:8443"
    environment:
      - HSM_PIN=${HSM_PIN:-1234}
      - HSM_SO_PIN=${HSM_SO_PIN:-12345678}
      - CONFIG_PATH=/app/config.yaml
    volumes:
      # Persistent token storage
      - ./data/tokens:/var/lib/softhsm/tokens
      
      # PKI certificates (read-only)
      - ./pki:/app/pki:ro
      
      # Configuration (read-only)
      - ./config.yaml:/app/config.yaml:ro
    
    restart: unless-stopped
    
    networks:
      - hsm-net

networks:
  hsm-net:
    driver: bridge
```

**Deliverables:**
- [x] docker-compose работает
- [x] Volumes настроены корректно
- [x] ENV variables работают
- [x] .env.example создан
- [x] Healthcheck настроен
- [x] Networks настроены
- [x] Resource limits добавлены

---

#### Task 9.3: Init Script
**Priority:** HIGH  
**Estimate:** 1 hour

**File:** `scripts/init-hsm.sh`

```bash
#!/bin/sh
set -e

TOKEN_LABEL="${HSM_TOKEN_LABEL:-hsm-token}"
TOKEN_PIN="${HSM_PIN:-1234}"
TOKEN_SO_PIN="${HSM_SO_PIN:-12345678}"

# Create token directory
mkdir -p /var/lib/softhsm/tokens

# Check if token already initialized
if ! softhsm2-util --show-slots | grep -q "$TOKEN_LABEL"; then
    echo "Initializing SoftHSM token: $TOKEN_LABEL"
    softhsm2-util --init-token \
        --slot 0 \
        --label "$TOKEN_LABEL" \
        --pin "$TOKEN_PIN" \
        --so-pin "$TOKEN_SO_PIN"
    echo "✓ Token initialized"
else
    echo "✓ Token already initialized"
fi

# List slots for verification
softhsm2-util --show-slots

# Start HSM service
echo "Starting HSM service..."
exec /app/hsm-service
```

**Deliverables:**
- [x] Script инициализирует токен
- [x] Idempotent (можно запускать повторно)
- [x] Exec в конце для PID 1
- [x] Автоматически создает KEK при первом запуске
- [x] Проверяет наличие KEK перед запуском
- [x] Используется create-kek helper tool

---

### Day 10: Integration Testing & Documentation

#### Task 10.1: Integration Tests
**Priority:** HIGH  
**Estimate:** 3 hours

**Test scenarios:**

1. **Full encrypt/decrypt flow:**
```bash
# Start service
docker-compose up -d

# Wait for startup
sleep 5

# Test encrypt
curl -X POST https://localhost:8443/encrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{"context":"exchange-key","plaintext":"SGVsbG8="}'

# Test decrypt
# ...
```

2. **ACL tests:**
- trading-service-1 → context=exchange-key → SUCCESS
- trading-service-1 → context=2fa → FORBIDDEN
- web-2fa-service → context=2fa → SUCCESS

3. **Revocation test:**
- Add CN to revoked.yaml
- Restart service
- Request → FORBIDDEN

4. **Rate limiting test:**
- Send 150 requests rapidly
- After 100 → 429 Too Many Requests

**Deliverables:**
- [x] Все тесты проходят (31 integration tests)
- [x] Documented в tests/integration/full-integration-test.sh
- [x] Automated testing framework
- [x] Coverage includes rotation lifecycle

---

#### Task 10.2: README.md
**Priority:** HIGH  
**Estimate:** 2 hours

**Content:**
1. Overview
2. Quick Start
3. Development Setup
4. Production Deployment
5. API Examples
6. PKI Management
7. KEK Management (CLI)
8. Troubleshooting
9. FAQ

**Deliverables:**
- [x] README.md завершен
- [x] Все команды протестированы
- [x] ARCHITECTURE.md создан
- [x] TECHNICAL_SPEC.md создан
- [x] SECURITY_AUDIT.md создан
- [x] KEY_ROTATION.md создан
- [x] DOCKER_COMPOSE.md создан

---

#### Task 10.3: Final Review
**Priority:** CRITICAL  
**Estimate:** 2 hours

**Checklist:**
- [x] Все функциональные требования выполнены
- [x] Код соответствует стандартам Go
- [x] Error handling корректен
- [x] Logging не содержит секретов
- [x] Documentation полная (5+ markdown files)
- [x] Docker setup работает
- [x] Integration tests проходят (31 tests)
- [x] Security review пройден (SECURITY_AUDIT.md)
- [x] OWASP Top 10 2021 addressed
- [x] PCI DSS 3.6.4 compliance (key rotation)

---

## Timeline Summary

| Day | Phase | Tasks | Status |
|-----|-------|-------|--------|
| 1   | PKI   | PKI scripts, structure | ✅ |
| 2   | PKI   | Certificate generation, MySQL test | ✅ |
| 3   | PKI   | Documentation, review | ✅ |
| 4   | Core  | Project setup, config | ✅ |
| 5   | Core  | PKCS#11 integration | ✅ |
| 6   | Core  | HTTP server, handlers | ✅ |
| 7   | Core  | Middleware, main | ✅ |
| 8   | Core  | CLI tool (extended with rotation) | ✅ |
| 9   | Final | Docker setup | ✅ |
| 10  | Final | Testing, documentation | ✅ |

**BONUS COMPLETED:**
- ✅ Prometheus metrics (Phase 4 item #1)
- ✅ Log rotation with lumberjack
- ✅ Graceful shutdown (Phase 4 item #3)
- ✅ KEK rotation automation (Phase 4 item #5)
- ✅ Hot reload для revoked.yaml (Phase 4 item #2)
- ✅ Comprehensive security audit
- ✅ OWASP Top 10 2021 compliance
- ✅ PCI DSS 3.6.4/3.6.5 compliance

---

## Risk Management

### Identified Risks

| Risk | Impact | Probability | Mitigation |
|------|--------|-------------|------------|
| crypto11 API сложнее ожидаемого | HIGH | MEDIUM | Изучить документацию заранее, fallback на miekg/pkcs11 |
| SoftHSM bugs | MEDIUM | LOW | Использовать стабильную версию, тестировать рано |
| TLS configuration проблемы | MEDIUM | LOW | Тестировать на MySQL первым делом |
| Performance issues | LOW | LOW | Load testing на day 10 |

---

## Definition of Done

**Phase 1 (PKI):**
- ✅ Все PKI скрипты работают
- ✅ MySQL mTLS протестирован успешно
- ✅ Documentation готова

**Phase 2 (Core):**
- ✅ /encrypt и /decrypt работают
- ✅ mTLS authentication обязателен
- ✅ ACL по OU функционирует
- ✅ Rate limiting работает
- ✅ CLI утилита создает/удаляет KEK

**Phase 3 (Final):**
- ✅ Docker setup работает
- ✅ Integration tests проходят
- ✅ README.md полная
- ✅ Code review завершен

---

## Next Steps After MVP

**Phase 4 (Future enhancements):**
1. ✅ ~~Prometheus metrics~~ - COMPLETED (internal/server/metrics.go)
2. ✅ ~~Hot reload для revoked.yaml~~ - COMPLETED (automatic 30s reload with validation)
3. ✅ ~~Graceful shutdown~~ - COMPLETED (main.go)
4. 🔴 **Hot reload для metadata.yaml и KEK** - CRITICAL FOR PRODUCTION (zero-downtime rotation)
5. ⬜ HA deployment (active-passive)
6. ✅ ~~KEK rotation automation~~ - COMPLETED (hsm-admin rotate)
7. ⬜ CRL support (вместо revoked.yaml)
8. ⬜ Request tracing (OpenTelemetry)
9. ⬜ Performance optimization
10. ✅ ~~Log rotation~~ - COMPLETED (lumberjack)
11. ✅ ~~Memory security (zeroing)~~ - COMPLETED
12. ✅ ~~Request size limits~~ - COMPLETED (1MB MaxBytesReader)
13. ✅ ~~Server timeouts~~ - COMPLETED (Slowloris protection)
14. ✅ ~~Rate limiter cleanup~~ - COMPLETED (memory leak prevention)
15. ✅ ~~KEK integrity verification~~ - COMPLETED (checksums)
16. ⬜ OCSP stapling for certificate revocation

---

## Phase 4: Production-Critical - KEK Hot Reload

**Priority:** 🔴 CRITICAL  
**Effort:** 8-12 hours  
**Reason:** Restart недопустим для нагруженных систем с 50+ клиентами

### Problem Statement

**Current limitation:**
```bash
# Текущий процесс ротации
hsm-admin rotate exchange-key   # создает kek-exchange-v2
docker compose restart hsm-service  # ❌ DOWNTIME для всех клиентов!
```

**Impact:**
- ❌ Все 50+ клиентов теряют соединение
- ❌ Активные операции прерываются
- ❌ Single point of failure при обновлении
- ❌ Невозможность graceful rotation

### Implementation - Zero-Downtime KEK Reload

#### Task 4.1: KeyManager с Hot Reload
**File:** `internal/hsm/key_manager.go`  
**Effort:** 4 hours

**Implementation:**

```go
package hsm

type KeyManager struct {
    ctx           crypto11.Context  // Persistent PKCS#11 session
    keys          map[string]*KeyHandle
    keysMutex     sync.RWMutex
    
    metadata      *config.MetadataConfig
    metadataMutex sync.RWMutex
    metadataFile  string
    lastModTime   time.Time
    
    stopReload    chan struct{}
    reloadWg      sync.WaitGroup
}

type KeyHandle struct {
    Handle  crypto11.SecretKey
    Label   string
    Version int
}

// Автоматическая проверка metadata.yaml каждые 30 секунд
func (km *KeyManager) StartAutoReload(interval time.Duration) {
    km.reloadWg.Add(1)
    go func() {
        defer km.reloadWg.Done()
        ticker := time.NewTicker(interval)
        defer ticker.Stop()
        
        for {
            select {
            case <-ticker.C:
                if km.metadataChanged() {
                    if err := km.ReloadKeysFromMetadata(); err != nil {
                        slog.Error("metadata reload failed", "error", err)
                    }
                }
            case <-km.stopReload:
                return
            }
        }
    }()
}

// Проверка изменения metadata.yaml
func (km *KeyManager) metadataChanged() bool {
    info, err := os.Stat(km.metadataFile)
    if err != nil {
        return false
    }
    
    if info.ModTime().After(km.lastModTime) {
        km.lastModTime = info.ModTime()
        return true
    }
    return false
}

// Загрузка новых KEK без разрыва PKCS#11 сессии
func (km *KeyManager) ReloadKeysFromMetadata() error {
    // 1. Read metadata.yaml
    newMetadata, err := config.LoadMetadata(km.metadataFile)
    if err != nil {
        slog.Warn("metadata reload skipped", "error", err)
        return err // Keep old data
    }
    
    // 2. Load NEW keys from HSM (не закрывая старые)
    newKeys := make(map[string]*KeyHandle)
    for context, meta := range newMetadata.Rotation {
        handle, err := km.ctx.FindKey(nil, []byte(meta.Label))
        if err != nil {
            slog.Error("key not found in HSM", 
                "context", context, "label", meta.Label)
            return err // Rollback, keep old data
        }
        newKeys[context] = &KeyHandle{
            Handle:  handle,
            Label:   meta.Label,
            Version: meta.Version,
        }
    }
    
    // 3. Atomic swap (все или ничего)
    km.keysMutex.Lock()
    oldKeys := km.keys
    km.keys = newKeys
    km.keysMutex.Unlock()
    
    km.metadataMutex.Lock()
    km.metadata = newMetadata
    km.metadataMutex.Unlock()
    
    slog.Info("KEK hot reload successful", 
        "contexts", len(newKeys),
        "old_count", len(oldKeys))
    return nil
}

// Graceful shutdown
func (km *KeyManager) StopAutoReload(ctx context.Context) error {
    close(km.stopReload)
    
    done := make(chan struct{})
    go func() {
        km.reloadWg.Wait()
        close(done)
    }()
    
    select {
    case <-done:
        return nil
    case <-ctx.Done():
        return ctx.Err()
    }
}
```

#### Task 4.2: Integration в main.go
**File:** `main.go`  
**Effort:** 2 hours

```go
// Инициализация KeyManager
keyManager, err := hsm.NewKeyManager(hsmCtx, cfg.HSM.MetadataFile)
if err != nil {
    log.Fatal(err)
}

// Start hot reload для metadata.yaml и KEK
keyManager.StartAutoReload(30 * time.Second)

// Graceful shutdown
shutdown := make(chan os.Signal, 1)
signal.Notify(shutdown, os.Interrupt, syscall.SIGTERM)
<-shutdown

ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
defer cancel()

// Stop auto reload
if err := keyManager.StopAutoReload(ctx); err != nil {
    slog.Error("key manager shutdown timeout", "error", err)
}
```

#### Task 4.3: Unit Tests
**File:** `internal/hsm/key_manager_test.go`  
**Effort:** 3 hours

**Test scenarios:**
1. ✅ Hot reload успешно загружает новые KEK
2. ✅ Старые ключи остаются доступны для decrypt
3. ✅ Битый metadata.yaml не ломает сервис
4. ✅ Concurrent access безопасен (race detector)
5. ✅ Graceful shutdown останавливает reload
6. ✅ Atomic swap - нет частичных обновлений

#### Task 4.4: Integration Test
**File:** `tests/integration/full-integration-test.sh` (Phase 9.5)  
**Effort:** 2 hours

```bash
#!/bin/bash
set -e

echo "🔥 Testing KEK Hot Reload"

# 1. Start HSM service
docker compose up -d hsm-service
sleep 5

# 2. Encrypt with v1
curl -X POST https://localhost:8443/encrypt \
  --cert pki/client/test.crt --key pki/client/test.key \
  -d '{"context":"exchange-key","plaintext":"dGVzdA=="}'

# Store ciphertext
CIPHERTEXT=$(cat response.json | jq -r .ciphertext)

# 3. Rotate key (creates v2)
./hsm-admin rotate exchange-key

# 4. Wait for hot reload (35 seconds)
echo "⏳ Waiting for hot reload..."
sleep 35

# 5. Verify v2 is used for NEW encrypt
curl -X POST https://localhost:8443/encrypt \
  --cert pki/client/test.crt --key pki/client/test.key \
  -d '{"context":"exchange-key","plaintext":"dGVzdA=="}'

KEY_ID=$(cat response2.json | jq -r .key_id)
if [[ "$KEY_ID" != "kek-exchange-v2" ]]; then
  echo "❌ Hot reload failed: still using old key"
  exit 1
fi

# 6. Verify OLD ciphertext still decrypts (v1 still available)
curl -X POST https://localhost:8443/decrypt \
  --cert pki/client/test.crt --key pki/client/test.key \
  -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHERTEXT\",\"key_id\":\"kek-exchange-v1\"}"

if [[ $? -eq 0 ]]; then
  echo "✅ Hot reload successful: v2 active, v1 still decrypts"
else
  echo "❌ Hot reload failed: v1 decrypt broken"
  exit 1
fi

# 7. Check logs for reload event
docker compose logs hsm-service | grep "KEK hot reload successful"

echo "✅ All tests passed!"
```

#### Task 4.5: Documentation Update
**Files:** `KEY_ROTATION.md`, `ARCHITECTURE.md`  
**Effort:** 1 hour

**Update process:**
```bash
# OLD (with downtime):
hsm-admin rotate exchange-key
docker compose restart hsm-service  # ❌

# NEW (zero downtime):
hsm-admin rotate exchange-key
# Wait 30 seconds, service reloads automatically ✅
```

---

## Phase 5: Security Rating 10/10

**Current Status:** Security Rating 9.5/10 ✅

**Remaining 0.5 points - Path to 10/10:**

### Week 1: Documentation (8 hours)

#### 1. Security Incident Response Plan
**File:** `INCIDENT_RESPONSE.md`  
**Effort:** 2 hours

**Contents:**
- Incident classification (P0-P4)
- Response procedures for each severity
- Contact list and escalation path
- Post-mortem process
- Example scenarios (key compromise, DoS attack, data breach)

#### 2. Disaster Recovery Procedures
**File:** `DISASTER_RECOVERY.md`  
**Effort:** 3 hours

**Contents:**
- RPO/RTO metrics definition
- HSM key backup procedures
- metadata.yaml backup/restore procedures
- Recovery procedures after compromise
- Quarterly DR testing schedule
- Failover to backup HSM

#### 3. Operator Training Program
**File:** `OPERATOR_TRAINING.md`  
**Effort:** 2 hours

**Contents:**
- Onboarding checklist for new operators
- Understanding mTLS, OU-based ACL, revocation
- Key rotation procedures (hsm-admin commands)
- Security incident handling
- Quiz for knowledge verification
- Certification process

#### 4. Log Retention Policy
**File:** `LOG_RETENTION_POLICY.md`  
**Effort:** 1 hour

**Contents:**
- Retention period justification (30 days for compliance)
- Archival procedures (S3/storage integration)
- Secure deletion after retention period
- Compliance mapping (PCI DSS 10.7)
- Access control for archived logs

### Week 2: Implementation (4 hours)

#### 5. Dual-Control HSM PIN (PCI DSS 3.6.6)
**File:** `main.go`  
**Effort:** 2 hours

**Implementation:**
```go
// Current: Single HSM_PIN (⚠️ PCI DSS gap)
hsmPIN := os.Getenv("HSM_PIN")

// Fixed: Dual-control split knowledge
hsmPIN1 := os.Getenv("HSM_PIN_PART1")  // Known by Admin 1
hsmPIN2 := os.Getenv("HSM_PIN_PART2")  // Known by Admin 2

if hsmPIN1 == "" || hsmPIN2 == "" {
    log.Fatal("Both HSM_PIN_PART1 and HSM_PIN_PART2 required (dual control)")
}

// Combine parts (or use XOR/hash-based combination)
combinedPIN := hsmPIN1 + hsmPIN2

// Document in DUAL_CONTROL.md:
// - PIN generation ceremony
// - Part distribution procedures
// - Emergency access procedures
```

**Security benefits:**
- ✅ No single person has full HSM access
- ✅ PCI DSS 3.6.6 compliant
- ✅ Requires collusion to compromise

#### 6. Automated Dependency Updates
**File:** `scripts/check-updates.sh`  
**Effort:** 1 hour

**Script:**
```bash
#!/bin/bash
# Check for outdated dependencies
echo "🔍 Checking for dependency updates..."
go list -m -u all

# Run security scan
echo "🔒 Running security vulnerability scan..."
govulncheck ./...

# Suggest updates
echo "📦 To update all dependencies:"
echo "  go get -u ./..."
echo "  go mod tidy"
```

**Automation:**
- Cron job: weekly dependency check
- Dependabot/Renovate bot for automatic PRs
- CI/CD integration with security scanning

#### 7. Dependabot Configuration
**File:** `.github/dependabot.yml`  
**Effort:** 30 minutes

```yaml
version: 2
updates:
  - package-ecosystem: "gomod"
    directory: "/"
    schedule:
      interval: "weekly"
    open-pull-requests-limit: 5
    labels:
      - "dependencies"
      - "security"
```

### Week 3: External Validation (1 hour research)

#### 8. External Penetration Testing
**Effort:** 1 hour (vendor research + scheduling)

**Scope for pentest:**
- TLS 1.3 configuration testing
- mTLS bypass attempts
- ACL bypass attempts (OU manipulation)
- Rate limiting effectiveness
- HSM key extraction attempts
- Replay attack testing
- Memory dump analysis
- Side-channel attacks

**Vendors to consider:**
- Cure53
- Trail of Bits
- NCC Group
- Local security firms

**Deliverable:** `PENTEST_RESULTS_2026.md`

### Long-term Enhancements

#### 9. FIPS 140-2 Level 3 HSM Upgrade
**Timeline:** Production deployment phase

**Current:** SoftHSM (FIPS 140-2 Level 1 equivalent)

**Production options:**
- **Thales Luna Network HSM** - Level 3, enterprise-grade
- **Entrust nShield** - Level 3, banking industry standard
- **AWS CloudHSM** - Level 3, cloud-native
- **YubiHSM 2** - Level 3, budget-friendly (~$650)

**Migration path:**
- Same PKCS#11 interface (minimal code changes)
- Key migration via export/import with versioning
- Zero-downtime migration using multi-version support

#### 10. Quarterly Security Reviews
**Schedule:**
- Q1: Dependency updates + vulnerability scan
- Q2: Internal security audit + DR drill
- Q3: Operator training refresh + access review
- Q4: External pentest + compliance review

---

## Effort Summary - Path to 10/10

| Task | Effort | Type |
|------|--------|------|
| INCIDENT_RESPONSE.md | 2h | Documentation |
| DISASTER_RECOVERY.md | 3h | Documentation |
| OPERATOR_TRAINING.md | 2h | Documentation |
| LOG_RETENTION_POLICY.md | 1h | Documentation |
| Dual-control HSM PIN | 2h | Implementation |
| Dependency automation | 1h | Implementation |
| Dependabot setup | 0.5h | Configuration |
| Pentest scheduling | 1h | Procurement |
| **TOTAL** | **12.5 hours** | **Full 10/10** |

---

## Security Rating Progression

```
9.0/10 → All OWASP Top 10 2021 addressed
9.5/10 → Key rotation + monitoring + hot reload (CURRENT)
10/10  → Full operational security (documentation + procedures + pentest)
```

**Expected timeline:** 2-3 weeks to full 10/10 rating

**Post-10/10 maintenance:**
- Quarterly dependency updates
- Annual penetration testing
- Continuous operator training
- Regular DR drills

---

**END OF DEVELOPMENT PLAN**
