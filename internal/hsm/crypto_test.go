package hsm

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"testing"
)

// createMockHSMContext creates a mock HSM context for testing
func createMockHSMContext(t *testing.T) *HSMContext {
	// Create a test AES key (256 bits)
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	// Create AES cipher
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("Failed to create AES cipher: %v", err)
	}

	// Create GCM
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("Failed to create GCM: %v", err)
	}

	return &HSMContext{
		ctx: nil, // No real PKCS#11 context for tests
		keys: map[string]cipher.AEAD{
			"test-key": gcm,
		},
	}
}

func TestBuildAAD(t *testing.T) {
	tests := []struct {
		name     string
		context  string
		clientCN string
	}{
		{
			name:     "standard AAD",
			context:  "exchange-key",
			clientCN: "trading-service-1",
		},
		{
			name:     "2fa context",
			context:  "2fa",
			clientCN: "web-2fa-service",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aad := BuildAAD(tt.context, tt.clientCN)
			// AAD should be 32 bytes (SHA-256 hash)
			if len(aad) != 32 {
				t.Errorf("BuildAAD() length = %d, want 32", len(aad))
			}
			// AAD should be deterministic
			aad2 := BuildAAD(tt.context, tt.clientCN)
			if !bytes.Equal(aad, aad2) {
				t.Error("BuildAAD() not deterministic")
			}
		})
	}
}

func TestEncryptDecrypt(t *testing.T) {
	hsm := createMockHSMContext(t)

	plaintext := []byte("secret data for testing")
	context := "exchange-key"
	clientCN := "trading-service-1"

	// Test encrypt
	ciphertext, err := hsm.Encrypt(plaintext, context, clientCN, "test-key")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Ciphertext should be longer than plaintext (nonce + ciphertext + tag)
	if len(ciphertext) <= len(plaintext) {
		t.Errorf("Ciphertext length %d should be > plaintext length %d", len(ciphertext), len(plaintext))
	}

	// Test decrypt
	decrypted, err := hsm.Decrypt(ciphertext, context, clientCN, "test-key")
	// Verify round-trip
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted data doesn't match original.\nWant: %s\nGot:  %s", plaintext, decrypted)
	}
}

func TestAADMismatch(t *testing.T) {
	hsm := createMockHSMContext(t)

	plaintext := []byte("secret data")
	context1 := "exchange-key"
	context2 := "2fa" // Different context
	clientCN := "trading-service-1"

	// Encrypt with context1
	ciphertext, err := hsm.Encrypt(plaintext, context1, clientCN, "test-key")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Try to decrypt with context2 (should fail due to AAD mismatch)
	_, err = hsm.Decrypt(ciphertext, context2, clientCN, "test-key")
	if err == nil {
		t.Error("Decrypt() with wrong context should fail, but succeeded")
		t.Errorf("Expected ErrDecryptionFailed, got: %v", err)
	}
}

func TestKeyNotFound(t *testing.T) {
	hsm := createMockHSMContext(t)

	plaintext := []byte("secret data")
	context := "exchange-key"
	clientCN := "trading-service-1"

	// Try to encrypt with non-existent key
	_, err := hsm.Encrypt(plaintext, context, clientCN, "nonexistent-key")
	if err != ErrKeyNotFound && !bytes.Contains([]byte(err.Error()), []byte("key not found")) {
		t.Errorf("Expected ErrKeyNotFound, got: %v", err)
	}
}

func TestInvalidCiphertext(t *testing.T) {
	hsm := createMockHSMContext(t)

	context := "exchange-key"
	clientCN := "trading-service-1"

	// Test with too short ciphertext
	shortCiphertext := []byte("short")
	_, err := hsm.Decrypt(shortCiphertext, context, clientCN, "test-key")

	if err != ErrInvalidCiphertext {
		t.Errorf("Expected ErrInvalidCiphertext, got: %v", err)
	}
}

func TestEmptyPlaintext(t *testing.T) {
	hsm := createMockHSMContext(t)

	plaintext := []byte("")
	context := "exchange-key"
	clientCN := "trading-service-1"

	// Encrypt empty data
	ciphertext, err := hsm.Encrypt(plaintext, context, clientCN, "test-key")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Decrypt
	decrypted, err := hsm.Decrypt(ciphertext, context, clientCN, "test-key")
	// Verify
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted data doesn't match empty plaintext")
	}
}
