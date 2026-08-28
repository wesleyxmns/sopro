package process

import (
	"math"
	"time"
)

type LeakStatus string

const (
	LeakObserving LeakStatus = "observing"
	LeakStable    LeakStatus = "stable"
	LeakSuspected LeakStatus = "suspected"
)

type LeakAssessment struct {
	Status               LeakStatus
	GrowthBytesPerSecond float64
	Confidence           float64
	Samples              int
}

type LeakGuardConfig struct {
	Window                  time.Duration
	MinSamples              int
	MinGrowthBytesPerSecond float64
	MinConfidence           float64
}

type leakSample struct {
	at    time.Time
	bytes uint64
}

type LeakGuard struct {
	config  LeakGuardConfig
	history map[Identity][]leakSample
}

func DefaultLeakGuardConfig() LeakGuardConfig {
	return LeakGuardConfig{
		Window:                  5 * time.Minute,
		MinSamples:              8,
		MinGrowthBytesPerSecond: 1024 * 1024 / 60,
		MinConfidence:           .8,
	}
}

func NewLeakGuard(config LeakGuardConfig) *LeakGuard {
	defaults := DefaultLeakGuardConfig()
	if config.Window <= 0 {
		config.Window = defaults.Window
	}
	if config.MinSamples < 2 {
		config.MinSamples = defaults.MinSamples
	}
	if config.MinGrowthBytesPerSecond <= 0 {
		config.MinGrowthBytesPerSecond = defaults.MinGrowthBytesPerSecond
	}
	if config.MinConfidence <= 0 || config.MinConfidence > 1 {
		config.MinConfidence = defaults.MinConfidence
	}
	return &LeakGuard{config: config, history: make(map[Identity][]leakSample)}
}

func (guard *LeakGuard) Observe(at time.Time, processes []Info) map[Identity]LeakAssessment {
	if guard == nil {
		return nil
	}
	if at.IsZero() {
		at = time.Now()
	}
	seen := make(map[Identity]bool, len(processes))
	result := make(map[Identity]LeakAssessment, len(processes))
	cutoff := at.Add(-guard.config.Window)
	for _, candidate := range processes {
		id := candidate.Identity
		seen[id] = true
		samples := append(guard.history[id], leakSample{at: at, bytes: candidate.MemoryBytes})
		first := 0
		for first < len(samples) && samples[first].at.Before(cutoff) {
			first++
		}
		samples = append([]leakSample(nil), samples[first:]...)
		guard.history[id] = samples
		result[id] = guard.assess(samples)
	}
	for id := range guard.history {
		if !seen[id] {
			delete(guard.history, id)
		}
	}
	return result
}

func (guard *LeakGuard) assess(samples []leakSample) LeakAssessment {
	assessment := LeakAssessment{Status: LeakObserving, Samples: len(samples)}
	if len(samples) < guard.config.MinSamples {
		return assessment
	}
	origin := samples[0].at
	var sumX, sumY, sumXX, sumXY float64
	for _, sample := range samples {
		x := sample.at.Sub(origin).Seconds()
		y := float64(sample.bytes)
		sumX += x
		sumY += y
		sumXX += x * x
		sumXY += x * y
	}
	n := float64(len(samples))
	denominator := n*sumXX - sumX*sumX
	if denominator == 0 {
		assessment.Status = LeakStable
		return assessment
	}
	slope := (n*sumXY - sumX*sumY) / denominator
	meanY := sumY / n
	intercept := (sumY - slope*sumX) / n
	var residual, total float64
	for _, sample := range samples {
		x := sample.at.Sub(origin).Seconds()
		y := float64(sample.bytes)
		prediction := intercept + slope*x
		residual += math.Pow(y-prediction, 2)
		total += math.Pow(y-meanY, 2)
	}
	confidence := 1.0
	if total > 0 {
		confidence = math.Max(0, 1-residual/total)
	}
	assessment.GrowthBytesPerSecond = slope
	assessment.Confidence = confidence
	assessment.Status = LeakStable
	if slope >= guard.config.MinGrowthBytesPerSecond && confidence >= guard.config.MinConfidence {
		assessment.Status = LeakSuspected
	}
	return assessment
}
