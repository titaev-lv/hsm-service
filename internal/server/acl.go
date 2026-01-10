package server

import (
	"context"
	"crypto/x509"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"sync"
	"time"

	"github.com/titaev-lv/hsm-service/internal/config"
	"gopkg.in/yaml.v3"
)

// ACLChecker handles authorization checks based on OU and revocation
type ACLChecker struct {
	config       *config.ACLConfig
	revoked      map[string]bool // CN -> revoked
	revokedMutex sync.RWMutex

	// Hot reload support
	reloadInterval time.Duration
	lastModTime    time.Time
	stopReload     chan struct{}
	reloadWg       sync.WaitGroup
	stopOnce       sync.Once
}

// RevokedList represents the structure of revoked.yaml
type RevokedList struct {
	Revoked []struct {
		CN     string `yaml:"cn"`
		Serial string `yaml:"serial"`
		Reason string `yaml:"reason"`
		Date   string `yaml:"date"`
	} `yaml:"revoked"`
}

// NewACLChecker creates a new ACL checker with auto-reload support
func NewACLChecker(cfg *config.ACLConfig) (*ACLChecker, error) {
	checker := &ACLChecker{
		config:         cfg,
		revoked:        make(map[string]bool),
		reloadInterval: 30 * time.Second, // Check every 30 seconds
		stopReload:     make(chan struct{}),
	}

	// Load revoked list initially
	if err := checker.LoadRevoked(); err != nil {
		return nil, fmt.Errorf("failed to load revoked list: %w", err)
	}

	// Start auto-reload goroutine
	checker.StartAutoReload()

	return checker, nil
}

// StartAutoReload starts background goroutine for periodic reload
func (a *ACLChecker) StartAutoReload() {
	a.reloadWg.Add(1)
	go func() {
		defer a.reloadWg.Done()

		ticker := time.NewTicker(a.reloadInterval)
		defer ticker.Stop()

		slog.Info("started revoked.yaml auto-reload",
			"interval", a.reloadInterval.String(),
			"file", a.config.RevokedFile)

		for {
			select {
			case <-ticker.C:
				if err := a.TryReload(); err != nil {
					slog.Warn("auto-reload failed", "path", a.config.RevokedFile)
					// Don't expose error details in logs
				}
			case <-a.stopReload:
				slog.Info("stopped revoked.yaml auto-reload")
				return
			}
		}
	}()
}

