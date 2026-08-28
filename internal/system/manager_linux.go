//go:build linux

package system

import (
	"fmt"
	"os"
	"os/exec"
	"sort"
	"syscall"

	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

type LinuxPlatformManager struct{}

func NewPlatformManager() PlatformManager {
	return &LinuxPlatformManager{}
}

func (l *LinuxPlatformManager) GetMetrics() (SystemMetrics, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return SystemMetrics{}, err
	}
	s, err := mem.SwapMemory()
	if err != nil {
		return SystemMetrics{}, err
	}
	return SystemMetrics{
		TotalRAM:    v.Total,
		UsedRAM:     v.Used,
		FreeRAM:     v.Free,
		CacheRAM:    v.Cached + v.Buffers,
		SwapTotal:   s.Total,
		SwapUsed:    s.Used,
		Reclaimable: v.Cached + v.Buffers,
	}, nil
}

func (l *LinuxPlatformManager) GetProcesses(limit int) ([]ProcessInfo, error) {
	pids, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var infos []ProcessInfo
	for _, p := range pids {
		memPct, err := p.MemoryPercent()
		if err != nil {
			continue // Skip transient processes
		}
		cpuPct, _ := p.CPUPercent()
		cmd, _ := p.Name()
		user, _ := p.Username()

		// Categorização simples de risco
		risk := "OK"
		if p.Pid <= 100 || cmd == "systemd" || cmd == "gnome-shell" || cmd == "kwin" {
			risk = "CRIT"
		}

		infos = append(infos, ProcessInfo{
			PID:     p.Pid,
			User:    user,
			MemPct:  memPct,
			CPUPct:  cpuPct,
			Command: cmd,
			State:   "Running",
			Risk:    risk,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].MemPct > infos[j].MemPct
	})

	if limit > 0 && len(infos) > limit {
		infos = infos[:limit]
	}
	return infos, nil
}

func (l *LinuxPlatformManager) KillProcess(pid int32) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: must be positive")
	}
	p, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGKILL)
}

func (l *LinuxPlatformManager) PauseProcess(pid int32) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: must be positive")
	}
	p, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGSTOP)
}

func (l *LinuxPlatformManager) ResumeProcess(pid int32) error {
	if pid <= 0 {
		return fmt.Errorf("invalid PID: must be positive")
	}
	p, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return p.Signal(syscall.SIGCONT)
}

func (l *LinuxPlatformManager) CleanSystemCache() (uint64, error) {
	if os.Geteuid() != 0 {
		return 0, os.ErrPermission
	}
	cmd := exec.Command("sh", "-c", "sync; echo 3 > /proc/sys/vm/drop_caches")
	if err := cmd.Run(); err != nil {
		return 0, err
	}
	return 0, nil // Quantificação real de quanto limpou pode ser implementada na fase 2
}
