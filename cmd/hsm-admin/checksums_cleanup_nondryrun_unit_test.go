package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func writeCleanupChecksumsConfig(t *testing.T, dir, metadataPath string, maxVersions, cleanupDays int) string {
	t.Helper()
	configPath := filepath.Join(dir, "config.yaml")
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
		"  max_versions: " + itoa(maxVersions) + "\n" +
		"  cleanup_after_days: " + itoa(cleanupDays) + "\n" +
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
	return configPath
}

func itoa(v int) string {
	return strconv.Itoa(v)
}

func TestUpdateChecksumsCommand_NonDryRun_SuccessWritesMetadata(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeCleanupChecksumsConfig(t, dir, metadataPath, 3, 30)

	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	ctx := &fakeHSMContext{}
	ctx.findFn = func(_ []byte, _ []byte) (hsmKey, error) { return &fakeHSMKey{}, nil }
	installHSMFactoryHook(t, func(_ *config.Config, _ string) (hsmContext, error) {
		return ctx, nil
	})

	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		if err := updateChecksumsCommand([]string{"--config", configPath}); err != nil {
			t.Fatalf("updateChecksumsCommand() error: %v", err)
		}
	})

	if !strings.Contains(out, "Updated 2 checksum") {
		t.Fatalf("expected update summary, got: %s", out)
	}
	if !ctx.closed {
		t.Fatal("expected HSM context to be closed")
	}

	updated, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if !strings.Contains(string(updated), "checksum:") {
		t.Fatalf("expected checksum in metadata, got: %s", string(updated))
	}
	if !strings.Contains(string(updated), "created_at:") {
		t.Fatalf("expected created_at in metadata, got: %s", string(updated))
	}
}

func TestUpdateChecksumsCommand_NonDryRun_FindErrorsSkipAndNoUpdates(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeCleanupChecksumsConfig(t, dir, metadataPath, 3, 30)

	now := time.Now().UTC().Format(time.RFC3339)
	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + now + "\"\n" +
		"        checksum: \"abc\"\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	ctx := &fakeHSMContext{}
	ctx.findFn = func(_ []byte, _ []byte) (hsmKey, error) { return nil, errors.New("boom") }
	installHSMFactoryHook(t, func(_ *config.Config, _ string) (hsmContext, error) {
		return ctx, nil
	})

	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		if err := updateChecksumsCommand([]string{"--config", configPath}); err != nil {
			t.Fatalf("updateChecksumsCommand() error: %v", err)
		}
	})

	if !strings.Contains(out, "All checksums are up-to-date") {
		t.Fatalf("expected up-to-date message when all versions skipped, got: %s", out)
	}
}

func TestCleanupOldVersionsCommand_NonDryRun_ForceSuccessUpdatesMetadata(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeCleanupChecksumsConfig(t, dir, metadataPath, 2, 30)

	old := time.Now().AddDate(0, 0, -120).UTC().Format(time.RFC3339)
	recent := time.Now().AddDate(0, 0, -2).UTC().Format(time.RFC3339)
	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v3\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + old + "\"\n" +
		"      - label: \"kek-exchange-v2\"\n" +
		"        version: 2\n" +
		"        created_at: \"" + recent + "\"\n" +
		"      - label: \"kek-exchange-v3\"\n" +
		"        version: 3\n" +
		"        created_at: \"" + recent + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	ctx := &fakeHSMContext{}
	ctx.findFn = func(_ []byte, _ []byte) (hsmKey, error) { return &fakeHSMKey{}, nil }
	installHSMFactoryHook(t, func(_ *config.Config, _ string) (hsmContext, error) {
		return ctx, nil
	})

	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		if err := cleanupOldVersionsCommand([]string{"--config", configPath, "--force"}); err != nil {
			t.Fatalf("cleanupOldVersionsCommand() error: %v", err)
		}
	})

	if !strings.Contains(out, "CLEANUP COMPLETE - Deleted 1 versions") {
		t.Fatalf("expected cleanup summary, got: %s", out)
	}
	if !strings.Contains(out, "Metadata updated") {
		t.Fatalf("expected metadata updated output, got: %s", out)
	}

	updated, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if strings.Contains(string(updated), "kek-exchange-v1") {
		t.Fatalf("expected old version removed from metadata, got: %s", string(updated))
	}
	if !ctx.closed {
		t.Fatal("expected HSM context to be closed")
	}
}

func TestCleanupOldVersionsCommand_NonDryRun_DeleteFailuresContinue(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeCleanupChecksumsConfig(t, dir, metadataPath, 2, 30)

	old := time.Now().AddDate(0, 0, -120).UTC().Format(time.RFC3339)
	recent := time.Now().AddDate(0, 0, -2).UTC().Format(time.RFC3339)
	metaYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v3\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"        created_at: \"" + old + "\"\n" +
		"      - label: \"kek-exchange-v2\"\n" +
		"        version: 2\n" +
		"        created_at: \"" + recent + "\"\n" +
		"      - label: \"kek-exchange-v3\"\n" +
		"        version: 3\n" +
		"        created_at: \"" + recent + "\"\n"
	if err := os.WriteFile(metadataPath, []byte(metaYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	ctx := &fakeHSMContext{}
	ctx.findFn = func(_ []byte, label []byte) (hsmKey, error) {
		if string(label) == "kek-exchange-v1" {
			return nil, errors.New("not found")
		}
		return &fakeHSMKey{}, nil
	}
	installHSMFactoryHook(t, func(_ *config.Config, _ string) (hsmContext, error) {
		return ctx, nil
	})

	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		if err := cleanupOldVersionsCommand([]string{"--config", configPath, "--force"}); err != nil {
			t.Fatalf("cleanupOldVersionsCommand() error: %v", err)
		}
	})

	if !strings.Contains(out, "CLEANUP COMPLETE") {
		t.Fatalf("expected cleanup completion output, got: %s", out)
	}
}