// StopAutoReload stops the background reload goroutine
func (a *ACLChecker) StopAutoReload(ctx context.Context) error {
	a.stopOnce.Do(func() {
		close(a.stopReload)
	})

	// Wait with timeout
	done := make(chan struct{})
	go func() {
		a.reloadWg.Wait()
		close(done)
	}()

	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// TryReload attempts to reload revoked.yaml if file was modified
// Returns nil if successful or file unchanged
func (a *ACLChecker) TryReload() error {
	// Get file info
	info, err := os.Stat(a.config.RevokedFile)
	if err != nil {
		if os.IsNotExist(err) {
			// File deleted - clear revoked list
			a.revokedMutex.Lock()
			a.revoked = make(map[string]bool)
			a.lastModTime = time.Time{}
			a.revokedMutex.Unlock()
			slog.Info("revoked.yaml deleted, cleared revocation list")
			return nil
		}
		return fmt.Errorf("failed to stat file: %w", err)
	}

	// Check if file was modified (protected read)
	modTime := info.ModTime()
	a.revokedMutex.RLock()
	lastMod := a.lastModTime
	a.revokedMutex.RUnlock()

	if !modTime.After(lastMod) {
		// File not changed
		return nil
	}

	// File changed - try to reload with validation
	if err := a.LoadRevokedSafe(); err != nil {
		// Keep old data on error
		slog.Warn("revoked.yaml reload skipped due to validation error",
			"path", a.config.RevokedFile)
		return err
	}

	// Update modification time on success (protected write)
	a.revokedMutex.Lock()
	a.lastModTime = modTime
	revokedCount := len(a.revoked)
	a.revokedMutex.Unlock()

	slog.Info("revoked.yaml reloaded successfully",
		"path", a.config.RevokedFile,
		"count", revokedCount)

	return nil
}

// LoadRevokedSafe loads and validates revoked.yaml without updating state on error
func (a *ACLChecker) LoadRevokedSafe() error {
	// Read file
	data, err := os.ReadFile(a.config.RevokedFile)
	if err != nil {
		return fmt.Errorf("failed to read file: %w", err)
	}

	// Parse YAML
	var revokedList RevokedList
	if err := yaml.Unmarshal(data, &revokedList); err != nil {
		return fmt.Errorf("invalid YAML format: %w", err)
	}

	// Validate structure
	if err := a.validateRevokedList(&revokedList); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Build new map
	newRevoked := make(map[string]bool)
	for _, cert := range revokedList.Revoked {
		if cert.CN == "" {
			return fmt.Errorf("empty CN in revoked list")
		}
		newRevoked[cert.CN] = true
	}

	// Atomic update
	a.revokedMutex.Lock()
	a.revoked = newRevoked
	a.revokedMutex.Unlock()

	return nil
}

// validateRevokedList validates the revoked list structure
func (a *ACLChecker) validateRevokedList(list *RevokedList) error {
	if list == nil {
		return errors.New("nil revoked list")
	}

	// Check for duplicate CNs
	seen := make(map[string]bool)
	for i, cert := range list.Revoked {
		if cert.CN == "" {
			return fmt.Errorf("entry %d has empty CN", i)
		}
		if seen[cert.CN] {
			return fmt.Errorf("duplicate CN: %s", cert.CN)
		}
		seen[cert.CN] = true
	}

	return nil
}

// LoadRevoked loads the revoked certificates list from YAML (initial load)
func (a *ACLChecker) LoadRevoked() error {
	data, err := os.ReadFile(a.config.RevokedFile)
	if err != nil {
		// If file doesn't exist, start with empty list
		if os.IsNotExist(err) {
			a.revokedMutex.Lock()
			a.revoked = make(map[string]bool)
			a.lastModTime = time.Time{}
			a.revokedMutex.Unlock()
			return nil
		}
		return fmt.Errorf("failed to read revoked file: %w", err)
	}

	var revokedList RevokedList
	if err := yaml.Unmarshal(data, &revokedList); err != nil {
		return fmt.Errorf("failed to parse revoked YAML: %w", err)
	}

	// Validate
	if err := a.validateRevokedList(&revokedList); err != nil {
		return fmt.Errorf("validation failed: %w", err)
	}

	// Build revoked map
	a.revokedMutex.Lock()
	a.revoked = make(map[string]bool)
	for _, cert := range revokedList.Revoked {
		a.revoked[cert.CN] = true
	}

	// Get initial modification time
	if info, err := os.Stat(a.config.RevokedFile); err == nil {
		a.lastModTime = info.ModTime()
	}
	a.revokedMutex.Unlock()

	return nil
}

// CheckAccess verifies if a client certificate has access to the specified context
func (a *ACLChecker) CheckAccess(cert *x509.Certificate, context string) error {
	if cert == nil {
		return errors.New("certificate is nil")
	}

	cn := cert.Subject.CommonName

	// 1. Check revoked list
	a.revokedMutex.RLock()
	revoked := a.revoked[cn]
	a.revokedMutex.RUnlock()

	if revoked {
		// Metrics: track revocation failure
		RecordRevocationFailure()
		// Don't expose CN in error (information disclosure)
		return errors.New("certificate revoked")
	}

	// 2. Extract OU (Organizational Unit)
	if len(cert.Subject.OrganizationalUnit) == 0 {
		return errors.New("certificate has no OU")
	}
	ou := cert.Subject.OrganizationalUnit[0]

	// 3. Check OU permissions
	allowedContexts, ok := a.config.Mappings[ou]
	if !ok {
		// Don't expose OU in error (information disclosure)
		return errors.New("access denied: unknown organizational unit")
	}

	// 4. Check if context is allowed for this OU
	for _, allowed := range allowedContexts {
		if allowed == context {
			return nil // Access granted
		}
	}

	// Don't expose OU or context in error (information disclosure)
	return errors.New("access denied: insufficient permissions")
}

// IsRevoked checks if a certificate is revoked by CN
func (a *ACLChecker) IsRevoked(cn string) bool {
	a.revokedMutex.RLock()
	defer a.revokedMutex.RUnlock()
	return a.revoked[cn]
}
