package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"time"

	processdomain "github.com/wesleyxmns/sopro/internal/process"
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

type ContainerMeta struct {
	ID      string
	Name    string
	Image   string
	Project string
	Service string
	Pid     int32
	State   string
}

type DockerProvider struct {
	runner          CommandRunner
	containersByPid map[int32]*ContainerMeta
	containersByID  map[string]*ContainerMeta
	allContainers   []ContainerMeta
	lastFetch       time.Time
	cacheTTL        time.Duration
	mu              sync.RWMutex
}

func NewDockerProvider(runner ...CommandRunner) *DockerProvider {
	var r CommandRunner = osCommandRunner{}
	if len(runner) > 0 && runner[0] != nil {
		r = runner[0]
	}
	return &DockerProvider{
		runner:          r,
		containersByPid: make(map[int32]*ContainerMeta),
		containersByID:  make(map[string]*ContainerMeta),
		allContainers:   make([]ContainerMeta, 0),
		cacheTTL:        5 * time.Second,
	}
}

func (d *DockerProvider) Name() string {
	return "docker"
}

func (d *DockerProvider) refreshCache(ctx context.Context) {
	d.mu.RLock()
	if time.Since(d.lastFetch) < d.cacheTTL && len(d.containersByID) > 0 {
		d.mu.RUnlock()
		return
	}
	d.mu.RUnlock()

	d.mu.Lock()
	defer d.mu.Unlock()

	if time.Since(d.lastFetch) < d.cacheTTL && len(d.containersByID) > 0 {
		return
	}

	psOutput, err := d.runner.Run(ctx, "docker", "ps", "-a", "-q")
	if err != nil || len(psOutput) == 0 {
		d.lastFetch = time.Now()
		return
	}

	ids := strings.Fields(string(psOutput))
	if len(ids) == 0 {
		d.containersByPid = make(map[int32]*ContainerMeta)
		d.containersByID = make(map[string]*ContainerMeta)
		d.allContainers = make([]ContainerMeta, 0)
		d.lastFetch = time.Now()
		return
	}

	inspectArgs := append([]string{"inspect"}, ids...)
	inspectOutput, err := d.runner.Run(ctx, "docker", inspectArgs...)
	if err != nil || len(inspectOutput) == 0 {
		d.lastFetch = time.Now()
		return
	}

	type inspectDetail struct {
		ID     string `json:"Id"`
		Name   string `json:"Name"`
		State  struct {
			Pid     int    `json:"Pid"`
			Status  string `json:"Status"`
			Running bool   `json:"Running"`
			Paused  bool   `json:"Paused"`
		} `json:"State"`
		Config struct {
			Image  string            `json:"Image"`
			Labels map[string]string `json:"Labels"`
		} `json:"Config"`
	}

	var parsed []inspectDetail
	if err := json.Unmarshal(inspectOutput, &parsed); err != nil {
		d.lastFetch = time.Now()
		return
	}

	newByPid := make(map[int32]*ContainerMeta)
	newByID := make(map[string]*ContainerMeta)
	allList := make([]ContainerMeta, 0, len(parsed))

	for _, c := range parsed {
		name := strings.TrimPrefix(c.Name, "/")
		meta := &ContainerMeta{
			ID:      c.ID,
			Name:    name,
			Image:   c.Config.Image,
			Project: c.Config.Labels["com.docker.compose.project"],
			Service: c.Config.Labels["com.docker.compose.service"],
			Pid:     int32(c.State.Pid),
			State:   c.State.Status,
		}
		allList = append(allList, *meta)
		if meta.Pid > 0 {
			newByPid[meta.Pid] = meta
		}
		newByID[c.ID] = meta
		if len(c.ID) >= 12 {
			newByID[c.ID[:12]] = meta
		}
		if name != "" {
			newByID[name] = meta
		}
	}

	d.containersByPid = newByPid
	d.containersByID = newByID
	d.allContainers = allList
	d.lastFetch = time.Now()
}

func (d *DockerProvider) AllContainers(ctx context.Context) []ContainerMeta {
	d.refreshCache(ctx)
	d.mu.RLock()
	defer d.mu.RUnlock()
	result := make([]ContainerMeta, len(d.allContainers))
	copy(result, d.allContainers)
	return result
}

func (d *DockerProvider) DiscoverEntities(ctx context.Context) []processdomain.Info {
	containers := d.AllContainers(ctx)
	var entities []processdomain.Info
	for _, c := range containers {
		if c.Pid == 0 || c.State != "running" {
			name := c.Name
			if name == "" {
				name = c.ID[:min(12, len(c.ID))]
			}
			contexts := []processdomain.ContextTag{processdomain.ContextTag("docker-container")}
			if c.Project != "" && c.Service != "" {
				contexts = []processdomain.ContextTag{processdomain.ContextDockerCompose}
			}
			entities = append(entities, processdomain.Info{
				Identity:      processdomain.Identity{PID: 0},
				Command:       name,
				CommandLine:   fmt.Sprintf("docker (%s: %s)", c.State, c.Image),
				User:          "-",
				Category:      processdomain.CategoryContainer,
				State:         processdomain.StateStopped,
				Risk:          processdomain.RiskOK,
				ContainerID:   c.ID,
				ContainerName: c.Name,
				ImageName:     c.Image,
				Contexts:      contexts,
			})
		}
	}
	return entities
}

