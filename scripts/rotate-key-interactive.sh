#!/bin/bash
# HSM Key Rotation - Interactive Script
# Выполняет полный цикл ротации ключа с подтверждением на каждом шаге

set -euo pipefail

# Цвета для вывода
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Конфигурация
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
LOG_FILE="/var/log/hsm-rotation.log"
BACKUP_DIR="/var/backups/hsm"

# HSM PIN
HSM_PIN="${HSM_PIN:-}"

# Функции
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

confirm() {
    local prompt="$1"
    local response
    
    echo -e "${YELLOW}❓${NC} $prompt (yes/no): "
    read -r response
    
    if [ "$response" != "yes" ]; then
        warning "Operation cancelled by user"
        exit 0
    fi
}

# Проверка прав root
if [ "$EUID" -ne 0 ]; then 
    error "This script must be run as root (sudo)"
    exit 1
fi

# Проверка HSM_PIN
if [ -z "$HSM_PIN" ]; then
    error "HSM_PIN environment variable is not set"
    echo "Please set: export HSM_PIN=your_pin"
    exit 1
fi

# Создание директории для бэкапов
mkdir -p "$BACKUP_DIR"

echo ""
echo "=========================================="
echo "   HSM Key Rotation - Interactive Mode"
echo "=========================================="
echo ""

# Шаг 1: Проверка текущего статуса
info "Step 1: Checking current rotation status..."
docker exec hsm-service /app/hsm-admin rotation-status

echo ""
confirm "Continue with rotation?"

# Шаг 2: Выбор ключа для ротации
echo ""
info "Step 2: Select key to rotate"
echo ""
echo "Available keys needing rotation:"
docker exec hsm-service /app/hsm-admin rotation-status | grep "NEEDS ROTATION" -B 5 || {
    success "No keys need rotation at this time"
    exit 0
}

echo ""
echo "Enter the key label to rotate (e.g., kek-exchange-v1):"
read -r OLD_KEY_LABEL

if [ -z "$OLD_KEY_LABEL" ]; then
    error "Key label cannot be empty"
    exit 1
fi

# Валидация формата
if ! echo "$OLD_KEY_LABEL" | grep -qP '^[a-z0-9-]+-v\d+$'; then
    error "Invalid key label format. Expected: <name>-v<number>"
    exit 1
fi

# Извлечение базового имени и версии
BASE_NAME=$(echo "$OLD_KEY_LABEL" | sed 's/-v[0-9]*$//')
OLD_VERSION=$(echo "$OLD_KEY_LABEL" | grep -oP 'v\K\d+$')
NEW_VERSION=$((OLD_VERSION + 1))
NEW_KEY_LABEL="${BASE_NAME}-v${NEW_VERSION}"

info "Old key: $OLD_KEY_LABEL (version $OLD_VERSION)"
info "New key: $NEW_KEY_LABEL (version $NEW_VERSION)"

echo ""
confirm "Proceed with rotation from v$OLD_VERSION to v$NEW_VERSION?"

# Шаг 3: Резервное копирование
echo ""
info "Step 3: Creating backups..."

TIMESTAMP=$(date +%Y%m%d-%H%M%S)
BACKUP_CONFIG="$BACKUP_DIR/config.yaml.$TIMESTAMP"
BACKUP_TOKEN="$BACKUP_DIR/token.$TIMESTAMP.tar.gz"

# Бэкап config.yaml
cp "$PROJECT_DIR/config.yaml" "$BACKUP_CONFIG"
success "Config backed up to: $BACKUP_CONFIG"

# Бэкап HSM token
docker run --rm -v "$(pwd)/data/tokens:/tokens" alpine tar czf - -C /tokens . > "$BACKUP_TOKEN"
success "HSM token backed up to: $BACKUP_TOKEN"

# Шаг 4: Создание нового ключа
echo ""
info "Step 4: Creating new KEK..."

# Генерация нового ID (используем версию как ID)
NEW_ID=$(printf "%02d" "$NEW_VERSION")

info "Executing: /app/create-kek $NEW_KEY_LABEL $NEW_ID <pin> $NEW_VERSION"
docker exec -e HSM_PIN="$HSM_PIN" hsm-service \
    /app/create-kek "$NEW_KEY_LABEL" "$NEW_ID" "$HSM_PIN" "$NEW_VERSION" || {
    error "Failed to create new KEK"
    exit 1
}

success "New KEK created: $NEW_KEY_LABEL"

