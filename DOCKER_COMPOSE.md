# Docker Compose Quick Start

## Initial Setup

1. **Скопировать .env.example → .env**
   ```bash
   cp .env.example .env
   ```

2. **Отредактировать .env** (установить сильные PINs для продакшена)
   ```bash
   nano .env
   ```

3. **Убедиться что data/tokens существует**
   ```bash
   mkdir -p data/tokens
   chmod 755 data/tokens
   ```

## Basic Commands

### Start service
```bash
docker-compose up -d
```

### View logs
```bash
docker-compose logs -f
```

### Stop service
```bash
docker-compose down
```

### Restart service
```bash
docker-compose restart
```

### Rebuild and restart
```bash
docker-compose up -d --build
```

## Service Management

### Check status
```bash
docker-compose ps
```

### View health status
```bash
docker-compose ps hsm-service
# or
docker inspect --format='{{.State.Health.Status}}' hsm-service
```

### Execute commands in container
```bash
# Interactive shell
docker-compose exec hsm-service /bin/sh

# List KEKs
docker-compose exec hsm-service /app/hsm-admin list-kek

# Check SoftHSM token
docker-compose exec hsm-service softhsm2-util --show-slots
```

## Testing

### Health check
```bash
curl -k https://localhost:8443/health \
  --cert pki/client/trading-service-1.crt \
  --key pki/client/trading-service-1.key \
  --cacert pki/ca/ca.crt
```

### Encrypt test
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

## KEK Management

### List KEKs
```bash
docker-compose exec hsm-service /app/hsm-admin list-kek --verbose
```

### Export KEK metadata
```bash
docker-compose exec hsm-service /app/hsm-admin export-metadata \
  --output /tmp/kek-metadata.json

docker cp hsm-service:/tmp/kek-metadata.json ./kek-metadata.json
```

### Create new KEK (manual process)
```bash
# Option 1: Use create-kek utility directly
docker-compose exec hsm-service /app/create-kek "kek-new-service-v1" "05" "1234"

# Option 2: See instructions from hsm-admin
docker-compose exec hsm-service /app/hsm-admin create-kek

# After creating KEK:
# 1. Add new KEK to config.yaml
# 2. Restart service: docker-compose restart
```

## Maintenance

### View logs with timestamps
```bash
docker-compose logs -f --timestamps
```

### Check resource usage
```bash
docker stats hsm-service
```

### Inspect volumes
```bash
docker-compose exec hsm-service ls -lh /var/lib/softhsm/tokens/
docker-compose exec hsm-service ls -lh /app/pki/
```

### Backup token data
```bash
# Stop service
docker-compose down

# Backup tokens
tar -czf hsm-tokens-backup-$(date +%Y%m%d).tar.gz data/tokens/

# Start service
docker-compose up -d
```

### Restore token data
```bash
# Stop service
docker-compose down

# Restore tokens
rm -rf data/tokens/*
tar -xzf hsm-tokens-backup-20260108.tar.gz

# Start service
docker-compose up -d
```

## Network

### Inspect network
```bash
docker network inspect hsm-net
```

### Check service connectivity
```bash
docker-compose exec hsm-service netstat -tuln
docker-compose exec hsm-service wget -O- --no-check-certificate https://localhost:8443/health
```

## Troubleshooting

### Service won't start
```bash
# Check logs
docker-compose logs hsm-service

# Check config syntax
docker-compose config

# Validate volumes
ls -la data/tokens/
ls -la pki/
ls -la config.yaml
```

### Token initialization issues
```bash
# Check token status
docker-compose exec hsm-service softhsm2-util --show-slots

# Reinitialize token (CAUTION: destroys existing token!)
docker-compose down
rm -rf data/tokens/*
docker-compose up -d
```

### Permission issues
```bash
# Fix volume permissions
chmod 755 data/tokens
chmod 644 config.yaml
chmod 644 softhsm2.conf
chmod -R 644 pki/ca/*.crt
chmod -R 644 pki/server/*.crt
chmod -R 600 pki/server/*.key
```

### Health check failing
```bash
# Check health check logs
docker inspect hsm-service | jq '.[0].State.Health'

# Manual health check from inside container
docker-compose exec hsm-service wget --spider --no-check-certificate https://localhost:8443/health
```

## Production Deployment

### Security checklist
- [ ] Change default PINs in .env
- [ ] Set .env permissions to 600
- [ ] Use strong TLS certificates (not self-signed)
- [ ] Configure firewall rules
- [ ] Enable Docker secrets (instead of .env)
- [ ] Set up log rotation
- [ ] Configure monitoring/alerting
- [ ] Regular token backups
- [ ] Document recovery procedures

### Docker secrets (production)
```yaml
# docker-compose.yml modification for secrets
services:
  hsm-service:
    secrets:
      - hsm_pin
      - hsm_so_pin
    environment:
      - HSM_PIN_FILE=/run/secrets/hsm_pin
      - HSM_SO_PIN_FILE=/run/secrets/hsm_so_pin

secrets:
  hsm_pin:
    file: ./secrets/hsm_pin.txt
  hsm_so_pin:
    file: ./secrets/hsm_so_pin.txt
```

### Monitoring
```bash
# Set up healthcheck monitoring
while true; do
  STATUS=$(docker inspect --format='{{.State.Health.Status}}' hsm-service)
  if [ "$STATUS" != "healthy" ]; then
    echo "ALERT: HSM service unhealthy: $STATUS"
    # Send alert (email/SMS/Slack)
  fi
  sleep 60
done
```

## Clean Up

### Remove all data
```bash
docker-compose down -v
rm -rf data/tokens/*
```

### Remove image
```bash
docker-compose down --rmi all
```

### Full cleanup
```bash
docker-compose down -v --rmi all
rm -rf data/tokens/*
```
