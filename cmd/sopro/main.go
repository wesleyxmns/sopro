package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/wesleyxmns/sopro/internal/app"
	"github.com/wesleyxmns/sopro/internal/audit"
	"github.com/wesleyxmns/sopro/internal/config"
	"github.com/wesleyxmns/sopro/internal/daemon"
	"github.com/wesleyxmns/sopro/internal/memory"
	"github.com/wesleyxmns/sopro/internal/platform"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
	"github.com/wesleyxmns/sopro/internal/provider"
	"github.com/wesleyxmns/sopro/internal/tui"
	"github.com/wesleyxmns/sopro/internal/updater"
	"github.com/wesleyxmns/sopro/internal/version"

	tea "github.com/charmbracelet/bubbletea"
)

// configFile holds sopro.conf values, consulted after real environment
// variables and before built-in defaults.
var configFile = config.File{}

func main() {
	if loaded, err := config.Load(config.Path()); err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao ler configuração: %v\n", err)
		os.Exit(2)
	} else {
		configFile = loaded
	}

	if handled, reclaimed, err := platform.RunPrivilegedHelper(context.Background(), os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, reclaimed)
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "update" {
		handleUpdateCommand(os.Args[2:])
		return
	}

	if len(os.Args) > 1 && os.Args[1] == "audit" {
		handleAuditCommand(os.Args[2:])
		return
	}

	showVersion := flag.Bool("version", false, "exibe a versão do Sopro e sai")
	flag.BoolVar(showVersion, "v", false, "exibe a versão do Sopro e sai (atalho)")

	defaultTheme := envString("SOPRO_THEME", "auto")
	if defaultTheme == "" {
		defaultTheme = "auto"
	}
	themeName := flag.String("theme", defaultTheme, "tema da interface: auto, dark, light, mono ou cyber")
	defaultRisk := processdomain.DefaultRiskPolicyConfig()
	criticalPIDMax := flag.Int("risk-critical-pid-max", envInt("SOPRO_RISK_CRITICAL_PID_MAX", int(defaultRisk.CriticalPIDMax)), "maior PID sempre considerado crítico; 0 desativa")
	criticalCommands := flag.String("risk-critical-commands", envString("SOPRO_RISK_CRITICAL_COMMANDS", strings.Join(defaultRisk.CriticalCommands, ",")), "comandos críticos separados por vírgula")
	warningCommands := flag.String("risk-warning-commands", envString("SOPRO_RISK_WARNING_COMMANDS", ""), "comandos de atenção separados por vírgula")
	terminationGrace := flag.Duration("terminate-grace", envDuration("SOPRO_TERMINATE_GRACE", 2*time.Second), "tempo entre término gracioso e encerramento forçado")
	auditPath := flag.String("audit-log", envString("SOPRO_AUDIT_LOG", audit.DefaultPath()), "arquivo JSONL de auditoria das ações")
	daemonMode := flag.Bool("daemon", false, "executa o Sopro em modo daemon de segundo plano sem TUI")
	daemonInterval := flag.Duration("daemon-interval", envDuration("SOPRO_DAEMON_INTERVAL", 3*time.Second), "intervalo de amostragem do daemon")
	daemonEnforce := flag.Bool("daemon-enforce", envBool("SOPRO_DAEMON_ENFORCE", false), "executa ações de alívio automáticas (padrão: falso / modo observação)")
	daemonSustained := flag.Duration("daemon-sustained", envDuration("SOPRO_DAEMON_SUSTAINED", 15*time.Second), "duração contínua de pressão necessária para recomendar/executar alívio")
	daemonCooldown := flag.Duration("daemon-cooldown", envDuration("SOPRO_DAEMON_COOLDOWN", 60*time.Second), "intervalo mínimo entre ações de alívio")
	daemonMemThreshold := flag.Float64("daemon-memory-threshold", envFloat("SOPRO_DAEMON_MEMORY_THRESHOLD", 90.0), "limiar de uso de memória (%) para considerar pressão")
	daemonJSON := flag.Bool("daemon-json", envBool("SOPRO_DAEMON_JSON", false), "emite decisões do daemon como JSON (uma por linha)")
	daemonWebhook := flag.String("daemon-webhook-url", envString("SOPRO_DAEMON_WEBHOOK_URL", ""), "URL para alertas webhook do daemon (vazio desativa)")

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Sopro — Observabilidade e controle de processos e memória\n\n")
		fmt.Fprintf(os.Stderr, "Uso:\n")
		fmt.Fprintf(os.Stderr, "  sopro [opções]\n")
		fmt.Fprintf(os.Stderr, "  sopro update [--check]   atualiza o Sopro para a versão mais recente\n")
		fmt.Fprintf(os.Stderr, "  sopro audit [--last N]   lista os últimos eventos de auditoria\n\n")
		fmt.Fprintf(os.Stderr, "Opções:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nAtalhos na TUI:\n")
		fmt.Fprintf(os.Stderr, "  setas       navegar pelos processos\n")
		fmt.Fprintf(os.Stderr, "  /           pesquisa fuzzy de comandos\n")
		fmt.Fprintf(os.Stderr, "  f, tab      alternar filtros de categoria (sistema, containers, dev, etc.)\n")
		fmt.Fprintf(os.Stderr, "  s           alternar ordenação (memória, CPU, comando)\n")
		fmt.Fprintf(os.Stderr, "  g           alternar agrupamento (lista, categorias, árvore)\n")
		fmt.Fprintf(os.Stderr, "  p           pausar / retomar processo\n")
		fmt.Fprintf(os.Stderr, "  x           encerrar processo graciosamente (SIGTERM)\n")
		fmt.Fprintf(os.Stderr, "  k           forçar encerramento imediato (SIGKILL)\n")
		fmt.Fprintf(os.Stderr, "  c           limpar cache do serviço selecionado\n")
		fmt.Fprintf(os.Stderr, "  T           limpeza total dos caches (SO + serviços)\n")
		fmt.Fprintf(os.Stderr, "  d/r/z/s     ações em containers Docker (stop, restart, pause, start)\n")
		fmt.Fprintf(os.Stderr, "  b/j/w/v     ações contextuais (navegador, JVM, git)\n")
		fmt.Fprintf(os.Stderr, "  q           sair\n")
	}

	flag.Parse()

	if *showVersion {
		fmt.Println(version.Info())
		return
	}

	theme, err := tui.ThemeFor(*themeName)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	riskPolicy, err := processdomain.NewRiskPolicy(processdomain.RiskPolicyConfig{
		CriticalPIDMax:   int32(*criticalPIDMax),
		CriticalCommands: splitCommands(*criticalCommands),
		WarningCommands:  splitCommands(*warningCommands),
	})
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	auditRecorder, err := audit.Open(*auditPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	defer auditRecorder.Close()
	dependencies := platform.New()
	dependencies.Audit = auditRecorder
	providerRegistry := provider.NewRegistry(
		provider.NewDockerProvider(),
		provider.NewGitProvider(),
		provider.NewCDPProvider(),
		provider.NewJVMProvider(),
	)
	service := app.NewService(
		dependencies,
		app.WithRiskPolicy(riskPolicy),
		app.WithTerminationGracePeriod(*terminationGrace),
		app.WithProviderRegistry(providerRegistry),
	)

	if *daemonMode {
		ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
		defer cancel()

		daemonCfg := daemon.Config{
			Interval:          *daemonInterval,
			ObserveOnly:       !*daemonEnforce,
			MemoryUsagePct:    *daemonMemThreshold,
			SustainedDuration: *daemonSustained,
			Cooldown:          *daemonCooldown,
			AllowCacheClean:   true,
		}

		modeLabel := "OBSERVAÇÃO (dry-run seguro)"
		if *daemonEnforce {
			modeLabel = "ATIVO (ações automáticas habilitadas)"
		}
		fmt.Printf("Iniciando Sopro Daemon em modo %s (intervalo: %v, limiar: %.1f%%)\n", modeLabel, *daemonInterval, *daemonMemThreshold)

		var consoleNotifier daemon.Notifier = daemon.NewLogNotifier(os.Stdout)
		if *daemonJSON {
			consoleNotifier = daemon.NewJSONNotifier(os.Stdout)
		}
		auditNotifier := daemon.NewAuditNotifier(auditRecorder)
		notifiers := []daemon.Notifier{consoleNotifier, auditNotifier}
		if strings.TrimSpace(*daemonWebhook) != "" {
			notifiers = append(notifiers, daemon.NewWebhookNotifier(*daemonWebhook))
		}
		multiNotifier := daemon.NewMultiNotifier(notifiers...)

		d := daemon.New(service, daemonCfg, multiNotifier)
		if err := d.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "Sopro Daemon finalizou com erro: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Sopro Daemon encerrado graciosamente.")
		return
	}

	program := tea.NewProgram(tui.NewModel(service, tui.WithTheme(theme)), tea.WithAltScreen())
	finalModel, err := program.Run()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Sopro falhou: %v\n", err)
		os.Exit(1)
	}
	if model, ok := finalModel.(tui.Model); ok && model.RestartRequested {
		fmt.Fprintln(os.Stderr, "Reiniciando com a nova versão…")
		if err := updater.Restart(); err != nil {
			fmt.Fprintf(os.Stderr, "Reinício automático falhou (%v). Reinicie manualmente.\n", err)
			os.Exit(1)
		}
	}
}

