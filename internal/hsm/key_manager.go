package hsm

import (
	"context"
	"crypto/cipher"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/ThalesGroup/crypto11"
	"github.com/titaev-lv/hsm-service/internal/config"
)

// KeyManager manages HSM keys with hot reload capability
type KeyManager struct {
	// PKCS#11 context (persistent, never closed during reload)
	ctx *crypto11.Context

	// Current state (protected by mutex)
	keys           map[string]cipher.AEAD  // label -> GCM cipher
	contextToLabel map[string]string       // context -> current label
	metadata       map[string]*KeyMetadata // label -> metadata
	mu             sync.RWMutex

	// Metadata file tracking
	metadataFile string
	lastModTime  time.Time
	hsmConfig    *config.HSMConfig

	// Auto-reload control
	stopReload chan struct{}
	reloadWg   sync.WaitGroup
	stopOnce   sync.Once
}

// NewKeyManager creates a new KeyManager with initial state
func NewKeyManager(ctx *crypto11.Context, cfg *config.HSMConfig, metadata *config.Metadata) (*KeyManager, error) {
	km := &KeyManager{
		ctx:          ctx,
		metadataFile: cfg.MetadataFile,
		hsmConfig:    cfg,
		stopReload:   make(chan struct{}),
	}

	// Load initial state
	if err := km.loadKeys(metadata); err != nil {
		return nil, fmt.Errorf("failed to load initial keys: %w", err)
	}

	// Set initial modTime
	if info, err := os.Stat(km.metadataFile); err == nil {
		km.lastModTime = info.ModTime()
	}

	return km, nil
}

// loadKeys loads keys from metadata into internal cache
func (km *KeyManager) loadKeys(metadata *config.Metadata) error {
	newKeys := make(map[string]cipher.AEAD)
	newContextToLabel := make(map[string]string)
	newMetadata := make(map[string]*KeyMetadata)

	for context, keyConfig := range km.hsmConfig.Keys {
		if keyConfig.Type != "aes" {
			continue // Skip non-AES keys
		}

		// Get metadata for this context
		meta, ok := metadata.Rotation[context]
		if !ok {
			return fmt.Errorf("metadata not found for context: %s", context)
		}

		// Save context -> current label mapping
		newContextToLabel[context] = meta.Current

		// Load all versions of the key
		for _, version := range meta.Versions {
			// Find key by label
			secretKey, err := km.ctx.FindKey(nil, []byte(version.Label))
			if err != nil {
				slog.Warn("KEK not found in HSM",
					"label", version.Label,
					"error", err)
				continue
			}

			if secretKey == nil {
				slog.Warn("key not found in token", "label", version.Label)
				continue
			}

			// Verify checksum if available
			if version.Checksum != "" {
				computedChecksum := computeKeyChecksum(version.Label, secretKey)
				if computedChecksum != version.Checksum {
					return fmt.Errorf("KEK integrity verification failed for %s: checksum mismatch", version.Label)
				}
				slog.Info("KEK integrity verified",
					"label", version.Label,
					"checksum", computedChecksum[:8])
			}

			// Create GCM cipher
			gcm, err := secretKey.NewGCM()
			if err != nil {
				slog.Warn("failed to create GCM for key",
					"label", version.Label,
					"error", err)
				continue
			}

			// Cache the GCM cipher
			newKeys[version.Label] = gcm

			// Store metadata
			createdAt := time.Now()
			if version.CreatedAt != nil {
				createdAt = *version.CreatedAt
			}

			newMetadata[version.Label] = &KeyMetadata{
				Label:            version.Label,
				Version:          version.Version,
				CreatedAt:        createdAt,
				RotationInterval: keyConfig.RotationInterval,
			}

			slog.Info("Loaded KEK",
				"label", version.Label,
				"version", version.Version)
		}

		// Ensure current version was loaded
		if newKeys[meta.Current] == nil {
			return fmt.Errorf("current KEK not loaded: %s", meta.Current)
		}
	}

	if len(newKeys) == 0 {
		return fmt.Errorf("no AES keys found in configuration")
	}

	// Atomic update
	km.mu.Lock()
	km.keys = newKeys
	km.contextToLabel = newContextToLabel
	km.metadata = newMetadata
	km.mu.Unlock()

	return nil
}

// StartAutoReload starts automatic metadata reload
func (km *KeyManager) StartAutoReload(interval time.Duration) {
	km.reloadWg.Add(1)
	go func() {
		defer km.reloadWg.Done()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				if km.metadataChanged() {
					if err := km.ReloadMetadata(); err != nil {
						slog.Error("metadata reload failed", "error", err)
					}
				}
			case <-km.stopReload:
				slog.Info("Stopping metadata auto-reload")
				return
			}
		}
	}()

	slog.Info("Started metadata auto-reload", "interval", interval)
}

