#!/bin/bash

# E2E Test: ACL Real-time Reload
# Tests dynamic revocation and restoration of client certificates

set -e

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/../../.." && pwd)"
cd "$PROJECT_ROOT"

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
NC='\033[0m'

print_test() { echo -e "${BLUE}[TEST]${NC} $1"; }
print_success() { echo -e "${GREEN}✓${NC} $1"; }
print_warning() { echo -e "${YELLOW}⚠${NC} $1"; }
print_error() { echo -e "${RED}✗${NC} $1"; exit 1; }

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

find_ca_cert() {
    for p in \
        "$PROJECT_ROOT/pki/test/ca/ca.crt" \
        "$PROJECT_ROOT/pki/ca/ca.crt"; do
        if [ -f "$p" ]; then
            echo "$p"
            return 0
        fi
    done
    return 1
}

find_revoked_file() {
    for p in \
        "$PROJECT_ROOT/revoked-test.yaml" \
        "$PROJECT_ROOT/pki/test/revoked.yaml" \
        "$PROJECT_ROOT/pki/revoked.yaml"; do
        if [ -f "$p" ]; then
            echo "$p"
            return 0
        fi
    done

    # Fallback to path declared in config if present.
    local configured
    configured=$(grep -E '^\s*revoked_file:\s*' "$PROJECT_ROOT/config.yaml" 2>/dev/null | head -1 | awk '{print $2}' | tr -d '"')
    if [ -n "$configured" ]; then
        if [ -f "$configured" ]; then
            echo "$configured"
            return 0
        fi
        if [ -f "$PROJECT_ROOT/$configured" ]; then
            echo "$PROJECT_ROOT/$configured"
            return 0
        fi
    fi

    return 1
}

find_service_container() {
    if docker ps --format '{{.Names}}' | grep -qx 'hsm-service-test'; then
        echo "hsm-service-test"
        return 0
    fi
    if docker ps --format '{{.Names}}' | grep -qx 'hsm-service'; then
        echo "hsm-service"
        return 0
    fi
    return 1
}

probe_health() {
    local url="$1"
    curl -sk --max-time 3 --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" "$url/health" >/dev/null 2>&1
}

# Configuration
BASE_URL="${HSM_URL:-https://localhost:8443}"
CA_CERT="${CA_CERT:-$(find_ca_cert || true)}"
CLIENT_CERT="${CLIENT_CERT:-$(find_client_cert || true)}"
CLIENT_KEY="${CLIENT_KEY:-$(find_client_key || true)}"
REVOKED_FILE="${REVOKED_FILE:-$(find_revoked_file || true)}"
SERVICE_CONTAINER="${SERVICE_CONTAINER:-$(find_service_container || true)}"

if [ -z "$CA_CERT" ] || [ ! -f "$CA_CERT" ]; then
    print_error "CA cert not found"
fi
if [ -z "$CLIENT_CERT" ] || [ ! -f "$CLIENT_CERT" ]; then
    print_error "Client cert not found"
fi
if [ -z "$CLIENT_KEY" ] || [ ! -f "$CLIENT_KEY" ]; then
    print_error "Client key not found"
fi
if [ -z "$REVOKED_FILE" ] || [ ! -f "$REVOKED_FILE" ]; then
    print_error "revoked.yaml not found"
fi

if ! probe_health "$BASE_URL"; then
    if probe_health "https://localhost:8444"; then
        BASE_URL="https://localhost:8444"
        print_warning "Auto-detected test endpoint: $BASE_URL"
    fi
fi

# Extract client CN from certificate
CLIENT_CN=$(openssl x509 -in "$CLIENT_CERT" -noout -subject | sed -n 's/.*CN *= *\([^,\/]*\).*/\1/p')
if [ -z "$CLIENT_CN" ]; then
    print_error "Failed to parse client CN from certificate: $CLIENT_CERT"
fi

print_test "Scenario: ACL Real-time Reload"
echo "=========================================="
echo "Client CN: $CLIENT_CN"
echo ""

# Backup original revoked.yaml
if [ -n "$SERVICE_CONTAINER" ]; then
    docker cp "$SERVICE_CONTAINER":/app/revoked.yaml "${REVOKED_FILE}.backup"
else
    cp "$REVOKED_FILE" "${REVOKED_FILE}.backup"
fi

