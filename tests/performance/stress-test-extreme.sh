#!/bin/bash
# EXTREME Stress Testing - Finding the TRUE breaking point
# This test pushes the system beyond normal operational limits
#
# Requirements:
#   - vegeta: https://github.com/tsenart/vegeta
#   - Sufficient system resources (ulimits, file descriptors)

set -euo pipefail

# Configuration
HSM_URL="${HSM_URL:-https://localhost:8443}"
RESULTS_DIR="${RESULTS_DIR:-./stress-results-extreme}"
DURATION="${DURATION:-30s}"
MAX_RATE="${MAX_RATE:-100000}" # 100k req/s - extreme!

# Client certificates (mTLS required)
CLIENT_CERT="${CLIENT_CERT:-pki/client/hsm-trading-client-1.crt}"
CLIENT_KEY="${CLIENT_KEY:-pki/client/hsm-trading-client-1.key}"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
MAGENTA='\033[0;35m'
NC='\033[0m' # No Color

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[✓]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[⚠]${NC} $1"
}

log_error() {
    echo -e "${RED}[✗]${NC} $1"
}

log_extreme() {
    echo -e "${MAGENTA}[🔥 EXTREME]${NC} $1"
}

# Create results directory
mkdir -p "$RESULTS_DIR"
log_info "Results will be saved to: $RESULTS_DIR"

# Check if vegeta is installed
if ! command -v vegeta &> /dev/null; then
    log_error "vegeta not found. Install with: go install github.com/tsenart/vegeta@latest"
    exit 1
fi

# Check if service is running
log_info "Checking if HSM service is running..."
if curl -k -s --max-time 5 \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    "$HSM_URL/health" > /dev/null 2>&1; then
    log_success "HSM service is running"
else
    log_error "HSM service is not responding at $HSM_URL"
    log_error "Start it with: docker compose up -d"
    exit 1
fi

# Check if certificates exist
if [ ! -f "$CLIENT_CERT" ] || [ ! -f "$CLIENT_KEY" ]; then
    log_error "Client certificates not found: $CLIENT_CERT / $CLIENT_KEY"
    exit 1
fi

log_extreme "==========================================="
log_extreme "EXTREME STRESS TEST - BREAKING POINT HUNT"
log_extreme "==========================================="
log_extreme "Target: $HSM_URL"
log_extreme "Duration: $DURATION per test"
log_extreme "Max Rate: $MAX_RATE req/s"
log_extreme "WARNING: This test will push the system to its limits!"
echo ""
sleep 2

# Create request body files
echo '{"context":"exchange-key","plaintext":"SGVsbG8gV29ybGQh"}' > /tmp/vegeta-body.json

# Get ciphertext for decrypt tests
log_info "Preparing decrypt test data..."
ENCRYPT_RESPONSE=$(curl -k -s \
    --cert "$CLIENT_CERT" \
    --key "$CLIENT_KEY" \
    -X POST "$HSM_URL/encrypt" \
    -H "Content-Type: application/json" \
    -d '{"context":"exchange-key","plaintext":"SGVsbG8gV29ybGQh"}')

CIPHERTEXT=$(echo "$ENCRYPT_RESPONSE" | grep -o '"ciphertext":"[^"]*"' | cut -d'"' -f4)
KEY_ID=$(echo "$ENCRYPT_RESPONSE" | grep -o '"key_id":"[^"]*"' | cut -d'"' -f4)

if [ -z "$CIPHERTEXT" ] || [ -z "$KEY_ID" ]; then
    log_error "Failed to get ciphertext or key_id for decrypt tests"
    exit 1
fi

echo "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHERTEXT\",\"key_id\":\"$KEY_ID\"}" > /tmp/vegeta-decrypt-body.json
log_success "Test data prepared (key_id: $KEY_ID)"

echo ""
log_extreme "Test 1: ULTRA HIGH Load - Finding the Breaking Point"
echo ""

# Test 1: Extreme incremental load
BREAKING_POINT_FOUND=false

