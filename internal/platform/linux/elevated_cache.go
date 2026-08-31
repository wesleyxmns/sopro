//go:build linux

package linux

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/wesleyxmns/sopro/internal/app"
)

const PrivilegedCleanCacheCommand = "__sopro-clean-cache"

type commandRunner func(context.Context, string, ...string) ([]byte, error)

type ElevatedCacheCleaner struct {
	manager    *Manager
	euid       func() int
	executable func() (string, error)
	lookPath   func(string) (string, error)
	run        commandRunner
}

var _ app.CacheCleaner = (*ElevatedCacheCleaner)(nil)

func NewElevatedCacheCleaner(manager *Manager) *ElevatedCacheCleaner {
	return &ElevatedCacheCleaner{
		manager:    manager,
		euid:       os.Geteuid,
		executable: os.Executable,
		lookPath:   exec.LookPath,
		run: func(ctx context.Context, name string, args ...string) ([]byte, error) {
			return exec.CommandContext(ctx, name, args...).Output()
		},
	}
}

func (c *ElevatedCacheCleaner) Available() bool {
	if c == nil || c.manager == nil {
		return false
	}
	if c.euid() == 0 {
		return true
	}
	_, err := c.lookPath("pkexec")
	return err == nil
}

func (c *ElevatedCacheCleaner) CleanCache(ctx context.Context) (uint64, error) {
	if c == nil || c.manager == nil {
		return 0, app.ErrUnsupported
	}
	if c.euid() == 0 {
		return c.manager.CleanCache(ctx)
	}

	pkexec, err := c.lookPath("pkexec")
	if err != nil {
		return 0, fmt.Errorf("%w: pkexec não está disponível", os.ErrPermission)
	}
	executable, err := c.executable()
	if err != nil {
		return 0, fmt.Errorf("localizar executável do Sopro: %w", err)
	}
	output, err := c.run(ctx, pkexec, executable, PrivilegedCleanCacheCommand)
	if err != nil {
		return 0, fmt.Errorf("elevar limpeza de cache: %w", err)
	}
	reclaimed, err := strconv.ParseUint(strings.TrimSpace(string(output)), 10, 64)
	if err != nil {
		return 0, fmt.Errorf("resposta inválida do helper privilegiado: %w", err)
	}
	return reclaimed, nil
}

type RuntimeCapabilities struct {
	manager *Manager
	cleaner *ElevatedCacheCleaner
}

var _ app.CapabilityProvider = (*RuntimeCapabilities)(nil)

func NewRuntimeCapabilities(manager *Manager, cleaner *ElevatedCacheCleaner) *RuntimeCapabilities {
	return &RuntimeCapabilities{manager: manager, cleaner: cleaner}
}

func (c *RuntimeCapabilities) Capabilities() app.Capabilities {
	if c == nil || c.manager == nil {
		return app.Capabilities{}
	}
	capabilities := c.manager.Capabilities()
	capabilities.CanCleanCache = c.cleaner != nil && c.cleaner.Available()
	return capabilities
}

func IsElevationUnavailable(err error) bool {
	return errors.Is(err, os.ErrPermission)
}
