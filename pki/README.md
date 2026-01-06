# PKI Management Guide

## Overview

This directory contains the Public Key Infrastructure (PKI) for the HSM Service project. It includes scripts for managing certificates, tracking issued certificates, and handling revocations.

## Directory Structure

```
pki/
├── ca/                    # Certificate Authority files
│   ├── ca.crt            # CA certificate (public)
│   └── ca.key            # CA private key (KEEP SECRET!)
├── server/               # Server certificates
│   ├── *.crt            # Server certificate files
│   └── *.key            # Server private keys
├── client/               # Client certificates
│   ├── *.crt            # Client certificate files
│   └── *.key            # Client private keys
├── scripts/              # PKI management scripts
│   ├── issue-server-cert.sh
│   ├── issue-client-cert.sh
│   └── revoke-cert.sh
├── inventory.yaml        # Certificate inventory (auto-updated)
├── revoked.yaml         # Revoked certificates list
└── README.md            # This file
```

## Prerequisites

- OpenSSL installed
- Python 3 with PyYAML (`pip3 install pyyaml`)
- Existing CA certificate and key in `pki/ca/`

## Certificate Types

### 1. Server Certificates

Used for TLS servers (HSM service, MySQL, ClickHouse, etc.)

**Subject format:**
```
/C=RU/ST=Moscow/L=Moscow/O=Private/OU=Services/CN=<service-name>
```

**Features:**
- Subject Alternative Names (SAN) for DNS and IP
- Extended Key Usage: serverAuth
- Validity: 365 days

### 2. Client Certificates

Used for mTLS authentication

**Subject format:**
```
/C=RU/ST=Moscow/L=Moscow/O=Private/OU=<OU>/CN=<client-name>
```

**Organizational Units (OU) and their access:**

| OU       | HSM Access Context | Purpose                          |
|----------|-------------------|----------------------------------|
| Trading  | exchange-key      | Trading services (encrypt DEKs)  |
| 2FA      | 2fa               | 2FA services (encrypt secrets)   |
| Database | -                 | MySQL/ClickHouse clients only    |
| Admin    | admin (future)    | Administrative access            |

**Features:**
- Extended Key Usage: clientAuth
- Validity: 365 days
- Access rights determined by OU

## Usage

### Issue Server Certificate

**Syntax:**
```bash
./pki/scripts/issue-server-cert.sh <cn> <san-dns> [<san-ip>]
```

**Parameters:**
- `cn` - Common Name (e.g., `hsm-service.local`)
- `san-dns` - DNS names for SAN, comma-separated (e.g., `"hsm-service.local,localhost"`)
- `san-ip` - (Optional) IP addresses for SAN, comma-separated (e.g., `"127.0.0.1,10.0.0.5"`)

**Examples:**

```bash
# HSM Service
./pki/scripts/issue-server-cert.sh \
  hsm-service.local \
  "hsm-service.local,localhost" \
  "127.0.0.1"

# MySQL Server
./pki/scripts/issue-server-cert.sh \
  mysql-server.local \
  "mysql-server.local,mysql.local,mysql-master.local" \
  "10.0.0.5"

# ClickHouse Server
./pki/scripts/issue-server-cert.sh \
  clickhouse-server.local \
  "clickhouse-server.local,clickhouse.local" \
  "10.0.0.6"
```

**Output:**
- `pki/server/<cn>.crt` - Certificate
- `pki/server/<cn>.key` - Private key (0600 permissions)
- `pki/inventory.yaml` - Updated automatically

---

### Issue Client Certificate

**Syntax:**
```bash
./pki/scripts/issue-client-cert.sh <cn> <ou>
```

**Parameters:**
- `cn` - Common Name (e.g., `trading-service-1`)
- `ou` - Organizational Unit: `Trading`, `2FA`, `Database`, or `Admin`

**Examples:**

```bash
# Trading service (can use exchange-key context)
./pki/scripts/issue-client-cert.sh trading-service-1 Trading
./pki/scripts/issue-client-cert.sh trading-service-2 Trading

# 2FA service (can use 2fa context)
./pki/scripts/issue-client-cert.sh web-2fa-service 2FA
./pki/scripts/issue-client-cert.sh mobile-2fa-app 2FA

# Database client (mTLS only, no HSM access)
./pki/scripts/issue-client-cert.sh backend-app-1 Database
./pki/scripts/issue-client-cert.sh analytics-service Database
```

**Output:**
- `pki/client/<cn>.crt` - Certificate
- `pki/client/<cn>.key` - Private key (0600 permissions)
- `pki/inventory.yaml` - Updated automatically

