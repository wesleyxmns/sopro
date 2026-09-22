package provider

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

	processdomain "github.com/wesleyxmns/sopro/internal/process"

	gprocess "github.com/shirou/gopsutil/v3/process"
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

func (j *JVMProvider) CacheTargets(proc processdomain.Info) []CacheTarget {
	if proc.PID <= 0 {
		return nil
	}
	command := proc.Command
	if command == "" {
		command = "java"
	}
	return []CacheTarget{
		{
			Source:   "JVM",
			ActionID: "jvm.run_gc",
			Label:    "Coleta de lixo (GC) no heap",
			Detail:   fmt.Sprintf("PID %d · %s", proc.PID, command),
			Proc:     proc,
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

func (j *JVMProvider) Execute(ctx context.Context, actionID string, proc processdomain.Info) (uint64, error) {
	if proc.PID <= 0 {
		return 0, fmt.Errorf("%w: PID inválido para jcmd", ErrUnsupported)
	}

	switch actionID {
	case "jvm.run_gc":
		before := residentBytes(proc.PID)
		if _, err := j.runner.Run(ctx, "jcmd", strconv.Itoa(int(proc.PID)), "GC.run"); err != nil {
			return 0, err
		}
		return reclaimedBytes(before, residentBytes(proc.PID)), nil
	default:
		return 0, fmt.Errorf("%w: ação %s", ErrActionNotFound, actionID)
	}
}

func residentBytes(pid int32) uint64 {
	proc, err := gprocess.NewProcess(pid)
	if err != nil {
		return 0
	}
	info, err := proc.MemoryInfo()
	if err != nil || info == nil {
		return 0
	}
	return info.RSS
}

// reclaimedBytes reports the resident drop across a cleanup. A zero reading
// means the process vanished or is unreadable, never reclaimed memory.
func reclaimedBytes(before, after uint64) uint64 {
	if after == 0 || after >= before {
		return 0
	}
	return before - after
}