for rate in 20000 25000 30000 40000 50000 75000 100000; do
    log_extreme "Testing ENCRYPT at $rate req/s..."
    
    echo "POST $HSM_URL/encrypt" | vegeta attack \
        -duration=$DURATION \
        -rate=$rate \
        -body=/tmp/vegeta-body.json \
        -header='Content-Type: application/json' \
        -cert="$CLIENT_CERT" \
        -key="$CLIENT_KEY" \
        -insecure \
        -timeout=30s \
        -max-workers=2000 \
        -keepalive=true \
        > "$RESULTS_DIR/extreme-encrypt-${rate}.bin" 2>/dev/null || true
    
    # Analyze results
    vegeta report -type=text "$RESULTS_DIR/extreme-encrypt-${rate}.bin" > "$RESULTS_DIR/extreme-encrypt-${rate}.txt" 2>/dev/null || true
    
    # Extract metrics
    success_rate=$(cat "$RESULTS_DIR/extreme-encrypt-${rate}.txt" | grep "Success" | awk '{print $3}' | tr -d '%' || echo "0")
    p95_latency=$(cat "$RESULTS_DIR/extreme-encrypt-${rate}.txt" | grep "Latencies" | awk '{print $5}' || echo "N/A")
    throughput=$(cat "$RESULTS_DIR/extreme-encrypt-${rate}.txt" | grep "Requests" | awk '{print $6}' | tr -d ',' || echo "0")
    
    if (( $(echo "$success_rate < 90" | bc -l) )); then
        log_warning "Success rate dropped to ${success_rate}% at ${rate} req/s"
        log_extreme "🎯 BREAKING POINT FOUND: ~${rate} req/s (${success_rate}% success)"
        BREAKING_POINT_FOUND=true
        break
    else
        log_success "${rate} req/s: ${success_rate}% success, P95=${p95_latency}, throughput=${throughput} req/s"
    fi
    
    sleep 5 # Cool down between extreme tests
done

if [ "$BREAKING_POINT_FOUND" = false ]; then
    log_extreme "🚀 NO BREAKING POINT FOUND! System survived 100k req/s!"
fi

echo ""
log_extreme "Test 2: DECRYPT Ultra High Load - Finding the Breaking Point"
echo ""

# Test 2: Extreme decrypt load (same levels as encrypt)
DECRYPT_BREAKING_POINT_FOUND=false

for rate in 20000 25000 30000 40000 50000 75000 100000; do
    log_extreme "Testing DECRYPT at $rate req/s..."
    
    echo "POST $HSM_URL/decrypt" | vegeta attack \
        -duration=$DURATION \
        -rate=$rate \
        -body=/tmp/vegeta-decrypt-body.json \
        -header='Content-Type: application/json' \
        -cert="$CLIENT_CERT" \
        -key="$CLIENT_KEY" \
        -insecure \
        -timeout=30s \
        -max-workers=2000 \
        -keepalive=true \
        > "$RESULTS_DIR/extreme-decrypt-${rate}.bin" 2>/dev/null || true
    
    vegeta report -type=text "$RESULTS_DIR/extreme-decrypt-${rate}.bin" > "$RESULTS_DIR/extreme-decrypt-${rate}.txt" 2>/dev/null || true
    
    success_rate=$(cat "$RESULTS_DIR/extreme-decrypt-${rate}.txt" | grep "Success" | awk '{print $3}' | tr -d '%' || echo "0")
    p95_latency=$(cat "$RESULTS_DIR/extreme-decrypt-${rate}.txt" | grep "Latencies" | awk '{print $5}' || echo "N/A")
    throughput=$(cat "$RESULTS_DIR/extreme-decrypt-${rate}.txt" | grep "Requests" | awk '{print $6}' || echo "N/A")
    
    if (( $(echo "$success_rate < 90" | bc -l) )); then
        log_warning "Success rate dropped to ${success_rate}% at ${rate} req/s"
        log_extreme "🎯 DECRYPT BREAKING POINT FOUND: ~${rate} req/s (${success_rate}% success)"
        DECRYPT_BREAKING_POINT_FOUND=true
        break
    else
        log_success "${rate} req/s: ${success_rate}% success, P95=${p95_latency}, throughput=${throughput} req/s"
    fi
    
    sleep 5
done

if [ "$DECRYPT_BREAKING_POINT_FOUND" = false ]; then
    log_extreme "🚀 NO DECRYPT BREAKING POINT FOUND! System survived 100k req/s!"
