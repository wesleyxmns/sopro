package provider

import (
	"context"
	"testing"

	processdomain "sopro/internal/process"
)

func TestGitProviderSupports(t *testing.T) {
	p := NewGitProvider(newMockCommandRunner())

	if !p.Supports(processdomain.Info{Category: processdomain.CategoryDevelopment}) {
		t.Fatal("expected support for CategoryDevelopment")
	}
	if !p.Supports(processdomain.Info{Cwd: "/some/path"}) {
		t.Fatal("expected support when process has Cwd")
	}
	if !p.Supports(processdomain.Info{Command: "git-credential-helper"}) {
		t.Fatal("expected support for git command")
	}
}

func TestGitProviderDetectWithBranch(t *testing.T) {
	runner := newMockCommandRunner()
	dir := "/home/user/projects/sopro"
	runner.responses["git -C /home/user/projects/sopro rev-parse --show-toplevel"] = []byte("/home/user/projects/sopro\n")
	runner.responses["git -C /home/user/projects/sopro branch --show-current"] = []byte("feature/phase5\n")

	p := NewGitProvider(runner)
	proc := processdomain.Info{
		Category: processdomain.CategoryDevelopment,
		Command:  "gopls",
		Cwd:      dir,
	}

	contexts := p.Detect(context.Background(), proc)
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(contexts))
	}
	if contexts[0].Tag != processdomain.ContextGitRepository {
		t.Fatalf("tag = %q; want %q", contexts[0].Tag, processdomain.ContextGitRepository)
	}
	if contexts[0].Label != "git: sopro (feature/phase5)" {
		t.Fatalf("label = %q; want 'git: sopro (feature/phase5)'", contexts[0].Label)
	}
	if contexts[0].Details["repository"] != "sopro" || contexts[0].Details["branch"] != "feature/phase5" {
		t.Fatalf("details = %+v", contexts[0].Details)
	}
}

func TestGitProviderActionsAndExecute(t *testing.T) {
	runner := newMockCommandRunner()
	dir := "/home/user/sopro"
	runner.responses["git -C /home/user/sopro status --short"] = []byte(" M main.go\n")
	runner.responses["git -C /home/user/sopro fetch --dry-run"] = []byte("")

	p := NewGitProvider(runner)
	proc := processdomain.Info{
		Cwd: dir,
	}

	actions := p.Actions(context.Background(), proc)
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}

	ctx := context.Background()
	if err := p.Execute(ctx, "git.status", proc); err != nil {
		t.Fatalf("git.status failed: %v", err)
	}
	if err := p.Execute(ctx, "git.fetch", proc); err != nil {
		t.Fatalf("git.fetch failed: %v", err)
	}
	if err := p.Execute(ctx, "git.nonexistent", proc); err == nil {
		t.Fatal("expected error on nonexistent action")
	}
}
