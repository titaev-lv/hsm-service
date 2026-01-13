# 🔐 PKI Setup - Настройка Certificate Authority и сертификатов

> **Цель**: Подготовить PKI инфраструктуру для HSM Service (CA, серверные и клиентские сертификаты)

## 📋 Что такое PKI и зачем это нужно?

**PKI (Public Key Infrastructure)** - инфраструктура открытых ключей для безопасной аутентификации через сертификаты.

HSM Service использует **mutual TLS (mTLS)** - двустороннюю аутентификацию:
- **Сервер** проверяет клиентский сертификат
- **Клиент** проверяет серверный сертификат
- **CA** (Certificate Authority) - доверенный центр сертификации, который подписывает все сертификаты

### Что вам нужно подготовить:

```
pki/
├── ca/
│   ├── ca.crt         # Сертификат CA (публичный)
│   └── ca.key         # Приватный ключ CA (СЕКРЕТНЫЙ!)
├── server/
│   ├── hsm-service.local.crt    # Серверный сертификат
│   └── hsm-service.local.key    # Серверный ключ
└── client/
    ├── trading-service-1.crt    # Клиентский сертификат (OU=Trading)
    ├── trading-service-1.key    # Клиентский ключ
    ├── 2fa-service-1.crt        # Клиентский сертификат (OU=2FA)
    └── 2fa-service-1.key        # Клиентский ключ
```

---

## ✅ Предварительные требования

- **OpenSSL** установлен (`openssl version`)
- **Базовое понимание** PKI концепций (опционально)
- **Права на запись** в директорию `pki/`

---

## 🏗️ Шаг 1: Создание Certificate Authority (CA)

### Проверка: У вас УЖЕ есть CA?

```bash
ls -la pki/ca/ca.key pki/ca/ca.crt
```

