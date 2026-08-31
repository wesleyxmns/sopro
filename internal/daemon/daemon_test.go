package daemon

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/wesleyxmns/sopro/internal/app"
	"github.com/wesleyxmns/sopro/internal/memory"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
)

type mockService struct {
	snapshot       app.Snapshot
	cleanCacheErr  error
	cleanCacheCall int
	terminateErr   error
	terminateCall  int
}

func (m *mockService) Snapshot(context.Context, int) (app.Snapshot, error) {
	return m.snapshot, nil
}

func (m *mockService) CleanCache(context.Context) (uint64, error) {
	m.cleanCacheCall++
	return 1024 * 1024, m.cleanCacheErr
}

func (m *mockService) Terminate(context.Context, processdomain.Identity) error {
	m.terminateCall++
	return m.terminateErr
}

type mockNotifier struct {
	decisions []Decision
}

func (m *mockNotifier) Notify(d Decision) {
	m.decisions = append(m.decisions, d)
}

func TestDaemonObserveOnlyDoesNotExecuteActions(t *testing.T) {
	svc := &mockService{
		snapshot: app.Snapshot{
			Memory: memory.Snapshot{
				Total:       1000,
				Used:        950, // 95% usage
				Reclaimable: 200,
			},
			Processes: []processdomain.Info{
				{
					Identity: processdomain.Identity{PID: 101},
					Leak:     processdomain.LeakAssessment{Status: processdomain.LeakSuspected},
				},
			},
		},
	}
	notifier := &mockNotifier{}
	cfg := Config{
		ObserveOnly:       true,
		MemoryUsagePct:    90.0,
		SustainedDuration: 0, // Trigger immediately for test
		AllowCacheClean:   true,
	}

	d := New(svc, cfg, notifier)
	decision, err := d.Tick(context.Background())
	if err != nil {
		t.Fatalf("unexpected tick error: %v", err)
	}

	if !decision.UnderPressure {
		t.Fatal("expected under pressure to be true")
	}
	if decision.Executed {
		t.Fatal("action must not be executed in observe-only mode")
	}
	if svc.cleanCacheCall != 0 {
		t.Fatalf("expected 0 clean cache calls, got %d", svc.cleanCacheCall)
	}
	if len(decision.SuspectedProcesses) != 1 || decision.SuspectedProcesses[0].PID != 101 {
		t.Fatalf("unexpected suspected processes: %+v", decision.SuspectedProcesses)
	}
	if !strings.Contains(decision.Reason, "modo observação") {
		t.Fatalf("reason = %q; want observe mode explanation", decision.Reason)
	}
}

func TestDaemonEnforceExecutesAndEntersCooldown(t *testing.T) {
	svc := &mockService{
		snapshot: app.Snapshot{
			Memory: memory.Snapshot{
				Total:       1000,
				Used:        950,
				Reclaimable: 200,
			},
		},
	}
	cfg := Config{
		ObserveOnly:       false,
		MemoryUsagePct:    90.0,
		SustainedDuration: 0,
		Cooldown:          10 * time.Minute,
		AllowCacheClean:   true,
	}

	d := New(svc, cfg)
	ctx := context.Background()

	// First tick executes
	decision1, err := d.Tick(ctx)
	if err != nil {
		t.Fatalf("tick 1 error: %v", err)
	}
	if !decision1.Executed || svc.cleanCacheCall != 1 {
		t.Fatalf("expected action executed, got executed=%v, calls=%d", decision1.Executed, svc.cleanCacheCall)
	}

	// Second tick immediately after is in cooldown
	decision2, err := d.Tick(ctx)
	if err != nil {
		t.Fatalf("tick 2 error: %v", err)
	}
	if decision2.Executed || svc.cleanCacheCall != 1 {
		t.Fatalf("expected cooldown without execution, got executed=%v, calls=%d", decision2.Executed, svc.cleanCacheCall)
	}
	if !strings.Contains(decision2.Reason, "cooldown") {
		t.Fatalf("reason = %q; want cooldown mention", decision2.Reason)
	}
}

func TestDaemonSustainedDurationRequiresContinuousPressure(t *testing.T) {
	svc := &mockService{
		snapshot: app.Snapshot{
			Memory: memory.Snapshot{
				Total:       1000,
				Used:        950,
				Reclaimable: 200,
			},
		},
	}
	cfg := Config{
		ObserveOnly:       false,
		MemoryUsagePct:    90.0,
		SustainedDuration: 10 * time.Second,
		AllowCacheClean:   true,
	}

	d := New(svc, cfg)
	ctx := context.Background()

	// First tick starts tracking sustained pressure
	decision, _ := d.Tick(ctx)
	if decision.Executed || svc.cleanCacheCall != 0 {
		t.Fatal("action must not execute before sustained duration is reached")
	}
	if !strings.Contains(decision.Reason, "aguardando limite") {
		t.Fatalf("reason = %q; want waiting for duration limit", decision.Reason)
	}

	// Memory pressure drops
	svc.snapshot.Memory.Used = 400 // 40%
	normalDecision, _ := d.Tick(ctx)
	if normalDecision.UnderPressure {
		t.Fatal("expected pressure to be normal")
	}
	if d.pressureStartTime != nil {
		t.Fatal("pressure start time was not reset after pressure dropped")
	}
}

func TestDaemonRunStopsOnContextCancel(t *testing.T) {
	svc := &mockService{
		snapshot: app.Snapshot{
			Memory: memory.Snapshot{Total: 1000, Used: 200},
		},
	}
	cfg := Config{Interval: 10 * time.Millisecond, ObserveOnly: true}
	d := New(svc, cfg)

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Millisecond)
	defer cancel()

	err := d.Run(ctx)
	if err != context.DeadlineExceeded {
		t.Fatalf("expected DeadlineExceeded, got %v", err)
	}
}
