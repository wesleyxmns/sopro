package app

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/wesleyxmns/sopro/internal/audit"
	"github.com/wesleyxmns/sopro/internal/memory"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
	"github.com/wesleyxmns/sopro/internal/provider"
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
	executed  string
	reclaimed uint64
}

func (s *stubProvider) Name() string                     { return "stub" }
func (s *stubProvider) Supports(processdomain.Info) bool { return true }
func (s *stubProvider) Detect(_ context.Context, _ processdomain.Info) []provider.ContextInfo {
	return []provider.ContextInfo{{Tag: processdomain.ContextDockerCompose, Label: "compose: demo/app"}}
}
func (s *stubProvider) Actions(_ context.Context, _ processdomain.Info) []provider.Action {
	return []provider.Action{{ID: "stub.restart", Label: "restart"}}
}
func (s *stubProvider) Execute(_ context.Context, actionID string, _ processdomain.Info) (uint64, error) {
	s.executed = actionID
	return s.reclaimed, nil
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

	if _, err := service.ExecuteContextualAction(context.Background(), "stub.restart", proc); err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if p.executed != "stub.restart" {
		t.Fatalf("executed = %q; want 'stub.restart'", p.executed)
	}

	// Executing incompatible action must be rejected with ErrIncompatibleAction
	_, err = service.ExecuteContextualAction(context.Background(), "docker.stop", proc)
	if !errors.Is(err, provider.ErrIncompatibleAction) {
		t.Fatalf("expected ErrIncompatibleAction, got %v", err)
	}
}

type containerStubProvider struct{}

func (c *containerStubProvider) Name() string                     { return "docker-stub" }
func (c *containerStubProvider) Supports(processdomain.Info) bool { return true }
func (c *containerStubProvider) Detect(_ context.Context, proc processdomain.Info) []provider.ContextInfo {
	if proc.PID == 200 {
		return []provider.ContextInfo{
			{
				Tag:   processdomain.ContextDockerCompose,
				Label: "compose: sangati/postgres",
				Details: map[string]string{
					"container_name":  "sangati_postgres",
					"container_id":    "acf8947e8c35",
					"image":           "postgres:15-alpine",
					"compose_project": "sangati",
					"compose_service": "postgres",
				},
			},
		}
	}
	return nil
}
func (c *containerStubProvider) Actions(_ context.Context, _ processdomain.Info) []provider.Action {
	return nil
}
func (c *containerStubProvider) Execute(_ context.Context, _ string, _ processdomain.Info) (uint64, error) {
	return 0, nil
}

func TestExecuteContextualActionReportsReclaimedBytes(t *testing.T) {
	stub := &stubProvider{reclaimed: 2048}
	recorder := &auditRecorderStub{}
	service := NewService(
		Dependencies{Audit: recorder},
		WithProviderRegistry(provider.NewRegistry(stub)),
	)
	proc := processdomain.Info{Identity: processdomain.Identity{PID: 7}}

	reclaimed, err := service.ExecuteContextualAction(context.Background(), "stub.restart", proc)
	if err != nil {
		t.Fatal(err)
	}
	if reclaimed != 2048 {
		t.Fatalf("reclaimed = %d; want 2048", reclaimed)
	}
	if len(recorder.events) != 1 || recorder.events[0].ReclaimedBytes != 2048 {
		t.Fatalf("audit events = %+v; want reclaimed bytes recorded", recorder.events)
	}
}

func TestPreviewBlankTabsRejectsMissingProviders(t *testing.T) {
	var nilService *Service
	if _, err := nilService.PreviewBlankTabs(context.Background(), processdomain.Info{}); err != ErrUnsupported {
		t.Fatalf("err = %v; want ErrUnsupported", err)
	}
	service := NewService(Dependencies{})
	if _, err := service.PreviewBlankTabs(context.Background(), processdomain.Info{}); err != ErrUnsupported {
		t.Fatalf("err = %v; want ErrUnsupported", err)
	}
}

