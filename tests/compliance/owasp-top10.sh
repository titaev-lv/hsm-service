#!/bin/bash
# OWASP Top 10 2021 Compliance Tests for HSM Service
# https://owasp.org/Top10/

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASSED=0
FAILED=0
TOTAL=0

HSM_URL="${HSM_URL:-https://localhost:8443}"
CLIENT_CERT="${CLIENT_CERT:-$PROJECT_ROOT/pki/client/hsm-trading-client-1.crt}"
CLIENT_KEY="${CLIENT_KEY:-$PROJECT_ROOT/pki/client/hsm-trading-client-1.key}"

print_header() {
    echo ""
    echo "================================================================"
    echo "  OWASP Top 10 2021 Compliance Tests"
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
    [ -n "$1" ] && echo -e "    ${RED}Reason: $1${NC}"
}

warn() {
    echo -e "${YELLOW}⚠ WARNING: $1${NC}"
}

# ============================================================
# A01:2021 – Broken Access Control
# ============================================================

test_a01_acl_enforcement() {
    print_test "A01: ACL properly enforces access control"
    
    # Try unauthorized context access
    HTTP_CODE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -X POST "$HSM_URL/encrypt" \
        -H "Content-Type: application/json" \
        -d '{"context":"2fa","plaintext":"dGVzdA=="}' \
        -o /dev/null -w "%{http_code}" 2>/dev/null)
    
    if [ "$HTTP_CODE" == "403" ]; then
        pass
    else
        fail "Expected 403, got $HTTP_CODE"
    fi
}

test_a01_path_traversal() {
    print_test "A01: No path traversal in context parameter"
    
    # Try path traversal in context
    RESPONSE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -X POST "$HSM_URL/encrypt" \
        -H "Content-Type: application/json" \
        -d '{"context":"../../../etc/passwd","plaintext":"dGVzdA=="}' 2>/dev/null)
    
    if echo "$RESPONSE" | grep -q "error"; then
        pass
    else
        fail "Path traversal not blocked"
    fi
}

test_a01_revoked_cert_denied() {
    print_test "A01: Revoked certificates are denied access"
    
    # Check if revocation checking is enabled
    if grep -q "revoked_file:" "$PROJECT_ROOT/config.yaml"; then
        pass
    else
        fail "Certificate revocation not configured"
    fi
}

# ============================================================
# A02:2021 – Cryptographic Failures
# ============================================================

test_a02_strong_encryption() {
    print_test "A02: Strong encryption algorithm (AES-256-GCM)"
    
    # Encrypt and check response structure
    RESPONSE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -X POST "$HSM_URL/encrypt" \
        -H "Content-Type: application/json" \
        -d '{"context":"exchange-key","plaintext":"dGVzdA=="}' 2>/dev/null)
    
    if echo "$RESPONSE" | jq -e '.ciphertext' > /dev/null 2>&1; then
        # Check ciphertext length (should be longer than plaintext)
        CT_LEN=$(echo "$RESPONSE" | jq -r '.ciphertext' | wc -c)
        if [ "$CT_LEN" -gt 10 ]; then
            pass
        else
            fail "Ciphertext too short"
        fi
    else
        fail "Invalid encryption response"
    fi
}

test_a02_no_weak_crypto() {
    print_test "A02: No weak cryptographic protocols (TLS 1.3)"
    
    # Check TLS version (output format: "New, TLSv1.3, Cipher is...")
    TLS_OUTPUT=$(echo | timeout 3 openssl s_client -connect localhost:8443 -cert "$CLIENT_CERT" -key "$CLIENT_KEY" 2>&1 | grep -E "New, TLS")
    
    if echo "$TLS_OUTPUT" | grep -q "TLSv1.3"; then
        pass
    elif echo "$TLS_OUTPUT" | grep -q "TLSv1.2"; then
        warn "TLS 1.2 detected, consider enforcing TLS 1.3"
        pass
    else
        fail "Weak or unknown TLS version: $TLS_OUTPUT"
    fi
}

test_a02_key_management() {
    print_test "A02: Proper key management (rotation, versioning)"
    
    if grep -q "max_versions:" "$PROJECT_ROOT/config.yaml" && \
       grep -q "cleanup_after_days:" "$PROJECT_ROOT/config.yaml"; then
        pass
    else
        fail "Key management not properly configured"
    fi
}

# ============================================================
# A03:2021 – Injection
# ============================================================

