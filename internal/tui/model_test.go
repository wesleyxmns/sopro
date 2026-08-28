package tui

import (
	"strings"
	"testing"
	"time"
	"memcleaner/internal/system"
	tea "github.com/charmbracelet/bubbletea"
)

func TestNewModel(t *testing.T) {
	m := NewModel()
	// IsRoot could be true or false depending on the environment, we just check it doesn't panic.
	_ = m.IsRoot
	
	if m.Cursor != 0 {
		t.Errorf("Expected cursor to be 0, got %d", m.Cursor)
	}
	
	// In environments like CI or headless nodes without /proc, NewModel may fail to fetch processes
	// and record an error message immediately. We allow this, but other messages are failures.
	if m.Message != "" && !strings.HasPrefix(m.Message, "Error getting ") {
		t.Errorf("Expected message to be empty or an environment reading error, got %q", m.Message)
	}
}

func TestModel_Update_TickMsg_ResolvesPreviousError(t *testing.T) {
	m := NewModel()
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
	m := NewModel()
	m.Message = "SIGKILL sent to PID 123"

	updatedModel, cmd := m.Update(tickMsg(time.Now()))
	if cmd == nil {
		t.Error("Expected tick cmd to be returned, got nil")
	}

	m2 := updatedModel.(Model)
	if m2.Message != "SIGKILL sent to PID 123" {
		t.Errorf("Expected user status message to be preserved, but got: %s", m2.Message)
	}
}

func TestModel_Update_Navigation(t *testing.T) {
	m := NewModel()
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
