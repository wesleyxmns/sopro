PREFIX ?= /usr/local
BINDIR ?= $(PREFIX)/bin
DESTDIR ?=

APP_NAME := sopro
PACKAGE := github.com/wesleyxmns/sopro/cmd/sopro
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
COMMIT ?= $(shell git rev-parse --short HEAD 2>/dev/null || echo "none")
BUILD_DATE ?= $(shell date -u +'%Y-%m-%dT%H:%M:%SZ')

GO ?= go
GOFLAGS ?=
LDFLAGS := -s -w \
	-X 'github.com/wesleyxmns/sopro/internal/version.Version=$(VERSION)' \
	-X 'github.com/wesleyxmns/sopro/internal/version.Commit=$(COMMIT)' \
	-X 'github.com/wesleyxmns/sopro/internal/version.BuildDate=$(BUILD_DATE)' \
	-X 'github.com/wesleyxmns/sopro/internal/version.BuiltBy=make'

.PHONY: all build test clean install uninstall

all: build

build:
	@echo "==> Compilando $(APP_NAME) $(VERSION)..."
	@mkdir -p bin
	CGO_ENABLED=0 $(GO) build $(GOFLAGS) -ldflags "$(LDFLAGS)" -o bin/$(APP_NAME) ./cmd/$(APP_NAME)

test:
	@echo "==> Executando testes..."
	$(GO) test -v ./...

install: build
	@echo "==> Instalando em $(DESTDIR)$(BINDIR)/$(APP_NAME)..."
	install -d $(DESTDIR)$(BINDIR)
	install -m 755 bin/$(APP_NAME) $(DESTDIR)$(BINDIR)/$(APP_NAME)
	@echo "==> Sopro instalado com sucesso em $(DESTDIR)$(BINDIR)/$(APP_NAME)"

uninstall:
	@echo "==> Removendo $(DESTDIR)$(BINDIR)/$(APP_NAME)..."
	rm -f $(DESTDIR)$(BINDIR)/$(APP_NAME)
	@echo "==> Sopro desinstalado com sucesso."

clean:
	@echo "==> Limpando artefatos..."
	rm -rf bin/
