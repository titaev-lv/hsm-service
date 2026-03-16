package main

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	oldStdout := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	os.Stdout = w

	fn()

	_ = w.Close()
	os.Stdout = oldStdout

	out, err := io.ReadAll(r)
	_ = r.Close()
	if err != nil {
		t.Fatalf("io.ReadAll: %v", err)
	}
	return string(out)
}

func TestPrintUsage_IncludesCoreCommands(t *testing.T) {
	out := captureStdout(t, func() {
		printUsage()
	})

	if !strings.Contains(out, "HSM Admin Tool - KEK Management") {
		t.Fatalf("printUsage() missing header, got: %s", out)
	}
	if !strings.Contains(out, "create-kek") || !strings.Contains(out, "rotation-status") {
		t.Fatalf("printUsage() missing expected commands, got: %s", out)
	}
}

func TestCopyFile_ReadError(t *testing.T) {
	dir := t.TempDir()
	err := copyFile(filepath.Join(dir, "missing.yaml"), filepath.Join(dir, "out.yaml"))
	if err == nil {
		t.Fatal("copyFile() expected read error for missing source")
	}
}

func TestExportMetadata_WritesJSONFile(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")
	outputPath := filepath.Join(dir, "kek-metadata.json")

	writeStatusTestConfig(t, configPath, metadataPath)
	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	out := captureStdout(t, func() {
		exportMetadata([]string{"--config", configPath, "--output", outputPath})
	})

	if !strings.Contains(out, "Metadata exported to") {
		t.Fatalf("exportMetadata output missing success text, got: %s", out)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	var payload map[string]any
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal output json: %v", err)
	}
	if got, ok := payload["kek_count"].(float64); !ok || int(got) != 1 {
		t.Fatalf("kek_count = %v, want 1", payload["kek_count"])
	}
}

func TestExportMetadata_MissingMetadataStillWritesJSON(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "missing-metadata.yaml")
	outputPath := filepath.Join(dir, "kek-metadata.json")

	writeStatusTestConfig(t, configPath, metadataPath)

	out := captureStdout(t, func() {
		exportMetadata([]string{"--config", configPath, "--output", outputPath})
	})

	if !strings.Contains(out, "Metadata exported to") {
		t.Fatalf("exportMetadata output missing success text, got: %s", out)
	}

	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}

	var payload struct {
		KEKCount float64 `json:"kek_count"`
		KEKs     []struct {
			ConfigKey string `json:"config_key"`
			Label     string `json:"label"`
		} `json:"keks"`
	}
	if err := json.Unmarshal(data, &payload); err != nil {
		t.Fatalf("unmarshal output json: %v", err)
	}
	if int(payload.KEKCount) != 1 {
		t.Fatalf("kek_count = %v, want 1", payload.KEKCount)
	}
	if len(payload.KEKs) != 1 || payload.KEKs[0].ConfigKey != "exchange-key" || payload.KEKs[0].Label != "" {
		t.Fatalf("unexpected exported KEKs: %+v", payload.KEKs)
	}
}

func TestRotateKeyCommand_MetadataContextNotFound(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	metadataYAML := "rotation:\n  another-key:\n    current: \"kek-another-v1\"\n    versions:\n      - label: \"kek-another-v1\"\n        version: 1\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)
	err := rotateKeyCommand([]string{"exchange-key"})
	if err == nil {
		t.Fatal("rotateKeyCommand() expected missing context error")
	}
	if !strings.Contains(err.Error(), "not found in metadata") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}

func TestRotateKeyCommand_NoVersionsFound(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	metadataYAML := "rotation:\n  exchange-key:\n    current: \"kek-exchange-v1\"\n    versions: []\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)
	err := rotateKeyCommand([]string{"exchange-key"})
	if err == nil {
		t.Fatal("rotateKeyCommand() expected no versions error")
	}
	if !strings.Contains(err.Error(), "no versions found") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}

func TestRotateKeyCommand_CurrentVersionMissingInList(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v2\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)
	err := rotateKeyCommand([]string{"exchange-key"})
	if err == nil {
		t.Fatal("rotateKeyCommand() expected current version missing error")
	}
	if !strings.Contains(err.Error(), "current version kek-exchange-v2 not found") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}

func TestRotateKeyCommand_InvalidCurrentLabelFormat(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kekexchangev1\"\n" +
		"    versions:\n" +
		"      - label: \"kekexchangev1\"\n" +
		"        version: 1\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)
	err := rotateKeyCommand([]string{"exchange-key"})
	if err == nil {
		t.Fatal("rotateKeyCommand() expected invalid label format error")
	}
	if !strings.Contains(err.Error(), "invalid key label format") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}

func TestRotateKeyCommand_DuplicateVersionLabelRejected(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"      - label: \"kek-exchange-v3\"\n" +
		"        version: 2\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)
	err := rotateKeyCommand([]string{"exchange-key"})
	if err == nil {
		t.Fatal("rotateKeyCommand() expected duplicate version error")
	}
	if !strings.Contains(err.Error(), "already exists") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}

