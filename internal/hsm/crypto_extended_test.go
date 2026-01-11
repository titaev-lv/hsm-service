package hsm

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"runtime"
	"sync"
	"testing"
)

// TestLargePayload tests encryption/decryption of large data (>1MB)
func TestLargePayload(t *testing.T) {
	hsm := createMockHSMContext(t)

	// Create 5MB payload
	largeData := make([]byte, 5*1024*1024)
	for i := range largeData {
		largeData[i] = byte(i % 256)
	}

	context := "exchange-key"
	clientCN := "trading-service-1"

	// Encrypt large payload
	ciphertext, err := hsm.Encrypt(largeData, context, clientCN, "test-key")
	if err != nil {
		t.Fatalf("Encrypt() large payload error = %v", err)
	}

	// Verify ciphertext is larger (nonce + tag overhead)
	expectedMinSize := len(largeData) + 12 + 16 // nonce + tag
	if len(ciphertext) < expectedMinSize {
		t.Errorf("Ciphertext size %d should be >= %d", len(ciphertext), expectedMinSize)
	}

	// Decrypt
	decrypted, err := hsm.Decrypt(ciphertext, context, clientCN, "test-key")
	if err != nil {
		t.Fatalf("Decrypt() large payload error = %v", err)
	}

	// Verify round-trip
	if !bytes.Equal(largeData, decrypted) {
		t.Error("Decrypted large payload doesn't match original")
	}
}

// TestMultipleKeyVersions tests encryption/decryption with multiple key versions
func TestMultipleKeyVersions(t *testing.T) {
	// Create mock HSM with multiple key versions
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

	hsm := &HSMContext{
		keys: map[string]cipher.AEAD{
			"exchange-key-v1": gcm1,
			"exchange-key-v2": gcm2,
		},
	}

	plaintext := []byte("secret data for version testing")
	context := "exchange-key"
	clientCN := "trading-service-1"

	// Encrypt with v1
	ciphertext1, err := hsm.Encrypt(plaintext, context, clientCN, "exchange-key-v1")
	if err != nil {
		t.Fatalf("Encrypt() v1 error = %v", err)
	}

	// Encrypt with v2
	ciphertext2, err := hsm.Encrypt(plaintext, context, clientCN, "exchange-key-v2")
	if err != nil {
		t.Fatalf("Encrypt() v2 error = %v", err)
	}

	// Ciphertexts should be different (different keys)
	if bytes.Equal(ciphertext1, ciphertext2) {
		t.Error("Ciphertexts encrypted with different keys should differ")
	}

	// Decrypt v1 ciphertext with v1 key
	decrypted1, err := hsm.Decrypt(ciphertext1, context, clientCN, "exchange-key-v1")
	if err != nil {
		t.Fatalf("Decrypt() v1 error = %v", err)
	}
	if !bytes.Equal(plaintext, decrypted1) {
		t.Error("V1 decryption failed")
	}

	// Decrypt v2 ciphertext with v2 key
	decrypted2, err := hsm.Decrypt(ciphertext2, context, clientCN, "exchange-key-v2")
	if err != nil {
		t.Fatalf("Decrypt() v2 error = %v", err)
	}
	if !bytes.Equal(plaintext, decrypted2) {
		t.Error("V2 decryption failed")
	}

	// Cross-decryption should fail (v1 ciphertext with v2 key)
	_, err = hsm.Decrypt(ciphertext1, context, clientCN, "exchange-key-v2")
	if err == nil {
		t.Error("Decrypting v1 ciphertext with v2 key should fail")
	}
}

// TestNonceUniqueness verifies that nonces are unique across encryptions
func TestNonceUniqueness(t *testing.T) {
	hsm := createMockHSMContext(t)

	plaintext := []byte("test data")
	context := "exchange-key"
	clientCN := "trading-service-1"

	nonces := make(map[string]bool)
	iterations := 10000

	for i := 0; i < iterations; i++ {
		ciphertext, err := hsm.Encrypt(plaintext, context, clientCN, "test-key")
		if err != nil {
			t.Fatalf("Encrypt() iteration %d error = %v", i, err)
		}

		// Extract nonce (first 12 bytes)
		nonce := ciphertext[:12]
		nonceStr := base64.StdEncoding.EncodeToString(nonce)

		if nonces[nonceStr] {
			t.Fatalf("Nonce collision detected at iteration %d: %s", i, nonceStr)
		}
		nonces[nonceStr] = true
	}

	t.Logf("Generated %d unique nonces out of %d encryptions", len(nonces), iterations)
}

