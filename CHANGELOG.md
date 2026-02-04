# Changelog 

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

