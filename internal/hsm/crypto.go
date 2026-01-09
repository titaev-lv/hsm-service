package hsm

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
	"fmt"
)

var (
	// ErrKeyNotFound is returned when the requested key is not available
	ErrKeyNotFound = errors.New("key not found")

	// ErrInvalidCiphertext is returned when ciphertext is too short or malformed
	ErrInvalidCiphertext = errors.New("invalid ciphertext")

	// ErrDecryptionFailed is returned when decryption fails (AAD mismatch or corrupted data)
	ErrDecryptionFailed = errors.New("decryption failed")
)

// BuildAAD constructs Additional Authenticated Data from context and client CN
// Uses SHA-256 hashing to prevent AAD collisions from separator ambiguity
// BuildAAD constructs Additional Authenticated Data from context and client CN
// Uses SHA-256 hashing to prevent AAD collisions from separator ambiguity
// Example: context="exchange" + CN="key|admin" vs context="exchange|key" + CN="admin"
// would produce different hashes, preventing confusion attacks
func BuildAAD(context, clientCN string) []byte {
	h := sha256.New()
	h.Write([]byte(context))
	h.Write([]byte{0x00}) // NULL byte separator (cannot appear in strings)
	h.Write([]byte(clientCN))
	return h.Sum(nil) // Returns 32-byte SHA-256 hash
}

// Encrypt encrypts plaintext using AES-GCM with the specified key
// Returns: nonce (12 bytes) || ciphertext || tag (16 bytes)
func (h *HSMContext) Encrypt(plaintext []byte, aad []byte, keyLabel string) ([]byte, error) {
	// 1. Get GCM cipher for the key
	gcm, ok := h.keys[keyLabel]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrKeyNotFound, keyLabel)
	}

	// 2. Generate random nonce (12 bytes for GCM)
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	// 3. Encrypt with AAD
	// Seal appends ciphertext and tag to nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, aad)

	return ciphertext, nil
}

// Decrypt decrypts ciphertext using AES-GCM with the specified key
// Expects: nonce (12 bytes) || ciphertext || tag (16 bytes)
func (h *HSMContext) Decrypt(ciphertext []byte, aad []byte, keyLabel string) ([]byte, error) {
	// 1. Get GCM cipher for the key
	gcm, ok := h.keys[keyLabel]
	if !ok {
		return nil, fmt.Errorf("%w: %s", ErrKeyNotFound, keyLabel)
	}

	// 2. Validate ciphertext length
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	// 3. Extract nonce and encrypted data
	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	// 4. Decrypt with AAD verification
	plaintext, err := gcm.Open(nil, nonce, encrypted, aad)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return plaintext, nil
}
