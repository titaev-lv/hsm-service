#!/bin/bash
# Stress testing script for HSM Service
# Tests system under extreme load to find breaking points
#
# Requirements:
#   - vegeta: https://github.com/tsenart/vegeta
#   - Install: go install github.com/tsenart/vegeta@latest

set -euo pipefail

# Configuration
HSM_URL="${HSM_URL:-https://localhost:8443}"
RESULTS_DIR="${RESULTS_DIR:-./stress-results}"
DURATION="${DURATION:-60s}"
MAX_RATE="${MAX_RATE:-5000}" # requests/second

# Client certificates (mTLS required)
CLIENT_CERT="${CLIENT_CERT:-pki/client/hsm-trading-client-1.crt}"
CLIENT_KEY="${CLIENT_KEY:-pki/client/hsm-trading-client-1.key}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[!]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

# Create results directory
mkdir -p "$RESULTS_DIR"
log_info "Results will be saved to: $RESULTS_DIR"

# Check if vegeta is installed
if ! command -v vegeta &> /dev/null; then
    log_error "vegeta not found. Install with: go install github.com/tsenart/vegeta@latest"
    exit 1
fi

# Check if certificates exist
if [ ! -f "$CLIENT_CERT" ] || [ ! -f "$CLIENT_KEY" ]; then
    log_error "Client certificates not found: $CLIENT_CERT / $CLIENT_KEY"
    exit 1
fi

# Check if service is running
log_info "Checking if HSM service is running..."
if ! curl -k -s --cert "$CLIENT_CERT" --key "$CLIENT_KEY" "$HSM_URL/health" > /dev/null 2>&1; then
    log_error "HSM service is not running at $HSM_URL"
    exit 1
fi
log_success "HSM service is running"

echo ""
log_info "========================================="
log_info "Starting Stress Test"
log_info "========================================="
log_info "Target: $HSM_URL"
log_info "Duration: $DURATION per test"
log_info "Max Rate: $MAX_RATE req/s"
echo ""

# Test 1: Incremental Load Test
log_info "Test 1: Incremental Load (finding breaking point)..."
echo ""

# Create request body file
echo '{"context":"exchange-key","plaintext":"SGVsbG8gV29ybGQh"}' > /tmp/vegeta-body.json

for rate in 100 500 1000 2000 3000 4000 5000; do
    log_info "Testing at $rate req/s..."
    
    echo "POST $HSM_URL/encrypt" | vegeta attack \
        -duration=30s \
        -rate=$rate \
        -body=/tmp/vegeta-body.json \
        -header='Content-Type: application/json' \
        -cert="$CLIENT_CERT" \
        -key="$CLIENT_KEY" \
        -insecure \
        > "$RESULTS_DIR/incremental-${rate}.bin"
    
    # Analyze results
    vegeta report -type=text "$RESULTS_DIR/incremental-${rate}.bin" > "$RESULTS_DIR/incremental-${rate}.txt"
    
    # Extract success rate
    success_rate=$(cat "$RESULTS_DIR/incremental-${rate}.txt" | grep "Success" | awk '{print $3}' | tr -d '%')
    p95_latency=$(cat "$RESULTS_DIR/incremental-${rate}.txt" | grep "Latencies" | awk '{print $5}')
    
    if (( $(echo "$success_rate < 99" | bc -l) )); then
        log_warning "Success rate dropped to ${success_rate}% at ${rate} req/s"
        log_warning "Breaking point found!"
        break
    else
        log_success "${rate} req/s: ${success_rate}% success, P95=${p95_latency}"
    fi
    
    sleep 2 # Cool down between tests
done

echo ""
log_info "Test 2: Sustained High Load..."

# Test 2: Sustained high load
echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=$DURATION \
    -rate=1000 \
    -cert="$CLIENT_CERT" \
    -key="$CLIENT_KEY" \
    -body=/tmp/vegeta-body.json \
    -header='Content-Type: application/json' \
    -insecure \
    > "$RESULTS_DIR/sustained-high.bin"

vegeta report -type=text "$RESULTS_DIR/sustained-high.bin" > "$RESULTS_DIR/sustained-high.txt"
vegeta plot "$RESULTS_DIR/sustained-high.bin" > "$RESULTS_DIR/sustained-high.html"

log_success "Sustained load test complete"
cat "$RESULTS_DIR/sustained-high.txt"

echo ""
log_info "Test 3: Spike Test (sudden traffic increase)..."

# Test 3: Spike test
echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=10s \
    -rate=5000 \
    -cert="$CLIENT_CERT" \
    -key="$CLIENT_KEY" \
    -body=/tmp/vegeta-body.json \
    -header='Content-Type: application/json' \
    -insecure \
    > "$RESULTS_DIR/spike.bin"

vegeta report -type=text "$RESULTS_DIR/spike.bin" > "$RESULTS_DIR/spike.txt"
log_success "Spike test complete"
cat "$RESULTS_DIR/spike.txt"

echo ""
log_info "Test 4: Endurance Test (check for memory leaks)..."

# Test 4: Long-running endurance test
log_info "Running moderate load for 5 minutes to detect memory leaks..."
echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=5m \
    -rate=100 \
    -cert="$CLIENT_CERT" \
    -key="$CLIENT_KEY" \
    -body=/tmp/vegeta-body.json \
    -header='Content-Type: application/json' \
    -insecure \
    > "$RESULTS_DIR/endurance.bin"

vegeta report -type=text "$RESULTS_DIR/endurance.bin" > "$RESULTS_DIR/endurance.txt"
log_success "Endurance test complete"
cat "$RESULTS_DIR/endurance.txt"

# Summary
echo ""
log_info "========================================="
log_info "Stress Test Complete!"
log_info "========================================="
log_info "Results saved to: $RESULTS_DIR"
echo ""
log_info "View HTML plots:"
log_info "  - Sustained Load: file://$PWD/$RESULTS_DIR/sustained-high.html"
echo ""
log_info "Next steps:"
log_info "  1. Review results in $RESULTS_DIR/"
log_info "  2. Check service logs for errors"
log_info "  3. Monitor memory usage: docker stats"
log_info "  4. Run: vegeta report $RESULTS_DIR/sustained-high.bin for detailed analysis"
echo ""
