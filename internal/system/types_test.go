package system_test

import (
	"testing"
	"memcleaner/internal/system"
)

func TestProcessInfo_Init(t *testing.T) {
	p := system.ProcessInfo{
		PID:     1,
		User:    "root",
		MemPct:  1.5,
		CPUPct:  2.0,
		Command: "init",
	}

	if p.PID != 1 {
		t.Errorf("expected PID 1, got %d", p.PID)
	}
}

func TestSystemMetrics_Init(t *testing.T) {
	m := system.SystemMetrics{
		TotalRAM: 1024,
	}

	if m.TotalRAM != 1024 {
		t.Errorf("expected TotalRAM 1024, got %d", m.TotalRAM)
	}
}