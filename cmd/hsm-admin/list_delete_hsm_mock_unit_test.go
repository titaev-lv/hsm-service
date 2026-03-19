package main

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/titaev-lv/hsm-service/internal/config"
)

type fakeHSMKey struct {
	deleteErr error
	deleted   bool
}

func (k *fakeHSMKey) Delete() error {
	k.deleted = true
	return k.deleteErr
}

type fakeHSMContext struct {
	findFn  func(id []byte, label []byte) (hsmKey, error)
	closed  bool
	findCnt int
}

func (c *fakeHSMContext) FindKey(id []byte, label []byte) (hsmKey, error) {
	c.findCnt++
	if c.findFn != nil {
		return c.findFn(id, label)
	}
	return nil, nil
}

func (c *fakeHSMContext) Close() {
	c.closed = true
}

func installHSMFactoryHook(t *testing.T, fn func(cfg *config.Config, pin string) (hsmContext, error)) {
	t.Helper()
	orig := newHSMCtx
	newHSMCtx = fn
	t.Cleanup(func() {
		newHSMCtx = orig
	})
}

func writeListDeleteConfig(t *testing.T, dir, metadataPath string) string {
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

func TestListKEK_Verbose_ShowsHSMAvailability(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeListDeleteConfig(t, dir, metadataPath)

	metadataYAML := "rotation:\n" +
		"  exchange-key:\n" +
		"    current: \"kek-exchange-v2\"\n" +
		"    versions:\n" +
		"      - label: \"kek-exchange-v1\"\n" +
		"        version: 1\n" +
		"      - label: \"kek-exchange-v2\"\n" +
		"        version: 2\n"
	if err := os.WriteFile(metadataPath, []byte(metadataYAML), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	fakeCtx := &fakeHSMContext{}
	fakeCtx.findFn = func(_ []byte, label []byte) (hsmKey, error) {
		if string(label) == "kek-exchange-v2" {
			return &fakeHSMKey{}, nil
		}
		return nil, nil
	}
	installHSMFactoryHook(t, func(_ *config.Config, _ string) (hsmContext, error) {
		return fakeCtx, nil
	})

	installMainHooks(t)
	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		listKEK([]string{"--config", configPath, "--verbose"})
	})

	if !strings.Contains(out, "kek-exchange-v1: ⚠") {
		t.Fatalf("expected NOT FOUND marker, got: %s", out)
	}
	if !strings.Contains(out, "kek-exchange-v2: ✓ Available in HSM") {
		t.Fatalf("expected Available marker, got: %s", out)
	}
	if fakeCtx.findCnt < 2 {
		t.Fatalf("expected at least two FindKey calls, got %d", fakeCtx.findCnt)
	}
	if !fakeCtx.closed {
		t.Fatal("expected fake context to be closed")
	}
}

func TestListKEK_MissingMetadata_PrintsWarningAndContinues(t *testing.T) {
	dir := t.TempDir()
	missingMetadataPath := filepath.Join(dir, "missing-metadata.yaml")
	configPath := writeListDeleteConfig(t, dir, missingMetadataPath)

	fakeCtx := &fakeHSMContext{}
	installHSMFactoryHook(t, func(_ *config.Config, _ string) (hsmContext, error) {
		return fakeCtx, nil
	})

	installMainHooks(t)
	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		listKEK([]string{"--config", configPath})
	})

	if !strings.Contains(out, "Config Key: exchange-key") {
		t.Fatalf("expected config key output, got: %s", out)
	}
	if !strings.Contains(out, "Label: (not in metadata)") {
		t.Fatalf("expected not-in-metadata marker, got: %s", out)
	}
	if !strings.Contains(out, "Total: 1 KEK(s)") {
		t.Fatalf("expected total count output, got: %s", out)
	}
}

func TestDeleteKEK_FindError_TriggersFatalf(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeListDeleteConfig(t, dir, metadataPath)

	fakeCtx := &fakeHSMContext{}
	fakeCtx.findFn = func(_ []byte, _ []byte) (hsmKey, error) {
		return nil, errors.New("find failed")
	}
	installHSMFactoryHook(t, func(_ *config.Config, _ string) (hsmContext, error) {
		return fakeCtx, nil
	})

	installMainHooks(t)
	t.Setenv("HSM_PIN", "1234")
	got := expectMainHookPanic(t, func() {
		deleteKEK([]string{"--config", configPath, "--label", "kek-a", "--confirm"})
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to find KEK") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestDeleteKEK_KeyNotFound_TriggersFatalf(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeListDeleteConfig(t, dir, metadataPath)

	fakeCtx := &fakeHSMContext{}
	fakeCtx.findFn = func(_ []byte, _ []byte) (hsmKey, error) {
		return nil, nil
	}
	installHSMFactoryHook(t, func(_ *config.Config, _ string) (hsmContext, error) {
		return fakeCtx, nil
	})

	installMainHooks(t)
	t.Setenv("HSM_PIN", "1234")
	got := expectMainHookPanic(t, func() {
		deleteKEK([]string{"--config", configPath, "--label", "kek-a", "--confirm"})
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "KEK not found") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestDeleteKEK_DeleteError_TriggersFatalf(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeListDeleteConfig(t, dir, metadataPath)

	key := &fakeHSMKey{deleteErr: errors.New("delete failed")}
	fakeCtx := &fakeHSMContext{}
	fakeCtx.findFn = func(_ []byte, _ []byte) (hsmKey, error) {
		return key, nil
	}
	installHSMFactoryHook(t, func(_ *config.Config, _ string) (hsmContext, error) {
		return fakeCtx, nil
	})

	installMainHooks(t)
	t.Setenv("HSM_PIN", "1234")
	got := expectMainHookPanic(t, func() {
		deleteKEK([]string{"--config", configPath, "--label", "kek-a", "--confirm"})
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to delete KEK") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
	if !key.deleted {
		t.Fatal("expected fake key delete to be attempted")
	}
}

func TestDeleteKEK_Success_PrintsConfirmation(t *testing.T) {
	dir := t.TempDir()
	metadataPath := filepath.Join(dir, "metadata.yaml")
	configPath := writeListDeleteConfig(t, dir, metadataPath)

	key := &fakeHSMKey{}
	fakeCtx := &fakeHSMContext{}
	fakeCtx.findFn = func(_ []byte, _ []byte) (hsmKey, error) {
		return key, nil
	}
	installHSMFactoryHook(t, func(_ *config.Config, _ string) (hsmContext, error) {
		return fakeCtx, nil
	})

	installMainHooks(t)
	t.Setenv("HSM_PIN", "1234")
	out := captureStdout(t, func() {
		deleteKEK([]string{"--config", configPath, "--label", "kek-a", "--confirm"})
	})

	if !strings.Contains(out, "Searching for KEK: kek-a") {
		t.Fatalf("expected search output, got: %s", out)
	}
	if !strings.Contains(out, "KEK deleted successfully") {
		t.Fatalf("expected success output, got: %s", out)
	}
	if !key.deleted {
		t.Fatal("expected key to be deleted")
	}
	if !fakeCtx.closed {
		t.Fatal("expected fake context to be closed")
	}
}
