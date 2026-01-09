package server

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

// AuditLogMiddleware logs all requests with client information and duration
func AuditLogMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// Extract client certificate info
		var clientCN string
		var clientOU string
		if r.TLS != nil && len(r.TLS.PeerCertificates) > 0 {
			cert := r.TLS.PeerCertificates[0]
			clientCN = cert.Subject.CommonName
			if len(cert.Subject.OrganizationalUnit) > 0 {
				clientOU = cert.Subject.OrganizationalUnit[0]
			}
		}

		// Call next handler
		next.ServeHTTP(w, r)

		// Log audit event
		AuditLogger().Info("request",
			"method", r.Method,
			"path", r.URL.Path,
			"client_cn", clientCN,
			"client_ou", clientOU,
			"remote_addr", r.RemoteAddr,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	})
}

// RecoveryMiddleware recovers from panics and logs them
func RecoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				slog.Error("panic recovered",
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
		slog.Debug("incoming request",
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
		slog.Info("rate limiter cleanup",
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
				http.Error(w, "Unauthorized", http.StatusUnauthorized)
				return
			}

			clientCN := r.TLS.PeerCertificates[0].Subject.CommonName

			// Check rate limit
			if !limiter.GetLimiter(clientCN).Allow() {
				slog.Warn("rate limit exceeded",
					"client_cn", clientCN,
					"path", r.URL.Path,
				)
				w.Header().Set("Retry-After", "1")
				respondError(w, http.StatusTooManyRequests, "rate limit exceeded")
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
