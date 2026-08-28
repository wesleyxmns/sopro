//go:build windows

package platform

import "testing"

func TestNewWiresWindowsDependencies(t *testing.T) {
	dependencies := New()
	if dependencies.Snapshots == nil ||
		dependencies.Processes == nil ||
		dependencies.Cache == nil ||
		dependencies.Capabilities == nil {
		t.Fatal("windows factory returned incomplete dependencies")
	}
	if dependencies.ProcessWaiter != nil {
		t.Fatal("windows factory advertised unsupported process waiting")
	}
}
