package tui

import (
	"time"

	"github.com/wesleyxmns/sopro/internal/app"
	"github.com/wesleyxmns/sopro/internal/control"
	"github.com/wesleyxmns/sopro/internal/updater"
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

type updateCheckedMsg struct {
	release *updater.ReleaseInfo
	isNew   bool
	err     error
}

type updateAppliedMsg struct {
	release *updater.ReleaseInfo
	err     error
}

