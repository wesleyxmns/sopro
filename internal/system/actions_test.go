package system

import (
	"os"
	"testing"
)

func TestIsRoot_DoesNotPanic(t *testing.T) {
	// Apenas garantir que roda sem erro
	_ = IsRoot()
}

func TestKillProcess_InvalidPID(t *testing.T) {
	err := KillProcess(-9999)
	if err == nil {
		t.Error("Expected error when killing invalid PID")
	}
	expectedErr := "invalid PID: must be greater than 0"
	if err.Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, err.Error())
	}

	errZero := KillProcess(0)
	if errZero == nil {
		t.Error("Expected error when killing PID 0")
	}
	if errZero.Error() != expectedErr {
		t.Errorf("Expected error %q, got %q", expectedErr, errZero.Error())
	}
}

func TestDropCaches_NonRoot_ReturnsPermissionError(t *testing.T) {
	if IsRoot() {
		t.Skip("Skipping test; environment is running as root")
	}
	err := DropCaches()
	if err != os.ErrPermission {
		t.Errorf("Expected os.ErrPermission, got %v", err)
	}
}