---

### Revoke Certificate

**Syntax:**
```bash
./pki/scripts/revoke-cert.sh <cn> <reason>
```

**Parameters:**
- `cn` - Common Name of certificate to revoke
- `reason` - One of: `compromised`, `decommissioned`, `superseded`, `cessation`

**Reasons:**
- `compromised` - Private key or certificate compromised
- `decommissioned` - Service no longer in use
- `superseded` - Certificate replaced by new one
- `cessation` - Service ceased operation

**Example:**

```bash
# Revoke compromised certificate
./pki/scripts/revoke-cert.sh old-trading-service compromised

# Revoke decommissioned service
./pki/scripts/revoke-cert.sh test-service decommissioned
```

**What happens:**
1. Certificate is added to `pki/revoked.yaml`
2. After HSM service restart (or SIGHUP), certificate is rejected
3. Client will receive `403 Forbidden` on connection attempts

**To apply revocation:**
```bash
# Restart HSM service
docker-compose restart hsm-service

# OR send SIGHUP for hot reload (if implemented)
kill -HUP $(pgrep hsm-service)
```

---

## Certificate Inventory

The file `pki/inventory.yaml` automatically tracks all issued certificates.

**Example content:**

```yaml
certificates:
  servers:
    - cn: hsm-service.local
      ou: Services
      issued: '2026-01-06'
      expires: '2027-01-06'
      serial: '01'
      san_dns:
        - hsm-service.local
        - localhost
      san_ip:
        - 127.0.0.1
      file: server/hsm-service.local
  
  clients:
    - cn: trading-service-1
      ou: Trading
      issued: '2026-01-06'
      expires: '2027-01-06'
      serial: '02'
      access_contexts:
        - exchange-key
      file: client/trading-service-1
```

**View inventory:**
```bash
cat pki/inventory.yaml
```

**Find expiring certificates:**
```bash
# Find certificates expiring in next 30 days (manual check)
# TODO: Create script for this
```

---

## Revocation List

The file `pki/revoked.yaml` contains all revoked certificates.

**Example content:**

```yaml
revoked:
  - cn: old-trading-service
    serial: '05'
    revoked_date: '2026-01-03T10:00:00Z'
    reason: compromised
```

**View revoked list:**
```bash
cat pki/revoked.yaml
```

---

## Testing Certificates

### Verify Certificate

```bash
# Verify server certificate
openssl verify -CAfile pki/ca/ca.crt pki/server/hsm-service.local.crt

# Verify client certificate
openssl verify -CAfile pki/ca/ca.crt pki/client/trading-service-1.crt
```

### View Certificate Details

```bash
# View certificate
openssl x509 -in pki/server/hsm-service.local.crt -noout -text

# Check subject
openssl x509 -in pki/client/trading-service-1.crt -noout -subject

# Check SAN
openssl x509 -in pki/server/hsm-service.local.crt -noout -text | grep -A1 "Subject Alternative Name"

# Check expiration
openssl x509 -in pki/client/trading-service-1.crt -noout -dates
```

### Test mTLS Connection

**To HSM Service:**
```bash
curl -X POST https://hsm-service.local:8443/encrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{"context":"exchange-key","plaintext":"SGVsbG8gV29ybGQ="}'
```

**To MySQL:**
```bash
mysql \
  --host=192.168.50.5 \
  --user=testuser \
  --ssl-ca=pki/ca/ca.crt \
  --ssl-cert=pki/client/mysql-client-test.crt \
  --ssl-key=pki/client/mysql-client-test.key \
  -e "SELECT 'mTLS works!'"
```

**Expected output:**
```
+-------------+
| mTLS works! |
+-------------+
| mTLS works! |
+-------------+
```

**Negative tests (should fail):**

1. Connection without certificate:
```bash
mysql --host=192.168.50.5 --user=testuser -e "SELECT 'Test'"
# ERROR 1045 (28000): Access denied for user 'testuser'@'...' (using password: NO)
```

2. Connection with unsigned certificate:
```bash
# Create self-signed certificate (not from CA)
openssl req -new -newkey rsa:2048 -days 1 -nodes -x509 \
  -subj "/CN=fake-cert" -keyout /tmp/fake.key -out /tmp/fake.crt

# Try to connect
mysql --host=192.168.50.5 --user=testuser \
  --ssl-ca=pki/ca/ca.crt \
  --ssl-cert=/tmp/fake.crt \
  --ssl-key=/tmp/fake.key \
  -e "SELECT 'Test'"
# ERROR 2013 (HY000): Lost connection to MySQL server at 'reading authorization packet'
```

