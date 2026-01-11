package hsm

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"sync"
	"testing"
	"time"
)

// mockKeyManager creates a mock KeyManager for testing without real HSM
func mockKeyManager(t *testing.T) *KeyManager {
	// Create test AES key
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		t.Fatalf("Failed to create AES cipher: %v", err)
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		t.Fatalf("Failed to create GCM: %v", err)
	}

	now := time.Now()
	km := &KeyManager{
		ctx: nil, // No real PKCS#11 context
		keys: map[string]cipher.AEAD{
			"exchange-key-v1": gcm,
		},
		contextToLabel: map[string]string{
			"exchange-key": "exchange-key-v1",
		},
		metadata: map[string]*KeyMetadata{
			"exchange-key-v1": {
				Label:            "exchange-key-v1",
				Version:          1,
				CreatedAt:        now,
				RotationInterval: 90 * 24 * time.Hour,
			},
		},
		stopReload: make(chan struct{}),
	}

	return km
}

// TestKeyManagerEncrypt tests encryption through KeyManager
func TestKeyManagerEncrypt(t *testing.T) {
	km := mockKeyManager(t)

	plaintext := []byte("test data for key manager")
	context := "exchange-key"
	clientCN := "trading-service-1"

	// Encrypt
	ciphertext, keyLabel, err := km.Encrypt(plaintext, context, clientCN)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	if keyLabel != "exchange-key-v1" {
		t.Errorf("Expected key label 'exchange-key-v1', got '%s'", keyLabel)
	}

	if len(ciphertext) == 0 {
		t.Error("Ciphertext should not be empty")
	}

	// Verify ciphertext is longer than plaintext
	if len(ciphertext) <= len(plaintext) {
		t.Errorf("Ciphertext length %d should be > plaintext length %d",
			len(ciphertext), len(plaintext))
	}
}

// TestKeyManagerDecrypt tests decryption through KeyManager
func TestKeyManagerDecrypt(t *testing.T) {
	km := mockKeyManager(t)

	plaintext := []byte("test data for decryption")
	context := "exchange-key"
	clientCN := "trading-service-1"

	// Encrypt first
	ciphertext, keyLabel, err := km.Encrypt(plaintext, context, clientCN)
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Decrypt
	decrypted, err := km.Decrypt(ciphertext, context, clientCN, keyLabel)
	if err != nil {
		t.Fatalf("Decrypt() error = %v", err)
	}

	// Verify round-trip
	if !bytes.Equal(plaintext, decrypted) {
		t.Errorf("Decrypted data doesn't match.\nWant: %s\nGot:  %s",
			plaintext, decrypted)
	}
}

// TestKeyManagerEncryptInvalidContext tests encryption with non-existent context
func TestKeyManagerEncryptInvalidContext(t *testing.T) {
	km := mockKeyManager(t)

	plaintext := []byte("test data")
	context := "nonexistent-context"
	clientCN := "trading-service-1"

	_, _, err := km.Encrypt(plaintext, context, clientCN)
	if err == nil {
		t.Error("Encrypt() with invalid context should fail")
	}

	expectedMsg := "no key configured for context"
	if err != nil && !bytes.Contains([]byte(err.Error()), []byte(expectedMsg)) {
		t.Errorf("Expected error containing '%s', got: %v", expectedMsg, err)
	}
}

// TestKeyManagerDecryptInvalidKey tests decryption with non-existent key
func TestKeyManagerDecryptInvalidKey(t *testing.T) {
	km := mockKeyManager(t)

	ciphertext := []byte("dummy ciphertext data here")
	context := "exchange-key"
	clientCN := "trading-service-1"
	keyLabel := "nonexistent-key"

	_, err := km.Decrypt(ciphertext, context, clientCN, keyLabel)
	if err == nil {
		t.Error("Decrypt() with invalid key should fail")
	}

	if err != nil && err != ErrKeyNotFound {
		// Check if error message contains "key not found"
		if !bytes.Contains([]byte(err.Error()), []byte("key not found")) {
			t.Errorf("Expected ErrKeyNotFound or similar, got: %v", err)
		}
	}
}

