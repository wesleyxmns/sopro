package process

import (
	"fmt"
	"strings"
)

type RiskPolicyConfig struct {
	CriticalPIDMax   int32
	CriticalCommands []string
	WarningCommands  []string
}

type RiskPolicy struct {
	criticalPIDMax int32
	critical       map[string]struct{}
	warning        map[string]struct{}
}

func DefaultRiskPolicyConfig() RiskPolicyConfig {
	return RiskPolicyConfig{
		CriticalPIDMax: 100,
		CriticalCommands: []string{
			"systemd", "init", "gnome-shell", "kwin", "kwin_wayland", "sshd",
			"system", "registry", "smss.exe", "csrss.exe", "wininit.exe", "services.exe", "lsass.exe",
		},
	}
}

func NewRiskPolicy(config RiskPolicyConfig) (RiskPolicy, error) {
	if config.CriticalPIDMax < 0 {
		return RiskPolicy{}, fmt.Errorf("critical PID maximum must be non-negative")
	}
	return RiskPolicy{
		criticalPIDMax: config.CriticalPIDMax,
		critical:       commandSet(config.CriticalCommands),
		warning:        commandSet(config.WarningCommands),
	}, nil
}

func DefaultRiskPolicy() RiskPolicy {
	policy, _ := NewRiskPolicy(DefaultRiskPolicyConfig())
	return policy
}

func (p RiskPolicy) Classify(info Info) Risk {
	command := normalizeCommand(info.Command)
	if p.criticalPIDMax > 0 && info.PID <= p.criticalPIDMax {
		return RiskCritical
	}
	if _, ok := p.critical[command]; ok {
		return RiskCritical
	}
	if _, ok := p.warning[command]; ok {
		return RiskWarning
	}
	return RiskOK
}

func commandSet(commands []string) map[string]struct{} {
	set := make(map[string]struct{}, len(commands))
	for _, command := range commands {
		if normalized := normalizeCommand(command); normalized != "" {
			set[normalized] = struct{}{}
		}
	}
	return set
}

func normalizeCommand(command string) string {
	return strings.ToLower(strings.TrimSpace(command))
}
