package system

import "testing"

func TestIsRoot_DoesNotPanic(t *testing.T) {
	// Apenas garantir que roda sem erro
	_ = IsRoot()
}

func TestKillProcess_InvalidPID(t *testing.T) {
	err := KillProcess(-9999)
	if err == nil {
		t.Error("Expected error when killing invalid PID")
	}
}
