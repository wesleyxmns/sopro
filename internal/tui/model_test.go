package tui

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/wesleyxmns/sopro/internal/app"
	"github.com/wesleyxmns/sopro/internal/control"
	"github.com/wesleyxmns/sopro/internal/memory"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
	"github.com/wesleyxmns/sopro/internal/provider"
	"github.com/wesleyxmns/sopro/internal/updater"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
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

func TestSearchModeNavigatesFilteredResultsWithArrows(t *testing.T) {
	model, backend := newTestModel()
	model.applySnapshot(backend.snapshot)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("/")})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("proc")})
	model = updated.(Model)
	if len(model.Snapshot.Processes) != 3 {
		t.Fatalf("expected three filtered results, got %d", len(model.Snapshot.Processes))
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	if model.Cursor != 1 {
		t.Fatalf("down arrow cursor = %d, want 1", model.Cursor)
	}
	if !model.Searching || model.query.Search != "proc" {
		t.Fatal("arrow navigation changed the active search")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyUp})
	model = updated.(Model)
	if model.Cursor != 0 {
		t.Fatalf("up arrow cursor = %d, want 0", model.Cursor)
	}
}

func TestModelContextualActionRequests(t *testing.T) {
	model, backend := newTestModel()
	snapshot := backend.snapshot
	snapshot.Processes[0].Category = processdomain.CategoryContainer
	snapshot.Processes[0].ContainerName = "sangati_postgres"
	snapshot.Processes[0].Contexts = []processdomain.ContextTag{processdomain.ContextDockerCompose}
	model.applySnapshot(snapshot)

	// Press 'd' to stop docker container
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m := updated.(Model)
	if m.Pending == nil || m.Pending.Action != control.ActionDockerStop {
		t.Fatalf("expected ActionDockerStop, got %+v", m.Pending)
	}
	if !strings.Contains(m.Message, "sangati_postgres") {
		t.Fatalf("expected container name in message, got %q", m.Message)
	}

	// Cancel with esc
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.Pending != nil {
		t.Fatal("expected pending action to be canceled")
	}

	// Press 'z' to pause docker container
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("z")})
	m = updated.(Model)
	if m.Pending == nil || m.Pending.Action != control.ActionDockerPause {
		t.Fatalf("expected ActionDockerPause, got %+v", m.Pending)
	}

	// Pressing JVM shortcut 'j' on a container process must be ignored
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("j")})
	m = updated.(Model)
	if m.Pending != nil {
		t.Fatalf("expected 'j' (JVM GC) to be ignored on a container process, got pending: %+v", m.Pending)
	}

	// Test stopped container triggers ActionDockerStart on 's' and 'd'
	snapshot.Processes[0].State = processdomain.StateStopped
	m.applySnapshot(snapshot)

	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("s")})
	m = updated.(Model)
	if m.Pending == nil || m.Pending.Action != control.ActionDockerStart {
		t.Fatalf("expected ActionDockerStart on 's', got %+v", m.Pending)
	}
	if !strings.Contains(m.Message, "sangati_postgres") {
		t.Fatalf("expected container name in message, got %q", m.Message)
	}

	// Cancel and press 'd'
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("d")})
	m = updated.(Model)
	if m.Pending == nil || m.Pending.Action != control.ActionDockerStart {
		t.Fatalf("expected ActionDockerStart on 'd' for stopped container, got %+v", m.Pending)
	}
}

func TestUpdateCheckedMsg_SetsUpdateAvailable(t *testing.T) {
	m, _ := newTestModel()
	m.Width, m.Height = 120, 40
	release := &updater.ReleaseInfo{
		TagName: "v0.2.0",
		Version: "0.2.0",
	}
	updated, _ := m.Update(updateCheckedMsg{release: release, isNew: true})
	m = updated.(Model)
	if m.UpdateAvailable == nil {
		t.Fatal("expected UpdateAvailable to be set after updateCheckedMsg with isNew=true")
	}
	if m.UpdateAvailable.TagName != "v0.2.0" {
		t.Fatalf("expected TagName v0.2.0, got %s", m.UpdateAvailable.TagName)
	}
}

