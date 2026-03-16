package hsm

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
	"sort"
	"testing"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func mustNewGCM(t *testing.T) cipher.AEAD {
	t.Helper()
	key := []byte("0123456789abcdef0123456789abcdef") // 32 bytes
	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("aes.NewCipher: %v", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("cipher.NewGCM: %v", err)
	}
	return gcm
}

func newUnitKeyManager(t *testing.T, mode string) *KeyManager {
	t.Helper()
	label := "kek-exchange-v1"
	return &KeyManager{
		keys: map[string]cipher.AEAD{
			label: mustNewGCM(t),
		},
		contextToLabel: map[string]string{
			"exchange-key": label,
		},
		metadata: map[string]*KeyMetadata{
			label: {
				Label:            label,
				Version:          1,
				CreatedAt:        time.Now().Add(-120 * 24 * time.Hour),
				RotationInterval: 90 * 24 * time.Hour,
			},
		},
		config: &config.Config{
			HSM: config.HSMConfig{
				Keys: map[string]config.KeyConfig{
					"exchange-key": {Type: "aes", Mode: mode},
				},
			},
		},
	}
}

func TestKeyManagerEncryptDecrypt_PrivateModeRoundTrip(t *testing.T) {
	km := newUnitKeyManager(t, "private")
	plaintext := []byte("secret payload")

	ciphertext, label, err := km.Encrypt(plaintext, "exchange-key", "Trading", "trader-1")
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}
	if label != "kek-exchange-v1" {
		t.Fatalf("Encrypt() label = %q, want %q", label, "kek-exchange-v1")
	}

	decrypted, err := km.Decrypt(ciphertext, "exchange-key", "Trading", "trader-1", label)
	if err != nil {
		t.Fatalf("Decrypt() error: %v", err)
	}
	if string(decrypted) != string(plaintext) {
		t.Fatalf("Decrypt() = %q, want %q", decrypted, plaintext)
	}
}

func TestKeyManagerDecrypt_PrivateModeAADMismatch(t *testing.T) {
	km := newUnitKeyManager(t, "private")
	ciphertext, label, err := km.Encrypt([]byte("secret"), "exchange-key", "Trading", "trader-1")
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	_, err = km.Decrypt(ciphertext, "exchange-key", "Trading", "trader-2", label)
	if err == nil {
		t.Fatal("Decrypt() expected error for AAD mismatch")
	}
	if !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("Decrypt() error = %v, want ErrDecryptionFailed", err)
	}
}

func TestKeyManagerDecrypt_SharedModeSameOUAllowed(t *testing.T) {
	km := newUnitKeyManager(t, "shared")
	ciphertext, label, err := km.Encrypt([]byte("secret"), "exchange-key", "Trading", "trader-1")
	if err != nil {
		t.Fatalf("Encrypt() error: %v", err)
	}

	_, err = km.Decrypt(ciphertext, "exchange-key", "Trading", "trader-2", label)
	if err != nil {
		t.Fatalf("Decrypt() in shared mode should succeed for same OU: %v", err)
	}
}

func TestKeyManagerEncrypt_UnknownContext(t *testing.T) {
	km := newUnitKeyManager(t, "private")
	_, _, err := km.Encrypt([]byte("x"), "unknown", "Trading", "trader-1")
	if err == nil {
		t.Fatal("Encrypt() expected error for unknown context")
	}
}

func TestKeyManagerEncrypt_KeyLabelConfiguredButCipherMissing(t *testing.T) {
	km := newUnitKeyManager(t, "private")
	km.contextToLabel["exchange-key"] = "kek-missing-v1"

	_, _, err := km.Encrypt([]byte("x"), "exchange-key", "Trading", "trader-1")
	if err == nil {
		t.Fatal("Encrypt() expected key not found error")
	}
	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Encrypt() error = %v, want ErrKeyNotFound", err)
	}
}

func TestKeyManagerDecrypt_UnknownContext(t *testing.T) {
	km := newUnitKeyManager(t, "private")
	_, err := km.Decrypt([]byte("abc"), "unknown", "Trading", "trader-1", "kek-exchange-v1")
	if err == nil {
		t.Fatal("Decrypt() expected error for unknown context")
	}
}

func TestKeyManagerDecrypt_KeyNotFound(t *testing.T) {
	km := newUnitKeyManager(t, "private")
	_, err := km.Decrypt([]byte("abc"), "exchange-key", "Trading", "trader-1", "missing")
	if err == nil {
		t.Fatal("Decrypt() expected key not found error")
	}
	if !errors.Is(err, ErrKeyNotFound) {
		t.Fatalf("Decrypt() error = %v, want ErrKeyNotFound", err)
	}
}

func TestKeyManagerDecrypt_InvalidCiphertextTooShort(t *testing.T) {
	km := newUnitKeyManager(t, "private")
	_, err := km.Decrypt([]byte("short"), "exchange-key", "Trading", "trader-1", "kek-exchange-v1")
	if err == nil {
		t.Fatal("Decrypt() expected invalid ciphertext error")
	}
	if !errors.Is(err, ErrInvalidCiphertext) {
		t.Fatalf("Decrypt() error = %v, want ErrInvalidCiphertext", err)
	}
}

func TestKeyManagerGettersAndRotationList(t *testing.T) {
	km := newUnitKeyManager(t, "private")
	label := "kek-exchange-v1"

	labels := km.GetKeyLabels()
	sort.Strings(labels)
	if len(labels) != 1 || labels[0] != label {
		t.Fatalf("GetKeyLabels() = %v, want [%s]", labels, label)
	}
	if !km.HasKey(label) {
		t.Fatalf("HasKey(%q) = false, want true", label)
	}

	ctxLabel, err := km.GetKeyLabelByContext("exchange-key")
	if err != nil {
		t.Fatalf("GetKeyLabelByContext() error: %v", err)
	}
	if ctxLabel != label {
		t.Fatalf("GetKeyLabelByContext() = %q, want %q", ctxLabel, label)
	}

	meta, err := km.GetKeyMetadata(label)
	if err != nil {
		t.Fatalf("GetKeyMetadata() error: %v", err)
	}
	if meta.Version != 1 {
		t.Fatalf("GetKeyMetadata().Version = %d, want 1", meta.Version)
	}

	needs := km.GetKeysNeedingRotation()
	if len(needs) != 1 || needs[0] != label {
		t.Fatalf("GetKeysNeedingRotation() = %v, want [%s]", needs, label)
	}
}

func TestKeyManagerGetters_UnknownValues(t *testing.T) {
	km := newUnitKeyManager(t, "private")
	if _, err := km.GetKeyLabelByContext("missing"); err == nil {
		t.Fatal("GetKeyLabelByContext() expected error for unknown context")
	}
	if _, err := km.GetKeyMetadata("missing"); err == nil {
		t.Fatal("GetKeyMetadata() expected error for unknown label")
	}
}
