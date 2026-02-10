# 📚 Документация HSM Service - Порядок изучения

## 🎯 С чего начать?

### Новичок в проекте? Читай по порядку:

1. **[README.md](README.md)** (5 минут) 
   - Что это такое?
   - Зачем это нужно?
   - Основные возможности

2. **[PKI_SETUP.md](PKI_SETUP.md)** (15 минут)
   - Настройка Certificate Authority (CA)
   - Генерация серверных и клиентских сертификатов
   - Проверка PKI инфраструктуры

3. **[QUICKSTART_DOCKER.md](QUICKSTART_DOCKER.md)** (5 минут) 
   - Быстрый запуск в Docker
   - Первый тестовый запрос
   - Проверка что все работает

4. **[ARCHITECTURE.md](ARCHITECTURE.md)** (20 минут)
   - Как все устроено внутри
   - Какие компоненты есть
   - Как данные идут через систему

---

## 🔧 Для разработчиков

### Backend разработчик
```
1. README.md → Что это
2. PKI_SETUP.md → Настройка PKI
3. QUICKSTART_DOCKER.md → Быстрый старт
4. ARCHITECTURE.md → Как устроено
5. API.md → Как использовать API
```

### DevOps инженер
```
1. README.md → Что это
2. PKI_SETUP.md → Настройка PKI
3. QUICKSTART_DOCKER.md → Запуск локально
4. PRODUCTION_DEBIAN.md → Установка на Debian 13
5. MONITORING.md → Мониторинг и алерты
```

### Security engineer
```
1. README.md → Что это
2. PKI_SETUP.md → Настройка PKI и безопасность CA
3. SECURITY_AUDIT.md → Аудит безопасности
4. KEY_ROTATION.md → Ротация ключей
```

---

## 📖 Полный список документации

### 🟢 Основные (обязательно читать)

| Документ | Описание | Для кого | Время |
|----------|----------|----------|-------|
| [README.md](README.md) | Обзор проекта, возможности | Все | 5 мин |
| [PKI_SETUP.md](PKI_SETUP.md) | Настройка PKI (CA, сертификаты) | Все | 15 мин |
| [QUICKSTART_DOCKER.md](QUICKSTART_DOCKER.md) | Быстрый старт (Docker) | Все | 5 мин |
| [BUILD.md](BUILD.md) | Сборка production бинарников | Backend/DevOps | 15 мин |
| [ARCHITECTURE.md](ARCHITECTURE.md) | Архитектура системы | Backend/DevOps | 20 мин |
| [API.md](API.md) | API Reference | Backend | 15 мин |

### 🟡 Развертывание и эксплуатация

| Документ | Описание | Для кого | Время |
|----------|----------|----------|-------|
| [BUILD.md](BUILD.md) | Сборка оптимизированных бинарников для production | DevOps/Backend | 15 мин |
| [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) | Установка на Debian 13 + nftables | DevOps | 30 мин |
| [MONITORING.md](MONITORING.md) | Prometheus метрики, алерты, audit/error логи | DevOps | 15 мин |
| [BACKUP_RESTORE.md](BACKUP_RESTORE.md) | Бэкап и восстановление | DevOps | 10 мин |

### 🔴 Безопасность

| Документ | Описание | Для кого | Время |
|----------|----------|----------|-------|
| [SECURITY_AUDIT.md](SECURITY_AUDIT.md) | OWASP Top 10 + PCI DSS аудит | Security | 40 мин |
| [KEY_ROTATION.md](KEY_ROTATION.md) | Ротация KEK | Security/DevOps | 15 мин |

### 🔵 Дополнительные (по необходимости)

| Документ | Описание | Для кого | Время |
|----------|----------|----------|-------|
| [PKI_SETUP.md](PKI_SETUP.md) | Полное руководство по PKI (CA, сертификаты, ротация, отзыв) | DevOps/Security | 25 мин |
| [CLI_TOOLS.md](CLI_TOOLS.md) | hsm-admin команды | DevOps | 10 мин |
| [TROUBLESHOOTING.md](TROUBLESHOOTING.md) | Решение проблем | DevOps/Backend | 15 мин |
| [tests/README.md](tests/README.md) | Руководство по тестированию | QA/DevOps | 20 мин |

