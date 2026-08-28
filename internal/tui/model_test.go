package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"sopro/internal/app"
	"sopro/internal/control"
	"sopro/internal/memory"
	processdomain "sopro/internal/process"

	tea "github.com/charmbracelet/bubbletea"
)

type fakeBackend struct {
	snapshot app.Snapshot
	err      error
	actions  []control.Request
}

func (f *fakeBackend) Snapshot(context.Context, int) (app.Snapshot, error) {
	return f.snapshot, f.err
}

func (f *fakeBackend) record(action control.Action, id processdomain.Identity) error {
	f.actions = append(f.actions, control.Request{Action: action, Process: id})
	return f.err
}

func (f *fakeBackend) Terminate(_ context.Context, id processdomain.Identity) error {
	return f.record(control.ActionTerminate, id)
}

func (f *fakeBackend) Kill(_ context.Context, id processdomain.Identity) error {
	return f.record(control.ActionKill, id)
}

func (f *fakeBackend) Pause(_ context.Context, id processdomain.Identity) error {
	return f.record(control.ActionPause, id)
}

func (f *fakeBackend) Resume(_ context.Context, id processdomain.Identity) error {
	return f.record(control.ActionResume, id)
}

func (f *fakeBackend) CleanCache(context.Context) (uint64, error) {
	f.actions = append(f.actions, control.Request{Action: control.ActionClean})
	return 1024, f.err
}

func (f *fakeBackend) Capabilities() app.Capabilities {
	return app.Capabilities{
		Platform: "test", Elevated: true, CanTerminate: true, CanKill: true,
		CanPause: true, CanResume: true, CanCleanCache: true,
	}
}

func newTestModel() (Model, *fakeBackend) {
	backend := &fakeBackend{snapshot: testSnapshot()}
	service := app.NewService(app.Dependencies{
		Snapshots: backend, Processes: backend, Cache: backend, Capabilities: backend,
	})
	model := NewModel(service)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model = updated.(Model)
	model.ShowSplash = false
	return model, backend
}

func testSnapshot() app.Snapshot {
	return app.Snapshot{
		Memory: memory.Snapshot{
			Total: 1024 * 1024 * 1024, Used: 512 * 1024 * 1024,
			Available: 512 * 1024 * 1024, Cache: 128 * 1024 * 1024,
		},
		Processes: []processdomain.Info{
			{Identity: processdomain.Identity{PID: 100, StartedAt: 1}, User: "user1", MemoryBytes: 1000, MemoryPct: 10, CPUPct: 2, Command: "proc1", State: processdomain.StateRunning, Risk: processdomain.RiskOK},
			{Identity: processdomain.Identity{PID: 200, StartedAt: 2}, User: "user2", MemoryBytes: 800, MemoryPct: 8, CPUPct: 1, Command: "proc2", State: processdomain.StatePaused, Risk: processdomain.RiskWarning},
			{Identity: processdomain.Identity{PID: 300, StartedAt: 3}, User: "user3", MemoryBytes: 500, MemoryPct: 5, CPUPct: .5, Command: "proc3", State: processdomain.StateRunning, Risk: processdomain.RiskOK},
		},
	}
}

func TestModelLoadsSnapshotAsynchronously(t *testing.T) {
	model, backend := newTestModel()
	if !model.Loading {
		t.Fatal("new model must start in loading state")
	}

	message := loadSnapshotCmd(model.service)()
	updated, _ := model.Update(message)
	model = updated.(Model)
	if model.Loading {
		t.Fatal("snapshot completion must clear loading")
	}
	if got := len(model.Snapshot.Processes); got != len(backend.snapshot.Processes) {
		t.Fatalf("got %d processes, want %d", got, len(backend.snapshot.Processes))
	}
}

func TestModelSnapshotErrorPreservesLastGoodData(t *testing.T) {
	model, backend := newTestModel()
	model.applySnapshot(backend.snapshot)
	backend.err = errors.New("offline")

	updated, _ := model.Update(loadSnapshotCmd(model.service)())
	model = updated.(Model)
	if len(model.Snapshot.Processes) != 3 {
		t.Fatal("last good snapshot was discarded")
	}
	if !strings.Contains(model.Message, "offline") {
		t.Fatalf("expected visible error, got %q", model.Message)
	}
}

