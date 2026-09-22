package provider

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	processdomain "github.com/wesleyxmns/sopro/internal/process"
)

type GitProvider struct {
	runner    CommandRunner
	mu        sync.RWMutex
	repoByDir map[string]cachedRepo
	cacheTTL  time.Duration
}

type cachedRepo struct {
	repoDir   string
	branch    string
	found     bool
	fetchedAt time.Time
}

func NewGitProvider(runner ...CommandRunner) *GitProvider {
	var r CommandRunner = osCommandRunner{}
	if len(runner) > 0 && runner[0] != nil {
		r = runner[0]
	}
	return &GitProvider{runner: r, repoByDir: make(map[string]cachedRepo), cacheTTL: 30 * time.Second}
}

func (g *GitProvider) Name() string {
	return "git"
}

func (g *GitProvider) Supports(proc processdomain.Info) bool {
	if proc.Category == processdomain.CategoryDevelopment {
		return true
	}
	if proc.Cwd != "" {
		return true
	}
	lower := strings.ToLower(proc.Command)
	return strings.Contains(lower, "git") || strings.Contains(lower, "node") || strings.Contains(lower, "go") || strings.Contains(lower, "python")
}

func (g *GitProvider) Detect(ctx context.Context, proc processdomain.Info) []ContextInfo {
	targetDir := proc.Cwd
	if targetDir == "" {
		return nil
	}

	repoDir, branch := g.cachedRepoInfo(ctx, targetDir)
	if repoDir == "" {
		return nil
	}

	repoName := filepath.Base(repoDir)
	label := fmt.Sprintf("git: %s", repoName)
	if branch != "" {
		label = fmt.Sprintf("git: %s (%s)", repoName, branch)
	}

	return []ContextInfo{
		{
			Tag:   processdomain.ContextGitRepository,
			Label: label,
			Details: map[string]string{
				"repository": repoName,
				"path":       repoDir,
				"branch":     branch,
			},
		},
	}
}

// cachedRepoInfo memoizes repo lookups per directory, including negative
// results. Repository metadata only feeds labels, so a longer TTL than the
// snapshot tick is safe.
func (g *GitProvider) cachedRepoInfo(ctx context.Context, dir string) (string, string) {
	g.mu.RLock()
	entry, ok := g.repoByDir[dir]
	g.mu.RUnlock()
	if ok && time.Since(entry.fetchedAt) < g.cacheTTL {
		if !entry.found {
			return "", ""
		}
		return entry.repoDir, entry.branch
	}
	repoDir, branch := g.findRepoInfo(ctx, dir)
	g.mu.Lock()
	g.repoByDir[dir] = cachedRepo{repoDir: repoDir, branch: branch, found: repoDir != "", fetchedAt: time.Now()}
	g.mu.Unlock()
	return repoDir, branch
}

func (g *GitProvider) findRepoInfo(ctx context.Context, dir string) (string, string) {
	out, err := g.runner.Run(ctx, "git", "-C", dir, "rev-parse", "--show-toplevel")
	if err == nil {
		repoDir := strings.TrimSpace(string(out))
		if repoDir != "" {
			branchOut, _ := g.runner.Run(ctx, "git", "-C", dir, "branch", "--show-current")
			branch := strings.TrimSpace(string(branchOut))
			return repoDir, branch
		}
	}

	curr := dir
	for {
		gitPath := filepath.Join(curr, ".git")
		if stat, err := os.Stat(gitPath); err == nil && (stat.IsDir() || !stat.IsDir()) {
			return curr, ""
		}
		parent := filepath.Dir(curr)
		if parent == curr || parent == "" {
			break
		}
		curr = parent
	}
	return "", ""
}

func (g *GitProvider) Actions(ctx context.Context, proc processdomain.Info) []Action {
	if proc.Cwd == "" {
		return nil
	}
	return []Action{
		{
			ID:          "git.status",
			Scope:       ScopeGit,
			Label:       "git status",
			Description: "Executa verificação de status do repositório",
			Danger:      false,
		},
		{
			ID:          "git.fetch",
			Scope:       ScopeGit,
			Label:       "git fetch",
			Description: "Busca atualizações no repositório remoto",
			Danger:      false,
		},
	}
}

func (g *GitProvider) Execute(ctx context.Context, actionID string, proc processdomain.Info) (uint64, error) {
	if proc.Cwd == "" {
		return 0, fmt.Errorf("%w: processo sem diretório de trabalho", ErrUnsupported)
	}
	switch actionID {
	case "git.status":
		_, err := g.runner.Run(ctx, "git", "-C", proc.Cwd, "status", "--short")
		return 0, err
	case "git.fetch":
		_, err := g.runner.Run(ctx, "git", "-C", proc.Cwd, "fetch", "--dry-run")
		return 0, err
	default:
		return 0, fmt.Errorf("%w: ação %s", ErrActionNotFound, actionID)
	}
}
