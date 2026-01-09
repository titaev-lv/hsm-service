# Процедура ротации KEK

## Обзор

Этот документ описывает процедуру ротации KEK (Key Encryption Key) для HSM Service. Ротация ключей — критически важная практика безопасности, требуемая стандартом PCI DSS Requirement 3.6.4.

## Политика ротации

- **Интервал по умолчанию**: 90 дней (2160 часов)
- **Период перекрытия**: 7 дней (оба ключа - старый и новый - работают одновременно)
- **Версионирование**: Ключи версионируются (например, kek-exchange-v1 → kek-exchange-v2)

## Автоматические предупреждения о ротации

Сервис автоматически проверяет ключи, требующие ротации, при запуске. Если какие-либо ключи просрочены, в логах появятся предупреждения:

```
⚠️  WARNING: The following keys need rotation:
  - kek-exchange-v1 (created: 2025-10-10, rotation interval: 2160h0m0s, version: 1)
⚠️  Run 'hsm-admin rotation-status' to check all keys
⚠️  Run 'hsm-admin rotate <label>' to rotate keys
```

## Проверка статуса ротации

Чтобы проверить статус ротации всех ключей:

```bash
hsm-admin rotation-status
```

Пример вывода:
```
Key Rotation Status:
====================

✓ Context: exchange-key
  Label:             kek-exchange-v1
  Version:           1
  Created:           2026-01-09 10:30:00
  Rotation Interval: 2160h0m0s
  Next Rotation:     2026-04-09
  Status:            OK (89 days remaining)

⚠️  Context: 2fa
  Label:             kek-2fa-v1
  Version:           1
  Created:           2025-10-10 10:30:00
  Rotation Interval: 2160h0m0s
  Next Rotation:     2026-01-08
  Status:            NEEDS ROTATION (1 days overdue)
```

## Шаги ручной ротации

### Шаг 1: Проверка ключей, требующих ротации

```bash
docker exec hsm-service /app/hsm-admin rotation-status
```

### Шаг 2: Ротация ключа

```bash
docker exec hsm-service /app/hsm-admin rotate kek-exchange-v1
```

Эта команда выполнит:
1. Разбор метки ключа для извлечения базового имени и версии
2. Генерацию нового номера версии (v1 → v2)
3. Создание нового KEK с меткой `kek-exchange-v2`
4. Обновление `config.yaml` с информацией о новом ключе
5. Создание резервной копии старого конфига (`config.yaml.backup-TIMESTAMP`)

Пример вывода:
```
Creating new KEK: kek-exchange-v2
✓ Created KEK: kek-exchange-v2 (handle: 3, ID: 02, version: 2, created: 2026-01-09T14:30:00Z)
Created config backup: config.yaml.backup-20260109-143000
✓ Key rotation completed:
  Old key: kek-exchange-v1 (version 1)
  New key: kek-exchange-v2 (version 2)

⚠️  IMPORTANT:
  1. Restart the HSM service to load the new key
  2. Re-encrypt all data encrypted with the old key
  3. After 7 days overlap period, delete the old key:
     hsm-admin delete-kek --label kek-exchange-v1 --confirm
```

### Шаг 3: Перезапуск сервиса

```bash
docker compose restart hsm-service
```

Проверка загрузки нового ключа:
```bash
curl -sk https://localhost:8443/health | jq
```

Ожидаемый вывод:
```json
{
  "status": "healthy",
  "hsm_available": true,
  "kek_status": {
    "kek-2fa-v1": "available",
    "kek-exchange-v1": "available",
    "kek-exchange-v2": "available"
  }
}
```

### Шаг 4: Обновление клиентских приложений

Обновите клиентские приложения для использования новой версии ключа. В течение 7-дневного периода перекрытия будут работать оба ключа - старый и новый.

Тест шифрования с новым ключом:
```bash
curl -sk -X POST https://localhost:8443/encrypt \
  --cert pki/client/hsm-trading-client-1.crt \
  --key pki/client/hsm-trading-client-1.key \
  --cacert pki/ca/ca.crt \
  -H 'Content-Type: application/json' \
  -d '{"context":"exchange-key","plaintext":"VGVzdA=="}'
```

Ответ должен показывать новый ключ:
```json
{
  "ciphertext": "...",
  "key_id": "kek-exchange-v2"
}
```

### Шаг 5: Повторное шифрование всех данных (зависит от приложения)

Необходимо повторно зашифровать все данные, зашифрованные старым ключом. Это зависит от конкретного приложения и способа хранения данных:

**Пример для базы данных:**
```sql
-- Псевдокод - адаптируйте под ваше приложение
UPDATE encrypted_data
SET 
  ciphertext = encrypt_with_new_key(decrypt_with_old_key(ciphertext)),
  key_version = 2
WHERE key_version = 1;
```

**Пример скрипта:**
```bash
#!/bin/bash
# Повторное шифрование всех данных с v1 на v2

for record in $(get_all_encrypted_records); do
  # Расшифровка старым ключом
  plaintext=$(curl -X POST https://localhost:8443/decrypt \
    --cert client.crt --key client.key --cacert ca.crt \
    -d "{\"context\":\"exchange-key\",\"ciphertext\":\"$record\",\"key_id\":\"kek-exchange-v1\"}")
  
  # Шифрование новым ключом
  new_ciphertext=$(curl -X POST https://localhost:8443/encrypt \
    --cert client.crt --key client.key --cacert ca.crt \
    -d "{\"context\":\"exchange-key\",\"plaintext\":\"$plaintext\"}")
  
  # Обновление БД
  update_record "$record_id" "$new_ciphertext" "kek-exchange-v2"
done
```