# Step 1: Verify client can access protected operation initially
print_test "Step 1: Verify client can encrypt (baseline)"
BASELINE_PAYLOAD='{"context":"exchange-key","plaintext":"QUNMIC1iYXNlbGluZQ=="}'
RESPONSE=$(curl -s -w "\n%{http_code}" --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" -d "$BASELINE_PAYLOAD" "$BASE_URL/encrypt" 2>&1)

HTTP_CODE=$(echo "$RESPONSE" | tail -1)
HTTP_BODY=$(echo "$RESPONSE" | head -n -1)
if [ "$HTTP_CODE" != "200" ] || ! echo "$HTTP_BODY" | grep -q "ciphertext"; then
    echo "HTTP Code: $HTTP_CODE"
    echo "Response: $HTTP_BODY"
    print_error "Client cannot perform baseline encrypt"
fi
print_success "Client successfully encrypted (baseline)"

# Step 2: Add client to revoked list
print_test "Step 2: Add client to revocation list"
REVOCATION_DATE="$(date -u +%Y-%m-%dT%H:%M:%SZ)"
if [ -n "$SERVICE_CONTAINER" ]; then
        printf 'revoked_certificates:\n  - cn: "%s"\n    reason: "E2E test revocation"\n    date: "%s"\n' \
                "$CLIENT_CN" "$REVOCATION_DATE" | docker exec -i "$SERVICE_CONTAINER" sh -lc 'cat > /app/revoked.yaml'
else
        cat > "$REVOKED_FILE" << EOF
revoked_certificates:
    - cn: "$CLIENT_CN"
        reason: "E2E test revocation"
        date: "$REVOCATION_DATE"
EOF
fi
# Some filesystems expose coarse timestamp precision; force mtime bump for reload watcher.
sleep 1
if [ -n "$SERVICE_CONTAINER" ]; then
    docker exec "$SERVICE_CONTAINER" sh -lc 'touch /app/revoked.yaml'
else
    touch "$REVOKED_FILE"
fi
print_success "Added $CLIENT_CN to revoked.yaml"

# Step 3: Wait for auto-reload (30 seconds + buffer)
print_test "Step 3: Wait for ACL auto-reload (35 seconds)"
for i in {35..1}; do
    echo -ne "\rWaiting... $i seconds   "
    sleep 1
done
echo ""
print_success "Auto-reload period elapsed"

# Step 4: Verify client is now blocked on protected endpoint
print_test "Step 4: Verify client is blocked"
BLOCKED_PAYLOAD='{"context":"exchange-key","plaintext":"QUNMIC1ibG9ja2Vk"}'
BLOCKED_RESPONSE=$(curl -s -w "\n%{http_code}" --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" -d "$BLOCKED_PAYLOAD" "$BASE_URL/encrypt" 2>&1)

BLOCKED_CODE=$(echo "$BLOCKED_RESPONSE" | tail -1)
BLOCKED_BODY=$(echo "$BLOCKED_RESPONSE" | head -n -1)

if [ "$BLOCKED_CODE" = "403" ] || echo "$BLOCKED_BODY" | grep -qi "revoked\|forbidden"; then
    print_success "Client correctly blocked (HTTP $BLOCKED_CODE)"
else
    echo "HTTP Code: $BLOCKED_CODE"
    echo "Response: $BLOCKED_BODY"
    # Restore original content without inode swap (important for bind-mounted files)
    if [ -n "$SERVICE_CONTAINER" ]; then
        cat "${REVOKED_FILE}.backup" | docker exec -i "$SERVICE_CONTAINER" sh -lc 'cat > /app/revoked.yaml'
    else
        cp "${REVOKED_FILE}.backup" "$REVOKED_FILE"
    fi
    print_error "Client was NOT blocked (ACL reload failed)"
fi

# Step 5: Remove from revoked list
print_test "Step 5: Restore client (remove from revocation list)"
if [ -n "$SERVICE_CONTAINER" ]; then
    cat "${REVOKED_FILE}.backup" | docker exec -i "$SERVICE_CONTAINER" sh -lc 'cat > /app/revoked.yaml'
else
    cp "${REVOKED_FILE}.backup" "$REVOKED_FILE"
fi
sleep 1
if [ -n "$SERVICE_CONTAINER" ]; then
    docker exec "$SERVICE_CONTAINER" sh -lc 'touch /app/revoked.yaml'
else
    touch "$REVOKED_FILE"
fi
rm -f "${REVOKED_FILE}.backup"
print_success "Removed $CLIENT_CN from revoked.yaml"

# Step 6: Wait for auto-reload again
print_test "Step 6: Wait for ACL auto-reload (35 seconds)"
for i in {35..1}; do
    echo -ne "\rWaiting... $i seconds   "
    sleep 1
done
echo ""
print_success "Auto-reload period elapsed"

# Step 7: Verify client can encrypt again
print_test "Step 7: Verify client can encrypt again"
PLAINTEXT="QUNMIFJlbG9hZCBUZXN0"
ENC_RESPONSE=$(curl -s --cacert "$CA_CERT" --cert "$CLIENT_CERT" --key "$CLIENT_KEY" \
    -H "Content-Type: application/json" \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$PLAINTEXT\"}" \
    "$BASE_URL/encrypt")

if ! echo "$ENC_RESPONSE" | grep -q "ciphertext"; then
    echo "Response: $ENC_RESPONSE"
    print_error "Encryption failed after restoration"
fi
print_success "Client successfully restored"

echo ""
echo "=========================================="
print_success "✓ ACL Real-time Reload E2E Test PASSED"
echo "=========================================="
echo ""
echo "Summary:"
echo "  1. ✓ Client encrypted successfully (baseline)"
echo "  2. ✓ Client added to revocation list"
echo "  3. ✓ Auto-reload detected changes (30s)"
echo "  4. ✓ Client correctly blocked on encrypt endpoint"
echo "  5. ✓ Client removed from revocation list"
echo "  6. ✓ Auto-reload detected restoration (30s)"
echo "  7. ✓ Client successfully restored"
