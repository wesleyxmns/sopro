package tui

import (
	"time"

	"github.com/wesleyxmns/sopro/internal/app"
	"github.com/wesleyxmns/sopro/internal/control"
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
