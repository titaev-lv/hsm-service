package hsm

import (
	"crypto/rand"
	"crypto/sha256"
	"errors"
)

var (
	// ErrKeyNotFound is returned when the requested key is not available
	ErrKeyNotFound = errors.New("key not found")

	// ErrInvalidCiphertext is returned when ciphertext is too short or malformed
	ErrInvalidCiphertext = errors.New("invalid ciphertext")

	// ErrDecryptionFailed is returned when decryption fails (AAD mismatch or corrupted data)
	ErrDecryptionFailed = errors.New("decryption failed")
)

// ReadRandom fills the buffer with cryptographically secure random bytes
func ReadRandom(buf []byte) (int, error) {
	return rand.Read(buf)
}

// BuildAAD constructs Additional Authenticated Data from context and client identifier
// Mode determines which identifier to use:
// - "shared": uses OU (allows sharing within organizational unit)
// - "private": uses clientCN (isolated per client)
// Uses SHA-256 hashing to prevent AAD collisions from separator ambiguity
// Example: context="exchange" + CN="key|admin" vs context="exchange|key" + CN="admin"
// would produce different hashes, preventing confusion attacks
func BuildAAD(context, ou, clientCN, mode string) []byte {
	h := sha256.New()
	h.Write([]byte(context))
	h.Write([]byte{0x00}) // NULL byte separator (cannot appear in strings)

	// Choose identifier based on mode
	if mode == "shared" {
		h.Write([]byte(ou)) // Shared within OU
	} else {
		h.Write([]byte(clientCN)) // Private to specific client
	}

	return h.Sum(nil) // Returns 32-byte SHA-256 hash
}
