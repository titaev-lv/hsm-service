package config

import (
	"testing"
)

func TestParseSize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    int64
		wantErr bool
	}{
		// Valid plain bytes
		{"plain bytes", "1024", 1024, false},
		{"zero", "0", 0, false},

		// Valid kilobytes (k/K)
		{"lowercase k", "1k", 1024, false},
		{"uppercase K", "1K", 1024, false},
		{"512k", "512k", 512 * 1024, false},

		// Valid megabytes (m/M)
		{"lowercase m", "1m", 1024 * 1024, false},
		{"uppercase M", "1M", 1024 * 1024, false},
		{"4M", "4M", 4 * 1024 * 1024, false},
		{"100M max", "100M", 100 * 1024 * 1024, false},

		// Invalid cases
		{"negative", "-1", 0, true},
		{"negative k", "-10k", 0, true},
		{"exceeds 100MB", "101M", 0, true},
		{"invalid suffix", "10X", 0, true},
		{"empty string", "", 0, true},
		{"non-numeric", "abc", 0, true},
		{"multiple suffixes", "10kM", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseSize(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseSize(%q) error = %v, wantErr %v", tt.input, err, tt.wantErr)
				return
			}
			if !tt.wantErr && got != tt.want {
				t.Errorf("ParseSize(%q) = %d, want %d", tt.input, got, tt.want)
			}
		})
	}
}

func TestHTTP2Config_Parse(t *testing.T) {
	tests := []struct {
		name    string
		config  *HTTP2Config
		wantErr bool
		check   func(*testing.T, *ParsedHTTP2Config)
	}{
		{
			name: "valid configuration with human-readable sizes",
			config: &HTTP2Config{
				MaxConcurrentStreams:     "2000",
				InitialWindowSize:        "4M",
				MaxFrameSize:             "1M",
				MaxHeaderListSize:        "2M",
				IdleTimeoutSeconds:       120,
				MaxUploadBufferPerConn:   "4M",
				MaxUploadBufferPerStream: "4M",
			},
			wantErr: false,
			check: func(t *testing.T, p *ParsedHTTP2Config) {
				if p.MaxConcurrentStreams != 2000 {
					t.Errorf("MaxConcurrentStreams = %d, want 2000", p.MaxConcurrentStreams)
				}
				if p.InitialWindowSize != 4*1024*1024 {
					t.Errorf("InitialWindowSize = %d, want %d", p.InitialWindowSize, 4*1024*1024)
				}
				if p.MaxFrameSize != 1*1024*1024 {
					t.Errorf("MaxFrameSize = %d, want %d", p.MaxFrameSize, 1*1024*1024)
				}
				if p.IdleTimeoutSeconds != 120 {
					t.Errorf("IdleTimeoutSeconds = %d, want 120", p.IdleTimeoutSeconds)
				}
			},
		},
		{
			name: "valid with plain bytes",
			config: &HTTP2Config{
				MaxConcurrentStreams: "1000",
				InitialWindowSize:    "1048576", // 1MB in bytes
				MaxFrameSize:         "524288",  // 512KB in bytes
			},
			wantErr: false,
			check: func(t *testing.T, p *ParsedHTTP2Config) {
				if p.InitialWindowSize != 1048576 {
					t.Errorf("InitialWindowSize = %d, want 1048576", p.InitialWindowSize)
				}
			},
		},
		{
			name:    "empty config (all optional)",
			config:  &HTTP2Config{},
			wantErr: false,
			check: func(t *testing.T, p *ParsedHTTP2Config) {
				if p.MaxConcurrentStreams != 0 {
					t.Errorf("MaxConcurrentStreams = %d, want 0", p.MaxConcurrentStreams)
				}
			},
		},
		{
			name: "invalid max_concurrent_streams",
			config: &HTTP2Config{
				MaxConcurrentStreams: "invalid",
			},
			wantErr: true,
		},
		{
			name: "initial_window_size exceeds 100MB",
			config: &HTTP2Config{
				InitialWindowSize: "101M",
			},
			wantErr: true,
		},
		{
			name: "max_frame_size exceeds 100MB",
			config: &HTTP2Config{
				MaxFrameSize: "200M",
			},
			wantErr: true,
		},
		{
			name: "negative initial_window_size",
			config: &HTTP2Config{
				InitialWindowSize: "-1M",
			},
			wantErr: true,
		},
		{
			name: "invalid size format",
			config: &HTTP2Config{
				MaxHeaderListSize: "abc",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := tt.config.Parse()
			if (err != nil) != tt.wantErr {
				t.Errorf("HTTP2Config.Parse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && tt.check != nil {
				tt.check(t, parsed)
			}
		})
	}
}

func TestHTTP2Config_RealWorldScenarios(t *testing.T) {
	t.Run("aggressive settings for dedicated HSM machine", func(t *testing.T) {
		config := &HTTP2Config{
			MaxConcurrentStreams:     "2000",
			InitialWindowSize:        "4M",
			MaxFrameSize:             "1M",
			MaxHeaderListSize:        "2M",
			IdleTimeoutSeconds:       120,
			MaxUploadBufferPerConn:   "4M",
			MaxUploadBufferPerStream: "4M",
		}

		parsed, err := config.Parse()
		if err != nil {
			t.Fatalf("Failed to parse aggressive config: %v", err)
		}

		// Verify aggressive settings are correctly applied
		if parsed.MaxConcurrentStreams < 1000 {
			t.Errorf("MaxConcurrentStreams too low for aggressive config: %d", parsed.MaxConcurrentStreams)
		}
		if parsed.InitialWindowSize < 1*1024*1024 {
			t.Errorf("InitialWindowSize too low for aggressive config: %d", parsed.InitialWindowSize)
		}

		t.Logf("✓ Aggressive HTTP/2 config validated: MaxStreams=%d, InitWindow=%dMB",
			parsed.MaxConcurrentStreams, parsed.InitialWindowSize/(1024*1024))
	})

	t.Run("conservative settings for shared environment", func(t *testing.T) {
		config := &HTTP2Config{
			MaxConcurrentStreams: "250",
			InitialWindowSize:    "64k",
			MaxFrameSize:         "16k",
		}

		parsed, err := config.Parse()
		if err != nil {
			t.Fatalf("Failed to parse conservative config: %v", err)
		}

		if parsed.MaxConcurrentStreams != 250 {
			t.Errorf("MaxConcurrentStreams = %d, want 250", parsed.MaxConcurrentStreams)
		}
		if parsed.InitialWindowSize != 64*1024 {
			t.Errorf("InitialWindowSize = %d, want %d", parsed.InitialWindowSize, 64*1024)
		}

		t.Logf("✓ Conservative HTTP/2 config validated")
	})
}
