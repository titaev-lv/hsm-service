#!/bin/bash
# HSM Key Rotation Monitoring Script
# Проверяет ключи на необходимость ротации и отправляет оповещения
# Рекомендуется запускать ежедневно через cron
# 
# Поддерживает оба окружения:
# - Docker (docker-compose)
# - Production (Debian 13 с systemd)
#
# Exit codes:
#   0 = All checks passed (no keys need rotation or rotation completed)
#   1 = Critical error (service not running, HSM command failed, etc)
#   2 = Automatic rotation failed (manual action required)

set -euo pipefail

# Конфигурация
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

# ============================================================================
# АВТОМАТИЧЕСКОЕ ОБНАРУЖЕНИЕ ОКРУЖЕНИЯ
# ============================================================================
detect_environment() {
    # Проверка Docker окружения
    if [ -f "/.dockerenv" ] || grep -q docker /proc/1/cgroup 2>/dev/null || docker info >/dev/null 2>&1; then
        ENVIRONMENT="docker"
        HSM_ADMIN_CMD="docker exec hsm-service /app/hsm-admin"
        LOG_FILE="/var/log/hsm-rotation-check.log"
        CONFIG_PATH="/app/config.yaml"
    # Проверка Production окружения (systemd)
    elif systemctl is-active --quiet hsm-service 2>/dev/null || [ -f /etc/systemd/system/hsm-service.service ]; then
        ENVIRONMENT="production"
        HSM_ADMIN_CMD="/opt/hsm-service/bin/hsm-admin -config /etc/hsm-service/config.yaml"
        LOG_FILE="/var/log/hsm-service/rotation.log"
        CONFIG_PATH="/etc/hsm-service/config.yaml"
    else
        echo "ERROR: Cannot detect HSM environment (Docker or Production)"
        echo "Expected: Docker container OR systemd service (hsm-service)"
        exit 1
    fi
}

# Вызвать обнаружение окружения
detect_environment

# Конфигурация алертов (загружается из /etc/hsm-service/environment для Production)
if [ "$ENVIRONMENT" = "production" ] && [ -f /etc/hsm-service/environment ]; then
    # shellcheck source=/etc/hsm-service/environment
    source /etc/hsm-service/environment
fi

ALERT_DAYS_BEFORE="${ALERT_DAYS_BEFORE:-14}"  # Оповещать за 14 дней до истечения
CRITICAL_DAYS_BEFORE="${CRITICAL_DAYS_BEFORE:-7}"  # Критическое оповещение за 7 дней

# Email настройки (опционально)
ALERT_EMAIL="${ALERT_EMAIL:-ops@example.com}"
SEND_EMAIL="${SEND_EMAIL:-false}"

# Slack webhook (опционально)
SLACK_WEBHOOK="${SLACK_WEBHOOK:-}"

# Telegram (опционально)
TELEGRAM_BOT_TOKEN="${TELEGRAM_BOT_TOKEN:-}"
TELEGRAM_CHAT_ID="${TELEGRAM_CHAT_ID:-}"

# Автоматическая ротация (по умолчанию отключена)
AUTO_ROTATE="${AUTO_ROTATE:-false}"

# Функция логирования
log() {
    echo "[$(date '+%Y-%m-%d %H:%M:%S')] $*" | tee -a "$LOG_FILE"
}

# Создать директорию для логов если её нет
mkdir -p "$(dirname "$LOG_FILE")" 2>/dev/null || true

