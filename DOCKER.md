# Docker Quick Reference

## Build

```bash
docker build -t hsm-service:latest .
```

## Run (standalone)

```bash
# Basic run with default PIN
docker run -d \
  --name hsm-service \
  -p 8443:8443 \
  -e HSM_PIN=1234 \
  hsm-service:latest

# With persistent token storage
docker run -d \
  --name hsm-service \
  -p 8443:8443 \
  -e HSM_PIN=1234 \
  -v $(pwd)/data/tokens:/var/lib/softhsm/tokens \
  -v $(pwd)/pki:/app/pki:ro \
  hsm-service:latest
```

## Exec into container

```bash
# Interactive shell
docker exec -it hsm-service /bin/sh

# List KEKs
docker exec hsm-service /app/hsm-admin list-kek

# Export metadata
docker exec hsm-service /app/hsm-admin export-metadata --output /tmp/metadata.json
docker cp hsm-service:/tmp/metadata.json ./kek-metadata.json
```

## Logs

```bash
docker logs hsm-service
docker logs -f hsm-service  # Follow
```

## Stop/Start

```bash
docker stop hsm-service
docker start hsm-service
docker restart hsm-service
```

## Remove

```bash
docker rm -f hsm-service
docker rmi hsm-service:latest
```

## Image Info

```bash
# Check image size
docker images hsm-service:latest

# Inspect image
docker inspect hsm-service:latest

# Image history
docker history hsm-service:latest
```

## Health Check

```bash
# Manual health check (from host)
curl -k https://localhost:8443/health \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt

# Docker health status
docker inspect --format='{{.State.Health.Status}}' hsm-service
```

## Troubleshooting

### Check SoftHSM token
```bash
docker exec hsm-service softhsm2-util --show-slots
```

### List PKCS#11 objects
```bash
docker exec hsm-service pkcs11-tool \
  --module /usr/lib/softhsm/libsofthsm2.so \
  --list-objects \
  --login --pin 1234
```

### Check config
```bash
docker exec hsm-service cat /app/config.yaml
```

### Test encryption
```bash
curl -k -X POST https://localhost:8443/encrypt \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt \
  -H "Content-Type: application/json" \
  -d '{
    "context": "exchange-key",
    "plaintext": "SGVsbG8gV29ybGQh"
  }'
```

## Build Optimizations

### Build without cache
```bash
docker build --no-cache -t hsm-service:latest .
```

### Multi-platform build
```bash
docker buildx build \
  --platform linux/amd64,linux/arm64 \
  -t hsm-service:latest \
  .
```

### Reduce image size
- Already using multi-stage build ✓
- Alpine base image (52.4 MB) ✓
- .dockerignore configured ✓

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `HSM_PIN` | `1234` | HSM token PIN |
| `HSM_SO_PIN` | `12345678` | HSM Security Officer PIN |
| `HSM_TOKEN_LABEL` | `hsm-token` | Token label |
| `CONFIG_PATH` | `/app/config.yaml` | Config file path |

## Volumes

| Path | Purpose |
|------|---------|
| `/var/lib/softhsm/tokens` | Persistent token storage |
| `/app/pki` | PKI certificates (mount read-only) |
| `/app/config.yaml` | Configuration file |

## Exposed Ports

| Port | Protocol | Description |
|------|----------|-------------|
| `8443` | HTTPS | Main service API |
