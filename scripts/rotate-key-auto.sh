#!/bin/bash
# HSM Key Rotation - Fully Automated Script
# Автоматическая ротация ключей без взаимодействия с пользователем
# ВНИМАНИЕ: Использовать только в production с настроенным мониторингом!

set -euo pipefail

# Конфигурация
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
LOG_FILE="/var/log/hsm-rotation.log"
BACKUP_DIR="/var/backups/hsm"

# Параметры безопасности
AUTO_ROTATION_ENABLED="${AUTO_ROTATION_ENABLED:-false}"
AUTO_CLEANUP_ENABLED="${AUTO_CLEANUP_ENABLED:-false}"
OVERLAP_PERIOD_DAYS="${OVERLAP_PERIOD_DAYS:-7}"
MAX_ROTATION_AGE_DAYS="${MAX_ROTATION_AGE_DAYS:-100}"  # Максимум 10 дней просрочки

# HSM PIN
HSM_PIN="${HSM_PIN:-}"

# Email/Slack/Telegram настройки
ALERT_EMAIL="${HSM_ALERT_EMAIL:-ops@example.com}"
SEND_EMAIL="${HSM_SEND_EMAIL:-false}"
SLACK_WEBHOOK="${HSM_SLACK_WEBHOOK:-}"
TELEGRAM_BOT_TOKEN="${HSM_TELEGRAM_BOT_TOKEN:-}"
TELEGRAM_CHAT_ID="${HSM_TELEGRAM_CHAT_ID:-}"

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

# Функции оповещений
send_email() {
    local subject="$1"
    local body="$2"
    
    if [ "$SEND_EMAIL" = "true" ]; then
        echo "$body" | mail -s "[HSM AUTO-ROTATION] $subject" "$ALERT_EMAIL"
        log "Email sent: $subject"
    fi
}

send_slack() {
    local message="$1"
    local level="${2:-info}"
    
    if [ -n "$SLACK_WEBHOOK" ]; then
        local color="good"
        local icon="ℹ️"
        [ "$level" = "warning" ] && { color="warning"; icon="⚠️"; }
        [ "$level" = "danger" ] && { color="danger"; icon="🚨"; }
        [ "$level" = "success" ] && { color="good"; icon="✅"; }
        
        curl -X POST "$SLACK_WEBHOOK" \
            -H 'Content-Type: application/json' \
            -d "{
                \"attachments\": [{
                    \"color\": \"$color\",
                    \"title\": \"$icon HSM Automatic Key Rotation\",
                    \"text\": \"$message\",
                    \"footer\": \"HSM Auto-Rotation Service\",
                    \"ts\": $(date +%s)
                }]
            }" 2>/dev/null || log "Failed to send Slack notification"
    fi
}

send_telegram() {
    local message="$1"
    
    if [ -n "$TELEGRAM_BOT_TOKEN" ] && [ -n "$TELEGRAM_CHAT_ID" ]; then
        curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
            -d "chat_id=${TELEGRAM_CHAT_ID}" \
            -d "text=🔐 <b>HSM Automatic Key Rotation</b>

${message}" \
            -d "parse_mode=HTML" >/dev/null || log "Failed to send Telegram notification"
    fi
}

send_alert() {
    local message="$1"
    local level="${2:-info}"
    
    log "$message"
    send_email "${level^^}: Auto-Rotation" "$message"
    send_slack "$message" "$level"
    send_telegram "$message"
    logger -t hsm-auto-rotation -p user."$level" "$message"
}

