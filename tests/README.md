# 🧪 HSM Service Test Suite

Comprehensive testing framework for the HSM Service including unit tests, integration tests, E2E scenarios, and security scans.

## 📁 Structure

```
tests/
├── run-all-tests.sh              # Master test runner (all phases)
├── integration/
│   └── full-integration-test.sh  # Complete integration test (42 tests)
├── e2e/                          # End-to-end scenario tests
│   ├── run-all.sh                # E2E runner
│   └── scenarios/
│       ├── key-rotation-e2e.sh   # Complete key rotation workflow
│       ├── disaster-recovery.sh  # Backup/restore scenario
│       └── acl-realtime-reload.sh # Dynamic ACL changes
├── security/
│   └── security-scan.sh          # Security vulnerability scanner (8 checks)
├── manual/
│   └── test-mtls-only.sh         # Manual mTLS-only test
└── README.md                     # This file

scripts/
├── check-key-rotation.sh         # Key rotation verification
├── cleanup-old-keys.sh           # PCI DSS compliance cleanup
├── auto-rotate-keys.sh           # Automated rotation
└── init-hsm.sh                   # HSM initialization

internal/
├── config/config_test.go         # Config unit tests
├── hsm/
│   ├── crypto_test.go            # Encryption/decryption tests
│   ├── key_manager_test.go       # Hot reload tests
│   └── rotation_test.go          # Rotation logic tests
└── server/
    ├── acl_test.go               # ACL validation tests
    ├── acl_reload_test.go        # ACL auto-reload tests
    ├── handlers_test.go          # HTTP handler tests
    ├── middleware_test.go        # Rate limiting tests
    └── logger_test.go            # Logging tests
```

## 🚀 Quick Start

### Run All Tests (Recommended)
```bash
# Full test suite (unit + integration + E2E)
./tests/run-all-tests.sh

# Or individually:
# 1. Unit tests
go test -v -race ./...

# 2. Integration tests (requires Docker)
./tests/integration/full-integration-test.sh

# 3. E2E scenario tests
./tests/e2e/run-all.sh

# 4. Security scan
./tests/security/security-scan.sh
```

## 📋 Test Categories

### 1. Unit Tests (Go)
**Coverage**: ~80%  
**Runtime**: ~5 seconds  
**Command**: `go test -v ./...`

Tests individual functions and modules in isolation:
- Encryption/decryption operations
- Key manager hot reload
- ACL validation and auto-reload
- HTTP handlers
- Rate limiting
- Configuration loading

### 2. Integration Tests (Bash + Docker)
**Coverage**: 42 test cases  
**Runtime**: ~10 minutes  
**Command**: `./tests/integration/full-integration-test.sh`

**Phases**:
1. Docker cleanup and rebuild
2. Build from scratch (no cache)
3. PKI verification
4. Metadata initialization
5. Service startup
6. HSM initialization
7. Basic functionality (encrypt/decrypt)
8. Key rotation (v1→v2→v3→v4)
9. KEK hot reload (zero-downtime)
10. Cleanup old versions (PCI DSS)
11. mTLS security validation
12. Volume persistence
13. Environment variables override

### 3. E2E Scenario Tests
**Coverage**: 3 critical workflows  
**Runtime**: ~5 minutes  
**Command**: `./tests/e2e/run-all.sh`

**Scenarios**:
1. **Key Rotation E2E**
   - Encrypt with v1 → Rotate → Decrypt old data → Encrypt with v2
   - Verifies backward compatibility during overlap period

2. **Disaster Recovery**
   - Create data → Backup → Destroy → Restore → Verify
   - Tests complete backup/restore procedure

3. **ACL Real-time Reload**
   - Connect → Revoke → Block → Restore → Connect
   - Tests dynamic certificate revocation without restart

### 4. Security Scans
**Runtime**: ~2 minutes  
**Command**: `./tests/security/security-scan.sh`

**Checks**:
- `gosec` - Go security vulnerabilities
- `go vet` - Standard Go checks
- `staticcheck` - Advanced static analysis
- `govulncheck` - Dependency vulnerabilities
- `trivy` - Container image CVE scan
- TLS certificate validation
- Hardcoded secrets detection
- Dockerfile best practices

## 🎯 Running Specific Tests

### Run Single E2E Scenario
```bash
cd tests/e2e
./scenarios/key-rotation-e2e.sh
```

