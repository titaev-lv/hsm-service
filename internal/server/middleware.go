package server

import (
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

type contextKey string

const requestIDHeader = "X-Request-ID"

const requestIDKey contextKey = "request_id"

var (
	logMiddleware = slog.With("module", "middleware")
	logRateLimit  = slog.With("module", "rate_limit")
)

type auditResponseWriter struct {
	http.ResponseWriter
	status         int
	auditContext   string
	auditKeyID     string
	auditErrorCode string
	bytesWritten   int64
}

func (w *auditResponseWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

func (w *auditResponseWriter) Write(b []byte) (int, error) {
	n, err := w.ResponseWriter.Write(b)
	w.bytesWritten += int64(n)
	return n, err
}

func RequestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if id, ok := v.(string); ok {
			return id
		}
	}
	return ""
}

func SetAuditContext(w http.ResponseWriter, ctx string) {
	if aw, ok := w.(*auditResponseWriter); ok {
		aw.auditContext = ctx
	}
}

func SetAuditKeyID(w http.ResponseWriter, keyID string) {
	if aw, ok := w.(*auditResponseWriter); ok {
		aw.auditKeyID = keyID
	}
}

func SetAuditErrorCode(w http.ResponseWriter, code string) {
	if aw, ok := w.(*auditResponseWriter); ok {
		aw.auditErrorCode = code
	}
}

func generateRequestID() string {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return ""
	}
	return hex.EncodeToString(buf)
}

func auditActionFromPath(path string) string {
	switch path {
	case "/encrypt":
		return "encrypt"
	case "/decrypt":
		return "decrypt"
	case "/health":
		return "health"
	default:
		return "unknown"
	}
}

func auditResultFromStatus(status int) string {
	if status >= http.StatusOK && status < http.StatusMultipleChoices {
		return "success"
	}
	if status == http.StatusUnauthorized || status == http.StatusForbidden || status == http.StatusTooManyRequests {
		return "denied"
	}
	return "error"
}

func tlsVersionString(version uint16) string {
	switch version {
	case tls.VersionTLS13:
		return "TLS1.3"
	case tls.VersionTLS12:
		return "TLS1.2"
	case tls.VersionTLS11:
		return "TLS1.1"
	case tls.VersionTLS10:
		return "TLS1.0"
	default:
		return "unknown"
	}
}

// AuditLogMiddleware logs all requests with client information and duration
func AuditLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		requestID := r.Header.Get(requestIDHeader)
		if requestID == "" {
			requestID = generateRequestID()
		}
		if requestID != "" {
			w.Header().Set(requestIDHeader, requestID)
		}

		ctx := context.WithValue(r.Context(), requestIDKey, requestID)
		r = r.WithContext(ctx)

		aw := &auditResponseWriter{ResponseWriter: w, status: http.StatusOK}

		// Extract client certificate info
		var clientCN string
		var clientOU string
		var tlsVersion string
		var cipherSuite string
		var certSerial string
		var certIssuer string
		var certNotBefore string
		var certNotAfter string
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			cert := r.TLS.PeerCertificates[0]
			clientCN = cert.Subject.CommonName
			if len(cert.Subject.OrganizationalUnit) > 0 {
				clientOU = cert.Subject.OrganizationalUnit[0]
			}
			tlsVersion = tlsVersionString(r.TLS.Version)
			cipherSuite = tls.CipherSuiteName(r.TLS.CipherSuite)
			certSerial = cert.SerialNumber.String()
			certIssuer = cert.Issuer.String()
			certNotBefore = cert.NotBefore.UTC().Format("2006-01-02T15:04:05.000000Z")
			certNotAfter = cert.NotAfter.UTC().Format("2006-01-02T15:04:05.000000Z")
		}

		// Call next handler
		next.ServeHTTP(aw, r)

		// Record request duration metric
		duration := time.Since(start).Seconds()
		RequestDuration.WithLabelValues(r.URL.Path).Observe(duration)

		auditErrorCode := aw.auditErrorCode
		if auditErrorCode == "" {
			switch aw.status {
			case http.StatusUnauthorized:
				auditErrorCode = "unauthorized"
			case http.StatusForbidden:
				auditErrorCode = "acl_denied"
			case http.StatusTooManyRequests:
				auditErrorCode = "rate_limit"
			case http.StatusMethodNotAllowed:
				auditErrorCode = "method_not_allowed"
			case http.StatusBadRequest:
				auditErrorCode = "bad_request"
			case http.StatusInternalServerError:
				auditErrorCode = "internal_error"
			}
		}

		// Log audit event
		AuditLogger().Info("request",
			"request_id", requestID,
			"event_type", "audit",
			"action", auditActionFromPath(r.URL.Path),
			"method", r.Method,
			"path", r.URL.Path,
			"status_code", aw.status,
			"result", auditResultFromStatus(aw.status),
			"error_code", auditErrorCode,
			"client_cn", clientCN,
			"client_ou", clientOU,
			"remote_addr", r.RemoteAddr,
			"duration_ms", time.Since(start).Milliseconds(),
			"context", aw.auditContext,
			"key_id", aw.auditKeyID,
			"tls_version", tlsVersion,
			"cipher_suite", cipherSuite,
			"client_cert_serial", certSerial,
			"client_cert_issuer", certIssuer,
			"client_cert_not_before", certNotBefore,
			"client_cert_not_after", certNotAfter,
		)

		accessFields := []any{
			"request_id", requestID,
			"method", r.Method,
			"path", r.URL.Path,
			"status", aw.status,
			"duration_ms", time.Since(start).Milliseconds(),
			"client_ip", r.RemoteAddr,
			"user_agent", r.UserAgent(),
			"tls_version", tlsVersion,
		}
		if r.ContentLength >= 0 {
			accessFields = append(accessFields, "request_size_bytes", r.ContentLength)
		}
		accessFields = append(accessFields, "response_size_bytes", aw.bytesWritten)

		AccessLogger().Info("request", accessFields...)
	})
}

