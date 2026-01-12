package config

import (
	"fmt"
	"strconv"
	"strings"
)

// ParseSize parses human-readable size strings (e.g., "1M", "512k", "1024")
// Supports: k/K (kilobytes), m/M (megabytes), or plain bytes
// Returns size in bytes
func ParseSize(s string) (int64, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return 0, fmt.Errorf("empty size string")
	}

	// Check for suffix
	lastChar := s[len(s)-1]
	var multiplier int64 = 1

	switch lastChar {
	case 'k', 'K':
		multiplier = 1024
		s = s[:len(s)-1]
	case 'm', 'M':
		multiplier = 1024 * 1024
		s = s[:len(s)-1]
	}

	// Parse numeric part
	value, err := strconv.ParseInt(strings.TrimSpace(s), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size format: %w", err)
	}

	if value < 0 {
		return 0, fmt.Errorf("size cannot be negative")
	}

	result := value * multiplier

	// Sanity check: max 100MB for HTTP/2 settings
	if result > 100*1024*1024 {
		return 0, fmt.Errorf("size too large: %d bytes (max 100M)", result)
	}

	return result, nil
}

// HTTP2Config defines HTTP/2 server configuration
// All size fields accept human-readable formats: "1M", "512k", "65536"
type HTTP2Config struct {
	MaxConcurrentStreams     string `yaml:"max_concurrent_streams"`       // e.g., "1000"
	InitialWindowSize        string `yaml:"initial_window_size"`          // e.g., "2M", "2097152"
	MaxFrameSize             string `yaml:"max_frame_size"`               // e.g., "1M", "1048576"
	MaxHeaderListSize        string `yaml:"max_header_list_size"`         // e.g., "1M"
	IdleTimeoutSeconds       int    `yaml:"idle_timeout_seconds"`         // e.g., 120
	MaxUploadBufferPerConn   string `yaml:"max_upload_buffer_per_conn"`   // e.g., "2M"
	MaxUploadBufferPerStream string `yaml:"max_upload_buffer_per_stream"` // e.g., "2M"
}

// ParsedHTTP2Config contains parsed integer values
type ParsedHTTP2Config struct {
	MaxConcurrentStreams     uint32
	InitialWindowSize        int32
	MaxFrameSize             uint32
	MaxHeaderListSize        uint32
	IdleTimeoutSeconds       int
	MaxUploadBufferPerConn   int32
	MaxUploadBufferPerStream int32
}

// Parse converts string-based config to actual integers
func (h *HTTP2Config) Parse() (*ParsedHTTP2Config, error) {
	parsed := &ParsedHTTP2Config{
		IdleTimeoutSeconds: h.IdleTimeoutSeconds,
	}

	var err error

	// MaxConcurrentStreams (no size suffix, just number)
	if h.MaxConcurrentStreams != "" {
		maxStreams, err := strconv.ParseUint(h.MaxConcurrentStreams, 10, 32)
		if err != nil {
			return nil, fmt.Errorf("invalid max_concurrent_streams: %w", err)
		}
		parsed.MaxConcurrentStreams = uint32(maxStreams)
	}

	// InitialWindowSize (HTTP/2 spec: max 2^31-1 bytes, but we limit to 100MB for safety)
	if h.InitialWindowSize != "" {
		size, err := ParseSize(h.InitialWindowSize)
		if err != nil {
			return nil, fmt.Errorf("invalid initial_window_size: %w", err)
		}
		// HTTP/2 spec allows up to 2147483647 (2GB-1), but we validate max 100MB
		if size > 100*1024*1024 {
			return nil, fmt.Errorf("initial_window_size exceeds 100MB limit: %d", size)
		}
		parsed.InitialWindowSize = int32(size)
	}

	// MaxFrameSize (HTTP/2 spec: 16KB to 16MB-1, we limit to 100MB)
	if h.MaxFrameSize != "" {
		size, err := ParseSize(h.MaxFrameSize)
		if err != nil {
			return nil, fmt.Errorf("invalid max_frame_size: %w", err)
		}
		if size > 100*1024*1024 {
			return nil, fmt.Errorf("max_frame_size exceeds 100MB limit: %d", size)
		}
		parsed.MaxFrameSize = uint32(size)
	}

	// MaxHeaderListSize
	if h.MaxHeaderListSize != "" {
		size, err := ParseSize(h.MaxHeaderListSize)
		if err != nil {
			return nil, fmt.Errorf("invalid max_header_list_size: %w", err)
		}
		if size > 100*1024*1024 {
			return nil, fmt.Errorf("max_header_list_size exceeds 100MB limit: %d", size)
		}
		parsed.MaxHeaderListSize = uint32(size)
	}

	// MaxUploadBufferPerConn
	if h.MaxUploadBufferPerConn != "" {
		size, err := ParseSize(h.MaxUploadBufferPerConn)
		if err != nil {
			return nil, fmt.Errorf("invalid max_upload_buffer_per_conn: %w", err)
		}
		if size > 100*1024*1024 {
			return nil, fmt.Errorf("max_upload_buffer_per_conn exceeds 100MB limit: %d", size)
		}
		parsed.MaxUploadBufferPerConn = int32(size)
	}

	// MaxUploadBufferPerStream
	if h.MaxUploadBufferPerStream != "" {
		size, err := ParseSize(h.MaxUploadBufferPerStream)
		if err != nil {
			return nil, fmt.Errorf("invalid max_upload_buffer_per_stream: %w", err)
		}
		if size > 100*1024*1024 {
			return nil, fmt.Errorf("max_upload_buffer_per_stream exceeds 100MB limit: %d", size)
		}
		parsed.MaxUploadBufferPerStream = int32(size)
	}

	return parsed, err
}