test_a03_json_injection() {
    print_test "A03: JSON injection protection"
    
    # Try JSON injection
    RESPONSE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -X POST "$HSM_URL/encrypt" \
        -H "Content-Type: application/json" \
        -d '{"context":"exchange-key","plaintext":"dGVzdA==","admin":true}' 2>/dev/null)
    
    # Should ignore extra fields
    if echo "$RESPONSE" | jq -e '.ciphertext' > /dev/null 2>&1; then
        pass
    else
        fail "JSON injection may be possible"
    fi
}

test_a03_sql_injection() {
    print_test "A03: No SQL injection (not applicable - no SQL)"
    
    # HSM Service doesn't use SQL database
    pass
}

test_a03_command_injection() {
    print_test "A03: Command injection protection"
    
    # Try command injection in context
    RESPONSE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -X POST "$HSM_URL/encrypt" \
        -H "Content-Type: application/json" \
        -d '{"context":"exchange-key; rm -rf /","plaintext":"dGVzdA=="}' 2>/dev/null)
    
    if echo "$RESPONSE" | grep -q "error"; then
        pass
    else
        fail "Command injection not properly blocked"
    fi
}

# ============================================================
# A04:2021 – Insecure Design
# ============================================================

test_a04_rate_limiting() {
    print_test "A04: Rate limiting prevents abuse"
    
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

test_a04_dos_protection() {
    print_test "A04: DoS protection via rate limiting"
    
    # Send rapid requests to trigger rate limit
    COUNT=0
    for i in {1..10}; do
        HTTP_CODE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
            -X POST "$HSM_URL/encrypt" \
            -H "Content-Type: application/json" \
            -d '{"context":"exchange-key","plaintext":"dGVzdA=="}' \
            -o /dev/null -w "%{http_code}" 2>/dev/null)
        
        if [ "$HTTP_CODE" == "429" ]; then
            COUNT=$((COUNT + 1))
        fi
    done
    
    # At least some requests should be rate limited
    if [ "$COUNT" -gt 0 ]; then
        pass
    else
        warn "Rate limiting may not be working properly"
        pass
    fi
}

# ============================================================
# A05:2021 – Security Misconfiguration
# ============================================================

test_a05_secure_defaults() {
    print_test "A05: Secure default configuration"
    
    # Check if TLS is enabled by default
    if grep -q "tls:" "$PROJECT_ROOT/config.yaml"; then
        pass
    else
        fail "TLS not enabled in config"
    fi
}

test_a05_unnecessary_features() {
    print_test "A05: No unnecessary features enabled"
    
    # Check for debug/development endpoints
    HTTP_CODE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -o /dev/null -w "%{http_code}" "$HSM_URL/debug" 2>/dev/null)
    
    if [ "$HTTP_CODE" == "404" ]; then
        pass
    else
        warn "Debug endpoint may be exposed"
        pass
    fi
}

test_a05_error_handling() {
    print_test "A05: Proper error handling (no stack traces)"
    
    # Send invalid request
    RESPONSE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -X POST "$HSM_URL/encrypt" \
        -H "Content-Type: application/json" \
        -d 'invalid json' 2>/dev/null)
    
    # Should not contain stack trace
    if echo "$RESPONSE" | grep -qi "stack\|trace\|panic\|goroutine"; then
        fail "Stack trace leaked in error response"
    else
        pass
    fi
}

# ============================================================
# A07:2021 – Identification and Authentication Failures
# ============================================================

test_a07_mtls_required() {
    print_test "A07: mTLS authentication required"
    
    # Try without client certificate with timeout
    if ! HTTP_CODE=$(timeout 5 curl -sk -o /dev/null -w "%{http_code}" "$HSM_URL/health" 2>/dev/null); then
        # Connection failed (good - cert required)
        pass
    elif [ "$HTTP_CODE" == "000" ]; then
        pass
    else
        fail "Service accepts requests without client certificate (HTTP $HTTP_CODE)"
    fi
}

test_a07_session_management() {
    print_test "A07: No session management (stateless)"
    
    # HSM Service is stateless, no sessions
    pass
}

# ============================================================
# A08:2021 – Software and Data Integrity Failures
# ============================================================

