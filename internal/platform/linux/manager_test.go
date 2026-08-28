//go:build linux

package linux

import (
	"context"
	"os"
	"os/exec"
	"testing"

	"sopro/internal/platform/platformtest"
	processdomain "sopro/internal/process"

	gprocess "github.com/shirou/gopsutil/v3/process"
)

func TestManagerPlatformContract(t *testing.T) {
	platformtest.VerifyContract(t, New(), "linux")
}

func TestManagerSnapshot(t *testing.T) {
	snapshot, err := New().Snapshot(context.Background(), 5)
	if err != nil {
		t.Fatalf("Snapshot() error = %v", err)
	}
	if snapshot.Memory.Total == 0 {
		t.Fatal("expected total memory")
	}
	if len(snapshot.Processes) == 0 || len(snapshot.Processes) > 5 {
		t.Fatalf("expected between 1 and 5 processes, got %d", len(snapshot.Processes))
	}
}

func TestManagerRejectsInvalidIdentity(t *testing.T) {
	if err := New().Kill(context.Background(), processdomain.Identity{}); err == nil {
		t.Fatal("expected invalid identity error")
	}
}

func TestManagerRejectsNegativeProcessLimit(t *testing.T) {
	if _, err := New().Snapshot(context.Background(), -1); err == nil {
		t.Fatal("expected negative limit to fail")
	}
}

func TestManagerCleanCacheRequiresRoot(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	if _, err := New().CleanCache(context.Background()); !os.IsPermission(err) {
		t.Fatalf("expected permission error, got %v", err)
	}
}

func TestManagerPauseResumeAndKill(t *testing.T) {
	command := exec.Command("sleep", "10")
	if err := command.Start(); err != nil {
		t.Fatalf("start sleep: %v", err)
	}
	defer func() {
		_ = command.Process.Kill()
		_ = command.Wait()
	}()

	proc, err := gprocess.NewProcess(int32(command.Process.Pid))
	if err != nil {
		t.Fatalf("open child process: %v", err)
	}
	startedAt, err := proc.CreateTime()
	if err != nil {
		t.Fatalf("read child start time: %v", err)
	}
	id := processdomain.Identity{PID: int32(command.Process.Pid), StartedAt: startedAt}
	manager := New()

	if err := manager.Pause(context.Background(), id); err != nil {
		t.Fatalf("pause: %v", err)
	}
	if err := manager.Resume(context.Background(), id); err != nil {
		t.Fatalf("resume: %v", err)
	}
	if err := manager.Kill(context.Background(), id); err != nil {
		t.Fatalf("kill: %v", err)
	}
}
