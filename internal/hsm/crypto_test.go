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
expected string
}{
{
name:     "standard AAD",
context:  "exchange-key",
clientCN: "trading-service-1",
expected: "exchange-key|trading-service-1",
},
{
name:     "2fa context",
context:  "2fa",
clientCN: "web-2fa-service",
expected: "2fa|web-2fa-service",
},
}

for _, tt := range tests {
t.Run(tt.name, func(t *testing.T) {
aad := BuildAAD(tt.context, tt.clientCN)
if string(aad) != tt.expected {
t.Errorf("BuildAAD() = %s, want %s", string(aad), tt.expected)
}
})
}
}

func TestEncryptDecrypt(t *testing.T) {
hsm := createMockHSMContext(t)

plaintext := []byte("secret data for testing")
aad := BuildAAD("exchange-key", "trading-service-1")

// Test encrypt
ciphertext, err := hsm.Encrypt(plaintext, aad, "test-key")
if err != nil {
t.Fatalf("Encrypt() error = %v", err)
}

// Ciphertext should be longer than plaintext (nonce + ciphertext + tag)
if len(ciphertext) <= len(plaintext) {
t.Errorf("Ciphertext length %d should be > plaintext length %d", len(ciphertext), len(plaintext))
}

// Test decrypt
decrypted, err := hsm.Decrypt(ciphertext, aad, "test-key")
if err != nil {
t.Fatalf("Decrypt() error = %v", err)
}

// Verify round-trip
if !bytes.Equal(plaintext, decrypted) {
t.Errorf("Decrypted data doesn't match original.\nWant: %s\nGot:  %s", plaintext, decrypted)
}
}

func TestAADMismatch(t *testing.T) {
hsm := createMockHSMContext(t)

plaintext := []byte("secret data")
aad1 := BuildAAD("exchange-key", "trading-service-1")
aad2 := BuildAAD("2fa", "trading-service-1") // Different context

// Encrypt with aad1
ciphertext, err := hsm.Encrypt(plaintext, aad1, "test-key")
if err != nil {
t.Fatalf("Encrypt() error = %v", err)
}

// Try to decrypt with aad2 (should fail)
_, err = hsm.Decrypt(ciphertext, aad2, "test-key")
if err == nil {
t.Error("Decrypt() with wrong AAD should fail, but succeeded")
}

// Verify it's a decryption failure
if err != ErrDecryptionFailed && !bytes.Contains([]byte(err.Error()), []byte("decryption failed")) {
t.Errorf("Expected ErrDecryptionFailed, got: %v", err)
}
}

func TestKeyNotFound(t *testing.T) {
hsm := createMockHSMContext(t)

plaintext := []byte("secret data")
aad := BuildAAD("exchange-key", "trading-service-1")

// Try to encrypt with non-existent key
_, err := hsm.Encrypt(plaintext, aad, "nonexistent-key")
if err == nil {
t.Error("Encrypt() with non-existent key should fail")
}

if err != ErrKeyNotFound && !bytes.Contains([]byte(err.Error()), []byte("key not found")) {
t.Errorf("Expected ErrKeyNotFound, got: %v", err)
}
}

func TestInvalidCiphertext(t *testing.T) {
hsm := createMockHSMContext(t)

aad := BuildAAD("exchange-key", "trading-service-1")

// Test with too short ciphertext
shortCiphertext := []byte("short")
_, err := hsm.Decrypt(shortCiphertext, aad, "test-key")
if err == nil {
t.Error("Decrypt() with short ciphertext should fail")
}

if err != ErrInvalidCiphertext {
t.Errorf("Expected ErrInvalidCiphertext, got: %v", err)
}
}

func TestEmptyPlaintext(t *testing.T) {
hsm := createMockHSMContext(t)

plaintext := []byte("")
aad := BuildAAD("exchange-key", "trading-service-1")

// Encrypt empty data
ciphertext, err := hsm.Encrypt(plaintext, aad, "test-key")
if err != nil {
t.Fatalf("Encrypt() error = %v", err)
}

// Decrypt
decrypted, err := hsm.Decrypt(ciphertext, aad, "test-key")
if err != nil {
t.Fatalf("Decrypt() error = %v", err)
}

// Verify
if !bytes.Equal(plaintext, decrypted) {
t.Errorf("Decrypted data doesn't match empty plaintext")
}
}
