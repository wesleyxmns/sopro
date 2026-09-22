//go:build windows

package windows

import (
	"golang.org/x/sys/windows"
)

var emptyWorkingSetProc = windows.NewLazySystemDLL("kernel32.dll").NewProc("EmptyWorkingSet")

// trimWorkingSet asks Windows to move the process working set to standby.
// Best-effort: any failure (access denied, vanished PID) is silently skipped.
func trimWorkingSet(pid int32) {
	if pid <= 0 {
		return
	}
	handle, err := windows.OpenProcess(
		windows.PROCESS_SET_QUOTA|windows.PROCESS_QUERY_INFORMATION,
		false,
		uint32(pid),
	)
	if err != nil {
		return
	}
	defer windows.CloseHandle(handle)
	_, _, _ = emptyWorkingSetProc.Call(uintptr(handle))
}
