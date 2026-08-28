//go:build linux

package platform

import "testing"

func TestNewWiresLinuxDependencies(t *testing.T) {
	dependencies := New()
	if dependencies.Snapshots == nil ||
		dependencies.Processes == nil ||
		dependencies.ProcessWaiter == nil ||
		dependencies.Cache == nil ||
		dependencies.Capabilities == nil {
		t.Fatal("linux factory returned incomplete dependencies")
	}
}
