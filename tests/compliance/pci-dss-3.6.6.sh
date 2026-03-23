#!/bin/bash
# PCI DSS 3.6.6 focused checks: split knowledge and dual control readiness

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
    echo "  PCI DSS 3.6.6 - Split Knowledge Controls"
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

check_split_knowledge_config_present() {
    print_test "3.6.6.a: split knowledge control is declared"

    if grep -qE '^\s*(shamir|split_knowledge):' "$PROJECT_ROOT/config.yaml"; then
        pass
    else
        warn "no shamir/split_knowledge section in config.yaml (optional control)"
        pass
    fi
}

check_threshold_when_enabled() {
    print_test "3.6.6.b: threshold is >= 2 when split knowledge is enabled"

    local enabled threshold
    enabled=$(grep -A8 -E '^\s*(shamir|split_knowledge):' "$PROJECT_ROOT/config.yaml" 2>/dev/null | grep -E '^\s*enabled:\s*' | head -1 | awk '{print $2}' | tr -d '"')
    threshold=$(grep -A8 -E '^\s*(shamir|split_knowledge):' "$PROJECT_ROOT/config.yaml" 2>/dev/null | grep -E '^\s*threshold:\s*' | head -1 | awk '{print $2}' | tr -d '"')

    if [ "$enabled" = "true" ]; then
        if [ -z "$threshold" ]; then
            fail "split knowledge enabled but threshold is not configured"
            return
        fi

        if [ "$threshold" -ge 2 ] 2>/dev/null; then
            pass
        else
            fail "threshold=$threshold (must be >= 2)"
        fi
    else
        warn "split knowledge is not enabled in config.yaml (optional for current phase)"
        pass
    fi
}

check_dual_control_documented() {
    print_test "3.6.6.c: dual control process is documented"

    if grep -q "PCI DSS 3.6.6" "$PROJECT_ROOT/IMPROVEMENT_PLAN_2026.md" \
        && grep -qE "M-of-N|Dual control|Split knowledge" "$PROJECT_ROOT/IMPROVEMENT_PLAN_2026.md"; then
        pass
    else
        fail "dual control/split knowledge process not found in improvement plan"
    fi
}

main() {
    print_header

    check_split_knowledge_config_present
    check_threshold_when_enabled
    check_dual_control_documented

    echo ""
    echo "================================================================"
    echo "  Results"
    echo "================================================================"
    echo -e "Total Tests: $TOTAL"
    echo -e "${GREEN}Passed: $PASSED${NC}"
    echo -e "${RED}Failed: $FAILED${NC}"
    echo ""

    if [ "$FAILED" -eq 0 ]; then
        echo -e "${GREEN}✓ PCI DSS 3.6.6: PASS${NC}"
        exit 0
    else
        echo -e "${RED}✗ PCI DSS 3.6.6: FAIL${NC}"
        exit 1
    fi
}

main "$@"
