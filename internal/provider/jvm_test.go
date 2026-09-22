package provider

import (
	"context"
	"fmt"
	"testing"

	processdomain "github.com/wesleyxmns/sopro/internal/process"
)

func TestJVMProviderSupports(t *testing.T) {
	p := NewJVMProvider(newMockCommandRunner())

	if !p.Supports(processdomain.Info{Category: processdomain.CategoryJVM}) {
		t.Fatal("expected support for CategoryJVM")
	}
	if !p.Supports(processdomain.Info{Command: "java"}) {
		t.Fatal("expected support for java command")
	}
	if !p.Supports(processdomain.Info{Command: "/snap/intellij-idea/118/bin/idea"}) {
		t.Fatal("expected support for intellij-idea")
	}
	if p.Supports(processdomain.Info{Category: processdomain.CategoryBrowser, Command: "chrome"}) {
		t.Fatal("did not expect support for chrome")
	}
}

func TestJVMProviderDetect(t *testing.T) {
	p := NewJVMProvider(newMockCommandRunner())
	proc := processdomain.Info{
		Identity:    processdomain.Identity{PID: 2493913},
		Category:    processdomain.CategoryJVM,
		Command:     "java",
		CommandLine: "/usr/lib/jvm/java-1.21.0-openjdk/bin/java -Xmx4g -Djava.awt.headless=true",
	}

	contexts := p.Detect(context.Background(), proc)
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(contexts))
	}
	if contexts[0].Label != "JVM (Heap Max: 4G)" {
		t.Fatalf("label = %q; want 'JVM (Heap Max: 4G)'", contexts[0].Label)
	}
	if contexts[0].Details["max_heap"] != "4G" {
		t.Fatalf("details = %+v", contexts[0].Details)
	}
}

func TestJVMProviderActionsAndExecute(t *testing.T) {
	runner := newMockCommandRunner()
	pid := int32(2493913)
	runner.responses[fmt.Sprintf("jcmd %d GC.run", pid)] = []byte("Command executed successfully\n")

	p := NewJVMProvider(runner)
	proc := processdomain.Info{
		Identity: processdomain.Identity{PID: pid},
		Category: processdomain.CategoryJVM,
		Command:  "java",
	}

	actions := p.Actions(context.Background(), proc)
	if len(actions) != 1 || actions[0].ID != "jvm.run_gc" {
		t.Fatalf("actions = %+v", actions)
	}
	if actions[0].Scope != ScopeJVM {
		t.Fatalf("expected ScopeJVM, got %v", actions[0].Scope)
	}

	ctx := context.Background()
	if _, err := p.Execute(ctx, "jvm.run_gc", proc); err != nil {
		t.Fatalf("failed to execute jvm.run_gc: %v", err)
	}
	if len(runner.calls) != 1 {
		t.Fatalf("expected 1 command call, got %d", len(runner.calls))
	}
}

func TestReclaimedBytesGuardsUnreadableDrops(t *testing.T) {
	cases := []struct {
		name        string
		before, aft uint64
		want        uint64
	}{
		{"drop", 1000, 600, 400},
		{"growth", 600, 1000, 0},
		{"same", 600, 600, 0},
		{"vanished", 600, 0, 0},
		{"unreadable before", 0, 600, 0},
	}
	for _, tc := range cases {
		if got := reclaimedBytes(tc.before, tc.aft); got != tc.want {
			t.Fatalf("%s: got %d, want %d", tc.name, got, tc.want)
		}
	}
}

func TestJVMProviderCacheTargets(t *testing.T) {
	p := NewJVMProvider(newMockCommandRunner())
	proc := processdomain.Info{
		Identity: processdomain.Identity{PID: 4242},
		Category: processdomain.CategoryJVM,
		Command:  "java",
	}

	targets := p.CacheTargets(proc)
	if len(targets) != 1 {
		t.Fatalf("expected 1 cache target, got %d", len(targets))
	}
	target := targets[0]
	if target.ActionID != "jvm.run_gc" || target.Source != "JVM" {
		t.Fatalf("unexpected target: %+v", target)
	}
	if target.Proc.PID != 4242 {
		t.Fatalf("target proc PID = %d; want 4242", target.Proc.PID)
	}

	if targets := p.CacheTargets(processdomain.Info{}); len(targets) != 0 {
		t.Fatalf("expected no targets without PID, got %+v", targets)
	}
}
