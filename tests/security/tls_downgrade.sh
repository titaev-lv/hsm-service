#!/bin/bash
# TLS downgrade resistance test.
# Ensures TLS 1.0/1.1 are rejected and TLS 1.2+ is accepted.

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../.." && pwd)"

TLS_HOST="${TLS_HOST:-localhost}"
TLS_PORT="${TLS_PORT:-8443}"
TARGET="$TLS_HOST:$TLS_PORT"

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

accepts_weak_tls() {
    local output="$1"
    local proto_regex="$2"

    # Accepted weak TLS should negotiate protocol and a non-NONE cipher.
    if echo "$output" | grep -qiE "$proto_regex" && \
       ! echo "$output" | grep -qiE "Cipher is \(NONE\)|no peer certificate|handshake failure|alert protocol version"; then
        return 0
    fi

    return 1
}

sclient_probe() {
    local proto_flag="$1"
    local cert_args=()
    if [ -n "$CLIENT_CERT" ] && [ -n "$CLIENT_KEY" ]; then
        cert_args=(-cert "$CLIENT_CERT" -key "$CLIENT_KEY")
    fi
    echo | timeout 5 openssl s_client -connect "$TARGET" "$proto_flag" "${cert_args[@]}" 2>&1 || true
}

# quick reachability check
if ! timeout 3 bash -lc "echo >/dev/tcp/$TLS_HOST/$TLS_PORT" 2>/dev/null; then
    echo "SKIP: TLS target unreachable at $TARGET"
    exit 2
fi

weak10=$(sclient_probe "-tls1")
weak11=$(sclient_probe "-tls1_1")
strong12=$(sclient_probe "-tls1_2")

if accepts_weak_tls "$weak10" "Protocol[[:space:]]*:[[:space:]]*TLSv1(\.|$)"; then
    echo "FAIL: server appears to accept TLS 1.0"
    exit 1
fi

if accepts_weak_tls "$weak11" "Protocol[[:space:]]*:[[:space:]]*TLSv1\.1"; then
    echo "FAIL: server appears to accept TLS 1.1"
    exit 1
fi

if ! echo "$strong12" | grep -qiE "Protocol[[:space:]]*:[[:space:]]*TLSv1\.(2|3)"; then
    echo "FAIL: could not establish TLS 1.2+ connection"
    exit 1
fi

echo "PASS: TLS downgrade resistance OK (TLS 1.0/1.1 rejected, TLS 1.2+ available)"
exit 0
