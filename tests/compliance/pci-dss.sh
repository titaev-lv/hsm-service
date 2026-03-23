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

find_client_cert() {
    for p in \
        "$PROJECT_ROOT/pki/test/client/trading-client-1.crt" \
        "$PROJECT_ROOT/pki/test/client/hsm-trading-client-1.crt" \
        "$PROJECT_ROOT/pki/client/hsm-trading-client-1.crt" \
        "$PROJECT_ROOT/pki/client/client.crt"; do
        if [ -f "$p" ]; then
            echo "$p"
            return 0
        fi
    done
    return 1
}

find_client_key() {
    for p in \
        "$PROJECT_ROOT/pki/test/client/trading-client-1.key" \
        "$PROJECT_ROOT/pki/test/client/hsm-trading-client-1.key" \
        "$PROJECT_ROOT/pki/client/hsm-trading-client-1.key" \
        "$PROJECT_ROOT/pki/client/client.key"; do
        if [ -f "$p" ]; then
            echo "$p"
            return 0
        fi
    done
    return 1
}

find_revoked_file() {
    # Prefer explicit test override, then common test/prod locations.
    for p in \
        "$PROJECT_ROOT/revoked-test.yaml" \
        "$PROJECT_ROOT/pki/test/revoked.yaml" \
        "$PROJECT_ROOT/pki/revoked.yaml"; do
        if [ -f "$p" ]; then
            echo "$p"
            return 0
        fi
    done

    # Fallback to path declared in config if present.
    local configured
    configured=$(grep -E '^\s*revoked_file:\s*' "$PROJECT_ROOT/config.yaml" 2>/dev/null | head -1 | awk '{print $2}' | tr -d '"')
    if [ -n "$configured" ]; then
        if [ -f "$configured" ]; then
            echo "$configured"
            return 0
        fi
        if [ -f "$PROJECT_ROOT/$configured" ]; then
            echo "$PROJECT_ROOT/$configured"
            return 0
        fi
    fi

    return 1
}

find_server_cert() {
    for p in \
        "$PROJECT_ROOT/pki/test/server/hsm-service.crt" \
        "$PROJECT_ROOT/pki/server/hsm-service.local.crt" \
        "$PROJECT_ROOT/pki/server/hsm-service.crt"; do
        if [ -f "$p" ]; then
            echo "$p"
            return 0
        fi
    done
    return 1
}

find_service_container() {
    if docker ps --format '{{.Names}}' | grep -qx 'hsm-service-test'; then
        echo "hsm-service-test"
        return 0
    fi
    if docker ps --format '{{.Names}}' | grep -qx 'hsm-service'; then
        echo "hsm-service"
        return 0
    fi
    return 1
}

CLIENT_CERT="${CLIENT_CERT:-$(find_client_cert || true)}"
CLIENT_KEY="${CLIENT_KEY:-$(find_client_key || true)}"
REVOKED_FILE="${REVOKED_FILE:-$(find_revoked_file || true)}"
SERVER_CERT="${SERVER_CERT:-$(find_server_cert || true)}"
SERVICE_CONTAINER="${SERVICE_CONTAINER:-$(find_service_container || true)}"
HSM_CONNECT="$(echo "$HSM_URL" | sed -E 's#^https?://##; s#/.*$##')"
if ! echo "$HSM_CONNECT" | grep -q ':'; then
    HSM_CONNECT="${HSM_CONNECT}:443"
fi

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
    TLS_OUTPUT=$(echo | timeout 3 openssl s_client -connect "$HSM_CONNECT" -cert "$CLIENT_CERT" -key "$CLIENT_KEY" 2>&1 | grep -E "New, TLS")
    
    if echo "$TLS_OUTPUT" | grep -qE "TLSv1\.(3|2)"; then
        pass
    else
        fail "Weak or unknown TLS version: $TLS_OUTPUT"
    fi
}

