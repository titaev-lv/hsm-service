# 🛡️ Compliance Tests

Automated compliance testing for HSM Service against industry standards.

## 📋 Test Suites

### 1. PCI DSS Compliance

**Standard**: PCI DSS v4.0 (Payment Card Industry Data Security Standard)

**Script**: `pci-dss.sh`

**Coverage**:
- ✅ Requirement 3: Protect Stored Data
  - Key rotation ≤ 90 days
  - Automatic key cleanup
  - No plaintext in logs
  
- ✅ Requirement 4: Encrypt Data Transmission
  - TLS 1.2+ only
  - Strong cipher suites
  - mTLS certificate validation
  
- ✅ Requirement 8: Access Control
  - ACL enforcement
  - Certificate revocation
  - Role-based access
  
- ✅ Requirement 10: Logging & Monitoring
  - Audit logging
  - Structured logs (JSON)
  - Metrics endpoint

**Run**:
```bash
./tests/compliance/pci-dss.sh
```

**Expected Output**:
```
================================================================
  PCI DSS Compliance Tests
================================================================

=== PCI DSS Requirement 3: Protect Stored Data ===
[1] PCI DSS 3.5.1: Key rotation interval ≤ 90 days... ✓ PASS
[2] PCI DSS 3.5.2: Automatic cleanup of old key versions... ✓ PASS
[3] PCI DSS 3.5.3: Limited number of active key versions... ✓ PASS
[4] PCI DSS 3.4: No plaintext data in logs... ✓ PASS

...

Total Tests: 17
Passed: 17
Failed: 0

✓ PCI DSS Compliance: PASS
```

---

### 2. OWASP Top 10 2021

**Standard**: OWASP Top 10 2021 Web Application Security Risks

**Script**: `owasp-top10.sh`

**Coverage**:
- ✅ A01: Broken Access Control
  - ACL enforcement
  - Path traversal prevention
  - Revoked certificate denial
  
- ✅ A02: Cryptographic Failures
  - Strong encryption (AES-256-GCM)
  - TLS 1.3
  - Proper key management
  
- ✅ A03: Injection
  - JSON injection protection
  - Command injection prevention
  
- ✅ A04: Insecure Design
  - Rate limiting
  - DoS protection
  
- ✅ A05: Security Misconfiguration
  - Secure defaults
  - No debug endpoints
  - Proper error handling
  
- ✅ A07: Authentication Failures
  - mTLS required
  - Stateless design
  
- ✅ A08: Data Integrity Failures
  - Input validation
  
- ✅ A09: Logging & Monitoring Failures
  - Security event logging
  - Metrics monitoring
  - Audit trail
  
- ✅ A10: SSRF
  - Not applicable (no external requests)

**Run**:
```bash
./tests/compliance/owasp-top10.sh
```

---

## 🚀 Running All Compliance Tests

### Quick Test

```bash
# Run both PCI DSS and OWASP tests
./tests/compliance/pci-dss.sh && ./tests/compliance/owasp-top10.sh
```

### With Custom Configuration

```bash
# Custom HSM URL and certificates
HSM_URL=https://prod.example.com:8443 \
CLIENT_CERT=/path/to/cert.crt \
CLIENT_KEY=/path/to/key.key \
./tests/compliance/pci-dss.sh
```

### CI/CD Integration

```bash
# Exit on first failure
set -e
./tests/compliance/pci-dss.sh
./tests/compliance/owasp-top10.sh
```

---

## 📊 Test Results Interpretation

### PASS ✓

All compliance tests passed. Service meets the standard requirements.

### FAIL ✗

One or more tests failed. Review the output for specific failures:

```
[5] PCI DSS 4.2.1: TLS 1.2+ only... ✗ FAIL
    Reason: Weak TLS version detected: TLSv1.0
```

**Action**: Fix the identified issue and re-run tests.

### WARNING ⚠

Non-critical issue detected, but test passed:

```
[8] PCI DSS 10.3: Structured logging... ⚠ WARNING: JSON logging not configured
```

**Action**: Consider addressing warnings for best practices.

---

## 🔧 Troubleshooting

### Service Not Reachable

```
ERROR: HSM Service not reachable at https://localhost:8443
```

**Solution**:
```bash
# Start the service
docker compose up -d

# Verify it's running
./tests/performance/smoke-test.sh
```

---

### Certificate Issues

```
FAIL: Service accepts requests without client certificate
```

**Solution**: Check that mTLS is properly configured in `config.yaml`:
```yaml
server:
  tls:
    ca_path: /app/pki/ca/ca.crt
    cert_path: /app/pki/server/hsm-service.local.crt
    key_path: /app/pki/server/hsm-service.local.key
```

---

### Rate Limiting Test Fails

If DoS protection test fails to trigger rate limiting:

**Solution**: Check rate limit configuration:
```yaml
rate_limit:
  requests_per_second: 100  # Ensure this is > 0
  burst: 50
```

---

## 📚 Standards Documentation

- **PCI DSS**: https://www.pcisecuritystandards.org/
- **OWASP Top 10**: https://owasp.org/Top10/
- **TLS Best Practices**: https://wiki.mozilla.org/Security/Server_Side_TLS

---

## 🎯 Compliance Checklist

Before production deployment:

- [ ] ✅ All PCI DSS tests pass
- [ ] ✅ All OWASP Top 10 tests pass
- [ ] ✅ Certificate rotation procedures documented
- [ ] ✅ Incident response plan in place
- [ ] ✅ Regular compliance audits scheduled
- [ ] ✅ Security team trained on standards

---

## 📈 Continuous Compliance

### Daily

```bash
# Quick health check
./tests/compliance/pci-dss.sh | grep "PCI DSS Compliance: PASS"
```

### Weekly

```bash
# Full compliance scan
./tests/compliance/pci-dss.sh
./tests/compliance/owasp-top10.sh
```

### Monthly

```bash
# Full security audit
./tests/security/security-scan.sh
./tests/compliance/pci-dss.sh
./tests/compliance/owasp-top10.sh
```

---

## 🔐 Security Notes

1. **Never disable security features** to pass compliance tests
2. **Document all exceptions** if certain tests cannot pass
3. **Automate compliance testing** in CI/CD pipeline
4. **Keep standards updated** - check for new versions quarterly
5. **Audit logs** should be reviewed regularly, not just tested

---

**Last Updated**: 2026-01-11  
**Standards Version**: PCI DSS v4.0, OWASP Top 10 2021
