# Changelog 

## v2.0.0 - 2026-02-18

### Breaking Changes
- Remove standalone `create-kek` binary (use `hsm-admin create-kek`)

### Build & Packaging
- Drop create-kek build steps from Makefile, Dockerfile, and release packaging

### Documentation
- Remove create-kek references from build and CLI docs

### Bug Fixes
- Fix CloseLogger untyped bool

## v1.2.0 - 2026-02-17

### Features
- Add `hsm-admin create-kek` subcommand with direct PKCS#11 key creation
- Use internal create-kek logic during rotation (no external binary)

### Deprecations
- Mark standalone `create-kek` utility as deprecated

### Scripts
- Update init-hsm.sh to use `hsm-admin create-kek`

### Documentation
- Update production and CLI guides to reference `hsm-admin create-kek`
- Mark `create-kek` build/deploy steps as optional

## v1.1.4 - 2026-02-15

### Features
- Add graceful shutdown handling for SIGTERM/SIGINT with server.Shutdown(ctx)
- Add panic recovery middleware with stack traces in error.log

### Reliability
- Close log writers on shutdown to flush audit/access/error logs

### Tests
- Add recovery middleware unit test
- Add integration checks for SIGTERM shutdown and clean logs

### Documentation
- Document graceful shutdown and panic recovery behavior

## v1.1.3 - 2026-02-13

### Features
- Add access.log for HTTP requests with request_id and size metrics
- Include request/response sizes and TLS version in access logs

### Configuration
- Add access log config fields and env overrides (access_path, access_to_stdout)

### Tests
- Update integration test config/compose templates for access log paths
- Fix docker-compose test templates for logs-test volume indentation

### Documentation
- Document access log in production, monitoring, quickstart, and troubleshooting guides

## v1.1.2 - 2026-02-12

### Features
- Split audit and error logs with independent rotation and JSON output
- Add request tracking via X-Request-ID with status/result/error_code in audit logs
- Include TLS/PCI fields in audit logs (tls_version, tls_cipher, cert metadata)
- Add module tags for api/acl/crypto/middleware/rate_limit loggers
- Standardize timestamps to UTC RFC3339 microseconds in slog output

### Configuration
- Make logging fully config-driven with defaults and env overrides
- Add options for audit stdout mirroring and debug mirroring to error logs

### Reliability
- Fail fast if log directories are not writable (write/rename checks)
- Restrict log directory permissions to 0750

### Tests
- Add logging config default/env override tests
- Add logger initialization and log path validation tests
- Update integration and e2e scripts to create log dirs and mount /logs

### Documentation
- Update README, monitoring, security, troubleshooting, and production guides
- Document audit/error log split, request_id, and log path requirements


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

