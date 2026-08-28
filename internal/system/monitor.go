package system

import (
	"fmt"
	"sort"

	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

func GetSystemMetrics() (SystemMetrics, error) {
	v, err := mem.VirtualMemory()
	if err != nil {
		return SystemMetrics{}, err
	}
	s, err := mem.SwapMemory()
	if err != nil {
		return SystemMetrics{}, err
	}
	return SystemMetrics{
		TotalRAM:  v.Total,
		UsedRAM:   v.Used,
		FreeRAM:   v.Free,
		CacheRAM:  v.Cached + v.Buffers,
		SwapTotal: s.Total,
		SwapUsed:  s.Used,
	}, nil
}

func GetTopProcesses(limit int) ([]ProcessInfo, error) {
	pids, err := process.Processes()
	if err != nil {
		return nil, err
	}

	var infos []ProcessInfo
	for _, p := range pids {
		memPct, err := p.MemoryPercent()
		if err != nil {
			continue // Skip processes that disappear
		}
		// CPU, Name, and Username read errors are acceptable to ignore so the process list
		// is still partially populated even if a process disappears or restricts access.
		cpuPct, _ := p.CPUPercent()
		cmd, _ := p.Name()
		user, _ := p.Username()

		infos = append(infos, ProcessInfo{
			PID:     p.Pid,
			User:    user,
			MemPct:  memPct,
			CPUPct:  cpuPct,
			Command: cmd,
		})
	}

	sort.Slice(infos, func(i, j int) bool {
		return infos[i].MemPct > infos[j].MemPct
	})

	if limit < 0 {
		return nil, fmt.Errorf("limit must be non-negative")
	}
	if limit > 0 && len(infos) > limit {
		infos = infos[:limit]
	}
	return infos, nil
}

