//go:build linux

package linux

import (
	"context"
	"errors"
	"os"
	"reflect"
	"testing"
)

func TestElevatedCacheCleanerRunsOnlyFixedHelperCommand(t *testing.T) {
	cleaner := NewElevatedCacheCleaner(New())
	cleaner.euid = func() int { return 1000 }
	cleaner.executable = func() (string, error) { return "/opt/sopro", nil }
	cleaner.lookPath = func(name string) (string, error) {
		if name != "pkexec" {
			t.Fatalf("lookup = %q; want pkexec", name)
		}
		return "/usr/bin/pkexec", nil
	}
	var got []string
	cleaner.run = func(_ context.Context, name string, args ...string) ([]byte, error) {
		got = append([]string{name}, args...)
		return []byte("4096\n"), nil
	}

	reclaimed, err := cleaner.CleanCache(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"/usr/bin/pkexec", "/opt/sopro", PrivilegedCleanCacheCommand}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("command = %#v; want %#v", got, want)
	}
	if reclaimed != 4096 {
		t.Fatalf("reclaimed = %d; want 4096", reclaimed)
	}
}

func TestElevatedCacheCleanerRequiresPkexec(t *testing.T) {
	cleaner := NewElevatedCacheCleaner(New())
	cleaner.euid = func() int { return 1000 }
	cleaner.lookPath = func(string) (string, error) { return "", errors.New("not found") }

	if _, err := cleaner.CleanCache(context.Background()); !errors.Is(err, os.ErrPermission) {
		t.Fatalf("error = %v; want permission error", err)
	}
}

func TestRuntimeCapabilitiesAdvertisePointElevation(t *testing.T) {
	manager := New()
	cleaner := NewElevatedCacheCleaner(manager)
	cleaner.euid = func() int { return 1000 }
	cleaner.lookPath = func(string) (string, error) { return "/usr/bin/pkexec", nil }

	capabilities := NewRuntimeCapabilities(manager, cleaner).Capabilities()
	if capabilities.Elevated || !capabilities.CanCleanCache {
		t.Fatalf("unexpected capabilities: %+v", capabilities)
	}
}