fi

echo ""
log_extreme "Test 3: MASSIVE Spike Attack (simulate DDoS)"
echo ""

# Test 3: Massive spike test
log_extreme "Launching 100k req/s spike for 20 seconds..."
echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=20s \
    -rate=100000 \
    -cert="$CLIENT_CERT" \
    -key="$CLIENT_KEY" \
    -body=/tmp/vegeta-body.json \
    -header='Content-Type: application/json' \
    -insecure \
    -timeout=30s \
    -max-workers=5000 \
    -keepalive=true \
    > "$RESULTS_DIR/extreme-spike-100k.bin" 2>/dev/null || true

vegeta report -type=text "$RESULTS_DIR/extreme-spike-100k.bin" > "$RESULTS_DIR/extreme-spike-100k.txt" 2>/dev/null || true
log_success "Spike test complete"
cat "$RESULTS_DIR/extreme-spike-100k.txt"

echo ""
log_extreme "Test 4: Mixed Workload (Encrypt + Decrypt simultaneously)"
echo ""

# Test 4: Parallel encrypt + decrypt
log_extreme "Running 10k encrypt + 10k decrypt simultaneously..."

echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=30s \
    -rate=10000 \
    -body=/tmp/vegeta-body.json \
    -header='Content-Type: application/json' \
    -cert="$CLIENT_CERT" \
    -key="$CLIENT_KEY" \
    -insecure \
    -timeout=10s \
    -max-workers=1000 \
    > "$RESULTS_DIR/mixed-encrypt.bin" 2>/dev/null &

ENCRYPT_PID=$!

echo "POST $HSM_URL/decrypt" | vegeta attack \
    -duration=30s \
    -rate=10000 \
    -body=/tmp/vegeta-decrypt-body.json \
    -header='Content-Type: application/json' \
    -cert="$CLIENT_CERT" \
    -key="$CLIENT_KEY" \
    -insecure \
    -timeout=10s \
    -max-workers=1000 \
    > "$RESULTS_DIR/mixed-decrypt.bin" 2>/dev/null &

DECRYPT_PID=$!

wait $ENCRYPT_PID
wait $DECRYPT_PID

vegeta report -type=text "$RESULTS_DIR/mixed-encrypt.bin" > "$RESULTS_DIR/mixed-encrypt.txt" 2>/dev/null || true
vegeta report -type=text "$RESULTS_DIR/mixed-decrypt.bin" > "$RESULTS_DIR/mixed-decrypt.txt" 2>/dev/null || true

log_success "Mixed workload test complete"
echo "ENCRYPT results:"
cat "$RESULTS_DIR/mixed-encrypt.txt"
echo ""
echo "DECRYPT results:"
cat "$RESULTS_DIR/mixed-decrypt.txt"

echo ""
log_extreme "Test 5: Large Payload Test"
echo ""

# Test 5: Large payload (512 bytes plaintext)  
# Note: Using static payload is OK - all normal operations use same-size payloads repeatedly
log_extreme "Testing with 512-byte payload at 5000 req/s..."
# Remove newlines from base64 output to ensure valid JSON
LARGE_PLAINTEXT=$(openssl rand -base64 384 | tr -d '\n')
echo "{\"context\":\"exchange-key\",\"plaintext\":\"$LARGE_PLAINTEXT\"}" > /tmp/vegeta-large-body.json

echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=30s \
    -rate=5000 \
    -body=/tmp/vegeta-large-body.json \
    -header='Content-Type: application/json' \
    -cert="$CLIENT_CERT" \
    -key="$CLIENT_KEY" \
    -insecure \
    -timeout=10s \
    -max-workers=500 \
    -keepalive=true \
    > "$RESULTS_DIR/large-payload.bin" 2>/dev/null || true

vegeta report -type=text "$RESULTS_DIR/large-payload.bin" > "$RESULTS_DIR/large-payload.txt" 2>/dev/null || true
log_success "Large payload test complete"
cat "$RESULTS_DIR/large-payload.txt"

echo ""
log_extreme "Test 6: Round-Trip Latency (Encrypt → Decrypt)"
echo ""

