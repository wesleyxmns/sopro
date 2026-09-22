package control

import (
	"time"

	processdomain "github.com/wesleyxmns/sopro/internal/process"
	"github.com/wesleyxmns/sopro/internal/provider"
)

type Action string

const (
	ActionTerminate     Action = "terminate"
	ActionKill          Action = "kill"
	ActionPause         Action = "pause"
	ActionResume        Action = "resume"
	ActionClean         Action = "clean-cache"
	ActionCleanAll      Action = "clean-cache-all"
	ActionDockerStop    Action = "docker.stop"
	ActionDockerStart   Action = "docker.start"
	ActionDockerRestart Action = "docker.restart"
	ActionDockerPause   Action = "docker.pause"
	ActionCDPCloseBlank Action = "cdp.close_blank"
	ActionJVMRunGC      Action = "jvm.run_gc"
	ActionGitStatus     Action = "git.status"
	ActionGitFetch      Action = "git.fetch"
)

type Request struct {
	Action        Action
	Process       processdomain.Identity
	ContainerName string
	ContainerID   string
	// Target carries the full selected process for contextual actions, so
	// providers can resolve ports, commands and working directories.
	Target processdomain.Info
	// Targets carries the previewed cache units for ActionCleanAll.
	Targets []provider.CacheTarget
}

type Result struct {
	Action    Action
	Process   processdomain.Identity
	Reclaimed uint64
	Finished  time.Time
	// Succeeded and Failed count per-target outcomes of ActionCleanAll.
	Succeeded int
	Failed    int
}
