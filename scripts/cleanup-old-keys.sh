#!/bin/bash
# HSM Old Key Cleanup Script
# Удаляет старые ключи после завершения overlap period

set -euo pipefail

# Конфигурация
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
LOG_FILE="/var/log/hsm-rotation.log"
OVERLAP_PERIOD_DAYS=7

# Функции
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m'

error() { echo -e "${RED}✗${NC} $*"; log "ERROR: $*"; }
success() { echo -e "${GREEN}✓${NC} $*"; log "SUCCESS: $*"; }
warning() { echo -e "${YELLOW}⚠${NC} $*"; log "WARNING: $*"; }

echo "=========================================="
echo "   HSM Old Key Cleanup"
echo "=========================================="
echo ""

# Поиск старых ключей в metadata.yaml
OLD_CONTEXTS=$(grep -E '^\s+[a-z0-9-]+-old:' "$PROJECT_DIR/metadata.yaml" | sed 's/://g' | tr -d ' ' || true)

if [ -z "$OLD_CONTEXTS" ]; then
    success "No old keys found in metadata"
    exit 0
fi

echo "Found old key contexts:"
echo "$OLD_CONTEXTS"
echo ""

while read -r context; do
    if [ -z "$context" ]; then
        continue
    fi
    
    # Получаем label и created_at из metadata.yaml
    LABEL=$(grep -A 5 "^  $context:" "$PROJECT_DIR/metadata.yaml" | grep "label:" | awk '{print $2}')
    CREATED_AT=$(grep -A 5 "^  $context:" "$PROJECT_DIR/metadata.yaml" | grep "created_at:" | awk '{print $2}')
    
    if [ -z "$LABEL" ]; then
        warning "Could not find label for context: $context"
        continue
    fi
    
    echo "Checking key: $LABEL"
    echo "  Context: $context"
    echo "  Created: ${CREATED_AT:-unknown}"
    
    if [ -n "$CREATED_AT" ]; then
        # Вычисляем возраст ключа
        CREATED_TIMESTAMP=$(date -d "$CREATED_AT" +%s 2>/dev/null || echo "0")
        NOW_TIMESTAMP=$(date +%s)
        AGE_DAYS=$(( (NOW_TIMESTAMP - CREATED_TIMESTAMP) / 86400 ))
        
        echo "  Age: $AGE_DAYS days"
        
        if [ "$AGE_DAYS" -lt "$OVERLAP_PERIOD_DAYS" ]; then
            warning "Key is only $AGE_DAYS days old, overlap period not complete"
            echo "  Minimum age required: $OVERLAP_PERIOD_DAYS days"
            echo "  Please wait $((OVERLAP_PERIOD_DAYS - AGE_DAYS)) more days"
            echo ""
            continue
        fi
    fi
    
    # Запрос подтверждения
    echo ""
    echo -e "${YELLOW}❓${NC} Delete key $LABEL from HSM? (yes/no): "
    read -r response
    
    if [ "$response" = "yes" ]; then
        # Удаление ключа из HSM
        docker exec hsm-service /app/hsm-admin delete-kek --label "$LABEL" --confirm || {
            error "Failed to delete key from HSM: $LABEL"
            continue
        }
        
        success "Key deleted from HSM: $LABEL"
        
        # Удаление из metadata.yaml
        echo "Remove from metadata.yaml? (yes/no): "
        read -r response2
        
        if [ "$response2" = "yes" ]; then
            # Создаем резервную копию
            BACKUP_META="$PROJECT_DIR/metadata.yaml.backup-$(date +%Y%m%d-%H%M%S)"
            cp "$PROJECT_DIR/metadata.yaml" "$BACKUP_META"
            success "Metadata backed up to: $BACKUP_META"
            
            # Используем Python для удаления из metadata.yaml
            python3 << PYTHON_METADATA
import yaml

with open('$PROJECT_DIR/metadata.yaml', 'r') as f:
    metadata = yaml.safe_load(f)

# Удаляем ключ
if '$context' in metadata['rotation']:
    del metadata['rotation']['$context']
    print("✓ Removed from metadata: $context")

with open('$PROJECT_DIR/metadata.yaml', 'w') as f:
    yaml.dump(metadata, f, default_flow_style=False, sort_keys=False)
PYTHON_METADATA
            
            success "Removed from metadata.yaml: $context"
            
            # Удаление из config.yaml (ACL mappings и key definition)
            BACKUP_CONFIG="$PROJECT_DIR/config.yaml.backup-$(date +%Y%m%d-%H%M%S)"
            cp "$PROJECT_DIR/config.yaml" "$BACKUP_CONFIG"
            success "Config backed up to: $BACKUP_CONFIG"
            
            python3 << PYTHON_CONFIG
import yaml

with open('$PROJECT_DIR/config.yaml', 'r') as f:
    config = yaml.safe_load(f)

# Удаляем ключ из config
if '$context' in config['hsm']['keys']:
    del config['hsm']['keys']['$context']
    print("✓ Removed from config: $context")

# Удаляем из ACL mappings
for ou, contexts in config['acl']['mappings'].items():
    if '$context' in contexts:
        contexts.remove('$context')

with open('$PROJECT_DIR/config.yaml', 'w') as f:
    yaml.dump(config, f, default_flow_style=False, sort_keys=False)
PYTHON_CONFIG
            
            success "Removed from config.yaml: $context"
            
            # Перезапуск сервиса
            echo "Restart service to apply changes? (yes/no): "
            read -r response3
            
            if [ "$response3" = "yes" ]; then
                cd "$PROJECT_DIR"
                docker compose restart hsm-service
                sleep 3
                
                # Проверка здоровья
                if curl -sk https://localhost:8443/health 2>&1 | grep -q '"status":"healthy"'; then
                    success "Service restarted successfully"
                else
                    error "Service health check failed! Rolling back..."
                    cp "$BACKUP_META" "$PROJECT_DIR/metadata.yaml"
                    cp "$BACKUP_CONFIG" "$PROJECT_DIR/config.yaml"
                    docker compose restart hsm-service
                fi
            fi
        fi
    else
        warning "Skipped: $LABEL"
    fi
    
    echo ""
done <<< "$OLD_CONTEXTS"

echo "=========================================="
echo "Cleanup completed"
echo "=========================================="
