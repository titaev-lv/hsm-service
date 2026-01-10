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
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
	"github.com/titaev-lv/hsm-service/internal/hsm"
)

// mockKeyManager implements a minimal KeyManager interface for testing
type mockKeyManager struct {
	keys           map[string]cipher.AEAD
	contextToLabel map[string]string
}

func (m *mockKeyManager) Encrypt(plaintext []byte, context, clientCN string) ([]byte, string, error) {
	// Return mock encrypted data
	return []byte("mock-ciphertext"), "mock-key-v1", nil
}

func (m *mockKeyManager) Decrypt(ciphertext []byte, context, clientCN, keyLabel string) ([]byte, error) {
	// Return mock decrypted data
	return []byte("mock-plaintext"), nil
}

func (m *mockKeyManager) GetKeyLabels() []string {
	labels := make([]string, 0, len(m.keys))
	for label := range m.keys {
		labels = append(labels, label)
	}
	return labels
}

func (m *mockKeyManager) HasKey(label string) bool {
	_, exists := m.keys[label]
	return exists
}

func (m *mockKeyManager) GetKeyLabelByContext(context string) (string, error) {
	return "mock-key-v1", nil
}

func (m *mockKeyManager) GetKeyMetadata(label string) (*hsm.KeyMetadata, error) {
	// Return mock metadata
	return &hsm.KeyMetadata{
		Label:            label,
		CreatedAt:        time.Now(),
		RotationInterval: 0,
		Version:          1,
	}, nil
}

func (m *mockKeyManager) GetKeysNeedingRotation() []string {
	// No keys need rotation in tests
	return []string{}
}

// createMockKeyManager creates a mock KeyManager for testing
func createMockKeyManager() *mockKeyManager {
	// Create a test AES key
	key := make([]byte, 32)
	rand.Read(key)

	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)

	return &mockKeyManager{
		keys: map[string]cipher.AEAD{
			"mock-key-v1": gcm,
		},
		contextToLabel: map[string]string{
			"exchange-key": "mock-key-v1",
			"2fa":          "mock-key-v1",
		},
	}
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
	keyManager := createMockKeyManager()

	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")
	os.WriteFile(revokedFile, []byte("revoked: []"), 0644)

	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings:    map[string][]string{},
	}
	aclChecker, _ := NewACLChecker(cfg)

	handler := EncryptHandler(keyManager, aclChecker)

	// Send invalid JSON
	req := createRequestWithCert("POST", "/encrypt", []byte("{invalid json}"), "test-service", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("Expected status 400, got %d", w.Code)
	}
}

func TestEncryptHandler_ACLForbidden(t *testing.T) {
	keyManager := createMockKeyManager()

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

	handler := EncryptHandler(keyManager, aclChecker)

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
	keyManager := createMockKeyManager()

	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")
	os.WriteFile(revokedFile, []byte("revoked: []"), 0644)

	cfg := &config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings:    map[string][]string{},
	}
	aclChecker, _ := NewACLChecker(cfg)

	handler := EncryptHandler(keyManager, aclChecker)

	// Send GET instead of POST
	req := createRequestWithCert("GET", "/encrypt", nil, "test-service", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusMethodNotAllowed {
		t.Errorf("Expected status 405, got %d", w.Code)
	}
}

func TestHealthHandler(t *testing.T) {
	keyManager := createMockKeyManager()

	handler := HealthHandler(keyManager)

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
