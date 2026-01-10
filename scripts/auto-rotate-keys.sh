#!/bin/bash
# HSM Auto Rotation - Simple wrapper for hsm-admin
# Автоматическая ротация ключей через hsm-admin + KeyManager hot reload
#
# Phase 4: Использует hsm-admin rotate + автоматическое обнаружение изменений
# KeyManager автоматически перезагрузит ключи в течение 30 секунд (zero downtime!)

set -euo pipefail

# Конфигурация
LOG_FILE="${HSM_ROTATION_LOG:-/var/log/hsm-auto-rotation.log}"
DRY_RUN="${DRY_RUN:-false}"

# Цвета для логирования
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# Функции логирования
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

info() {
    echo -e "${BLUE}ℹ ${NC}$*"
    log "INFO: $*"
}

success() {
    echo -e "${GREEN}✓${NC} $*"
    log "SUCCESS: $*"
}

warning() {
    echo -e "${YELLOW}⚠${NC} $*"
    log "WARNING: $*"
}

error() {
    echo -e "${RED}✗${NC} $*"
    log "ERROR: $*"
}

# Проверка Docker
if ! docker info >/dev/null 2>&1; then
    error "Docker is not running"
    exit 1
fi

# Проверка контейнера HSM
if ! docker ps | grep -q hsm-service; then
    error "hsm-service container is not running"
    exit 1
fi

info "Checking keys requiring rotation..."

# Получить список ключей, требующих ротации
KEYS=$(docker exec hsm-service /app/hsm-admin rotation-status 2>&1 | \
       grep "NEEDS ROTATION" -B 5 | \
       grep "Context:" | \
       awk '{print $2}' || true)

if [ -z "$KEYS" ]; then
    success "No keys need rotation at this time"
    exit 0
fi

info "Keys requiring rotation:"
echo "$KEYS" | while read -r key; do
    echo "  - $key"
done

if [ "$DRY_RUN" = "true" ]; then
    warning "DRY RUN mode - no rotation will be performed"
    exit 0
fi

# Счётчики
SUCCESS_COUNT=0
FAIL_COUNT=0

# Ротация каждого ключа
echo "$KEYS" | while read -r context; do
    info "Rotating key context: $context"
    
    if docker exec hsm-service /app/hsm-admin rotate "$context"; then
        success "Rotation completed: $context"
        SUCCESS_COUNT=$((SUCCESS_COUNT + 1))
        
        # KeyManager автоматически обнаружит изменения в metadata.yaml
        # в течение 30 секунд и перезагрузит ключи (zero downtime!)
        info "KeyManager will automatically reload keys within 30 seconds"
        
    else
        error "Rotation failed: $context"
        FAIL_COUNT=$((FAIL_COUNT + 1))
    fi
    
    echo ""
done

# Итог
echo "=========================================="
echo "Auto-rotation summary:"
echo "  Successful: $SUCCESS_COUNT"
echo "  Failed: $FAIL_COUNT"
echo "=========================================="

if [ "$FAIL_COUNT" -gt 0 ]; then
    error "Some rotations failed - check logs"
    exit 1
fi

success "All rotations completed successfully"
exit 0
