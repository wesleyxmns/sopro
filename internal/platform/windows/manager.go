//go:build windows

package windows

import (
	"context"
	"fmt"
	"os"
	"sort"
	"time"

	"sopro/internal/app"
	"sopro/internal/memory"
	processdomain "sopro/internal/process"

	"github.com/shirou/gopsutil/v3/mem"
	gprocess "github.com/shirou/gopsutil/v3/process"
)

type Manager struct{}

var (
	_ app.SnapshotSource     = (*Manager)(nil)
	_ app.ProcessController  = (*Manager)(nil)
	_ app.CacheCleaner       = (*Manager)(nil)
	_ app.CapabilityProvider = (*Manager)(nil)
)

func New() *Manager { return &Manager{} }

func (m *Manager) Snapshot(ctx context.Context, limit int) (app.Snapshot, error) {
	if limit < 0 {
		return app.Snapshot{}, fmt.Errorf("limit must be non-negative")
	}
	vm, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return app.Snapshot{}, err
	}
	swap, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		return app.Snapshot{}, err
	}
	processes, err := gprocess.ProcessesWithContext(ctx)
	if err != nil {
		return app.Snapshot{}, err
	}

	infos := make([]processdomain.Info, 0, len(processes))
	for _, proc := range processes {
		memPct, err := proc.MemoryPercentWithContext(ctx)
		if err != nil {
			continue
		}
		cpuPct, _ := proc.CPUPercentWithContext(ctx)
		name, _ := proc.NameWithContext(ctx)
		user, _ := proc.UsernameWithContext(ctx)
		startedAt, _ := proc.CreateTimeWithContext(ctx)
		parentPID, _ := proc.PpidWithContext(ctx)
		memoryInfo, _ := proc.MemoryInfoWithContext(ctx)
		var resident uint64
		if memoryInfo != nil {
			resident = memoryInfo.RSS
		}
		cmdline, _ := proc.CmdlineWithContext(ctx)
		cwd, _ := proc.CwdWithContext(ctx)

		infos = append(infos, processdomain.Info{
			Identity:    processdomain.Identity{PID: proc.Pid, StartedAt: startedAt},
			ParentPID:   parentPID,
			User:        user,
			MemoryBytes: resident,
			MemoryPct:   memPct,
			CPUPct:      cpuPct,
			Command:     name,
			CommandLine: cmdline,
			Cwd:         cwd,
			State:       processdomain.StateRunning,
		})
	}
	sort.SliceStable(infos, func(i, j int) bool { return infos[i].MemoryPct > infos[j].MemoryPct })
	if limit > 0 && len(infos) > limit {
		infos = infos[:limit]
	}

	return app.Snapshot{
		Memory: memory.Snapshot{
			Total: vm.Total, Used: vm.Used, Available: vm.Available, Free: vm.Free,
			Cache: vm.Cached, SwapTotal: swap.Total, SwapUsed: swap.Used, Reclaimable: vm.Cached,
		},
		Processes: infos,
		TakenAt:   time.Now(),
	}, nil
}

func (m *Manager) Terminate(ctx context.Context, id processdomain.Identity) error {
	return app.ErrUnsupported
}

func (m *Manager) Kill(ctx context.Context, id processdomain.Identity) error {
	if err := id.Validate(); err != nil {
		return err
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	proc, err := gprocess.NewProcess(id.PID)
	if err != nil {
		return err
	}
	startedAt, err := proc.CreateTimeWithContext(ctx)
	if err != nil {
		return err
	}
	if id.StartedAt != 0 && startedAt != id.StartedAt {
		return fmt.Errorf("process identity changed: PID %d was reused", id.PID)
	}
	osProcess, err := os.FindProcess(int(id.PID))
	if err != nil {
		return err
	}
	return osProcess.Kill()
}

func (m *Manager) Pause(context.Context, processdomain.Identity) error  { return app.ErrUnsupported }
func (m *Manager) Resume(context.Context, processdomain.Identity) error { return app.ErrUnsupported }
func (m *Manager) CleanCache(context.Context) (uint64, error)           { return 0, app.ErrUnsupported }

func (m *Manager) Capabilities() app.Capabilities {
	return app.Capabilities{Platform: "windows", CanKill: true}
}
