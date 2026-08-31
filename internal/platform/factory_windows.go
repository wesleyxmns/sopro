//go:build windows

package platform

import (
	"github.com/wesleyxmns/sopro/internal/app"
	windowsplatform "github.com/wesleyxmns/sopro/internal/platform/windows"
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
