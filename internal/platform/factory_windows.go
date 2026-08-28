//go:build windows

package platform

import (
	"sopro/internal/app"
	windowsplatform "sopro/internal/platform/windows"
)

func New() app.Dependencies {
	manager := windowsplatform.New()
	return app.Dependencies{
		Snapshots:    manager,
		Processes:    manager,
		Cache:        manager,
		Capabilities: manager,
	}
}