func TestUpdateCheckedMsg_NoUpdateDoesNothing(t *testing.T) {
	m, _ := newTestModel()
	m.Width, m.Height = 120, 40
	updated, _ := m.Update(updateCheckedMsg{release: nil, isNew: false})
	m = updated.(Model)
	if m.UpdateAvailable != nil {
		t.Fatal("expected UpdateAvailable to remain nil when no update available")
	}
}

func TestBackgroundUpdateCheckRevalidatesCachedResult(t *testing.T) {
	m, _ := newTestModel()
	cached := &updater.ReleaseInfo{TagName: "v0.2.3", Version: "0.2.3"}

	updated, _ := m.Update(updateCheckedMsg{release: cached, isNew: true, source: updateCheckCache})
	m = updated.(Model)
	if m.UpdateAvailable == nil {
		t.Fatal("expected cached update to be shown immediately")
	}

	updated, _ = m.Update(updateCheckedMsg{isNew: false, source: updateCheckBackground})
	m = updated.(Model)
	if m.CheckingUpdate || m.UpdateAvailable != nil {
		t.Fatal("background result did not replace stale cached state")
	}

	updated, _ = m.Update(updateCheckedMsg{release: cached, isNew: true, source: updateCheckCache})
	m = updated.(Model)
	if m.UpdateAvailable != nil {
		t.Fatal("late cache result replaced an already revalidated state")
	}
}

func TestUpdateKeyU_SetsPendingUpdate(t *testing.T) {
	m, _ := newTestModel()
	m.Width, m.Height = 120, 40
	m.ShowSplash = false
	m.UpdateAvailable = &updater.ReleaseInfo{TagName: "v0.2.0"}

	// Test uppercase U
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	m = updated.(Model)
	if m.PendingUpdate == nil {
		t.Fatal("expected PendingUpdate to be set after pressing U")
	}
	if !strings.Contains(m.Message, "v0.2.0") {
		t.Fatalf("expected message to contain version, got %q", m.Message)
	}

	// Cancel and test lowercase u
	m.PendingUpdate = nil
	m.Message = ""
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("u")})
	m = updated.(Model)
	if m.PendingUpdate == nil {
		t.Fatal("expected PendingUpdate to be set after pressing u (lowercase)")
	}
	if !strings.Contains(m.Message, "v0.2.0") {
		t.Fatalf("expected message to contain version, got %q", m.Message)
	}
}

func TestUpdateKeyUChecksAgainWithoutUpdateAvailable(t *testing.T) {
	m, _ := newTestModel()
	m.Width, m.Height = 120, 40
	m.ShowSplash = false

	updated, command := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("U")})
	m = updated.(Model)
	if m.PendingUpdate != nil {
		t.Fatal("expected PendingUpdate to remain nil when no update available")
	}
	if command == nil || !m.CheckingUpdate {
		t.Fatal("expected U to force a new update check")
	}
	if !strings.Contains(m.Message, "Verificando atualizações") {
		t.Fatalf("expected visible checking feedback, got %q", m.Message)
	}
}

func TestUpdatePendingCancel(t *testing.T) {
	m, _ := newTestModel()
	m.Width, m.Height = 120, 40
	m.ShowSplash = false
	m.PendingUpdate = &updater.ReleaseInfo{TagName: "v0.2.0"}

	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyEsc})
	m = updated.(Model)
	if m.PendingUpdate != nil {
		t.Fatal("expected PendingUpdate to be nil after pressing esc")
	}
	if !strings.Contains(m.Message, "cancelada") {
		t.Fatalf("expected cancellation message, got %q", m.Message)
	}
}

func TestUpdateAppliedMsg_Success(t *testing.T) {
	m, _ := newTestModel()
	m.Width, m.Height = 120, 40
	m.Acting = true
	release := &updater.ReleaseInfo{TagName: "v0.2.0"}
	m.UpdateAvailable = release

	updated, _ := m.Update(updateAppliedMsg{release: release, err: nil})
	m = updated.(Model)
	if m.UpdateAvailable != nil {
		t.Fatal("expected UpdateAvailable to be cleared after successful update")
	}
	if !strings.Contains(m.Message, "v0.2.0") || !strings.Contains(m.Message, "✔") {
		t.Fatalf("expected success message with version, got %q", m.Message)
	}
}

