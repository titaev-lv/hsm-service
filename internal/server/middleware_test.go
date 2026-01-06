package server

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
