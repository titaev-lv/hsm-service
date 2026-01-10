package hsm

// CryptoProvider is the interface for encryption/decryption operations
type CryptoProvider interface {
	// Encrypt encrypts plaintext, returns ciphertext and key label
	Encrypt(plaintext []byte, context, clientCN string) (ciphertext []byte, keyLabel string, err error)

	// Decrypt decrypts ciphertext using the specified key label
	Decrypt(ciphertext []byte, context, clientCN, keyLabel string) ([]byte, error)

	// GetKeyLabels returns all available key labels
	GetKeyLabels() []string

	// HasKey checks if a key exists
	HasKey(label string) bool

	// GetKeyLabelByContext returns the current key label for a context
	GetKeyLabelByContext(context string) (string, error)

	// GetKeyMetadata returns metadata for a key
	GetKeyMetadata(label string) (*KeyMetadata, error)

	// GetKeysNeedingRotation returns keys that need rotation
	GetKeysNeedingRotation() []string
}