✅ **Если файлы существуют** → переходите к [Шагу 2](#-шаг-2-генерация-серверного-сертификата)

❌ **Если файлов нет** → создайте CA ниже

---

### 1.1. Создание приватного ключа CA

```bash
# Создать приватный ключ CA (4096 бит, защищенный паролем)
openssl genrsa -aes256 -out ca.key 4096
```

**Важно**:
- ⚠️ Введите **сильный пароль** (минимум 12 символов)
- ⚠️ **Запомните пароль** - он потребуется для подписи сертификатов
- 🔒 Храните `ca.key` в защищенном месте (KMS, Vault, offline storage)

---

### 1.2. Создание самоподписанного сертификата CA

```bash
# Создать самоподписанный сертификат CA (валиден 10 лет)
openssl req -new -x509 -days 3650 -key ca.key -out ca.crt \
  -subj "/C=RU/ST=Moscow/L=Moscow/O=YourCompany/OU=Security/CN=HSM-CA"
```

**Параметры**:
- `C=RU` - Страна (можете изменить)
- `ST=Moscow` - Регион
- `L=Moscow` - Город
- `O=YourCompany` - Название организации (**измените на свое**)
- `OU=Security` - Отдел (Security, IT, etc.)
- `CN=HSM-CA` - Common Name для CA

**Проверка**:
```bash
openssl x509 -in ca.crt -noout -text | head -20
```

---

### 1.3. Размещение CA файлов

```bash
# Скопировать в проект
mkdir -p pki/ca
mv ca.key pki/ca/
mv ca.crt pki/ca/

# Установить правильные права доступа
chmod 600 pki/ca/ca.key    # Только владелец может читать
chmod 644 pki/ca/ca.crt    # Все могут читать
```

---

### 🔒 Безопасность CA ключа

**КРИТИЧЕСКИ ВАЖНО**:
- ✅ Храните `ca.key` в защищенном месте (KMS, Vault, offline storage)
- ✅ Используйте **сильный пароль** для защиты ca.key
- ✅ **Не коммитьте** ca.key в Git (уже в `.gitignore`)
- ✅ Регулярно делайте **backup** ca.key и ca.crt
- ✅ Рассмотрите использование **Hardware HSM** для production CA
- ⚠️ Если ca.key скомпрометирован → **вся PKI становится небезопасной**

**Backup CA**:
```bash
# Backup на внешний носитель
cp pki/ca/ca.key /path/to/secure/backup/ca.key.$(date +%Y%m%d)
cp pki/ca/ca.crt /path/to/secure/backup/ca.crt.$(date +%Y%m%d)
```

---

## 🖥️ Шаг 2: Генерация серверного сертификата

Серверный сертификат используется для TLS сервера HSM Service.

### 2.1. Для HSM Service

```bash
./pki/scripts/issue-server-cert.sh hsm-service.local
```

**Что происходит**:
1. Создается приватный ключ сервера (`pki/server/hsm-service.local.key`)
2. Генерируется CSR (Certificate Signing Request)
3. CA подписывает сертификат → `pki/server/hsm-service.local.crt`

**С дополнительными DNS/IP** (для production):
```bash
./pki/scripts/issue-server-cert.sh \
  hsm-service.local \
  "hsm-service.local,hsm.example.com,localhost" \
  "10.0.0.5,127.0.0.1"
```

---

### 2.2. Проверка серверного сертификата

```bash
# Просмотр сертификата
openssl x509 -in pki/server/hsm-service.local.crt -noout -text

# Проверка subject
openssl x509 -in pki/server/hsm-service.local.crt -noout -subject
# subject=CN=hsm-service.local,OU=Services,O=Private,L=Moscow,ST=Moscow,C=RU

# Проверка дат действия
openssl x509 -in pki/server/hsm-service.local.crt -noout -dates
# notBefore=Jan 10 12:00:00 2026 GMT
# notAfter=Jan 10 12:00:00 2027 GMT

# Проверка SAN (Subject Alternative Names)
openssl x509 -in pki/server/hsm-service.local.crt -noout -ext subjectAltName
```

---

## 👤 Шаг 3: Генерация клиентских сертификатов

Клиентские сертификаты используются для mTLS аутентификации. **Organizational Unit (OU)** определяет права доступа через ACL.

### 3.1. Organizational Units (OU) и доступ

| OU       | Доступные contexts | Применение                          |
|----------|-------------------|-------------------------------------|
| Trading  | exchange-key      | Trading сервисы (шифрование DEKs)   |
| 2FA      | 2fa               | 2FA сервисы (шифрование secrets)    |
| Billing  | billing           | Billing сервисы                     |
| Admin    | * (все)           | Административный доступ             |

**ACL настраивается в** `config.yaml`:
```yaml
acl:
  mappings:
    Trading: [exchange-key]
    2FA: [2fa]
    Billing: [billing]
```

---

### 3.1.1. Режимы AAD и требования к сертификатам (ВАЖНО!)

HSM Service поддерживает два режима AAD (Additional Authenticated Data):

#### **Shared Mode** (mode: shared)
- **AAD**: SHA256(context + OU)
- **Sharing**: Все клиенты с одинаковым OU могут расшифровывать данные друг друга
- **Use case**: Envelope encryption, когда несколько сервисов шифруют DEKs для общей базы данных

**Пример конфигурации**:
```yaml
hsm:
  keys:
    exchange-key:
      type: aes
      mode: shared    # AAD использует OU, не CN
```

**Требуемые сертификаты** (для envelope encryption с Trading сервисами):
```bash
# Все Trading сервисы должны иметь одинаковый OU=Trading
./pki/scripts/issue-client-cert.sh trading-service-1 Trading
./pki/scripts/issue-client-cert.sh trading-service-2 Trading
./pki/scripts/issue-client-cert.sh billing-worker-1 Trading

# Результат: все 3 клиента могут расшифровывать DEKs друг друга
# AAD = SHA256("exchange-key" + NULL + "Trading") - одинаковый для всех
```

#### **Private Mode** (mode: private, по умолчанию)
- **AAD**: SHA256(context + CN)
- **Isolation**: Каждый клиент изолирован, не может расшифровать чужие данные
- **Use case**: 2FA secrets, приватные ключи, индивидуальные credentials

**Пример конфигурации**:
```yaml
hsm:
  keys:
    2fa:
      type: aes
      mode: private   # AAD использует CN (default)
```

**Требуемые сертификаты** (для изолированных 2FA сервисов):
```bash
# Каждый 2FA сервис имеет свой уникальный CN
./pki/scripts/issue-client-cert.sh 2fa-service-1 2FA
./pki/scripts/issue-client-cert.sh 2fa-service-2 2FA

# Результат: каждый сервис видит только СВОИ данные
# AAD для service-1 = SHA256("2fa" + NULL + "2fa-service-1")
# AAD для service-2 = SHA256("2fa" + NULL + "2fa-service-2")
```

#### **Таблица режимов и use cases**

| Context      | Mode    | OU      | Use Case                           | Сертификаты                              |
|--------------|---------|---------|-----------------------------------|------------------------------------------|
| exchange-key | shared  | Trading | Envelope encryption, DEK sharing   | trading-service-1, trading-service-2, ... |
| 2fa          | private | 2FA     | 2FA secrets (изолированные)        | 2fa-service-1, 2fa-service-2, ...        |
| user-keys    | private | Users   | User private keys                  | user-service-1, user-service-2, ...      |
| billing      | shared  | Billing | Shared billing encryption keys     | billing-api-1, billing-worker-1, ...     |

**ВАЖНО**: 
- ✅ Для **shared mode** создавайте несколько сертификатов с **одинаковым OU**
- ✅ Для **private mode** каждый сертификат должен иметь **уникальный CN**
- ⚠️ Нельзя смешивать режимы - если context имеет mode=shared, все клиенты с нужным OU получат доступ

---

### 3.1.2. Примеры генерации сертификатов для типовых сценариев

#### Сценарий 1: Trading Platform (Envelope Encryption)

**Задача**: 3 Trading сервиса шифруют DEKs для общей БД

```bash
# Все с OU=Trading для shared mode
./pki/scripts/issue-client-cert.sh trading-api-1 Trading
./pki/scripts/issue-client-cert.sh trading-api-2 Trading
./pki/scripts/issue-client-cert.sh trading-worker-1 Trading
```

**Config**:
```yaml
hsm:
  keys:
    exchange-key:
      mode: shared    # Все Trading клиенты могут decrypt друг друга
acl:
  mappings:
    Trading: [exchange-key]
```

#### Сценарий 2: Multi-tenant 2FA Service (Isolation)

**Задача**: Каждый 2FA сервис изолирован, видит только свои secrets

```bash
# Каждый с уникальным CN, но OU=2FA для ACL
./pki/scripts/issue-client-cert.sh 2fa-tenant-a 2FA
./pki/scripts/issue-client-cert.sh 2fa-tenant-b 2FA
./pki/scripts/issue-client-cert.sh 2fa-tenant-c 2FA
```

**Config**:
```yaml
hsm:
  keys:
    2fa:
      mode: private   # Каждый tenant изолирован (default)
acl:
  mappings:
    2FA: [2fa]
```

#### Сценарий 3: Mixed (Trading + 2FA)

**Задача**: Trading сервисы могут share DEKs, но 2FA изолированы

```bash
# Trading сервисы (shared)
./pki/scripts/issue-client-cert.sh trading-service-1 Trading
./pki/scripts/issue-client-cert.sh trading-service-2 Trading

# 2FA сервисы (private)
./pki/scripts/issue-client-cert.sh 2fa-service-1 2FA
./pki/scripts/issue-client-cert.sh 2fa-service-2 2FA
```

**Config**:
```yaml
hsm:
  keys:
    exchange-key:
      mode: shared    # Trading share
    2fa:
      mode: private   # 2FA isolated
acl:
  mappings:
    Trading: [exchange-key]
    2FA: [2fa]
```

---

### 3.2. Генерация клиентского сертификата

**Синтаксис**:
```bash
./pki/scripts/issue-client-cert.sh <client-name> <OU>
```

**Примеры**:

```bash
# Trading сервис
./pki/scripts/issue-client-cert.sh trading-service-1 Trading

# 2FA сервис
./pki/scripts/issue-client-cert.sh 2fa-service-1 2FA

# Billing сервис
./pki/scripts/issue-client-cert.sh billing-service-1 Billing

# Для тестирования
./pki/scripts/issue-client-cert.sh test-client Trading
```

**Что создается**:
- `pki/client/<client-name>.key` - Приватный ключ клиента
- `pki/client/<client-name>.crt` - Сертификат клиента с OU

---

### 3.3. Проверка клиентского сертификата

```bash
# Проверка OU (КРИТИЧЕСКИ ВАЖНО для ACL!)
openssl x509 -in pki/client/trading-service-1.crt -noout -subject
# subject=CN=trading-service-1,OU=Trading,O=Private,L=Moscow,ST=Moscow,C=RU
#                           ^^^^^^^^^ ← Должно быть правильное OU!

# Проверка дат
openssl x509 -in pki/client/trading-service-1.crt -noout -dates

# Проверка Extended Key Usage (должно быть clientAuth)
openssl x509 -in pki/client/trading-service-1.crt -noout -ext extendedKeyUsage
# X509v3 Extended Key Usage:
#     TLS Web Client Authentication
```

---

## 🔄 Шаг 4: Проверка всей PKI

После создания всех сертификатов, проверьте структуру:

```bash
# Проверить структуру директорий
tree pki/

# Ожидаемая структура:
# pki/
# ├── ca/
# │   ├── ca.crt
# │   └── ca.key
# ├── server/
# │   ├── hsm-service.local.crt
# │   └── hsm-service.local.key
# └── client/
#     ├── trading-service-1.crt
#     ├── trading-service-1.key
#     ├── 2fa-service-1.crt
#     └── 2fa-service-1.key
```

**Проверка прав доступа**:
```bash
ls -la pki/ca/
# -rw------- ca.key  ← Только владелец
# -rw-r--r-- ca.crt  ← Все могут читать

ls -la pki/server/
# -rw------- hsm-service.local.key
# -rw-r--r-- hsm-service.local.crt

ls -la pki/client/
# -rw------- trading-service-1.key
# -rw-r--r-- trading-service-1.crt
```

---

## ✅ Шаг 5: Тестирование PKI

### 5.1. Проверка цепочки сертификатов

```bash
# Проверить серверный сертификат против CA
openssl verify -CAfile pki/ca/ca.crt pki/server/hsm-service.local.crt
# pki/server/hsm-service.local.crt: OK

# Проверить клиентский сертификат против CA
openssl verify -CAfile pki/ca/ca.crt pki/client/trading-service-1.crt
# pki/client/trading-service-1.crt: OK
```

✅ Если `OK` - всё правильно!

❌ Если ошибка:
```
error 20 at 0 depth lookup: unable to get local issuer certificate
```
→ Проблема с CA сертификатом или цепочкой

---

### 5.2. Тест TLS handshake (опционально)

Если HSM Service уже запущен:

```bash
# Тест серверного сертификата
openssl s_client -connect localhost:8443 \
  -CAfile pki/ca/ca.crt \
  -cert pki/client/trading-service-1.crt \
  -key pki/client/trading-service-1.key

# Проверьте вывод:
# Verify return code: 0 (ok)  ← Должно быть 0
```

---

## 🔧 Troubleshooting

### ❌ Проблема: "error 18 at 0 depth lookup: self signed certificate"

**Причина**: CA сертификат не доверенный

**Решение**:
```bash
# Убедитесь что используете правильный CA
openssl verify -CAfile pki/ca/ca.crt pki/ca/ca.crt
# pki/ca/ca.crt: OK
```

---

### ❌ Проблема: "Permission denied" при создании сертификатов

**Причина**: Нет прав на запись в `pki/`

**Решение**:
```bash
# Проверить права
ls -ld pki/

# Дать права (если нужно)
chmod 755 pki/
chmod 755 pki/ca pki/server pki/client
```

---

### ❌ Проблема: "unable to load CA private key" при issue-*-cert.sh

**Причина**: ca.key защищен паролем или недоступен

**Решение**:
```bash
# Проверить что ca.key существует и доступен
ls -la pki/ca/ca.key

# Проверить что можно прочитать ключ
openssl rsa -in pki/ca/ca.key -check
# Enter pass phrase for pki/ca/ca.key: [введите пароль]
```

---

### ❌ Проблема: "OU not authorized for context" в HSM Service

**Причина**: OU в клиентском сертификате не соответствует ACL

**Решение**:
```bash
# Проверить OU в сертификате
openssl x509 -in pki/client/trading-service-1.crt -noout -subject
# subject=CN=trading-service-1,OU=Trading,O=...

# Проверить ACL в config.yaml
cat config.yaml | grep -A5 "acl:"
# acl:
#   mappings:
#     Trading: [exchange-key]  ← OU=Trading может только exchange-key
```

**Пересоздать сертификат с правильным OU**:
```bash
rm pki/client/trading-service-1.*
./pki/scripts/issue-client-cert.sh trading-service-1 Trading
```

---

## 🔄 Ротация сертификатов

### Когда нужна ротация?

- ✅ Сертификат истекает (проверьте через `openssl x509 -noout -dates`)
- ✅ Компрометация приватного ключа
- ✅ Изменение имени сервиса/клиента
- ✅ Добавление новых SAN (Subject Alternative Names)

### Ротация серверного сертификата

```bash
# 1. Создать новый сертификат
./pki/scripts/issue-server-cert.sh hsm-service.local.new

# 2. Остановить HSM Service
docker-compose down  # или systemctl stop hsm-service

# 3. Заменить сертификаты
mv pki/server/hsm-service.local.crt pki/server/hsm-service.local.crt.old
mv pki/server/hsm-service.local.key pki/server/hsm-service.local.key.old
mv pki/server/hsm-service.local.new.crt pki/server/hsm-service.local.crt
mv pki/server/hsm-service.local.new.key pki/server/hsm-service.local.key

# 4. Запустить HSM Service
docker-compose up -d  # или systemctl start hsm-service

# 5. Проверить
curl --cacert pki/ca/ca.crt \
     --cert pki/client/trading-service-1.crt \
     --key pki/client/trading-service-1.key \
     https://localhost:8443/health
```

### Ротация клиентского сертификата

```bash
# 1. Создать новый сертификат
./pki/scripts/issue-client-cert.sh trading-service-1-new Trading

# 2. Обновить клиентское приложение (использовать новый .crt/.key)

# 3. Удалить старый (опционально)
rm pki/client/trading-service-1.crt pki/client/trading-service-1.key

# 4. Переименовать новый
mv pki/client/trading-service-1-new.crt pki/client/trading-service-1.crt
mv pki/client/trading-service-1-new.key pki/client/trading-service-1.key
```

---

## 🚫 Отзыв сертификатов (Revocation)

Если клиентский сертификат скомпрометирован:

```bash
# Отозвать сертификат
./pki/scripts/revoke-cert.sh trading-service-1

# Проверить revoked.yaml
cat pki/revoked.yaml
# revoked_certs:
#   - cn: trading-service-1
#     revoked_at: '2026-01-10T15:30:00Z'
#     reason: key-compromise
```

**HSM Service автоматически перезагружает `revoked.yaml` каждые 30 секунд**.

Подробнее: [REVOCATION_RELOAD.md](REVOCATION_RELOAD.md)

---

## 📚 Что дальше?

После настройки PKI:

### Для Development (локальная разработка):
- 🐳 **Docker**: [QUICKSTART_DOCKER.md](QUICKSTART_DOCKER.md)
- 🔧 **Native Go binary**: [QUICKSTART_NATIVE.md](QUICKSTART_NATIVE.md)

### Для Production:
- 🏭 **Debian 13 + nftables**: [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md)

### Продвинутое управление PKI:
- 📖 **Детальная документация**: [pki/README.md](pki/README.md)
- 🔄 **Ротация ключей HSM**: [KEY_ROTATION.md](KEY_ROTATION.md)
- 🛠️ **CLI утилиты**: [CLI_TOOLS.md](CLI_TOOLS.md)

---

## 🔗 Полезные ссылки

- [OpenSSL Documentation](https://www.openssl.org/docs/)
- [X.509 Certificate Specification](https://tools.ietf.org/html/rfc5280)
- [TLS 1.3 RFC](https://tools.ietf.org/html/rfc8446)
- [mTLS Best Practices](https://www.cloudflare.com/learning/access-management/what-is-mutual-tls/)

---

**Готово!** Ваша PKI инфраструктура настроена и готова к использованию 🎉
