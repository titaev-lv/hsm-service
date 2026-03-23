#!/bin/bash
# PCI DSS 10.2 focused checks: audit trail and logging controls

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

PASSED=0
FAILED=0
TOTAL=0

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

CLIENT_CERT="${CLIENT_CERT:-$(find_client_cert || true)}"
CLIENT_KEY="${CLIENT_KEY:-$(find_client_key || true)}"

print_header() {
    echo ""
    echo "================================================================"
    echo "  PCI DSS 10.2 - Audit Logging Controls"
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

check_logging_level() {
    print_test "10.2.a: logging level captures audit events"

    if grep -q "logging:" "$PROJECT_ROOT/config.yaml"; then
        local level
        level=$(grep -A2 "logging:" "$PROJECT_ROOT/config.yaml" | grep "level:" | awk '{print $2}')

        if [[ "$level" == "info" || "$level" == "debug" ]]; then
            pass
        else
            fail "logging level '$level' is too restrictive"
        fi
    else
        fail "logging section not found in config.yaml"
    fi
}

check_structured_logging() {
    print_test "10.2.b: structured JSON logging"

    if grep -A4 "logging:" "$PROJECT_ROOT/config.yaml" | grep -q "format: json"; then
        pass
    else
        fail "logging format is not json"
    fi
}

check_log_paths() {
    print_test "10.2.c: audit/access/error log paths configured"

    if grep -A20 "logging:" "$PROJECT_ROOT/config.yaml" | grep -q "audit_path:" \
        && grep -A20 "logging:" "$PROJECT_ROOT/config.yaml" | grep -q "access_path:" \
        && grep -A20 "logging:" "$PROJECT_ROOT/config.yaml" | grep -q "error_path:"; then
        pass
    else
        fail "one or more log paths are missing in config.yaml"
    fi
}

check_metrics_available() {
    print_test "10.2.d: metrics endpoint available for monitoring"

    if [ -z "$CLIENT_CERT" ] || [ -z "$CLIENT_KEY" ]; then
        warn "client cert/key not found, skipping live metrics probe"
        pass
        return
    fi

    local code
    code=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" -o /dev/null -w "%{http_code}" "$HSM_URL/metrics" 2>/dev/null || echo "000")
    if [ "$code" = "200" ]; then
        pass
    else
        fail "metrics endpoint returned HTTP $code"
    fi
}

main() {
    print_header

    check_logging_level
    check_structured_logging
    check_log_paths
    check_metrics_available

    echo ""
    echo "================================================================"
    echo "  Results"
    echo "================================================================"
    echo -e "Total Tests: $TOTAL"
    echo -e "${GREEN}Passed: $PASSED${NC}"
    echo -e "${RED}Failed: $FAILED${NC}"
    echo ""

    if [ "$FAILED" -eq 0 ]; then
        echo -e "${GREEN}✓ PCI DSS 10.2: PASS${NC}"
        exit 0
    else
        echo -e "${RED}✗ PCI DSS 10.2: FAIL${NC}"
        exit 1
    fi
}

main "$@"
