#!/bin/sh
set -e

# Configuration from environment variables
TOKEN_LABEL="${HSM_TOKEN_LABEL:-hsm-token}"
TOKEN_PIN="${HSM_PIN:-1234}"
TOKEN_SO_PIN="${HSM_SO_PIN:-12345678}"

echo "========================================="
echo "HSM Service Initialization"
echo "========================================="

# Create token directory if it doesn't exist
mkdir -p /var/lib/softhsm/tokens

# Check if token is already initialized
if ! softhsm2-util --show-slots | grep -q "$TOKEN_LABEL"; then
    echo "⏳ Initializing SoftHSM token: $TOKEN_LABEL"
    softhsm2-util --init-token \
        --slot 0 \
        --label "$TOKEN_LABEL" \
        --pin "$TOKEN_PIN" \
        --so-pin "$TOKEN_SO_PIN"
    echo "✓ Token initialized successfully"
else
    echo "✓ Token '$TOKEN_LABEL' already initialized"
fi

# Display token information
echo ""
echo "Token slots:"
softhsm2-util --show-slots

echo ""
echo "========================================="
echo "KEK Management"
echo "========================================="
echo "Note: Use 'hsm-admin' to manage KEKs"
echo "Example:"
echo "  docker exec <container> /app/hsm-admin list-kek"
echo "  docker exec <container> /app/hsm-admin export-metadata"
echo ""

# Export HSM_PIN for the service
export HSM_PIN="$TOKEN_PIN"

echo "========================================="
echo "KEK Setup"
echo "========================================="

# Check total KEK count in HSM
KEK_COUNT=$(pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin "$TOKEN_PIN" \
  --list-objects --type secrkey 2>/dev/null | grep -c "Secret Key" || true)

echo "✓ Found $KEK_COUNT KEK(s) in HSM token"

# Check if required KEKs exist by label
CHECK_KEK_EXCHANGE=$(pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin "$TOKEN_PIN" \
  --list-objects --type secrkey 2>/dev/null | grep -c "label:.*kek-exchange-v1" || true)

CHECK_KEK_2FA=$(pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin "$TOKEN_PIN" \
  --list-objects --type secrkey 2>/dev/null | grep -c "label:.*kek-2fa-v1" || true)

# Create missing KEKs
CREATED_ANY=false

if [ "$CHECK_KEK_EXCHANGE" -eq 0 ]; then
    echo "⚠️  kek-exchange-v1 not found. Creating..."
    /app/create-kek "kek-exchange-v1" "$TOKEN_PIN" 1 || echo "Failed to create kek-exchange-v1"
    CREATED_ANY=true
else
    echo "✓ kek-exchange-v1 already exists"
fi

if [ "$CHECK_KEK_2FA" -eq 0 ]; then
    echo "⚠️  kek-2fa-v1 not found. Creating..."
    /app/create-kek "kek-2fa-v1" "$TOKEN_PIN" 1 || echo "Failed to create kek-2fa-v1"
    CREATED_ANY=true
else
    echo "✓ kek-2fa-v1 already exists"
fi

if [ "$CREATED_ANY" = true ]; then
    echo ""
    echo "✓ Default KEKs created"
fi

echo ""
echo "========================================="
echo "Starting HSM Service..."
echo "========================================="
echo ""

# Start the HSM service
exec /app/hsm-service