### Шаг 6: Период ожидания (7 дней)

В течение этого периода:
- Доступны оба ключа - старый и новый
- Новые шифрования используют новый ключ
- Старые данные все еще можно расшифровать старым ключом
- Мониторинг на предмет возможных проблем

### Шаг 7: Удаление старого ключа

После периода перекрытия и проверки, что все данные перешифрованы:

```bash
docker exec hsm-service /app/hsm-admin delete-kek --label kek-exchange-v1 --confirm
```

Проверка удаления:
```bash
docker exec hsm-service /app/hsm-admin list-kek
```

## Формат конфигурационного файла

Ключи в `config.yaml` теперь включают метаданные ротации:

```yaml
hsm:
  pkcs11_lib: /usr/lib/softhsm/libsofthsm2.so
  slot_id: hsm-token
  keys:
    exchange-key:
      label: kek-exchange-v2
      type: aes
      rotation_interval: 2160h  # 90 дней
      version: 2
      created_at: 2026-01-09T14:30:00Z
    2fa:
      label: kek-2fa-v1
      type: aes
      rotation_interval: 2160h  # 90 дней
      version: 1
      created_at: 2026-01-09T10:30:00Z
```

## Соглашение об именовании ключей

Ключи должны следовать этому шаблону именования:
```
<базовое-имя>-v<версия>

Примеры:
  kek-exchange-v1
  kek-exchange-v2
  kek-2fa-v1
  kek-payment-v1
```

## Экстренная ротация

Если ключ скомпрометирован, выполните немедленную ротацию:

1. **Немедленно ротируйте ключ** (пропустите обычный период ожидания)
2. **Отзовите доступ** к скомпрометированному ключу
3. **Перешифруйте все данные** новым ключом (не ждите периода перекрытия)
4. **Удалите старый ключ** немедленно
5. **Проведите аудит всех логов доступа** на предмет подозрительной активности
6. **Уведомите службу безопасности** и заинтересованных лиц

```bash
# Экстренная ротация
docker exec hsm-service /app/hsm-admin rotate kek-exchange-v1
docker compose restart hsm-service
# Немедленное перешифрование всех данных
# Удаление старого ключа без ожидания
docker exec hsm-service /app/hsm-admin delete-kek --label kek-exchange-v1 --confirm
```

## Автоматизация

Для автоматической ротации (cron job):

```bash
#!/bin/bash
# /etc/cron.daily/hsm-key-rotation-check

# Проверка статуса ротации
NEEDS_ROTATION=$(docker exec hsm-service /app/hsm-admin rotation-status | grep "NEEDS ROTATION" | wc -l)

if [ "$NEEDS_ROTATION" -gt 0 ]; then
  echo "Ключи требуют ротации - отправка оповещения"
  # Отправка оповещения команде эксплуатации
  send_alert "Ключи HSM требуют ротации - проверьте и выполните ротацию вручную"
  
  # Запись в syslog
  logger -t hsm-rotation "WARNING: Ключи требуют ротации"
fi
```

## Соответствие PCI DSS

Эта процедура ротации удовлетворяет требованию PCI DSS Requirement 3.6.4:

- ✅ **3.6.4.a**: Ключи ротируются через определенные интервалы (90 дней)
- ✅ **3.6.4.b**: Ключи ротируются при компрометации (процедура экстренной ротации)
- ✅ **3.6.4.c**: Старые ключи выводятся из эксплуатации/удаляются после периода перекрытия
- ✅ **3.6.4.d**: Новые ключи заменяют старые для всех новых шифрований

## Устранение неполадок

### Проблема: Сервис не запускается после ротации

**Причина**: Файл конфигурации может быть поврежден или новый ключ не создан

**Решение**:
```bash
# Проверка синтаксиса конфига
docker exec hsm-service cat /app/config.yaml

# Список ключей в HSM
docker exec hsm-service pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --token-label hsm-token --list-objects --type secrkey --login --pin 1234

# Восстановление из резервной копии при необходимости
docker exec hsm-service cp /app/config.yaml.backup-TIMESTAMP /app/config.yaml
docker compose restart hsm-service
```

### Проблема: Старые данные не расшифровываются

**Причина**: Старый ключ был удален до того, как все данные были перешифрованы

**Решение**:
1. Проверьте наличие резервной копии токена
2. Восстановите старый ключ из резервной копии
3. Завершите перешифрование
4. Удалите старый ключ только после проверки

### Проблема: Команда ротации завершается с ошибкой

**Причина**: Недостаточно прав или проблема с подключением к HSM

**Решение**:
```bash
# Проверка установки HSM_PIN
docker exec hsm-service env | grep HSM_PIN

# Проверка доступности токена
docker exec hsm-service pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --token-label hsm-token --login --pin 1234 --test

# Проверка прав на config.yaml
docker exec hsm-service ls -la /app/config.yaml
```

## Лучшие практики

1. **Планируйте ротации** на время окон обслуживания
2. **Делайте резервные копии config.yaml** перед ротацией
3. **Делайте резервные копии токена HSM** перед ротацией
4. **Тестируйте ротацию** сначала в staging окружении
5. **Мониторьте сервис** во время и после ротации
6. **Документируйте** все ротации в журнале изменений
7. **Проверяйте** целостность данных после перешифрования
8. **Сохраняйте** логи ротации для аудита

## См. также

- [SECURITY_AUDIT.md](SECURITY_AUDIT.md) - Отчет по аудиту безопасности
- [TECHNICAL_SPEC.md](TECHNICAL_SPEC.md) - Техническая спецификация
- PCI DSS v4.0 Requirement 3.6 - Управление криптографическими ключами