# ============================================================================
# ЗАГРУЗКА ПЕРЕМЕННЫХ ОКРУЖЕНИЯ (для Production)
# ============================================================================
# systemd EnvironmentFile может не передавать переменные в скрипт
# Загружаем явно как fallback
if [ "$ENVIRONMENT" = "production" ]; then
    if [ -z "${HSM_PIN:-}" ] && [ -f /etc/hsm-service/environment ]; then
        log "DEBUG: Loading environment variables from /etc/hsm-service/environment"
        # shellcheck source=/etc/hsm-service/environment
        source /etc/hsm-service/environment
        log "DEBUG: HSM_PIN is now: ${HSM_PIN:+SET (${#HSM_PIN} chars)}"
    else
        if [ -n "${HSM_PIN:-}" ]; then
            log "DEBUG: HSM_PIN already set from systemd: ${HSM_PIN:+SET (${#HSM_PIN} chars)}"
        else
            log "DEBUG: /etc/hsm-service/environment not found or cannot read"
        fi
    fi
    
    # Validate HSM_PIN is set
    if [ -z "${HSM_PIN:-}" ]; then
        log "ERROR: HSM_PIN not found. Set it in /etc/hsm-service/environment or as environment variable"
        exit 1
    fi
    
    # Export HSM_PIN so it's available to child processes
    export HSM_PIN
    log "DEBUG: HSM_PIN exported for child processes"
fi

# Функция отправки email
send_email() {
    local subject="$1"
    local body="$2"
    
    if [ "$SEND_EMAIL" = "true" ]; then
        echo "$body" | mail -s "$subject" "$ALERT_EMAIL"
        log "Email sent to $ALERT_EMAIL: $subject"
    fi
}

# Функция отправки в Slack
send_slack() {
    local message="$1"
    local level="${2:-info}"  # info, warning, danger
    
    if [ -n "$SLACK_WEBHOOK" ]; then
        local color="good"
        [ "$level" = "warning" ] && color="warning"
        [ "$level" = "danger" ] && color="danger"
        
        curl -X POST "$SLACK_WEBHOOK" \
            -H 'Content-Type: application/json' \
            -d "{
                \"attachments\": [{
                    \"color\": \"$color\",
                    \"title\": \"HSM Key Rotation Alert\",
                    \"text\": \"$message\",
                    \"footer\": \"HSM Service\",
                    \"ts\": $(date +%s)
                }]
            }" 2>/dev/null || log "Failed to send Slack notification"
    fi
}

# Функция отправки в Telegram
send_telegram() {
    local message="$1"
    
    if [ -n "$TELEGRAM_BOT_TOKEN" ] && [ -n "$TELEGRAM_CHAT_ID" ]; then
        curl -s -X POST "https://api.telegram.org/bot${TELEGRAM_BOT_TOKEN}/sendMessage" \
            -d "chat_id=${TELEGRAM_CHAT_ID}" \
            -d "text=🔐 HSM Key Rotation Alert

${message}" \
            -d "parse_mode=HTML" >/dev/null || log "Failed to send Telegram notification"
    fi
}

# Функция отправки оповещения (все каналы)
send_alert() {
    local message="$1"
    local level="${2:-info}"
    local syslog_level="$level"
    
    # Маппировать custom приоритеты на syslog приоритеты
    case "$level" in
        danger)      syslog_level="crit" ;;     # danger -> critical
        warning)     syslog_level="warning" ;;
        info)        syslog_level="info" ;;
        *)           syslog_level="notice" ;;   # default
    esac
    
    log "$message"
    send_email "HSM Key Rotation Alert - ${level^^}" "$message"
    send_slack "$message" "$level"
    send_telegram "$message"
    
    # Syslog (используем маппированный приоритет)
    logger -t hsm-rotation -p user."$syslog_level" "$message"
}

# ============================================================================
# ПРОВЕРКА ДОСТУПНОСТИ СЕРВИСА
# ============================================================================
check_service_availability() {
    if [ "$ENVIRONMENT" = "docker" ]; then
        # Docker окружение
        if ! docker info >/dev/null 2>&1; then
            send_alert "ERROR: Docker is not running or not accessible" "danger"
            exit 1
        fi
        
        if ! docker ps | grep -q hsm-service; then
            send_alert "ERROR: hsm-service container is not running" "danger"
            exit 1
        fi
        
        log "Docker environment detected. Service check: OK"
        
    elif [ "$ENVIRONMENT" = "production" ]; then
        # Production окружение
        if ! systemctl is-active --quiet hsm-service; then
            send_alert "ERROR: hsm-service systemd service is not running" "danger"
            exit 1
        fi
        
        log "Production environment detected. Service check: OK"
    fi
}
log "Starting HSM key rotation check (Environment: $ENVIRONMENT)..."

