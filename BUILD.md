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
# 1. Установить системные зависимости
sudo apt-get install libsofthsm2-dev  # Debian/Ubuntu
# или
sudo dnf install softhsm-devel        # RHEL/CentOS
# или  
brew install softhsm                  # macOS

# 2. Загрузить Go зависимости
go mod download
go mod tidy

# 3. Собрать все бинарники одной командой
make build

# Результат:
# - build/hsm-service      (основной сервис)
# - build/hsm-admin        (CLI утилита)

Или вручную:

# Создать директорию для бинарников
mkdir -p build

# Собрать hsm-service
GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w -X main.Version=1.0.0" \
  -trimpath \
  -o build/hsm-service \
  main.go

# Собрать hsm-admin
GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -trimpath \
  -o build/hsm-admin \
  ./cmd/hsm-admin

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

# libsofthsm2-dev (ВАЖНО: требуется для CGO компиляции)
sudo apt-get install libsofthsm2-dev  # Debian/Ubuntu
# или
sudo dnf install softhsm-devel        # RHEL/CentOS
# или
brew install softhsm                  # macOS
```

### Почему нужна libsofthsm2-dev?

Проект использует:
- `miekg/pkcs11` - CGO пакет (требует PKCS#11 заголовки)
- `ThalesGroup/crypto11` - зависит от miekg/pkcs11

**❌ ВАЖНО:** Нельзя собирать с `CGO_ENABLED=0`! Будут ошибки `undefined: pkcs11.ObjectHandle`

**✅ Правильно:** Собирать с включенным CGO (по умолчанию) при наличии libsofthsm2-dev

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
GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w -X main.Version=$(git describe --tags --always)" \
  -trimpath \
  -o build/hsm-service \
  main.go
```

**Флаги объяснение**:
- `GOOS=linux GOARCH=amd64` - целевая ОС и архитектура
- `-ldflags="-s -w"` - удаление debug информации и symbol table
- `-X main.Version=...` - встраивание версии в бинарник
- `-trimpath` - удаление абсолютных путей (security)
- **CGO ВКЛЮЧЕН** (по умолчанию) - требуется для PKCS#11

⚠️ **НЕ используйте `CGO_ENABLED=0`** - приведет к ошибкам compilation

**Размер**: ~10-15 MB (без UPX), ~5-8 MB (с UPX)

---

### 2. hsm-admin (CLI утилита)

```bash
GOOS=linux GOARCH=amd64 go build \
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

## ⚡ Оптимизация бинарников

### Шаг 1: Build с оптимизациями

```bash
# Стандартная оптимизация (рекомендуется)
GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -trimpath \
  -o build/hsm-service \
  main.go
```

**Почему именно так:**
- ✅ CGO включен (требуется для PKCS#11)
- ✅ `-ldflags="-s -w"` - удаляет debug информацию
- ✅ `-trimpath` - убирает абсолютные пути
- ✅ Оптимально для production

---

### Шаг 2: Strip debug symbols (если не использовали -ldflags="-s -w")

```bash
strip build/hsm-service
strip build/hsm-admin
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

⚠️ **Важно**: Для кросс-компиляции нужны соответствующие PKCS#11 заголовки. На Linux AMD64 обычно не составляет проблему.

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
# build/hsm-service: ELF 64-bit LSB executable, x86-64, dynamically linked, stripped
```

**Ожидаемо**:
- ✅ `ELF 64-bit LSB executable`
- ✅ `dynamically linked` (есть внешние зависимости, например, libsofthsm2)
- ✅ `stripped` (no debug symbols)

Проект использует CGO и PKCS#11, поэтому всегда будет динамическая линковка с libsofthsm2 и libc.

---

### 2. Проверка зависимостей

```bash
ldd build/hsm-service
# linux-vdso.so.1 (0x00007bba92814000)
# libc.so.6 => /lib/x86_64-linux-gnu/libc.so.6 (0x00007bba92400000)
# /lib64/ld-linux-x86-64.so.2 (0x00007bba92816000)

