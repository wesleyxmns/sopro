package system

type PlatformManager interface {
	GetMetrics() (SystemMetrics, error)
	GetProcesses(limit int) ([]ProcessInfo, error)
	KillProcess(pid int32) error
	PauseProcess(pid int32) error
	ResumeProcess(pid int32) error
	CleanSystemCache() (uint64, error)
}