func TestRotateKeyCommand_MissingPINAfterMetadataChecks(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"      - label: \"kek-exchange-v2\"\n" +
		"        version: 2\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)
	t.Setenv("HSM_PIN", "")
	err := rotateKeyCommand([]string{"exchange-key"})
	if err == nil {
		t.Fatal("rotateKeyCommand() expected missing HSM_PIN error")
	}
	if !strings.Contains(err.Error(), "HSM_PIN environment variable not set") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}

func TestCleanupOldVersionsCommand_MetadataLoadError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "missing-metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	t.Setenv("HSM_PIN", "1234")

	err := cleanupOldVersionsCommand([]string{"--config", configPath})
	if err == nil {
		t.Fatal("cleanupOldVersionsCommand() expected metadata load error")
	}
	if !strings.Contains(err.Error(), "failed to load metadata") {
		t.Fatalf("cleanupOldVersionsCommand() error = %v", err)
	}
}

func TestCleanupOldVersionsCommand_DryRunSingleVersionSkips(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	metadataYAML := "rotation:\n  exchange-key:\n    current: \"kek-exchange-v1\"\n    versions:\n      - label: \"kek-exchange-v1\"\n        version: 1\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		if err := cleanupOldVersionsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
			t.Fatalf("cleanupOldVersionsCommand() error: %v", err)
		}
	})

	if !strings.Contains(out, "Only 1 version, skipping") {
		t.Fatalf("cleanup output missing single-version skip, got: %s", out)
	}
}

func TestCleanupOldVersionsCommand_DryRunNoVersionsToDelete(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")
	recentCreatedAt := time.Now().AddDate(0, 0, -5).UTC().Format(time.RFC3339)

	configYAML := "server:\n" +
		"  port: \"8443\"\n" +
		"  tls:\n" +
		"    cert_path: \"/tmp/cert.crt\"\n" +
		"    key_path: \"/tmp/cert.key\"\n" +
		"    ca_path: \"/tmp/ca.crt\"\n" +
		"hsm:\n" +
		"  pkcs11_lib: \"/tmp/fake-pkcs11.so\"\n" +
		"  slot_id: \"slot-0\"\n" +
		"  metadata_file: \"" + metadataPath + "\"\n" +
		"  max_versions: 3\n" +
		"  cleanup_after_days: 365\n" +
		"  keys:\n" +
		"    exchange-key:\n" +
		"      type: \"aes\"\n" +
		"acl:\n" +
		"  mappings:\n" +
		"    Trading:\n" +
		"      - exchange-key\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v2\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + recentCreatedAt + "\"\n" +
		"      - label: \"kek-exchange-v2\"\n" +
		"        version: 2\n" +
		"        created_at: \"" + recentCreatedAt + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		if err := cleanupOldVersionsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
			t.Fatalf("cleanupOldVersionsCommand() error: %v", err)
		}
	})

	if !strings.Contains(out, "No versions to delete") {
		t.Fatalf("cleanup output missing no-op message, got: %s", out)
	}
}

func TestUpdateChecksumsCommand_MetadataLoadError(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "missing-metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	t.Setenv("HSM_PIN", "1234")

	err := updateChecksumsCommand([]string{"--config", configPath})
	if err == nil {
		t.Fatal("updateChecksumsCommand() expected metadata load error")
	}
	if !strings.Contains(err.Error(), "failed to load metadata") {
		t.Fatalf("updateChecksumsCommand() error = %v", err)
	}
}

func TestCheckRotationStatusCommand_OverdueStatus(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	old := time.Now().AddDate(0, 0, -120).UTC().Format(time.RFC3339)
	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + old + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("CONFIG_PATH", configPath)
	out, err := captureStdoutFromStatusCall(t)
	if err != nil {
		t.Fatalf("checkRotationStatusCommand() error: %v", err)
	}
	if !strings.Contains(out, "NEEDS ROTATION") || !strings.Contains(out, "days overdue") {
		t.Fatalf("status output missing overdue state, got: %s", out)
	}
}

