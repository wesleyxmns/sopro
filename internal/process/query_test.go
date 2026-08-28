package process

import "testing"

func TestQueryFiltersFuzzyAndSorts(t *testing.T) {
	processes := []Info{
		{Identity: Identity{PID: 1}, Command: "google-chrome", User: "wesley", Category: CategoryBrowser, MemoryBytes: 100, CPUPct: 2},
		{Identity: Identity{PID: 2}, Command: "postgres", User: "postgres", Category: CategoryDatabase, MemoryBytes: 300, CPUPct: 1},
		{Identity: Identity{PID: 3}, Command: "firefox", User: "wesley", Category: CategoryBrowser, MemoryBytes: 200, CPUPct: 3},
	}
	got := (Query{Search: "frfx", Category: CategoryBrowser, Sort: SortCPU}).Apply(processes)
	if len(got) != 1 || got[0].PID != 3 {
		t.Fatalf("unexpected query result: %+v", got)
	}
}

func TestQueryDoesNotMutateInput(t *testing.T) {
	processes := []Info{{Identity: Identity{PID: 1}, MemoryBytes: 1}, {Identity: Identity{PID: 2}, MemoryBytes: 2}}
	_ = (Query{Sort: SortMemory}).Apply(processes)
	if processes[0].PID != 1 {
		t.Fatal("query mutated input")
	}
}