func TestUpdateAppliedMsg_Error(t *testing.T) {
	m, _ := newTestModel()
	m.Width, m.Height = 120, 40
	m.Acting = true
	release := &updater.ReleaseInfo{TagName: "v0.2.0"}
	m.UpdateAvailable = release

	updated, _ := m.Update(updateAppliedMsg{release: release, err: errors.New("permission denied")})
	m = updated.(Model)
	if !strings.Contains(m.Message, "permission denied") {
		t.Fatalf("expected error message, got %q", m.Message)
	}
}

func TestUpdateAppliedMsg_PermissionErrorShowsAction(t *testing.T) {
	m, _ := newTestModel()
	release := &updater.ReleaseInfo{TagName: "v0.2.3"}

	updated, _ := m.Update(updateAppliedMsg{release: release, err: updater.ErrPermissionDenied})
	m = updated.(Model)
	if !strings.Contains(m.Message, "sudo sopro update") {
		t.Fatalf("expected actionable permission guidance, got %q", m.Message)
	}
}

func TestUpdateAppliedMsg_ShowsUpdatedBinaryPath(t *testing.T) {
	m, _ := newTestModel()
	m.Width, m.Height = 120, 40
	release := &updater.ReleaseInfo{TagName: "v0.4.1"}

	updated, _ := m.Update(updateAppliedMsg{release: release, path: "/usr/local/bin/sopro", err: nil})
	m = updated.(Model)
	if !strings.Contains(m.Message, "/usr/local/bin/sopro") {
		t.Fatalf("success message omits updated binary path: %q", m.Message)
	}
}

func TestSnapshotRefreshPreservesUpdaterMessages(t *testing.T) {
	tests := []struct {
		name    string
		message tea.Msg
		want    string
	}{
		{
			name:    "check failure",
			message: updateCheckedMsg{err: errors.New("offline"), source: updateCheckManual},
			want:    "offline",
		},
		{
			name:    "permission failure",
			message: updateAppliedMsg{release: &updater.ReleaseInfo{TagName: "v0.3.1"}, err: updater.ErrPermissionDenied},
			want:    "sudo sopro update",
		},
		{
			name:    "successful update",
			message: updateAppliedMsg{release: &updater.ReleaseInfo{TagName: "v0.3.1"}},
			want:    "Sopro atualizado para v0.3.1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m, backend := newTestModel()
			updated, _ := m.Update(tt.message)
			m = updated.(Model)

			updated, _ = m.Update(snapshotLoadedMsg{snapshot: backend.snapshot})
			m = updated.(Model)
			if !strings.Contains(m.Message, tt.want) {
				t.Fatalf("snapshot refresh cleared updater message: got %q, want %q", m.Message, tt.want)
			}
		})
	}
}

func TestSuccessfulUpdateSchedulesRestart(t *testing.T) {
	model, _ := newTestModel()
	updated, command := model.Update(updateAppliedMsg{release: &updater.ReleaseInfo{TagName: "v9.9.9"}})
	model = updated.(Model)
	if !model.RestartRequested {
		t.Fatal("successful update did not request a restart")
	}
	if !strings.Contains(model.Message, "Reiniciando") {
		t.Fatalf("message = %q; want restart feedback", model.Message)
	}
	if command == nil {
		t.Fatal("successful update did not schedule the restart delay")
	}
}

func TestKeysAreIgnoredWhileRestartIsPending(t *testing.T) {
	model, _ := newTestModel()
	model.RestartRequested = true
	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	model = updated.(Model)
	if model.Pending != nil {
		t.Fatalf("keys must be ignored while restart is pending, got %+v", model.Pending)
	}
}

