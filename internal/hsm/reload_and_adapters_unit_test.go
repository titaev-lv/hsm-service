package hsm

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func writeMetadataYAML(t *testing.T, path, content string) {
	t.Helper()
	if err := config.SaveMetadata(path, mustLoadMetadataYAML(t, content)); err != nil {
		t.Fatalf("SaveMetadata() error: %v", err)
	}
}

func mustLoadMetadataYAML(t *testing.T, content string) *config.Metadata {
	t.Helper()
	tmp := filepath.Join(t.TempDir(), "metadata.yaml")
	if err := osWriteFile0600(tmp, []byte(content)); err != nil {
		t.Fatalf("osWriteFile0600() error: %v", err)
	}
	meta, err := config.LoadMetadata(tmp)
	if err != nil {
		t.Fatalf("LoadMetadata() error: %v", err)
	}
	return meta
}

func osWriteFile0600(path string, data []byte) error {
	return os.WriteFile(path, data, 0o600)
}

func TestKeyManagerReloadMetadata_LoadKeysValidationError(t *testing.T) {
	metadataPath := filepath.Join(t.TempDir(), "metadata.yaml")
	writeMetadataYAML(t, metadataPath, `rotation:
  another-key:
    current: "kek-another-v1"
    versions:
      - label: "kek-another-v1"
        version: 1
`)

	km := &KeyManager{
		metadataFile: metadataPath,
		hsmConfig: &config.HSMConfig{Keys: map[string]config.KeyConfig{
			"exchange-key": {Type: "aes"},
		}},
		finder: &mockFinder{},
	}

	err := km.ReloadMetadata()
	if err == nil || !strings.Contains(err.Error(), "metadata not found for context") {
		t.Fatalf("ReloadMetadata() error = %v, want metadata mapping error", err)
	}
}

func TestKeyManagerReloadMetadata_Success(t *testing.T) {
	metadataPath := filepath.Join(t.TempDir(), "metadata.yaml")
	writeMetadataYAML(t, metadataPath, `rotation:
  exchange-key:
    current: "kek-exchange-v1"
    rotation_interval_days: 45
    versions:
      - label: "kek-exchange-v1"
        version: 1
`)

	km := &KeyManager{
		metadataFile: metadataPath,
		hsmConfig: &config.HSMConfig{Keys: map[string]config.KeyConfig{
			"exchange-key": {Type: "aes"},
		}},
		finder: &mockFinder{keys: map[string]gcmFactory{
			"kek-exchange-v1": &mockSecretKey{gcm: mustNewGCM(t)},
		}},
		maxVersions:      3,
		cleanupAfterDays: 30,
	}

	if err := km.ReloadMetadata(); err != nil {
		t.Fatalf("ReloadMetadata() error: %v", err)
	}

	if !km.HasKey("kek-exchange-v1") {
		t.Fatal("expected key kek-exchange-v1 to be loaded")
	}

	label, err := km.GetKeyLabelByContext("exchange-key")
	if err != nil {
		t.Fatalf("GetKeyLabelByContext() error: %v", err)
	}
	if label != "kek-exchange-v1" {
		t.Fatalf("GetKeyLabelByContext() = %q, want kek-exchange-v1", label)
	}

	meta, err := km.GetKeyMetadata("kek-exchange-v1")
	if err != nil {
		t.Fatalf("GetKeyMetadata() error: %v", err)
	}
	if meta.RotationInterval != 45*24*time.Hour {
		t.Fatalf("RotationInterval = %v, want %v", meta.RotationInterval, 45*24*time.Hour)
	}
}

func TestCrypto11Finder_NilContextReturnsError(t *testing.T) {
	f := &crypto11Finder{}
	_, err := f.FindKey(nil, []byte("kek-exchange-v1"))
	if err == nil || !strings.Contains(err.Error(), "context is nil") {
		t.Fatalf("FindKey() error = %v, want nil-context error", err)
	}
}

func TestCrypto11Handle_NilContextSafety(t *testing.T) {
	h := &crypto11Handle{}

	_, err := h.FindKey(nil, []byte("kek-exchange-v1"))
	if err == nil || !strings.Contains(err.Error(), "context is nil") {
		t.Fatalf("FindKey() error = %v, want nil-context error", err)
	}

	if err := h.Close(); err != nil {
		t.Fatalf("Close() with nil context error: %v", err)
	}

	if got := h.Raw(); got != nil {
		t.Fatalf("Raw() = %v, want nil", got)
	}
}
