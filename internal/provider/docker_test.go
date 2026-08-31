package provider

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"testing"

	processdomain "github.com/wesleyxmns/sopro/internal/process"
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
	runner.responses[fmt.Sprintf("docker pause %s", containerID)] = []byte("paused\n")

	p := NewDockerProvider(runner)
	proc := processdomain.Info{
		Category: processdomain.CategoryContainer,
		Command:  fmt.Sprintf("containerd-shim -id %s", containerID),
	}

	actions := p.Actions(context.Background(), proc)
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions, got %d", len(actions))
	}
	if actions[0].ID != "docker.stop" || actions[1].ID != "docker.restart" || actions[2].ID != "docker.pause" {
		t.Fatalf("actions = %+v", actions)
	}

	ctx := context.Background()
	if err := p.Execute(ctx, "docker.stop", proc); err != nil {
		t.Fatalf("docker.stop failed: %v", err)
	}
	if err := p.Execute(ctx, "docker.restart", proc); err != nil {
		t.Fatalf("docker.restart failed: %v", err)
	}
	if err := p.Execute(ctx, "docker.pause", proc); err != nil {
		t.Fatalf("docker.pause failed: %v", err)
	}

	if err := p.Execute(ctx, "unknown.action", proc); err == nil {
		t.Fatal("expected error on unknown action")
	}
}

func TestDockerProviderPIDMapping(t *testing.T) {
	runner := newMockCommandRunner()
	runner.responses["docker ps -a -q"] = []byte("c1d2e3f4a5b6\n")
	inspectJSON := `[{
		"Id": "c1d2e3f4a5b67890abcdef123456",
		"Name": "/sangati_postgres",
		"State": {
			"Pid": 12345,
			"Status": "running",
			"Running": true
		},
		"Config": {
			"Image": "postgres:15-alpine",
			"Labels": {
				"com.docker.compose.project": "sangati",
				"com.docker.compose.service": "postgres"
			}
		}
	}]`
	runner.responses["docker inspect c1d2e3f4a5b6"] = []byte(inspectJSON)

	p := NewDockerProvider(runner)
	proc := processdomain.Info{
		Identity: processdomain.Identity{PID: 12345},
		Command:  "postgres",
	}

	contexts := p.Detect(context.Background(), proc)
	if len(contexts) != 1 {
		t.Fatalf("expected 1 context from PID mapping, got %d", len(contexts))
	}
	if contexts[0].Tag != processdomain.ContextDockerCompose {
		t.Fatalf("expected ContextDockerCompose, got %v", contexts[0].Tag)
	}
	if contexts[0].Details["container_name"] != "sangati_postgres" {
		t.Fatalf("container_name = %q; want sangati_postgres", contexts[0].Details["container_name"])
	}
	if contexts[0].Details["image"] != "postgres:15-alpine" {
		t.Fatalf("image = %q; want postgres:15-alpine", contexts[0].Details["image"])
	}

	actions := p.Actions(context.Background(), proc)
	if len(actions) != 3 {
		t.Fatalf("expected 3 actions for mapped container, got %d", len(actions))
	}
}

func TestDockerProviderStoppedContainersAndStart(t *testing.T) {
	runner := newMockCommandRunner()
	runner.responses["docker ps -a -q"] = []byte("stopped123\n")
	inspectJSON := `[{
		"Id": "stopped1234567890abcdef123456",
		"Name": "/stopped_redis",
		"State": {
			"Pid": 0,
			"Status": "exited",
			"Running": false
		},
		"Config": {
			"Image": "redis:7-alpine",
			"Labels": {}
		}
	}]`
	runner.responses["docker inspect stopped123"] = []byte(inspectJSON)
	runner.responses["docker start stopped_redis"] = []byte("stopped_redis\n")

	p := NewDockerProvider(runner)
	containers := p.AllContainers(context.Background())
	if len(containers) != 1 {
		t.Fatalf("expected 1 container, got %d", len(containers))
	}
	if containers[0].Name != "stopped_redis" || containers[0].State != "exited" {
		t.Fatalf("unexpected container: %+v", containers[0])
	}

	proc := processdomain.Info{
		ContainerName: "stopped_redis",
		State:         processdomain.StateStopped,
		Category:      processdomain.CategoryContainer,
	}

	actions := p.Actions(context.Background(), proc)
	if len(actions) != 1 || actions[0].ID != "docker.start" {
		t.Fatalf("expected 1 docker.start action, got %+v", actions)
	}

	if err := p.Execute(context.Background(), "docker.start", proc); err != nil {
		t.Fatalf("docker.start failed: %v", err)
	}
}


