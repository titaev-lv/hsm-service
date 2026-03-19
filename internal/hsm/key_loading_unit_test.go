package hsm

import (
	"crypto/cipher"
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ThalesGroup/crypto11"
	"github.com/titaev-lv/hsm-service/internal/config"
)

type mockSecretKey struct {
	gcm    cipher.AEAD
	gcmErr error
}

func (m *mockSecretKey) NewGCM() (cipher.AEAD, error) {
	if m.gcmErr != nil {
		return nil, m.gcmErr
	}
	return m.gcm, nil
}

type mockFinder struct {
	keys map[string]gcmFactory
	errs map[string]error
}

func (m *mockFinder) FindKey(_id, label []byte) (gcmFactory, error) {
	l := string(label)
	if err, ok := m.errs[l]; ok {
		return nil, err
	}
	if key, ok := m.keys[l]; ok {
		return key, nil
	}
	return nil, nil
}

type mockHandle struct {
	finder    *mockFinder
	closed    bool
	closeErr  error
	rawCtxPtr *crypto11.Context
}

func (m *mockHandle) FindKey(id, label []byte) (gcmFactory, error) {
	return m.finder.FindKey(id, label)
}

func (m *mockHandle) Close() error {
	m.closed = true
	return m.closeErr
}

func (m *mockHandle) Raw() *crypto11.Context {
	return m.rawCtxPtr
}

func testConfigForLoad(metadataPath string) *config.Config {
	return &config.Config{
		HSM: config.HSMConfig{
			MetadataFile: metadataPath,
			Keys: map[string]config.KeyConfig{
				"exchange-key": {Type: "aes", Mode: "shared"},
			},
		},
	}
}

func testMetadataOneVersion(label string) *config.Metadata {
	now := config.RFC3339Micro(time.Now().UTC())
	return &config.Metadata{
		Rotation: map[string]config.KeyMetadata{
			"exchange-key": {
				Current: label,
				Versions: []config.KeyVersion{
					{Label: label, Version: 1, CreatedAt: &now},
				},
			},
		},
	}
}

func TestNewKeyManagerWithFinder_SuccessAndDefaults(t *testing.T) {
	metaPath := filepath.Join(t.TempDir(), "metadata.yaml")
	cfg := testConfigForLoad(metaPath)
	metadata := testMetadataOneVersion("kek-exchange-key-v1")

	finder := &mockFinder{
		keys: map[string]gcmFactory{
			"kek-exchange-key-v1": &mockSecretKey{gcm: mustNewGCM(t)},
		},
	}

	km, err := newKeyManagerWithFinder(nil, finder, cfg, metadata)
	if err != nil {
		t.Fatalf("newKeyManagerWithFinder() error: %v", err)
	}

	if km.maxVersions != 3 {
		t.Fatalf("maxVersions = %d, want 3", km.maxVersions)
	}
	if km.cleanupAfterDays != 30 {
		t.Fatalf("cleanupAfterDays = %d, want 30", km.cleanupAfterDays)
	}

	if !km.HasKey("kek-exchange-key-v1") {
		t.Fatal("expected loaded key kek-exchange-key-v1")
	}

	label, err := km.GetKeyLabelByContext("exchange-key")
	if err != nil {
		t.Fatalf("GetKeyLabelByContext() error: %v", err)
	}
	if label != "kek-exchange-key-v1" {
		t.Fatalf("GetKeyLabelByContext() = %q, want kek-exchange-key-v1", label)
	}

	meta, err := km.GetKeyMetadata("kek-exchange-key-v1")
	if err != nil {
		t.Fatalf("GetKeyMetadata() error: %v", err)
	}
	if meta.RotationInterval != 90*24*time.Hour {
		t.Fatalf("RotationInterval = %v, want %v", meta.RotationInterval, 90*24*time.Hour)
	}
}

func TestNewKeyManager_NilContextError(t *testing.T) {
	cfg := testConfigForLoad(filepath.Join(t.TempDir(), "metadata.yaml"))
	_, err := NewKeyManager(nil, cfg, testMetadataOneVersion("kek-exchange-key-v1"))
	if err == nil || !strings.Contains(err.Error(), "context is nil") {
		t.Fatalf("expected nil context error, got: %v", err)
	}
}

