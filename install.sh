#!/bin/sh
# Sopro Installer — https://github.com/wesleyxmns/sopro
# Instalação rápida via: curl -fsSL https://raw.githubusercontent.com/wesleyxmns/sopro/main/install.sh | sh

set -e

REPO="wesleyxmns/sopro"
BIN_NAME="sopro"
DEFAULT_INSTALL_DIR="/usr/local/bin"
FALLBACK_INSTALL_DIR="$HOME/.local/bin"

# Cores para saída no terminal
RED='\033[0;31m'
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
BOLD='\033[1m'
NC='\033[0m' # No Color

log_info() {
    printf "${BLUE}==>${NC} ${BOLD}%s${NC}\n" "$1"
}

log_success() {
    printf "${GREEN}==>${NC} ${BOLD}%s${NC}\n" "$1"
}

log_warn() {
    printf "${YELLOW}==>${NC} %s\n" "$1"
}

log_error() {
    printf "${RED}==>${NC} ${BOLD}%s${NC}\n" "$1" >&2
}

# 1. Detecção do Sistema Operacional e Arquitetura
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
ARCH="$(uname -m)"

case "$OS" in
    linux)  OS="linux" ;;
    darwin) OS="darwin" ;;
    *)
        log_error "Sistema operacional não suportado: $OS"
        exit 1
        ;;
esac

case "$ARCH" in
    x86_64|amd64)  ARCH="amd64" ;;
    aarch64|arm64) ARCH="arm64" ;;
    *)
        log_error "Arquitetura de processador não suportada: $ARCH"
        exit 1
        ;;
esac

log_info "Detectado: ${OS}/${ARCH}"

# 2. Obter versão mais recente da API do GitHub Releases se não informada
if [ -z "$VERSION" ]; then
    log_info "Buscando versão mais recente do Sopro no GitHub..."
    LATEST_RELEASE_URL="https://api.github.com/repos/${REPO}/releases/latest"
    VERSION=$(curl -sL "$LATEST_RELEASE_URL" 2>/dev/null | grep '"tag_name":' | head -n 1 | sed -E 's/.*"([^"]+)".*/\1/' || true)
    
    if [ -z "$VERSION" ]; then
        log_warn "Não foi possível obter a tag mais recente via API. Tentando versão estável padrão..."
        VERSION="v0.1.0"
    fi
fi

CLEAN_VERSION="${VERSION#v}"
TARBALL="${BIN_NAME}_${CLEAN_VERSION}_${OS}_${ARCH}.tar.gz"
DOWNLOAD_URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"
CHECKSUM_URL="https://github.com/${REPO}/releases/download/${VERSION}/checksums.txt"

# 3. Download para diretório temporário
TMP_DIR="$(mktemp -d 2>/dev/null || mktemp -d -t 'sopro-install')"
cleanup() {
    rm -rf "$TMP_DIR"
}
trap cleanup EXIT INT TERM

log_info "Baixando Sopro ${VERSION} (${TARBALL})..."
if ! curl -f -sL "$DOWNLOAD_URL" -o "${TMP_DIR}/${TARBALL}"; then
    log_error "Falha ao baixar o arquivo: $DOWNLOAD_URL"
    log_error "Verifique se a versão ${VERSION} existe no repositório."
    exit 1
fi

# 4. Verificação de Integridade (Checksum SHA256)
log_info "Verificando integridade SHA256..."
if curl -f -sL "$CHECKSUM_URL" -o "${TMP_DIR}/checksums.txt" 2>/dev/null; then
    cd "$TMP_DIR"
    EXPECTED_SUM="$(grep "${TARBALL}" checksums.txt | awk '{print $1}' || true)"
    if [ -n "$EXPECTED_SUM" ]; then
        if command -v sha256sum >/dev/null 2>&1; then
            ACTUAL_SUM="$(sha256sum "${TARBALL}" | awk '{print $1}')"
        elif command -v shasum >/dev/null 2>&1; then
            ACTUAL_SUM="$(shasum -a 256 "${TARBALL}" | awk '{print $1}')"
        else
            ACTUAL_SUM=""
        fi

        if [ -n "$ACTUAL_SUM" ]; then
            if [ "$EXPECTED_SUM" = "$ACTUAL_SUM" ]; then
                log_success "Checksum SHA256 validado com sucesso!"
            else
                log_error "FALHA NO CHECKSUM! O arquivo baixado não corresponde ao hash oficial."
                exit 1
            fi
        fi
    fi
    cd - >/dev/null
else
    log_warn "Arquivo de checksums não disponível para esta versão. Prosseguindo..."
fi

# 5. Extração
tar -xzf "${TMP_DIR}/${TARBALL}" -C "$TMP_DIR"
if [ ! -f "${TMP_DIR}/${BIN_NAME}" ]; then
    log_error "Binário '${BIN_NAME}' não encontrado dentro do tarball."
    exit 1
fi

# 6. Escolha e criação do diretório de destino
INSTALL_DIR="${INSTALL_DIR:-$DEFAULT_INSTALL_DIR}"

if [ ! -d "$INSTALL_DIR" ]; then
    if [ -w "$(dirname "$INSTALL_DIR")" ]; then
        mkdir -p "$INSTALL_DIR"
    else
        log_info "Criando diretório $INSTALL_DIR com sudo..."
        sudo mkdir -p "$INSTALL_DIR"
    fi
fi

log_info "Instalando ${BIN_NAME} em ${INSTALL_DIR}..."
if [ -w "$INSTALL_DIR" ]; then
    mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
    chmod 755 "${INSTALL_DIR}/${BIN_NAME}"
else
    if command -v sudo >/dev/null 2>&1; then
        sudo mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
        sudo chmod 755 "${INSTALL_DIR}/${BIN_NAME}"
    else
        log_warn "Sem permissão para escrever em ${INSTALL_DIR}. Tentando ${FALLBACK_INSTALL_DIR}..."
        INSTALL_DIR="$FALLBACK_INSTALL_DIR"
        mkdir -p "$INSTALL_DIR"
        mv "${TMP_DIR}/${BIN_NAME}" "${INSTALL_DIR}/${BIN_NAME}"
        chmod 755 "${INSTALL_DIR}/${BIN_NAME}"
    fi
fi

# 7. Verificação de $PATH
log_success "Sopro instalado com sucesso em ${INSTALL_DIR}/${BIN_NAME}!"

case ":$PATH:" in
    *":$INSTALL_DIR:"*) ;;
    *)
        log_warn "Atenção: O diretório ${INSTALL_DIR} não está no seu \$PATH."
        log_warn "Adicione a seguinte linha ao seu ~/.bashrc ou ~/.zshrc:"
        log_warn "  export PATH=\"\$PATH:${INSTALL_DIR}\""
        ;;
esac

printf "\nExecute %b para iniciar o Sopro!\n\n" "${BOLD}sopro${NC}"
