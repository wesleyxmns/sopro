package process

import (
	"testing"
	"time"
)

func TestLeakGuardDetectsSustainedGrowth(t *testing.T) {
	guard := NewLeakGuard(LeakGuardConfig{Window: time.Minute, MinSamples: 4, MinGrowthBytesPerSecond: 100, MinConfidence: .9})
	id := Identity{PID: 42, StartedAt: 1}
	start := time.Unix(100, 0)
	var assessment LeakAssessment
	for index := 0; index < 4; index++ {
		assessment = guard.Observe(start.Add(time.Duration(index)*time.Second), []Info{{Identity: id, MemoryBytes: uint64(1000 + index*200)}})[id]
	}
	if assessment.Status != LeakSuspected || assessment.Confidence < .9 {
		t.Fatalf("unexpected assessment: %+v", assessment)
	}
}

func TestLeakGuardWaitsForEnoughSamples(t *testing.T) {
	guard := NewLeakGuard(LeakGuardConfig{MinSamples: 4})
	id := Identity{PID: 42}
	assessment := guard.Observe(time.Now(), []Info{{Identity: id, MemoryBytes: 1000}})[id]
	if assessment.Status != LeakObserving {
		t.Fatalf("status = %q; want observing", assessment.Status)
	}
}

func TestLeakGuardShortRampIsNotALeak(t *testing.T) {
	guard := NewLeakGuard(DefaultLeakGuardConfig())
	id := Identity{PID: 42, StartedAt: 1}
	start := time.Unix(100, 0)
	var assessment LeakAssessment
	// 8 samples of smooth ~1MB/s growth: normal GC garbage cadence between
	// collections, not a leak. Must not be flagged.
	for index := 0; index < 8; index++ {
		at := start.Add(time.Duration(index*2) * time.Second)
		assessment = guard.Observe(at, []Info{{Identity: id, MemoryBytes: uint64(10_000_000 + index*2_000_000)}})[id]
	}
	if assessment.Status == LeakSuspected {
		t.Fatalf("short ramp flagged as leak: %+v", assessment)
	}
}

func TestLeakGuardDetectsLongSteadyRamp(t *testing.T) {
	guard := NewLeakGuard(DefaultLeakGuardConfig())
	id := Identity{PID: 42, StartedAt: 1}
	start := time.Unix(100, 0)
	var assessment LeakAssessment
	for index := 0; index < 40; index++ {
		at := start.Add(time.Duration(index*2) * time.Second)
		assessment = guard.Observe(at, []Info{{Identity: id, MemoryBytes: uint64(10_000_000 + index*2_000_000)}})[id]
	}
	if assessment.Status != LeakSuspected {
		t.Fatalf("status = %q; want suspected for sustained growth", assessment.Status)
	}
}

func TestLeakGuardIgnoresGCSawtooth(t *testing.T) {
	guard := NewLeakGuard(DefaultLeakGuardConfig())
	id := Identity{PID: 42, StartedAt: 1}
	start := time.Unix(100, 0)
	var assessment LeakAssessment
	// Ramps that periodically drop back to baseline (GC collections) are
	// healthy and must not be flagged, no matter how steep the ramps are.
	for index := 0; index < 40; index++ {
		at := start.Add(time.Duration(index*2) * time.Second)
		assessment = guard.Observe(at, []Info{{Identity: id, MemoryBytes: uint64(10_000_000 + (index%10)*2_000_000)}})[id]
	}
	if assessment.Status == LeakSuspected {
		t.Fatalf("GC sawtooth flagged as leak: %+v", assessment)
	}
}
