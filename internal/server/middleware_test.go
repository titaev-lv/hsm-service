package server

import (
	"context"
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"golang.org/x/time/rate"
)

// Rate Limiting Tests

func TestRateLimiter_GetLimiter(t *testing.T) {
	rl := NewRateLimiter(10, 5)

	// First call should create new limiter
	limiter1 := rl.GetLimiter("client-1")
	if limiter1 == nil {
		t.Fatal("Expected non-nil limiter")
	}

	// Second call should return same limiter
	limiter2 := rl.GetLimiter("client-1")
	if limiter1 != limiter2 {
		t.Error("Expected same limiter instance for same client")
	}

	// Different client should get different limiter
	limiter3 := rl.GetLimiter("client-2")
	if limiter1 == limiter3 {
		t.Error("Expected different limiter for different client")
	}
}

func TestRateLimitMiddleware_Allow(t *testing.T) {
	rl := NewRateLimiter(10, 5)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RateLimitMiddleware(rl)(handler)

	// Create request with client cert
	req := createRequestWithCert("GET", "/test", nil, "test-client", "TestOU")
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}
}

func TestRateLimitMiddleware_Exceed(t *testing.T) {
	// Very low limit for testing
	rl := NewRateLimiter(1, 1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RateLimitMiddleware(rl)(handler)

	// First request should succeed
	req1 := createRequestWithCert("GET", "/test", nil, "test-client", "TestOU")
	w1 := httptest.NewRecorder()
	middleware.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("First request: expected status 200, got %d", w1.Code)
	}

	// Second immediate request should be rate limited
	req2 := createRequestWithCert("GET", "/test", nil, "test-client", "TestOU")
	w2 := httptest.NewRecorder()
	middleware.ServeHTTP(w2, req2)

	if w2.Code != http.StatusTooManyRequests {
		t.Errorf("Second request: expected status 429, got %d", w2.Code)
	}

	// Check Retry-After header
	retryAfter := w2.Header().Get("Retry-After")
	if retryAfter != "1" {
		t.Errorf("Expected Retry-After header '1', got '%s'", retryAfter)
	}
}

func TestRateLimitMiddleware_PerClient(t *testing.T) {
	// Low limit for testing
	rl := NewRateLimiter(1, 1)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RateLimitMiddleware(rl)(handler)

	// Client 1 makes request
	req1 := createRequestWithCert("GET", "/test", nil, "client-1", "TestOU")
	w1 := httptest.NewRecorder()
	middleware.ServeHTTP(w1, req1)

	if w1.Code != http.StatusOK {
		t.Errorf("Client 1: expected status 200, got %d", w1.Code)
	}

	// Client 2 should still be able to make request (different limiter)
	req2 := createRequestWithCert("GET", "/test", nil, "client-2", "TestOU")
	w2 := httptest.NewRecorder()
	middleware.ServeHTTP(w2, req2)

	if w2.Code != http.StatusOK {
		t.Errorf("Client 2: expected status 200, got %d", w2.Code)
	}
}

func TestRateLimitMiddleware_NoCert(t *testing.T) {
	rl := NewRateLimiter(10, 5)

	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	})

	middleware := RateLimitMiddleware(rl)(handler)

	// Request without TLS certificate
	req := httptest.NewRequest("GET", "/test", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Errorf("Expected status 401, got %d", w.Code)
	}
}

func TestRecoveryMiddleware_Panic(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})

	middleware := RecoveryMiddleware(handler)

	req := httptest.NewRequest("GET", "/panic", nil)
	w := httptest.NewRecorder()

	middleware.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("expected status 500, got %d", w.Code)
	}
}

func TestAuditResponseWriter_WriteHeaderAndWrite(t *testing.T) {
	rec := httptest.NewRecorder()
	aw := &auditResponseWriter{ResponseWriter: rec, status: http.StatusOK, startTime: time.Now()}

	aw.WriteHeader(http.StatusCreated)
	if got := rec.Header().Get("X-HSM-Processing-Us"); got == "" {
		t.Fatalf("expected X-HSM-Processing-Us header to be set")
	}
	if aw.status != http.StatusCreated {
		t.Fatalf("status = %d, want %d", aw.status, http.StatusCreated)
	}

	n, err := aw.Write([]byte("ok"))
	if err != nil {
		t.Fatalf("Write() error: %v", err)
	}
	if n != 2 || aw.bytesWritten != 2 {
		t.Fatalf("n=%d bytesWritten=%d, want 2/2", n, aw.bytesWritten)
	}
}

