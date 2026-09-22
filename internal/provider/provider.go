package provider

import (
	"context"
	"errors"
	"regexp"

	processdomain "github.com/wesleyxmns/sopro/internal/process"
)

type ActionScope string

const (
	ScopeUniversal ActionScope = "universal"
	ScopeContainer ActionScope = "container"
	ScopeBrowser   ActionScope = "browser"
	ScopeJVM       ActionScope = "jvm"
	ScopeGit       ActionScope = "git"
	ScopeGlobal    ActionScope = "global"
)

var (
	ErrActionNotFound     = errors.New("action not found")
	ErrUnsupported        = errors.New("provider does not support action")
	ErrIncompatibleAction = errors.New("action is incompatible with process type/scope")
)

var (
	bearerPattern        = regexp.MustCompile(`(?i)(bearer\s+)[A-Za-z0-9\-_.~+/]+=*`)
	uriAuthPattern       = regexp.MustCompile(`(?i)([a-z]+://[^:\s]+:)([^/\s]+)(@[^/\s]+)`)
	sensitiveFlagPattern = regexp.MustCompile(`(?i)(--(?:password|passwd|pass|secret|token|api-?key|jwt|private-?key|access-?token)[=\s])([^\s]+)`)
	envSecretPattern     = regexp.MustCompile(`(?i)((?:PASSWORD|PASSWD|PASS|SECRET|TOKEN|API_KEY|AUTH_TOKEN|JWT_SECRET)[=:])([^\s]+)`)
)

// MaskSensitiveArgs removes secrets, passwords, connection strings and tokens from command lines.
func MaskSensitiveArgs(commandLine string) string {
	result := bearerPattern.ReplaceAllString(commandLine, "${1}******")
	result = uriAuthPattern.ReplaceAllString(result, "${1}******${3}")
	result = sensitiveFlagPattern.ReplaceAllString(result, "${1}******")
	result = envSecretPattern.ReplaceAllString(result, "${1}******")
	return result
}

type Action struct {
	ID          string
	Scope       ActionScope
	Label       string
	Description string
	Danger      bool
}

type ContextInfo struct {
	Tag     processdomain.ContextTag
	Label   string
	Details map[string]string
}

type Provider interface {
	Name() string
	Supports(proc processdomain.Info) bool
	Detect(ctx context.Context, proc processdomain.Info) []ContextInfo
	Actions(ctx context.Context, proc processdomain.Info) []Action
	// Execute runs actionID, returning the reclaimed bytes estimate
	// (0 when the action frees no measurable memory).
	Execute(ctx context.Context, actionID string, proc processdomain.Info) (uint64, error)
}

// CacheTarget describes one safely-cleanable cache unit of a process.
//
// Only non-disruptive cleanups qualify: the owning service must keep running
// normally after the cleanup (e.g. JVM garbage collection, closing blank
// browser tabs). Destructive or disk-oriented operations such as container
// removal or image pruning are intentionally not cache targets.
type CacheTarget struct {
	// Source names the owning service in UI language (e.g. "JVM").
	Source string
	// ActionID is the provider action that performs the cleanup.
	ActionID string
	// Label is a short human-readable description of what will be cleaned.
	Label string
	// Detail carries disambiguating info (e.g. "PID 1234", "porta 9222").
	Detail string
	// Proc is the process the cleanup acts on.
	Proc processdomain.Info
}

// CacheScanner is an optional Provider extension reporting the safely-cleanable
// cache units of a process. Detection must be pure (no I/O) so previews can be
// rendered synchronously before the user confirms.
type CacheScanner interface {
	CacheTargets(proc processdomain.Info) []CacheTarget
}

// blankTabLister is an optional Provider extension listing closable blank
// tabs. Unlike CacheTargets it may perform I/O, so callers must run it
// asynchronously and keep the confirmation usable while it loads.
type blankTabLister interface {
	Provider
	BlankTabs(ctx context.Context, proc processdomain.Info) ([]BlankTab, error)
}

type EntitySource interface {
	DiscoverEntities(ctx context.Context) []processdomain.Info
}

type Registry struct {
	providers []Provider
}

func NewRegistry(providers ...Provider) *Registry {
	r := &Registry{}
	for _, p := range providers {
		r.Register(p)
	}
	return r
}

func (r *Registry) Register(p Provider) {
	if p != nil {
		r.providers = append(r.providers, p)
	}
}

func (r *Registry) Providers() []Provider {
	return append([]Provider(nil), r.providers...)
}

func (r *Registry) Detect(ctx context.Context, proc processdomain.Info) []ContextInfo {
	var contexts []ContextInfo
	for _, p := range r.providers {
		if ctx.Err() != nil {
			break
		}
		if p.Supports(proc) {
			contexts = append(contexts, p.Detect(ctx, proc)...)
		}
	}
	return contexts
}

func (r *Registry) Actions(ctx context.Context, proc processdomain.Info) []Action {
	var actions []Action
	for _, p := range r.providers {
		if ctx.Err() != nil {
			break
		}
		if p.Supports(proc) {
			actions = append(actions, p.Actions(ctx, proc)...)
		}
	}
	return actions
}

func (r *Registry) CacheTargets(proc processdomain.Info) []CacheTarget {
	if r == nil {
		return nil
	}
	var targets []CacheTarget
	for _, p := range r.providers {
		scanner, ok := p.(CacheScanner)
		if !ok || !p.Supports(proc) {
			continue
		}
		targets = append(targets, scanner.CacheTargets(proc)...)
	}
	return targets
}

func (r *Registry) BlankTabs(ctx context.Context, proc processdomain.Info) ([]BlankTab, error) {
	if r == nil {
		return nil, ErrIncompatibleAction
	}
	for _, p := range r.providers {
		lister, ok := p.(blankTabLister)
		if !ok || !p.Supports(proc) {
			continue
		}
		return lister.BlankTabs(ctx, proc)
	}
	return nil, ErrIncompatibleAction
}

func (r *Registry) SupportsAction(ctx context.Context, actionID string, proc processdomain.Info) bool {
	for _, p := range r.providers {
		if p.Supports(proc) {
			for _, a := range p.Actions(ctx, proc) {
				if a.ID == actionID {
					return true
				}
			}
		}
	}
	return false
}

func (r *Registry) Execute(ctx context.Context, actionID string, proc processdomain.Info) (uint64, error) {
	for _, p := range r.providers {
		if p.Supports(proc) {
			for _, a := range p.Actions(ctx, proc) {
				if a.ID == actionID {
					return p.Execute(ctx, actionID, proc)
				}
			}
		}
	}
	return 0, ErrIncompatibleAction
}

func (r *Registry) DiscoverEntities(ctx context.Context) []processdomain.Info {
	var entities []processdomain.Info
	for _, p := range r.providers {
		if ctx.Err() != nil {
			break
		}
		if source, ok := p.(EntitySource); ok {
			entities = append(entities, source.DiscoverEntities(ctx)...)
		}
	}
	return entities
}
