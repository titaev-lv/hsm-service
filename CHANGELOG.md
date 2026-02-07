# Changelog 

## v1.1.1 - 2026-02-08

### Security Fixes
- Fix PIN exposure in rotation command logs (mask sensitive data)
- Prevent command injection in rotate command (G204/CWE-78)
- Add path validation to prevent directory traversal attacks (G304/CWE-22)
- Restrict metadata file permissions to 0600 (G302/G306/CWE-276)
- Add error handling for unhandled errors (G104/CWE-703)
- Add explicit overflow checks with nosec annotations (G115/CWE-190)

### Features
- Add automatic checksum updates after key rotation
- Implement RFC3339Micro timestamp format for consistent YAML serialization

### Bug Fixes
- Preserve inode when saving metadata files (bind mount compatibility)
- Correct config.yaml PKI paths for proper certificate validation
- Fix certificate revocation test robustness

### Tests
- Add unit tests for validateFilePath with directory traversal protection
- Improve integration test isolation with dedicated containers
- Add full-featured certificate revocation test
- Auto-generate PKI for self-contained integration tests

### Documentation
- Update timestamp format to RFC3339Micro in all documentation
- Add missing rotation_interval_days parameter examples

---

## v1.1.0

### New Features
- **Automated PKI & Docker Setup**: Added `init-pki-docker.sh` script for one-command initialization
  - Generates Root CA and all certificates (server + client) automatically
  - Creates metadata.yaml configuration
  - Builds and starts Docker container
  - Validates service health
  - Supports `--force` and `--skip-docker` options

### Documentation
- Improved QUICKSTART_DOCKER.md with:
  - ⚡ One-command quick start section
  - 📖 Step-by-step manual setup fallback
  - 💡 Useful commands reference
  - ❓ Troubleshooting guide
- Added VERSIONING.md with detailed versioning strategy

### Infrastructure
- Fixed metadata.yaml mounting (reverted to mount strategy from Dockerfile copy)
- Certificate path improvements
- Added VERSION file support for Docker builds

### Bug Fixes
- Fix metadata.yaml handling in docker-compose.yml
- Fix metadata.yaml Dockerfile logic
- Correct certificate paths in configuration

---

## v1.0.1

### Core 
- Base HSM service and PKCS#11/SoftHSM integration.
- CLI tools: `hsm-admin`, `create-kek`.
- Key rotation scripts and KEK hot‑reload.

### Security & Compliance
- OWASP Top 10 fixes (A02/A03/A04/A05/A08/A09).
- Request size limits, timeouts, rate limiting.
- Logging/monitoring improvements, log rotation, Prometheus metrics.
- AAD validation and KEK integrity verification.

### Infra & Deployment
- Docker/Docker Compose.
- PKI bootstrap and guides.
- Init/rotation/backup/restore scripts with integrity checks.

### Testing & Quality
- Unit/integration/e2e/performance/security/compliance tests.
- Load, stress, and extreme tests.
- HTTP/2 and network stack tuning.

### Documentation
- Quickstart/production/security audit docs.
- Recovery, monitoring, firewall/SELinux/AppArmor guides.

