//go:build windows

package updater

import (
	"os"
	"os/exec"
)

// Restart spawns the updated binary and exits the current process. Windows
// cannot replace the running process image, so this function only returns
// when the respawn fails.
func Restart() error {
	executable, err := os.Executable()
	if err != nil {
		return err
	}
	child := exec.Command(executable, os.Args[1:]...)
	child.Stdin, child.Stdout, child.Stderr = os.Stdin, os.Stdout, os.Stderr
	if err := child.Start(); err != nil {
		return err
	}
	os.Exit(0)
	return nil
}