// TestConcurrentEncryption tests thread safety of encryption operations
func TestConcurrentEncryption(t *testing.T) {
	hsm := createMockHSMContext(t)

	plaintext := []byte("concurrent test data")
	context := "exchange-key"
	clientCN := "trading-service-1"

	var wg sync.WaitGroup
	goroutines := 100
	iterations := 10

	errors := make(chan error, goroutines*iterations)

	for g := 0; g < goroutines; g++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			for i := 0; i < iterations; i++ {
				// Encrypt
				ciphertext, err := hsm.Encrypt(plaintext, context, clientCN, "test-key")
				if err != nil {
					errors <- err
					return
				}

				// Decrypt
				decrypted, err := hsm.Decrypt(ciphertext, context, clientCN, "test-key")
				if err != nil {
					errors <- err
					return
				}

				// Verify
				if !bytes.Equal(plaintext, decrypted) {
					errors <- err
					return
				}
			}
		}(g)
	}

	wg.Wait()
	close(errors)

	// Check for errors
	errorCount := 0
	for err := range errors {
		t.Errorf("Concurrent operation error: %v", err)
		errorCount++
	}

	if errorCount > 0 {
		t.Fatalf("Total errors: %d", errorCount)
	}

	t.Logf("Successfully completed %d concurrent encrypt/decrypt operations", goroutines*iterations)
}

// TestAADCollisionResistance ensures different context+CN combinations produce different AADs
func TestAADCollisionResistance(t *testing.T) {
	tests := []struct {
		name     string
		context1 string
		cn1      string
		context2 string
		cn2      string
	}{
		{
			name:     "separator ambiguity",
			context1: "exchange",
			cn1:      "key-admin",
			context2: "exchange-key",
			cn2:      "admin",
		},
		{
			name:     "similar strings",
			context1: "2fa",
			cn1:      "service-1",
			context2: "2fa-service",
			cn2:      "1",
		},
		{
			name:     "empty context",
			context1: "",
			cn1:      "client",
			context2: "client",
			cn2:      "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aad1 := BuildAAD(tt.context1, tt.cn1)
			aad2 := BuildAAD(tt.context2, tt.cn2)

			if bytes.Equal(aad1, aad2) {
				t.Errorf("AAD collision: context1=%q cn1=%q vs context2=%q cn2=%q",
					tt.context1, tt.cn1, tt.context2, tt.cn2)
			}
		})
	}
}

// TestClientCNMismatch ensures different client CNs produce different AADs
func TestClientCNMismatch(t *testing.T) {
	hsm := createMockHSMContext(t)

	plaintext := []byte("secret data")
	context := "exchange-key"
	clientCN1 := "trading-service-1"
	clientCN2 := "trading-service-2"

	// Encrypt with clientCN1
	ciphertext, err := hsm.Encrypt(plaintext, context, clientCN1, "test-key")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	// Try to decrypt with clientCN2 (should fail)
	_, err = hsm.Decrypt(ciphertext, context, clientCN2, "test-key")
	if err == nil {
		t.Error("Decrypt() with wrong clientCN should fail")
	}

	// Decrypt with correct clientCN (should succeed)
	decrypted, err := hsm.Decrypt(ciphertext, context, clientCN1, "test-key")
	if err != nil {
		t.Fatalf("Decrypt() with correct clientCN error = %v", err)
	}

	if !bytes.Equal(plaintext, decrypted) {
		t.Error("Decrypted data doesn't match original")
	}
}