# Test 6: Measure full round-trip latency
log_extreme "Testing encrypt → decrypt round-trip at 5000 req/s..."

# Create a script for round-trip testing
cat > /tmp/roundtrip-test.sh << 'ROUNDTRIP_EOF'
#!/bin/bash
CERT="pki/client/hsm-trading-client-1.crt"
KEY="pki/client/hsm-trading-client-1.key"
URL="https://localhost:8443"

# Encrypt
ENC_RESP=$(curl -k -s --cert "$CERT" --key "$KEY" \
    -X POST "$URL/encrypt" \
    -H "Content-Type: application/json" \
    -d '{"context":"exchange-key","plaintext":"SGVsbG8gV29ybGQh"}')

CIPHER=$(echo "$ENC_RESP" | jq -r '.ciphertext')
KEY_ID=$(echo "$ENC_RESP" | jq -r '.key_id')

# Decrypt
DEC_RESP=$(curl -k -s --cert "$CERT" --key "$KEY" \
    -X POST "$URL/decrypt" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$CIPHER\",\"key_id\":\"$KEY_ID\"}")

echo "$DEC_RESP"
ROUNDTRIP_EOF

chmod +x /tmp/roundtrip-test.sh

# Run 1000 round-trips and measure time
start_time=$(date +%s.%N)
for i in {1..1000}; do
    /tmp/roundtrip-test.sh > /dev/null 2>&1
done
end_time=$(date +%s.%N)

total_time=$(echo "$end_time - $start_time" | bc)
avg_latency=$(echo "scale=2; $total_time / 1000" | bc)
throughput=$(echo "scale=2; 1000 / $total_time" | bc)

echo "Round-trip results (1000 iterations):" > "$RESULTS_DIR/roundtrip-latency.txt"
echo "  Total time: ${total_time}s" >> "$RESULTS_DIR/roundtrip-latency.txt"
echo "  Average latency: ${avg_latency}s per round-trip" >> "$RESULTS_DIR/roundtrip-latency.txt"
echo "  Throughput: ${throughput} round-trips/sec" >> "$RESULTS_DIR/roundtrip-latency.txt"

log_success "Round-trip test complete"
cat "$RESULTS_DIR/roundtrip-latency.txt"

echo ""
log_extreme "Test 7: Burst Recovery (Resilience Test)"
echo ""

# Test 7: Test recovery after burst
log_extreme "Phase 1: Baseline (5000 req/s for 10s)..."
echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=10s \
    -rate=5000 \
    -body=/tmp/vegeta-body.json \
    -header='Content-Type: application/json' \
    -cert="$CLIENT_CERT" \
    -key="$CLIENT_KEY" \
    -insecure \
    -timeout=10s \
    -max-workers=500 \
    -keepalive=true \
    > "$RESULTS_DIR/burst-baseline.bin" 2>/dev/null || true

log_extreme "Phase 2: BURST (50000 req/s for 5s)..."
echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=5s \
    -rate=50000 \
    -body=/tmp/vegeta-body.json \
    -header='Content-Type: application/json' \
    -cert="$CLIENT_CERT" \
    -key="$CLIENT_KEY" \
    -insecure \
    -timeout=10s \
    -max-workers=3000 \
    -keepalive=true \
    > "$RESULTS_DIR/burst-attack.bin" 2>/dev/null || true

sleep 2

log_extreme "Phase 3: Recovery check (5000 req/s for 10s)..."
echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=10s \
    -rate=5000 \
    -body=/tmp/vegeta-body.json \
    -header='Content-Type: application/json' \
    -cert="$CLIENT_CERT" \
    -key="$CLIENT_KEY" \
    -insecure \
    -timeout=10s \
    -max-workers=500 \
    -keepalive=true \
    > "$RESULTS_DIR/burst-recovery.bin" 2>/dev/null || true

vegeta report -type=text "$RESULTS_DIR/burst-baseline.bin" > "$RESULTS_DIR/burst-baseline.txt" 2>/dev/null || true
vegeta report -type=text "$RESULTS_DIR/burst-attack.bin" > "$RESULTS_DIR/burst-attack.txt" 2>/dev/null || true
vegeta report -type=text "$RESULTS_DIR/burst-recovery.bin" > "$RESULTS_DIR/burst-recovery.txt" 2>/dev/null || true

