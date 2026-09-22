<div align="center">

<img src="assets/logo.svg" alt="Sopro Logo" width="500" />

</div>

# Sopro — Monitor de memória RAM e gerenciador de processos no terminal

[![CI](https://github.com/wesleyxmns/sopro/actions/workflows/ci.yml/badge.svg)](https://github.com/wesleyxmns/sopro/actions/workflows/ci.yml)

*Terminal RAM monitor and process manager for Linux and Windows: clean page cache, manage Docker containers, browser tabs and JVM processes — a fast TUI written in Go.*

Sopro é uma ferramenta de linha de comando (CLI) e TUI responsiva para **observabilidade de memória e controle de processos** com limites explícitos de segurança e ações contextuais inteligentes (Docker, Navegadores, JVM, Git). Monitore o uso de RAM em tempo real, limpe cache e page cache do Linux, libere memória, gerencie containers Docker, abas do navegador e processos JVM direto do terminal Linux e Windows.

A interface usa Bubble Tea, Bubbles e Lip Gloss; o acesso ao sistema operacional fica isolado em adaptadores Linux e Windows.

---

## 🚀 Instalação

### Opção 1: Instalação Rápida (Linux)

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

# Verificar ou instalar a release mais recente
sopro update --check
sopro update

# Exibir ajuda e todas as opções de linha de comando
sopro --help

# Iniciar em modo daemon (observação em segundo plano sem interface)
sopro --daemon

# Daemon com decisões em JSON (uma por linha, para scraping)
sopro --daemon --daemon-json

# Daemon com alertas webhook (transições de pressão e remédios executados)
sopro --daemon --daemon-webhook-url https://exemplo.com/hook

# Listar os últimos eventos de auditoria
sopro audit --last 20
```

Quando o binário estiver em um diretório protegido, como `/usr/local/bin`, o
comando de atualização solicitará permissão administrativa pelo `sudo`. A senha
é recebida diretamente pelo sistema e não é lida pelo Sopro.

---

## ⚙️ Configuração

Além de flags e variáveis `SOPRO_*`, o Sopro lê o arquivo `~/.config/sopro/sopro.conf` (ou o caminho de `SOPRO_CONFIG`) no formato `CHAVE=valor`, uma por linha. Precedência: **flag > variável de ambiente > arquivo > padrão**.

```bash
# ~/.config/sopro/sopro.conf
SOPRO_THEME=dark
SOPRO_DAEMON_INTERVAL=5s
SOPRO_DAEMON_MEMORY_THRESHOLD=85
```

Para operar o daemon via systemd, veja o exemplo em [`contrib/sopro-daemon.service`](contrib/sopro-daemon.service) (modo observação por padrão; descomente a linha `--daemon-enforce` para ações automáticas).

## ⌨️ Atalhos de Teclado (TUI)

| Atalho | Ação |
|---|---|
| `↑` / `↓` | Navegar pela lista de processos |
| `/` | Pesquisa fuzzy rápida por nome/comando |
| `f`, `tab` | Alternar filtros de categoria (Sistema, Containers, Browser, Dev, JVM, etc.) |
| `s` | Alternar ordenação por Memória ↓, CPU ↓ ou Comando ↑ |
| `g` | Alternar modo de agrupamento (Lista plana, Categorias, Árvore de processos) |
| `p` | Pausar (`SIGSTOP`) ou retomar (`SIGCONT`) processo |
| `x` | Encerrar processo graciosamente (`SIGTERM`) |
| `k` | Forçar encerramento imediato (`SIGKILL`) |
| `c` | Limpar cache do serviço selecionado (JVM, navegador via CDP) |
| `T` | Limpeza total: SO + todos os serviços com cache limpável |
| `d` / `r` / `z` / `s` | Ações de container Docker (stop, restart, pause, start) |
| `b` | Fechar abas em branco do navegador via CDP |
| `u` | Verificar/instalar atualização (reinicia sozinho após instalar) |
| `j` | Forçar Garbage Collection em runtime JVM (`jcmd GC.run`) |
| `w` / `v` | Ações de repositório Git (`git status`, `git fetch`) |
| `enter` / `y` | Confirmar ação no diálogo modal |
| `esc` / `n` | Cancelar ação / limpar busca |
| `q` | Sair do Sopro |

---

## 🧹 Limpeza de cache

Só é considerado limpável o cache cuja limpeza **não altera bruscamente o funcionamento** do serviço:

- **Sistema operacional:** page cache, dentries e inodes no Linux (`sync` + `drop_caches`, requer privilégio); working sets via `EmptyWorkingSet` no Windows.
- **JVM:** coleta de lixo no heap (`jcmd <pid> GC.run`); o resultado informa os bytes residentes liberados quando mensuráveis.
- **Navegador:** fechamento de abas em branco via CDP (exige `--remote-debugging-port`).

Containers Docker e repositórios Git não entram na limpeza: não há operação segura e não disruptiva que libere cache deles (prune Docker libera disco, não RAM, e é destrutivo).

- `c` limpa o cache do serviço selecionado.
- `T` varre SO + serviços e limpa tudo de uma vez.
- Ambos listam no modal de confirmação exatamente o que será limpo antes de executar.

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
