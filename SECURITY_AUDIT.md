# Security Audit Report: HSM Service

**Date:** January 9, 2026  
**Scope:** OWASP Top 10 2021 + PCI DSS Compliance  
**Severity:** 🔴 Critical | 🟠 High | 🟡 Medium | 🟢 Low

---

## Executive Summary

**Overall Security Rating: 7.5/10** ⚠️

The HSM service implements several strong security controls (mTLS, TLS 1.3, rate limiting, audit logging), but has **critical gaps** in input validation, error handling, and operational security that must be addressed before production deployment.

---

## OWASP Top 10 2021 Analysis

### ✅ A01:2021 – Broken Access Control (STRONG)
**Status:** Well implemented

**Strengths:**
- ✅ mTLS enforced (`tls.RequireAndVerifyClientCert`)
- ✅ OU-based ACL (`CheckAccess` on every request)
- ✅ Per-client rate limiting by CN
- ✅ Context-based key segregation

**Recommendations:**
- ✅ Already optimal

---

### ✅ A02:2021 – Cryptographic Failures (FIXED)

**Status:** All critical issues resolved

1. **✅ FIXED: Key rotation mechanism implemented**
   - ✅ KEK rotation via `hsm-admin rotate` command
   - ✅ Multi-version support (overlap period for zero-downtime)
   - ✅ PCI DSS cleanup via `hsm-admin cleanup-old-versions`
   - ✅ Integration tests cover full rotation lifecycle
   - **Status:** PCI DSS 3.6.4 compliant

2. **✅ FIXED: Plaintext in memory**
   ```go
   // handlers.go:93-100
   plaintext, err := base64.StdEncoding.DecodeString(req.Plaintext)
   if err != nil {
       respondError(w, http.StatusBadRequest, "invalid base64 plaintext")
       return
   }
   // Zero plaintext memory after use (security: prevent memory dumps)
   defer func() {
       for i := range plaintext {
           plaintext[i] = 0
       }
   }()
   ```
   - ✅ Memory zeroed after use in both EncryptHandler and DecryptHandler
   - ✅ Applied to plaintext and ciphertext buffers
   - **Status:** Mitigated memory dump risk

3. **✅ FIXED: AAD construction strengthened**
   ```go
   // crypto.go:20-28
   func BuildAAD(context, clientCN string) []byte {
       h := sha256.New()
       h.Write([]byte(context))
       h.Write([]byte{0x00}) // NULL byte separator
       h.Write([]byte(clientCN))
       return h.Sum(nil) // Returns 32-byte SHA-256 hash
   }
   ```
   - ✅ SHA-256 hashing prevents separator ambiguity
   - ✅ NULL byte separator ensures no collisions
   - ✅ Fixed-length 32-byte output
   - **Status:** Collision-resistant AAD construction
   
   **Vulnerability explained:**
   - **Before:** Simple concatenation `context + "|" + clientCN`
   - **Problem:** `"exchange|key" + "|" + "admin"` == `"exchange" + "|" + "key|admin"` (same AAD!)
   - **Attack scenario:** Attacker encrypts with context="exchange", CN="key|admin", then replays with different context/CN split
   - **After:** SHA-256 ensures `hash("exchange" + 0x00 + "key|admin")` ≠ `hash("exchange|key" + 0x00 + "admin")`

**Recommendations:**
```go
// All critical cryptographic issues have been addressed:
// ✅ Key rotation implemented (hsm-admin rotate)
// ✅ Memory zeroing implemented (defer cleanup)
// ✅ AAD collision prevention (SHA-256 hashing)
```

---

### ✅ A03:2021 – Injection (FIXED)

**Status:** Information disclosure vulnerabilities resolved

**Strengths:**
- ✅ No SQL/NoSQL usage
- ✅ JSON parsing with type safety
- ✅ Base64 decoding with validation
- ✅ No shell commands from user input
- ✅ Generic error messages (no user input leakage)

**Fixed Issues:**
1. **✅ Context in error messages** 
   ```go
   // Before: fmt.Sprintf("no key for context: %s", req.Context)
   // After: "invalid context"  // Generic, no user data exposed
   ```

2. **✅ ACL error messages sanitized**
   ```go
   // Before: fmt.Errorf("certificate revoked: %s", cn)
   // Before: fmt.Errorf("OU %s not allowed for context %s", ou, context)
   // After: errors.New("certificate revoked")
   // After: errors.New("access denied: insufficient permissions")
   ```
   - Detailed information logged server-side only
   - Generic errors prevent information disclosure
   - Attackers can't enumerate valid contexts/OUs

---

### 🟠 A04:2021 – Insecure Design (MEDIUM RISK)

