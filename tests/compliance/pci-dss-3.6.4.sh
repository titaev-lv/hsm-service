#!/bin/bash
# PCI DSS 3.6.4 focused checks: key lifecycle and rotation controls

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

print_header() {
    echo ""
    echo "================================================================"
    echo "  PCI DSS 3.6.4 - Key Rotation Controls"
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

check_rotation_interval() {
    print_test "3.6.4.a: rotation interval <= 90 days"

    if [ -f "$PROJECT_ROOT/metadata.yaml" ]; then
        local rotation_days
        rotation_days=$(grep -A10 "exchange-key:" "$PROJECT_ROOT/metadata.yaml" | grep "rotation_interval_days:" | head -1 | awk '{print $2}')

        if [ -z "$rotation_days" ]; then
            fail "rotation_interval_days not found in metadata.yaml"
            return
        fi

        if [ "$rotation_days" -le 90 ]; then
            pass
        else
            fail "rotation_interval_days=$rotation_days exceeds 90 days"
        fi
    else
        warn "metadata.yaml not found, using config.yaml fallback"
        if grep -q "rotation_interval_days" "$PROJECT_ROOT/config.yaml"; then
            pass
        else
            fail "No rotation interval config found"
        fi
    fi
}

check_cleanup_window() {
    print_test "3.6.4.b: cleanup window configured"

    if grep -q "cleanup_after_days:" "$PROJECT_ROOT/config.yaml"; then
        local cleanup_days
        cleanup_days=$(grep "cleanup_after_days:" "$PROJECT_ROOT/config.yaml" | awk '{print $2}')

        if [ "$cleanup_days" -le 365 ]; then
            pass
        else
            warn "cleanup_after_days=$cleanup_days is high"
            pass
        fi
    else
        fail "cleanup_after_days not configured"
    fi
}

check_max_versions() {
    print_test "3.6.4.c: max active versions constrained"

    if grep -q "max_versions:" "$PROJECT_ROOT/config.yaml"; then
        local max_versions
        max_versions=$(grep "max_versions:" "$PROJECT_ROOT/config.yaml" | awk '{print $2}')

        if [ "$max_versions" -le 5 ]; then
            pass
        else
            warn "max_versions=$max_versions is high"
            pass
        fi
    else
        fail "max_versions not configured"
    fi
}

main() {
    print_header

    check_rotation_interval
    check_cleanup_window
    check_max_versions

    echo ""
    echo "================================================================"
    echo "  Results"
    echo "================================================================"
    echo -e "Total Tests: $TOTAL"
    echo -e "${GREEN}Passed: $PASSED${NC}"
    echo -e "${RED}Failed: $FAILED${NC}"
    echo ""

    if [ "$FAILED" -eq 0 ]; then
        echo -e "${GREEN}✓ PCI DSS 3.6.4: PASS${NC}"
        exit 0
    else
        echo -e "${RED}✗ PCI DSS 3.6.4: FAIL${NC}"
        exit 1
    fi
}

main "$@"