# Шаг 5: Обновление конфигурации
echo ""
info "Step 5: Updating configuration for overlap period..."

# Определение контекста из config.yaml
CONTEXT=$(grep -B 2 "label: $OLD_KEY_LABEL" "$PROJECT_DIR/config.yaml" | grep -oP '^\s+\K[^:]+(?=:)' | head -1)

if [ -z "$CONTEXT" ]; then
    error "Could not find context for key $OLD_KEY_LABEL in config.yaml"
    exit 1
fi

info "Found context: $CONTEXT"

# Создаем временный Python скрипт для обновления YAML
cat > /tmp/update_hsm_config.py << 'PYTHON_SCRIPT'
import sys
import yaml
from datetime import datetime

config_file = sys.argv[1]
context = sys.argv[2]
old_label = sys.argv[3]
new_label = sys.argv[4]
new_version = int(sys.argv[5])

with open(config_file, 'r') as f:
    config = yaml.safe_load(f)

# Получаем старую конфигурацию ключа
old_key_config = config['hsm']['keys'][context].copy()

# Создаем новый ключ
config['hsm']['keys'][context] = {
    'label': new_label,
    'type': old_key_config['type'],
    'rotation_interval': old_key_config.get('rotation_interval', '2160h'),
    'version': new_version,
    'created_at': datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ')
}

# Сохраняем старый ключ с суффиксом -old
old_context = context + '-old'
config['hsm']['keys'][old_context] = old_key_config

# Обновляем ACL mappings
for ou, contexts in config['acl']['mappings'].items():
    if context in contexts and old_context not in contexts:
        contexts.append(old_context)

with open(config_file, 'w') as f:
    yaml.dump(config, f, default_flow_style=False, sort_keys=False)

print(f"✓ Config updated: {context} -> {new_label} (v{new_version})")
print(f"✓ Old key preserved: {old_context} -> {old_label}")
PYTHON_SCRIPT

python3 /tmp/update_hsm_config.py \
    "$PROJECT_DIR/config.yaml" \
    "$CONTEXT" \
    "$OLD_KEY_LABEL" \
    "$NEW_KEY_LABEL" \
    "$NEW_VERSION"

rm /tmp/update_hsm_config.py

success "Configuration updated for overlap period"

# Шаг 6: Перезапуск сервиса
echo ""
info "Step 6: Restarting HSM service..."

cd "$PROJECT_DIR"
docker compose restart hsm-service

sleep 5

# Проверка здоровья
HEALTH=$(docker exec hsm-service wget -qO- --no-check-certificate https://localhost:8443/health 2>&1 || true)

if echo "$HEALTH" | grep -q '"status":"healthy"'; then
    success "Service restarted successfully and is healthy"
else
    error "Service health check failed!"
    echo "$HEALTH"
    warning "Rolling back to backup..."
    cp "$BACKUP_CONFIG" "$PROJECT_DIR/config.yaml"
    docker compose restart hsm-service
    exit 1
fi

# Проверка наличия обоих ключей
echo ""
info "Verifying both keys are available..."
docker exec hsm-service /app/hsm-admin rotation-status

# Шаг 7: Инструкции
echo ""
echo "=========================================="
echo -e "${GREEN}✓ KEY ROTATION COMPLETED${NC}"
echo "=========================================="
echo ""
echo "OVERLAP PERIOD (7 days) - Both keys are active:"
echo "  • New encryptions will use: $NEW_KEY_LABEL"
echo "  • Old data can be decrypted with: $OLD_KEY_LABEL"
echo ""
echo "NEXT STEPS:"
echo "1. ✓ New key created and loaded"
echo "2. ⚠  Re-encrypt all data encrypted with $OLD_KEY_LABEL"
echo "3. ⏰ Wait 7 days for overlap period"
echo "4. ⚠  Delete old key after verification:"
echo "   docker exec hsm-service /app/hsm-admin delete-kek --label $OLD_KEY_LABEL --confirm"
echo ""
echo "BACKUPS CREATED:"
echo "  Config: $BACKUP_CONFIG"
echo "  Token:  $BACKUP_TOKEN"
echo ""
echo "MONITORING:"
echo "  Check status: docker exec hsm-service /app/hsm-admin rotation-status"
echo "  Health check: curl -sk https://localhost:8443/health | jq"
echo ""
echo "See $PROJECT_DIR/KEY_ROTATION.md for full documentation"
echo ""

log "Rotation completed: $OLD_KEY_LABEL -> $NEW_KEY_LABEL"