func lookupEnv(name string) (string, bool) {
	if value, ok := os.LookupEnv(name); ok {
		return value, true
	}
	return configFile.Lookup(name)
}

func envString(name, fallback string) string {
	if value, ok := lookupEnv(name); ok {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, ok := lookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s inválido: %v\n", name, err)
		os.Exit(2)
	}
	return parsed
}

func splitCommands(value string) []string {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.Split(value, ",")
}

func envDuration(name string, fallback time.Duration) time.Duration {
	value, ok := lookupEnv(name)
	if !ok || strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		fmt.Fprintf(os.Stderr, "%s inválido: %v\n", name, err)
		os.Exit(2)
	}
	return parsed
}

func envBool(name string, fallback bool) bool {
	if value, ok := lookupEnv(name); ok {
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func envFloat(name string, fallback float64) float64 {
	if value, ok := lookupEnv(name); ok {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func handleAuditCommand(args []string) {
	auditFlags := flag.NewFlagSet("audit", flag.ExitOnError)
	last := auditFlags.Int("last", 20, "quantos eventos recentes listar")
	auditFlags.IntVar(last, "n", 20, "quantos eventos recentes listar (atalho)")
	logPath := auditFlags.String("audit-log", envString("SOPRO_AUDIT_LOG", audit.DefaultPath()), "arquivo JSONL de auditoria das ações")
	_ = auditFlags.Parse(args)

	events, err := audit.ReadLast(*logPath, *last)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao ler auditoria: %v\n", err)
		os.Exit(1)
	}
	if len(events) == 0 {
		fmt.Println("Nenhum evento de auditoria registrado.")
		return
	}
	for _, event := range events {
		fmt.Println(formatAuditEvent(event))
	}
}

func formatAuditEvent(event audit.Event) string {
	var parts []string
	parts = append(parts, event.FinishedAt.Format("2006-01-02 15:04:05"))
	parts = append(parts, event.Action)
	if event.PID > 0 {
		parts = append(parts, fmt.Sprintf("pid %d", event.PID))
	}
	if event.ReclaimedBytes > 0 {
		parts = append(parts, memory.FormatBytes(event.ReclaimedBytes)+" recuperados")
	}
	if event.Success {
		parts = append(parts, "ok")
	} else {
		parts = append(parts, "FALHA")
		if event.Error != "" {
			parts = append(parts, event.Error)
		}
	}
	return strings.Join(parts, " · ")
}

func handleUpdateCommand(args []string) {
	updateFlags := flag.NewFlagSet("update", flag.ExitOnError)
	checkOnly := updateFlags.Bool("check", false, "apenas verifica se há atualizações disponíveis sem instalar")
	updateFlags.BoolVar(checkOnly, "c", false, "apenas verifica se há atualizações disponíveis sem instalar (atalho)")
	_ = updateFlags.Parse(args)
	if !*checkOnly {
		executable, requiresElevation, err := updater.UpdateTarget()
		if err != nil {
			fmt.Fprintf(os.Stderr, "Erro ao preparar atualização: %v\n", err)
			os.Exit(1)
		}
		if requiresElevation {
			fmt.Println("A instalação atual requer permissão administrativa; solicitando via sudo...")
			command, commandErr := updater.ElevatedCommand(executable, append([]string{"update"}, args...)...)
			if commandErr != nil {
				fmt.Fprintf(os.Stderr, "Erro ao preparar atualização: %v\n", commandErr)
				os.Exit(1)
			}
			command.Stdin = os.Stdin
			command.Stdout = os.Stdout
			command.Stderr = os.Stderr
			if commandErr = command.Run(); commandErr != nil {
				fmt.Fprintf(os.Stderr, "Erro na atualização com permissão administrativa: %v\n", commandErr)
				os.Exit(1)
			}
			return
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	checker := updater.NewChecker()
	fmt.Println("Buscando atualizações no GitHub...")
	release, isNew, err := checker.Check(ctx, version.Version, true)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Erro ao buscar atualizações: %v\n", err)
		os.Exit(1)
	}

	if !isNew {
		fmt.Printf("O Sopro já está na versão mais recente (%s).\n", version.Short())
		return
	}

	fmt.Printf("Nova versão disponível: %s (versão atual: %s)\n", release.TagName, version.Short())
	if release.ReleaseNotes != "" {
		fmt.Printf("\nNovidades:\n%s\n\n", release.ReleaseNotes)
	}

	if *checkOnly {
		fmt.Println("Para atualizar, execute: sopro update")
		return
	}

	fmt.Printf("Baixando e instalando %s...\n", release.TagName)
	if err := updater.Apply(ctx, release); err != nil {
		fmt.Fprintf(os.Stderr, "Erro na atualização: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("✔ Sopro atualizado com sucesso para %s!\n", release.TagName)
}
