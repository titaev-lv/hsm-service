# Certificate Revocation Test - Test 11.3

## Summary

Test 11.3 in the full integration test suite provides a **production-grade** certificate revocation verification that tests the complete ACL revocation workflow.

## Why This Test Matters

Previously, Test 11.3 simply checked if a certificate was already in `revoked.yaml`. The new implementation:

✅ **Dynamically revokes** a certificate at runtime  
✅ **Validates pre-state**: Certificate works before revocation  
✅ **Simulates real-world**: Adds certificate to revocation list while service running  
✅ **Tests auto-reload**: Verifies ACL checker detects changes within 30 seconds  
✅ **Validates blocking**: Confirms HTTP 403 response and error message  
✅ **Cleanup**: Restores original state without affecting other tests  

## Test Flow (5 Steps)

### Step 1: Pre-Revocation Verification
```bash
GET /encrypt with trading-client-1.crt
→ HTTP 200 (Certificate accepted)
```

**Validates**: Certificate works before adding to revocation list

### Step 2: Add to Revoked List
```bash
cat > revoked.yaml << EOF
revoked_certificates:
  - cn: "trading-client-1"
    serial: "01"
    reason: "Test revocation"
    date: "2026-02-05T16:00:00Z"
EOF
```

**Validates**: YAML format matches Go struct expectations

### Step 3: Wait for Auto-Reload
```bash
for 0.5s intervals up to 35 seconds:
  GET /encrypt with trading-client-1.crt
  → HTTP 403? (Revocation detected)
```

**Validates**: ACL checker reloads within expected timeframe (default: 30s)  
**Shows progress**: Reports elapsed time every 5 seconds

### Step 4: Post-Revocation Validation
```bash
GET /encrypt with trading-client-1.crt
→ HTTP 403 (Certificate revoked)
→ Body contains: "revoked" | "forbidden" | "access denied"
```

**Validates**: Revoked certificate properly blocked by ACL

### Step 5: Cleanup
```bash
mv revoked.yaml.backup revoked.yaml
```

**Validates**: Test isolation - subsequent tests unaffected

## Technical Details

| Aspect | Implementation |
|--------|-----------------|
| **Certificate Source** | `pki/test/client/trading-client-1.crt` |
| **CN Extraction** | OpenSSL: `openssl x509 -in cert -noout -subject` |
| **YAML Format** | `revoked_certificates:` (matches RevokedList struct) |
| **Reload Interval** | 30 seconds (default from acl.go) |
| **Wait Timeout** | 35 seconds (1.17x reload interval for safety) |
| **Check Interval** | 0.5 seconds (fast feedback) |
| **HTTP Success Code** | 200 (before revocation) |
| **HTTP Blocked Code** | 403 (after revocation) |

## File Changes Made

- [tests/integration/full-integration-test.sh](tests/integration/full-integration-test.sh)
  - Replaced lines 1022-1059 (37 lines)
  - With new implementation: ~115 lines
  - Added robust error handling and progress feedback

- [REVOCATION_TEST.md](REVOCATION_TEST.md)
  - New documentation file
  - Troubleshooting guide
  - Expected behaviors table

## Git Commits

```
3b052ee - Add full-featured certificate revocation test (Test 11.3)
382ccbd - Document certificate revocation test methodology
```

## Testing the Test

### Run the full integration test:
```bash
cd /home/dev/docker/ct-system/services/hsm-service
bash tests/integration/full-integration-test.sh
```

### Expected output during Test 11.3:
```
[TEST 55/59] Test 11.3: Dynamic certificate revocation and ACL blocking
ℹ Using certificate CN: trading-client-1
ℹ Step 1: Testing certificate works before revocation...
✓ Certificate accepted before revocation (HTTP 200)
ℹ Step 2: Adding certificate to revoked.yaml...
✓ Certificate added to revoked.yaml
ℹ New revoked.yaml content:
  revoked_certificates:
    - cn: "trading-client-1"
      serial: "01"
      reason: "Test revocation"
      date: "2026-02-05T16:00:00Z"
ℹ Step 3: Waiting for ACL auto-reload (up to 35 seconds)...
ℹ Waiting... (5 seconds elapsed)
ℹ Waiting... (10 seconds elapsed)
✓ Certificate revocation detected after 31 seconds
ℹ Step 4: Verifying revoked certificate is blocked...
✓ Revoked certificate blocked by ACL (HTTP 403)
ℹ Step 5: Restoring revoked.yaml to original state...
✓ revoked.yaml restored
```

## Key Insights

1. **Reload Timing**: Actual detection time depends on when file modification occurs relative to the 30-second check cycle
2. **Atomic Updates**: File is moved to prevent partial reads during auto-reload
3. **Error Messages**: ACL returns generic "certificate revoked" without exposing CN
4. **Progress Feedback**: Test shows waiting progress to help debug slow systems

## Related Code

- [acl.go](internal/server/acl.go) - Lines 106-123: TryReload() function
- [acl_reload_test.go](internal/server/acl_reload_test.go) - Unit tests for reload
- [acl.go](internal/server/acl.go) - Lines 165-183: LoadRevoked() function

## Troubleshooting

### Test hangs at "Waiting for ACL auto-reload"

**Check service logs:**
```bash
docker logs ct-system-hsm-service-1 | grep -i "revoked\|reload"
```

**Possible causes:**
- Auto-reload goroutine crashed
- File modification not detected
- Reload interval very long

### Test fails: "Server accepted revoked certificate!"

**Verify revoked.yaml was created:**
```bash
cat revoked.yaml
ls -la revoked.yaml
```

**Check file wasn't created as directory:**
```bash
stat revoked.yaml  # Should show "regular file"
```

**Check ACL initialization:**
```bash
docker exec ct-system-hsm-service-1 ls -la /app/revoked.yaml
```

### Test fails: "Certificate rejected before revocation"

**Cause**: ACL configuration issue or revoked list has stale data  
**Solution**: Check if certificate exists in revoked.yaml from previous test  
**Fix**: Ensure cleanup step completed in previous test