**Issues Found:**

1. **✅ FIXED: Request size limits implemented**
   ```go
   // handlers.go (EncryptHandler and DecryptHandler)
   const maxRequestSize = 1 * 1024 * 1024 // 1MB
   r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)
   ```
   - ✅ 1MB limit prevents DoS via large payloads
   - ✅ Applied to both /encrypt and /decrypt endpoints
   - ✅ Returns 413 Request Entity Too Large automatically
   - **Status:** DoS protection via oversized requests implemented

2. **🟠 HIGH: No timeout configuration**
   ```go
   // server.go:77
   httpServer := &http.Server{
       Addr:      ":" + cfg.Port,
       Handler:   handler,
       TLSConfig: tlsConfig,
       // Missing: ReadTimeout, WriteTimeout, IdleTimeout
   }
   ```
   - Slowloris attacks possible

3. **🟡 MEDIUM: Rate limiter memory leak**
   ```go
   // middleware.go:92
   if !exists {
       limiter = rate.NewLimiter(rate.Limit(rl.rps), rl.burst)
       rl.limiters[clientCN] = limiter  // Never cleaned up
   }
   ```
   - Grows unbounded with unique CNs

**Recommendations:**
```go
// 1. Add request limits
const maxRequestSize = 1 * 1024 * 1024 // 1MB
r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)

// 2. Add timeouts
httpServer := &http.Server{
    Addr:         ":" + cfg.Port,
    Handler:      handler,
    TLSConfig:    tlsConfig,
    ReadTimeout:  10 * time.Second,
    WriteTimeout: 10 * time.Second,
    IdleTimeout:  60 * time.Second,
}

// 3. Add limiter cleanup
func (rl *RateLimiter) Cleanup() {
    ticker := time.NewTicker(1 * time.Hour)
    for range ticker.C {
        rl.mu.Lock()
        // Remove limiters not used in 24h
        for cn, limiter := range rl.limiters {
            if time.Since(limiter.lastUsed) > 24*time.Hour {
                delete(rl.limiters, cn)
            }
        }
        rl.mu.Unlock()
    }
}
```

---

### 🟡 A05:2021 – Security Misconfiguration (MEDIUM)

**Issues Found:**

1. **🟡 MEDIUM: TLS 1.2 fallback disabled**
   ```go
   MinVersion: tls.VersionTLS13,
   ```
   - **Good:** Strong security
   - **Risk:** Legacy client compatibility issues
   - **Recommendation:** Document TLS 1.3-only requirement

2. **🟡 MEDIUM: No cipher suite for TLS 1.2**
   - If TLS 1.2 support added, no ciphers configured
   - **Recommendation:** Keep TLS 1.3 only OR add secure TLS 1.2 ciphers

3. **🟢 LOW: Error messages too verbose**
   ```go
   // handlers.go:119
   respondError(w, http.StatusInternalServerError, "encryption failed")
   ```
   - Generic error is good, but internal errors logged

**Strengths:**
- ✅ Strong cipher suites (AES-256-GCM, ChaCha20-Poly1305)
- ✅ mTLS enforced
- ✅ Minimum TLS 1.3

---

### 🟢 A06:2021 – Vulnerable Components (LOW RISK)

**Dependencies:**
```
✅ github.com/ThalesGroup/crypto11 v1.6.0 (mature, maintained)
✅ golang.org/x/time v0.14.0 (official Go package)
✅ gopkg.in/yaml.v3 v3.0.1 (widely used)
✅ github.com/miekg/pkcs11 v1.1.1 (de-facto standard)
```

**Recommendations:**
- Set up Dependabot/Renovate for automated updates
- Regular `go list -m -u all` checks
- Pin specific versions in go.mod

---

### 🟢 A07:2021 – Authentication Failures (STRONG)

**Status:** Excellent implementation

**Strengths:**
- ✅ Certificate-based authentication (mTLS)
- ✅ CN extraction and validation
- ✅ No password/token handling
- ✅ Certificate revocation check (`revoked.yaml`)

**Minor Enhancement:**
```go
// Add OCSP stapling for real-time revocation
tlsConfig.OCSPStapling = true
```

---

### 🟠 A08:2021 – Software/Data Integrity (MEDIUM RISK)

**Issues Found:**

1. **🔴 CRITICAL: No KEK integrity verification**
   - KEKs loaded without version/checksum validation
   - **Risk:** KEK tampering undetected

2. **🟠 HIGH: No AAD validation on decrypt**
   ```go
   // crypto.go:55
   plaintext, err := gcm.Open(nil, nonce, ciphertext[nonceSize:], aad)
   ```
   - AAD is verified by GCM, but caller can pass wrong AAD
   - **Risk:** Context/CN mismatch allows unauthorized decrypt

