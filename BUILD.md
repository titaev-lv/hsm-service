# 🔨 Build Instructions - Сборка бинарников для Production

> **Цель**: Собрать оптимизированные бинарники HSM Service для копирования на production сервер

## 📋 Содержание

- [Быстрая сборка](#быстрая-сборка)
- [Требования](#требования)
- [Сборка всех компонентов](#сборка-всех-компонентов)
- [Оптимизация бинарников](#оптимизация-бинарников)
- [Cross-compilation](#cross-compilation)
- [Проверка собранных файлов](#проверка-собранных-файлов)
- [Подготовка к deployment](#подготовка-к-deployment)

---

## 🚀 Быстрая сборка

```bash
# Собрать все бинарники одной командой
make build

# Результат:
# - build/hsm-service      (основной сервис)
# - build/hsm-admin        (CLI утилита)
# - build/create-kek       (создание KEK ключей)
```

**Или вручную**:

```bash
# Создать директорию для бинарников
mkdir -p build

# Собрать hsm-service
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w -X main.Version=1.0.0" \
  -o build/hsm-service \
  main.go

# Собрать hsm-admin
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -o build/hsm-admin \
  ./cmd/hsm-admin

# Собрать create-kek
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -o build/create-kek \
  ./cmd/create-kek
```

---

## ✅ Требования

### Обязательные

```bash
# Go 1.22 или новее
go version
# go version go1.22.0 linux/amd64

# Git (для версионирования)
git --version
```

### Опциональные (для оптимизации)

```bash
# UPX - сжатие бинарников (опционально)
sudo apt install upx-ucl  # Debian/Ubuntu
sudo pacman -S upx        # Arch Linux
brew install upx          # macOS

# strip - удаление debug символов (обычно уже есть)
which strip
```

---

## 🔧 Сборка всех компонентов

### 1. hsm-service (основной сервис)

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w -X main.Version=$(git describe --tags --always)" \
  -trimpath \
  -o build/hsm-service \
  main.go
```

**Флаги объяснение**:
- `CGO_ENABLED=0` - статическая компиляция (no libc dependency)
- `GOOS=linux` - целевая ОС
- `GOARCH=amd64` - целевая архитектура
- `-ldflags="-s -w"` - удаление debug информации и symbol table
- `-X main.Version=...` - встраивание версии в бинарник
- `-trimpath` - удаление абсолютных путей (security)

**Размер**: ~10-15 MB (без UPX), ~5-8 MB (с UPX)

---

### 2. hsm-admin (CLI утилита)

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -trimpath \
  -o build/hsm-admin \
  ./cmd/hsm-admin
```

**Использование**:
- Управление KEK ключами
- Ротация ключей
- Cleanup старых версий
- Проверка статуса ротации

**Размер**: ~8-12 MB

---

### 3. create-kek (создание KEK)

```bash
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -trimpath \
  -o build/create-kek \
  ./cmd/create-kek
```

**Использование**:
- Создание KEK ключей в HSM
- Используется в init-hsm.sh

**Размер**: ~7-10 MB

---

## ⚡ Оптимизация бинарников

### Шаг 1: Build с оптимизациями

```bash
# Максимальные оптимизации компилятора
CGO_ENABLED=0 go build \
  -ldflags="-s -w" \
  -gcflags="all=-l -B" \
  -trimpath \
  -o build/hsm-service \
  main.go
```

**Флаги gcflags**:
- `-l` - отключить inlining (экономия места)
- `-B` - отключить bounds checking (небольшое ускорение)

⚠️ **Внимание**: `-gcflags="all=-l -B"` может немного снизить производительность. Для production рекомендуется стандартная сборка без `-gcflags`.

---

### Шаг 2: Strip debug symbols (если не использовали -ldflags="-s -w")

```bash
strip build/hsm-service
strip build/hsm-admin
strip build/create-kek
```

**Результат**: Уменьшение размера на ~30%

---

### Шаг 3: UPX compression (опционально)

```bash
# Установить UPX
sudo apt install upx-ucl

# Сжать бинарники
upx --best --lzma build/hsm-service
upx --best --lzma build/hsm-admin
upx --best --lzma build/create-kek
```

**Результат**: Уменьшение размера на ~50-70%

**Плюсы**:
- ✅ Меньше размер на диске
- ✅ Быстрее копирование по сети
- ✅ Меньше памяти при загрузке (распаковка в RAM)

**Минусы**:
- ❌ Немного медленнее старт (распаковка)
- ❌ Может быть ложноположительный в antivirus
- ❌ Сложнее debugging

**Рекомендация**: Использовать UPX только если критичен размер файлов (slow network, limited storage).

---

## 🌍 Cross-compilation

### Сборка для разных платформ

```bash
# Linux AMD64 (стандарт)
GOOS=linux GOARCH=amd64 go build -o build/hsm-service-linux-amd64 main.go

# Linux ARM64 (для ARM серверов)
GOOS=linux GOARCH=arm64 go build -o build/hsm-service-linux-arm64 main.go

# macOS AMD64 (для локальной разработки на Intel Mac)
GOOS=darwin GOARCH=amd64 go build -o build/hsm-service-darwin-amd64 main.go

# macOS ARM64 (для Apple Silicon)
GOOS=darwin GOARCH=arm64 go build -o build/hsm-service-darwin-arm64 main.go
```

### Проверка поддерживаемых платформ

```bash
go tool dist list | grep linux
# linux/amd64
# linux/arm64
# linux/386
# linux/arm
```

---

## ✔️ Проверка собранных файлов

### 1. Проверка типа файла

```bash
file build/hsm-service
# build/hsm-service: ELF 64-bit LSB executable, x86-64, statically linked, stripped
```

**Ожидаемо**:
- ✅ `ELF 64-bit LSB executable`
- ✅ `statically linked` (no external dependencies)
- ✅ `stripped` (no debug symbols)

---

### 2. Проверка зависимостей

```bash
ldd build/hsm-service
# not a dynamic executable
```

**Ожидаемо**: `not a dynamic executable` - static binary ✅

**Если видите зависимости**:
```
linux-vdso.so.1 => (0x00007ffc...)
libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6
```
→ Значит `CGO_ENABLED=1`, пересоберите с `CGO_ENABLED=0`

---

### 3. Проверка размера

```bash
ls -lh build/
# -rwxr-xr-x 1 user user  12M Jan 14 10:00 hsm-service
# -rwxr-xr-x 1 user user  10M Jan 14 10:01 hsm-admin
# -rwxr-xr-x 1 user user 8.5M Jan 14 10:02 create-kek
```

**Типичные размеры**:
- hsm-service: 10-15 MB (без UPX), 5-8 MB (с UPX)
- hsm-admin: 8-12 MB
- create-kek: 7-10 MB

---

### 4. Проверка версии

```bash
./build/hsm-service --version
# HSM Service version 1.0.0 (commit abc123)
```

---

### 5. Тестовый запуск

```bash
# Проверка что бинарник запускается
./build/hsm-service --help
./build/hsm-admin --help
./build/create-kek --help
```

---

## 📦 Подготовка к deployment

### Создание release пакета

```bash
#!/bin/bash
# build-release.sh

VERSION=$(git describe --tags --always)
RELEASE_NAME="hsm-service-${VERSION}-linux-amd64"
RELEASE_DIR="release/${RELEASE_NAME}"

# Создать директорию
mkdir -p "${RELEASE_DIR}/bin"
mkdir -p "${RELEASE_DIR}/config"
mkdir -p "${RELEASE_DIR}/scripts"

# Собрать бинарники
echo "Building binaries..."
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w -X main.Version=${VERSION}" \
  -trimpath \
  -o "${RELEASE_DIR}/bin/hsm-service" \
  main.go

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -trimpath \
  -o "${RELEASE_DIR}/bin/hsm-admin" \
  ./cmd/hsm-admin

CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -trimpath \
  -o "${RELEASE_DIR}/bin/create-kek" \
  ./cmd/create-kek

# Скопировать конфигурацию
cp config.yaml "${RELEASE_DIR}/config/config.yaml.example"
cp metadata.yaml.example "${RELEASE_DIR}/config/"
cp softhsm2.conf "${RELEASE_DIR}/config/"

# Скопировать скрипты
cp scripts/init-hsm.sh "${RELEASE_DIR}/scripts/"
cp scripts/auto-rotate-keys.sh "${RELEASE_DIR}/scripts/"
cp scripts/cleanup-old-keys.sh "${RELEASE_DIR}/scripts/"
chmod +x "${RELEASE_DIR}/scripts/"*.sh

# Скопировать документацию
cp README.md "${RELEASE_DIR}/"
cp PRODUCTION_DEBIAN.md "${RELEASE_DIR}/"
cp LICENSE "${RELEASE_DIR}/"

# Создать checksums
cd release
sha256sum "${RELEASE_NAME}/bin/"* > "${RELEASE_NAME}/CHECKSUMS.txt"

# Создать tar.gz
tar -czf "${RELEASE_NAME}.tar.gz" "${RELEASE_NAME}/"

echo "✓ Release created: release/${RELEASE_NAME}.tar.gz"
ls -lh "${RELEASE_NAME}.tar.gz"
```

**Запуск**:
```bash
chmod +x build-release.sh
./build-release.sh
```

**Результат**:
```
release/
└── hsm-service-1.0.0-linux-amd64.tar.gz  (~15-20 MB)
    ├── bin/
    │   ├── hsm-service
    │   ├── hsm-admin
    │   └── create-kek
    ├── config/
    │   ├── config.yaml.example
    │   ├── metadata.yaml.example
    │   └── softhsm2.conf
    ├── scripts/
    │   ├── init-hsm.sh
    │   ├── auto-rotate-keys.sh
    │   └── cleanup-old-keys.sh
    ├── CHECKSUMS.txt
    ├── README.md
    ├── PRODUCTION_DEBIAN.md
    └── LICENSE
```

---

### Копирование на production сервер

```bash
# Скопировать release пакет
scp release/hsm-service-1.0.0-linux-amd64.tar.gz prod-server:/tmp/

# На prod сервере
ssh prod-server
cd /tmp
tar -xzf hsm-service-1.0.0-linux-amd64.tar.gz
cd hsm-service-1.0.0-linux-amd64

# Проверить checksums
sha256sum -c CHECKSUMS.txt

# Установить
sudo mkdir -p /opt/hsm-service
sudo cp -r bin/ config/ scripts/ /opt/hsm-service/
sudo chmod +x /opt/hsm-service/bin/*
sudo chmod +x /opt/hsm-service/scripts/*
```

**Далее**: См. [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) для настройки production окружения.

---

## 🔍 Проверка перед деплоем

### Checklist

- [ ] Собраны все 3 бинарника (hsm-service, hsm-admin, create-kek)
- [ ] `CGO_ENABLED=0` (статическая компиляция)
- [ ] `ldd` показывает `not a dynamic executable`
- [ ] `file` показывает `statically linked, stripped`
- [ ] Версия встроена (`--version` работает)
- [ ] Бинарники запускаются (`--help` работает)
- [ ] Размеры адекватные (~10-15 MB каждый)
- [ ] CHECKSUMS.txt создан
- [ ] Конфигурация включена в пакет
- [ ] Скрипты включены и executable

---

## 📊 Сравнение методов сборки

| Метод | Размер | Startup | Dependencies | Рекомендация |
|-------|--------|---------|--------------|--------------|
| Standard build | 15 MB | Fast | None | ✅ Production |
| + strip | 10 MB | Fast | None | ✅ Production |
| + UPX --best | 5 MB | Medium | None | ⚠️ Только если нужно |
| Dynamic (CGO=1) | 12 MB | Fast | libc, others | ❌ Не рекомендуется |

**Рекомендация для production**: Standard build с `-ldflags="-s -w"` (strip встроен)

---

## 🛠️ Makefile для автоматизации

```makefile
# Makefile

VERSION := $(shell git describe --tags --always --dirty)
BUILD_TIME := $(shell date -u '+%Y-%m-%d_%H:%M:%S')
LDFLAGS := -s -w -X main.Version=$(VERSION) -X main.BuildTime=$(BUILD_TIME)

.PHONY: all build clean test release

all: build

build:
	@echo "Building hsm-service..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="$(LDFLAGS)" \
		-trimpath \
		-o build/hsm-service \
		main.go
	@echo "Building hsm-admin..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w" \
		-trimpath \
		-o build/hsm-admin \
		./cmd/hsm-admin
	@echo "Building create-kek..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w" \
		-trimpath \
		-o build/create-kek \
		./cmd/create-kek
	@echo "✓ Build complete"

clean:
	@rm -rf build/ release/
	@echo "✓ Cleaned"

test:
	@go test -v -race -cover ./...

release: build
	@./build-release.sh

install: build
	@sudo cp build/hsm-service /usr/local/bin/
	@sudo cp build/hsm-admin /usr/local/bin/
	@sudo cp build/create-kek /usr/local/bin/
	@echo "✓ Installed to /usr/local/bin/"
```

**Использование**:
```bash
make build           # Собрать все
make clean           # Очистить
make test            # Запустить тесты
make release         # Создать release пакет
make install         # Установить локально
```

---

## 📚 Что дальше?

После сборки бинарников:

1. **Копирование на prod**: См. раздел "Копирование на production сервер" выше
2. **Настройка окружения**: [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md)
3. **Настройка PKI**: [PKI_SETUP.md](PKI_SETUP.md)
4. **Запуск и тестирование**: [QUICKSTART_NATIVE.md](QUICKSTART_NATIVE.md)

---

## 🔗 Ссылки

- [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) - Production deployment на Debian 13
- [PKI_SETUP.md](PKI_SETUP.md) - Настройка сертификатов
- [QUICKSTART_NATIVE.md](QUICKSTART_NATIVE.md) - Запуск нативного бинарника
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Решение проблем

---

**Готово!** Теперь у вас есть оптимизированные бинарники для production deployment 🚀
