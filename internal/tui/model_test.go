package tui

import (
	"testing"
	"time"
)

func TestNewModel(t *testing.T) {
	m := NewModel()
	// IsRoot could be true or false depending on the environment, we just check it doesn't panic.
	_ = m.IsRoot
	
	if m.Cursor != 0 {
		t.Errorf("Expected cursor to be 0, got %d", m.Cursor)
	}
	
	if m.Message != "" {
		t.Errorf("Expected message to be empty, got %s", m.Message)
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
