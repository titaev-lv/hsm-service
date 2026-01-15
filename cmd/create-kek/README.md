# create-kek - KEK Creation Utility

Simple utility to create AES-256 KEKs in HSM using low-level PKCS#11 API.

## Why This Tool Exists

- `pkcs11-tool --keygen` doesn't work reliably with SoftHSM (CKR_ATTRIBUTE_VALUE_INVALID errors)
- `crypto11` library doesn't expose public methods for key generation
- This tool uses raw PKCS#11 API to create keys with correct attributes

## Usage

```bash
./create-kek <label> <id-hex> <pin>
```

**Arguments:**
- `label` - Key label (e.g., "kek-exchange-key-v1")
- `id-hex` - Key ID in hex format (e.g., "01", "02", "ff")
- `pin` - HSM user PIN

**Example:**
```bash
./create-kek "kek-trading-v1" "05" "1234"
```

## Output

```
✓ Created KEK: kek-trading-v1 (handle: 3, ID: 05)
```

## Key Attributes

The utility creates AES-256 secret keys with:
- `CKA_VALUE_LEN`: 32 bytes (256 bits)
- `CKA_TOKEN`: true (persistent)
- `CKA_PRIVATE`: true (requires login)
- `CKA_SENSITIVE`: true (cannot be read)
- `CKA_ENCRYPT`: true
- `CKA_DECRYPT`: true
- `CKA_WRAP`: true (can wrap other keys)
- `CKA_UNWRAP`: true (can unwrap other keys)
- `CKA_EXTRACTABLE`: false (cannot be exported)

## Usage in Docker

```bash
# From host
docker-compose exec hsm-service /app/create-kek "kek-new-v1" "10" "1234"

# Inside container
/app/create-kek "kek-new-v1" "10" "$HSM_PIN"
```

## After Creating KEK

1. Add to config.yaml:
```yaml
hsm:
  keys:
    new-service:
      label: kek-new-v1
      type: aes
```

2. Restart service:
```bash
docker-compose restart
```

## ID Assignment

Use unique hex IDs for each KEK:
- `01` - kek-exchange-key-v1
- `02` - kek-2fa-v1
- `03` - kek-payment-v1
- `04-ff` - Available for new KEKs

## Troubleshooting

**Error: "Initialize failed"**
- Check PKCS#11 library path: `/usr/lib/softhsm/libsofthsm2.so`

**Error: "No slots found"**
- Initialize token first: `softhsm2-util --init-token --slot 0`

**Error: "Login failed"**
- Check PIN is correct
- Ensure token is initialized

**Error: "GenerateKey failed"**
- Check ID is unique (not already used)
- Ensure sufficient HSM memory