log_success "Burst recovery test complete"
echo "Baseline:"
cat "$RESULTS_DIR/burst-baseline.txt" | grep "Success\|Latencies"
echo ""
echo "Burst attack:"
cat "$RESULTS_DIR/burst-attack.txt" | grep "Success\|Latencies"
echo ""
echo "Recovery:"
cat "$RESULTS_DIR/burst-recovery.txt" | grep "Success\|Latencies"

echo ""
log_extreme "Test 8: Different Contexts (ACL Load Distribution)"
echo ""

# Test 8: Test multiple contexts simultaneously
log_extreme "Testing 3 contexts simultaneously (5k req/s each = 15k total)..."

# Create bodies for different contexts
echo '{"context":"exchange-key","plaintext":"SGVsbG8gV29ybGQh"}' > /tmp/vegeta-exchange.json
echo '{"context":"2fa","plaintext":"U2VjcmV0Q29kZQ=="}' > /tmp/vegeta-2fa.json

# Run concurrent tests for different contexts
echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=15s \
    -rate=7500 \
    -body=/tmp/vegeta-exchange.json \
    -header='Content-Type: application/json' \
    -cert="pki/client/hsm-trading-client-1.crt" \
    -key="pki/client/hsm-trading-client-1.key" \
    -insecure \
    -timeout=10s \
    -max-workers=500 \
    -keepalive=true \
    > "$RESULTS_DIR/context-exchange.bin" 2>/dev/null || true &
EXCHANGE_PID=$!

echo "POST $HSM_URL/encrypt" | vegeta attack \
    -duration=15s \
    -rate=7500 \
    -body=/tmp/vegeta-2fa.json \
    -header='Content-Type: application/json' \
    -cert="pki/client/hsm-2fa-client-1.crt" \
    -key="pki/client/hsm-2fa-client-1.key" \
    -insecure \
    -timeout=10s \
    -max-workers=500 \
    -keepalive=true \
    > "$RESULTS_DIR/context-2fa.bin" 2>/dev/null || true &
TFA_PID=$!

wait $EXCHANGE_PID
wait $TFA_PID

vegeta report -type=text "$RESULTS_DIR/context-exchange.bin" > "$RESULTS_DIR/context-exchange.txt" 2>/dev/null || true
vegeta report -type=text "$RESULTS_DIR/context-2fa.bin" > "$RESULTS_DIR/context-2fa.txt" 2>/dev/null || true

log_success "Multi-context test complete"
echo "Exchange context results:"
cat "$RESULTS_DIR/context-exchange.txt" | grep "Success\|Throughput"
echo ""
echo "2FA context results:"
cat "$RESULTS_DIR/context-2fa.txt" | grep "Success\|Throughput"

# Cleanup
rm -f /tmp/vegeta-*.json /tmp/roundtrip-test.sh

# Summary
echo ""
log_extreme "==========================================="
log_extreme "EXTREME STRESS TEST COMPLETE"
log_extreme "==========================================="
log_info "Results saved to: $RESULTS_DIR"
echo ""
log_info "Test Summary:"
log_info "  - Encrypt extreme load: extreme-encrypt-*.txt"
log_info "  - Decrypt extreme load: extreme-decrypt-*.txt"
log_info "  - 100k spike test: extreme-spike-100k.txt"
log_info "  - Mixed workload: mixed-*.txt"
log_info "  - Large payload: large-payload.txt"
log_info "  - Round-trip latency: roundtrip-latency.txt"
log_info "  - Burst recovery: burst-*.txt"
log_info "  - Multi-context: context-*.txt"
echo ""
log_info "Next steps:"
log_info "  1. Review all results in $RESULTS_DIR/"
log_info "  2. Check service logs: docker compose logs hsm-service"
log_info "  3. Check system resources: docker stats hsm-service"
log_info "  4. Analyze latency distribution: vegeta plot $RESULTS_DIR/*.bin"
echo ""

if [ "$BREAKING_POINT_FOUND" = true ]; then
    log_warning "System breaking point was found - review results for optimization opportunities"
else
    log_extreme "🏆 CONGRATULATIONS! System survived ALL extreme tests!"
fi
