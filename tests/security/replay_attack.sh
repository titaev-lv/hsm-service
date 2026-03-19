#!/bin/bash
# Replay resistance heuristics for encryption endpoint.
# Verifies non-deterministic ciphertext for identical plaintext/context.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

HSM_URL_FROM_ENV="${HSM_URL:-}"
HSM_URL="${HSM_URL:-https://localhost:8443}"

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

extract_ciphertext() {
    # Minimal JSON extraction without jq.
    sed -n 's/.*"ciphertext"[[:space:]]*:[[:space:]]*"\([^"]*\)".*/\1/p'
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

body='{"context":"exchange-key","plaintext":"cmVwbGF5LXRlc3Q="}'

resp1=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -X POST "$HSM_URL/encrypt" \
    -d "$body")

resp2=$(curl -sk --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -X POST "$HSM_URL/encrypt" \
    -d "$body")

ct1=$(printf '%s' "$resp1" | extract_ciphertext)
ct2=$(printf '%s' "$resp2" | extract_ciphertext)

if [ -z "$ct1" ] || [ -z "$ct2" ]; then
    echo "FAIL: could not parse ciphertext from API responses"
    echo "resp1=$resp1"
    echo "resp2=$resp2"
    exit 1
fi

if [ "$ct1" = "$ct2" ]; then
    echo "FAIL: deterministic ciphertext detected for identical plaintext"
    echo "Potential nonce reuse / replay weakness"
    exit 1
fi

echo "PASS: ciphertext differs across repeated requests (nonce/randomness OK)"
exit 0
