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

	"sopro/internal/app"
	"sopro/internal/audit"
	"sopro/internal/daemon"
	"sopro/internal/platform"
	processdomain "sopro/internal/process"
	"sopro/internal/provider"
	"sopro/internal/tui"

	tea "github.com/charmbracelet/bubbletea"
)

func main() {
	if handled, reclaimed, err := platform.RunPrivilegedHelper(context.Background(), os.Args[1:]); handled {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Fprintln(os.Stdout, reclaimed)
		return
	}

	defaultTheme := os.Getenv("SOPRO_THEME")
	if defaultTheme == "" {
		defaultTheme = "auto"
	}
	themeName := flag.String("theme", defaultTheme, "tema: auto, dark, light, mono ou cyber")
	defaultRisk := processdomain.DefaultRiskPolicyConfig()
	criticalPIDMax := flag.Int("risk-critical-pid-max", envInt("SOPRO_RISK_CRITICAL_PID_MAX", int(defaultRisk.CriticalPIDMax)), "maior PID sempre considerado crítico; 0 desativa")
	criticalCommands := flag.String("risk-critical-commands", envString("SOPRO_RISK_CRITICAL_COMMANDS", strings.Join(defaultRisk.CriticalCommands, ",")), "comandos críticos separados por vírgula")
	warningCommands := flag.String("risk-warning-commands", os.Getenv("SOPRO_RISK_WARNING_COMMANDS"), "comandos de atenção separados por vírgula")
	terminationGrace := flag.Duration("terminate-grace", envDuration("SOPRO_TERMINATE_GRACE", 2*time.Second), "tempo entre término gracioso e encerramento forçado")
	auditPath := flag.String("audit-log", envString("SOPRO_AUDIT_LOG", audit.DefaultPath()), "arquivo JSONL de auditoria das ações")
	daemonMode := flag.Bool("daemon", false, "executa o Sopro em modo daemon de segundo plano sem TUI")
	daemonInterval := flag.Duration("daemon-interval", envDuration("SOPRO_DAEMON_INTERVAL", 3*time.Second), "intervalo de amostragem do daemon")
	daemonEnforce := flag.Bool("daemon-enforce", envBool("SOPRO_DAEMON_ENFORCE", false), "executa ações de alívio automáticas (padrão: falso / modo observação)")
	daemonSustained := flag.Duration("daemon-sustained", envDuration("SOPRO_DAEMON_SUSTAINED", 15*time.Second), "duração contínua de pressão necessária para recomendar/executar alívio")
	daemonCooldown := flag.Duration("daemon-cooldown", envDuration("SOPRO_DAEMON_COOLDOWN", 60*time.Second), "intervalo mínimo entre ações de alívio")
	daemonMemThreshold := flag.Float64("daemon-memory-threshold", envFloat("SOPRO_DAEMON_MEMORY_THRESHOLD", 90.0), "limiar de uso de memória (%) para considerar pressão")
	flag.Parse()

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

		logNotifier := daemon.NewLogNotifier(os.Stdout)
		auditNotifier := daemon.NewAuditNotifier(auditRecorder)
		multiNotifier := daemon.NewMultiNotifier(logNotifier, auditNotifier)

		d := daemon.New(service, daemonCfg, multiNotifier)
		if err := d.Run(ctx); err != nil && !errors.Is(err, context.Canceled) {
			fmt.Fprintf(os.Stderr, "Sopro Daemon finalizou com erro: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("Sopro Daemon encerrado graciosamente.")
		return
	}

	program := tea.NewProgram(tui.NewModel(service, tui.WithTheme(theme)), tea.WithAltScreen())
	if _, err := program.Run(); err != nil {
		fmt.Fprintf(os.Stderr, "Sopro falhou: %v\n", err)
		os.Exit(1)
	}
}

func envString(name, fallback string) string {
	if value, ok := os.LookupEnv(name); ok {
		return value
	}
	return fallback
}

func envInt(name string, fallback int) int {
	value, ok := os.LookupEnv(name)
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
	value, ok := os.LookupEnv(name)
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
	if value, ok := os.LookupEnv(name); ok {
		parsed, err := strconv.ParseBool(strings.TrimSpace(value))
		if err == nil {
			return parsed
		}
	}
	return fallback
}

func envFloat(name string, fallback float64) float64 {
	if value, ok := os.LookupEnv(name); ok {
		parsed, err := strconv.ParseFloat(strings.TrimSpace(value), 64)
		if err == nil {
			return parsed
		}
	}
	return fallback
}