**Recommendations:**
```go
// 1. KEK integrity check
type KEKChecksum struct {
    Label    string
    Checksum [32]byte  // SHA-256 of key attributes
}

// 2. AAD re-construction on decrypt
func (h *HSMContext) Decrypt(ciphertext []byte, context, clientCN, keyLabel string) ([]byte, error) {
    aad := BuildAAD(context, clientCN)  // Force correct AAD
    // ... decrypt with verified AAD
}
```

---

### 🟡 A09:2021 – Logging/Monitoring Failures (MEDIUM)

**Issues Found:**

1. **🟠 HIGH: Sensitive data in logs**
   ```go
   // handlers.go:69
   slog.Warn("invalid JSON in encrypt request", "error", err)
   ```
   - Error might contain plaintext from JSON

2. **🟡 MEDIUM: No log rotation**
   - Logs grow unbounded
   - **Risk:** Disk exhaustion

3. **🟡 MEDIUM: No alerting on anomalies**
   - No detection of:
     - High error rates
     - Unusual access patterns
     - Failed ACL checks spike

**Recommendations:**
```go
// 1. Sanitize logs
slog.Warn("invalid JSON in request", "path", r.URL.Path)  // No error details

// 2. Add log rotation (use lumberjack)
import "gopkg.in/natefinch/lumberjack.v2"

logger := &lumberjack.Logger{
    Filename:   "/var/log/hsm-service.log",
    MaxSize:    100, // MB
    MaxBackups: 10,
    MaxAge:     30, // days
    Compress:   true,
}

// 3. Add metrics
var (
    encryptRequests = prometheus.NewCounterVec(...)
    aclFailures     = prometheus.NewCounter(...)
)
```

---

### ✅ A10:2021 – SSRF (NOT APPLICABLE)

No outbound HTTP requests made by service.

---

## PCI DSS Compliance

### Requirement 3: Protect Stored Cardholder Data

| Control | Status | Notes |
|---------|--------|-------|
| 3.4: Render PAN unreadable | ✅ | KEK-wrapped encryption |
| 3.5: Document key management | ⚠️ | Missing rotation policy |
| 3.6: Key management procedures | 🔴 | **CRITICAL GAPS** |
| 3.6.1: Generation of strong keys | ✅ | AES-256 via HSM |
| 3.6.2: Secure key distribution | ✅ | Keys never leave HSM |
| 3.6.3: Secure key storage | ✅ | HSM-protected |
| 3.6.4: Key rotation | 🔴 | **NOT IMPLEMENTED** |
| 3.6.5: Retirement of old keys | 🔴 | **NOT IMPLEMENTED** |
| 3.6.6: Split knowledge/dual control | ⚠️ | Single HSM PIN |

---

### Requirement 4: Encrypt Transmission

| Control | Status | Notes |
|---------|--------|-------|
| 4.1: Strong cryptography (TLS) | ✅ | TLS 1.3, strong ciphers |
| 4.2: Never send unencrypted PAN | ✅ | mTLS enforced |

---

### Requirement 6: Secure Systems

| Control | Status | Notes |
|---------|--------|-------|
| 6.5.1: Injection flaws | ✅ | No SQL/command injection |
| 6.5.3: Insecure crypto | ⚠️ | No key rotation |
| 6.5.4: Insecure communications | ✅ | TLS 1.3 + mTLS |
| 6.5.8: Improper access control | ✅ | OU-based ACL |

---

### Requirement 8: Identify and Authenticate

| Control | Status | Notes |
|---------|--------|-------|
| 8.1: Assign unique ID | ✅ | Certificate CN |
| 8.2: Strong authentication | ✅ | Certificate-based |
| 8.3: Multi-factor authentication | ⚠️ | Single factor (cert only) |

---

### Requirement 10: Track and Monitor

| Control | Status | Notes |
|---------|--------|-------|
| 10.1: Audit trails | ✅ | AuditLogMiddleware |
| 10.2: Automated audit trails | ✅ | Every request logged |
| 10.3: Record audit details | ✅ | CN, OU, timestamp, duration |
| 10.7: Retain audit logs | ⚠️ | No retention policy |

---

## Critical Security Gaps

### 🔴 PRIORITY 1: Key Rotation (PCI DSS 3.6.4)

**Current State:** Keys never rotate  
**Required:** Rotate KEKs every 90-365 days

