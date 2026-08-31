package provider

import (
	"context"
	"strings"
	"testing"

	processdomain "github.com/wesleyxmns/sopro/internal/process"
)

type mockProvider struct {
	name      string
	supports  bool
	contexts  []ContextInfo
	actions   []Action
	executed  string
	returnErr error
}

func (m *mockProvider) Name() string { return m.name }
func (m *mockProvider) Supports(processdomain.Info) bool { return m.supports }
func (m *mockProvider) Detect(context.Context, processdomain.Info) []ContextInfo { return m.contexts }
func (m *mockProvider) Actions(context.Context, processdomain.Info) []Action { return m.actions }
func (m *mockProvider) Execute(_ context.Context, actionID string, _ processdomain.Info) error {
	m.executed = actionID
	return m.returnErr
}

func TestRegistryDetectAndActions(t *testing.T) {
	p1 := &mockProvider{
		name:     "docker",
		supports: true,
		contexts: []ContextInfo{
			{Tag: processdomain.ContextDockerCompose, Label: "compose: web-app"},
		},
		actions: []Action{
			{ID: "docker-stop", Label: "Parar container"},
		},
	}
	p2 := &mockProvider{
		name:     "git",
		supports: false,
		contexts: []ContextInfo{
			{Tag: processdomain.ContextGitRepository, Label: "git: sopro"},
		},
	}

	registry := NewRegistry(p1, p2)
	if len(registry.Providers()) != 2 {
		t.Fatalf("expected 2 providers, got %d", len(registry.Providers()))
	}

	ctx := context.Background()
	proc := processdomain.Info{Identity: processdomain.Identity{PID: 100}}

	contexts := registry.Detect(ctx, proc)
	if len(contexts) != 1 || contexts[0].Tag != processdomain.ContextDockerCompose {
		t.Fatalf("unexpected contexts: %+v", contexts)
	}

	actions := registry.Actions(ctx, proc)
	if len(actions) != 1 || actions[0].ID != "docker-stop" {
		t.Fatalf("unexpected actions: %+v", actions)
	}

	if err := registry.Execute(ctx, "docker-stop", proc); err != nil {
		t.Fatalf("failed to execute action: %v", err)
	}
	if p1.executed != "docker-stop" {
		t.Fatalf("expected executed 'docker-stop', got %q", p1.executed)
	}

	if err := registry.Execute(ctx, "nonexistent", proc); err != ErrIncompatibleAction && err != ErrActionNotFound {
		t.Fatalf("expected ErrIncompatibleAction or ErrActionNotFound, got %v", err)
	}
}

func TestRegistryActionScopeAndSupportsAction(t *testing.T) {
	dockerProvider := &mockProvider{
		name:     "docker",
		supports: true,
		actions: []Action{
			{ID: "docker.stop", Scope: ScopeContainer, Label: "Parar container"},
		},
	}
	cdpProvider := &mockProvider{
		name:     "cdp",
		supports: false,
		actions: []Action{
			{ID: "cdp.close_blank", Scope: ScopeBrowser, Label: "Fechar abas vazias"},
		},
	}

	registry := NewRegistry(dockerProvider, cdpProvider)
	ctx := context.Background()
	dockerProc := processdomain.Info{Identity: processdomain.Identity{PID: 100}}

	if !registry.SupportsAction(ctx, "docker.stop", dockerProc) {
		t.Fatal("expected SupportsAction to return true for docker.stop on dockerProc")
	}
	if registry.SupportsAction(ctx, "cdp.close_blank", dockerProc) {
		t.Fatal("expected SupportsAction to return false for cdp.close_blank on dockerProc")
	}

	// Executing cdp action on docker process must return ErrIncompatibleAction
	err := registry.Execute(ctx, "cdp.close_blank", dockerProc)
	if err != ErrIncompatibleAction {
		t.Fatalf("expected ErrIncompatibleAction, got %v", err)
	}
}

func TestMaskSensitiveArgs(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "CLI password flag with equals",
			input:    "mysql -u root --password=supersecret -h localhost",
			expected: "mysql -u root --password=****** -h localhost",
		},
		{
			name:     "CLI token flag with space",
			input:    "curl -H Authorization: --token my-secret-token https://api.com",
			expected: "curl -H Authorization: --token ****** https://api.com",
		},
		{
			name:     "Database URI with credentials",
			input:    "postgres -d postgres://admin:P@ssword123@db.internal:5432/production",
			expected: "postgres -d postgres://admin:******@db.internal:5432/production",
		},
		{
			name:     "Bearer token in auth header",
			input:    "app --auth Bearer eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
			expected: "app --auth Bearer ******",
		},
		{
			name:     "Inline environment secret",
			input:    "node server.js JWT_SECRET=top_secret_key_123",
			expected: "node server.js JWT_SECRET=******",
		},
		{
			name:     "Normal command without secrets",
			input:    "gopls -mode=stdio",
			expected: "gopls -mode=stdio",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := MaskSensitiveArgs(tc.input)
			if got != tc.expected {
				t.Fatalf("MaskSensitiveArgs(%q) =\n got:  %q\n want: %q", tc.input, got, tc.expected)
			}
			if strings.Contains(got, "supersecret") || strings.Contains(got, "my-secret-token") ||
				strings.Contains(got, "P@ssword123") || strings.Contains(got, "top_secret_key_123") {
				t.Fatalf("secret leaked in masked output: %s", got)
			}
		})
	}
}