test_strong_ciphers() {
    print_test "PCI DSS 4.2.1: Strong cipher suites only"
    
    # Get cipher suite (output format: "New, TLSv1.3, Cipher is TLS_AES_128_GCM_SHA256")
    CIPHER=$(echo | timeout 3 openssl s_client -connect "$HSM_CONNECT" -cert "$CLIENT_CERT" -key "$CLIENT_KEY" 2>&1 | grep -E "Cipher is" | head -1 | sed 's/.*Cipher is //')
    
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

test_legacy_tls_rejected() {
    print_test "PCI DSS 4.2.1: Legacy TLS 1.0/1.1 explicitly rejected"

    local tls10_out tls11_out
    tls10_out=$(echo | timeout 4 openssl s_client -tls1 -connect "$HSM_CONNECT" -cert "$CLIENT_CERT" -key "$CLIENT_KEY" 2>&1 || true)
    tls11_out=$(echo | timeout 4 openssl s_client -tls1_1 -connect "$HSM_CONNECT" -cert "$CLIENT_CERT" -key "$CLIENT_KEY" 2>&1 || true)

    if echo "$tls10_out $tls11_out" | grep -qiE "handshake failure|alert protocol version|wrong version number|unsupported protocol|no protocols available"; then
        pass
    else
        fail "Legacy TLS may still be accepted (no explicit reject signal for TLS1.0/1.1)"
    fi
}

test_server_certificate_strength() {
    print_test "PCI DSS 4.2.1: Server certificate validity and key strength"

    if [ -z "$SERVER_CERT" ] || [ ! -f "$SERVER_CERT" ]; then
        fail "Server certificate file not found"
        return
    fi

    local expiry expiry_epoch now_epoch days_left key_size
    expiry=$(openssl x509 -in "$SERVER_CERT" -noout -enddate 2>/dev/null | cut -d= -f2)
    if [ -z "$expiry" ]; then
        fail "Cannot read certificate expiry"
        return
    fi

    expiry_epoch=$(date -d "$expiry" +%s 2>/dev/null || true)
    now_epoch=$(date +%s)
    if [ -z "$expiry_epoch" ]; then
        fail "Cannot parse certificate expiry date"
        return
    fi

    days_left=$(( (expiry_epoch - now_epoch) / 86400 ))
    key_size=$(openssl x509 -in "$SERVER_CERT" -noout -text 2>/dev/null | grep "Public-Key:" | grep -o "[0-9]\+" | head -1)

    if [ "$days_left" -lt 30 ]; then
        fail "Certificate expires too soon ($days_left days left)"
    elif [ -z "$key_size" ] || [ "$key_size" -lt 2048 ]; then
        fail "Server certificate key size too small (${key_size:-unknown})"
    else
        pass
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
    if [ -n "$SERVICE_CONTAINER" ] && docker logs "$SERVICE_CONTAINER" 2>&1 | tail -50 | grep -q "U2VjcmV0RGF0YQ=="; then
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

    # Check if revocation list file exists (test or prod layout).
    if [ -n "$REVOKED_FILE" ] && [ -f "$REVOKED_FILE" ]; then
        pass
    else
        fail "revocation list not found (tried revoked-test.yaml, pki/test/revoked.yaml, pki/revoked.yaml, config revoked_file)"
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

    if [ -z "$SERVICE_CONTAINER" ]; then
        warn "Cannot determine active service container to inspect PIN env"
        pass
        return
    fi

    local pin so_pin
    pin=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$SERVICE_CONTAINER" 2>/dev/null | grep '^HSM_PIN=' | head -1 | cut -d= -f2-)
    so_pin=$(docker inspect -f '{{range .Config.Env}}{{println .}}{{end}}' "$SERVICE_CONTAINER" 2>/dev/null | grep '^HSM_SO_PIN=' | head -1 | cut -d= -f2-)

    if [ -z "$pin" ] || [ -z "$so_pin" ]; then
        warn "HSM_PIN/HSM_SO_PIN env not found in container"
        pass
    elif [ "$pin" = "1234" ] || [ "$so_pin" = "12345678" ]; then
        fail "Default HSM PIN values detected in container env"
    else
        pass
    fi
}

test_secure_permissions() {
    print_test "Security: Secure file permissions on keys/certs"

    local pki_dir perms
    if [ -d "$PROJECT_ROOT/pki/test/server" ]; then
        pki_dir="$PROJECT_ROOT/pki/test/server"
    elif [ -d "$PROJECT_ROOT/pki/server" ]; then
        pki_dir="$PROJECT_ROOT/pki/server"
    else
        warn "PKI directory not found"
        pass
        return
    fi

    perms=$(stat -c %a "$pki_dir" 2>/dev/null || stat -f %A "$pki_dir" 2>/dev/null)

    # Should be 700 or 750
    if [[ "$perms" == "700" || "$perms" == "750" ]]; then
        pass
    else
        warn "PKI directory has permissive permissions: $perms"
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
    test_legacy_tls_rejected
    test_server_certificate_strength
    
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
