package app

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/wesleyxmns/sopro/internal/audit"
	"github.com/wesleyxmns/sopro/internal/memory"
	processdomain "github.com/wesleyxmns/sopro/internal/process"
	"github.com/wesleyxmns/sopro/internal/provider"
)

var ErrUnsupported = errors.New("operation is not supported on this platform")

type ActionError struct {
	Operation error
	Audit     error
}

func (e *ActionError) Error() string {
	if e.Operation == nil {
		return fmt.Sprintf("operation succeeded but audit failed: %v", e.Audit)
	}
	return fmt.Sprintf("operation failed: %v; audit failed: %v", e.Operation, e.Audit)
}

func (e *ActionError) Unwrap() error {
	if e.Operation != nil {
		return e.Operation
	}
	return e.Audit
}

type Snapshot struct {
	Memory    memory.Snapshot
	Processes []processdomain.Info
	TakenAt   time.Time
}

type Capabilities struct {
	Platform      string
	Elevated      bool
	CanTerminate  bool
	CanKill       bool
	CanPause      bool
	CanResume     bool
	CanCleanCache bool
}

type SnapshotSource interface {
	Snapshot(context.Context, int) (Snapshot, error)
}

type ProcessController interface {
	Terminate(context.Context, processdomain.Identity) error
	Kill(context.Context, processdomain.Identity) error
	Pause(context.Context, processdomain.Identity) error
	Resume(context.Context, processdomain.Identity) error
}

type ProcessWaiter interface {
	WaitForExit(context.Context, processdomain.Identity) error
}

type CacheCleaner interface {
	CleanCache(context.Context) (uint64, error)
}

type CapabilityProvider interface {
	Capabilities() Capabilities
}

type Dependencies struct {
	Snapshots     SnapshotSource
	Processes     ProcessController
	ProcessWaiter ProcessWaiter
	Cache         CacheCleaner
	Capabilities  CapabilityProvider
	Audit         audit.Recorder
}

type Service struct {
	deps             Dependencies
	riskPolicy       processdomain.RiskPolicy
	classifier       processdomain.Classifier
	terminationGrace time.Duration
	leakGuard        *processdomain.LeakGuard
	providers        *provider.Registry
}

type ServiceOption func(*Service)

func WithRiskPolicy(policy processdomain.RiskPolicy) ServiceOption {
	return func(service *Service) {
		service.riskPolicy = policy
	}
}

func WithTerminationGracePeriod(period time.Duration) ServiceOption {
	return func(service *Service) {
		if period > 0 {
			service.terminationGrace = period
		}
	}
}

func WithProviderRegistry(registry *provider.Registry) ServiceOption {
	return func(service *Service) {
		service.providers = registry
	}
}

func NewService(deps Dependencies, options ...ServiceOption) *Service {
	service := &Service{
		deps:             deps,
		riskPolicy:       processdomain.DefaultRiskPolicy(),
		classifier:       processdomain.DefaultClassifier(),
		terminationGrace: 2 * time.Second,
		leakGuard:        processdomain.NewLeakGuard(processdomain.DefaultLeakGuardConfig()),
	}
	for _, option := range options {
		option(service)
	}
	return service
}

func (s *Service) Snapshot(ctx context.Context, limit int) (Snapshot, error) {
	if s == nil || s.deps.Snapshots == nil {
		return Snapshot{}, ErrUnsupported
	}
	snapshot, err := s.deps.Snapshots.Snapshot(ctx, limit)
	if err != nil {
		return Snapshot{}, err
	}
	for index := range snapshot.Processes {
		snapshot.Processes[index].CommandLine = provider.MaskSensitiveArgs(snapshot.Processes[index].CommandLine)
		snapshot.Processes[index].Risk = s.riskPolicy.Classify(snapshot.Processes[index])
		snapshot.Processes[index].Category = s.classifier.Classify(snapshot.Processes[index])
		if s.providers != nil {
			contexts := s.providers.Detect(ctx, snapshot.Processes[index])
			for _, c := range contexts {
				snapshot.Processes[index].Contexts = append(snapshot.Processes[index].Contexts, c.Tag)
				if c.Details != nil {
					if name := c.Details["container_name"]; name != "" {
						snapshot.Processes[index].ContainerName = name
						snapshot.Processes[index].Category = processdomain.CategoryContainer
					}
					if id := c.Details["container_id"]; id != "" {
						snapshot.Processes[index].ContainerID = id
						snapshot.Processes[index].Category = processdomain.CategoryContainer
					}
					if img := c.Details["image"]; img != "" {
						snapshot.Processes[index].ImageName = img
					}
				}
			}
		}
	}
	if s.providers != nil {
		extraEntities := s.providers.DiscoverEntities(ctx)
		for _, entity := range extraEntities {
			entity.CommandLine = provider.MaskSensitiveArgs(entity.CommandLine)
			snapshot.Processes = append(snapshot.Processes, entity)
		}
	}

	assessments := s.leakGuard.Observe(snapshot.TakenAt, snapshot.Processes)
	for index := range snapshot.Processes {
		snapshot.Processes[index].Leak = assessments[snapshot.Processes[index].Identity]
	}
	return snapshot, nil
}