func TestBrowserKeyPreviewsBlankTabs(t *testing.T) {
	model, backend := newTestModel()
	snapshot := backend.snapshot
	snapshot.Processes[0].Category = processdomain.CategoryBrowser
	snapshot.Processes[0].Command = "chrome"
	snapshot.Processes[0].CommandLine = "chrome --remote-debugging-port=9222"
	model.applySnapshot(snapshot)

	updated, command := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("b")})
	model = updated.(Model)
	if model.Pending == nil || model.Pending.Action != control.ActionCDPCloseBlank {
		t.Fatalf("expected pending close_blank, got %+v", model.Pending)
	}
	if command == nil {
		t.Fatal("expected async tab preview command")
	}

	identity := snapshot.Processes[0].Identity
	tabs := []provider.BlankTab{
		{Title: "New Tab"}, {URL: "about:blank"}, {Title: "Blank"}, {Title: "Empty"}, {Title: "Void"},
	}
	updated, _ = model.Update(blankTabsLoadedMsg{proc: identity, tabs: tabs})
	model = updated.(Model)
	if len(model.PendingTabs) != 5 {
		t.Fatalf("pending tabs = %d; want 5", len(model.PendingTabs))
	}
	view := model.View()
	for _, expected := range []string{"Serão fechadas (5)", "New Tab", "about:blank", "+2 outras"} {
		if !strings.Contains(view, expected) {
			t.Fatalf("confirmation modal omitted %q", expected)
		}
	}

	updated, _ = model.Update(blankTabsLoadedMsg{proc: processdomain.Identity{PID: 999}, tabs: []provider.BlankTab{{Title: "Stale"}}})
	model = updated.(Model)
	if len(model.PendingTabs) != 5 {
		t.Fatal("stale preview overwrote current pending tabs")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	if model.PendingTabs != nil {
		t.Fatal("cancel did not clear pending tabs")
	}
}

func TestContextualResultMessageIncludesReclaimedBytes(t *testing.T) {
	model, _ := newTestModel()
	updated, _ := model.Update(actionFinishedMsg{
		result: control.Result{Action: control.ActionJVMRunGC, Process: processdomain.Identity{PID: 7}, Reclaimed: 2048},
	})
	model = updated.(Model)
	if !strings.Contains(model.Message, "2.00 KB") || !strings.Contains(model.Message, "recuperados") {
		t.Fatalf("message = %q; want reclaimed bytes", model.Message)
	}
}

func TestElevationHintMatchesPlatform(t *testing.T) {
	if got := elevationHint("linux"); got != "execute sudo sopro update" {
		t.Fatalf("linux hint = %q", got)
	}
	if got := elevationHint("windows"); !strings.Contains(got, "Administrador") || strings.Contains(got, "sudo") {
		t.Fatalf("windows hint = %q; must not mention sudo", got)
	}
}

func TestRestartMessageQuitsTUI(t *testing.T) {
	model, _ := newTestModel()
	_, command := model.Update(restartMsg{})
	if command == nil {
		t.Fatal("restart message returned no command")
	}
	if _, ok := command().(tea.QuitMsg); !ok {
		t.Fatal("restart message did not quit the TUI")
	}
}

func TestSnapshotRecoveryClearsOnlySnapshotFailure(t *testing.T) {
	m, backend := newTestModel()
	updated, _ := m.Update(snapshotLoadedMsg{err: errors.New("offline")})
	m = updated.(Model)
	if !strings.Contains(m.Message, "Falha ao atualizar: offline") {
		t.Fatalf("expected snapshot failure, got %q", m.Message)
	}

	updated, _ = m.Update(snapshotLoadedMsg{snapshot: backend.snapshot})
	m = updated.(Model)
	if m.Message != "" {
		t.Fatalf("snapshot recovery kept stale failure: %q", m.Message)
	}

	m.Message = "Ação encerrar concluída no PID 100"
	updated, _ = m.Update(snapshotLoadedMsg{snapshot: backend.snapshot})
	m = updated.(Model)
	if m.Message != "" {
		t.Fatalf("snapshot refresh changed cleanup for ordinary status: %q", m.Message)
	}
}