```

**Ожидаемо**: Зависит от `libsofthsm2.so` и `libc` ✅

**Это нормально!** Проект использует CGO для PKCS#11, поэтому зависит от libsofthsm2.

**Если видите много других зависимостей:**
- Попробуйте пересобрать на целевом сервере
- Или используйте Docker для гарантированной совместимости

---

### 3. Проверка размера

```bash
ls -lh build/
# -rwxr-xr-x 1 user user  9.6M Jan 14 10:00 hsm-service
# -rwxr-xr-x 1 user user  4.0M Jan 14 10:01 hsm-admin
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
GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w -X main.Version=${VERSION}" \
  -trimpath \
  -o "${RELEASE_DIR}/bin/hsm-service" \
  main.go

GOOS=linux GOARCH=amd64 go build \
  -ldflags="-s -w" \
  -trimpath \
  -o "${RELEASE_DIR}/bin/hsm-admin" \
  ./cmd/hsm-admin

# Скопировать конфигурацию
cp config.yaml "${RELEASE_DIR}/config/config.yaml.example"
cp metadata.yaml.example "${RELEASE_DIR}/config/"
cp softhsm2.conf "${RELEASE_DIR}/config/"

# Скопировать скрипты
cp scripts/init-hsm.sh "${RELEASE_DIR}/scripts/"
cp scripts/check-key-rotation.sh "${RELEASE_DIR}/scripts/"
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
    │   └── hsm-admin
    ├── config/
    │   ├── config.yaml.example
    │   ├── metadata.yaml.example
    │   └── softhsm2.conf
    ├── scripts/
    │   ├── init-hsm.sh
    │   └── check-key-rotation.sh
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

- [ ] Собраны все 2 бинарника (hsm-service, hsm-admin)
- [ ] Размеры адекватные (~10-15 MB каждый)
- [ ] CHECKSUMS.txt создан
- [ ] Конфигурация включена в пакет
- [ ] Скрипты включены и executable

---

## 📊 Сравнение методов сборки

| Метод | Размер | Static | CGO | Рекомендация |
|-------|--------|--------|-----|--------------|
| Standard build | 15-20 MB | ❌ No | ✅ Yes | ✅ **Production** |
| + strip | 10-15 MB | ❌ No | ✅ Yes | ✅ Production |
| + UPX --best | 5-8 MB | ❌ No | ✅ Yes | ⚠️ Если нужно |
| CGO_ENABLED=0 | ❌ Error | - | - | ❌ **Не работает!** |

**⚠️ ВАЖНО**: Этот проект **НЕ МОЖЕТ** быть собран как static binary (`CGO_ENABLED=0`), потому что использует PKCS#11 (CGO).

**Рекомендация для production**: 
- Standard build с `-ldflags="-s -w"` 
- Зависит от libsofthsm2 на целевом сервере (обычно уже установлена)

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
	@GOOS=linux GOARCH=amd64 go build \
		-ldflags="$(LDFLAGS)" \
		-trimpath \
		-o build/hsm-service \
		main.go
	@echo "Building hsm-admin..."
	@GOOS=linux GOARCH=amd64 go build \
		-ldflags="-s -w" \
		-trimpath \
		-o build/hsm-admin \
		./cmd/hsm-admin
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
4. **Запуск и тестирование**: [QUICKSTART_DOCKER.md](QUICKSTART_DOCKER.md)

---

## 🔗 Ссылки

- [PRODUCTION_DEBIAN.md](PRODUCTION_DEBIAN.md) - Production deployment на Debian 13
- [PKI_SETUP.md](PKI_SETUP.md) - Настройка сертификатов
- [QUICKSTART_DOCKER.md](QUICKSTART_DOCKER.md) - Быстрый старт в Docker
- [TROUBLESHOOTING.md](TROUBLESHOOTING.md) - Решение проблем

---

**Готово!** Теперь у вас есть оптимизированные бинарники для production deployment 🚀
