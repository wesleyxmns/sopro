//go:build linux

package platform

import (
	"context"
	"fmt"
	"os"

	linuxplatform "github.com/wesleyxmns/sopro/internal/platform/linux"
)

func RunPrivilegedHelper(ctx context.Context, args []string) (bool, uint64, error) {
	if len(args) != 1 || args[0] != linuxplatform.PrivilegedCleanCacheCommand {
		return false, 0, nil
	}
	if os.Geteuid() != 0 {
		return true, 0, fmt.Errorf("helper de cache exige elevação: %w", os.ErrPermission)
	}
	reclaimed, err := linuxplatform.New().CleanCache(ctx)
	return true, reclaimed, err
}