func cacheFixtureSnapshot() app.Snapshot {
	return app.Snapshot{
		Memory: memory.Snapshot{Reclaimable: 2 * 1024 * 1024 * 1024},
		Processes: []processdomain.Info{
			{
				Identity:    processdomain.Identity{PID: 7001, StartedAt: 1},
				Command:     "java",
				CommandLine: "java -Xmx4g -jar app.jar",
				Category:    processdomain.CategoryJVM,
				State:       processdomain.StateRunning,
			},
			{
				Identity:    processdomain.Identity{PID: 7002, StartedAt: 2},
				Command:     "chrome",
				CommandLine: "chrome --remote-debugging-port=9222",
				Category:    processdomain.CategoryBrowser,
				State:       processdomain.StateRunning,
			},
			{
				Identity: processdomain.Identity{PID: 7003, StartedAt: 3},
				Command:  "bash",
				Category: processdomain.CategorySystem,
				State:    processdomain.StateRunning,
			},
		},
	}
}

func newCacheTestModel(snapshot app.Snapshot) Model {
	backend := &fakeBackend{snapshot: snapshot}
	service := app.NewService(app.Dependencies{
		Snapshots: backend, Processes: backend, Cache: backend, Capabilities: backend,
	}, app.WithProviderRegistry(provider.NewRegistry(provider.NewJVMProvider(), provider.NewCDPProvider())))
	model := NewModel(service)
	updated, _ := model.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	model = updated.(Model)
	model.ShowSplash = false
	model.applySnapshot(snapshot)
	return model
}

func TestModelCleanServiceKeyResolvesSelectedTarget(t *testing.T) {
	model := newCacheTestModel(cacheFixtureSnapshot())

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	model = updated.(Model)
	if model.Pending == nil || model.Pending.Action != control.ActionJVMRunGC {
		t.Fatalf("expected pending JVM GC, got %+v", model.Pending)
	}
	if model.Pending.Target.PID != 7001 {
		t.Fatalf("pending target PID = %d; want 7001", model.Pending.Target.PID)
	}
	if view := model.View(); !strings.Contains(view, "FORÇAR GC NA JVM") || !strings.Contains(view, "PID 7001") {
		t.Fatal("confirmation modal did not preview the JVM cleanup")
	}

	// Move to the browser and resolve its CDP target.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	model = updated.(Model)
	if model.Pending == nil || model.Pending.Action != control.ActionCDPCloseBlank {
		t.Fatalf("expected pending CDP close_blank, got %+v", model.Pending)
	}

	// Plain processes have no cleanable cache.
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("c")})
	model = updated.(Model)
	if model.Pending != nil {
		t.Fatalf("expected no pending action for bash, got %+v", model.Pending)
	}
	if !strings.Contains(model.Message, "Nenhum cache limpável") {
		t.Fatalf("message = %q; want no-cleanable-cache feedback", model.Message)
	}
}

func TestModelCleanAllKeyPreviewsEveryTarget(t *testing.T) {
	model := newCacheTestModel(cacheFixtureSnapshot())

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("T")})
	model = updated.(Model)
	if model.Pending == nil || model.Pending.Action != control.ActionCleanAll {
		t.Fatalf("expected pending clean-all, got %+v", model.Pending)
	}
	if len(model.Pending.Targets) != 3 {
		t.Fatalf("targets = %+v; want OS + browser + JVM", model.Pending.Targets)
	}
	view := model.View()
	for _, expected := range []string{
		"LIMPEZA TOTAL DE CACHE",
		"• SO: Page cache, dentries e inodes · ≈ 2.00 GB",
		"Navegador",
		"JVM",
		"[ enter / y ] CONFIRMAR",
	} {
		if !strings.Contains(view, expected) {
			t.Fatalf("clean-all modal omitted %q", expected)
		}
	}
}

func TestModelCleanAllModalTruncatesLongTargetLists(t *testing.T) {
	snapshot := cacheFixtureSnapshot()
	for pid := int32(8000); pid < 8008; pid++ {
		snapshot.Processes = append(snapshot.Processes, processdomain.Info{
			Identity: processdomain.Identity{PID: pid},
			Command:  "java",
			Category: processdomain.CategoryJVM,
			State:    processdomain.StateRunning,
		})
	}
	model := newCacheTestModel(snapshot)
	model.Width, model.Height = 60, 30

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("t")})
	model = updated.(Model)
	if model.Pending == nil || len(model.Pending.Targets) != 11 {
		t.Fatalf("targets = %d; want 11 (OS + browser + 9 JVMs)", len(model.Pending.Targets))
	}
	view := model.View()
	if !strings.Contains(view, "+5 outro(s)") {
		t.Fatal("clean-all modal did not collapse overflow targets")
	}
	for lineNumber, line := range strings.Split(view, "\n") {
		if got := lipgloss.Width(line); got > model.Width {
			t.Fatalf("line %d width %d exceeds %d: %q", lineNumber+1, got, model.Width, line)
		}
	}
}