func TestModelNavigationAndStableSelection(t *testing.T) {
	model, backend := newTestModel()
	model.applySnapshot(backend.snapshot)
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.Cursor != 1 {
		t.Fatalf("cursor = %d; want 1", model.Cursor)
	}

	reordered := backend.snapshot
	reordered.Processes = []processdomain.Info{
		backend.snapshot.Processes[1],
		backend.snapshot.Processes[0],
		backend.snapshot.Processes[2],
	}
	model.applySnapshot(reordered)
	if model.Cursor != 0 || model.Snapshot.Processes[model.Cursor].PID != 200 {
		t.Fatal("selection did not follow stable process identity")
	}
}

func TestModelKeepsOnlyThirtyMemorySamples(t *testing.T) {
	model, backend := newTestModel()
	for index := 0; index < 35; index++ {
		snapshot := backend.snapshot
		snapshot.Memory.Used = uint64(index)
		model.applySnapshot(snapshot)
	}
	if len(model.memoryHistory) != 30 {
		t.Fatalf("history length = %d; want 30", len(model.memoryHistory))
	}
	if model.memoryHistory[0].Used != 5 || model.memoryHistory[29].Used != 34 {
		t.Fatalf("unexpected history bounds: first=%d last=%d", model.memoryHistory[0].Used, model.memoryHistory[29].Used)
	}
}

func TestModelConfirmsAndExecutesKill(t *testing.T) {
	model, backend := newTestModel()
	model.applySnapshot(backend.snapshot)

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("k")})
	model = updated.(Model)
	if command != nil || model.Pending == nil {
		t.Fatal("kill must wait for confirmation")
	}

	updated, command = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if command == nil || !model.Acting {
		t.Fatal("confirmed kill must return an asynchronous command")
	}
	message := command()
	model.Update(message)
	if len(backend.actions) != 1 || backend.actions[0].Process != backend.snapshot.Processes[0].Identity {
		t.Fatal("kill did not use the selected stable identity")
	}
}

func TestModelWindowResizeAndLayouts(t *testing.T) {
	model, backend := newTestModel()
	model.applySnapshot(backend.snapshot)

	for _, size := range []tea.WindowSizeMsg{
		{Width: 60, Height: 20},
		{Width: 90, Height: 24},
		{Width: 120, Height: 30},
	} {
		updated, _ := model.Update(size)
		model = updated.(Model)
		view := model.View()
		if !strings.Contains(view, "SOPRO") || !strings.Contains(view, "Processos") {
			t.Fatalf("layout %dx%d omitted core content", size.Width, size.Height)
		}
	}
}

func TestModelDistinguishesAuditFailureAfterSuccessfulAction(t *testing.T) {
	model, _ := newTestModel()
	message := actionFinishedMsg{
		result: control.Result{Action: control.ActionKill, Process: processdomain.Identity{PID: 42}},
		err:    &app.ActionError{Audit: errors.New("disk full")},
	}

	updated, command := model.Update(message)
	model = updated.(Model)
	if !strings.Contains(model.Message, "Ação concluída, mas a auditoria falhou") {
		t.Fatalf("message = %q", model.Message)
	}
	if command == nil || !model.Loading {
		t.Fatal("successful action with audit failure did not refresh the snapshot")
	}
}

func TestModelInteractiveControlsSearchFilterSortGroup(t *testing.T) {
	model, backend := newTestModel()
	model.applySnapshot(backend.snapshot)

	// Test Search ('/')
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = updated.(Model)
	if !model.Searching {
		t.Fatal("search mode was not activated with '/'")
	}

	// Type runes in search
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("proc2")})
	model = updated.(Model)
	if model.query.Search != "proc2" || len(model.Snapshot.Processes) != 1 || model.Snapshot.Processes[0].PID != 200 {
		t.Fatalf("expected filtered to proc2, got %d processes", len(model.Snapshot.Processes))
	}

	// Exit search with enter
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEnter})
	model = updated.(Model)
	if model.Searching {
		t.Fatal("enter did not close search input mode")
	}

	// Clear search with '/' then Esc
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.query.Search != "" || len(model.Snapshot.Processes) != 3 {
		t.Fatal("esc did not clear search filter")
	}

	// Test Sort ('s')
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	model = updated.(Model)
	if model.query.Sort != processdomain.SortCPU {
		t.Fatalf("expected SortCPU, got %v", model.query.Sort)
	}

	// Test Group ('g')
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = updated.(Model)
	if model.groupMode != groupCategory {
		t.Fatalf("expected groupCategory, got %v", model.groupMode)
	}
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("g")})
	model = updated.(Model)
	if model.groupMode != groupTree {
		t.Fatalf("expected groupTree, got %v", model.groupMode)
	}
}

