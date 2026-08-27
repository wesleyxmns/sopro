package tui

import (
	"testing"
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
