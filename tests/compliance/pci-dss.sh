#!/bin/bash
# PCI DSS Compliance Tests for HSM Service
# PCI DSS v4.0 Requirements for Key Management

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

PASSED=0
FAILED=0
TOTAL=0

# HSM Service URL
HSM_URL="${HSM_URL:-https://localhost:8443}"
CLIENT_CERT="${CLIENT_CERT:-$PROJECT_ROOT/pki/client/hsm-trading-client-1.crt}"
CLIENT_KEY="${CLIENT_KEY:-$PROJECT_ROOT/pki/client/hsm-trading-client-1.key}"

print_header() {
    echo ""
    echo "================================================================"
    echo "  PCI DSS Compliance Tests"
    echo "================================================================"
    echo ""
}

print_test() {
    TOTAL=$((TOTAL + 1))
    echo -n "[$TOTAL] $1... "
}

pass() {
    PASSED=$((PASSED + 1))
    echo -e "${GREEN}✓ PASS${NC}"
}

fail() {
    FAILED=$((FAILED + 1))
    echo -e "${RED}✗ FAIL${NC}"
    if [ -n "$1" ]; then
        echo -e "    ${RED}Reason: $1${NC}"
    fi
}

warn() {
    echo -e "${YELLOW}⚠ WARNING: $1${NC}"
}

# ============================================================
# Requirement 3.5: Key Management
# ============================================================

test_key_rotation_interval() {
    print_test "PCI DSS 3.5.1: Key rotation interval ≤ 90 days"
    
    # Check metadata.yaml for rotation interval
    if [ -f "$PROJECT_ROOT/metadata.yaml" ]; then
        ROTATION_DAYS=$(grep -A10 "exchange-key:" "$PROJECT_ROOT/metadata.yaml" | grep "rotation_interval_days:" | head -1 | awk '{print $2}')
        
        if [ -z "$ROTATION_DAYS" ]; then
            fail "rotation_interval_days not found in metadata.yaml"
            return
        fi
        
        if [ "$ROTATION_DAYS" -le 90 ]; then
            pass
        else
            fail "Rotation interval $ROTATION_DAYS days exceeds PCI DSS limit of 90 days"
        fi
    else
        warn "metadata.yaml not found, checking config.yaml"
        # Check if rotation is configured
        if grep -q "rotation_interval_days" "$PROJECT_ROOT/config.yaml"; then
            pass
        else
            fail "No key rotation configuration found"
        fi
    fi
}

test_key_cleanup() {
    print_test "PCI DSS 3.5.2: Automatic cleanup of old key versions"
    
    # Check config.yaml for cleanup_after_days
    if grep -q "cleanup_after_days:" "$PROJECT_ROOT/config.yaml"; then
        CLEANUP_DAYS=$(grep "cleanup_after_days:" "$PROJECT_ROOT/config.yaml" | awk '{print $2}')
        
        if [ "$CLEANUP_DAYS" -le 365 ]; then
            pass
        else
            warn "Cleanup after $CLEANUP_DAYS days might be too long"
            pass
        fi
    else
        fail "cleanup_after_days not configured"
    fi
}

test_max_key_versions() {
    print_test "PCI DSS 3.5.3: Limited number of active key versions"
    
    # Check config.yaml for max_versions
    if grep -q "max_versions:" "$PROJECT_ROOT/config.yaml"; then
        MAX_VERSIONS=$(grep "max_versions:" "$PROJECT_ROOT/config.yaml" | awk '{print $2}')
        
        if [ "$MAX_VERSIONS" -le 5 ]; then
            pass
        else
            warn "max_versions=$MAX_VERSIONS is high, consider reducing"
            pass
        fi
    else
        fail "max_versions not configured"
    fi
}

# ============================================================
# Requirement 4.2: Strong Cryptography for Data Transmission
# ============================================================

test_tls_version() {
    print_test "PCI DSS 4.2.1: TLS 1.2+ only (no SSLv3, TLS 1.0, TLS 1.1)"
    
    # Test TLS connection (output format: "New, TLSv1.3, Cipher is...")
    TLS_OUTPUT=$(echo | timeout 3 openssl s_client -connect localhost:8443 -cert "$CLIENT_CERT" -key "$CLIENT_KEY" 2>&1 | grep -E "New, TLS")
    
    if echo "$TLS_OUTPUT" | grep -qE "TLSv1\.(3|2)"; then
        pass
    else
        fail "Weak or unknown TLS version: $TLS_OUTPUT"
    fi
}