func (d *DockerProvider) findMeta(ctx context.Context, proc processdomain.Info) *ContainerMeta {
	d.refreshCache(ctx)

	d.mu.RLock()
	defer d.mu.RUnlock()

	if proc.PID > 0 {
		if meta, ok := d.containersByPid[proc.PID]; ok {
			return meta
		}
	}
	if proc.ContainerID != "" {
		if meta, ok := d.containersByID[proc.ContainerID]; ok {
			return meta
		}
	}
	if proc.ContainerName != "" {
		if meta, ok := d.containersByID[proc.ContainerName]; ok {
			return meta
		}
	}

	targetID := extractContainerID(proc.Command)
	if targetID != "" {
		if meta, ok := d.containersByID[targetID]; ok {
			return meta
		}
	}

	return nil
}

func (d *DockerProvider) Supports(proc processdomain.Info) bool {
	if proc.Category == processdomain.CategoryContainer || proc.ContainerID != "" || proc.ContainerName != "" {
		return true
	}
	d.mu.RLock()
	if _, ok := d.containersByPid[proc.PID]; ok {
		d.mu.RUnlock()
		return true
	}
	d.mu.RUnlock()

	lower := strings.ToLower(proc.Command)
	return strings.Contains(lower, "docker") || strings.Contains(lower, "containerd")
}

func (d *DockerProvider) Detect(ctx context.Context, proc processdomain.Info) []ContextInfo {
	meta := d.findMeta(ctx, proc)
	if meta != nil {
		details := map[string]string{
			"container_id":   meta.ID,
			"container_name": meta.Name,
		}
		if meta.Image != "" {
			details["image"] = meta.Image
		}
		if meta.Project != "" && meta.Service != "" {
			details["compose_project"] = meta.Project
			details["compose_service"] = meta.Service
			return []ContextInfo{
				{
					Tag:     processdomain.ContextDockerCompose,
					Label:   fmt.Sprintf("compose: %s/%s", meta.Project, meta.Service),
					Details: details,
				},
			}
		}
		label := meta.Name
		if label == "" {
			label = meta.ID[:min(12, len(meta.ID))]
		}
		return []ContextInfo{
			{
				Tag:     processdomain.ContextTag("docker-container"),
				Label:   fmt.Sprintf("container: %s", label),
				Details: details,
			},
		}
	}

	targetID := extractContainerID(proc.Command)
	if targetID == "" {
		return nil
	}

	details := map[string]string{
		"container_id": targetID,
	}

	output, err := d.runner.Run(ctx, "docker", "inspect", targetID)
	if err == nil && len(output) > 0 {
		var containers []struct {
			ID     string `json:"Id"`
			Name   string `json:"Name"`
			Config struct {
				Labels map[string]string `json:"Labels"`
			} `json:"Config"`
		}
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

func (d *DockerProvider) resolveTarget(proc processdomain.Info) string {
	if proc.ContainerName != "" {
		return proc.ContainerName
	}
	if proc.ContainerID != "" {
		return proc.ContainerID
	}
	d.mu.RLock()
	if meta, ok := d.containersByPid[proc.PID]; ok {
		d.mu.RUnlock()
		if meta.Name != "" {
			return meta.Name
		}
		return meta.ID
	}
	d.mu.RUnlock()
	return extractContainerID(proc.Command)
}

func (d *DockerProvider) Actions(ctx context.Context, proc processdomain.Info) []Action {
	target := d.resolveTarget(proc)
	if target == "" {
		return nil
	}

	displayTarget := target
	if len(displayTarget) > 12 && !strings.Contains(displayTarget, "-") && !strings.Contains(displayTarget, "_") {
		displayTarget = displayTarget[:12]
	}

	if proc.State == processdomain.StateStopped {
		return []Action{
			{
				ID:          "docker.start",
				Scope:       ScopeContainer,
				Label:       "iniciar container",
				Description: fmt.Sprintf("Executa 'docker start' no container %s", displayTarget),
				Danger:      false,
			},
		}
	}

	return []Action{
		{
			ID:          "docker.stop",
			Scope:       ScopeContainer,
			Label:       "parar container",
			Description: fmt.Sprintf("Executa 'docker stop' no container %s", displayTarget),
			Danger:      false,
		},
		{
			ID:          "docker.restart",
			Scope:       ScopeContainer,
			Label:       "reiniciar container",
			Description: fmt.Sprintf("Executa 'docker restart' no container %s", displayTarget),
			Danger:      false,
		},
		{
			ID:          "docker.pause",
			Scope:       ScopeContainer,
			Label:       "pausar container",
			Description: fmt.Sprintf("Executa 'docker pause' no container %s", displayTarget),
			Danger:      false,
		},
	}
}

func (d *DockerProvider) Execute(ctx context.Context, actionID string, proc processdomain.Info) error {
	target := d.resolveTarget(proc)
	if target == "" {
		return fmt.Errorf("%w: nenhum container identificado no processo", ErrUnsupported)
	}

	switch actionID {
	case "docker.start":
		_, err := d.runner.Run(ctx, "docker", "start", target)
		return err
	case "docker.stop":
		_, err := d.runner.Run(ctx, "docker", "stop", target)
		return err
	case "docker.restart":
		_, err := d.runner.Run(ctx, "docker", "restart", target)
		return err
	case "docker.pause":
		_, err := d.runner.Run(ctx, "docker", "pause", target)
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
