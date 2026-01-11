package hsm

import (
	"testing"
	"time"
)

// TestNeedsRotation tests the rotation logic
func TestNeedsRotation(t *testing.T) {
	tests := []struct {
		name             string
		createdAt        time.Time
		rotationInterval time.Duration
		expected         bool
	}{
		{
			name:             "needs rotation - 100 days old, 90 day interval",
			createdAt:        time.Now().Add(-100 * 24 * time.Hour),
			rotationInterval: 90 * 24 * time.Hour,
			expected:         true,
		},
		{
			name:             "does not need rotation - 80 days old, 90 day interval",
			createdAt:        time.Now().Add(-80 * 24 * time.Hour),
			rotationInterval: 90 * 24 * time.Hour,
			expected:         false,
		},
		{
			name:             "exactly at threshold - 90 days old, 90 day interval",
			createdAt:        time.Now().Add(-90 * 24 * time.Hour),
			rotationInterval: 90 * 24 * time.Hour,
			expected:         true, // After() returns true when time is after threshold
		},
		{
			name:             "just over threshold - 91 days old, 90 day interval",
			createdAt:        time.Now().Add(-91 * 24 * time.Hour),
			rotationInterval: 90 * 24 * time.Hour,
			expected:         true,
		},
		{
			name:             "brand new key - 1 day old",
			createdAt:        time.Now().Add(-24 * time.Hour),
			rotationInterval: 90 * 24 * time.Hour,
			expected:         false,
		},
		{
			name:             "zero interval - never rotate",
			createdAt:        time.Now().Add(-365 * 24 * time.Hour),
			rotationInterval: 0,
			expected:         false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &KeyMetadata{
				Label:            "test-key",
				Version:          1,
				CreatedAt:        tt.createdAt,
				RotationInterval: tt.rotationInterval,
			}

			result := meta.NeedsRotation()
			if result != tt.expected {
				t.Errorf("NeedsRotation() = %v, want %v (age: %v, interval: %v)",
					result, tt.expected,
					time.Since(tt.createdAt),
					tt.rotationInterval)
			}
		})
	}
}

// TestKeyMetadataAge tests age calculation
func TestKeyMetadataAge(t *testing.T) {
	now := time.Now()

	tests := []struct {
		name      string
		createdAt time.Time
		minAge    time.Duration
		maxAge    time.Duration
	}{
		{
			name:      "1 day old",
			createdAt: now.Add(-24 * time.Hour),
			minAge:    23 * time.Hour,
			maxAge:    25 * time.Hour,
		},
		{
			name:      "30 days old",
			createdAt: now.Add(-30 * 24 * time.Hour),
			minAge:    29 * 24 * time.Hour,
			maxAge:    31 * 24 * time.Hour,
		},
		{
			name:      "just created",
			createdAt: now,
			minAge:    0,
			maxAge:    1 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			meta := &KeyMetadata{
				Label:     "test-key",
				CreatedAt: tt.createdAt,
			}

			age := time.Since(meta.CreatedAt)

			if age < tt.minAge || age > tt.maxAge {
				t.Errorf("Key age %v not in expected range [%v, %v]",
					age, tt.minAge, tt.maxAge)
			}
		})
	}
}

// TestKeyMetadataRotationBoundary tests edge cases around rotation boundary
func TestKeyMetadataRotationBoundary(t *testing.T) {
	interval := 90 * 24 * time.Hour

	// Test multiple points around the boundary
	testPoints := []struct {
		daysOld  int
		expected bool
	}{
		{89, false}, // Just before
		{90, true},  // Exactly at threshold (After() returns true)
		{91, true},  // Just after
		{92, true},  // After
		{100, true}, // Well after
	}

	for _, tp := range testPoints {
		t.Run(string(rune(tp.daysOld)), func(t *testing.T) {
			meta := &KeyMetadata{
				Label:            "test-key",
				CreatedAt:        time.Now().Add(-time.Duration(tp.daysOld) * 24 * time.Hour),
				RotationInterval: interval,
			}

			result := meta.NeedsRotation()
			if result != tp.expected {
				t.Errorf("Key %d days old: NeedsRotation() = %v, want %v",
					tp.daysOld, result, tp.expected)
			}
		})
	}
}

// TestMultipleKeyVersionsRotationStatus tests multiple key versions
func TestMultipleKeyVersionsRotationStatus(t *testing.T) {
	now := time.Now()
	interval := 90 * 24 * time.Hour

	keys := []*KeyMetadata{
		{
			Label:            "key-v1",
			Version:          1,
			CreatedAt:        now.Add(-200 * 24 * time.Hour), // 200 days old
			RotationInterval: interval,
		},
		{
			Label:            "key-v2",
			Version:          2,
			CreatedAt:        now.Add(-100 * 24 * time.Hour), // 100 days old
			RotationInterval: interval,
		},
		{
			Label:            "key-v3",
			Version:          3,
			CreatedAt:        now.Add(-50 * 24 * time.Hour), // 50 days old
			RotationInterval: interval,
		},
		{
			Label:            "key-v4",
			Version:          4,
			CreatedAt:        now.Add(-10 * 24 * time.Hour), // 10 days old
			RotationInterval: interval,
		},
	}

	expected := []bool{true, true, false, false}

	for i, key := range keys {
		result := key.NeedsRotation()
		if result != expected[i] {
			t.Errorf("%s (age: %.0f days): NeedsRotation() = %v, want %v",
				key.Label,
				time.Since(key.CreatedAt).Hours()/24,
				result,
				expected[i])
		}
	}
}

// TestNeedsRotationWithZeroInterval tests that zero interval means never rotate
func TestNeedsRotationWithZeroInterval(t *testing.T) {
	// Even very old keys with zero interval should not need rotation
	ages := []time.Duration{
		1 * 24 * time.Hour,
		90 * 24 * time.Hour,
		365 * 24 * time.Hour,
		1000 * 24 * time.Hour,
	}

	for _, age := range ages {
		t.Run(age.String(), func(t *testing.T) {
			meta := &KeyMetadata{
				Label:            "permanent-key",
				CreatedAt:        time.Now().Add(-age),
				RotationInterval: 0,
			}

			if meta.NeedsRotation() {
				t.Errorf("Key with zero interval should never need rotation (age: %v)", age)
			}
		})
	}
}

// BenchmarkNeedsRotation benchmarks rotation check performance
func BenchmarkNeedsRotation(b *testing.B) {
	meta := &KeyMetadata{
		Label:            "bench-key",
		CreatedAt:        time.Now().Add(-100 * 24 * time.Hour),
		RotationInterval: 90 * 24 * time.Hour,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = meta.NeedsRotation()
	}
}
