package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	processdomain "sopro/internal/process"
)

type CommandRunner interface {
	Run(ctx context.Context, name string, args ...string) ([]byte, error)
}

type osCommandRunner struct{}

func (osCommandRunner) Run(ctx context.Context, name string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, name, args...)
	return cmd.Output()
}

var (
	containerIDPattern = regexp.MustCompile(`(?:-id|--id|containerd-shim\s+-[a-zA-Z]*id\s+|docker\/containers\/)([0-9a-fA-F]{12,64})`)
	hexIDPattern       = regexp.MustCompile(`\b([0-9a-fA-F]{12,64})\b`)
)

type DockerProvider struct {
	runner CommandRunner
}

func NewDockerProvider(runner ...CommandRunner) *DockerProvider {
	var r CommandRunner = osCommandRunner{}
	if len(runner) > 0 && runner[0] != nil {
		r = runner[0]
	}
	return &DockerProvider{runner: r}
}

func (d *DockerProvider) Name() string {
	return "docker"
}

func (d *DockerProvider) Supports(proc processdomain.Info) bool {
	if proc.Category == processdomain.CategoryContainer {
		return true
	}
	lower := strings.ToLower(proc.Command)
	return strings.Contains(lower, "docker") || strings.Contains(lower, "containerd")
}

type inspectContainer struct {
	ID     string `json:"Id"`
	Name   string `json:"Name"`
	Config struct {
		Labels map[string]string `json:"Labels"`
	} `json:"Config"`
}

func (d *DockerProvider) Detect(ctx context.Context, proc processdomain.Info) []ContextInfo {
	targetID := extractContainerID(proc.Command)
	if targetID == "" {
		return nil
	}

	details := map[string]string{
		"container_id": targetID,
	}

	output, err := d.runner.Run(ctx, "docker", "inspect", targetID)
	if err == nil && len(output) > 0 {
		var containers []inspectContainer
		if err := json.Unmarshal(output, &containers); err == nil && len(containers) > 0 {
			c := containers[0]
			name := strings.TrimPrefix(c.Name, "/")
			if name != "" {
				details["container_name"] = name
			}

			labels := c.Config.Labels
			project := labels["com.docker.compose.project"]
			service := labels["com.docker.compose.service"]

			if project != "" && service != "" {
				details["compose_project"] = project
				details["compose_service"] = service
				return []ContextInfo{
					{
						Tag:     processdomain.ContextDockerCompose,
						Label:   fmt.Sprintf("compose: %s/%s", project, service),
						Details: details,
					},
				}
			}

			if name != "" {
				return []ContextInfo{
					{
						Tag:     processdomain.ContextTag("docker-container"),
						Label:   fmt.Sprintf("container: %s", name),
						Details: details,
					},
				}
			}
		}
	}

	shortID := targetID
	if len(shortID) > 12 {
		shortID = shortID[:12]
	}
	return []ContextInfo{
		{
			Tag:     processdomain.ContextTag("docker-container"),
			Label:   fmt.Sprintf("container: %s", shortID),
			Details: details,
		},
	}
}

func (d *DockerProvider) Actions(ctx context.Context, proc processdomain.Info) []Action {
	targetID := extractContainerID(proc.Command)
	if targetID == "" {
		return nil
	}

	return []Action{
		{
			ID:          "docker.stop",
			Label:       "parar container",
			Description: fmt.Sprintf("Executa 'docker stop' no container %s", targetID[:min(12, len(targetID))]),
			Danger:      false,
		},
		{
			ID:          "docker.restart",
			Label:       "reiniciar container",
			Description: fmt.Sprintf("Executa 'docker restart' no container %s", targetID[:min(12, len(targetID))]),
			Danger:      false,
		},
	}
}

func (d *DockerProvider) Execute(ctx context.Context, actionID string, proc processdomain.Info) error {
	targetID := extractContainerID(proc.Command)
	if targetID == "" {
		return fmt.Errorf("%w: nenhum container identificado no processo", ErrUnsupported)
	}

	switch actionID {
	case "docker.stop":
		_, err := d.runner.Run(ctx, "docker", "stop", targetID)
		return err
	case "docker.restart":
		_, err := d.runner.Run(ctx, "docker", "restart", targetID)
		return err
	default:
		return fmt.Errorf("%w: ação %s", ErrActionNotFound, actionID)
	}
}

func extractContainerID(command string) string {
	if match := containerIDPattern.FindStringSubmatch(command); len(match) > 1 {
		return match[1]
	}
	if match := hexIDPattern.FindStringSubmatch(command); len(match) > 1 {
		return match[1]
	}
	return ""
}
