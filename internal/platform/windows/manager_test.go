//go:build windows

package windows

import (
	"context"
	"testing"

	"sopro/internal/platform/platformtest"
	processdomain "sopro/internal/process"
)

func TestManagerPlatformContract(t *testing.T) {
	platformtest.VerifyContract(t, New(), "windows")
}

func TestManagerCapabilities(t *testing.T) {
	capabilities := New().Capabilities()
	if capabilities.Platform != "windows" || !capabilities.CanKill {
		t.Fatalf("unexpected Windows capabilities: %+v", capabilities)
	}
	if capabilities.CanTerminate || capabilities.CanPause || capabilities.CanResume || capabilities.CanCleanCache {
		t.Fatalf("Windows manager advertised unsupported capabilities: %+v", capabilities)
	}
}

func TestManagerRejectsInvalidKillIdentity(t *testing.T) {
	if err := New().Kill(context.Background(), processdomain.Identity{}); err == nil {
		t.Fatal("expected invalid process identity to fail")
	}
}

func TestManagerRejectsNegativeSnapshotLimit(t *testing.T) {
	if _, err := New().Snapshot(context.Background(), -1); err == nil {
		t.Fatal("expected negative snapshot limit to fail")
	}
}