// TestKeyManagerGetKeyLabels tests retrieving all key labels
func TestKeyManagerGetKeyLabels(t *testing.T) {
	km := mockKeyManager(t)

	labels := km.GetKeyLabels()

	if len(labels) != 1 {
		t.Errorf("Expected 1 key label, got %d", len(labels))
	}

	if len(labels) > 0 && labels[0] != "exchange-key-v1" {
		t.Errorf("Expected label 'exchange-key-v1', got '%s'", labels[0])
	}
}

// TestKeyManagerHasKey tests key existence check
func TestKeyManagerHasKey(t *testing.T) {
	km := mockKeyManager(t)

	tests := []struct {
		label  string
		exists bool
	}{
		{"exchange-key-v1", true},
		{"nonexistent-key", false},
		{"", false},
	}

	for _, tt := range tests {
		t.Run(tt.label, func(t *testing.T) {
			result := km.HasKey(tt.label)
			if result != tt.exists {
				t.Errorf("HasKey(%q) = %v, want %v", tt.label, result, tt.exists)
			}
		})
	}
}

// TestKeyManagerGetKeyLabelByContext tests context to label mapping
func TestKeyManagerGetKeyLabelByContext(t *testing.T) {
	km := mockKeyManager(t)

	tests := []struct {
		context     string
		expectLabel string
		expectError bool
	}{
		{"exchange-key", "exchange-key-v1", false},
		{"nonexistent", "", true},
	}

	for _, tt := range tests {
		t.Run(tt.context, func(t *testing.T) {
			label, err := km.GetKeyLabelByContext(tt.context)

			if tt.expectError && err == nil {
				t.Error("Expected error but got none")
			}

			if !tt.expectError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}

			if !tt.expectError && label != tt.expectLabel {
				t.Errorf("GetKeyLabelByContext() = %q, want %q", label, tt.expectLabel)
			}
		})
	}
}

// TestKeyManagerGetKeyMetadata tests metadata retrieval
func TestKeyManagerGetKeyMetadata(t *testing.T) {
	km := mockKeyManager(t)

	// Test existing key
	meta, err := km.GetKeyMetadata("exchange-key-v1")
	if err != nil {
		t.Fatalf("GetKeyMetadata() error = %v", err)
	}

	if meta.Label != "exchange-key-v1" {
		t.Errorf("Expected label 'exchange-key-v1', got '%s'", meta.Label)
	}

	if meta.Version != 1 {
		t.Errorf("Expected version 1, got %d", meta.Version)
	}

	// Test non-existent key
	_, err = km.GetKeyMetadata("nonexistent-key")
	if err == nil {
		t.Error("Expected error for non-existent key")
	}
}

// TestKeyManagerConcurrentAccess tests thread safety
func TestKeyManagerConcurrentAccess(t *testing.T) {
	km := mockKeyManager(t)

	var wg sync.WaitGroup
	goroutines := 50
	iterations := 20

	errors := make(chan error, goroutines*iterations*3)

	plaintext := []byte("concurrent access test")
	context := "exchange-key"
	clientCN := "trading-service-1"

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for i := 0; i < iterations; i++ {
				// Encrypt
				ciphertext, keyLabel, err := km.Encrypt(plaintext, context, clientCN)
				if err != nil {
					errors <- err
					continue
				}

				// Decrypt
				decrypted, err := km.Decrypt(ciphertext, context, clientCN, keyLabel)
				if err != nil {
					errors <- err
					continue
				}

				// Verify
				if !bytes.Equal(plaintext, decrypted) {
					errors <- err
					continue
				}

				// Read operations (should not block writers)
				_ = km.GetKeyLabels()
				_ = km.HasKey(keyLabel)
				_, _ = km.GetKeyLabelByContext(context)
				_, _ = km.GetKeyMetadata(keyLabel)
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Total concurrent errors: %d", errorCount)
	}

	t.Logf("Successfully completed %d concurrent operations across %d goroutines",
		goroutines*iterations, goroutines)
}