func TestKeyManagerLoadKeys_ErrorBranches(t *testing.T) {
	cfg := testConfigForLoad(filepath.Join(t.TempDir(), "metadata.yaml"))

	t.Run("finder missing", func(t *testing.T) {
		km := &KeyManager{hsmConfig: &cfg.HSM}
		err := km.loadKeys(testMetadataOneVersion("kek-exchange-key-v1"))
		if err == nil || !strings.Contains(err.Error(), "pkcs11 key finder") {
			t.Fatalf("expected finder error, got: %v", err)
		}
	})

	t.Run("metadata missing for context", func(t *testing.T) {
		km := &KeyManager{
			hsmConfig: &cfg.HSM,
			finder:    &mockFinder{},
		}
		err := km.loadKeys(&config.Metadata{Rotation: map[string]config.KeyMetadata{}})
		if err == nil || !strings.Contains(err.Error(), "metadata not found for context") {
			t.Fatalf("expected metadata missing error, got: %v", err)
		}
	})

	t.Run("current key not loaded", func(t *testing.T) {
		km := &KeyManager{
			hsmConfig: &cfg.HSM,
			finder:    &mockFinder{},
		}
		err := km.loadKeys(testMetadataOneVersion("kek-exchange-key-v1"))
		if err == nil || !strings.Contains(err.Error(), "current KEK not loaded") {
			t.Fatalf("expected current not loaded error, got: %v", err)
		}
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		md := testMetadataOneVersion("kek-exchange-key-v1")
		md.Rotation["exchange-key"] = config.KeyMetadata{
			Current: "kek-exchange-key-v1",
			Versions: []config.KeyVersion{
				{Label: "kek-exchange-key-v1", Version: 1, Checksum: "bad-checksum"},
			},
		}

		km := &KeyManager{
			hsmConfig: &cfg.HSM,
			finder: &mockFinder{keys: map[string]gcmFactory{
				"kek-exchange-key-v1": &mockSecretKey{gcm: mustNewGCM(t)},
			}},
		}
		err := km.loadKeys(md)
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("expected checksum mismatch error, got: %v", err)
		}
	})

	t.Run("no aes keys configured", func(t *testing.T) {
		nonAES := &config.HSMConfig{Keys: map[string]config.KeyConfig{"rsa-key": {Type: "rsa"}}}
		km := &KeyManager{hsmConfig: nonAES, finder: &mockFinder{}}
		err := km.loadKeys(&config.Metadata{Rotation: map[string]config.KeyMetadata{}})
		if err == nil || !strings.Contains(err.Error(), "no AES keys") {
			t.Fatalf("expected no AES keys error, got: %v", err)
		}
	})
}

func TestInitHSM_ConfigureError(t *testing.T) {
	cfg := &config.HSMConfig{
		PKCS11Lib: "this-path-does-not-exist",
		SlotID:    "test-slot",
		Keys: map[string]config.KeyConfig{
			"exchange-key": {Type: "aes"},
		},
	}
	_, err := InitHSM(cfg, testMetadataOneVersion("kek-exchange-key-v1"), "1234")
	if err == nil || !strings.Contains(err.Error(), "failed to configure crypto11") {
		t.Fatalf("expected configure error, got: %v", err)
	}
}

func TestInitHSMWithHandle_Success(t *testing.T) {
	now := config.RFC3339Micro(time.Now().UTC())
	cfg := &config.HSMConfig{
		Keys: map[string]config.KeyConfig{
			"exchange-key": {Type: "aes"},
			"ignored-rsa":  {Type: "rsa"},
		},
	}
	metadata := &config.Metadata{
		Rotation: map[string]config.KeyMetadata{
			"exchange-key": {
				Current: "kek-exchange-key-v2",
				Versions: []config.KeyVersion{
					{Label: "kek-exchange-key-v1", Version: 1, CreatedAt: &now},
					{Label: "kek-exchange-key-v2", Version: 2, CreatedAt: &now},
				},
			},
		},
	}

	h := &mockHandle{finder: &mockFinder{keys: map[string]gcmFactory{
		"kek-exchange-key-v1": &mockSecretKey{gcm: mustNewGCM(t)},
		"kek-exchange-key-v2": &mockSecretKey{gcm: mustNewGCM(t)},
	}}}

	ctx, err := initHSMWithHandle(h, cfg, metadata)
	if err != nil {
		t.Fatalf("initHSMWithHandle() error: %v", err)
	}
	if h.closed {
		t.Fatal("context should stay open on success")
	}
	if !ctx.HasKey("kek-exchange-key-v1") || !ctx.HasKey("kek-exchange-key-v2") {
		t.Fatal("expected both versions to be loaded")
	}

	label, err := ctx.GetKeyLabelByContext("exchange-key")
	if err != nil {
		t.Fatalf("GetKeyLabelByContext() error: %v", err)
	}
	if label != "kek-exchange-key-v2" {
		t.Fatalf("current label = %q, want kek-exchange-key-v2", label)
	}
}