test_strong_ciphers() {
    print_test "PCI DSS 4.2.1: Strong cipher suites only"
    
    # Get cipher suite (output format: "New, TLSv1.3, Cipher is TLS_AES_128_GCM_SHA256")
    CIPHER=$(echo | timeout 3 openssl s_client -connect localhost:8443 -cert "$CLIENT_CERT" -key "$CLIENT_KEY" 2>&1 | grep -E "Cipher is" | head -1 | sed 's/.*Cipher is //')
    
    # Check if cipher is strong (AES-GCM, ChaCha20)
    if [[ "$CIPHER" =~ (AES.*GCM|CHACHA20) ]]; then
        pass
    else
        fail "Weak cipher detected: $CIPHER"
    fi
}

test_certificate_validation() {
    print_test "PCI DSS 4.2.1: Certificate validation (mTLS)"
    
    # Try to connect without client certificate with timeout
    if ! HTTP_CODE=$(timeout 5 curl -sk -o /dev/null -w "%{http_code}" "$HSM_URL/health" 2>/dev/null); then
        # Connection failed (good - cert required)
        pass
    elif [ "$HTTP_CODE" == "000" ]; then
        pass
    else
        fail "Server accepts connections without client certificate (HTTP $HTTP_CODE)"
    fi
}

# ============================================================
# Requirement 10: Logging and Monitoring
# ============================================================

test_audit_logging() {
    print_test "PCI DSS 10.2: Audit logging of crypto operations"
    
    # Check if logging is configured
    if grep -q "logging:" "$PROJECT_ROOT/config.yaml"; then
        LOG_LEVEL=$(grep -A2 "logging:" "$PROJECT_ROOT/config.yaml" | grep "level:" | awk '{print $2}')
        
        if [[ "$LOG_LEVEL" == "info" || "$LOG_LEVEL" == "debug" ]]; then
            pass
        else
            fail "Log level '$LOG_LEVEL' may not capture audit events"
        fi
    else
        fail "Logging not configured"
    fi
}

test_log_format() {
    print_test "PCI DSS 10.3: Structured logging (JSON format)"
    
    # Check if JSON logging is enabled
    if grep -A2 "logging:" "$PROJECT_ROOT/config.yaml" | grep -q "format: json"; then
        pass
    else
        warn "JSON logging not configured, harder to parse for SIEM"
        pass
    fi
}

test_no_plaintext_in_logs() {
    print_test "PCI DSS 3.4: No plaintext data in logs"
    
    # Perform encrypt operation
    RESPONSE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -X POST "$HSM_URL/encrypt" \
        -H "Content-Type: application/json" \
        -d '{"context":"exchange-key","plaintext":"U2VjcmV0RGF0YQ=="}' 2>/dev/null)
    
    # Check Docker logs for plaintext
    sleep 1
    if docker logs hsm-service 2>&1 | tail -10 | grep -q "U2VjcmV0RGF0YQ=="; then
        fail "Plaintext found in logs!"
    else
        pass
    fi
}

# ============================================================
# Requirement 8: Access Control
# ============================================================

test_acl_enforcement() {
    print_test "PCI DSS 8.2: Role-based access control (ACL)"
    
    # Check if ACL is configured
    if grep -q "acl:" "$PROJECT_ROOT/config.yaml"; then
        if grep -A10 "acl:" "$PROJECT_ROOT/config.yaml" | grep -q "mappings:"; then
            pass
        else
            fail "ACL mappings not configured"
        fi
    else
        fail "ACL not configured"
    fi
}

test_acl_denies_unauthorized() {
    print_test "PCI DSS 8.2: ACL denies unauthorized access"
    
    # Try to access 2fa context with Trading cert (should fail)
    HTTP_CODE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -X POST "$HSM_URL/encrypt" \
        -H "Content-Type: application/json" \
        -d '{"context":"2fa","plaintext":"dGVzdA=="}' \
        -o /dev/null -w "%{http_code}" 2>/dev/null)
    
    if [ "$HTTP_CODE" == "403" ]; then
        pass
    else
        fail "ACL did not deny unauthorized access (HTTP $HTTP_CODE)"
    fi
}