func (s *Service) AvailableActions(ctx context.Context, proc processdomain.Info) []provider.Action {
	if s == nil || s.providers == nil {
		return nil
	}
	return s.providers.Actions(ctx, proc)
}

func (s *Service) ContextualActions(ctx context.Context, proc processdomain.Info) []provider.Action {
	return s.AvailableActions(ctx, proc)
}

func (s *Service) ExecuteContextualAction(ctx context.Context, actionID string, proc processdomain.Info) (uint64, error) {
	if s == nil || s.providers == nil {
		return 0, ErrUnsupported
	}
	if !s.providers.SupportsAction(ctx, actionID, proc) {
		return 0, provider.ErrIncompatibleAction
	}
	started := time.Now()
	reclaimed, err := s.providers.Execute(ctx, actionID, proc)
	return reclaimed, s.finishAction(actionID, proc.Identity, started, err, false, reclaimed)
}

func (s *Service) Capabilities() Capabilities {
	if s == nil || s.deps.Capabilities == nil {
		return Capabilities{}
	}
	return s.deps.Capabilities.Capabilities()
}

func (s *Service) Terminate(ctx context.Context, id processdomain.Identity) error {
	started := time.Now()
	if s == nil || s.deps.Processes == nil {
		return ErrUnsupported
	}
	escalated, err := s.terminate(ctx, id)
	return s.finishAction("terminate", id, started, err, escalated, 0)
}

func (s *Service) terminate(ctx context.Context, id processdomain.Identity) (bool, error) {
	if err := s.deps.Processes.Terminate(ctx, id); err != nil {
		return false, err
	}
	if s.deps.ProcessWaiter == nil {
		return false, nil
	}
	waitCtx, cancel := context.WithTimeout(ctx, s.terminationGrace)
	defer cancel()
	err := s.deps.ProcessWaiter.WaitForExit(waitCtx, id)
	if err == nil {
		return false, nil
	}
	if ctx.Err() != nil {
		return false, ctx.Err()
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		return false, err
	}
	return true, s.deps.Processes.Kill(ctx, id)
}

func (s *Service) Kill(ctx context.Context, id processdomain.Identity) error {
	started := time.Now()
	if s == nil || s.deps.Processes == nil {
		return ErrUnsupported
	}
	err := s.deps.Processes.Kill(ctx, id)
	return s.finishAction("kill", id, started, err, false, 0)
}

func (s *Service) Pause(ctx context.Context, id processdomain.Identity) error {
	started := time.Now()
	if s == nil || s.deps.Processes == nil {
		return ErrUnsupported
	}
	err := s.deps.Processes.Pause(ctx, id)
	return s.finishAction("pause", id, started, err, false, 0)
}

func (s *Service) Resume(ctx context.Context, id processdomain.Identity) error {
	started := time.Now()
	if s == nil || s.deps.Processes == nil {
		return ErrUnsupported
	}
	err := s.deps.Processes.Resume(ctx, id)
	return s.finishAction("resume", id, started, err, false, 0)
}

func (s *Service) CleanCache(ctx context.Context) (uint64, error) {
	started := time.Now()
	if s == nil || s.deps.Cache == nil {
		return 0, ErrUnsupported
	}
	reclaimed, err := s.deps.Cache.CleanCache(ctx)
	return reclaimed, s.finishAction("clean-cache", processdomain.Identity{}, started, err, false, reclaimed)
}

// CacheTargets reports the safely-cleanable cache units of a single process.
func (s *Service) CacheTargets(proc processdomain.Info) []provider.CacheTarget {
	if s == nil || s.providers == nil {
		return nil
	}
	return s.providers.CacheTargets(proc)
}

