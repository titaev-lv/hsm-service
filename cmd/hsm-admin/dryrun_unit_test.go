package main

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// writeDryRunConfig writes a minimal config.yaml that references metadataPath.
func writeDryRunConfig(t *testing.T, dir, metadataPath string) string {
	t.Helper()
	configPath := filepath.Join(dir, "config.yaml")
	yamlContent := "server:\n" +
		"  port: \"8443\"\n" +
		"  tls:\n" +
		"    cert_path: \"/tmp/cert.crt\"\n" +
		"    key_path: \"/tmp/cert.key\"\n" +
		"    ca_path: \"/tmp/ca.crt\"\n" +
		"hsm:\n" +
		"  pkcs11_lib: \"/tmp/fake.so\"\n" +
		"  slot_id: \"slot-0\"\n" +
		"  metadata_file: \"" + metadataPath + "\"\n" +
		"  max_versions: 3\n" +
		"  cleanup_after_days: 30\n" +
		"  keys:\n" +
		"    exchange-key:\n" +
		"      type: \"aes\"\n" +
		"acl:\n" +
		"  mappings:\n" +
		"    Trading:\n" +
		"      - exchange-key\n"
	if err := os.WriteFile(configPath, []byte(yamlContent), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return configPath
}

func labelChecksum(label string) string {
	h := sha256.New()
	h.Write([]byte(label))
	return hex.EncodeToString(h.Sum(nil))
}

// ---- updateChecksumsCommand dry-run tests --------------------------------

func TestUpdateChecksumsCommand_DryRun_NewChecksums(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeDryRunConfig(t, dir, metadataPath)

	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "test-pin")
	out := captureStdout(t, func() {
		if err := updateChecksumsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
			t.Fatalf("updateChecksumsCommand dry-run error: %v", err)
		}
	})

	if !strings.Contains(out, "Context: exchange-key") {
		t.Fatalf("expected context output, got: %s", out)
	}
	if !strings.Contains(out, "NEW checksum") {
		t.Fatalf("expected NEW checksum output, got: %s", out)
	}
	if !strings.Contains(out, "DRY RUN") {
		t.Fatalf("expected DRY RUN notice, got: %s", out)
	}
}

func TestUpdateChecksumsCommand_DryRun_UpToDate(t *testing.T) {
	existing := labelChecksum("kek-exchange-v1")

	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeDryRunConfig(t, dir, metadataPath)

	now := time.Now().UTC().Format(time.RFC3339)
	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + now + "\"\n" +
		"        checksum: \"" + existing + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "test-pin")
	out := captureStdout(t, func() {
		if err := updateChecksumsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
			t.Fatalf("updateChecksumsCommand dry-run error: %v", err)
		}
	})

	if !strings.Contains(out, "All checksums are up-to-date") {
		t.Fatalf("expected up-to-date output, got: %s", out)
	}
}

func TestUpdateChecksumsCommand_DryRun_MissingCreatedAt(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeDryRunConfig(t, dir, metadataPath)

	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "test-pin")
	if err := updateChecksumsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestUpdateChecksumsCommand_NoPin_ReturnsError(t *testing.T) {
	t.Setenv("HSM_PIN", "")
	err := updateChecksumsCommand([]string{})
	if err == nil || !strings.Contains(err.Error(), "HSM_PIN") {
		t.Fatalf("expected HSM_PIN error, got: %v", err)
	}
}

// ---- cleanupOldVersionsCommand dry-run tests ----------------------------

func TestCleanupOldVersionsCommand_DryRun_OnlyOneVersion(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeDryRunConfig(t, dir, metadataPath)

	now := time.Now().UTC().Format(time.RFC3339)
	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + now + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "test-pin")
	out := captureStdout(t, func() {
		if err := cleanupOldVersionsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
			t.Fatalf("cleanupOldVersionsCommand dry-run error: %v", err)
		}
	})

	if !strings.Contains(out, "Only 1 version") {
		t.Fatalf("expected 'Only 1 version' message, got: %s", out)
	}
}

func TestCleanupOldVersionsCommand_DryRun_NoVersionsToDelete(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeDryRunConfig(t, dir, metadataPath)

	now := time.Now().UTC().Format(time.RFC3339)
	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v2\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + now + "\"\n" +
		"      - label: \"kek-exchange-v2\"\n" +
		"        version: 2\n" +
		"        created_at: \"" + now + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "test-pin")
	out := captureStdout(t, func() {
		if err := cleanupOldVersionsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
			t.Fatalf("cleanupOldVersionsCommand dry-run error: %v", err)
		}
	})

	if !strings.Contains(out, "No versions to delete") {
		t.Fatalf("expected 'No versions to delete', got: %s", out)
	}
}

func TestCleanupOldVersionsCommand_DryRun_OldVersionMarkedForDeletion(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeDryRunConfig(t, dir, metadataPath)

	old := time.Now().AddDate(0, 0, -60).UTC().Format(time.RFC3339)
	now := time.Now().UTC().Format(time.RFC3339)
	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v2\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + old + "\"\n" +
		"      - label: \"kek-exchange-v2\"\n" +
		"        version: 2\n" +
		"        created_at: \"" + now + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "test-pin")
	out := captureStdout(t, func() {
		if err := cleanupOldVersionsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
			t.Fatalf("cleanupOldVersionsCommand dry-run error: %v", err)
		}
	})

	if !strings.Contains(out, "DRY-RUN") || !strings.Contains(out, "Would delete") {
		t.Fatalf("expected DRY-RUN deletion message, got: %s", out)
	}
	if !strings.Contains(out, "kek-exchange-v1") {
		t.Fatalf("expected v1 in deletion output, got: %s", out)
	}
}

func TestCleanupOldVersionsCommand_DryRun_NoCreatedAt(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeDryRunConfig(t, dir, metadataPath)

	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v2\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"      - label: \"kek-exchange-v2\"\n" +
		"        version: 2\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "test-pin")
	out := captureStdout(t, func() {
		if err := cleanupOldVersionsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
			t.Fatalf("cleanupOldVersionsCommand dry-run error: %v", err)
		}
	})

	if !strings.Contains(out, "no creation date") {
		t.Fatalf("expected 'no creation date' message, got: %s", out)
	}
}