// TestKeyManagerMultipleContexts tests multiple context support
func TestKeyManagerMultipleContexts(t *testing.T) {
	// Create keys for different contexts
	key1 := make([]byte, 32)
	key2 := make([]byte, 32)
	for i := range key1 {
		key1[i] = byte(i)
		key2[i] = byte(i + 1)
	}

	block1, _ := aes.NewCipher(key1)
	block2, _ := aes.NewCipher(key2)
	gcm1, _ := cipher.NewGCM(block1)
	gcm2, _ := cipher.NewGCM(block2)

	now := time.Now()
	km := &KeyManager{
		keys: map[string]cipher.AEAD{
			"exchange-key-v1": gcm1,
			"2fa-key-v1":      gcm2,
		},
		contextToLabel: map[string]string{
			"exchange-key": "exchange-key-v1",
			"2fa":          "2fa-key-v1",
		},
		metadata: map[string]*KeyMetadata{
			"exchange-key-v1": {
				Label:     "exchange-key-v1",
				Version:   1,
				CreatedAt: now,
			},
			"2fa-key-v1": {
				Label:     "2fa-key-v1",
				Version:   1,
				CreatedAt: now,
			},
		},
		stopReload: make(chan struct{}),
	}

	plaintext := []byte("multi-context test")
	clientCN := "service-1"

	// Encrypt with exchange-key context
	ct1, label1, err := km.Encrypt(plaintext, "exchange-key", clientCN)
	if err != nil {
		t.Fatalf("Encrypt() exchange-key error = %v", err)
	}

	// Encrypt with 2fa context
	ct2, label2, err := km.Encrypt(plaintext, "2fa", clientCN)
	if err != nil {
		t.Fatalf("Encrypt() 2fa error = %v", err)
	}

	// Labels should be different
	if label1 == label2 {
		t.Error("Different contexts should use different keys")
	}

	// Ciphertexts should be different (different keys + different AAD)
	if bytes.Equal(ct1, ct2) {
		t.Error("Ciphertexts from different contexts should differ")
	}

	// Decrypt with correct labels
	pt1, err := km.Decrypt(ct1, "exchange-key", clientCN, label1)
	if err != nil {
		t.Fatalf("Decrypt() exchange-key error = %v", err)
	}

	pt2, err := km.Decrypt(ct2, "2fa", clientCN, label2)
	if err != nil {
		t.Fatalf("Decrypt() 2fa error = %v", err)
	}

	// Verify round-trips
	if !bytes.Equal(plaintext, pt1) || !bytes.Equal(plaintext, pt2) {
		t.Error("Decryption failed for one or both contexts")
	}

	// Cross-context decryption should fail (wrong AAD)
	_, err = km.Decrypt(ct1, "2fa", clientCN, label1)
	if err == nil {
		t.Error("Cross-context decryption should fail")
	}
}

// TestKeyManagerGetKeysNeedingRotation tests rotation detection
func TestKeyManagerGetKeysNeedingRotation(t *testing.T) {
	key := make([]byte, 32)
	for i := range key {
		key[i] = byte(i)
	}

	block, _ := aes.NewCipher(key)
	gcm, _ := cipher.NewGCM(block)

	// Create key that needs rotation (created 100 days ago, 90 day interval)
	oldTime := time.Now().Add(-100 * 24 * time.Hour)
	recentTime := time.Now().Add(-10 * 24 * time.Hour)

	km := &KeyManager{
		keys: map[string]cipher.AEAD{
			"old-key":    gcm,
			"recent-key": gcm,
		},
		contextToLabel: map[string]string{
			"context1": "old-key",
			"context2": "recent-key",
		},
		metadata: map[string]*KeyMetadata{
			"old-key": {
				Label:            "old-key",
				Version:          1,
				CreatedAt:        oldTime,
				RotationInterval: 90 * 24 * time.Hour,
			},
			"recent-key": {
				Label:            "recent-key",
				Version:          1,
				CreatedAt:        recentTime,
				RotationInterval: 90 * 24 * time.Hour,
			},
		},
		stopReload: make(chan struct{}),
	}

	needsRotation := km.GetKeysNeedingRotation()

	// old-key should need rotation
	found := false
	for _, label := range needsRotation {
		if label == "old-key" {
			found = true
			break
		}
	}

	if !found {
		t.Error("old-key should need rotation but wasn't in the list")
	}

	t.Logf("Keys needing rotation: %v", needsRotation)
}

