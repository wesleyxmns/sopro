//go:build linux

package platform

import (
	"github.com/wesleyxmns/sopro/internal/app"
	linuxplatform "github.com/wesleyxmns/sopro/internal/platform/linux"
)

func New() app.Dependencies {
	manager := linuxplatform.New()
	cache := linuxplatform.NewElevatedCacheCleaner(manager)
	return app.Dependencies{
		Snapshots:     manager,
		Processes:     manager,
		ProcessWaiter: manager,
		Cache:         cache,
		Capabilities:  linuxplatform.NewRuntimeCapabilities(manager, cache),
	}
}
