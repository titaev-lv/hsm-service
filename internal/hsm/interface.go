package hsm

// CryptoProvider is the interface for encryption/decryption operations
type CryptoProvider interface {
	// Encrypt encrypts plaintext, returns ciphertext and key label
	// ou: organizational unit from client certificate (for shared mode)
	// clientCN: common name from client certificate (for private mode)
	Encrypt(plaintext []byte, context, ou, clientCN string) (ciphertext []byte, keyLabel string, err error)

	// Decrypt decrypts ciphertext using the specified key label
	// ou: organizational unit from client certificate (for shared mode)
	// clientCN: common name from client certificate (for private mode)
	Decrypt(ciphertext []byte, context, ou, clientCN, keyLabel string) ([]byte, error)

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
