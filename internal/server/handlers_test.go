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
	"errors"
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
	keyLabels      []string
	hasKeyByLabel  map[string]bool
	encryptResult  []byte
	encryptKeyID   string
	encryptErr     error
	decryptResult  []byte
	decryptErr     error
}

func (m *mockKeyManager) Encrypt(plaintext []byte, context, ou, clientCN string) ([]byte, string, error) {
	if m.encryptErr != nil {
		return nil, "", m.encryptErr
	}
	if m.encryptResult != nil {
		return m.encryptResult, m.encryptKeyID, nil
	}
	return []byte("mock-ciphertext"), "mock-key-v1", nil
}

func (m *mockKeyManager) Decrypt(ciphertext []byte, context, ou, clientCN, keyLabel string) ([]byte, error) {
	if m.decryptErr != nil {
		return nil, m.decryptErr
	}
	if m.decryptResult != nil {
		return m.decryptResult, nil
	}
	return []byte("mock-plaintext"), nil
}

func (m *mockKeyManager) GetKeyLabels() []string {
	if m.keyLabels != nil {
		labels := make([]string, len(m.keyLabels))
		copy(labels, m.keyLabels)
		return labels
	}

	labels := make([]string, 0, len(m.keys))
	for label := range m.keys {
		labels = append(labels, label)
	}
	return labels
}

func (m *mockKeyManager) HasKey(label string) bool {
	if m.hasKeyByLabel != nil {
		exists, ok := m.hasKeyByLabel[label]
		if ok {
			return exists
		}
	}

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
		encryptKeyID:  "mock-key-v1",
		decryptResult: []byte("mock-plaintext"),
	}
}

func createTestACLChecker(t *testing.T, mappings map[string][]string) *ACLChecker {
	t.Helper()

	tmpDir := t.TempDir()
	revokedFile := filepath.Join(tmpDir, "revoked.yaml")
	if err := os.WriteFile(revokedFile, []byte("revoked_certificates: []\n"), 0o644); err != nil {
		t.Fatalf("os.WriteFile(revoked): %v", err)
	}

	checker, err := NewACLChecker(&config.ACLConfig{
		RevokedFile: revokedFile,
		Mappings:    mappings,
	})
	if err != nil {
		t.Fatalf("NewACLChecker() error: %v", err)
	}

	return checker
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

func TestHealthHandler_DegradedWhenAnyKeyUnavailable(t *testing.T) {
	keyManager := createMockKeyManager()
	keyManager.keyLabels = []string{"kek-a", "kek-b"}
	keyManager.hasKeyByLabel = map[string]bool{
		"kek-a": true,
		"kek-b": false,
	}

	handler := HealthHandler(keyManager)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("Expected status 503 for degraded health, got %d", w.Code)
	}

	var resp HealthResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("Failed to parse health response: %v", err)
	}
	if resp.Status != "degraded" {
		t.Fatalf("Expected status degraded, got %q", resp.Status)
	}
	if resp.HSMAvailable {
		t.Fatalf("Expected HSMAvailable=false")
	}
	if resp.KEKStatus["kek-a"] != "available" || resp.KEKStatus["kek-b"] != "unavailable" {
		t.Fatalf("Unexpected KEK status map: %+v", resp.KEKStatus)
	}
}

func TestEncryptHandler_NoClientCertificate(t *testing.T) {
	keyManager := createMockKeyManager()
	aclChecker := createTestACLChecker(t, map[string][]string{"Trading": {"exchange-key"}})
	handler := EncryptHandler(keyManager, aclChecker)

	reqBody, _ := json.Marshal(EncryptRequest{Context: "exchange-key", Plaintext: base64.StdEncoding.EncodeToString([]byte("abc"))})
	req := httptest.NewRequest(http.MethodPost, "/encrypt", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected status 401, got %d", w.Code)
	}
}

func TestEncryptHandler_InvalidBase64Plaintext(t *testing.T) {
	keyManager := createMockKeyManager()
	aclChecker := createTestACLChecker(t, map[string][]string{"Trading": {"exchange-key"}})
	handler := EncryptHandler(keyManager, aclChecker)

	reqJSON, _ := json.Marshal(EncryptRequest{Context: "exchange-key", Plaintext: "%%%not-base64%%%"})
	req := createRequestWithCert(http.MethodPost, "/encrypt", reqJSON, "trading-service-1", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", w.Code)
	}
}