// TestCorruptedCiphertext tests handling of corrupted ciphertext
func TestCorruptedCiphertext(t *testing.T) {
	hsm := createMockHSMContext(t)

	plaintext := []byte("secret data")
	context := "exchange-key"
	clientCN := "trading-service-1"

	// Encrypt
	ciphertext, err := hsm.Encrypt(plaintext, context, clientCN, "test-key")
	if err != nil {
		t.Fatalf("Encrypt() error = %v", err)
	}

	tests := []struct {
		name       string
		corruptFn  func([]byte) []byte
		expectFail bool
	}{
		{
			name: "flip bit in ciphertext",
			corruptFn: func(ct []byte) []byte {
				corrupted := make([]byte, len(ct))
				copy(corrupted, ct)
				// Flip a bit in the middle
				corrupted[len(ct)/2] ^= 0x01
				return corrupted
			},
			expectFail: true,
		},
		{
			name: "truncate ciphertext",
			corruptFn: func(ct []byte) []byte {
				return ct[:len(ct)-5]
			},
			expectFail: true,
		},
		{
			name: "append extra bytes",
			corruptFn: func(ct []byte) []byte {
				return append(ct, []byte("extra")...)
			},
			expectFail: true,
		},
		{
			name: "modify nonce",
			corruptFn: func(ct []byte) []byte {
				corrupted := make([]byte, len(ct))
				copy(corrupted, ct)
				corrupted[0] ^= 0xFF
				return corrupted
			},
			expectFail: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			corruptedCiphertext := tt.corruptFn(ciphertext)
			_, err := hsm.Decrypt(corruptedCiphertext, context, clientCN, "test-key")

			if tt.expectFail && err == nil {
				t.Error("Expected decryption to fail with corrupted ciphertext")
			}
			if !tt.expectFail && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

// TestMemoryUsageUnderLoad verifies no memory leaks under repeated operations
func TestMemoryUsageUnderLoad(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping memory test in short mode")
	}

	hsm := createMockHSMContext(t)

	plaintext := []byte("memory leak test data")
	context := "exchange-key"
	clientCN := "trading-service-1"

	// Get baseline memory
	runtime.GC()
	var m1 runtime.MemStats
	runtime.ReadMemStats(&m1)
	baseline := m1.Alloc

	// Perform many encrypt/decrypt operations
	iterations := 1000
	for i := 0; i < iterations; i++ {
		ciphertext, err := hsm.Encrypt(plaintext, context, clientCN, "test-key")
		if err != nil {
			t.Fatalf("Encrypt() error = %v", err)
		}

		_, err = hsm.Decrypt(ciphertext, context, clientCN, "test-key")
		if err != nil {
			t.Fatalf("Decrypt() error = %v", err)
		}
	}

	// Get final memory
	runtime.GC()
	var m2 runtime.MemStats
	runtime.ReadMemStats(&m2)
	final := m2.Alloc

	// Calculate memory increase safely
	var memIncrease uint64
	if final > baseline {
		memIncrease = final - baseline
	} else {
		// Memory decreased (GC worked well)
		memIncrease = 0
	}

	// Memory increase should be reasonable (< 10MB)
	maxIncrease := uint64(10 * 1024 * 1024) // 10MB

	if memIncrease > maxIncrease {
		t.Errorf("Memory increase too large: %d bytes (%.2f MB) after %d iterations",
			memIncrease, float64(memIncrease)/(1024*1024), iterations)
	}

	t.Logf("Memory increase: %.2f KB after %d iterations (baseline: %d, final: %d)",
		float64(memIncrease)/1024, iterations, baseline, final)
}

// BenchmarkEncryption benchmarks encryption performance
func BenchmarkEncryption(b *testing.B) {
	hsm := createMockHSMContext(&testing.T{})

	plaintext := []byte("benchmark test data with reasonable length for realistic testing")
	context := "exchange-key"
	clientCN := "trading-service-1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hsm.Encrypt(plaintext, context, clientCN, "test-key")
		if err != nil {
			b.Fatalf("Encrypt() error = %v", err)
		}
	}
}

// BenchmarkDecryption benchmarks decryption performance
func BenchmarkDecryption(b *testing.B) {
	hsm := createMockHSMContext(&testing.T{})

	plaintext := []byte("benchmark test data with reasonable length for realistic testing")
	context := "exchange-key"
	clientCN := "trading-service-1"

	ciphertext, _ := hsm.Encrypt(plaintext, context, clientCN, "test-key")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := hsm.Decrypt(ciphertext, context, clientCN, "test-key")
		if err != nil {
			b.Fatalf("Decrypt() error = %v", err)
		}
	}
}

// BenchmarkConcurrentEncryption benchmarks concurrent encryption performance
func BenchmarkConcurrentEncryption(b *testing.B) {
	hsm := createMockHSMContext(&testing.T{})

	plaintext := []byte("concurrent benchmark data")
	context := "exchange-key"
	clientCN := "trading-service-1"

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			_, err := hsm.Encrypt(plaintext, context, clientCN, "test-key")
			if err != nil {
				b.Fatalf("Encrypt() error = %v", err)
			}
		}
	})
}

// BenchmarkBuildAAD benchmarks AAD construction
func BenchmarkBuildAAD(b *testing.B) {
	context := "exchange-key"
	clientCN := "trading-service-1"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = BuildAAD(context, clientCN)
	}
}
