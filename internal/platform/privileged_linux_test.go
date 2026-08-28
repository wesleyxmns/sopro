//go:build linux

package platform

import (
	"context"
	"errors"
	"os"
	"testing"

	linuxplatform "sopro/internal/platform/linux"
)

func TestPrivilegedHelperIgnoresNormalInvocation(t *testing.T) {
	handled, _, err := RunPrivilegedHelper(context.Background(), []string{"--theme", "dark"})
	if err != nil || handled {
		t.Fatalf("handled/error = %v/%v; want false/nil", handled, err)
	}
}

func TestPrivilegedHelperRejectsUnelevatedInvocation(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("running as root")
	}
	handled, _, err := RunPrivilegedHelper(context.Background(), []string{linuxplatform.PrivilegedCleanCacheCommand})
	if !handled || !errors.Is(err, os.ErrPermission) {
		t.Fatalf("handled/error = %v/%v; want true/permission", handled, err)
	}
}