func TestEncryptHandler_EncryptionFailure(t *testing.T) {
	keyManager := createMockKeyManager()
	keyManager.encryptErr = errors.New("encrypt failed")
	aclChecker := createTestACLChecker(t, map[string][]string{"Trading": {"exchange-key"}})
	handler := EncryptHandler(keyManager, aclChecker)

	reqJSON, _ := json.Marshal(EncryptRequest{
		Context:   "exchange-key",
		Plaintext: base64.StdEncoding.EncodeToString([]byte("plaintext")),
	})
	req := createRequestWithCert(http.MethodPost, "/encrypt", reqJSON, "trading-service-1", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusInternalServerError {
		t.Fatalf("Expected status 500, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(error response): %v", err)
	}
	if resp.Error != "encryption failed" {
		t.Fatalf("Expected generic encryption error, got %q", resp.Error)
	}
}

func TestEncryptHandler_Success(t *testing.T) {
	keyManager := createMockKeyManager()
	keyManager.encryptResult = []byte("cipher-bytes")
	keyManager.encryptKeyID = "mock-key-v2"
	aclChecker := createTestACLChecker(t, map[string][]string{"Trading": {"exchange-key"}})
	handler := EncryptHandler(keyManager, aclChecker)

	reqJSON, _ := json.Marshal(EncryptRequest{
		Context:   "exchange-key",
		Plaintext: base64.StdEncoding.EncodeToString([]byte("plaintext")),
	})
	req := createRequestWithCert(http.MethodPost, "/encrypt", reqJSON, "trading-service-1", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp EncryptResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(success response): %v", err)
	}
	if resp.KeyID != "mock-key-v2" {
		t.Fatalf("Unexpected key ID: %q", resp.KeyID)
	}
	if resp.Ciphertext != base64.StdEncoding.EncodeToString([]byte("cipher-bytes")) {
		t.Fatalf("Unexpected ciphertext: %q", resp.Ciphertext)
	}
}

func TestDecryptHandler_InvalidJSON(t *testing.T) {
	keyManager := createMockKeyManager()
	aclChecker := createTestACLChecker(t, map[string][]string{"Trading": {"exchange-key"}})
	handler := DecryptHandler(keyManager, aclChecker)

	req := createRequestWithCert("POST", "/decrypt", []byte("{invalid json}"), "trading-service-1", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", w.Code)
	}
}

func TestDecryptHandler_NoClientCertificate(t *testing.T) {
	keyManager := createMockKeyManager()
	aclChecker := createTestACLChecker(t, map[string][]string{"Trading": {"exchange-key"}})
	handler := DecryptHandler(keyManager, aclChecker)

	reqBody, _ := json.Marshal(DecryptRequest{Context: "exchange-key", Ciphertext: base64.StdEncoding.EncodeToString([]byte("abc")), KeyID: "mock-key-v1"})
	req := httptest.NewRequest(http.MethodPost, "/decrypt", bytes.NewReader(reqBody))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("Expected status 401, got %d", w.Code)
	}
}

func TestDecryptHandler_ACLForbidden(t *testing.T) {
	keyManager := createMockKeyManager()
	aclChecker := createTestACLChecker(t, map[string][]string{"Trading": {"exchange-key"}, "2FA": {"2fa"}})
	handler := DecryptHandler(keyManager, aclChecker)

	reqJSON, _ := json.Marshal(DecryptRequest{
		Context:    "2fa",
		Ciphertext: base64.StdEncoding.EncodeToString([]byte("ciphertext")),
		KeyID:      "mock-key-v1",
	})
	req := createRequestWithCert(http.MethodPost, "/decrypt", reqJSON, "trading-service-1", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("Expected status 403, got %d", w.Code)
	}
}

func TestDecryptHandler_InvalidBase64Ciphertext(t *testing.T) {
	keyManager := createMockKeyManager()
	aclChecker := createTestACLChecker(t, map[string][]string{"Trading": {"exchange-key"}})
	handler := DecryptHandler(keyManager, aclChecker)

	reqJSON, _ := json.Marshal(DecryptRequest{
		Context:    "exchange-key",
		Ciphertext: "%%%not-base64%%%",
		KeyID:      "mock-key-v1",
	})
	req := createRequestWithCert(http.MethodPost, "/decrypt", reqJSON, "trading-service-1", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", w.Code)
	}
}

func TestDecryptHandler_DecryptionFailure(t *testing.T) {
	keyManager := createMockKeyManager()
	keyManager.decryptErr = hsm.ErrDecryptionFailed
	aclChecker := createTestACLChecker(t, map[string][]string{"Trading": {"exchange-key"}})
	handler := DecryptHandler(keyManager, aclChecker)

	reqJSON, _ := json.Marshal(DecryptRequest{
		Context:    "exchange-key",
		Ciphertext: base64.StdEncoding.EncodeToString([]byte("ciphertext")),
		KeyID:      "mock-key-v1",
	})
	req := createRequestWithCert(http.MethodPost, "/decrypt", reqJSON, "trading-service-1", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("Expected status 400, got %d", w.Code)
	}

	var resp ErrorResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(error response): %v", err)
	}
	if resp.Error != "decryption failed" {
		t.Fatalf("Expected generic decryption error, got %q", resp.Error)
	}
}

func TestDecryptHandler_Success(t *testing.T) {
	keyManager := createMockKeyManager()
	keyManager.decryptResult = []byte("decrypted payload")
	aclChecker := createTestACLChecker(t, map[string][]string{"Trading": {"exchange-key"}})
	handler := DecryptHandler(keyManager, aclChecker)

	reqJSON, _ := json.Marshal(DecryptRequest{
		Context:    "exchange-key",
		Ciphertext: base64.StdEncoding.EncodeToString([]byte("ciphertext")),
		KeyID:      "mock-key-v1",
	})
	req := createRequestWithCert(http.MethodPost, "/decrypt", reqJSON, "trading-service-1", "Trading")
	w := httptest.NewRecorder()

	handler.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("Expected status 200, got %d", w.Code)
	}

	var resp DecryptResponse
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("json.Unmarshal(success response): %v", err)
	}
	if resp.Plaintext != base64.StdEncoding.EncodeToString([]byte("decrypted payload")) {
		t.Fatalf("Unexpected plaintext: %q", resp.Plaintext)
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