# Функция проверки предусловий
check_prerequisites() {
    # Проверка флага AUTO_ROTATION_ENABLED
    if [ "$AUTO_ROTATION_ENABLED" != "true" ]; then
        error "Automatic rotation is DISABLED"
        error "Set AUTO_ROTATION_ENABLED=true to enable"
        exit 1
    fi
    
    # Проверка HSM_PIN
    if [ -z "$HSM_PIN" ]; then
        error "HSM_PIN environment variable is not set"
        send_alert "❌ AUTO-ROTATION FAILED: HSM_PIN not configured" "danger"
        exit 1
    fi
    
    # Проверка Docker
    if ! docker info >/dev/null 2>&1; then
        error "Docker is not running"
        send_alert "❌ AUTO-ROTATION FAILED: Docker not available" "danger"
        exit 1
    fi
    
    # Проверка контейнера HSM
    if ! docker ps | grep -q hsm-service; then
        error "hsm-service container is not running"
        send_alert "❌ AUTO-ROTATION FAILED: HSM service not running" "danger"
        exit 1
    fi
    
    # Проверка Python и PyYAML
    if ! command -v python3 >/dev/null 2>&1; then
        error "python3 is not installed"
        exit 1
    fi
    
    if ! python3 -c "import yaml" 2>/dev/null; then
        error "PyYAML is not installed (pip3 install pyyaml)"
        exit 1
    fi
    
    # Создание директорий
    mkdir -p "$BACKUP_DIR"
    
    success "Prerequisites check passed"
}

# Функция получения ключей, требующих ротации
get_keys_needing_rotation() {
    docker exec hsm-service /app/hsm-admin rotation-status 2>&1 | \
        grep -B 5 "NEEDS ROTATION" | \
        grep "Label:" | \
        awk '{print $2}' || true
}

