package provider

import (
	"context"
	"errors"
	"regexp"

	processdomain "sopro/internal/process"
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
	Execute(ctx context.Context, actionID string, proc processdomain.Info) error
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

func (r *Registry) Execute(ctx context.Context, actionID string, proc processdomain.Info) error {
	for _, p := range r.providers {
		if p.Supports(proc) {
			for _, a := range p.Actions(ctx, proc) {
				if a.ID == actionID {
					return p.Execute(ctx, actionID, proc)
				}
			}
		}
	}
	return ErrIncompatibleAction
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
