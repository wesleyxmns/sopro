package tui

import (
	"time"

	"sopro/internal/app"
	"sopro/internal/control"
)

type tickMsg time.Time

type splashFinishedMsg struct{}

type snapshotLoadedMsg struct {
	snapshot app.Snapshot
	err      error
}

type actionFinishedMsg struct {
	result control.Result
	err    error
}
