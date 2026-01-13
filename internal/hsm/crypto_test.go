package hsm

import (
	"bytes"
	"testing"
)

func TestBuildAAD(t *testing.T) {
	tests := []struct {
		name     string
		context  string
		ou       string
		clientCN string
		mode     string
	}{
		{
			name:     "private mode",
			context:  "exchange-key",
			ou:       "Trading",
			clientCN: "trading-service-1",
			mode:     "private",
		},
		{
			name:     "shared mode",
			context:  "exchange-key",
			ou:       "Trading",
			clientCN: "trading-service-1",
			mode:     "shared",
		},
		{
			name:     "2fa private",
			context:  "2fa",
			ou:       "2FA",
			clientCN: "web-2fa-service",
			mode:     "private",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aad := BuildAAD(tt.context, tt.ou, tt.clientCN, tt.mode)
			// AAD should be 32 bytes (SHA-256 hash)
			if len(aad) != 32 {
				t.Errorf("BuildAAD() length = %d, want 32", len(aad))
			}
			// AAD should be deterministic
			aad2 := BuildAAD(tt.context, tt.ou, tt.clientCN, tt.mode)
			if !bytes.Equal(aad, aad2) {
				t.Error("BuildAAD() not deterministic")
			}
		})
	}

	// Test that shared mode produces same AAD for different clients in same OU
	t.Run("shared mode same OU", func(t *testing.T) {
		aad1 := BuildAAD("exchange-key", "Trading", "trading-service-1", "shared")
		aad2 := BuildAAD("exchange-key", "Trading", "trading-service-2", "shared")
		if !bytes.Equal(aad1, aad2) {
			t.Error("Shared mode should produce same AAD for same OU")
		}
	})

	// Test that private mode produces different AAD for different clients
	t.Run("private mode different clients", func(t *testing.T) {
		aad1 := BuildAAD("exchange-key", "Trading", "trading-service-1", "private")
		aad2 := BuildAAD("exchange-key", "Trading", "trading-service-2", "private")
		if bytes.Equal(aad1, aad2) {
			t.Error("Private mode should produce different AAD for different clients")
		}
	})
}
