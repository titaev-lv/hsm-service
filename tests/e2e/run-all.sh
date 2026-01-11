#!/bin/bash

# E2E Test Suite Runner
# Runs all end-to-end scenario tests

set -e

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_header() { echo -e "\n${BLUE}========== $1 ==========${NC}\n"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

print_header "E2E Test Suite - HSM Service"
echo "Date: $(date)"
echo "Location: $SCRIPT_DIR"
echo ""

PASSED=0
FAILED=0
SKIPPED=0

# Function to run a test
run_test() {
    local test_name="$1"
    local test_script="$2"
    
    echo ""
    print_header "$test_name"
    
    if [ ! -f "$test_script" ]; then
        print_error "Test script not found: $test_script"
        SKIPPED=$((SKIPPED + 1))
        return 1
    fi
    
    if bash "$test_script" 2>&1 | tee "/tmp/e2e-${test_name}.log"; then
        print_success "$test_name PASSED"
        PASSED=$((PASSED + 1))
        return 0
    else
        print_error "$test_name FAILED (see /tmp/e2e-${test_name}.log)"
        FAILED=$((FAILED + 1))
        return 1
    fi
}

# ==========================================
# Run E2E Tests
# ==========================================

# Test 1: Key Rotation Workflow
run_test "Key Rotation" "scenarios/key-rotation-e2e.sh"

# Test 2: Disaster Recovery
run_test "Disaster Recovery" "scenarios/disaster-recovery.sh"

# Test 3: ACL Real-time Reload
run_test "ACL Real-time Reload" "scenarios/acl-realtime-reload.sh"

# ==========================================
# Summary
# ==========================================
print_header "E2E Test Suite Summary"

echo "Results:"
echo "  ✓ Passed:  $PASSED"
echo "  ✗ Failed:  $FAILED"
echo "  ⊘ Skipped: $SKIPPED"
echo ""

TOTAL=$((PASSED + FAILED + SKIPPED))
echo "Total: $TOTAL tests"
echo ""

if [ "$FAILED" -eq 0 ]; then
    print_success "✓ All E2E tests passed!"
    echo ""
    echo "Logs saved to:"
    ls -1 /tmp/e2e-*.log 2>/dev/null | sed 's/^/  - /'
    exit 0
else
    print_error "✗ Some E2E tests failed"
    echo ""
    echo "Failed test logs:"
    for test in Key-Rotation Disaster-Recovery ACL-Real-time-Reload; do
        if [ -f "/tmp/e2e-${test}.log" ]; then
            if ! grep -q "PASSED" "/tmp/e2e-${test}.log" 2>/dev/null; then
                echo "  - /tmp/e2e-${test}.log"
            fi
        fi
    done
    exit 1
fi
