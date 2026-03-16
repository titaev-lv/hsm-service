package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadConfig_PathTraversalRejected(t *testing.T) {
	_, err := LoadConfig("../config.yaml")
	if err == nil {
		t.Fatal("LoadConfig() expected path traversal error")
	}
	if !strings.Contains(err.Error(), "invalid config path") {
		t.Fatalf("LoadConfig() error = %v, want invalid config path", err)
	}
}

func TestLoadMetadata_PathTraversalRejected(t *testing.T) {
	_, err := LoadMetadata("../metadata.yaml")
	if err == nil {
		t.Fatal("LoadMetadata() expected path traversal error")
	}
	if !strings.Contains(err.Error(), "invalid metadata path") {
		t.Fatalf("LoadMetadata() error = %v, want invalid metadata path", err)
	}
}

func TestSaveMetadata_PathTraversalRejected(t *testing.T) {
	err := SaveMetadata("../metadata.yaml", &Metadata{})
	if err == nil {
		t.Fatal("SaveMetadata() expected path traversal error")
	}
	if !strings.Contains(err.Error(), "invalid metadata path") {
		t.Fatalf("SaveMetadata() error = %v, want invalid metadata path", err)
	}
}

func TestLoadMetadata_InvalidYAML(t *testing.T) {
	path := filepath.Join(t.TempDir(), "metadata.yaml")
	bad := "rotation:\n  key-1: [\n"
	if err := os.WriteFile(path, []byte(bad), 0o600); err != nil {
		t.Fatalf("write metadata fixture: %v", err)
	}

	_, err := LoadMetadata(path)
	if err == nil {
		t.Fatal("LoadMetadata() expected YAML parse error")
	}
	if !strings.Contains(err.Error(), "parse metadata YAML") {
		t.Fatalf("LoadMetadata() error = %v, want parse metadata YAML", err)
	}
}

func TestApplyEnvOverrides_InvalidValuesIgnored(t *testing.T) {
	cfg := &Config{}
	cfg.Server.Port = 8443
	cfg.Logging.MaxSizeMB = 100
	cfg.Logging.MaxBackups = 10
	cfg.Logging.MaxAgeDays = 30
	compress := true
	cfg.Logging.Compress = &compress

	t.Setenv("HSM_SERVER_PORT", "not-a-number")
	t.Setenv("HSM_LOG_MAX_SIZE_MB", "bad")
	t.Setenv("HSM_LOG_MAX_BACKUPS", "bad")
	t.Setenv("HSM_LOG_MAX_AGE_DAYS", "bad")
	t.Setenv("HSM_LOG_COMPRESS", "not-bool")

	applyEnvOverrides(cfg)

	if cfg.Server.Port != 8443 {
		t.Fatalf("Server.Port = %d, want 8443", cfg.Server.Port)
	}
	if cfg.Logging.MaxSizeMB != 100 {
		t.Fatalf("Logging.MaxSizeMB = %d, want 100", cfg.Logging.MaxSizeMB)
	}
	if cfg.Logging.MaxBackups != 10 {
		t.Fatalf("Logging.MaxBackups = %d, want 10", cfg.Logging.MaxBackups)
	}
	if cfg.Logging.MaxAgeDays != 30 {
		t.Fatalf("Logging.MaxAgeDays = %d, want 30", cfg.Logging.MaxAgeDays)
	}
	if cfg.Logging.Compress == nil || *cfg.Logging.Compress != true {
		t.Fatalf("Logging.Compress = %v, want true", cfg.Logging.Compress)
	}
}
