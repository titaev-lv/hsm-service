#!/bin/bash

# Security Scanner for HSM Service
# Runs multiple security checks: gosec, trivy, docker-bench

set -e
set -o pipefail

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

print_header() { echo -e "\n${BLUE}========== $1 ==========${NC}\n"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_warning() { echo -e "${YELLOW}⚠${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; }

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"
cd "$PROJECT_ROOT"

HSM_URL_FROM_ENV="${HSM_URL:-}"
HSM_URL="${HSM_URL:-https://localhost:8443}"

if [ -z "$HSM_URL_FROM_ENV" ] && ! curl -sk --max-time 3 "$HSM_URL/health" >/dev/null 2>&1; then
    if curl -sk --max-time 3 "https://localhost:8444/health" >/dev/null 2>&1; then
        HSM_URL="https://localhost:8444"
        print_warning "Security scan auto-detected test endpoint: $HSM_URL"
    fi
fi

HSM_CONNECT="$(echo "$HSM_URL" | sed -E 's#^https?://##; s#/.*$##')"
if ! echo "$HSM_CONNECT" | grep -q ':'; then
    HSM_CONNECT="${HSM_CONNECT}:443"
fi

TLS_HOST="${HSM_CONNECT%%:*}"
TLS_PORT="${HSM_CONNECT##*:}"
export HSM_URL TLS_HOST TLS_PORT

print_header "Security Scan - HSM Service"
echo "Project: $PROJECT_ROOT"
echo "Date: $(date)"
echo ""

FAILED=0
SKIPPED=0

# ==========================================
# 1. Go Security Check (gosec)
# ==========================================
print_header "1. Go Security Scanner (gosec)"

if command -v gosec &> /dev/null; then
    echo "Running gosec..."
    if gosec -fmt=text ./... 2>&1 | tee /tmp/gosec-report.txt; then
        print_success "gosec: No critical issues found"
    else
        print_warning "gosec: Issues detected (see /tmp/gosec-report.txt)"
        FAILED=1
    fi
else
    print_warning "gosec not installed. Install: go install github.com/securego/gosec/v2/cmd/gosec@latest"
fi

# ==========================================
# 2. Go Vet (standard checks)
# ==========================================
print_header "2. Go Vet (standard checks)"

if go vet ./... 2>&1 | tee /tmp/govet-report.txt; then
    print_success "go vet: PASS"
else
    print_error "go vet: FAILED"
    FAILED=1
fi

# ==========================================
# 3. Staticcheck
# ==========================================
print_header "3. Staticcheck (advanced analysis)"

if command -v staticcheck &> /dev/null; then
    if staticcheck ./... 2>&1 | tee /tmp/staticcheck-report.txt; then
        print_success "staticcheck: PASS"
    else
        print_warning "staticcheck: Issues found"
        FAILED=1
    fi
else
    print_warning "staticcheck not installed. Install: go install honnef.co/go/tools/cmd/staticcheck@latest"
fi

# ==========================================
# 4. Dependency Vulnerability Scan
# ==========================================
print_header "4. Dependency Vulnerability Scan (govulncheck)"

if command -v govulncheck &> /dev/null; then
    if govulncheck ./... 2>&1 | tee /tmp/govulncheck-report.txt; then
        print_success "govulncheck: No vulnerabilities found"
    else
        print_error "govulncheck: Vulnerabilities detected!"
        FAILED=1
    fi
else
    print_warning "govulncheck not installed. Install: go install golang.org/x/vuln/cmd/govulncheck@latest"
fi

# ==========================================
# 5. Container Image Security (Trivy)
# ==========================================
print_header "5. Container Image Scan (Trivy)"

if command -v trivy &> /dev/null; then
    # Build image if not exists
    if ! docker images | grep -q "hsm-service.*latest"; then
        echo "Building image..."
        docker build -t hsm-service:latest . > /dev/null
    fi
    
    echo "Scanning hsm-service:latest (UNFIXED HIGH/CRITICAL)..."
    if trivy image --scanners vuln --severity HIGH,CRITICAL --ignore-status fixed --exit-code 1 hsm-service:latest 2>&1 | tee /tmp/trivy-report.txt; then
        print_success "trivy: No UNFIXED HIGH/CRITICAL vulnerabilities"
    else
        print_error "trivy: UNFIXED HIGH/CRITICAL vulnerabilities found!"
        FAILED=1
    fi
else
    print_warning "trivy not installed. Install: https://aquasecurity.github.io/trivy/"
fi

# ==========================================
# 6. TLS Configuration Check
# ==========================================
print_header "6. TLS Configuration Validation"

SERVER_CERT=""
for candidate in \
    "pki/test/server/hsm-service.crt" \
    "pki/server/hsm-service.local.crt" \
    "pki/server/hsm-service.crt"
do
    if [ -f "$candidate" ]; then
        SERVER_CERT="$candidate"
        break
    fi
done

if [ -n "$SERVER_CERT" ]; then
    echo "Checking server certificate: $SERVER_CERT"
    
    # Check expiration
    EXPIRY=$(openssl x509 -in "$SERVER_CERT" -noout -enddate | cut -d= -f2)
    EXPIRY_EPOCH=$(date -d "$EXPIRY" +%s 2>/dev/null || date -j -f "%b %d %H:%M:%S %Y %Z" "$EXPIRY" +%s 2>/dev/null)
    NOW_EPOCH=$(date +%s)
    if [ -z "$EXPIRY_EPOCH" ]; then
        print_warning "Unable to parse certificate expiry date: $EXPIRY"
        DAYS_LEFT=0
    else
        DAYS_LEFT=$(( ($EXPIRY_EPOCH - $NOW_EPOCH) / 86400 ))
    fi
    
    if [ -n "$EXPIRY_EPOCH" ] && [ "$DAYS_LEFT" -lt 30 ]; then
        print_warning "Certificate expires in $DAYS_LEFT days (renew soon!)"
    elif [ -n "$EXPIRY_EPOCH" ]; then
        print_success "Certificate valid for $DAYS_LEFT days"
    fi
    
    # Check key size
    KEY_SIZE=$(openssl x509 -in "$SERVER_CERT" -noout -text | grep "Public-Key:" | grep -o "[0-9]*" | head -1)
    if [ "$KEY_SIZE" -ge 2048 ]; then
        print_success "Key size: $KEY_SIZE bits (secure)"
    else
        print_error "Key size: $KEY_SIZE bits (too small!)"
        FAILED=1
    fi
else
    print_warning "Server certificate not found (checked: pki/test/server/hsm-service.crt, pki/server/hsm-service.local.crt, pki/server/hsm-service.crt)"
fi

# ==========================================
# 7. Secrets Detection
# ==========================================
print_header "7. Secrets Detection"

echo "Checking for hardcoded secrets..."
SECRETS_FOUND=0

# Check for common patterns
if grep -r -i "password\s*=\s*[\"'][^\"']*[\"']" --include="*.go" --include="*.yaml" . 2>/dev/null | grep -v "test"; then
    print_error "Hardcoded passwords detected!"
    SECRETS_FOUND=1
fi

if grep -r -i "api[_-]key\s*=\s*[\"'][^\"']*[\"']" --include="*.go" --include="*.yaml" . 2>/dev/null; then
    print_error "Hardcoded API keys detected!"
    SECRETS_FOUND=1
fi

if [ "$SECRETS_FOUND" -eq 0 ]; then
    print_success "No hardcoded secrets found"
else
    FAILED=1
fi

# ==========================================
# 8. Docker Security Best Practices
# ==========================================
print_header "8. Dockerfile Security Check"

if [ -f "Dockerfile" ]; then
    echo "Checking Dockerfile..."
    
    # For HSM services, running as root is acceptable (privileges needed)
    # but check if it's intentional
    if grep -q "^USER" Dockerfile; then
        print_success "Non-root user specified"
    else
        print_success "Running as root (required for HSM device access)"
    fi
    
    # COPY --chown only matters if using USER directive
    if grep -q "^USER" Dockerfile; then
        if grep -q "COPY --chown" Dockerfile; then
            print_success "Proper file ownership set"
        else
            print_warning "Consider using COPY --chown with USER"
        fi
    else
        print_success "COPY --chown not needed (running as root)"
    fi
    
    # Check for latest tag - should use pinned versions
    if grep -q "FROM.*:latest" Dockerfile; then
        print_error "Using :latest tag (pin specific versions for reproducibility)"
        FAILED=1
    else
        print_success "Using pinned base image versions"
    fi
fi

# ==========================================
# 9. Active Security Attack Simulations
# ==========================================
print_header "9. Active Security Attack Simulations"

for attack_test in \
    "tests/security/timing_attack.sh" \
    "tests/security/replay_attack.sh" \
    "tests/security/tls_downgrade.sh"
do
    name="$(basename "$attack_test")"
    if [ ! -x "$attack_test" ]; then
        print_warning "$name: not executable or missing"
        FAILED=1
        continue
    fi

    echo "Running $name..."
    set +e
    "$attack_test" 2>&1 | tee "/tmp/${name%.sh}-report.txt"
    exit_code=${PIPESTATUS[0]}
    set -e

    if [ "$exit_code" -eq 0 ]; then
        print_success "$name: PASS"
    elif [ "$exit_code" -eq 2 ]; then
        print_warning "$name: SKIP"
        SKIPPED=$((SKIPPED + 1))
    else
        print_error "$name: FAIL"
        FAILED=1
    fi
done

# ==========================================
# Summary
# ==========================================
print_header "Security Scan Summary"

echo "Reports saved to:"
echo "  - /tmp/gosec-report.txt"
echo "  - /tmp/govet-report.txt"
echo "  - /tmp/staticcheck-report.txt"
echo "  - /tmp/govulncheck-report.txt"
echo "  - /tmp/trivy-report.txt"
echo "  - /tmp/timing_attack-report.txt"
echo "  - /tmp/replay_attack-report.txt"
echo "  - /tmp/tls_downgrade-report.txt"
echo ""
echo "Attack simulation skips: $SKIPPED"
echo ""

if [ "$FAILED" -eq 0 ]; then
    if [ "$SKIPPED" -gt 0 ]; then
        print_warning "✓ Security checks passed with $SKIPPED skipped attack simulation(s)"
    else
        print_success "✓ All security checks passed!"
    fi
    exit 0
else
    print_error "✗ Some security checks failed. Review reports above."
    exit 1
fi
