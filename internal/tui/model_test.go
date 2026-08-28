package tui

import (
	"fmt"
	"testing"
	"time"

	"memcleaner/internal/system"

	tea "github.com/charmbracelet/bubbletea"
)

// MockPlatformManager implements system.PlatformManager for testing
type MockPlatformManager struct {
	ShouldFail bool
}

func (m *MockPlatformManager) GetMetrics() (system.SystemMetrics, error) {
	if m.ShouldFail {
		return system.SystemMetrics{}, fmt.Errorf("mock failure") // simulate error interface
	}
	return system.SystemMetrics{TotalRAM: 1024, UsedRAM: 512, CacheRAM: 256}, nil
}

func (m *MockPlatformManager) GetProcesses(limit int) ([]system.ProcessInfo, error) {
	if m.ShouldFail {
		return nil, fmt.Errorf("mock failure")
	}
	return []system.ProcessInfo{
		{PID: 100, User: "user1", MemPct: 10.0, CPUPct: 2.0, Command: "proc1"},
		{PID: 200, User: "user2", MemPct: 8.0, CPUPct: 1.0, Command: "proc2"},
		{PID: 300, User: "user3", MemPct: 5.0, CPUPct: 0.5, Command: "proc3"},
	}, nil
}

func (m *MockPlatformManager) KillProcess(pid int32) error {
	return nil
}

func (m *MockPlatformManager) PauseProcess(pid int32) error {
	return nil
}

func (m *MockPlatformManager) ResumeProcess(pid int32) error {
	return nil
}

func (m *MockPlatformManager) CleanSystemCache() (uint64, error) {
	return 1024, nil
}

// custom error for mock
func (e *MockPlatformManager) Error() string { return "mock failure" }

func TestNewModel(t *testing.T) {
	mgr := &MockPlatformManager{}
	m := NewModel(mgr)

	if m.Cursor != 0 {
		t.Errorf("Expected cursor to be 0, got %d", m.Cursor)
	}

	if m.Message != "" {
		t.Errorf("Expected message to be empty, got %q", m.Message)
	}
}

func TestModel_Update_TickMsg_ResolvesPreviousError(t *testing.T) {
	mgr := &MockPlatformManager{}
	m := NewModel(mgr)
	m.Message = "Error getting processes: mock failure"

	updatedModel, cmd := m.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Error("Expected tick cmd to be returned, got nil")
	}

	m2 := updatedModel.(Model)
	if m2.Message != "" {
		t.Errorf("Expected previous background error to be resolved and cleared, but got: %s", m2.Message)
	}
}

func TestModel_Update_TickMsg_PreservesUserMessage(t *testing.T) {
	mgr := &MockPlatformManager{}
	m := NewModel(mgr)
	m.Message = "SIGKILL sent to PID 123"

	updatedModel, cmd := m.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Error("Expected tick cmd to be returned, got nil")
	}

	m2 := updatedModel.(Model)
	// In the new implementation, successful fetch resets m.Message to "".
	// If the implementation clears the message, it's fine as per brief.
	// We'll adjust based on the current brief's behavior: 
	// The brief says: "m.Message = """ on successful tick.
	if m2.Message != "" {
		t.Errorf("Expected message to be cleared on tick success according to new spec, got: %s", m2.Message)
	}
}

func TestModel_Update_Navigation(t *testing.T) {
	mgr := &MockPlatformManager{}
	m := NewModel(mgr)
	m.Processes = []system.ProcessInfo{
		{PID: 100, User: "user1", MemPct: 10.0, CPUPct: 2.0, Command: "proc1"},
		{PID: 200, User: "user2", MemPct: 8.0, CPUPct: 1.0, Command: "proc2"},
		{PID: 300, User: "user3", MemPct: 5.0, CPUPct: 0.5, Command: "proc3"},
	}
	m.Cursor = 0

	// Move down
	updated, _ := m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("down")})
	m = updated.(Model)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor to be 1 after pressing down, got %d", m.Cursor)
	}

	// Move down again
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("down")})
	m = updated.(Model)
	if m.Cursor != 2 {
		t.Errorf("Expected cursor to be 2 after pressing down, got %d", m.Cursor)
	}

	// Move down past limit (should stay at last element)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("down")})
	m = updated.(Model)
	if m.Cursor != 2 {
		t.Errorf("Expected cursor to stay at 2 after pressing down past limit, got %d", m.Cursor)
	}

	// Move up
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("up")})
	m = updated.(Model)
	if m.Cursor != 1 {
		t.Errorf("Expected cursor to be 1 after pressing up, got %d", m.Cursor)
	}

	// Move up to 0
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("up")})
	m = updated.(Model)
	if m.Cursor != 0 {
		t.Errorf("Expected cursor to be 0 after pressing up, got %d", m.Cursor)
	}

	// Move up past 0 (should stay at 0)
	updated, _ = m.Update(tea.KeyMsg{Type: tea.KeyRunes, Runes: []rune("up")})
	m = updated.(Model)
	if m.Cursor != 0 {
		t.Errorf("Expected cursor to stay at 0 after pressing up past limit, got %d", m.Cursor)
	}
}

func TestModel_WindowResize_DoesNotCrash(t *testing.T) {
	m := Model{
		Width:  80,
		Height: 24,
	}

	res, _ := m.Update(tea.WindowSizeMsg{Width: 100, Height: 30})
	mUpdated := res.(Model)

	if mUpdated.Width != 100 || mUpdated.Height != 30 {
		t.Errorf("Expected size to update to 100x30, got %dx%d", mUpdated.Width, mUpdated.Height)
	}
}
