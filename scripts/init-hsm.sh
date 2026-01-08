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

# Check if KEKs exist
KEK_COUNT=$(pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin "$TOKEN_PIN" \
  --list-objects --type secrkey 2>/dev/null | grep -c "Secret Key" || true)

if [ "$KEK_COUNT" -eq 0 ]; then
    echo "⚠️  No KEKs found. Creating default KEKs from config.yaml..."
    echo ""
    
    # Create KEKs using create-kek helper
    /app/create-kek "kek-exchange-v1" "01" "$TOKEN_PIN" || echo "Failed to create kek-exchange-v1"
    /app/create-kek "kek-2fa-v1" "02" "$TOKEN_PIN" || echo "Failed to create kek-2fa-v1"
    /app/create-kek "kek-payment-v1" "03" "$TOKEN_PIN" || echo "Failed to create kek-payment-v1"
    
    echo ""
    echo "✓ Default KEKs created"
else
    echo "✓ Found $KEK_COUNT KEK(s) in HSM token"
fi

echo ""
echo "========================================="
echo "Starting HSM Service..."
echo "========================================="
echo ""

# Start the HSM service
exec /app/hsm-service
