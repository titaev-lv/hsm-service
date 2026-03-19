#!/bin/bash
# Timing attack heuristic test for HSM API.
# Compares latency profiles for valid vs invalid context requests.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

HSM_URL_FROM_ENV="${HSM_URL:-}"
HSM_URL="${HSM_URL:-https://localhost:8443}"
SAMPLES="${SAMPLES:-20}"
ABS_THRESHOLD_MS="${ABS_THRESHOLD_MS:-15}"
RATIO_THRESHOLD="${RATIO_THRESHOLD:-2.0}"

find_client_cert() {
    for p in \
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

if [ -z "$HSM_URL_FROM_ENV" ] && ! curl -sk --max-time 3 "$HSM_URL/health" >/dev/null 2>&1; then
    if curl -sk --max-time 3 "https://localhost:8444/health" >/dev/null 2>&1; then
        HSM_URL="https://localhost:8444"
    fi
fi

if ! curl -sk --max-time 3 "$HSM_URL/health" >/dev/null 2>&1; then
    echo "SKIP: service unreachable at $HSM_URL"
    exit 2
fi

if [ -z "$CLIENT_CERT" ] || [ -z "$CLIENT_KEY" ]; then
    echo "SKIP: client cert/key not found for mTLS requests"
    exit 2
fi

measure_avg_s() {
    local endpoint="$1"
    local body="$2"
    local total="0"
    local i
    for i in $(seq 1 "$SAMPLES"); do
        local t
        t=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
            -o /dev/null -w "%{time_total}" \
            -H "Content-Type: application/json" \
            -X POST "$HSM_URL/$endpoint" \
            -d "$body" 2>/dev/null || echo "0")
        total=$(awk -v a="$total" -v b="$t" 'BEGIN { printf "%.6f", a+b }')
    done
    awk -v s="$total" -v n="$SAMPLES" 'BEGIN { if (n==0) print "0"; else printf "%.6f", s/n }'
}

# valid context vs definitely-invalid context
VALID_BODY='{"context":"exchange-key","plaintext":"dGVzdA=="}'
INVALID_BODY='{"context":"__nonexistent_context__","plaintext":"dGVzdA=="}'

avg_valid=$(measure_avg_s "encrypt" "$VALID_BODY")
avg_invalid=$(measure_avg_s "encrypt" "$INVALID_BODY")

# Convert to milliseconds for human output and threshold check.
valid_ms=$(awk -v x="$avg_valid" 'BEGIN { printf "%.3f", x*1000 }')
invalid_ms=$(awk -v x="$avg_invalid" 'BEGIN { printf "%.3f", x*1000 }')
abs_delta_ms=$(awk -v a="$valid_ms" -v b="$invalid_ms" 'BEGIN { d=a-b; if (d<0) d=-d; printf "%.3f", d }')
ratio=$(awk -v a="$valid_ms" -v b="$invalid_ms" 'BEGIN { hi=(a>b)?a:b; lo=(a>b)?b:a; if (lo<0.001) lo=0.001; printf "%.3f", hi/lo }')

echo "Timing profile: valid=${valid_ms}ms invalid=${invalid_ms}ms delta=${abs_delta_ms}ms ratio=${ratio}x"

# Heuristic only: fail only on clearly suspicious and stable large gap.
if awk -v d="$abs_delta_ms" -v r="$ratio" -v dmin="$ABS_THRESHOLD_MS" -v rmin="$RATIO_THRESHOLD" 'BEGIN { exit !((d > dmin) && (r > rmin)) }'; then
    echo "FAIL: suspicious timing discrepancy (possible side-channel)"
    exit 1
fi

echo "PASS: no obvious timing side-channel above thresholds"
exit 0
