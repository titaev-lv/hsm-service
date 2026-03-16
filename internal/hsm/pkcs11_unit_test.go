package hsm

import (
	"crypto/cipher"
	"sort"
	"testing"
	"time"
)

func TestComputeKeyChecksum_DeterministicAndLabelBound(t *testing.T) {
	s1 := computeKeyChecksum("kek-a", nil)
	s2 := computeKeyChecksum("kek-a", nil)
	s3 := computeKeyChecksum("kek-b", nil)

	if s1 == "" {
		t.Fatal("computeKeyChecksum() returned empty checksum")
	}
	if s1 != s2 {
		t.Fatalf("computeKeyChecksum() is not deterministic: %q vs %q", s1, s2)
	}
	if s1 == s3 {
		t.Fatalf("computeKeyChecksum() should differ for different labels: %q vs %q", s1, s3)
	}
}

func TestHSMContextCloseAndGetContext_NilSafe(t *testing.T) {
	obj := &HSMContext{}
	if err := obj.Close(); err != nil {
		t.Fatalf("Close() with nil ctx error: %v", err)
	}
	if got := obj.GetContext(); got != nil {
		t.Fatalf("GetContext() = %v, want nil", got)
	}
}

func TestHSMContextGettersAndRotationList(t *testing.T) {
	old := time.Now().Add(-120 * 24 * time.Hour)
	recent := time.Now().Add(-10 * 24 * time.Hour)

	obj := &HSMContext{
		keys: map[string]cipher.AEAD{},
		contextToLabel: map[string]string{
			"exchange-key": "kek-exchange-v1",
		},
		metadata: map[string]*KeyMetadata{
			"kek-exchange-v1": {
				Label:            "kek-exchange-v1",
				Version:          1,
				CreatedAt:        old,
				RotationInterval: 90 * 24 * time.Hour,
			},
			"kek-exchange-v2": {
				Label:            "kek-exchange-v2",
				Version:          2,
				CreatedAt:        recent,
				RotationInterval: 90 * 24 * time.Hour,
			},
		},
	}

	labels := obj.GetKeyLabels()
	sort.Strings(labels)
	if len(labels) != 0 {
		t.Fatalf("GetKeyLabels() on empty keys map = %v, want []", labels)
	}
	if obj.HasKey("kek-exchange-v1") {
		t.Fatal("HasKey() should be false when keys map is empty")
	}

	label, err := obj.GetKeyLabelByContext("exchange-key")
	if err != nil {
		t.Fatalf("GetKeyLabelByContext() error: %v", err)
	}
	if label != "kek-exchange-v1" {
		t.Fatalf("GetKeyLabelByContext() = %q, want %q", label, "kek-exchange-v1")
	}
	if _, err := obj.GetKeyLabelByContext("missing"); err == nil {
		t.Fatal("GetKeyLabelByContext() expected error for missing context")
	}

	meta, err := obj.GetKeyMetadata("kek-exchange-v1")
	if err != nil {
		t.Fatalf("GetKeyMetadata() error: %v", err)
	}
	if meta.Version != 1 {
		t.Fatalf("GetKeyMetadata().Version = %d, want 1", meta.Version)
	}
	if _, err := obj.GetKeyMetadata("missing"); err == nil {
		t.Fatal("GetKeyMetadata() expected error for missing label")
	}

	needs := obj.GetKeysNeedingRotation()
	sort.Strings(needs)
	if len(needs) != 1 || needs[0] != "kek-exchange-v1" {
		t.Fatalf("GetKeysNeedingRotation() = %v, want [kek-exchange-v1]", needs)
	}
}