func TestCleanupOldVersionsCommand_DryRunDeletesOldVersionsOnlyInOutput(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	oldCreatedAt := time.Now().AddDate(0, 0, -120).UTC().Format(time.RFC3339)
	recentCreatedAt := time.Now().AddDate(0, 0, -5).UTC().Format(time.RFC3339)
	configYAML := "server:\n" +
		"  port: \"8443\"\n" +
		"  tls:\n" +
		"    cert_path: \"/tmp/cert.crt\"\n" +
		"    key_path: \"/tmp/cert.key\"\n" +
		"    ca_path: \"/tmp/ca.crt\"\n" +
		"hsm:\n" +
		"  pkcs11_lib: \"/tmp/fake-pkcs11.so\"\n" +
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
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v3\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + oldCreatedAt + "\"\n" +
		"      - label: \"kek-exchange-v2\"\n" +
		"        version: 2\n" +
		"        created_at: \"" + recentCreatedAt + "\"\n" +
		"      - label: \"kek-exchange-v3\"\n" +
		"        version: 3\n" +
		"        created_at: \"" + recentCreatedAt + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		if err := cleanupOldVersionsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
			t.Fatalf("cleanupOldVersionsCommand() error: %v", err)
		}
	})

	if !strings.Contains(out, "DRY RUN MODE - No changes will be made") {
		t.Fatalf("cleanup output missing dry-run banner, got: %s", out)
	}
	if !strings.Contains(out, "Would delete kek-exchange-v1 (v1)") {
		t.Fatalf("cleanup output missing delete preview, got: %s", out)
	}
	if !strings.Contains(out, "DRY RUN COMPLETE - Would delete 1 versions") {
		t.Fatalf("cleanup output missing final summary, got: %s", out)
	}

	after, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata after dry-run: %v", err)
	}
	if !strings.Contains(string(after), "kek-exchange-v1") {
		t.Fatalf("dry-run should not modify metadata, got: %s", string(after))
	}
}

func TestCleanupOldVersionsCommand_DryRunMarksExcessVersions(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")
	createdAt := time.Now().AddDate(0, 0, -5).UTC().Format(time.RFC3339)

	configYAML := "server:\n" +
		"  port: \"8443\"\n" +
		"  tls:\n" +
		"    cert_path: \"/tmp/cert.crt\"\n" +
		"    key_path: \"/tmp/cert.key\"\n" +
		"    ca_path: \"/tmp/ca.crt\"\n" +
		"hsm:\n" +
		"  pkcs11_lib: \"/tmp/fake-pkcs11.so\"\n" +
		"  slot_id: \"slot-0\"\n" +
		"  metadata_file: \"" + metadataPath + "\"\n" +
		"  max_versions: 2\n" +
		"  cleanup_after_days: 365\n" +
		"  keys:\n" +
		"    exchange-key:\n" +
		"      type: \"aes\"\n" +
		"acl:\n" +
		"  mappings:\n" +
		"    Trading:\n" +
		"      - exchange-key\n"
	if err := os.WriteFile(configPath, []byte(configYAML), 0o600); err != nil {
		t.Fatalf("write config: %v", err)
	}

	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v4\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + createdAt + "\"\n" +
		"      - label: \"kek-exchange-v2\"\n" +
		"        version: 2\n" +
		"        created_at: \"" + createdAt + "\"\n" +
		"      - label: \"kek-exchange-v3\"\n" +
		"        version: 3\n" +
		"        created_at: \"" + createdAt + "\"\n" +
		"      - label: \"kek-exchange-v4\"\n" +
		"        version: 4\n" +
		"        created_at: \"" + createdAt + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		if err := cleanupOldVersionsCommand([]string{"--config", configPath, "--dry-run"}); err != nil {
			t.Fatalf("cleanupOldVersionsCommand() error: %v", err)
		}
	})

	if !strings.Contains(out, "EXCEEDS MAX VERSIONS") {
		t.Fatalf("cleanup output missing max versions warning, got: %s", out)
	}
	if !strings.Contains(out, "Would delete kek-exchange-v1 (v1)") {
		t.Fatalf("cleanup output missing excess-version delete preview, got: %s", out)
	}
}

func TestCleanupOldVersionsCommand_ConfigureFailure(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	if err := os.WriteFile(metadataPath, []byte("rotation:\n  exchange-key:\n    current: \"kek-exchange-v1\"\n    versions:\n      - label: \"kek-exchange-v1\"\n        version: 1\n      - label: \"kek-exchange-v2\"\n        version: 2\n"), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "1234")
	err := cleanupOldVersionsCommand([]string{"--config", configPath, "--force"})
	if err == nil {
		t.Fatal("cleanupOldVersionsCommand() expected PKCS#11 configure error")
	}
	if !strings.Contains(err.Error(), "failed to configure PKCS#11") {
		t.Fatalf("cleanupOldVersionsCommand() error = %v", err)
	}
}

func TestUpdateChecksumsCommand_ConfigureFailure(t *testing.T) {
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

	writeStatusTestConfig(t, configPath, metadataPath)
	if err := os.WriteFile(metadataPath, []byte("rotation:\n  exchange-key:\n    current: \"kek-exchange-v1\"\n    versions:\n      - label: \"kek-exchange-v1\"\n        version: 1\n"), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	t.Setenv("HSM_PIN", "1234")
	err := updateChecksumsCommand([]string{"--config", configPath})
	if err == nil {
		t.Fatal("updateChecksumsCommand() expected PKCS#11 configure error")
	}
	if !strings.Contains(err.Error(), "failed to configure PKCS#11") {
		t.Fatalf("updateChecksumsCommand() error = %v", err)
	}
}
