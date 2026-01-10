package hsm

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
)

func TestKeyManagerHotReload(t *testing.T) {
	// Skip if running without HSM (CI environment)
	if os.Getenv("SKIP_HSM_TESTS") != "" {
		t.Skip("Skipping HSM tests (no SoftHSM available)")
	}

	// This test requires a running SoftHSM with initialized token
	// and a valid metadata.yaml file
	// TODO: Add full integration test with mock SoftHSM
	t.Skip("Integration test - requires SoftHSM setup")
}

func TestKeyManagerLoadKeys(t *testing.T) {
	// Unit test for loadKeys validation logic
	tests := []struct {
		name        string
		metadata    *config.Metadata
		expectError bool
		errorMsg    string
	}{
		{
			name: "empty metadata",
			metadata: &config.Metadata{
				Rotation: map[string]config.KeyMetadata{},
			},
			expectError: true,
			errorMsg:    "no AES keys found",
		},
		{
			name: "missing current version",
			metadata: &config.Metadata{
				Rotation: map[string]config.KeyMetadata{
					"exchange-key": {
						Current: "kek-exchange-v1",
						Versions: []config.KeyVersion{
							{
								Label:   "kek-exchange-v2",
								Version: 2,
							},
						},
					},
				},
			},
			expectError: true,
			errorMsg:    "current KEK not loaded",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// This would require a mock crypto11.Context
			// TODO: Implement when adding full unit test coverage
			t.Skip("Requires mock PKCS#11 context")
		})
	}
}

func TestKeyManagerAutoReload(t *testing.T) {
	// Test that auto-reload detects file changes
	if os.Getenv("SKIP_HSM_TESTS") != "" {
		t.Skip("Skipping HSM tests (no SoftHSM available)")
	}

	t.Skip("Integration test - requires full setup")
}

func TestKeyManagerGracefulShutdown(t *testing.T) {
	// Test that StopAutoReload() completes within timeout
	// This test can work without HSM since it's testing goroutine management

	// Create a minimal KeyManager (without PKCS#11 context)
	km := &KeyManager{
		stopReload: make(chan struct{}),
	}

	// Start auto-reload
	km.reloadWg.Add(1)
	go func() {
		defer km.reloadWg.Done()
		<-km.stopReload
	}()

	// Stop with timeout
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	err := km.StopAutoReload(ctx)
	if err != nil {
		t.Fatalf("StopAutoReload failed: %v", err)
	}
}

func TestKeyManagerThreadSafety(t *testing.T) {
	// Test concurrent access to keys map
	km := &KeyManager{
		contextToLabel: make(map[string]string),
		metadata:       make(map[string]*KeyMetadata),
	}

	// Simulate concurrent reads and writes
	done := make(chan bool)

	// Writer goroutine
	go func() {
		for i := 0; i < 100; i++ {
			km.mu.Lock()
			km.contextToLabel["test"] = "kek-test-v1"
			km.mu.Unlock()
			time.Sleep(1 * time.Millisecond)
		}
		done <- true
	}()

	// Reader goroutines
	for i := 0; i < 3; i++ {
		go func() {
			for j := 0; j < 100; j++ {
				km.mu.RLock()
				_ = km.contextToLabel["test"]
				km.mu.RUnlock()
				time.Sleep(1 * time.Millisecond)
			}
			done <- true
		}()
	}

	// Wait for all goroutines
	for i := 0; i < 4; i++ {
		<-done
	}

	// If we reach here without race detector errors, test passes
}
