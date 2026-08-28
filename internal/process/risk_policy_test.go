package process

import "testing"

func TestRiskPolicyClassifiesPIDAndCommands(t *testing.T) {
	policy, err := NewRiskPolicy(RiskPolicyConfig{
		CriticalPIDMax:   10,
		CriticalCommands: []string{"kernel"},
		WarningCommands:  []string{"database"},
	})
	if err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name string
		info Info
		want Risk
	}{
		{"critical PID", Info{Identity: Identity{PID: 5}, Command: "worker"}, RiskCritical},
		{"critical command", Info{Identity: Identity{PID: 50}, Command: " KERNEL "}, RiskCritical},
		{"warning command", Info{Identity: Identity{PID: 50}, Command: "Database"}, RiskWarning},
		{"ordinary process", Info{Identity: Identity{PID: 50}, Command: "editor"}, RiskOK},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := policy.Classify(test.info); got != test.want {
				t.Fatalf("Classify() = %q; want %q", got, test.want)
			}
		})
	}
}

func TestRiskPolicyCriticalCommandTakesPrecedence(t *testing.T) {
	policy, err := NewRiskPolicy(RiskPolicyConfig{
		CriticalCommands: []string{"shared"},
		WarningCommands:  []string{"shared"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := policy.Classify(Info{Identity: Identity{PID: 50}, Command: "shared"}); got != RiskCritical {
		t.Fatalf("Classify() = %q; want critical", got)
	}
}

func TestRiskPolicyRejectsNegativePIDMaximum(t *testing.T) {
	if _, err := NewRiskPolicy(RiskPolicyConfig{CriticalPIDMax: -1}); err == nil {
		t.Fatal("expected negative critical PID maximum to fail")
	}
}
