package daemon

import (
	"context"
	"fmt"
	"time"

	"sopro/internal/app"
	processdomain "sopro/internal/process"
)

type Config struct {
	Interval            time.Duration
	ObserveOnly         bool
	PSISomeAvg10        float64
	PSIFullAvg10        float64
	MemoryUsagePct      float64
	SustainedDuration   time.Duration
	Cooldown            time.Duration
	AllowCacheClean     bool
}

func DefaultConfig() Config {
	return Config{
		Interval:          3 * time.Second,
		ObserveOnly:       true,
		PSISomeAvg10:      10.0,
		PSIFullAvg10:      1.0,
		MemoryUsagePct:    90.0,
		SustainedDuration: 15 * time.Second,
		Cooldown:          60 * time.Second,
		AllowCacheClean:   true,
	}
}

type Service interface {
	Snapshot(ctx context.Context, limit int) (app.Snapshot, error)
	CleanCache(ctx context.Context) (uint64, error)
	Terminate(ctx context.Context, id processdomain.Identity) error
}

type Notifier interface {
	Notify(decision Decision)
}

type Decision struct {
	Timestamp          time.Time
	UnderPressure      bool
	PressureDuration   time.Duration
	RecommendedAction  string
	TargetProcess      *processdomain.Identity
	Executed           bool
	Reason             string
	SuspectedProcesses []processdomain.Identity
}

type Daemon struct {
	service           Service
	config            Config
	notifier          Notifier
	pressureStartTime *time.Time
	lastActionTime    time.Time
}

func New(service Service, config Config, notifier ...Notifier) *Daemon {
	var n Notifier
	if len(notifier) > 0 {
		n = notifier[0]
	}
	return &Daemon{
		service:  service,
		config:   config,
		notifier: n,
	}
}

func (d *Daemon) Tick(ctx context.Context) (Decision, error) {
	now := time.Now()
	snapshot, err := d.service.Snapshot(ctx, 50)
	if err != nil {
		return Decision{Timestamp: now, Reason: fmt.Sprintf("falha no snapshot: %v", err)}, err
	}

	underPressure := d.isUnderPressure(snapshot)
	var pressureDuration time.Duration

	if underPressure {
		if d.pressureStartTime == nil {
			d.pressureStartTime = &now
		}
		pressureDuration = now.Sub(*d.pressureStartTime)
	} else {
		d.pressureStartTime = nil
	}

	var suspected []processdomain.Identity
	for _, proc := range snapshot.Processes {
		if proc.Leak.Status == processdomain.LeakSuspected {
			suspected = append(suspected, proc.Identity)
		}
	}

	decision := Decision{
		Timestamp:          now,
		UnderPressure:      underPressure,
		PressureDuration:   pressureDuration,
		SuspectedProcesses: suspected,
	}

	if !underPressure {
		decision.Reason = "pressão normal"
		d.notify(decision)
		return decision, nil
	}

	if pressureDuration < d.config.SustainedDuration {
		decision.Reason = fmt.Sprintf("pressão elevada por %v (aguardando limite de %v)", pressureDuration.Round(time.Second), d.config.SustainedDuration)
		d.notify(decision)
		return decision, nil
	}

	if !d.lastActionTime.IsZero() && now.Sub(d.lastActionTime) < d.config.Cooldown {
		remaining := (d.config.Cooldown - now.Sub(d.lastActionTime)).Round(time.Second)
		decision.Reason = fmt.Sprintf("em período de cooldown (%v restantes)", remaining)
		d.notify(decision)
		return decision, nil
	}

	if d.config.AllowCacheClean && snapshot.Memory.Reclaimable > 0 {
		decision.RecommendedAction = "clean-cache"
		if d.config.ObserveOnly {
			decision.Executed = false
			decision.Reason = "modo observação: limpeza de cache recomendada mas não executada"
		} else {
			reclaimed, cleanErr := d.service.CleanCache(ctx)
			if cleanErr != nil {
				decision.Executed = false
				decision.Reason = fmt.Sprintf("execução de limpeza de cache falhou: %v", cleanErr)
			} else {
				decision.Executed = true
				d.lastActionTime = now
				decision.Reason = fmt.Sprintf("limpeza de cache executada com sucesso (%d bytes recuperados)", reclaimed)
			}
		}
	} else {
		decision.Reason = "pressão sustentada sem ação de alívio disponível"
	}

	d.notify(decision)
	return decision, nil
}

func (d *Daemon) isUnderPressure(snapshot app.Snapshot) bool {
	pressure := snapshot.Memory.Pressure
	if pressure.Supported {
		if pressure.Some.Avg10 >= d.config.PSISomeAvg10 || pressure.Full.Avg10 >= d.config.PSIFullAvg10 {
			return true
		}
	}

	if snapshot.Memory.Total > 0 {
		usage := float64(snapshot.Memory.Used) / float64(snapshot.Memory.Total) * 100
		if usage >= d.config.MemoryUsagePct {
			return true
		}
	}
	return false
}

func (d *Daemon) notify(decision Decision) {
	if d.notifier != nil {
		d.notifier.Notify(decision)
	}
}

func (d *Daemon) Run(ctx context.Context) error {
	ticker := time.NewTicker(d.config.Interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if _, err := d.Tick(ctx); err != nil && ctx.Err() != nil {
				return err
			}
		}
	}
}
