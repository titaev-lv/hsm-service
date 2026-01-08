# HSM Admin Tool

CLI утилита для управления KEK (Key Encryption Keys) в HSM.

## Команды

### 1. list-kek - Список KEK

Показывает все KEK, настроенные в config.yaml:

```bash
export HSM_PIN=1234
./hsm-admin list-kek
```

Вывод:
```
KEKs configured in config.yaml:

1. Config Key: exchange-key
   Label: kek-exchange-v1
   Type: aes

2. Config Key: 2fa
   Label: kek-2fa-v1
   Type: aes

Total: 2 KEK(s)
```

С verbose:
```bash
./hsm-admin list-kek --verbose
```

### 2. create-kek - Создать KEK

**Примечание**: Автоматическое создание KEK через crypto11 API ограничено. Используйте:

**Option 1: create-kek utility (рекомендуется)**
```bash
# Напрямую через низкоуровневый PKCS#11 API
/app/create-kek "kek-trading-v2" "05" "1234"

# Где:
#   kek-trading-v2 - label ключа
#   05 - ID ключа (hex, уникальный)
#   1234 - HSM PIN
```

**Option 2: pkcs11-tool**
```bash
pkcs11-tool --module /usr/lib/softhsm/libsofthsm2.so \
  --login --pin 1234 \
  --keygen --key-type AES:256 \
  --label kek-trading-v2 \
  --id 05
```

**Option 3: Инструкции от hsm-admin**
```bash
./hsm-admin create-kek --label kek-trading-v2 --context trading
# Покажет детальные инструкции для ручного создания
```

После создания добавить в config.yaml:
```yaml
hsm:
  keys:
    trading:
      label: kek-trading-v2
      type: aes
```

### 3. delete-kek - Удалить KEK

**⚠️ ОСТОРОЖНО**: Все данные, зашифрованные этим KEK, станут недоступны!

```bash
export HSM_PIN=1234
./hsm-admin delete-kek --label kek-old-v1 --confirm
```

Вывод:
```
Searching for KEK: kek-old-v1
✓ KEK deleted successfully: kek-old-v1

WARNING: All data encrypted with this KEK is now unrecoverable!

Next steps:
1. Remove KEK from config.yaml
2. Restart HSM service
```

### 4. export-metadata - Экспорт метаданных

Экспортирует конфигурацию всех KEK в JSON:

```bash
./hsm-admin export-metadata --output kek-metadata.json
```

Содержимое `kek-metadata.json`:
```json
{
  "token_label": "hsm-token",
  "pkcs11_lib": "/usr/lib/softhsm/libsofthsm2.so",
  "kek_count": 2,
  "keks": [
    {
      "config_key": "exchange-key",
      "label": "kek-exchange-v1",
      "type": "aes"
    },
    {
      "config_key": "2fa",
      "label": "kek-2fa-v1",
      "type": "aes"
    }
  ]
}
```

Полезно для:
- Аудита KEK
- Документации
- Backup конфигурации
- Миграции между средами

## Переменные окружения

| Переменная | Описание | Обязательна |
|-----------|----------|-------------|
| `HSM_PIN` | PIN для доступа к HSM токену | Да (кроме export-metadata) |
| `CONFIG_PATH` | Путь к config.yaml | Нет (по умолчанию: config.yaml) |

## Примеры использования

### Инициализация новых KEK

1. Создайте KEK в HSM (используя pkcs11-tool или softhsm2-util)
2. Добавьте в config.yaml
3. Проверьте: `./hsm-admin list-kek --verbose`
4. Экспортируйте: `./hsm-admin export-metadata`
5. Перезапустите HSM service

### Ротация KEK

1. Создайте новый KEK: `kek-exchange-v2`
2. Обновите config.yaml:
```yaml
keys:
  exchange-key-new:
    label: kek-exchange-v2
    type: aes
  exchange-key-old:
    label: kek-exchange-v1
    type: aes
```
3. Перешифруйте данные через API
4. Удалите старый KEK:
```bash
./hsm-admin delete-kek --label kek-exchange-v1 --confirm
```

### Проверка доступности KEK

```bash
export HSM_PIN=1234
./hsm-admin list-kek --verbose
```

Если KEK настроен в config.yaml, но не найден в HSM:
```
Status: ⚠️  NOT FOUND in HSM
```

Если KEK доступен:
```
Status: ✓ Available in HSM
```

## Ограничения

- **create-kek**: Требует ручного создания через pkcs11-tool из-за ограничений crypto11 API
- **list-kek**: Показывает только KEK из config.yaml (не сканирует весь токен)
- **delete-kek**: Необратимая операция - нет способа восстановить удаленный ключ
- **HSM PIN**: Всегда передается через ENV, никогда не хранится в config.yaml

## Безопасность

1. **HSM_PIN** никогда не логируется
2. **Ключевой материал** никогда не экспортируется (CKA_EXTRACTABLE=false)
3. **Metadata export** не содержит секретов - только labels и типы
4. **delete-kek** требует --confirm для предотвращения случайного удаления

## Troubleshooting

### "HSM_PIN environment variable not set"
```bash
export HSM_PIN=your-pin
```

### "Failed to configure PKCS#11"
- Проверьте путь к библиотеке в config.yaml
- Убедитесь что SoftHSM установлен: `which softhsm2-util`
- Проверьте права доступа: `ls -la /usr/lib/softhsm/`

### "KEK not found"
- Проверьте label в config.yaml
- Используйте `list-kek --verbose` для диагностики
- Убедитесь что токен инициализирован: `softhsm2-util --show-slots`

### "Failed to delete KEK"
- Убедитесь что используется правильный PIN
- Проверьте что KEK существует: `list-kek --verbose`
- Проверьте права на запись в токен
