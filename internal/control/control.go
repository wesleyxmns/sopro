package control

import (
	"time"

	processdomain "sopro/internal/process"
)

type Action string

const (
	ActionTerminate Action = "terminate"
	ActionKill      Action = "kill"
	ActionPause     Action = "pause"
	ActionResume    Action = "resume"
	ActionClean     Action = "clean-cache"
)

type Request struct {
	Action  Action
	Process processdomain.Identity
}

type Result struct {
	Action    Action
	Process   processdomain.Identity
	Reclaimed uint64
	Finished  time.Time
}
