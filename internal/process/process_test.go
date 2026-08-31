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

func TestInfoContainerFields(t *testing.T) {
	info := Info{
		Identity:      Identity{PID: 100, StartedAt: 1},
		Command:       "postgres",
		ContainerID:   "acf8947e8c35",
		ContainerName: "sangati_postgres",
		ImageName:     "postgres:15-alpine",
	}
	if info.ContainerName != "sangati_postgres" || info.ContainerID != "acf8947e8c35" || info.ImageName != "postgres:15-alpine" {
		t.Fatalf("container fields mismatch: %+v", info)
	}
}

