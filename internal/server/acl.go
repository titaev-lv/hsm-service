package server

import (
"crypto/x509"
"errors"
"fmt"
"os"
"sync"

"github.com/titaev-lv/hsm-service/internal/config"
"gopkg.in/yaml.v3"
)

// ACLChecker handles authorization checks based on OU and revocation
type ACLChecker struct {
config       *config.ACLConfig
revoked      map[string]bool // CN -> revoked
revokedMutex sync.RWMutex
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

// NewACLChecker creates a new ACL checker
func NewACLChecker(cfg *config.ACLConfig) (*ACLChecker, error) {
checker := &ACLChecker{
config:  cfg,
revoked: make(map[string]bool),
}

// Load revoked list
if err := checker.LoadRevoked(); err != nil {
return nil, fmt.Errorf("failed to load revoked list: %w", err)
}

return checker, nil
}

// LoadRevoked loads the revoked certificates list from YAML
func (a *ACLChecker) LoadRevoked() error {
a.revokedMutex.Lock()
defer a.revokedMutex.Unlock()

data, err := os.ReadFile(a.config.RevokedFile)
if err != nil {
// If file doesn't exist, start with empty list
if os.IsNotExist(err) {
a.revoked = make(map[string]bool)
return nil
}
return fmt.Errorf("failed to read revoked file: %w", err)
}

var revokedList RevokedList
if err := yaml.Unmarshal(data, &revokedList); err != nil {
return fmt.Errorf("failed to parse revoked YAML: %w", err)
}

// Rebuild revoked map
a.revoked = make(map[string]bool)
for _, cert := range revokedList.Revoked {
a.revoked[cert.CN] = true
}

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
return fmt.Errorf("certificate revoked: %s", cn)
}

// 2. Extract OU (Organizational Unit)
if len(cert.Subject.OrganizationalUnit) == 0 {
return errors.New("certificate has no OU")
}
ou := cert.Subject.OrganizationalUnit[0]

// 3. Check OU permissions
allowedContexts, ok := a.config.Mappings[ou]
if !ok {
return fmt.Errorf("unknown OU: %s", ou)
}

// 4. Check if context is allowed for this OU
for _, allowed := range allowedContexts {
if allowed == context {
return nil // Access granted
}
}

return fmt.Errorf("OU %s not allowed for context %s", ou, context)
}

// IsRevoked checks if a certificate is revoked by CN
func (a *ACLChecker) IsRevoked(cn string) bool {
a.revokedMutex.RLock()
defer a.revokedMutex.RUnlock()
return a.revoked[cn]
}