**Implementation:**
```go
type KeyRotationPolicy struct {
    RotationInterval time.Duration  // 90 days
    OverlapPeriod    time.Duration  // 7 days (old + new both valid)
}

// Automatic rotation process:
// 1. Generate new KEK (kek-exchange-v2)
// 2. Update config.yaml
// 3. Restart service
// 4. Re-encrypt data with new key
// 5. Delete old key after overlap period
```

---

### 🔴 PRIORITY 2: Request Size Limits

**Current State:** Unbounded request bodies  
**Risk:** DoS via 1GB JSON payload

**Fix:**
```go
// In EncryptHandler and DecryptHandler
const maxRequestSize = 1 * 1024 * 1024  // 1MB
r.Body = http.MaxBytesReader(w, r.Body, maxRequestSize)

// Also add to config.yaml
server:
  max_request_size: 1048576  # 1MB
```

---

### 🔴 PRIORITY 3: Memory Security

**Current State:** Plaintext persists in memory  
**Risk:** Memory dump exposes data

**Fix:**
```go
// Use golang.org/x/crypto/nacl/secretbox approach
// OR implement explicit zeroing

import "runtime"

func secureDecrypt(ciphertext []byte) ([]byte, error) {
    plaintext := make([]byte, len(ciphertext))
    defer func() {
        for i := range plaintext {
            plaintext[i] = 0
        }
        runtime.GC()  // Force GC to clear memory
    }()
    // ... decryption
    return plaintext, nil
}
```

---

### 🟠 PRIORITY 4: Server Timeouts

**Fix:**
```go
httpServer := &http.Server{
    Addr:              ":" + cfg.Port,
    Handler:           handler,
    TLSConfig:         tlsConfig,
    ReadTimeout:       10 * time.Second,
    ReadHeaderTimeout: 5 * time.Second,
    WriteTimeout:      10 * time.Second,
    IdleTimeout:       60 * time.Second,
    MaxHeaderBytes:    1 << 20,  // 1MB
}
```

---

### 🟠 PRIORITY 5: Rate Limiter Cleanup

**Fix:**
```go
type RateLimiter struct {
    limiters  map[string]*limiterEntry
    mu        sync.RWMutex
    rps       int
    burst     int
    cleanup   time.Duration
}

type limiterEntry struct {
    limiter  *rate.Limiter
    lastUsed time.Time
}

func (rl *RateLimiter) StartCleanup() {
    go func() {
        ticker := time.NewTicker(rl.cleanup)
        defer ticker.Stop()
        for range ticker.C {
            rl.cleanupStale(24 * time.Hour)
        }
    }()
}
```

---

## Compliance Summary

### PCI DSS Readiness: 70% ⚠️

**Compliant:**
- ✅ Requirement 4 (Encryption in transit)
- ✅ Requirement 8 (Authentication)
- ✅ Requirement 10 (Audit logging)

**Non-Compliant:**
- 🔴 Requirement 3.6.4 (Key rotation)
- 🔴 Requirement 3.6.5 (Key retirement)
- ⚠️ Requirement 6.5.3 (Cryptographic practices)

**Action Required:**
Implement key rotation before production deployment.

---

## Recommended Security Enhancements

### Short-term (Before Production)

1. **Add request limits** (DoS protection)
2. **Add server timeouts** (Slowloris protection)
3. **Implement memory zeroing** (Data security)
4. **Add rate limiter cleanup** (Memory leak)
5. **Document key rotation procedure** (PCI DSS compliance)

### Medium-term (First 90 days)

1. **Implement automated key rotation**
2. **Add SIEM integration** (log forwarding)
3. **Add Prometheus metrics**
4. **Implement alerting**
5. **Add log rotation**

### Long-term (Continuous Improvement)

1. **Pen testing**
2. **External security audit**
3. **FIPS 140-2 Level 3 HSM upgrade**
4. **Add hardware security module clustering**
5. **Implement key ceremony procedures**

---

## Security Checklist for Production

- [ ] Key rotation policy documented and implemented
- [ ] Request size limits configured (1MB)
- [ ] Server timeouts configured (10s read, 10s write)
- [ ] Rate limiter cleanup enabled
- [ ] Memory zeroing for sensitive data
- [ ] Log rotation configured
- [ ] Monitoring/alerting configured
- [ ] Security incident response plan
- [ ] Disaster recovery procedures
- [ ] Annual penetration testing scheduled
- [ ] Quarterly dependency updates
- [ ] Security training for operators

---

## Conclusion

The HSM service has a **solid security foundation** with strong cryptographic controls and access management. However, **critical gaps** in key management, resource limits, and operational security must be addressed before production deployment.

**Final Recommendation:**  
**Not ready for production** until Priority 1-3 issues are resolved.

**Timeline to Production Readiness:**  
Estimated 2-3 weeks of security hardening work.
