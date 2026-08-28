package process

import "testing"

func TestIdentityValidate(t *testing.T) {
	if err := (Identity{}).Validate(); err == nil {
		t.Fatal("expected zero PID to be rejected")
	}
	if err := (Identity{PID: 42, StartedAt: 10}).Validate(); err != nil {
		t.Fatalf("expected valid identity, got %v", err)
	}
}
