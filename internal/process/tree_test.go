package process

import "testing"

func TestBuildTreeOrdersParentsBeforeChildren(t *testing.T) {
	processes := []Info{
		{Identity: Identity{PID: 3}, ParentPID: 2, MemoryBytes: 10},
		{Identity: Identity{PID: 1}, MemoryBytes: 30},
		{Identity: Identity{PID: 2}, ParentPID: 1, MemoryBytes: 20},
	}
	got := BuildTree(processes)
	if len(got) != 3 || got[0].PID != 1 || got[1].PID != 2 || got[1].Depth != 1 || got[2].Depth != 2 {
		t.Fatalf("unexpected tree: %+v", got)
	}
}
