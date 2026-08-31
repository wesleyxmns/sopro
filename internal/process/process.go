package process

import "fmt"

type State string

const (
	StateUnknown State = "unknown"
	StateRunning State = "running"
	StatePaused  State = "paused"
	StateStopped State = "stopped"
)

type Risk string

const (
	RiskOK       Risk = "ok"
	RiskWarning  Risk = "warning"
	RiskCritical Risk = "critical"
)

// Identity prevents acting on a PID that has been reused by another process.
type Identity struct {
	PID       int32
	StartedAt int64
}

func (id Identity) Validate() error {
	if id.PID <= 0 {
		return fmt.Errorf("invalid PID: must be positive")
	}
	return nil
}

type Info struct {
	Identity
	ParentPID   int32
	User        string
	MemoryBytes uint64
	MemoryPct   float32
	CPUPct      float64
	Command     string
	CommandLine string
	Cwd         string
	State       State
	Risk        Risk
	Category    Category
	Contexts    []ContextTag
	ContainerID   string
	ContainerName string
	ImageName     string
	Reclaimable uint64
	Leak        LeakAssessment
}