// metadataChanged checks if metadata file has been modified
func (km *KeyManager) metadataChanged() bool {
	info, err := os.Stat(km.metadataFile)
	if err != nil {
		if os.IsNotExist(err) {
			slog.Warn("metadata file not found", "path", km.metadataFile)
		}
		return false
	}

	if info.ModTime().After(km.lastModTime) {
		km.lastModTime = info.ModTime()
		slog.Info("metadata file changed", "path", km.metadataFile)
		return true
	}

	return false
}

// ReloadMetadata reloads metadata and keys from file
func (km *KeyManager) ReloadMetadata() error {
	// 1. Load new metadata from file
	newMetadata, err := config.LoadMetadata(km.metadataFile)
	if err != nil {
		slog.Warn("metadata reload skipped due to load error", "error", err)
		return err
	}

	// 2. Load keys with new metadata (validates before applying)
	if err := km.loadKeys(newMetadata); err != nil {
		slog.Error("failed to load keys from new metadata", "error", err)
		return err
	}

	slog.Info("KEK hot reload successful",
		"contexts", len(km.contextToLabel),
		"total_keys", len(km.keys))

	return nil
}

// StopAutoReload stops the auto-reload goroutine gracefully
func (km *KeyManager) StopAutoReload(ctx context.Context) error {
	km.stopOnce.Do(func() {
		close(km.stopReload)
	})

	// Wait for goroutine to finish with timeout
	done := make(chan struct{})
	go func() {
		km.reloadWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		slog.Info("Metadata auto-reload stopped successfully")
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Encrypt encrypts plaintext using the current active key for the context
func (km *KeyManager) Encrypt(plaintext []byte, context, clientCN string) (ciphertext []byte, keyLabel string, err error) {
	// Get current key label for context
	km.mu.RLock()
	label, exists := km.contextToLabel[context]
	km.mu.RUnlock()

	if !exists {
		return nil, "", fmt.Errorf("no key configured for context: %s", context)
	}

	// Get GCM cipher
	km.mu.RLock()
	gcm, exists := km.keys[label]
	km.mu.RUnlock()

	if !exists {
		return nil, "", fmt.Errorf("%w: %s", ErrKeyNotFound, label)
	}

	// Build AAD
	aad := BuildAAD(context, clientCN)

	// Generate nonce
	nonce := make([]byte, gcm.NonceSize())
	if _, err := ReadRandom(nonce); err != nil {
		return nil, "", fmt.Errorf("failed to generate nonce: %w", err)
	}

	// Encrypt
	ciphertext = gcm.Seal(nonce, nonce, plaintext, aad)

	return ciphertext, label, nil
}

// Decrypt decrypts ciphertext using the specified key label
func (km *KeyManager) Decrypt(ciphertext []byte, context, clientCN, keyLabel string) ([]byte, error) {
	// Get GCM cipher
	km.mu.RLock()
	gcm, exists := km.keys[keyLabel]
	km.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("%w: %s", ErrKeyNotFound, keyLabel)
	}

	// Validate ciphertext length
	nonceSize := gcm.NonceSize()
	if len(ciphertext) < nonceSize {
		return nil, ErrInvalidCiphertext
	}

	// Extract nonce and encrypted data
	nonce := ciphertext[:nonceSize]
	encrypted := ciphertext[nonceSize:]

	// Build AAD
	aad := BuildAAD(context, clientCN)

	// Decrypt
	plaintext, err := gcm.Open(nil, nonce, encrypted, aad)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrDecryptionFailed, err)
	}

	return plaintext, nil
}

// GetKeyLabels returns all available key labels
func (km *KeyManager) GetKeyLabels() []string {
	km.mu.RLock()
	defer km.mu.RUnlock()

	labels := make([]string, 0, len(km.keys))
	for label := range km.keys {
		labels = append(labels, label)
	}
	return labels
}

// HasKey checks if a key exists
func (km *KeyManager) HasKey(label string) bool {
	km.mu.RLock()
	defer km.mu.RUnlock()

	_, exists := km.keys[label]
	return exists
}

// GetKeyLabelByContext returns the current key label for a context
func (km *KeyManager) GetKeyLabelByContext(context string) (string, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	label, exists := km.contextToLabel[context]
	if !exists {
		return "", fmt.Errorf("no key configured for context: %s", context)
	}
	return label, nil
}

// GetKeyMetadata returns metadata for a key
func (km *KeyManager) GetKeyMetadata(label string) (*KeyMetadata, error) {
	km.mu.RLock()
	defer km.mu.RUnlock()

	meta, exists := km.metadata[label]
	if !exists {
		return nil, fmt.Errorf("no metadata for key: %s", label)
	}
	return meta, nil
}

// GetKeysNeedingRotation returns keys that need rotation
func (km *KeyManager) GetKeysNeedingRotation() []string {
	km.mu.RLock()
	defer km.mu.RUnlock()

	var needsRotation []string
	for label, meta := range km.metadata {
		if meta.NeedsRotation() {
			needsRotation = append(needsRotation, label)
		}
	}
	return needsRotation
}

// Close closes the underlying PKCS#11 context
func (km *KeyManager) Close() error {
	if km.ctx != nil {
		return km.ctx.Close()
	}
	return nil
}
