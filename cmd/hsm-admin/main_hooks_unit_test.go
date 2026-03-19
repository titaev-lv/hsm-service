package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

type mainHookPanic struct {
	kind string
	code int
	msg  string
}

func installMainHooks(t *testing.T) {
	t.Helper()
	origExit := exitFunc
	origFatalf := fatalfFunc
	origFatal := fatalFunc

	exitFunc = func(code int) {
		panic(mainHookPanic{kind: "exit", code: code})
	}
	fatalfFunc = func(format string, args ...any) {
		panic(mainHookPanic{kind: "fatalf", msg: fmt.Sprintf(format, args...)})
	}
	fatalFunc = func(args ...any) {
		panic(mainHookPanic{kind: "fatal", msg: fmt.Sprint(args...)})
	}

	t.Cleanup(func() {
		exitFunc = origExit
		fatalfFunc = origFatalf
		fatalFunc = origFatal
	})
}

func resetGlobalFlags(t *testing.T) {
	t.Helper()
	orig := flag.CommandLine
	fs := flag.NewFlagSet(os.Args[0], flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	flag.CommandLine = fs
	t.Cleanup(func() {
		flag.CommandLine = orig
	})
}

func expectMainHookPanic(t *testing.T, fn func()) (got mainHookPanic) {
	t.Helper()
	defer func() {
		r := recover()
		if r == nil {
			t.Fatal("expected panic from main hook")
		}
		var ok bool
		got, ok = r.(mainHookPanic)
		if !ok {
			t.Fatalf("unexpected panic type: %T", r)
		}
	}()

	fn()
	return
}

func TestListKEK_MissingPin_TriggersFatal(t *testing.T) {
	installMainHooks(t)
	t.Setenv("HSM_PIN", "")

	got := expectMainHookPanic(t, func() {
		listKEK([]string{})
	})

	if got.kind != "fatal" || !strings.Contains(got.msg, "HSM_PIN environment variable not set") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestListKEK_ConfigLoadFailure_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	t.Setenv("HSM_PIN", "1234")

	got := expectMainHookPanic(t, func() {
		listKEK([]string{"--config", "/tmp/definitely-missing-config.yaml"})
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to load config") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestListKEK_ConfigureFailure_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	t.Setenv("HSM_PIN", "1234")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")
	writeStatusTestConfig(t, configPath, metadataPath)

	got := expectMainHookPanic(t, func() {
		listKEK([]string{"--config", configPath})
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to configure PKCS#11") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestDeleteKEK_MissingLabel_TriggersExit(t *testing.T) {
	installMainHooks(t)

	got := expectMainHookPanic(t, func() {
		deleteKEK([]string{})
	})

	if got.kind != "exit" || got.code != 1 {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestDeleteKEK_MissingConfirm_TriggersExit(t *testing.T) {
	installMainHooks(t)

	got := expectMainHookPanic(t, func() {
		deleteKEK([]string{"--label", "kek-a"})
	})

	if got.kind != "exit" || got.code != 1 {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestDeleteKEK_MissingPin_TriggersFatal(t *testing.T) {
	installMainHooks(t)
	t.Setenv("HSM_PIN", "")

	got := expectMainHookPanic(t, func() {
		deleteKEK([]string{"--label", "kek-a", "--confirm"})
	})

	if got.kind != "fatal" || !strings.Contains(got.msg, "HSM_PIN environment variable not set") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestDeleteKEK_ConfigLoadFailure_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	t.Setenv("HSM_PIN", "1234")

	got := expectMainHookPanic(t, func() {
		deleteKEK([]string{"--label", "kek-a", "--confirm", "--config", "/tmp/definitely-missing-config.yaml"})
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to load config") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestDeleteKEK_ConfigureFailure_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	t.Setenv("HSM_PIN", "1234")
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")
	writeStatusTestConfig(t, configPath, metadataPath)

	got := expectMainHookPanic(t, func() {
		deleteKEK([]string{"--label", "kek-a", "--confirm", "--config", configPath})
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to configure PKCS#11") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestMain_NoArgs_TriggersExit(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin"}
	resetGlobalFlags(t)

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "exit" || got.code != 1 {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestMain_UnknownCommand_TriggersExit(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "unknown-command"}
	resetGlobalFlags(t)

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "exit" || got.code != 1 {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestMain_GlobalConfigFlagWithoutCommand_TriggersExit(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "-config", "/tmp/test-config.yaml"}
	resetGlobalFlags(t)

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "exit" || got.code != 1 {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
	if env := os.Getenv("CONFIG_PATH"); env != "/tmp/test-config.yaml" {
		t.Fatalf("CONFIG_PATH = %q, want /tmp/test-config.yaml", env)
	}
}

func TestMain_GlobalShortConfigFlagWithoutCommand_TriggersExit(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "-c", "/tmp/test-short-config.yaml"}
	resetGlobalFlags(t)

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "exit" || got.code != 1 {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
	if env := os.Getenv("CONFIG_PATH"); env != "/tmp/test-short-config.yaml" {
		t.Fatalf("CONFIG_PATH = %q, want /tmp/test-short-config.yaml", env)
	}
}

func TestMain_CreateKEKError_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "create-kek"}
	resetGlobalFlags(t)

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Create KEK failed") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestMain_RotateError_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "rotate"}
	resetGlobalFlags(t)

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Rotation failed") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestMain_RotationStatusError_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "rotation-status"}
	resetGlobalFlags(t)
	t.Setenv("CONFIG_PATH", "/tmp/definitely-missing-config.yaml")

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to check rotation status") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestMain_CleanupError_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "cleanup-old-versions"}
	resetGlobalFlags(t)
	t.Setenv("HSM_PIN", "")

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to cleanup old versions") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestMain_UpdateChecksumsError_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "update-checksums"}
	resetGlobalFlags(t)
	t.Setenv("HSM_PIN", "")

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to update checksums") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestMain_ListKEKError_TriggersFatal(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "list-kek"}
	resetGlobalFlags(t)
	t.Setenv("HSM_PIN", "")

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "fatal" || !strings.Contains(got.msg, "HSM_PIN environment variable not set") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestMain_DeleteKEKError_TriggersExit(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "delete-kek"}
	resetGlobalFlags(t)

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "exit" || got.code != 1 {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestMain_ExportMetadataError_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	origArgs := os.Args
	t.Cleanup(func() { os.Args = origArgs })
	os.Args = []string{"hsm-admin", "export-metadata", "--config", "/tmp/definitely-missing-config.yaml"}
	resetGlobalFlags(t)

	got := expectMainHookPanic(t, func() {
		main()
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to load config") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestExportMetadata_ConfigLoadFailure_TriggersFatalf(t *testing.T) {
	installMainHooks(t)

	got := expectMainHookPanic(t, func() {
		exportMetadata([]string{"--config", "/tmp/definitely-missing-config.yaml"})
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to load config") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}

func TestExportMetadata_OutputCreateFailure_TriggersFatalf(t *testing.T) {
	installMainHooks(t)
	dir := t.TempDir()
	configPath := filepath.Join(dir, "config.yaml")
	metadataPath := filepath.Join(dir, "metadata.yaml")
	writeStatusTestConfig(t, configPath, metadataPath)
	if err := os.WriteFile(metadataPath, []byte("rotation: {}\n"), 0o600); err != nil {
		t.Fatalf("write metadata: %v", err)
	}

	missingDirOutput := filepath.Join(dir, "no-such-dir", "out.json")
	got := expectMainHookPanic(t, func() {
		exportMetadata([]string{"--config", configPath, "--output", missingDirOutput})
	})

	if got.kind != "fatalf" || !strings.Contains(got.msg, "Failed to create output file") {
		t.Fatalf("unexpected hook panic: %+v", got)
	}
}
