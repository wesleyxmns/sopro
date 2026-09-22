package tui

import (
	"time"

	"github.com/wesleyxmns/sopro/internal/app"
	"github.com/wesleyxmns/sopro/internal/control"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
	"github.com/wesleyxmns/sopro/internal/provider"
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

type updateCheckSource uint8

const (
	updateCheckCache updateCheckSource = iota
	updateCheckBackground
	updateCheckManual
)

type updateCheckedMsg struct {
	release *updater.ReleaseInfo
	isNew   bool
	err     error
	source  updateCheckSource
}

type updateAppliedMsg struct {
	release *updater.ReleaseInfo
	err     error
}

type restartMsg struct{}

type blankTabsLoadedMsg struct {
	proc processdomain.Identity
	tabs []provider.BlankTab
	err  error
}
