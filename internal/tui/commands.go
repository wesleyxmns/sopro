package tui

import (
	"context"
	"fmt"
	"time"

	"github.com/wesleyxmns/sopro/internal/app"
	"github.com/wesleyxmns/sopro/internal/control"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
	"github.com/wesleyxmns/sopro/internal/updater"

	tea "github.com/charmbracelet/bubbletea"
)

const (
	refreshInterval  = 2 * time.Second
	operationTimeout = 4 * time.Second
	cleanAllTimeout  = 30 * time.Second
	restartDelay     = 1200 * time.Millisecond
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

func fetchBlankTabsCmd(service *app.Service, proc processdomain.Info) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()
		tabs, err := service.PreviewBlankTabs(ctx, proc)
		return blankTabsLoadedMsg{proc: proc.Identity, tabs: tabs, err: err}
	}
}

func executeActionCmd(service *app.Service, request control.Request) tea.Cmd {
	timeout := operationTimeout
	if request.Action == control.ActionCleanAll {
		timeout = cleanAllTimeout
	}
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
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
		case control.ActionCleanAll:
			var cleaned app.CleanAllResult
			cleaned, err = service.CleanTargets(ctx, request.Targets)
			result.Reclaimed = cleaned.ReclaimedBytes
			result.Succeeded = cleaned.Succeeded
			result.Failed = cleaned.Failed
		default:
			proc := request.Target
			proc.Identity = request.Process
			if request.ContainerName != "" {
				proc.ContainerName = request.ContainerName
			}
			if request.ContainerID != "" {
				proc.ContainerID = request.ContainerID
			}
			result.Reclaimed, err = service.ExecuteContextualAction(ctx, string(request.Action), proc)
		}
		result.Finished = time.Now()
		return actionFinishedMsg{result: result, err: err}
	}
}

func cachedUpdateCmd(currentVersion string) tea.Cmd {
	return func() tea.Msg {
		checker := updater.NewChecker()
		release, isNew, _, _ := checker.Cached(currentVersion)
		return updateCheckedMsg{release: release, isNew: isNew, source: updateCheckCache}
	}
}

func checkUpdateCmd(currentVersion string, source updateCheckSource) tea.Cmd {
	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), operationTimeout)
		defer cancel()
		checker := updater.NewChecker()
		release, isNew, err := checker.Check(ctx, currentVersion, true)
		return updateCheckedMsg{release: release, isNew: isNew, err: err, source: source}
	}
}

func applyUpdateCmd(release *updater.ReleaseInfo) tea.Cmd {
	executable, requiresElevation, err := updater.UpdateTarget()
	if err != nil {
		return func() tea.Msg { return updateAppliedMsg{release: release, err: err} }
	}
	if requiresElevation {
		process, processErr := updater.ElevatedCommand(executable, "update")
		if processErr != nil {
			return func() tea.Msg { return updateAppliedMsg{release: release, err: processErr} }
		}
		return tea.ExecProcess(process, func(runErr error) tea.Msg {
			if runErr != nil {
				runErr = fmt.Errorf("não foi possível concluir a atualização com permissão administrativa: %w", runErr)
			}
			return updateAppliedMsg{release: release, err: runErr}
		})
	}

	return func() tea.Msg {
		ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		applyErr := updater.Apply(ctx, release)
		return updateAppliedMsg{release: release, err: applyErr}
	}
}
