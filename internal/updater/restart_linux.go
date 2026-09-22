//go:build linux

package updater

import (
	"os"
	"syscall"
)

// Restart replaces the running process with the updated binary on disk.
// It only returns when the replacement fails.
func Restart() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	return syscall.Exec(executable, os.Args, os.Environ())
}
