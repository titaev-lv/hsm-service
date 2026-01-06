package server

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"github.com/titaev-lv/hsm-service/internal/config"
	"github.com/titaev-lv/hsm-service/internal/hsm"
)

// createMockHSMContext creates a mock HSM context for testing
func createMockHSMContext() *hsm.HSMContext {
	// Create a test AES key (but we won't really use it in tests)
	key := make([]byte, 32)
	rand.Read(key)

	block, _ := aes.NewCipher(key)
	_, _ = cipher.NewGCM(block)

	// Return empty HSMContext - handlers will fail gracefully
	return &hsm.HSMContext{}
}

// Helper function to create a test request with client cert
func createRequestWithCert(method, path string, body []byte, cn, ou string) *http.Request {
	req := httptest.NewRequest(method, path, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	// Create fake TLS connection state
	cert := createTestCert(cn, ou)
	req.TLS = &tls.ConnectionState{
		PeerCertificates: []*x509.Certificate{cert},
	}

	return req
}

func TestEncryptHandler_InvalidJSON(t *testing.T) {
	hsmCtx := createMockHSMContext()

	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")
	os.WriteFile(revokedFile, []byte("revoked: []"), 0644)

	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings:    map[string][]string{},
	}
	aclChecker, _ := NewACLChecker(cfg)

	handler := EncryptHandler(hsmCtx, aclChecker)

	// Send invalid JSON
	req := createRequestWithCert("POST", "/encrypt", []byte("{invalid json}"), "test-service", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestEncryptHandler_ACLForbidden(t *testing.T) {
	hsmCtx := createMockHSMContext()

	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")
	os.WriteFile(revokedFile, []byte("revoked: []"), 0644)

	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings: map[string][]string{
			"Trading": {"exchange-key"},
			"2FA":     {"2fa"},
		},
	}
	aclChecker, _ := NewACLChecker(cfg)

	handler := EncryptHandler(hsmCtx, aclChecker)

	// Trading OU trying to access 2fa context (forbidden)
	reqBody := EncryptRequest{
		Context:   "2fa",
		Plaintext: base64.StdEncoding.EncodeToString([]byte("test")),
	}
	reqJSON, _ := json.Marshal(reqBody)

	req := createRequestWithCert("POST", "/encrypt", reqJSON, "trading-service-1", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Errorf("Expected status 403, got %d", w.Code)
	}
}

func TestEncryptHandler_MethodNotAllowed(t *testing.T) {
	hsmCtx := createMockHSMContext()

	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")
	os.WriteFile(revokedFile, []byte("revoked: []"), 0644)

	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings:    map[string][]string{},
	}
	aclChecker, _ := NewACLChecker(cfg)

	handler := EncryptHandler(hsmCtx, aclChecker)

	// Send GET instead of POST
	req := createRequestWithCert("GET", "/encrypt", nil, "test-service", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	hsmCtx := createMockHSMContext()

	handler := HealthHandler(hsmCtx)

	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK && w.Code != http.StatusServiceUnavailable {
		t.Errorf("Expected status 200 or 503, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}

	if resp.Status == "" {
		t.Error("Expected non-empty status")
	}

	if resp.KEKStatus == nil {
		t.Error("Expected KEKStatus map")
	}
}

func TestRespondJSON(t *testing.T) {
	w := httptest.NewRecorder()

	data := map[string]string{"message": "test"}
	respondJSON(w, http.StatusOK, data)

	if w.Code != http.StatusOK {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}
}

func TestRespondError(t *testing.T) {
	w := httptest.NewRecorder()

	respondError(w, http.StatusBadRequest, "test error")

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse error response: %v", err)
	}

	if resp.Error != "test error" {
		t.Errorf("Expected error 'test error', got '%s'", resp.Error)
	}
}