// TestKeyManagerEmptyPlaintext tests encryption of empty data
func TestKeyManagerEmptyPlaintext(t *testing.T) {
	km := mockKeyManager(t)

	plaintext := []byte("")
	context := "exchange-key"
	clientCN := "trading-service-1"

	// Encrypt empty plaintext
	ciphertext, keyLabel, err := km.Encrypt(plaintext, context, clientCN)
	if err != nil {
		t.Fatalf("Encrypt() empty plaintext error = %v", err)
	}

	// Decrypt
	decrypted, err := km.Decrypt(ciphertext, context, clientCN, keyLabel)
	if err != nil {
		t.Fatalf("Decrypt() empty plaintext error = %v", err)
	}

	// Verify
	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Empty plaintext round-trip failed")
	}
}

// TestKeyManagerVeryLargePayload tests encryption of large data
func TestKeyManagerVeryLargePayload(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping large payload test in short mode")
	}

	km := mockKeyManager(t)

	// Create 10MB payload
	largeData := make([]byte, 10*1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	context := "exchange-key"
	clientCN := "trading-service-1"

	// Encrypt
	ciphertext, keyLabel, err := km.Encrypt(largeData, context, clientCN)
	if err != nil {
		t.Fatalf("Encrypt() large payload error = %v", err)
	}

	// Decrypt
	decrypted, err := km.Decrypt(ciphertext, context, clientCN, keyLabel)
	if err != nil {
		t.Fatalf("Decrypt() large payload error = %v", err)
	}

	// Verify
	if !bytes.Equal(largeData, decrypted) {
		t.Error("Large payload round-trip failed")
	}

	t.Logf("Successfully encrypted/decrypted %d MB payload",
		len(largeData)/(1024*1024))
}

// BenchmarkKeyManagerEncrypt benchmarks KeyManager encryption
func BenchmarkKeyManagerEncrypt(b *testing.B) {
	km := mockKeyManager(&testing.T{})

	plaintext := []byte("benchmark data for key manager encryption testing with realistic length")
	context := "exchange-key"
	clientCN := "trading-service-1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, err := km.Encrypt(plaintext, context, clientCN)
		if err != nil {
			b.Fatalf("Encrypt() error = %v", err)
		}
	}
}

// BenchmarkKeyManagerDecrypt benchmarks KeyManager decryption
func BenchmarkKeyManagerDecrypt(b *testing.B) {
	km := mockKeyManager(&testing.T{})

	plaintext := []byte("benchmark data for key manager decryption testing with realistic length")
	context := "exchange-key"
	clientCN := "trading-service-1"

	ciphertext, keyLabel, _ := km.Encrypt(plaintext, context, clientCN)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := km.Decrypt(ciphertext, context, clientCN, keyLabel)
		if err != nil {
			b.Fatalf("Decrypt() error = %v", err)
		}
	}
}

// BenchmarkKeyManagerConcurrent benchmarks concurrent access
func BenchmarkKeyManagerConcurrent(b *testing.B) {
	km := mockKeyManager(&testing.T{})

	plaintext := []byte("concurrent benchmark data")
	context := "exchange-key"
	clientCN := "trading-service-1"

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			ciphertext, keyLabel, err := km.Encrypt(plaintext, context, clientCN)
			if err != nil {
				b.Fatalf("Encrypt() error = %v", err)
			}

			_, err = km.Decrypt(ciphertext, context, clientCN, keyLabel)
			if err != nil {
				b.Fatalf("Decrypt() error = %v", err)
			}
		}
	})
}
