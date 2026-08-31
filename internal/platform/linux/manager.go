//go:build linux

package linux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/wesleyxmns/sopro/internal/app"
	"github.com/wesleyxmns/sopro/internal/memory"
	processdomain "github.com/wesleyxmns/sopro/internal/process"

	"github.com/shirou/gopsutil/v3/mem"
	gprocess "github.com/shirou/gopsutil/v3/process"
)

var ErrProcessIdentityChanged = errors.New("process identity changed")

type Manager struct{}

var (
	_ app.SnapshotSource     = (*Manager)(nil)
	_ app.ProcessController  = (*Manager)(nil)
	_ app.ProcessWaiter      = (*Manager)(nil)
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
		return app.Snapshot{}, fmt.Errorf("read memory metrics: %w", err)
	}
	swap, err := mem.SwapMemoryWithContext(ctx)
	if err != nil {
		return app.Snapshot{}, fmt.Errorf("read swap metrics: %w", err)
	}
	pressure := readMemoryPressure()
	processes, err := gprocess.ProcessesWithContext(ctx)
	if err != nil {
		return app.Snapshot{}, fmt.Errorf("list processes: %w", err)
	}

	infos := make([]processdomain.Info, 0, len(processes))
	for _, proc := range processes {
		if err := ctx.Err(); err != nil {
			return app.Snapshot{}, err
		}

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
		statuses, _ := proc.StatusWithContext(ctx)

		state := processdomain.StateRunning
		for _, status := range statuses {
			if status == "T" || strings.Contains(strings.ToLower(status), "stop") {
				state = processdomain.StatePaused
				break
			}
		}

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
			State:       state,
		})
	}

	sort.SliceStable(infos, func(i, j int) bool {
		return infos[i].MemoryPct > infos[j].MemoryPct
	})
	if limit > 0 && len(infos) > limit {
		infos = infos[:limit]
	}

	return app.Snapshot{
		Memory: memory.Snapshot{
			Total:       vm.Total,
			Used:        vm.Used,
			Available:   vm.Available,
			Free:        vm.Free,
			Cache:       vm.Cached + vm.Buffers,
			SwapTotal:   swap.Total,
			SwapUsed:    swap.Used,
			Reclaimable: vm.Cached + vm.Buffers,
			Pressure:    pressure,
		},
		Processes: infos,
		TakenAt:   time.Now(),
	}, nil
}

func readMemoryPressure() memory.Pressure {
	file, err := os.Open("/proc/pressure/memory")
	if err != nil {
		return memory.Pressure{}
	}
	defer file.Close()
	pressure, err := memory.ParsePSI(file)
	if err != nil {
		return memory.Pressure{}
	}
	return pressure
}

func (m *Manager) Terminate(ctx context.Context, id processdomain.Identity) error {
	return m.signal(ctx, id, syscall.SIGTERM)
}

func (m *Manager) Kill(ctx context.Context, id processdomain.Identity) error {
	return m.signal(ctx, id, syscall.SIGKILL)
}

func (m *Manager) Pause(ctx context.Context, id processdomain.Identity) error {
	return m.signal(ctx, id, syscall.SIGSTOP)
}

func (m *Manager) Resume(ctx context.Context, id processdomain.Identity) error {
	return m.signal(ctx, id, syscall.SIGCONT)
}

func (m *Manager) WaitForExit(ctx context.Context, id processdomain.Identity) error {
	if err := id.Validate(); err != nil {
		return err
	}
	ticker := time.NewTicker(50 * time.Millisecond)
	defer ticker.Stop()
	for {
		exists, err := gprocess.PidExistsWithContext(ctx, id.PID)
		if err != nil {
			return err
		}
		if !exists {
			return nil
		}
		if id.StartedAt != 0 {
			proc, err := gprocess.NewProcess(id.PID)
			if err != nil {
				return nil
			}
			startedAt, err := proc.CreateTimeWithContext(ctx)
			if err != nil {
				return err
			}
			if startedAt != id.StartedAt {
				return nil
			}
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
		}
	}
}

func (m *Manager) signal(ctx context.Context, id processdomain.Identity, signal syscall.Signal) error {
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
		return fmt.Errorf("revalidate process %d: %w", id.PID, err)
	}
	if id.StartedAt != 0 && startedAt != id.StartedAt {
		return fmt.Errorf("%w: PID %d was reused", ErrProcessIdentityChanged, id.PID)
	}

	osProcess, err := os.FindProcess(int(id.PID))
	if err != nil {
		return err
	}
	return osProcess.Signal(signal)
}

func (m *Manager) CleanCache(ctx context.Context) (uint64, error) {
	if os.Geteuid() != 0 {
		return 0, os.ErrPermission
	}

	before, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return 0, err
	}
	if err := exec.CommandContext(ctx, "sync").Run(); err != nil {
		return 0, fmt.Errorf("sync filesystems: %w", err)
	}
	if err := os.WriteFile("/proc/sys/vm/drop_caches", []byte("3\n"), 0o644); err != nil {
		return 0, fmt.Errorf("drop caches: %w", err)
	}
	after, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return 0, err
	}
	beforeCache := before.Cached + before.Buffers
	afterCache := after.Cached + after.Buffers
	if afterCache >= beforeCache {
		return 0, nil
	}
	return beforeCache - afterCache, nil
}

func (m *Manager) Capabilities() app.Capabilities {
	return app.Capabilities{
		Platform:      "linux",
		Elevated:      os.Geteuid() == 0,
		CanTerminate:  true,
		CanKill:       true,
		CanPause:      true,
		CanResume:     true,
		CanCleanCache: os.Geteuid() == 0,
	}
}
