package control

import (
	"time"

	processdomain "github.com/wesleyxmns/sopro/internal/process"
)

type Action string

const (
	ActionTerminate          Action = "terminate"
	ActionKill               Action = "kill"
	ActionPause              Action = "pause"
	ActionResume             Action = "resume"
	ActionClean              Action = "clean-cache"
	ActionDockerStop         Action = "docker.stop"
	ActionDockerStart        Action = "docker.start"
	ActionDockerRestart      Action = "docker.restart"
	ActionDockerPause        Action = "docker.pause"
	ActionCDPCloseBlank      Action = "cdp.close_blank"
	ActionCDPDiscardInactive Action = "cdp.discard_inactive"
	ActionJVMRunGC           Action = "jvm.run_gc"
)

type Request struct {
	Action        Action
	Process       processdomain.Identity
	ContainerName string
	ContainerID   string
}

type Result struct {
	Action    Action
	Process   processdomain.Identity
	Reclaimed uint64
	Finished  time.Time
}