### Run Specific Unit Test
```bash
go test -v -run TestKeyManagerHotReload ./internal/hsm/
```

### Run Integration Test Phase
```bash
# See full-integration-test.sh and comment out unwanted phases
```

### Run Security Scan Only
```bash
./tests/security/security-scan.sh
```

## 📊 Test Coverage

### Current Status (as of 2026-01-11)

| Category | Coverage | Status |
|----------|----------|--------|
| Unit Tests | ~80% | ✅ Good |
| Integration Tests | 42/42 | ✅ Complete |
| E2E Scenarios | 3/3 | ✅ Complete |
| Security Scans | 8/8 | ✅ Complete |
| **Overall** | ~85% | ✅ Production Ready |

### Critical Paths Covered
- ✅ Encrypt/Decrypt operations
- ✅ Key rotation (multi-version support)
- ✅ Hot reload (zero-downtime)
- ✅ mTLS security
- ✅ ACL enforcement
- ✅ Rate limiting
- ✅ Volume persistence
- ✅ Disaster recovery
- ✅ PCI DSS compliance

## 🔧 Prerequisites

### For Unit Tests
```bash
go version  # Go 1.22+
```

### For Integration Tests
```bash
docker --version  # Docker 20.10+
docker compose version  # v2.0+
```

### For Security Scans (Optional but Recommended)
```bash
# Install security tools
go install github.com/securego/gosec/v2/cmd/gosec@latest
go install honnef.co/go/tools/cmd/staticcheck@latest
go install golang.org/x/vuln/cmd/govulncheck@latest

# Install trivy (container scanner)
# See: https://aquasecurity.github.io/trivy/
```

## 🐛 Debugging Failed Tests

### Integration Test Failed
```bash
# Check logs in /tmp/
cat /tmp/docker-build.log
cat /tmp/docker-compose-up.log

# Check container logs
docker logs hsm-service

# Run with verbose output
./tests/integration/full-integration-test.sh 2>&1 | tee test-debug.log
```

### E2E Test Failed
```bash
# Check individual test logs
cat /tmp/e2e-Key-Rotation.log
cat /tmp/e2e-Disaster-Recovery.log
cat /tmp/e2e-ACL-Real-time-Reload.log

# Run single scenario with debug
bash -x ./tests/e2e/scenarios/key-rotation-e2e.sh
```

### Unit Test Failed
```bash
# Run with verbose + race detection
go test -v -race ./internal/hsm/

# Run specific test
go test -v -run TestKeyManagerHotReload ./internal/hsm/

# With coverage
go test -cover ./...
```

## 📈 Continuous Integration

### GitHub Actions Example
```yaml
name: Tests
on: [push, pull_request]
jobs:
  test:
    runs-on: ubuntu-latest
    steps:
      - uses: actions/checkout@v3
      - uses: actions/setup-go@v4
        with:
          go-version: '1.22'
      - name: Unit Tests
        run: go test -v -race ./...
      - name: Integration Tests
        run: ./tests/integration/full-integration-test.sh
      - name: Security Scan
        run: ./tests/security/security-scan.sh
```

## 📝 Test Development Guidelines

### Adding New Unit Test
1. Create `*_test.go` file next to source
2. Follow table-driven test pattern
3. Use subtests with `t.Run()`
4. Add `// +build !race` if needed
5. Run with `-race` flag

### Adding New E2E Scenario
1. Create script in `tests/e2e/scenarios/`
2. Follow existing pattern (setup → test → cleanup)
3. Use color-coded output
4. Add to `tests/e2e/run-all.sh`
5. Document in this README

### Adding Integration Test Phase
1. Edit `tests/integration/full-integration-test.sh`
2. Increment `TOTAL_TESTS` counter
3. Add phase header
4. Use `print_test` for each test
5. Ensure cleanup on failure

## 🔐 Security Best Practices

- ✅ All tests run with non-root user
- ✅ No hardcoded secrets (use env vars)
- ✅ TLS certificates validated
- ✅ Container images scanned for CVEs
- ✅ Race condition detection enabled
- ✅ Dependencies checked for vulnerabilities

## 📞 Support

For issues or questions:
- Check existing test logs in `/tmp/`
- Review test documentation above
- Run tests with `-x` flag for debug output
- Check container logs with `docker logs hsm-service`

---

**Last Updated**: 2026-01-11  
**Test Suite Version**: 2.0  
**Maintainer**: HSM Service Team
