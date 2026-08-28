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
