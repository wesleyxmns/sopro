//go:build linux

package system

import (
	"os"
	"os/exec"
	"testing"
)

func TestLinuxPlatformManager_Metrics(t *testing.T) {
	var mgr PlatformManager = NewPlatformManager()
	metrics, err := mgr.GetMetrics()
	if err != nil {
		t.Fatalf("Failed to get metrics: %v", err)
	}
	if metrics.TotalRAM == 0 {
		t.Error("Expected TotalRAM to be greater than 0")
	}
}

func TestLinuxPlatformManager_GetProcesses(t *testing.T) {
	var mgr PlatformManager = NewPlatformManager()
	procs, err := mgr.GetProcesses(5)
	if err != nil {
		t.Fatalf("Failed to get processes: %v", err)
	}
	if len(procs) == 0 {
		t.Error("Expected to find at least one process")
	}
	if len(procs) > 5 {
		t.Errorf("Expected at most 5 processes, got %d", len(procs))
	}

	// Verify that critical processes are indeed marked as CRIT
	var foundCrit bool
	allProcs, err := mgr.GetProcesses(0)
	if err == nil {
		for _, p := range allProcs {
			if p.Risk == "CRIT" {
				foundCrit = true
				break
			}
		}
	}
	if !foundCrit {
		t.Error("Expected to find at least one CRIT process in all processes")
	}
}

func TestLinuxPlatformManager_KillProcess_Invalid(t *testing.T) {
	var mgr PlatformManager = NewPlatformManager()
	
	// Test KillProcess validations
	err := mgr.KillProcess(-1)
	if err == nil {
		t.Error("Expected error when killing invalid negative PID")
	}
	errZero := mgr.KillProcess(0)
	if errZero == nil {
		t.Error("Expected error when killing invalid PID 0")
	}

	// Test PauseProcess validations
	errPause := mgr.PauseProcess(-1)
	if errPause == nil {
		t.Error("Expected error when pausing invalid negative PID")
	}
	errPauseZero := mgr.PauseProcess(0)
	if errPauseZero == nil {
		t.Error("Expected error when pausing invalid PID 0")
	}

	// Test ResumeProcess validations
	errResume := mgr.ResumeProcess(-1)
	if errResume == nil {
		t.Error("Expected error when resuming invalid negative PID")
	}
	errResumeZero := mgr.ResumeProcess(0)
	if errResumeZero == nil {
		t.Error("Expected error when resuming invalid PID 0")
	}
}

func TestLinuxPlatformManager_CleanSystemCache_NonRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("Skipping test; running as root")
	}
	var mgr PlatformManager = NewPlatformManager()
	_, err := mgr.CleanSystemCache()
	if err != os.ErrPermission {
		t.Errorf("Expected os.ErrPermission, got %v", err)
	}
}

func TestLinuxPlatformManager_PauseResumeKill(t *testing.T) {
	cmd := exec.Command("sleep", "10")
	if err := cmd.Start(); err != nil {
		t.Fatalf("Failed to start sleep command: %v", err)
	}
	pid := int32(cmd.Process.Pid)

	var mgr PlatformManager = NewPlatformManager()

	// Pause the process
	if err := mgr.PauseProcess(pid); err != nil {
		t.Errorf("Failed to pause process: %v", err)
	}

	// Resume the process
	if err := mgr.ResumeProcess(pid); err != nil {
		t.Errorf("Failed to resume process: %v", err)
	}

	// Kill the process
	if err := mgr.KillProcess(pid); err != nil {
		t.Errorf("Failed to kill process: %v", err)
	}

	// Wait for process to finish
	_ = cmd.Wait()
}
