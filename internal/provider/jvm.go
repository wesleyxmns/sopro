package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	processdomain "sopro/internal/process"
)

var (
	jvmXmxPattern = regexp.MustCompile(`(?i)-Xmx([0-9]+[kmgKMG]?)`)
)

type JVMProvider struct {
	runner CommandRunner
}

func NewJVMProvider(runner ...CommandRunner) *JVMProvider {
	var r CommandRunner = osCommandRunner{}
	if len(runner) > 0 && runner[0] != nil {
		r = runner[0]
	}
	return &JVMProvider{runner: r}
}

func (j *JVMProvider) Name() string {
	return "jvm"
}

func (j *JVMProvider) Supports(proc processdomain.Info) bool {
	if proc.Category == processdomain.CategoryJVM {
		return true
	}
	lower := strings.ToLower(proc.Command)
	return strings.Contains(lower, "java") ||
		strings.Contains(lower, "openjdk") ||
		strings.Contains(lower, "idea") ||
		strings.Contains(lower, "datagrip") ||
		strings.Contains(lower, "pycharm")
}

func (j *JVMProvider) Detect(ctx context.Context, proc processdomain.Info) []ContextInfo {
	details := map[string]string{
		"pid": strconv.Itoa(int(proc.PID)),
	}

	label := "JVM"
	if match := jvmXmxPattern.FindStringSubmatch(proc.CommandLine); len(match) > 1 {
		heapMax := strings.ToUpper(match[1])
		details["max_heap"] = heapMax
		label = fmt.Sprintf("JVM (Heap Max: %s)", heapMax)
	}

	return []ContextInfo{
		{
			Tag:     processdomain.ContextTag("jvm-runtime"),
			Label:   label,
			Details: details,
		},
	}
}

func (j *JVMProvider) Actions(ctx context.Context, proc processdomain.Info) []Action {
	if proc.PID <= 0 {
		return nil
	}
	return []Action{
		{
			ID:          "jvm.run_gc",
			Scope:       ScopeJVM,
			Label:       "forçar GC",
			Description: fmt.Sprintf("Executa 'jcmd %d GC.run' para liberar heap", proc.PID),
			Danger:      false,
		},
	}
}

func (j *JVMProvider) Execute(ctx context.Context, actionID string, proc processdomain.Info) error {
	if proc.PID <= 0 {
		return fmt.Errorf("%w: PID inválido para jcmd", ErrUnsupported)
	}

	switch actionID {
	case "jvm.run_gc":
		_, err := j.runner.Run(ctx, "jcmd", strconv.Itoa(int(proc.PID)), "GC.run")
		return err
	default:
		return fmt.Errorf("%w: ação %s", ErrActionNotFound, actionID)
	}
}