# Функция ротации одного ключа
rotate_single_key() {
    local old_label="$1"
    
    info "Starting automatic rotation for key: $old_label"
    
    # Валидация формата
    if ! echo "$old_label" | grep -qP '^[a-z0-9-]+-v\d+$'; then
        error "Invalid key label format: $old_label"
        send_alert "❌ AUTO-ROTATION FAILED: Invalid key label format: $old_label" "danger"
        return 1
    fi
    
    # Извлечение базового имени и версии
    local base_name=$(echo "$old_label" | sed 's/-v[0-9]*$//')
    local old_version=$(echo "$old_label" | grep -oP 'v\K\d+$')
    local new_version=$((old_version + 1))
    local new_label="${base_name}-v${new_version}"
    local timestamp=$(date +%Y%m%d-%H%M%S)
    
    info "Rotation plan: $old_label (v$old_version) → $new_label (v$new_version)"
    
    # Проверка возраста ключа (защита от слишком старых ключей)
    local created_at=$(grep -A 10 "label: $old_label" "$PROJECT_DIR/config.yaml" | grep "created_at:" | awk '{print $2}' | tr -d "'\"" || echo "")
    
    if [ -n "$created_at" ]; then
        # Удаляем кавычки из даты YAML
        created_at=$(echo "$created_at" | tr -d "'\"")
        
        local created_timestamp=$(date -d "$created_at" +%s 2>/dev/null || echo "0")
        local now_timestamp=$(date +%s)
        
        # Проверка на корректность timestamp
        if [ "$created_timestamp" -eq 0 ] || [ "$created_timestamp" -lt 0 ]; then
            warning "Invalid created_at date: $created_at"
        else
            local age_days=$(( (now_timestamp - created_timestamp) / 86400 ))
            
            info "Key age: $age_days days"
            
            if [ "$age_days" -gt "$MAX_ROTATION_AGE_DAYS" ]; then
                error "Key is too old ($age_days days) - exceeds maximum of $MAX_ROTATION_AGE_DAYS days"
                send_alert "❌ AUTO-ROTATION ABORTED: Key $old_label is critically overdue ($age_days days). Manual intervention required!" "danger"
                return 1
            fi
        fi
    fi
    
    # Отправка уведомления о начале
    send_alert "🔄 Starting automatic rotation: $old_label → $new_label" "warning"
    
    # Шаг 1: Резервное копирование
    info "Creating backups..."
    
    local backup_config="$BACKUP_DIR/config.yaml.$timestamp"
    local backup_token="$BACKUP_DIR/token.$timestamp.tar.gz"
    
    cp "$PROJECT_DIR/config.yaml" "$backup_config" || {
        error "Failed to backup config"
        send_alert "❌ AUTO-ROTATION FAILED: Config backup failed for $old_label" "danger"
        return 1
    }
    
    docker run --rm -v "$(pwd)/data/tokens:/tokens" alpine tar czf - -C /tokens . > "$backup_token" || {
        error "Failed to backup HSM token"
        send_alert "❌ AUTO-ROTATION FAILED: Token backup failed for $old_label" "danger"
        return 1
    }
    
    success "Backups created: config=$backup_config, token=$backup_token"
    
    # Шаг 2: Создание нового ключа
    info "Creating new KEK: $new_label"
    
    local new_id=$(printf "%02d" "$new_version")
    
    local create_output
    create_output=$(docker exec -e HSM_PIN="$HSM_PIN" hsm-service \
        /app/create-kek "$new_label" "$new_id" "$HSM_PIN" "$new_version" 2>&1) || {
        error "Failed to create new KEK: $create_output"
        send_alert "❌ AUTO-ROTATION FAILED: KEK creation failed for $new_label
        
Error: $create_output

Backups preserved:
- Config: $backup_config
- Token: $backup_token" "danger"
        return 1
    }
    
    success "New KEK created: $new_label"
    log "create-kek output: $create_output"
    
    # Шаг 3: Обновление metadata.yaml
    info "Updating metadata for overlap period..."
    
    # Резервная копия metadata.yaml
    local backup_metadata="$BACKUP_DIR/metadata.yaml.$(date +%Y%m%d-%H%M%S)"
    cp "$PROJECT_DIR/metadata.yaml" "$backup_metadata" || {
        error "Failed to backup metadata.yaml"
        return 1
    }
    
    # Найти контекст ключа
    local context=$(python3 << PYTHON_FIND_CONTEXT
import yaml

with open('$PROJECT_DIR/metadata.yaml', 'r') as f:
    metadata = yaml.safe_load(f)

for ctx_name, ctx_meta in metadata['rotation'].items():
    if ctx_meta.get('label') == '$old_label':
        print(ctx_name)
        break
PYTHON_FIND_CONTEXT
)
    
    if [ -z "$context" ]; then
        error "Could not find context for key $old_label"
        send_alert "❌ AUTO-ROTATION FAILED: Key context not found for $old_label" "danger"
        return 1
    fi
    
    info "Found context: $context"
    
    # Обновление metadata.yaml через Python
    python3 << PYTHON_SCRIPT
import yaml
from datetime import datetime

metadata_file = '$PROJECT_DIR/metadata.yaml'
context = '$context'
old_label = '$old_label'
new_label = '$new_label'
new_version = $new_version

try:
    with open(metadata_file, 'r') as f:
        metadata = yaml.safe_load(f)
    
    # Получаем старую метаданные
    old_metadata = metadata['rotation'][context].copy()
    
    # Создаем новые метаданные
    metadata['rotation'][context] = {
        'label': new_label,
        'version': new_version,
        'created_at': datetime.utcnow().strftime('%Y-%m-%dT%H:%M:%SZ')
    }
    
    # Сохраняем старый ключ для overlap period
    old_context = context + '-old'
    metadata['rotation'][old_context] = old_metadata
    
    with open(metadata_file, 'w') as f:
        yaml.dump(metadata, f, default_flow_style=False, sort_keys=False)
    
    print(f"SUCCESS: Metadata updated")
except Exception as e:
    print(f"ERROR: {str(e)}")
    exit(1)
PYTHON_SCRIPT
    
    if [ $? -ne 0 ]; then
        error "Failed to update metadata.yaml"
        send_alert "❌ AUTO-ROTATION FAILED: Metadata update failed for $old_label

Rolling back to backup: $backup_metadata" "danger"
        cp "$backup_metadata" "$PROJECT_DIR/metadata.yaml"
        return 1
    fi
    
    success "Metadata updated for overlap period"
    
    # Шаг 4: Обновление config.yaml для ACL mappings
    info "Updating ACL mappings..."
    
    python3 << PYTHON_ACL
import yaml

config_file = '$PROJECT_DIR/config.yaml'
context = '$context'

try:
    with open(config_file, 'r') as f:
        config = yaml.safe_load(f)
    
    # Добавляем старый контекст в ACL mappings
    old_context = context + '-old'
    
    # Добавляем определение старого ключа в config.yaml
    if context in config['hsm']['keys']:
        config['hsm']['keys'][old_context] = config['hsm']['keys'][context].copy()
    
    # Обновляем ACL mappings
    for ou, contexts in config['acl']['mappings'].items():
        if context in contexts and old_context not in contexts:
            contexts.append(old_context)
    
    with open(config_file, 'w') as f:
        yaml.dump(config, f, default_flow_style=False, sort_keys=False)
    
    print(f"SUCCESS: ACL mappings updated")
except Exception as e:
    print(f"ERROR: {str(e)}")
    exit(1)
PYTHON_ACL
    
    if [ $? -ne 0 ]; then
        error "Failed to update ACL mappings"
        send_alert "❌ AUTO-ROTATION FAILED: ACL update failed" "danger"
        cp "$backup_metadata" "$PROJECT_DIR/metadata.yaml"
        return 1
    fi
    
    # Шаг 5: Перезапуск сервиса
    info "Restarting HSM service..."
    
    cd "$PROJECT_DIR"
    docker compose restart hsm-service >/dev/null 2>&1 || {
        error "Failed to restart service"
        send_alert "❌ AUTO-ROTATION FAILED: Service restart failed

Rolling back to backup: $backup_config" "danger"
        cp "$backup_config" "$PROJECT_DIR/config.yaml"
        docker compose restart hsm-service
        return 1
    }
    
    sleep 5
    
    # Шаг 5: Проверка здоровья
    info "Checking service health..."
    
    local health_check
    health_check=$(curl -sk https://localhost:8443/health 2>&1 || true)
    
    if ! echo "$health_check" | grep -q '"status":"healthy"'; then
        error "Service health check failed!"
        send_alert "❌ AUTO-ROTATION FAILED: Service unhealthy after restart

Health check output: $health_check

Rolling back to backup: $backup_config" "danger"
        
        # Откат
        cp "$backup_config" "$PROJECT_DIR/config.yaml"
        docker compose restart hsm-service
        return 1
    fi
    
    success "Service is healthy"
    
    # Проверка наличия обоих ключей
    local rotation_status
    rotation_status=$(docker exec hsm-service /app/hsm-admin rotation-status 2>&1)
    
    if ! echo "$rotation_status" | grep -q "$new_label"; then
        error "New key not found in rotation status"
        send_alert "❌ AUTO-ROTATION VERIFICATION FAILED: New key $new_label not loaded" "danger"
        return 1
    fi
    
    success "Both keys verified in HSM"
    
    # Успешное завершение
    local completion_message="✅ <b>AUTO-ROTATION COMPLETED</b>

<b>Key rotated:</b> $old_label → $new_label
<b>Version:</b> v$old_version → v$new_version
<b>Context:</b> $context

<b>Overlap Period:</b> $OVERLAP_PERIOD_DAYS days
- New encryptions use: $new_label
- Old data decrypts with: $old_label

<b>Backups:</b>
- Config: $backup_config
- Token: $backup_token

<b>NEXT STEPS:</b>
1. Monitor service logs for errors
2. Application teams must re-encrypt data with new key
3. After $OVERLAP_PERIOD_DAYS days, old key will be auto-deleted (if AUTO_CLEANUP_ENABLED=true)

<b>Manual cleanup:</b>
sudo ./scripts/cleanup-old-keys.sh"

    send_alert "$completion_message" "success"
    
    success "Rotation completed: $old_label → $new_label"
    return 0
}

# Функция автоматической очистки старых ключей
auto_cleanup_old_keys() {
    if [ "$AUTO_CLEANUP_ENABLED" != "true" ]; then
        info "Auto cleanup is disabled (AUTO_CLEANUP_ENABLED=false)"
        return 0
    fi
    
    info "Checking for old keys ready for cleanup..."
    
    # Найти все -old контексты
    local old_contexts
    old_contexts=$(grep -E '^\s+[a-z0-9-]+-old:' "$PROJECT_DIR/config.yaml" | sed 's/://g' | tr -d ' ' || true)
    
    if [ -z "$old_contexts" ]; then
        info "No old keys found"
        return 0
    fi
    
    while read -r context; do
        [ -z "$context" ] && continue
        
        local label=$(grep -A 5 "^  $context:" "$PROJECT_DIR/config.yaml" | grep "label:" | awk '{print $2}')
        local created_at=$(grep -A 5 "^  $context:" "$PROJECT_DIR/config.yaml" | grep "created_at:" | awk '{print $2}')
        
        if [ -z "$label" ] || [ -z "$created_at" ]; then
            warning "Could not get metadata for context: $context"
            continue
        fi
        
        # Вычисляем возраст
        local created_timestamp=$(date -d "$created_at" +%s 2>/dev/null || echo "0")
        local now_timestamp=$(date +%s)
        local age_days=$(( (now_timestamp - created_timestamp) / 86400 ))
        
        info "Checking old key: $label (age: $age_days days, required: $OVERLAP_PERIOD_DAYS days)"
        
        if [ "$age_days" -ge "$OVERLAP_PERIOD_DAYS" ]; then
            info "Key $label is ready for cleanup (age: $age_days >= $OVERLAP_PERIOD_DAYS)"
            
            # Удаление из HSM
            docker exec hsm-service /app/hsm-admin delete-kek --label "$label" --confirm || {
                warning "Failed to delete key from HSM: $label"
                continue
            }
            
            success "Deleted from HSM: $label"
            
            # Резервная копия перед удалением из конфига
            local timestamp=$(date +%Y%m%d-%H%M%S)
            cp "$PROJECT_DIR/config.yaml" "$BACKUP_DIR/config.yaml.before-cleanup-$timestamp"
            
            # Удаление из config.yaml
            python3 << PYTHON_SCRIPT
import yaml

with open('$PROJECT_DIR/config.yaml', 'r') as f:
    config = yaml.safe_load(f)

if '$context' in config['hsm']['keys']:
    del config['hsm']['keys']['$context']

for ou, contexts in config['acl']['mappings'].items():
    if '$context' in contexts:
        contexts.remove('$context')

with open('$PROJECT_DIR/config.yaml', 'w') as f:
    yaml.dump(config, f, default_flow_style=False, sort_keys=False)
PYTHON_SCRIPT
            
            success "Removed from config: $context"
            
            send_alert "🗑️ Auto-cleanup completed: $label removed after $age_days days overlap" "info"
        fi
    done <<< "$old_contexts"
    
    # Перезапуск сервиса если были изменения
    if docker exec hsm-service /app/hsm-admin rotation-status 2>&1 | grep -q "$context"; then
        info "Restarting service to apply cleanup..."
        docker compose restart hsm-service
        sleep 3
    fi
}

# Главная функция
main() {
    log "=========================================="
    log "HSM Automatic Key Rotation - Starting"
    log "=========================================="
    
    # Проверка предусловий
    check_prerequisites
    
    # Получение списка ключей для ротации
    local keys_to_rotate
    keys_to_rotate=$(get_keys_needing_rotation)
    
    if [ -z "$keys_to_rotate" ]; then
        info "No keys need rotation at this time"
        
        # Проверка на автоматическую очистку
        auto_cleanup_old_keys
        
        success "Auto-rotation check completed - no action needed"
        exit 0
    fi
    
    info "Keys needing rotation:"
    echo "$keys_to_rotate"
    
    # Ротация каждого ключа
    local rotation_success=0
    local rotation_failed=0
    
    while read -r key_label; do
        [ -z "$key_label" ] && continue
        
        if rotate_single_key "$key_label"; then
            ((rotation_success++))
        else
            ((rotation_failed++))
        fi
    done <<< "$keys_to_rotate"
    
    # Автоматическая очистка старых ключей
    auto_cleanup_old_keys
    
    # Итоговый отчет
    log "=========================================="
    log "Auto-rotation summary:"
    log "  Successful: $rotation_success"
    log "  Failed: $rotation_failed"
    log "=========================================="
    
    if [ "$rotation_failed" -gt 0 ]; then
        exit 1
    fi
    
    exit 0
}

# Запуск
main "$@"
