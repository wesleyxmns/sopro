package tui

import (
	"context"
	"time"

	"sopro/internal/app"
	"sopro/internal/control"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	refreshInterval  = 2 * time.Second
	operationTimeout = 4 * time.Second
	processLimit     = 200
	splashDuration   = 900 * time.Millisecond
)

func scheduleTick() tea.Cmd {
	return tea.Tick(refreshInterval, func(t time.Time) tea.Msg { return tickMsg(t) })
}

func finishSplashCmd() tea.Cmd {
	return tea.Tick(splashDuration, func(time.Time) tea.Msg { return splashFinishedMsg{} })
}

func loadSnapshotCmd(service *app.Service) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()
		snapshot, err := service.Snapshot(ctx, processLimit)
		return snapshotLoadedMsg{snapshot: snapshot, err: err}
	}
}

func executeActionCmd(service *app.Service, request control.Request) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()

		result := control.Result{Action: request.Action, Process: request.Process}
		var err error
		switch request.Action {
		case control.ActionTerminate:
			err = service.Terminate(ctx, request.Process)
		case control.ActionKill:
			err = service.Kill(ctx, request.Process)
		case control.ActionPause:
			err = service.Pause(ctx, request.Process)
		case control.ActionResume:
			err = service.Resume(ctx, request.Process)
		case control.ActionClean:
			result.Reclaimed, err = service.CleanCache(ctx)
		default:
			err = app.ErrUnsupported
		}
		result.Finished = time.Now()
		return actionFinishedMsg{result: result, err: err}
	}
}
