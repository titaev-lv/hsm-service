package hsm

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestKeyManagerMetadataChanged_MissingFileReturnsFalse(t *testing.T) {
	km := &KeyManager{metadataFile: filepath.Join(t.TempDir(), "missing.yaml")}
	if km.metadataChanged() {
		t.Fatal("metadataChanged() = true for missing file, want false")
	}
}

func TestKeyManagerMetadataChanged_DetectsUpdateAndThenFalse(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "metadata.yaml")
	if err := os.WriteFile(path, []byte("rotation: {}\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile: %v", err)
	}

	km := &KeyManager{metadataFile: path}
	if !km.metadataChanged() {
		t.Fatal("metadataChanged() = false on first seen file, want true")
	}
	if km.metadataChanged() {
		t.Fatal("metadataChanged() = true without file modification, want false")
	}

	time.Sleep(10 * time.Millisecond)
	if err := os.WriteFile(path, []byte("rotation:\n  test: {}\n"), 0o600); err != nil {
		t.Fatalf("os.WriteFile(update): %v", err)
	}
	if !km.metadataChanged() {
		t.Fatal("metadataChanged() = false after file update, want true")
	}
}

func TestKeyManagerStopAutoReload_Success(t *testing.T) {
	km := &KeyManager{stopReload: make(chan struct{})}
	km.reloadWg.Add(1)
	go func() {
		defer km.reloadWg.Done()
		<-km.stopReload
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	if err := km.StopAutoReload(ctx); err != nil {
		t.Fatalf("StopAutoReload() error: %v", err)
	}
}

func TestKeyManagerStopAutoReload_ContextTimeout(t *testing.T) {
	km := &KeyManager{stopReload: make(chan struct{})}
	km.reloadWg.Add(1)
	go func() {
		defer km.reloadWg.Done()
		// Simulate stuck goroutine that does not listen on stopReload.
		time.Sleep(200 * time.Millisecond)
	}()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	err := km.StopAutoReload(ctx)
	if err == nil {
		t.Fatal("StopAutoReload() expected deadline error")
	}
	if err != context.DeadlineExceeded {
		t.Fatalf("StopAutoReload() error = %v, want %v", err, context.DeadlineExceeded)
	}
}

func TestKeyManagerClose_NilContextNoError(t *testing.T) {
	km := &KeyManager{}
	if err := km.Close(); err != nil {
		t.Fatalf("Close() error: %v", err)
	}
}