func TestServiceEnrichesContainerEntitiesInSnapshot(t *testing.T) {
	registry := provider.NewRegistry(&containerStubProvider{})
	service := NewService(
		Dependencies{
			Snapshots: snapshotSourceStub{
				snapshot: Snapshot{
					Processes: []processdomain.Info{
						{
							Identity: processdomain.Identity{PID: 200},
							Command:  "postgres",
							Category: processdomain.CategoryDatabase, // Initially categorized as database
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
	if proc.Category != processdomain.CategoryContainer {
		t.Fatalf("Category = %q; want %q", proc.Category, processdomain.CategoryContainer)
	}
	if proc.ContainerName != "sangati_postgres" {
		t.Fatalf("ContainerName = %q; want 'sangati_postgres'", proc.ContainerName)
	}
	if proc.ContainerID != "acf8947e8c35" {
		t.Fatalf("ContainerID = %q; want 'acf8947e8c35'", proc.ContainerID)
	}
	if proc.ImageName != "postgres:15-alpine" {
		t.Fatalf("ImageName = %q; want 'postgres:15-alpine'", proc.ImageName)
	}
}

type entitySourceStubProvider struct {
	containerStubProvider
}

func (e *entitySourceStubProvider) DiscoverEntities(_ context.Context) []processdomain.Info {
	return []processdomain.Info{
		{
			Identity:      processdomain.Identity{PID: 0},
			Command:       "stopped_redis",
			ContainerName: "stopped_redis",
			ImageName:     "redis:7-alpine",
			Category:      processdomain.CategoryContainer,
			State:         processdomain.StateStopped,
		},
	}
}

func TestServiceInjectsStoppedContainersInSnapshot(t *testing.T) {
	registry := provider.NewRegistry(&entitySourceStubProvider{})
	service := NewService(
		Dependencies{
			Snapshots: snapshotSourceStub{
				snapshot: Snapshot{
					Processes: []processdomain.Info{
						{
							Identity: processdomain.Identity{PID: 100},
							Command:  "bash",
							Category: processdomain.CategorySystem,
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
	if len(snapshot.Processes) != 2 {
		t.Fatalf("expected 2 processes, got %d", len(snapshot.Processes))
	}
	stopped := snapshot.Processes[1]
	if stopped.State != processdomain.StateStopped {
		t.Fatalf("stopped.State = %v; want StateStopped", stopped.State)
	}
	if stopped.ContainerName != "stopped_redis" {
		t.Fatalf("stopped.ContainerName = %q; want 'stopped_redis'", stopped.ContainerName)
	}
}

type capabilityStub struct {
	canCleanCache bool
}

func (stub capabilityStub) Capabilities() Capabilities {
	return Capabilities{Platform: "test", CanCleanCache: stub.canCleanCache}
}

type cacheCleanerStub struct {
	reclaimed uint64
	err       error
	calls     int
}

func (stub *cacheCleanerStub) CleanCache(context.Context) (uint64, error) {
	stub.calls++
	return stub.reclaimed, stub.err
}

type cacheStubProvider struct {
	stubProvider
	fail map[string]error
}

func (c *cacheStubProvider) CacheTargets(proc processdomain.Info) []provider.CacheTarget {
	return []provider.CacheTarget{{
		Source: "Stub", ActionID: "stub.restart",
		Label: "stub cache", Detail: "shared",
		Proc: proc,
	}}
}

func (c *cacheStubProvider) Execute(_ context.Context, actionID string, _ processdomain.Info) (uint64, error) {
	c.executed = actionID
	if err, ok := c.fail[actionID]; ok {
		return 0, err
	}
	return 0, nil
}

func cleanAllSnapshot() Snapshot {
	return Snapshot{
		Memory: memory.Snapshot{Reclaimable: 4 * 1024 * 1024 * 1024},
		Processes: []processdomain.Info{
			{
				Identity:    processdomain.Identity{PID: 101},
				Command:     "chrome",
				CommandLine: "chrome --remote-debugging-port=9222",
				Category:    processdomain.CategoryBrowser,
			},
			{
				Identity:    processdomain.Identity{PID: 102},
				Command:     "chrome",
				CommandLine: "chrome --remote-debugging-port=9222 --type=renderer",
				Category:    processdomain.CategoryBrowser,
			},
			{
				Identity: processdomain.Identity{PID: 202},
				Command:  "java",
				Category: processdomain.CategoryJVM,
			},
			{
				Identity: processdomain.Identity{PID: 303},
				Command:  "bash",
				Category: processdomain.CategorySystem,
			},
		},
	}
}

func TestPreviewCleanAllListsOSFirstAndDedupesSharedTargets(t *testing.T) {
	service := NewService(
		Dependencies{Capabilities: capabilityStub{canCleanCache: true}},
		WithProviderRegistry(provider.NewRegistry(provider.NewJVMProvider(), provider.NewCDPProvider())),
	)

	targets := service.PreviewCleanAll(cleanAllSnapshot())
	if len(targets) != 3 {
		t.Fatalf("targets = %+v; want 3 (OS + browser + JVM)", targets)
	}
	if targets[0].ActionID != "clean-cache" || targets[0].Source != "SO" {
		t.Fatalf("first target = %+v; want the OS page cache", targets[0])
	}
	if targets[0].Detail != "≈ 4.00 GB" {
		t.Fatalf("first target detail = %q; want the reclaimable bytes", targets[0].Detail)
	}
	if targets[1].ActionID != "cdp.close_blank" {
		t.Fatalf("second target = %+v; want the deduplicated browser target", targets[1])
	}
	if targets[2].ActionID != "jvm.run_gc" || targets[2].Proc.PID != 202 {
		t.Fatalf("third target = %+v; want the JVM target for PID 202", targets[2])
	}
}

func TestPreviewCleanAllSkipsOSWithoutPrivilegeOrReclaimable(t *testing.T) {
	registry := provider.NewRegistry(provider.NewJVMProvider())

	unprivileged := NewService(
		Dependencies{Capabilities: capabilityStub{canCleanCache: false}},
		WithProviderRegistry(registry),
	)
	if targets := unprivileged.PreviewCleanAll(cleanAllSnapshot()); len(targets) != 1 || targets[0].ActionID != "jvm.run_gc" {
		t.Fatalf("unprivileged targets = %+v; want only the JVM target", targets)
	}

	privileged := NewService(
		Dependencies{Capabilities: capabilityStub{canCleanCache: true}},
		WithProviderRegistry(provider.NewRegistry()),
	)
	empty := cleanAllSnapshot()
	empty.Memory.Reclaimable = 0
	empty.Processes = empty.Processes[3:]
	if targets := privileged.PreviewCleanAll(empty); len(targets) != 0 {
		t.Fatalf("targets = %+v; want none without reclaimable bytes or providers", targets)
	}
}

func TestCleanTargetsRoutesOSAndProviders(t *testing.T) {
	cache := &cacheCleanerStub{reclaimed: 4096}
	stub := &cacheStubProvider{}
	recorder := &auditRecorderStub{}
	service := NewService(
		Dependencies{Cache: cache, Capabilities: capabilityStub{canCleanCache: true}, Audit: recorder},
		WithProviderRegistry(provider.NewRegistry(stub)),
	)
	proc := processdomain.Info{Identity: processdomain.Identity{PID: 9}}

	result, err := service.CleanTargets(context.Background(), []provider.CacheTarget{
		{Source: "Sistema operacional", ActionID: "clean-cache", Label: "page cache"},
		{Source: "Stub", ActionID: "stub.restart", Label: "stub cache", Proc: proc},
	})
	if err != nil {
		t.Fatalf("clean failed: %v", err)
	}
	if result.ReclaimedBytes != 4096 || result.Succeeded != 2 || result.Failed != 0 {
		t.Fatalf("result = %+v; want 4096 bytes and 2 successes", result)
	}
	if cache.calls != 1 || stub.executed != "stub.restart" {
		t.Fatalf("cache calls = %d, executed = %q", cache.calls, stub.executed)
	}
	if len(recorder.events) != 1 {
		t.Fatalf("audit events = %d; want 1", len(recorder.events))
	}
	event := recorder.events[0]
	if event.Action != "clean-cache-all" || !event.Success || event.ReclaimedBytes != 4096 {
		t.Fatalf("unexpected audit event: %+v", event)
	}
}

func TestCleanTargetsReportsPartialAndTotalFailures(t *testing.T) {
	newService := func(cacheErr, providerErr error) (*Service, *auditRecorderStub) {
		recorder := &auditRecorderStub{}
		service := NewService(
			Dependencies{
				Cache:        &cacheCleanerStub{reclaimed: 128, err: cacheErr},
				Capabilities: capabilityStub{canCleanCache: true},
				Audit:        recorder,
			},
			WithProviderRegistry(provider.NewRegistry(&cacheStubProvider{fail: map[string]error{"stub.restart": providerErr}})),
		)
		return service, recorder
	}
	targets := []provider.CacheTarget{
		{Source: "Sistema operacional", ActionID: "clean-cache", Label: "page cache"},
		{Source: "Stub", ActionID: "stub.restart", Label: "stub cache", Proc: processdomain.Info{Identity: processdomain.Identity{PID: 9}}},
	}

	partial, partialRecorder := newService(nil, errors.New("boom"))
	result, err := partial.CleanTargets(context.Background(), targets)
	if err != nil {
		t.Fatalf("partial cleanup must not fail, got %v", err)
	}
	if result.Succeeded != 1 || result.Failed != 1 || result.ReclaimedBytes != 128 {
		t.Fatalf("partial result = %+v", result)
	}
	if len(partialRecorder.events) != 1 || !partialRecorder.events[0].Success {
		t.Fatalf("partial audit events = %+v; want one success", partialRecorder.events)
	}

	total, totalRecorder := newService(errors.New("denied"), errors.New("boom"))
	result, err = total.CleanTargets(context.Background(), targets)
	if err == nil {
		t.Fatal("total failure must return an error")
	}
	if result.Succeeded != 0 || result.Failed != 2 {
		t.Fatalf("total result = %+v", result)
	}
	if len(totalRecorder.events) != 1 || totalRecorder.events[0].Success {
		t.Fatalf("total audit events = %+v; want one failure", totalRecorder.events)
	}

	if _, err := total.CleanTargets(context.Background(), nil); err == nil {
		t.Fatal("empty targets must return an error")
	}
}
