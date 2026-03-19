package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func installRotateHooks(t *testing.T, createFn func(cfg *config.Config, pin, label, contextName string, version, keySize int) error, copyFn func(src, dst string) error) {
	t.Helper()
	origCreate := rotateCreateKEK
	origCopy := rotateCopyFile
	rotateCreateKEK = createFn
	rotateCopyFile = copyFn
	t.Cleanup(func() {
		rotateCreateKEK = origCreate
		rotateCopyFile = origCopy
	})
}

func writeRotateConfigAndMetadata(t *testing.T, dir string) (string, string) {
	t.Helper()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")

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
		"    current: \"kek-exchange-v1\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	return configPath, metadataPath
}

func TestRotateKeyCommand_SuccessUpdatesMetadata(t *testing.T) {
	dir := t.TempDir()
	configPath, metadataPath := writeRotateConfigAndMetadata(t, dir)
	t.Setenv("CONFIG_PATH", configPath)
	t.Setenv("HSM_PIN", "1234")

	t.Chdir(dir)

	called := false
	var gotLabel string
	var gotVersion int
	installRotateHooks(t,
		func(_ *config.Config, _ string, label, contextName string, version, keySize int) error {
			called = true
			gotLabel = label
			gotVersion = version
			if contextName != "exchange-key" {
				return errors.New("unexpected context")
			}
			if keySize != defaultKeySize {
				return errors.New("unexpected key size")
			}
			return nil
		},
		copyFile,
	)

	if err := rotateKeyCommand([]string{"exchange-key"}); err != nil {
		t.Fatalf("rotateKeyCommand() error: %v", err)
	}

	if !called {
		t.Fatal("expected rotateCreateKEK hook to be called")
	}
	if gotLabel != "kek-exchange-v2" || gotVersion != 2 {
		t.Fatalf("unexpected new key params: label=%s version=%d", gotLabel, gotVersion)
	}

	updated, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	text := string(updated)
	if !strings.Contains(text, "current: kek-exchange-v2") {
		t.Fatalf("expected current key switch, got: %s", text)
	}
	if !strings.Contains(text, "label: kek-exchange-v2") || !strings.Contains(text, "version: 2") {
		t.Fatalf("expected new version entry, got: %s", text)
	}

	backups, err := filepath.Glob(filepath.Join(dir, "metadata.yaml.backup-*"))
	if err != nil {
		t.Fatalf("glob backups: %v", err)
	}
	if len(backups) == 0 {
		t.Fatal("expected backup file to be created")
	}
}

func TestRotateKeyCommand_CopyBackupWarningStillSucceeds(t *testing.T) {
	dir := t.TempDir()
	configPath, metadataPath := writeRotateConfigAndMetadata(t, dir)
	t.Setenv("CONFIG_PATH", configPath)
	t.Setenv("HSM_PIN", "1234")

	installRotateHooks(t,
		func(_ *config.Config, _ string, _ string, _ string, _ int, _ int) error { return nil },
		func(_, _ string) error { return errors.New("backup failed") },
	)

	if err := rotateKeyCommand([]string{"exchange-key"}); err != nil {
		t.Fatalf("rotateKeyCommand() should succeed even when backup copy fails, got: %v", err)
	}

	updated, err := os.ReadFile(metadataPath)
	if err != nil {
		t.Fatalf("read metadata: %v", err)
	}
	if !strings.Contains(string(updated), "current: kek-exchange-v2") {
		t.Fatalf("expected metadata to be updated despite backup warning, got: %s", string(updated))
	}
}

func TestRotateKeyCommand_CreateKEKFailure(t *testing.T) {
	dir := t.TempDir()
	configPath, _ := writeRotateConfigAndMetadata(t, dir)
	t.Setenv("CONFIG_PATH", configPath)
	t.Setenv("HSM_PIN", "1234")

	installRotateHooks(t,
		func(_ *config.Config, _ string, _ string, _ string, _ int, _ int) error {
			return errors.New("create failed")
		},
		copyFile,
	)

	err := rotateKeyCommand([]string{"exchange-key"})
	if err == nil || !strings.Contains(err.Error(), "failed to create new KEK") {
		t.Fatalf("rotateKeyCommand() error = %v", err)
	}
}