func TestInitHSMWithHandle_ErrorBranchesCloseContext(t *testing.T) {
	t.Run("metadata missing", func(t *testing.T) {
		h := &mockHandle{finder: &mockFinder{}}
		cfg := &config.HSMConfig{Keys: map[string]config.KeyConfig{"exchange-key": {Type: "aes"}}}
		_, err := initHSMWithHandle(h, cfg, &config.Metadata{Rotation: map[string]config.KeyMetadata{}})
		if err == nil || !strings.Contains(err.Error(), "metadata not found for context") {
			t.Fatalf("expected metadata missing error, got: %v", err)
		}
		if !h.closed {
			t.Fatal("expected context Close() on error")
		}
	})

	t.Run("checksum mismatch", func(t *testing.T) {
		h := &mockHandle{finder: &mockFinder{keys: map[string]gcmFactory{
			"kek-exchange-key-v1": &mockSecretKey{gcm: mustNewGCM(t)},
		}}}
		cfg := &config.HSMConfig{Keys: map[string]config.KeyConfig{"exchange-key": {Type: "aes"}}}
		md := &config.Metadata{Rotation: map[string]config.KeyMetadata{
			"exchange-key": {
				Current:  "kek-exchange-key-v1",
				Versions: []config.KeyVersion{{Label: "kek-exchange-key-v1", Version: 1, Checksum: "invalid"}},
			},
		}}
		_, err := initHSMWithHandle(h, cfg, md)
		if err == nil || !strings.Contains(err.Error(), "checksum mismatch") {
			t.Fatalf("expected checksum mismatch error, got: %v", err)
		}
		if !h.closed {
			t.Fatal("expected context Close() on checksum error")
		}
	})

	t.Run("gcm creation failure leads to current not loaded", func(t *testing.T) {
		h := &mockHandle{finder: &mockFinder{keys: map[string]gcmFactory{
			"kek-exchange-key-v1": &mockSecretKey{gcmErr: errors.New("gcm failed")},
		}}}
		cfg := &config.HSMConfig{Keys: map[string]config.KeyConfig{"exchange-key": {Type: "aes"}}}
		_, err := initHSMWithHandle(h, cfg, testMetadataOneVersion("kek-exchange-key-v1"))
		if err == nil || !strings.Contains(err.Error(), "current KEK not loaded") {
			t.Fatalf("expected current not loaded error, got: %v", err)
		}
		if !h.closed {
			t.Fatal("expected context Close() when current key is unavailable")
		}
	})

	t.Run("no aes keys", func(t *testing.T) {
		h := &mockHandle{finder: &mockFinder{}}
		cfg := &config.HSMConfig{Keys: map[string]config.KeyConfig{"rsa-key": {Type: "rsa"}}}
		_, err := initHSMWithHandle(h, cfg, &config.Metadata{Rotation: map[string]config.KeyMetadata{}})
		if err == nil || !strings.Contains(err.Error(), "no AES keys") {
			t.Fatalf("expected no AES keys error, got: %v", err)
		}
		if !h.closed {
			t.Fatal("expected context Close() on no-AES error")
		}
	})
}

func TestInitHSMWithHandle_MixedVersionBranches_Success(t *testing.T) {
	cfg := &config.HSMConfig{
		Keys: map[string]config.KeyConfig{
			"exchange-key": {Type: "aes"},
		},
	}

	currentLabel := "kek-exchange-key-v3"
	metadata := &config.Metadata{
		Rotation: map[string]config.KeyMetadata{
			"exchange-key": {
				Current: currentLabel,
				Versions: []config.KeyVersion{
					{Label: "kek-exchange-key-v1", Version: 1}, // FindKey error branch
					{Label: "kek-exchange-key-v2", Version: 2}, // nil key branch
					{
						Label:    currentLabel,
						Version:  3,
						Checksum: computeKeyChecksum(currentLabel, nil), // checksum success branch
						// CreatedAt intentionally nil -> fallback to time.Now()
					},
				},
			},
		},
	}

	h := &mockHandle{finder: &mockFinder{
		errs: map[string]error{
			"kek-exchange-key-v1": errors.New("not found"),
		},
		keys: map[string]gcmFactory{
			currentLabel: &mockSecretKey{gcm: mustNewGCM(t)},
		},
	}}

	ctx, err := initHSMWithHandle(h, cfg, metadata)
	if err != nil {
		t.Fatalf("initHSMWithHandle() error: %v", err)
	}
	if h.closed {
		t.Fatal("context should remain open on success")
	}
	if !ctx.HasKey(currentLabel) {
		t.Fatalf("expected current key %q to be loaded", currentLabel)
	}
	if ctx.HasKey("kek-exchange-key-v1") || ctx.HasKey("kek-exchange-key-v2") {
		t.Fatal("unexpected old versions loaded from error/nil branches")
	}

	meta, err := ctx.GetKeyMetadata(currentLabel)
	if err != nil {
		t.Fatalf("GetKeyMetadata() error: %v", err)
	}
	if meta.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt fallback to be initialized")
	}
}