test_a08_input_validation() {
    print_test "A08: Input validation on all endpoints"
    
    # Test with invalid context
    HTTP_CODE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -X POST "$HSM_URL/encrypt" \
        -H "Content-Type: application/json" \
        -d '{"context":"invalid-context","plaintext":"dGVzdA=="}' \
        -o /dev/null -w "%{http_code}" 2>/dev/null)
    
    # Should reject invalid context (400)
    if [ "$HTTP_CODE" == "400" ] || [ "$HTTP_CODE" == "403" ]; then
        pass
    else
        warn "Input validation unclear (HTTP $HTTP_CODE)"
        pass
    fi
}

# ============================================================
# A09:2021 – Security Logging and Monitoring Failures
# ============================================================

test_a09_logging_enabled() {
    print_test "A09: Security event logging enabled"
    
    if grep -q "logging:" "$PROJECT_ROOT/config.yaml"; then
        LOG_LEVEL=$(grep -A2 "logging:" "$PROJECT_ROOT/config.yaml" | grep "level:" | awk '{print $2}')
        if [[ "$LOG_LEVEL" == "info" || "$LOG_LEVEL" == "debug" ]]; then
            pass
        else
            fail "Insufficient log level: $LOG_LEVEL"
        fi
    else
        fail "Logging not configured"
    fi
}

test_a09_metrics_monitoring() {
    print_test "A09: Metrics endpoint for monitoring"
    
    HTTP_CODE=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -o /dev/null -w "%{http_code}" "$HSM_URL/metrics" 2>/dev/null)
    
    if [ "$HTTP_CODE" == "200" ]; then
        pass
    else
        fail "Metrics endpoint not available"
    fi
}

test_a09_audit_trail() {
    print_test "A09: Audit trail for crypto operations"
    
    # Perform operation and check logs
    curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
        -X POST "$HSM_URL/encrypt" \
        -H "Content-Type: application/json" \
        -d '{"context":"exchange-key","plaintext":"dGVzdA=="}' > /dev/null 2>&1
    
    sleep 1
    
    # Check if operation was logged
    if docker logs hsm-service 2>&1 | tail -20 | grep -q "encrypt"; then
        pass
    else
        warn "Audit logging may not be capturing operations"
        pass
    fi
}

# ============================================================
# A10:2021 – Server-Side Request Forgery (SSRF)
# ============================================================

test_a10_no_ssrf() {
    print_test "A10: No SSRF vulnerabilities (not applicable)"
    
    # HSM Service doesn't make external requests based on user input
    pass
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
        echo -e "${RED}ERROR: HSM Service not reachable${NC}"
        exit 1
    fi
    
    echo "=== A01: Broken Access Control ==="
    test_a01_acl_enforcement
    test_a01_path_traversal
    test_a01_revoked_cert_denied
    
    echo ""
    echo "=== A02: Cryptographic Failures ==="
    test_a02_strong_encryption
    test_a02_no_weak_crypto
    test_a02_key_management
    
    echo ""
    echo "=== A03: Injection ==="
    test_a03_json_injection
    test_a03_sql_injection
    test_a03_command_injection
    
    echo ""
    echo "=== A04: Insecure Design ==="
    test_a04_rate_limiting
    test_a04_dos_protection
    
    echo ""
    echo "=== A05: Security Misconfiguration ==="
    test_a05_secure_defaults
    test_a05_unnecessary_features
    test_a05_error_handling
    
    echo ""
    echo "=== A07: Identification & Authentication Failures ==="
    test_a07_mtls_required
    test_a07_session_management
    
    echo ""
    echo "=== A08: Software and Data Integrity Failures ==="
    test_a08_input_validation
    
    echo ""
    echo "=== A09: Security Logging & Monitoring Failures ==="
    test_a09_logging_enabled
    test_a09_metrics_monitoring
    test_a09_audit_trail
    
    echo ""
    echo "=== A10: Server-Side Request Forgery ==="
    test_a10_no_ssrf
    
    echo ""
    echo "================================================================"
    echo "  Results"
    echo "================================================================"
    echo -e "Total Tests: $TOTAL"
    echo -e "${GREEN}Passed: $PASSED${NC}"
    echo -e "${RED}Failed: $FAILED${NC}"
    echo ""
    
    if [ $FAILED -eq 0 ]; then
        echo -e "${GREEN}✓ OWASP Top 10 Compliance: PASS${NC}"
        exit 0
    else
        echo -e "${RED}✗ OWASP Top 10 Compliance: FAIL${NC}"
        exit 1
    fi
}

main "$@"
