<div align="center">

<img src="assets/logo.svg" alt="Sopro Logo" width="480" />

# Sopro ≋
*Observando a memória com calma e precisão.*

</div>

---

Sopro é uma ferramenta de linha de comando (CLI) e TUI responsiva para **observabilidade de memória e controle de processos** com limites explícitos de segurança e ações contextuais inteligentes (Docker, Navegadores, JVM, Git).

A interface usa Bubble Tea, Bubbles e Lip Gloss; o acesso ao sistema operacional fica isolado em adaptadores Linux e Windows.

---

## 🚀 Instalação

### Opção 1: Instalação Rápida (Linux / macOS)

Execute no terminal para baixar e instalar a versão estável mais recente:

```bash
curl -fsSL https://raw.githubusercontent.com/wesleyxmns/sopro/main/install.sh | sh
```

O instalador verifica a arquitetura do seu sistema, valida o checksum SHA256 do binário e instala em `/usr/local/bin/sopro`.

---

### Opção 2: Via `go install` (Desenvolvedores Go)

Se você possui a toolchain do Go instalada:

```bash
go install github.com/wesleyxmns/sopro/cmd/sopro@latest
```

*(Certifique-se de que `$(go env GOPATH)/bin` está no seu `$PATH`)*

---

### Opção 3: Compilação e Instalação Local (`Makefile`)

Clone o repositório e instale via `make`:

```bash
git clone https://github.com/wesleyxmns/sopro.git
cd sopro

# Instalar globalmente (requer sudo para /usr/local/bin)
sudo make install

# Ou instalar no espaço do usuário (~/.local/bin)
make install PREFIX=$HOME/.local
```

Para desinstalar:
```bash
sudo make uninstall
```

---

## 💻 Uso da CLI

Após a instalação, inicie o Sopro digitando diretamente no terminal:

```bash
# Iniciar a interface interativa (TUI)
sopro

# Iniciar com um tema visual específico (auto, dark, light, mono, cyber)
sopro --theme cyber

# Exibir a versão e metadados de compilação
sopro --version

# Exibir ajuda e todas as opções de linha de comando
sopro --help

# Iniciar em modo daemon (observação em segundo plano sem interface)
sopro --daemon
```

---

## ⌨️ Atalhos de Teclado (TUI)

| Atalho | Ação |
|---|---|
| `↑` / `↓`, `j` / `k` | Navegar pela lista de processos |
| `/` | Pesquisa fuzzy rápida por nome/comando |
| `f`, `tab` | Alternar filtros de categoria (Sistema, Containers, Browser, Dev, JVM, etc.) |
| `s` | Alternar ordenação por Memória ↓, CPU ↓ ou Comando ↑ |
| `g` | Alternar modo de agrupamento (Lista plana, Categorias, Árvore de processos) |
| `p` | Pausar (`SIGSTOP`) ou retomar (`SIGCONT`) processo |
| `x` | Encerrar processo graciosamente (`SIGTERM`) |
| `k` | Forçar encerramento imediato (`SIGKILL`) |
| `c` | Limpar cache do sistema operacional (quando permitido) |
| `d` / `r` / `z` / `s` | Ações de container Docker (stop, restart, pause, start) |
| `b` / `u` | Ações de navegador via CDP (fechar abas em branco, suspender abas) |
| `j` | Forçar Garbage Collection em runtime JVM (`jcmd GC.run`) |
| `w` / `v` | Ações de repositório Git (`git status`, `git fetch`) |
| `enter` / `y` | Confirmar ação no diálogo modal |
| `esc` / `n` | Cancelar ação / limpar busca |
| `q` | Sair do Sopro |

---

## 🛡️ Segurança e Auditoria

- **Anti-Reutilização de PID:** Toda ação valida o PID e o horário de criação do processo antes de enviar sinais, evitando atingir processos que assumiram o PID de um processo encerrado.
- **Lista Crítica de Segurança:** Processos vitais do sistema operacional (ex: `systemd`, `sshd`, `gnome-shell`, `services.exe`) e PIDs do kernel são protegidos contra encerramento acidental.
- **Trilha de Auditoria:** Toda ação executada ou recomendada é registrada em formato JSONL em `~/.config/sopro/actions.jsonl` (ou definido por `--audit-log`).

---

## 🏗️ Arquitetura

```text
cmd/sopro
  ├── internal/platform/{linux,windows}
  ├── internal/app
  ├── internal/provider/{docker,cdp,jvm,git}
  ├── internal/version
  └── internal/tui

internal/tui ─────► internal/app
internal/app ─────► internal/{memory,process,control,provider}
internal/platform ► implementa os contratos de SO consumidos por internal/app
```

---

## 🧪 Verificação e Qualidade

```bash
# Executar todos os testes
make test

# Compilar binário local
make build

# Validação cruzada para Windows
GOOS=windows go build ./...
```