---

## 🚀 Сценарии использования

### "Я хочу запустить сервис локально"
```
1. PKI_SETUP.md → Настройка PKI
2. QUICKSTART_DOCKER.md → Запуск Docker
3. API.md → Тестовые запросы
```

### "Я хочу задеплоить в продакшен"
```
1. PKI_SETUP.md → Подготовка сертификатов
2. PRODUCTION_DEBIAN.md → Установка на сервер
3. MONITORING.md → Настройка метрик
4. BACKUP_RESTORE.md → Настройка бэкапов
5. SECURITY_AUDIT.md → Проверка безопасности
```

### "Мне нужно понять как работает ротация ключей"
```
1. README.md → Общий контекст
2. KEY_ROTATION.md → Детали ротации
3. CLI_TOOLS.md → Команды для ротации
```

### "У меня сломалось, что делать?"
```
1. TROUBLESHOOTING.md → Частые проблемы
2. MONITORING.md → Проверка метрик
3. GitHub Issues → Создать issue если не помогло
```

---

## 📝 Статус документации

| Документ | Статус | Актуальность | Язык |
|----------|--------|--------------|------|
| README.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| PKI_SETUP.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| QUICKSTART_DOCKER.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| ARCHITECTURE.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| API.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| BUILD.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| PRODUCTION_DEBIAN.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| MONITORING.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| BACKUP_RESTORE.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| SECURITY_AUDIT.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| KEY_ROTATION.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| CLI_TOOLS.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| TROUBLESHOOTING.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |
| tests/README.md | ✅ Готов | ✅ Актуален | 🇷🇺 Русский |

---

## 🔄 Устаревшие/архивные документы

Эти документы сохранены для истории или содержат устаревшую информацию:

- **REVOCATION_RELOAD.md** - Информация перенесена в README.md (автоматическая перезагрузка revoked.yaml)

---

## 📊 Статистика документации

- **Всего документов**: 18 markdown файлов (корень) + tests/README.md + tests/EXTREME_TEST_RESULTS.md
- **Общий объем**: ~18,000 строк
- **Язык**: Русский (упрощенный)
- **Покрытие**: 100% критических областей
- **Последнее обновление**: 2026-01-13

---

## ❓ FAQ по документации

**Q: С чего начать если я совсем новичок?**  
A: README.md → PKI_SETUP.md → QUICKSTART_DOCKER.md → готово, можете запускать

**Q: Где найти примеры API запросов?**  
A: API.md → раздел "Examples"

**Q: Как настроить firewall на production сервере?**  
A: PRODUCTION_DEBIAN.md → раздел "Настройка nftables"

**Q: Какие метрики смотреть в Prometheus?**  
A: MONITORING.md → раздел "Критические метрики"

**Q: Как сделать бэкап ключей из HSM?**  
A: BACKUP_RESTORE.md → раздел "Бэкап KEK"

**Q: Сертификат отозван но сервис его пропускает, почему?**  
A: README.md → раздел "Certificate Revocation" (автоматическая перезагрузка revoked.yaml каждые 30 секунд)

---

## 🛠️ Инструменты для чтения документации

**Локально:**
```bash
# Markdown viewer
npm install -g markdown-it
markdown-it README.md > README.html

# Или используйте VS Code с Markdown Preview
code README.md  # Потом Ctrl+Shift+V
```

**Online:**
- GitHub автоматически рендерит .md файлы
- GitBook (если настроен CI/CD)
- Sphinx (для генерации HTML документации)

---

## 📧 Обратная связь

Нашли ошибку в документации? Что-то непонятно?

**Напишите**: titaev@gmail.com

**Помните**: хорошая документация = счастливая команда! 🎉