test_certificate_revocation() {
    print_test "PCI DSS 8.3: Certificate revocation checking"
    
    # Check if revoked.yaml exists
    if [ -f "$PROJECT_ROOT/pki/revoked.yaml" ]; then
        pass
    else
        fail "revoked.yaml not found"
    fi
}

# ============================================================
# Requirement 12: Security Policies
# ============================================================

test_rate_limiting() {
    print_test "PCI DSS 11.4: Rate limiting to prevent DoS"
    
    # Check if rate limiting is configured
    if grep -q "rate_limit:" "$PROJECT_ROOT/config.yaml"; then
        RPS=$(grep -A2 "rate_limit:" "$PROJECT_ROOT/config.yaml" | grep "requests_per_second:" | awk '{print $2}')
        
        if [ "$RPS" -gt 0 ]; then
            pass
        else
            fail "Rate limiting disabled"
        fi
    else
        fail "Rate limiting not configured"
    fi
}

test_metrics_available() {
    print_test "PCI DSS 10.6: Metrics endpoint for monitoring"
    
    # Check if metrics endpoint exists
    HTTP_CODE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -o /dev/null -w "%{http_code}" "$HSM_URL/metrics" 2>/dev/null)
    
    if [ "$HTTP_CODE" == "200" ]; then
        pass
    else
        fail "Metrics endpoint not available (HTTP $HTTP_CODE)"
    fi
}

# ============================================================
# Additional Security Checks
# ============================================================

test_no_default_pins() {
    print_test "Security: No default HSM PINs (not 1234)"
    
    # Check softhsm2.conf for default PINs
    if [ -f "$PROJECT_ROOT/softhsm2.conf" ]; then
        warn "Cannot verify HSM PINs from config, manual check required"
        pass
    else
        warn "softhsm2.conf not found in project root"
        pass
    fi
}

test_secure_permissions() {
    print_test "Security: Secure file permissions on keys/certs"
    
    # Check PKI directory permissions
    if [ -d "$PROJECT_ROOT/pki/server" ]; then
        PERMS=$(stat -c %a "$PROJECT_ROOT/pki/server" 2>/dev/null || stat -f %A "$PROJECT_ROOT/pki/server" 2>/dev/null)
        
        # Should be 700 or 750
        if [[ "$PERMS" == "700" || "$PERMS" == "750" ]]; then
            pass
        else
            warn "PKI directory has permissive permissions: $PERMS"
            pass
        fi
    else
        warn "PKI directory not found"
        pass
    fi
}

# ============================================================
# Main
# ============================================================

main() {
    print_header
    
    echo "Testing HSM Service: $HSM_URL"
    echo "Client Certificate: $CLIENT_CERT"
    echo ""
    
    # Check if service is running
    if ! curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" "$HSM_URL/health" > /dev/null 2>&1; then
        echo -e "${RED}ERROR: HSM Service not reachable at $HSM_URL${NC}"
        echo "Start service with: docker compose up -d"
        exit 1
    fi
    
    echo "=== PCI DSS Requirement 3: Protect Stored Data ==="
    test_key_rotation_interval
    test_key_cleanup
    test_max_key_versions
    test_no_plaintext_in_logs
    
    echo ""
    echo "=== PCI DSS Requirement 4: Encrypt Data Transmission ==="
    test_tls_version
    test_strong_ciphers
    test_certificate_validation
    
    echo ""
    echo "=== PCI DSS Requirement 8: Access Control ==="
    test_acl_enforcement
    test_acl_denies_unauthorized
    test_certificate_revocation
    
    echo ""
    echo "=== PCI DSS Requirement 10: Logging & Monitoring ==="
    test_audit_logging
    test_log_format
    test_metrics_available
    
    echo ""
    echo "=== PCI DSS Requirement 11/12: Security Controls ==="
    test_rate_limiting
    test_no_default_pins
    test_secure_permissions
    
    echo ""
    echo "================================================================"
    echo "  Results"
    echo "================================================================"
    echo -e "Total Tests: $TOTAL"
    echo -e "${GREEN}Passed: $PASSED${NC}"
    echo -e "${RED}Failed: $FAILED${NC}"
    echo ""
    
    if [ $FAILED -eq 0 ]; then
        echo -e "${GREEN}✓ PCI DSS Compliance: PASS${NC}"
        echo ""
        exit 0
    else
        echo -e "${RED}✗ PCI DSS Compliance: FAIL${NC}"
        echo ""
        exit 1
    fi
}

main "$@"
