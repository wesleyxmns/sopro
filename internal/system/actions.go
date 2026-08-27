package system

import (
	"os"
	"os/exec"
	"syscall"
)

func IsRoot() bool {
	return os.Geteuid() == 0
}

func KillProcess(pid int32) error {
	process, err := os.FindProcess(int(pid))
	if err != nil {
		return err
	}
	return process.Signal(syscall.SIGKILL)
}

func DropCaches() error {
	if !IsRoot() {
		return os.ErrPermission
	}
	cmd := exec.Command("sh", "-c", "sync; echo 3 > /proc/sys/vm/drop_caches")
	return cmd.Run()
}