func TestRequestIDFromContextAndAuditSetters(t *testing.T) {
	ctx := context.WithValue(context.Background(), requestIDKey, "req-123")
	if got := RequestIDFromContext(ctx); got != "req-123" {
		t.Fatalf("RequestIDFromContext()=%q, want req-123", got)
	}
	if got := RequestIDFromContext(context.Background()); got != "" {
		t.Fatalf("RequestIDFromContext(empty)=%q, want empty", got)
	}

	aw := &auditResponseWriter{ResponseWriter: httptest.NewRecorder(), status: http.StatusOK, startTime: time.Now()}
	SetAuditContext(aw, "exchange-key")
	SetAuditKeyID(aw, "kek-exchange-v1")
	SetAuditErrorCode(aw, "invalid_json")

	if aw.auditContext != "exchange-key" || aw.auditKeyID != "kek-exchange-v1" || aw.auditErrorCode != "invalid_json" {
		t.Fatalf("audit fields not set correctly: %+v", aw)
	}

	// No-op on plain ResponseWriter
	plainW := httptest.NewRecorder()
	SetAuditContext(plainW, "x")
	SetAuditKeyID(plainW, "y")
	SetAuditErrorCode(plainW, "z")
}

func TestGenerateRequestIDAndHelperMappings(t *testing.T) {
	id := generateRequestID()
	if len(id) != 32 {
		t.Fatalf("generateRequestID() length=%d, want 32", len(id))
	}

	if got := auditActionFromPath("/encrypt"); got != "encrypt" {
		t.Fatalf("auditActionFromPath(/encrypt)=%q", got)
	}
	if got := auditActionFromPath("/decrypt"); got != "decrypt" {
		t.Fatalf("auditActionFromPath(/decrypt)=%q", got)
	}
	if got := auditActionFromPath("/health"); got != "health" {
		t.Fatalf("auditActionFromPath(/health)=%q", got)
	}
	if got := auditActionFromPath("/unknown"); got != "unknown" {
		t.Fatalf("auditActionFromPath(/unknown)=%q", got)
	}

	if got := auditResultFromStatus(http.StatusOK); got != "success" {
		t.Fatalf("auditResultFromStatus(200)=%q", got)
	}
	if got := auditResultFromStatus(http.StatusForbidden); got != "denied" {
		t.Fatalf("auditResultFromStatus(403)=%q", got)
	}
	if got := auditResultFromStatus(http.StatusInternalServerError); got != "error" {
		t.Fatalf("auditResultFromStatus(500)=%q", got)
	}

	if got := tlsVersionString(tls.VersionTLS13); got != "TLS1.3" {
		t.Fatalf("tlsVersionString(TLS13)=%q", got)
	}
	if got := tlsVersionString(tls.VersionTLS12); got != "TLS1.2" {
		t.Fatalf("tlsVersionString(TLS12)=%q", got)
	}
	if got := tlsVersionString(tls.VersionTLS11); got != "TLS1.1" {
		t.Fatalf("tlsVersionString(TLS11)=%q", got)
	}
	if got := tlsVersionString(tls.VersionTLS10); got != "TLS1.0" {
		t.Fatalf("tlsVersionString(TLS10)=%q", got)
	}
	if got := tlsVersionString(0); got != "unknown" {
		t.Fatalf("tlsVersionString(unknown)=%q", got)
	}
}

func TestAuditLogMiddleware_SetsRequestIDAndHeaders(t *testing.T) {
	h := AuditLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if RequestIDFromContext(r.Context()) == "" {
			t.Fatalf("request id not propagated in context")
		}
		w.WriteHeader(http.StatusAccepted)
		_, _ = w.Write([]byte("ok"))
	}))

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	req.TLS = &tls.ConnectionState{}
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusAccepted {
		t.Fatalf("status=%d, want 202", w.Code)
	}
	if rid := w.Header().Get(requestIDHeader); rid == "" {
		t.Fatalf("expected %s header", requestIDHeader)
	}
	if p := w.Header().Get("X-HSM-Processing-Us"); p == "" {
		t.Fatalf("expected X-HSM-Processing-Us header")
	}
}

