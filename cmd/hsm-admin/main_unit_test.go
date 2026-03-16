package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func TestGetConfigPath_FromEnv(t *testing.T) {
	t.Setenv("CONFIG_PATH", "/tmp/custom-config.yaml")
	if got := getConfigPath(); got != "/tmp/custom-config.yaml" {
		t.Fatalf("getConfigPath() = %q, want %q", got, "/tmp/custom-config.yaml")
	}
}

func TestGetConfigPath_FromCurrentDir(t *testing.T) {
	t.Setenv("CONFIG_PATH", "")
	dir := t.TempDir()
	t.Chdir(dir)

	cfgPath := filepath.Join(dir, defaultConfigPath)
	if err := os.WriteFile(cfgPath, []byte("server: {}\n"), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	if got := getConfigPath(); got != defaultConfigPath {
		t.Fatalf("getConfigPath() = %q, want %q", got, defaultConfigPath)
	}
}

func TestGetConfigPath_DefaultWhenMissing(t *testing.T) {
	t.Setenv("CONFIG_PATH", "")
	t.Chdir(t.TempDir())

	if got := getConfigPath(); got != defaultConfigPath {
		t.Fatalf("getConfigPath() = %q, want %q", got, defaultConfigPath)
	}
}

func TestCopyFile_Success(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "src.yaml")
	dst := filepath.Join(dir, "dst.yaml")
	content := []byte("rotation: {}\n")

	if err := os.WriteFile(src, content, 0o600); err != nil {
		t.Fatalf("write src: %v", err)
	}

	if err := copyFile(src, dst); err != nil {
		t.Fatalf("copyFile() error: %v", err)
	}

	got, err := os.ReadFile(dst)
	if err != nil {
		t.Fatalf("read dst: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("copied content = %q, want %q", string(got), string(content))
	}
}

func TestCopyFile_PathTraversalRejected(t *testing.T) {
	err := copyFile("../src.yaml", "dst.yaml")
	if err == nil {
		t.Fatal("copyFile() expected path traversal error")
	}
	if !strings.Contains(err.Error(), "invalid path") {
		t.Fatalf("copyFile() error = %v, want invalid path", err)
	}
}

func TestGetKeys_ReturnsAllKeys(t *testing.T) {
	m := map[string]config.KeyMetadata{
		"a": {},
		"b": {},
	}
	keys := getKeys(m)
	if len(keys) != 2 {
		t.Fatalf("len(getKeys()) = %d, want 2", len(keys))
	}
	seen := map[string]bool{}
	for _, k := range keys {
		seen[k] = true
	}
	if !seen["a"] || !seen["b"] {
		t.Fatalf("getKeys() missing keys, got %v", keys)
	}
}

func TestGenerateKeyID_LengthAndHex(t *testing.T) {
	id, hexID := generateKeyID()
	if len(id) != 8 {
		t.Fatalf("len(id) = %d, want 8", len(id))
	}
	if len(hexID) != 16 {
		t.Fatalf("len(hexID) = %d, want 16", len(hexID))
	}
	for _, c := range hexID {
		if (c < '0' || c > '9') && (c < 'a' || c > 'f') {
			t.Fatalf("hexID contains non-hex char: %q", c)
		}
	}
}

func TestCreateKEKCommand_ValidateRequiredFlags(t *testing.T) {
	err := createKEKCommand([]string{"--label", "kek-a"})
	if err == nil {
		t.Fatal("createKEKCommand() expected required flags error")
	}
	if !strings.Contains(err.Error(), "--label and --context are required") {
		t.Fatalf("createKEKCommand() error = %v", err)
	}
}

func TestCreateKEKCommand_InvalidSize(t *testing.T) {
	err := createKEKCommand([]string{"--label", "kek-a", "--context", "exchange-key", "--size", "111"})
	if err == nil {
		t.Fatal("createKEKCommand() expected invalid size error")
	}
	if !strings.Contains(err.Error(), "--size must be 128, 192, or 256") {
		t.Fatalf("createKEKCommand() error = %v", err)
	}
}

func TestCreateKEKCommand_MissingPIN(t *testing.T) {
	t.Setenv("HSM_PIN", "")
	err := createKEKCommand([]string{"--label", "kek-a", "--context", "exchange-key"})
	if err == nil {
		t.Fatal("createKEKCommand() expected missing HSM_PIN error")
	}
	if !strings.Contains(err.Error(), "HSM_PIN environment variable not set") {
		t.Fatalf("createKEKCommand() error = %v", err)
	}
}

func TestRotateKeyCommand_UsageOnMissingArgs(t *testing.T) {
	err := rotateKeyCommand([]string{})
	if err == nil {
		t.Fatal("rotateKeyCommand() expected usage error")
	}
	if !strings.Contains(err.Error(), "usage: hsm-admin rotate <context>") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}

func TestCleanupOldVersionsCommand_MissingPIN(t *testing.T) {
	t.Setenv("HSM_PIN", "")
	err := cleanupOldVersionsCommand([]string{})
	if err == nil {
		t.Fatal("cleanupOldVersionsCommand() expected missing HSM_PIN error")
	}
	if !strings.Contains(err.Error(), "HSM_PIN environment variable not set") {
		t.Fatalf("cleanupOldVersionsCommand() error = %v", err)
	}
}

func TestUpdateChecksumsCommand_MissingPIN(t *testing.T) {
	t.Setenv("HSM_PIN", "")
	err := updateChecksumsCommand([]string{})
	if err == nil {
		t.Fatal("updateChecksumsCommand() expected missing HSM_PIN error")
	}
	if !strings.Contains(err.Error(), "HSM_PIN environment variable not set") {
		t.Fatalf("updateChecksumsCommand() error = %v", err)
	}
}
