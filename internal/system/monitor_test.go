package system

import "testing"

func TestGetSystemMetrics(t *testing.T) {
	metrics, err := GetSystemMetrics()
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if metrics.TotalRAM == 0 {
		t.Error("Expected TotalRAM to be greater than 0")
	}
}

func TestGetTopProcesses(t *testing.T) {
	procs, err := GetTopProcesses(5)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}
	if len(procs) == 0 {
		t.Error("Expected to find at least one process")
	}
}