// PreviewBlankTabs lists the tabs close_blank would close. It performs I/O
// and must be called asynchronously from the TUI.
func (s *Service) PreviewBlankTabs(ctx context.Context, proc processdomain.Info) ([]provider.BlankTab, error) {
	if s == nil || s.providers == nil {
		return nil, ErrUnsupported
	}
	return s.providers.BlankTabs(ctx, proc)
}

// PreviewCleanAll scans a snapshot for every safely-cleanable cache unit: the
// OS page cache first (when privileged and reclaimable), then one unit per
// provider target across all processes. Targets sharing the same action and
// detail (e.g. browser processes on the same CDP port) are listed once.
func (s *Service) PreviewCleanAll(snapshot Snapshot) []provider.CacheTarget {
	if s == nil {
		return nil
	}
	var targets []provider.CacheTarget
	if s.deps.Capabilities != nil && s.Capabilities().CanCleanCache && snapshot.Memory.Reclaimable > 0 {
		targets = append(targets, provider.CacheTarget{
			Source:   "SO",
			ActionID: "clean-cache",
			Label:    "Page cache, dentries e inodes",
			Detail:   "≈ " + memory.FormatBytes(snapshot.Memory.Reclaimable),
		})
	}
	if s.providers == nil {
		return targets
	}
	seen := make(map[string]struct{})
	for _, proc := range snapshot.Processes {
		for _, target := range s.providers.CacheTargets(proc) {
			// JVM details already carry the PID, so same-port browser
			// processes collapse into one target while JVMs stay per-PID.
			key := target.ActionID + "|" + target.Detail
			if _, duplicated := seen[key]; duplicated {
				continue
			}
			seen[key] = struct{}{}
			targets = append(targets, target)
		}
	}
	return targets
}

// CleanAllResult aggregates the outcome of cleaning every previewed target.
type CleanAllResult struct {
	ReclaimedBytes uint64
	Succeeded      int
	Failed         int
}

// CleanTargets cleans every given cache target, routing the OS page-cache
// target to the platform cleaner and the remaining targets to their provider
// actions. It returns an error only when no target could be cleaned, so
// partial cleanups still report the reclaimed bytes and per-target counts.
func (s *Service) CleanTargets(ctx context.Context, targets []provider.CacheTarget) (CleanAllResult, error) {
	started := time.Now()
	if s == nil {
		return CleanAllResult{}, ErrUnsupported
	}
	if len(targets) == 0 {
		return CleanAllResult{}, errors.New("no cleanable cache targets")
	}
	var result CleanAllResult
	var firstErr error
	for _, target := range targets {
		if err := ctx.Err(); err != nil {
			return result, s.finishAction("clean-cache-all", processdomain.Identity{}, started, err, false, result.ReclaimedBytes)
		}
		var err error
		if target.ActionID == "clean-cache" {
			var reclaimed uint64
			if s.deps.Cache == nil {
				err = ErrUnsupported
			} else {
				reclaimed, err = s.deps.Cache.CleanCache(ctx)
				result.ReclaimedBytes += reclaimed
			}
		} else if s.providers == nil || !s.providers.SupportsAction(ctx, target.ActionID, target.Proc) {
			err = provider.ErrIncompatibleAction
		} else {
			var reclaimed uint64
			reclaimed, err = s.providers.Execute(ctx, target.ActionID, target.Proc)
			result.ReclaimedBytes += reclaimed
		}
		if err != nil {
			result.Failed++
			if firstErr == nil {
				firstErr = fmt.Errorf("%s (%s): %w", target.Label, target.Source, err)
			}
			continue
		}
		result.Succeeded++
	}
	if result.Succeeded == 0 {
		return result, s.finishAction("clean-cache-all", processdomain.Identity{}, started, firstErr, false, result.ReclaimedBytes)
	}
	return result, s.finishAction("clean-cache-all", processdomain.Identity{}, started, nil, false, result.ReclaimedBytes)
}

func (s *Service) finishAction(action string, id processdomain.Identity, started time.Time, operationErr error, escalated bool, reclaimed uint64) error {
	if s.deps.Audit == nil {
		return operationErr
	}
	finished := time.Now()
	event := audit.Event{
		Action: action, PID: id.PID, ProcessStartedAt: id.StartedAt,
		StartedAt: started, FinishedAt: finished, Duration: finished.Sub(started),
		Success: operationErr == nil, Escalated: escalated, ReclaimedBytes: reclaimed,
	}
	if operationErr != nil {
		event.Error = operationErr.Error()
	}
	if auditErr := s.deps.Audit.Record(event); auditErr != nil {
		return &ActionError{Operation: operationErr, Audit: auditErr}
	}
	return operationErr
}