func TestRequestLogMiddleware_PassThrough(t *testing.T) {
	hit := false
	h := RequestLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hit = true
		w.WriteHeader(http.StatusNoContent)
	}))

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req = req.WithContext(context.WithValue(req.Context(), requestIDKey, "req-abc"))
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if !hit || w.Code != http.StatusNoContent {
		t.Fatalf("middleware did not pass through correctly: hit=%v status=%d", hit, w.Code)
	}
}

func TestRateLimiter_CleanupStale(t *testing.T) {
	rl := &RateLimiter{limiters: map[string]*limiterEntry{}, rps: 1, burst: 1}
	rl.limiters["old"] = &limiterEntry{limiter: rate.NewLimiter(1, 1), lastUsed: time.Now().Add(-48 * time.Hour)}
	rl.limiters["new"] = &limiterEntry{limiter: rate.NewLimiter(1, 1), lastUsed: time.Now()}

	rl.cleanupStale(24 * time.Hour)

	if _, ok := rl.limiters["old"]; ok {
		t.Fatalf("expected old limiter to be removed")
	}
	if _, ok := rl.limiters["new"]; !ok {
		t.Fatalf("expected new limiter to remain")
	}
}

func TestAuditLogMiddleware_StatusMappingsAndRequestIDPassthrough(t *testing.T) {
	statuses := []int{
		http.StatusUnauthorized,
		http.StatusForbidden,
		http.StatusTooManyRequests,
		http.StatusMethodNotAllowed,
		http.StatusBadRequest,
		http.StatusInternalServerError,
	}

	for _, status := range statuses {
		t.Run(http.StatusText(status), func(t *testing.T) {
			h := AuditLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				SetAuditContext(w, "exchange-key")
				SetAuditKeyID(w, "kek-exchange-v1")
				w.WriteHeader(status)
			}))

			req := createRequestWithCert(http.MethodPost, "/encrypt", []byte(`{"x":1}`), "client-a", "Trading")
			req.Header.Set(requestIDHeader, "req-fixed-id")
			req.ContentLength = -1 // cover access log branch where request size is not appended
			req.TLS.Version = tls.VersionTLS11
			req.TLS.CipherSuite = tls.TLS_AES_128_GCM_SHA256

			w := httptest.NewRecorder()
			h.ServeHTTP(w, req)

			if w.Code != status {
				t.Fatalf("status=%d, want %d", w.Code, status)
			}
			if rid := w.Header().Get(requestIDHeader); rid != "req-fixed-id" {
				t.Fatalf("request id header=%q, want req-fixed-id", rid)
			}
		})
	}
}

func TestAuditLogMiddleware_CustomAuditErrorCodePreserved(t *testing.T) {
	h := AuditLogMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		SetAuditErrorCode(w, "custom_error")
		w.WriteHeader(http.StatusBadRequest)
	}))

	req := createRequestWithCert(http.MethodPost, "/decrypt", []byte(`{"x":1}`), "client-b", "Risk")
	req.TLS.Version = tls.VersionTLS10
	req.TLS.CipherSuite = tls.TLS_CHACHA20_POLY1305_SHA256
	w := httptest.NewRecorder()

	h.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("status=%d, want %d", w.Code, http.StatusBadRequest)
	}
	if rid := w.Header().Get(requestIDHeader); rid == "" {
		t.Fatalf("expected %s header", requestIDHeader)
	}
}

func TestRateLimiter_CleanupLoopWithStop(t *testing.T) {
	rl := &RateLimiter{limiters: map[string]*limiterEntry{}, rps: 1, burst: 1}
	rl.limiters["old"] = &limiterEntry{limiter: rate.NewLimiter(1, 1), lastUsed: time.Now().Add(-48 * time.Hour)}

	stop := make(chan struct{})
	done := make(chan struct{})
	go func() {
		defer close(done)
		rl.cleanupLoopWithStop(5*time.Millisecond, 24*time.Hour, stop)
	}()

	deadline := time.Now().Add(250 * time.Millisecond)
	for {
		rl.mu.RLock()
		_, exists := rl.limiters["old"]
		rl.mu.RUnlock()
		if !exists {
			break
		}
		if time.Now().After(deadline) {
			close(stop)
			<-done
			t.Fatalf("stale limiter was not cleaned up in time")
		}
		time.Sleep(5 * time.Millisecond)
	}

	close(stop)
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatalf("cleanup loop did not stop after stop signal")
	}
}
