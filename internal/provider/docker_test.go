package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	processdomain "sopro/internal/process"
)

type mockCommandRunner struct {
	responses map[string][]byte
	errors    map[string]error
	calls     [][]string
}

func newMockCommandRunner() *mockCommandRunner {
	return &mockCommandRunner{
		responses: make(map[string][]byte),
		errors:    make(map[string]error),
	}
}

func (m *mockCommandRunner) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	cmdKey := name + " " + strings.Join(args, " ")
	m.calls = append(m.calls, append([]string{name}, args...))
	if err, ok := m.errors[cmdKey]; ok {
		return nil, err
	}
	if resp, ok := m.responses[cmdKey]; ok {
		return resp, nil
	}
	return nil, errors.New("command failed: not mocked")
}

func TestDockerProviderSupports(t *testing.T) {
	p := NewDockerProvider(newMockCommandRunner())

	if !p.Supports(processdomain.Info{Category: processdomain.CategoryContainer, Command: "app"}) {
		t.Fatal("expected support for CategoryContainer")
	}
	if !p.Supports(processdomain.Info{Command: "containerd-shim -id 1234567890ab"}) {
		t.Fatal("expected support for containerd-shim command")
	}
	if !p.Supports(processdomain.Info{Command: "docker-proxy -proto tcp"}) {
		t.Fatal("expected support for docker-proxy")
	}
	if p.Supports(processdomain.Info{Category: processdomain.CategoryBrowser, Command: "chrome"}) {
		t.Fatal("did not expect support for chrome browser")
	}
}

func TestDockerProviderDetectCompose(t *testing.T) {
	runner := newMockCommandRunner()
	containerID := "a1b2c3d4e5f67890abcdef1234567890"
	inspectJSON := `[{
		"Id": "` + containerID + `",
		"Name": "/sopro-web-1",
		"Config": {
			"Labels": {
				"com.docker.compose.project": "sopro-stack",
				"com.docker.compose.service": "web"
			}
		}
	}]`
	runner.responses[fmt.Sprintf("docker inspect %s", containerID)] = []byte(inspectJSON)

	p := NewDockerProvider(runner)
	proc := processdomain.Info{
		Category: processdomain.CategoryContainer,
		Command:  fmt.Sprintf("containerd-shim -id %s", containerID),
	}

	contexts := p.Detect(context.Background(), proc)
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(contexts))
	}
	if contexts[0].Tag != processdomain.ContextDockerCompose {
		t.Fatalf("tag = %q; want %q", contexts[0].Tag, processdomain.ContextDockerCompose)
	}
	if contexts[0].Label != "compose: sopro-stack/web" {
		t.Fatalf("label = %q; want %q", contexts[0].Label, "compose: sopro-stack/web")
	}
	if contexts[0].Details["compose_project"] != "sopro-stack" || contexts[0].Details["compose_service"] != "web" {
		t.Fatalf("details = %+v", contexts[0].Details)
	}
}

func TestDockerProviderDetectPlainContainer(t *testing.T) {
	runner := newMockCommandRunner()
	containerID := "112233445566778899aabbcc"
	inspectJSON := `[{
		"Id": "` + containerID + `",
		"Name": "/standalone-redis",
		"Config": {
			"Labels": {}
		}
	}]`
	runner.responses[fmt.Sprintf("docker inspect %s", containerID)] = []byte(inspectJSON)

	p := NewDockerProvider(runner)
	proc := processdomain.Info{
		Category: processdomain.CategoryContainer,
		Command:  fmt.Sprintf("docker run -d %s redis", containerID),
	}

	contexts := p.Detect(context.Background(), proc)
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context, got %d", len(contexts))
	}
	if contexts[0].Label != "container: standalone-redis" {
		t.Fatalf("label = %q; want 'container: standalone-redis'", contexts[0].Label)
	}
}

func TestDockerProviderDetectFallbackOnInspectFailure(t *testing.T) {
	runner := newMockCommandRunner()
	containerID := "deadbeef1234567890abcdef"
	runner.errors[fmt.Sprintf("docker inspect %s", containerID)] = errors.New("daemon offline")

	p := NewDockerProvider(runner)
	proc := processdomain.Info{
		Category: processdomain.CategoryContainer,
		Command:  fmt.Sprintf("containerd-shim -id %s", containerID),
	}

	contexts := p.Detect(context.Background(), proc)
	if len(contexts) != 1 {
		t.Fatalf("expected 1 fallback context, got %d", len(contexts))
	}
	if contexts[0].Label != "container: deadbeef1234" {
		t.Fatalf("fallback label = %q; want 'container: deadbeef1234'", contexts[0].Label)
	}
}

func TestDockerProviderActionsAndExecute(t *testing.T) {
	runner := newMockCommandRunner()
	containerID := "aabbccddeeff001122334455"
	runner.responses[fmt.Sprintf("docker stop %s", containerID)] = []byte("stopped\n")
	runner.responses[fmt.Sprintf("docker restart %s", containerID)] = []byte("restarted\n")

	p := NewDockerProvider(runner)
	proc := processdomain.Info{
		Category: processdomain.CategoryContainer,
		Command:  fmt.Sprintf("containerd-shim -id %s", containerID),
	}

	actions := p.Actions(context.Background(), proc)
	if len(actions) != 2 {
		t.Fatalf("expected 2 actions, got %d", len(actions))
	}
	if actions[0].ID != "docker.stop" || actions[1].ID != "docker.restart" {
		t.Fatalf("actions = %+v", actions)
	}

	ctx := context.Background()
	if err := p.Execute(ctx, "docker.stop", proc); err != nil {
		t.Fatalf("docker.stop failed: %v", err)
	}
	if err := p.Execute(ctx, "docker.restart", proc); err != nil {
		t.Fatalf("docker.restart failed: %v", err)
	}

	if err := p.Execute(ctx, "unknown.action", proc); err == nil {
		t.Fatal("expected error on unknown action")
	}
}