// RecoveryMiddleware recovers from panics and logs them
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				reqID := RequestIDFromContext(r.Context())
				logMiddleware.Error("panic recovered",
					"request_id", reqID,
					"error", err,
					"method", r.Method,
					"path", r.URL.Path,
				)
				http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			}
		}()
		next.ServeHTTP(w, r)
	})
}

// RequestLogMiddleware logs basic request information
func RequestLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := RequestIDFromContext(r.Context())
		logMiddleware.Debug("incoming request",
			"request_id", reqID,
			"method", r.Method,
			"path", r.URL.Path,
			"remote_addr", r.RemoteAddr,
		)
		next.ServeHTTP(w, r)
	})
}

// RateLimiter manages per-client rate limiters
type RateLimiter struct {
	limiters map[string]*limiterEntry // CN -> limiter entry
	mu       sync.RWMutex
	rps      int
	burst    int
}

// limiterEntry wraps a rate limiter with usage tracking
type limiterEntry struct {
	limiter  *rate.Limiter
	lastUsed time.Time
}

// NewRateLimiter creates a new rate limiter
func NewRateLimiter(rps, burst int) *RateLimiter {
	rl := &RateLimiter{
		limiters: make(map[string]*limiterEntry),
		rps:      rps,
		burst:    burst,
	}
	// Start cleanup goroutine
	go rl.cleanupLoop()
	return rl
}

// GetLimiter returns the rate limiter for a client CN
func (rl *RateLimiter) GetLimiter(clientCN string) *rate.Limiter {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	entry, exists := rl.limiters[clientCN]
	if !exists {
		entry = &limiterEntry{
			limiter:  rate.NewLimiter(rate.Limit(rl.rps), rl.burst),
			lastUsed: time.Now(),
		}
		rl.limiters[clientCN] = entry
	} else {
		// Update last used time
		entry.lastUsed = time.Now()
	}

	return entry.limiter
}

// cleanupLoop removes rate limiters that haven't been used in 24 hours
func (rl *RateLimiter) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		rl.cleanupStale(24 * time.Hour)
	}
}

// cleanupStale removes limiters not used within the specified duration
func (rl *RateLimiter) cleanupStale(maxAge time.Duration) {
	rl.mu.Lock()
	defer rl.mu.Unlock()

	now := time.Now()
	var removed int

	for cn, entry := range rl.limiters {
		if now.Sub(entry.lastUsed) > maxAge {
			delete(rl.limiters, cn)
			removed++
		}
	}

	if removed > 0 {
		logRateLimit.Info("rate limiter cleanup",
			"removed", removed,
			"remaining", len(rl.limiters),
		)
	}
}

// RateLimitMiddleware applies per-client rate limiting
func RateLimitMiddleware(limiter *RateLimiter) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Extract client CN from certificate
			if r.TLS == nil || len(r.TLS.PeerCertificates) == 0 {
				SetAuditErrorCode(w, "no_client_cert")
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			clientCN := r.TLS.PeerCertificates[0].Subject.CommonName

			// Check rate limit
			if !limiter.GetLimiter(clientCN).Allow() {
				reqID := RequestIDFromContext(r.Context())
				logRateLimit.Warn("rate limit exceeded",
					"request_id", reqID,
					"client_cn", clientCN,
					"path", r.URL.Path,
				)
				// Metrics: track rate limit hit
				RecordRateLimitHit(clientCN)
				w.Header().Set("Retry-After", "1")
				SetAuditErrorCode(w, "rate_limit")
				respondError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