**Testing Results (2026-01-06):**
- ✅ mTLS connection to MySQL (192.168.50.5) - SUCCESS
- ✅ Connection without certificate - REJECTED (ERROR 1045)
- ✅ Connection with unsigned certificate - REJECTED (ERROR 2013)

---

## Security Best Practices

### 1. Protect Private Keys

**File permissions:**
```bash
# CA key (most critical)
chmod 400 pki/ca/ca.key
chown root:root pki/ca/ca.key

# Server/client keys
chmod 600 pki/server/*.key
chmod 600 pki/client/*.key
```

**Storage:**
- Store CA key on separate secure VM
- Use hardware security module (HSM) for production CA
- Never commit `.key` files to git (`.gitignore` configured)

### 2. Certificate Rotation

**Recommendations:**
- Rotate certificates every 365 days (current validity)
- Renew 30 days before expiration
- Use shorter validity (90 days) for higher security

**Renewal process:**
```bash
# Issue new certificate with same CN
./pki/scripts/issue-server-cert.sh <cn> <san-dns> <san-ip>

# Update service configuration
# Restart service

# (Optional) Revoke old certificate
./pki/scripts/revoke-cert.sh <cn> superseded
```

### 3. Audit

**Regular checks:**
```bash
# List all certificates
cat pki/inventory.yaml

# Check revoked list
cat pki/revoked.yaml

# Find certificates expiring soon
# TODO: Create monitoring script
```

---

## Troubleshooting

### "CA certificate or key not found"

**Problem:** CA files missing in `pki/ca/`

**Solution:**
```bash
# Ensure CA files exist
ls -la pki/ca/
# Should show: ca.crt, ca.key

# If missing, copy from CA VM:
scp ca-vm:/path/to/ca.crt pki/ca/
scp ca-vm:/path/to/ca.key pki/ca/
chmod 400 pki/ca/ca.key
```

### "Certificate verification failed"

**Problem:** Certificate not signed by CA

**Solution:**
```bash
# Check certificate chain
openssl verify -CAfile pki/ca/ca.crt pki/server/hsm-service.local.crt

# Re-issue certificate if needed
```

### "python3: command not found"

**Problem:** Python 3 not installed

**Solution:**
```bash
# Ubuntu/Debian
sudo apt-get install python3 python3-pip
pip3 install pyyaml

# Alpine (Docker)
apk add python3 py3-pip
pip3 install pyyaml
```

### Certificate already exists

**Problem:** Attempting to issue certificate with existing CN

**Solution:**
- Script will prompt to overwrite
- Choose 'y' to replace, 'n' to abort
- Old certificate will be backed up (manually if needed)

---

## Quick Reference

```bash
# Issue server cert
./pki/scripts/issue-server-cert.sh <cn> "<dns1>,<dns2>" "<ip1>,<ip2>"

# Issue client cert
./pki/scripts/issue-client-cert.sh <cn> <OU>

# Revoke cert
./pki/scripts/revoke-cert.sh <cn> <reason>

# Verify cert
openssl verify -CAfile pki/ca/ca.crt pki/client/<cn>.crt

# View inventory
cat pki/inventory.yaml

# View revoked list
cat pki/revoked.yaml

# Test mTLS
curl --cert pki/client/<cn>.crt --key pki/client/<cn>.key --cacert pki/ca/ca.crt https://...
```

---

## Next Steps

After setting up PKI:

1. **Issue certificates for HSM service:**
   ```bash
   ./pki/scripts/issue-server-cert.sh hsm-service.local "hsm-service.local,localhost" "127.0.0.1"
   ```

2. **Issue test client certificates:**
   ```bash
   ./pki/scripts/issue-client-cert.sh trading-service-1 Trading
   ./pki/scripts/issue-client-cert.sh web-2fa-service 2FA
   ```

3. **Test on MySQL** (if available):
   ```bash
   ./pki/scripts/issue-server-cert.sh mysql-server.local "mysql-server.local" "10.0.0.5"
   ./pki/scripts/issue-client-cert.sh mysql-client-test Database
   ```

4. **Configure HSM service** with certificates:
   - Update `config.yaml` with certificate paths
   - Start HSM service
   - Test mTLS connections

---

## References

- [ARCHITECTURE.md](../ARCHITECTURE.md) - System architecture
- [TECHNICAL_SPEC.md](../TECHNICAL_SPEC.md) - Technical specification
- [DEVELOPMENT_PLAN.md](../DEVELOPMENT_PLAN.md) - Development roadmap
- OpenSSL documentation: https://www.openssl.org/docs/
