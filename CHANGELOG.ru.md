# Changelog 

## v1.0.1

### Основное 
- Базовый сервис HSM и интеграция PKCS#11/SoftHSM.
- CLI инструменты: hsm-admin, create-kek.
- Скрипты ротации ключей, hot‑reload KEK.

### Security & Compliance
- Исправления OWASP Top 10 (A02/A03/A04/A05/A08/A09).
- Лимиты размеров запросов, таймауты, rate limiting.
- Улучшение логирования/мониторинга, ротация логов, Prometheus‑метрики.
- Валидация AAD и проверка целостности KEK.

### Инфраструктура и деплой
- Docker/Docker Compose.
- PKI bootstrap и инструкции.
- Скрипты init/rotation/backup/restore, проверки целостности.

### Тестирование и качество
- Unit/integration/e2e/performance/security/compliance тесты.
- Нагрузочные, стресс‑ и extreme‑тесты.
- Оптимизация HTTP/2 и сетевого стека.

### Документация
- Руководства quickstart/production/security audit.
- Инструкции восстановления, мониторинга, firewall/SELinux/AppArmor.
