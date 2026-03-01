package config

import (
	"math"
	"testing"
)

func TestParsePort_OverflowAndInvalidValues(t *testing.T) {
	maxInt := int(^uint(0) >> 1)

	tests := []struct {
		name    string
		input   any
		wantErr bool
	}{
		{name: "uint64 in range", input: uint64(maxInt), wantErr: false},
		{name: "uint64 overflow", input: uint64(maxInt) + 1, wantErr: true},
		{name: "float integer", input: float64(8443), wantErr: false},
		{name: "float fractional", input: 8443.5, wantErr: true},
		{name: "float inf", input: math.Inf(1), wantErr: true},
		{name: "float nan", input: math.NaN(), wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := parsePort(tt.input)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
