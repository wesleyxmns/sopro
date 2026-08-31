package platformtest

import (
	"context"
	"errors"
	"os"
	"testing"

	"github.com/wesleyxmns/sopro/internal/app"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
)

type Adapter interface {
	app.SnapshotSource
	app.ProcessController
	app.CacheCleaner
	app.CapabilityProvider
}

// VerifyContract checks behavior shared by every platform adapter without
// executing any operation the adapter advertises as supported.
func VerifyContract(t *testing.T, adapter Adapter, platform string) {
	t.Helper()

	capabilities := adapter.Capabilities()
	if capabilities.Platform != platform {
		t.Fatalf("platform = %q; want %q", capabilities.Platform, platform)
	}
	if capabilities.CanPause != capabilities.CanResume {
		t.Fatalf("pause/resume capabilities must be paired: %+v", capabilities)
	}
	if _, err := adapter.Snapshot(context.Background(), -1); err == nil {
		t.Fatal("negative snapshot limit must fail")
	}

	id := processdomain.Identity{PID: 1}
	assertUnsupported := func(name string, supported bool, invoke func() error) {
		t.Helper()
		if supported {
			return
		}
		if err := invoke(); !errors.Is(err, app.ErrUnsupported) {
			t.Fatalf("%s error = %v; want ErrUnsupported", name, err)
		}
	}
	assertUnsupported("terminate", capabilities.CanTerminate, func() error {
		return adapter.Terminate(context.Background(), id)
	})
	assertUnsupported("kill", capabilities.CanKill, func() error {
		return adapter.Kill(context.Background(), id)
	})
	assertUnsupported("pause", capabilities.CanPause, func() error {
		return adapter.Pause(context.Background(), id)
	})
	assertUnsupported("resume", capabilities.CanResume, func() error {
		return adapter.Resume(context.Background(), id)
	})
	if !capabilities.CanCleanCache {
		_, err := adapter.CleanCache(context.Background())
		if !errors.Is(err, app.ErrUnsupported) && !errors.Is(err, os.ErrPermission) {
			t.Fatalf("clean-cache error = %v; want unsupported or permission denied", err)
		}
	}
}