func TestModelCleanAllResultMessage(t *testing.T) {
	model := newCacheTestModel(cacheFixtureSnapshot())
	updated, _ := model.Update(actionFinishedMsg{
		result: control.Result{Action: control.ActionCleanAll, Reclaimed: 2048, Succeeded: 2, Failed: 1},
	})
	model = updated.(Model)
	if !strings.Contains(model.Message, "Limpeza total concluída") ||
		!strings.Contains(model.Message, "2 ok") ||
		!strings.Contains(model.Message, "1 falha") {
		t.Fatalf("message = %q", model.Message)
	}
	if !model.Loading {
		t.Fatal("clean-all completion did not refresh the snapshot")
	}
}

type recordingProvider struct {
	got processdomain.Info
}

func (p *recordingProvider) Name() string                     { return "rec" }
func (p *recordingProvider) Supports(processdomain.Info) bool { return true }
func (p *recordingProvider) Detect(context.Context, processdomain.Info) []provider.ContextInfo {
	return nil
}
func (p *recordingProvider) Actions(context.Context, processdomain.Info) []provider.Action {
	return []provider.Action{{ID: "rec.act"}}
}
func (p *recordingProvider) Execute(_ context.Context, _ string, proc processdomain.Info) (uint64, error) {
	p.got = proc
	return 0, nil
}

func TestModelGitActionRequests(t *testing.T) {
	model, backend := newTestModel()
	snapshot := backend.snapshot
	snapshot.Processes[0].Category = processdomain.CategoryDevelopment
	snapshot.Processes[0].Command = "node"
	snapshot.Processes[0].Cwd = "/tmp/sopro-demo"
	model.applySnapshot(snapshot)

	updated, _ := model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	model = updated.(Model)
	if model.Pending == nil || model.Pending.Action != control.ActionGitStatus {
		t.Fatalf("expected pending git.status, got %+v", model.Pending)
	}
	if view := model.View(); !strings.Contains(view, "GIT STATUS") {
		t.Fatal("confirmation modal did not preview git status")
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("v")})
	model = updated.(Model)
	if model.Pending == nil || model.Pending.Action != control.ActionGitFetch {
		t.Fatalf("expected pending git.fetch, got %+v", model.Pending)
	}

	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyEsc})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyDown})
	model = updated.(Model)
	updated, _ = model.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("w")})
	model = updated.(Model)
	if model.Pending != nil {
		t.Fatalf("expected 'w' to be ignored on a non-dev process, got %+v", model.Pending)
	}
}

func TestExecuteActionCmdPassesFullTargetToProviders(t *testing.T) {
	recorder := &recordingProvider{}
	backend := &fakeBackend{snapshot: testSnapshot()}
	service := app.NewService(app.Dependencies{
		Snapshots: backend, Processes: backend, Cache: backend, Capabilities: backend,
	}, app.WithProviderRegistry(provider.NewRegistry(recorder)))
	selected := processdomain.Info{
		Identity:    processdomain.Identity{PID: 7002, StartedAt: 2},
		Command:     "chrome",
		CommandLine: "chrome --remote-debugging-port=9222",
		Category:    processdomain.CategoryBrowser,
	}

	message := executeActionCmd(service, control.Request{
		Action: control.Action("rec.act"), Process: selected.Identity, Target: selected,
	})()
	finished, ok := message.(actionFinishedMsg)
	if !ok || finished.err != nil {
		t.Fatalf("message = %#v", message)
	}
	if recorder.got.CommandLine != selected.CommandLine || recorder.got.Category != selected.Category {
		t.Fatalf("provider got %+v; want the full selected process", recorder.got)
	}
}