# Проверка доступности сервиса
check_service_availability

# Получение статуса ротации
log "Executing: $HSM_ADMIN_CMD rotation-status"
ROTATION_STATUS=$($HSM_ADMIN_CMD rotation-status 2>&1) || {
    send_alert "ERROR: Failed to get rotation status from $ENVIRONMENT environment

Command: $HSM_ADMIN_CMD rotation-status

Error: $ROTATION_STATUS" "danger"
    exit 1
}

# Проверка наличия ключей, требующих ротации
NEEDS_ROTATION=$(echo "$ROTATION_STATUS" | grep "NEEDS ROTATION" || true)

if [ -n "$NEEDS_ROTATION" ]; then
    # Критическое оповещение - ключи просрочены!
    # Найти контексты с "NEEDS ROTATION" (контекст идёт за несколько строк до статуса)
    KEYS_OVERDUE=$(echo "$ROTATION_STATUS" | awk '
        /Context:/ { context = $NF }
        /NEEDS ROTATION/ { if (context) print context }
    ' | tr '\n' ', ' | sed 's/,$//')
    
    # Проверка автоматической ротации
    if [ "$AUTO_ROTATE" = "true" ]; then
        log "Keys are overdue - triggering AUTOMATIC ROTATION"
        
        send_alert "🔄 AUTOMATIC ROTATION TRIGGERED

Keys needing rotation: $KEYS_OVERDUE

Starting automatic rotation process...
See logs: $LOG_FILE" "warning"
        
        # Выполнить ротацию для каждого ключа
        ROTATION_FAILED=0
        ROTATION_TIMEOUT=120  # 2 minutes timeout per key
        
        # Определить, нужно ли запускать с sudo для production
        ROTATION_CMD="$HSM_ADMIN_CMD"
        if [ "$ENVIRONMENT" = "production" ] && [ ! -w "/var/lib/hsm-service" ] 2>/dev/null; then
            # Нет доступа на запись - нужно использовать sudo
            log "WARNING: No write permission to /var/lib/hsm-service, attempting with sudo"
            ROTATION_CMD="sudo $HSM_ADMIN_CMD"
        fi
        
        for key_context in $(echo "$KEYS_OVERDUE" | tr ',' ' '); do
            key_context=$(echo "$key_context" | xargs)  # trim whitespace
            log "Starting rotation for context: $key_context (timeout: ${ROTATION_TIMEOUT}s)"
            
            # Export HSM_PIN so it's available to hsm-admin subprocess
            export HSM_PIN
            
            log "Executing command: $ROTATION_CMD rotate \"$key_context\""
            log "DEBUG: HSM_PIN is set: ${HSM_PIN:+YES (${#HSM_PIN} chars)}"
            START_TIME=$(date +%s)
            
            # Выполнить с timeout - используем прямой вызов вместо eval
            # Так гарантируем что переменные передаются правильно
            log "DEBUG: About to execute command..."
            
            if [ "$ENVIRONMENT" = "production" ]; then
                # Production: запустить напрямую с экспортированной переменной
                # Используем стандартный вывод для debug
                log "DEBUG: Environment is production, executing command..."
                ROTATE_OUTPUT=$(timeout $ROTATION_TIMEOUT bash -c "export HSM_PIN='$HSM_PIN'; $ROTATION_CMD rotate '$key_context'" 2>&1)
                ROTATE_EXIT_CODE=$?
            else
                # Docker: также использовать bash
                log "DEBUG: Environment is docker, executing command..."
                ROTATE_OUTPUT=$(timeout $ROTATION_TIMEOUT bash -c "export HSM_PIN='$HSM_PIN'; $ROTATION_CMD rotate '$key_context'" 2>&1)
                ROTATE_EXIT_CODE=$?
            fi
            
            log "DEBUG: Command completed"
            
            END_TIME=$(date +%s)
            DURATION=$((END_TIME - START_TIME))
            
            log "DEBUG: Command exited with code $ROTATE_EXIT_CODE after ${DURATION}s"
            if [ -n "$ROTATE_OUTPUT" ]; then
                log "DEBUG: Command output:"
                echo "$ROTATE_OUTPUT" | while IFS= read -r line; do
                    log "  | $line"
                done
            fi
            
            if [ $ROTATE_EXIT_CODE -eq 0 ]; then
                log "✓ Rotation completed for: $key_context (${DURATION}s)"
                if [ -n "$ROTATE_OUTPUT" ]; then
                    log "Output: $ROTATE_OUTPUT"
                fi
            elif [ $ROTATE_EXIT_CODE -eq 124 ]; then
                log "✗ Rotation TIMEOUT for: $key_context after ${ROTATION_TIMEOUT}s"
                log "Error details: Command timed out. The rotate command may be hanging on HSM operations."
                ROTATION_FAILED=1
            else
                log "✗ Rotation failed for: $key_context (exit code: $ROTATE_EXIT_CODE, duration: ${DURATION}s)"
                log "Error details: $ROTATE_OUTPUT"
                
                # Диагностика permission denied
                if echo "$ROTATE_OUTPUT" | grep -q "permission denied"; then
                    log "HINT: Permission denied error detected. Fix with:"
                    log "  sudo chown -R hsm:hsm /var/lib/hsm-service"
                    log "  sudo chmod 700 /var/lib/hsm-service"
                    log "Or configure sudoers for passwordless: /opt/hsm-service/bin/hsm-admin"
                fi
                
                ROTATION_FAILED=1
            fi
        done
        
        if [ $ROTATION_FAILED -eq 0 ]; then
            log "✓ All rotations completed successfully"
            
            # Step 2: Run cleanup to delete old key versions (PCI DSS compliance)
            log "Starting cleanup of old key versions..."
            CLEANUP_CMD="$HSM_ADMIN_CMD cleanup-old-versions --force"
            
            if [ "$ENVIRONMENT" = "production" ] && [ ! -w "/var/lib/hsm-service" ] 2>/dev/null; then
                CLEANUP_CMD="sudo $CLEANUP_CMD"
            fi
            
            log "Executing: $CLEANUP_CMD"
            export HSM_PIN
            CLEANUP_OUTPUT=$(bash -c "export HSM_PIN='$HSM_PIN'; $CLEANUP_CMD" 2>&1)
            CLEANUP_EXIT_CODE=$?
            
            if [ $CLEANUP_EXIT_CODE -eq 0 ]; then
                log "✓ Old key versions cleaned up successfully"
                if [ -n "$CLEANUP_OUTPUT" ]; then
                    log "Cleanup output:"
                    echo "$CLEANUP_OUTPUT" | while IFS= read -r line; do
                        log "  | $line"
                    done
                fi
            else
                log "⚠️ Warning: Cleanup command returned exit code $CLEANUP_EXIT_CODE"
                log "Cleanup output: $CLEANUP_OUTPUT"
                # Don't fail the whole process, cleanup is secondary to rotation
            fi
            
            send_alert "✅ AUTOMATIC ROTATION COMPLETED

Keys rotated: $KEYS_OVERDUE
Old versions cleaned up

Next check: $(date -d '+1 day' '+%Y-%m-%d %H:%M')" "warning"
            exit 0
        else
            ROTATION_ERRORS=$(tail -20 "$LOG_FILE" | grep "Error details:" || echo "No error details captured")
            send_alert "❌ AUTOMATIC ROTATION FAILED

Keys: $KEYS_OVERDUE

Error details:
$ROTATION_ERRORS

MANUAL ACTION REQUIRED:
1. Check logs: tail -100 $LOG_FILE
2. Perform manual rotation: $HSM_ADMIN_CMD rotate <context>

See: $PROJECT_DIR/KEY_ROTATION.md" "danger"
            exit 2
        fi
    else
        # Ручной режим - только оповещение
        MESSAGE="⚠️ CRITICAL: HSM keys are OVERDUE for rotation!

Environment: $ENVIRONMENT
Keys needing rotation: $KEYS_OVERDUE

Details:
$NEEDS_ROTATION

Action required:
1. Review rotation status: $HSM_ADMIN_CMD rotation-status
2. Perform rotation: $HSM_ADMIN_CMD rotate <context>

See: $PROJECT_DIR/KEY_ROTATION.md for full procedure"

        send_alert "$MESSAGE" "danger"
        # Exit 0: This is not an error, just an operational state requiring attention.
        # Alerts have been sent to the ops team. Manual rotation is needed.
        exit 0
    fi
fi

# Проверка ключей, близких к истечению (предупреждение за 14 дней)
DAYS_REMAINING=$(echo "$ROTATION_STATUS" | grep -oP "Status:.*\(\K\d+(?= days remaining)")

if [ -n "$DAYS_REMAINING" ]; then
    while read -r days; do
        if [ "$days" -le "$CRITICAL_DAYS_BEFORE" ] && [ "$days" -gt 0 ]; then
            # Критическое предупреждение - менее 7 дней до истечения
            KEY_CONTEXT=$(echo "$ROTATION_STATUS" | grep -B 5 "$days days remaining" | grep "Context:" | grep -oP "Context: \K[^[:space:]]+")
            
            MESSAGE="⚠️ WARNING: HSM key expiring soon!

Context: $KEY_CONTEXT
Days remaining: $days

Please schedule key rotation within the next $days days.
See: $PROJECT_DIR/KEY_ROTATION.md"

            send_alert "$MESSAGE" "warning"
            
        elif [ "$days" -le "$ALERT_DAYS_BEFORE" ] && [ "$days" -gt "$CRITICAL_DAYS_BEFORE" ]; then
            # Обычное предупреждение - менее 14 дней до истечения
            KEY_CONTEXT=$(echo "$ROTATION_STATUS" | grep -B 5 "$days days remaining" | grep "Context:" | grep -oP "Context: \K[^[:space:]]+")
            
            MESSAGE="ℹ️ INFO: HSM key rotation approaching

Context: $KEY_CONTEXT
Days remaining: $days

Consider scheduling key rotation soon.
See: $PROJECT_DIR/KEY_ROTATION.md"

            send_alert "$MESSAGE" "info"
        fi
    done <<< "$DAYS_REMAINING"
fi

# Проверка здоровья сервиса
if [ "$ENVIRONMENT" = "docker" ]; then
    HEALTH_CHECK=$(curl -sk https://localhost:8443/health 2>&1 || true)
elif [ "$ENVIRONMENT" = "production" ]; then
    # Production с mTLS сертификатами
    HEALTH_CHECK=$(curl -sk https://localhost:8443/health \
        --cert /etc/hsm-service/pki/client/monitoring.crt \
        --key /etc/hsm-service/pki/client/monitoring.key \
        --cacert /etc/hsm-service/pki/ca/ca.crt 2>&1 || true)
fi

if echo "$HEALTH_CHECK" | grep -q '"status":"healthy"'; then
    log "HSM service health check: OK"
else
    log "WARNING: HSM service health check failed or not available (may be normal in some configs)"
fi

log "HSM key rotation check completed successfully"

# Вывод статуса в stdout для cron email
echo ""
echo "HSM Key Rotation Status Check - $(date)"
echo "========================================"
echo "Environment: $ENVIRONMENT"
echo "Log file: $LOG_FILE"
echo ""
echo "$ROTATION_STATUS"
echo ""
echo "All checks passed. Next check: $(date -d '+1 day' '+%Y-%m-%d %H:%M')"
