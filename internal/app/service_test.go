package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"sopro/internal/audit"
	processdomain "sopro/internal/process"
	"sopro/internal/provider"
)

type snapshotSourceStub struct {
	snapshot Snapshot
}

type processControllerStub struct {
	terminated int
	killed     int
}

func (stub *processControllerStub) Terminate(context.Context, processdomain.Identity) error {
	stub.terminated++
	return nil
}

func (stub *processControllerStub) Kill(context.Context, processdomain.Identity) error {
	stub.killed++
	return nil
}

func (*processControllerStub) Pause(context.Context, processdomain.Identity) error  { return nil }
func (*processControllerStub) Resume(context.Context, processdomain.Identity) error { return nil }

type processWaiterStub struct {
	err error
}

type auditRecorderStub struct {
	events []audit.Event
	err    error
}

func (stub *auditRecorderStub) Record(event audit.Event) error {
	stub.events = append(stub.events, event)
	return stub.err
}

func (stub processWaiterStub) WaitForExit(ctx context.Context, _ processdomain.Identity) error {
	if stub.err != nil {
		return stub.err
	}
	<-ctx.Done()
	return ctx.Err()
}

func (stub snapshotSourceStub) Snapshot(context.Context, int) (Snapshot, error) {
	return stub.snapshot, nil
}

func TestServiceAppliesConfiguredRiskPolicy(t *testing.T) {
	policy, err := processdomain.NewRiskPolicy(processdomain.RiskPolicyConfig{
		WarningCommands: []string{"redis-server"},
	})
	if err != nil {
		t.Fatal(err)
	}
	service := NewService(Dependencies{Snapshots: snapshotSourceStub{snapshot: Snapshot{
		Processes: []processdomain.Info{{Identity: processdomain.Identity{PID: 500}, Command: "redis-server"}},
	}}}, WithRiskPolicy(policy))

	snapshot, err := service.Snapshot(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if got := snapshot.Processes[0].Risk; got != processdomain.RiskWarning {
		t.Fatalf("risk = %q; want warning", got)
	}
	if got := snapshot.Processes[0].Category; got != processdomain.CategoryDatabase {
		t.Fatalf("category = %q; want database", got)
	}
}

func TestTerminateReturnsWhenProcessExitsDuringGracePeriod(t *testing.T) {
	controller := &processControllerStub{}
	recorder := &auditRecorderStub{}
	service := NewService(Dependencies{
		Processes:     controller,
		ProcessWaiter: processWaiterStub{err: nil},
		Audit:         recorder,
	}, WithTerminationGracePeriod(time.Millisecond))

	if err := service.Terminate(context.Background(), processdomain.Identity{PID: 42}); err != nil {
		t.Fatal(err)
	}
	if controller.terminated != 1 || controller.killed != 1 {
		t.Fatalf("terminate/kill calls = %d/%d; want 1/1 after timeout", controller.terminated, controller.killed)
	}
	if len(recorder.events) != 1 || !recorder.events[0].Success || !recorder.events[0].Escalated {
		t.Fatalf("unexpected termination audit event: %+v", recorder.events)
	}
}

func TestTerminateDoesNotEscalateWhenWaiterReportsExit(t *testing.T) {
	controller := &processControllerStub{}
	service := NewService(Dependencies{
		Processes:     controller,
		ProcessWaiter: processWaiterStub{err: errors.New("exited")},
	})

	err := service.Terminate(context.Background(), processdomain.Identity{PID: 42})
	if err == nil || controller.killed != 0 {
		t.Fatalf("error/kill = %v/%d; want waiter error without escalation", err, controller.killed)
	}
}

type exitedWaiterStub struct{}

func (exitedWaiterStub) WaitForExit(context.Context, processdomain.Identity) error { return nil }

func TestTerminateStopsAfterGracefulExit(t *testing.T) {
	controller := &processControllerStub{}
	service := NewService(Dependencies{Processes: controller, ProcessWaiter: exitedWaiterStub{}})

	if err := service.Terminate(context.Background(), processdomain.Identity{PID: 42}); err != nil {
		t.Fatal(err)
	}
	if controller.terminated != 1 || controller.killed != 0 {
		t.Fatalf("terminate/kill calls = %d/%d; want 1/0", controller.terminated, controller.killed)
	}
}

func TestKillRecordsProcessIdentity(t *testing.T) {
	controller := &processControllerStub{}
	recorder := &auditRecorderStub{}
	service := NewService(Dependencies{Processes: controller, Audit: recorder})
	identity := processdomain.Identity{PID: 42, StartedAt: 1234}

	if err := service.Kill(context.Background(), identity); err != nil {
		t.Fatal(err)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("audit events = %d; want 1", len(recorder.events))
	}
	event := recorder.events[0]
	if event.Action != "kill" || event.PID != identity.PID || event.ProcessStartedAt != identity.StartedAt || !event.Success {
		t.Fatalf("unexpected kill audit event: %+v", event)
	}
}

func TestSuccessfulActionReportsAuditFailureSeparately(t *testing.T) {
	controller := &processControllerStub{}
	recorder := &auditRecorderStub{err: errors.New("disk full")}
	service := NewService(Dependencies{Processes: controller, Audit: recorder})

	err := service.Kill(context.Background(), processdomain.Identity{PID: 42})
	var actionErr *ActionError
	if !errors.As(err, &actionErr) || actionErr.Operation != nil || actionErr.Audit == nil {
		t.Fatalf("error = %#v; want audit-only ActionError", err)
	}
}

type stubProvider struct {
	executed string
}

func (s *stubProvider) Name() string { return "stub" }
func (s *stubProvider) Supports(processdomain.Info) bool { return true }
func (s *stubProvider) Detect(_ context.Context, _ processdomain.Info) []provider.ContextInfo {
	return []provider.ContextInfo{{Tag: processdomain.ContextDockerCompose, Label: "compose: demo/app"}}
}
func (s *stubProvider) Actions(_ context.Context, _ processdomain.Info) []provider.Action {
	return []provider.Action{{ID: "stub.restart", Label: "restart"}}
}
func (s *stubProvider) Execute(_ context.Context, actionID string, _ processdomain.Info) error {
	s.executed = actionID
	return nil
}

func TestServiceDetectsContextsAndMasksSensitiveArgs(t *testing.T) {
	p := &stubProvider{}
	registry := provider.NewRegistry(p)
	service := NewService(
		Dependencies{
			Snapshots: snapshotSourceStub{
				snapshot: Snapshot{
					Processes: []processdomain.Info{
						{
							Identity:    processdomain.Identity{PID: 100},
							Command:     "app",
							CommandLine: "app --password=secret123 --token=abc",
						},
					},
				},
			},
		},
		WithProviderRegistry(registry),
	)

	snapshot, err := service.Snapshot(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	proc := snapshot.Processes[0]
	if proc.CommandLine != "app --password=****** --token=******" {
		t.Fatalf("command line not masked: %s", proc.CommandLine)
	}
	if len(proc.Contexts) != 1 || proc.Contexts[0] != processdomain.ContextDockerCompose {
		t.Fatalf("contexts = %+v", proc.Contexts)
	}

	actions := service.ContextualActions(context.Background(), proc)
	if len(actions) != 1 || actions[0].ID != "stub.restart" {
		t.Fatalf("actions = %+v", actions)
	}

	if err := service.ExecuteContextualAction(context.Background(), "stub.restart", proc); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if p.executed != "stub.restart" {
		t.Fatalf("executed = %q; want 'stub.restart'", p.executed)
	}
}
