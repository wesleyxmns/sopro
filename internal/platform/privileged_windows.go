//go:build windows

package platform

import "context"

func RunPrivilegedHelper(context.Context, []string) (bool, uint64, error) {
	return false, 0, nil
}
