package hsm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func TestKeyManagerReloadMetadata_MissingFileReturnsError(t *testing.T) {
	km := &KeyManager{
		metadataFile: filepath.Join(t.TempDir(), "missing-metadata.yaml"),
	}

	if err := km.ReloadMetadata(); err == nil {
		t.Fatal("ReloadMetadata() expected error for missing metadata file")
	}
}

func TestKeyManagerStartAutoReload_StopsGracefully(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	km := &KeyManager{
		metadataFile: path,
		stopReload:   make(chan struct{}),
	}

	km.StartAutoReload(5 * time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := km.StopAutoReload(ctx); err != nil {
		t.Fatalf("StopAutoReload() after StartAutoReload error: %v", err)
	}
}

func TestKeyManagerStartAutoReload_ReloadPathTriggered(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	// Invalid YAML is enough here: we only need to exercise the reload attempt path.
	if err := os.WriteFile(path, []byte("rotation: ["), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	km := &KeyManager{
		metadataFile: path,
		hsmConfig:    &config.HSMConfig{Keys: map[string]config.KeyConfig{}},
		stopReload:   make(chan struct{}),
	}

	km.StartAutoReload(5 * time.Millisecond)
	time.Sleep(20 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := km.StopAutoReload(ctx); err != nil {
		t.Fatalf("StopAutoReload() after reload-path test error: %v", err)
	}
}

func TestKeyManagerCheckVersionLimits_NoPanicOnExcessVersionsAndAge(t *testing.T) {
	now := time.Now()
	old := config.RFC3339Micro(now.AddDate(0, 0, -120))
	recent := config.RFC3339Micro(now.AddDate(0, 0, -10))

	km := &KeyManager{
		maxVersions:      2,
		cleanupAfterDays: 30,
	}

	metadata := &config.Metadata{
		Rotation: map[string]config.KeyMetadata{
			"exchange-key": {
				Current: "kek-exchange-v3",
				Versions: []config.KeyVersion{
					{Label: "kek-exchange-v1", Version: 1, CreatedAt: &old},
					{Label: "kek-exchange-v2", Version: 2, CreatedAt: &old},
					{Label: "kek-exchange-v3", Version: 3, CreatedAt: &recent},
				},
			},
		},
	}

	km.checkVersionLimits(metadata)
}
