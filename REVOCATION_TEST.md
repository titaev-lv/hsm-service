# Certificate Revocation Test (Test 11.3)

## Overview
Test 11.3 in `tests/integration/full-integration-test.sh` provides a comprehensive test of the certificate revocation and ACL blocking mechanism.

## What It Tests

### Pre-Revocation State
- Uses a valid trading client certificate (`pki/test/client/trading-client-1.crt`)
- Verifies the certificate is **accepted** with HTTP 200 before revocation
- Confirms normal operation: `/encrypt` endpoint responds successfully

### Revocation Process
- Dynamically adds the certificate CN to `revoked.yaml` with proper format:
  ```yaml
  revoked_certificates:
    - cn: "trading-client-1"
      serial: "01"
      reason: "Test revocation"
      date: "2026-02-05T16:00:00Z"
  ```
- Creates backup of original file for restore

### Auto-Reload Detection
- Waits up to 35 seconds for the ACL auto-reload to detect the change
- Default reload interval: 30 seconds (see `acl.go` line 46)
- Polls every 0.5 seconds to detect when revocation takes effect
- Shows progress every 5 seconds (helpful for debugging)

### Post-Revocation Validation
- Verifies the same certificate is now **rejected** with HTTP 403
- Checks error message contains "revoked", "forbidden", or "access denied"
- Confirms ACL checker properly blocks the revoked certificate

### Cleanup
- Restores original `revoked.yaml` to empty state
- Ensures subsequent tests are not affected

## Expected Behavior

| Phase | Request | Expected Response | Status Code |
|-------|---------|-------------------|-------------|
| Before | Valid cert | Encrypt response | 200 |
| After | Same cert | ACL error | 403 |

## Test Execution Flow

```
1. Extract CN from trading-client-1.crt
2. Send /encrypt request → HTTP 200 (should pass)
3. Add CN to revoked.yaml
4. Wait for auto-reload to detect change
5. Send /encrypt request → HTTP 403 (should fail)
6. Restore revoked.yaml to original
```

## Key Implementation Details

- **YAML Format**: Uses `revoked_certificates:` key (matches Go struct)
- **CN Extraction**: Uses OpenSSL to extract CN from certificate
- **Atomic File Updates**: Backup/restore pattern prevents state corruption
- **Timeout Handling**: 35-second wait accounts for max reload interval
- **Progress Feedback**: Shows elapsed time to help debug slow reloads

## Related Files

- **Code**: [internal/server/acl.go](internal/server/acl.go) - ACL checker with auto-reload
- **Tests**: [internal/server/acl_reload_test.go](internal/server/acl_reload_test.go) - Unit tests for reload
- **YAML Format**: [revoked.yaml](revoked.yaml) - Default empty revocation list

## Troubleshooting

### Test Fails at Step 1
**Problem**: Certificate rejected before revocation  
**Cause**: ACL checker may have old revoked.yaml from previous test  
**Solution**: Ensure cleanup step runs in previous test

### Test Timeout During Reload Wait
**Problem**: Auto-reload not detected within 35 seconds  
**Cause**: May be normal if reload interval is very long  
**Solution**: Check service logs for reload errors
```bash
docker logs hsm-service | grep -i "revoked.yaml\|reload"
```

### Test Fails at Step 4
**Problem**: Revoked certificate still accepted  
**Cause**: Auto-reload may not be working  
**Solution**: Check if `revoked.yaml` was properly modified
```bash
cat revoked.yaml
```
