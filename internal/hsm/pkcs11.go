package hsm

import (
	"crypto/cipher"
	"fmt"

	"github.com/ThalesGroup/crypto11"
	"github.com/titaev-lv/hsm-service/internal/config"
)

// HSMContext represents the PKCS#11 session and cached key handles
type HSMContext struct {
	ctx  *crypto11.Context
	keys map[string]cipher.AEAD // label -> GCM cipher
}

// InitHSM initializes the PKCS#11 context and loads all configured keys
func InitHSM(cfg *config.HSMConfig, pin string) (*HSMContext, error) {
	// 1. Configure crypto11
	c11Config := &crypto11.Config{
		Path:       cfg.PKCS11Lib,
		TokenLabel: cfg.SlotID,
		Pin:        pin,
	}

	// 2. Initialize context
	ctx, err := crypto11.Configure(c11Config)
	if err != nil {
		return nil, fmt.Errorf("failed to configure crypto11: %w", err)
	}

	// 3. Find and cache all configured KEKs
	keys := make(map[string]cipher.AEAD)
	for label, keyConfig := range cfg.Keys {
		if keyConfig.Type != "aes" {
			continue // Skip non-AES keys for now
		}

		// Find key by label
		secretKey, err := ctx.FindKey(nil, []byte(keyConfig.Label))
		if err != nil {
			ctx.Close()
			return nil, fmt.Errorf("KEK not found: %s: %w", keyConfig.Label, err)
		}

		if secretKey == nil {
			ctx.Close()
			return nil, fmt.Errorf("key %s not found in token", keyConfig.Label)
		}

		// Create GCM cipher
		gcm, err := secretKey.NewGCM()
		if err != nil {
			ctx.Close()
			return nil, fmt.Errorf("failed to create GCM for key %s: %w", keyConfig.Label, err)
		}

		// Cache the GCM cipher
		keys[label] = gcm
	}

	if len(keys) == 0 {
		ctx.Close()
		return nil, fmt.Errorf("no AES keys found in configuration")
	}

	return &HSMContext{
		ctx:  ctx,
		keys: keys,
	}, nil
}

// Close closes the PKCS#11 session
func (h *HSMContext) Close() error {
	if h.ctx != nil {
		return h.ctx.Close()
	}
	return nil
}

// GetKeyLabels returns all available key labels
func (h *HSMContext) GetKeyLabels() []string {
	labels := make([]string, 0, len(h.keys))
	for label := range h.keys {
		labels = append(labels, label)
	}
	return labels
}

// HasKey checks if a key with given label exists
func (h *HSMContext) HasKey(label string) bool {
	_, exists := h.keys[label]
	return exists
}
