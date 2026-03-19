package main

import (
	"fmt"
	"math"
	"sync"
	"time"

	"github.com/globulario/services/golang/ai_router/ai_routerpb"
)

// safetyValidator enforces routing invariants to prevent bad decisions.
type safetyValidator struct {
	mu sync.Mutex

	// Previous weights for delta enforcement.
	previousWeights map[string]map[string]uint32 // service → endpoint → weight

	// Stability tracking: how many consecutive cycles each endpoint
	// has had a stable score (within hysteresis band).
	stableCycles map[string]int // service/endpoint → count

	// Last change time per endpoint for cooldown.
	lastChange map[string]time.Time // service/endpoint → time
}

func newSafetyValidator() *safetyValidator {
	return &safetyValidator{
		previousWeights: make(map[string]map[string]uint32),
		stableCycles:    make(map[string]int),
		lastChange:      make(map[string]time.Time),
	}
}

// safetyConfig holds tunable safety parameters per service class.
type safetyConfig struct {
	MaxWeightDelta    uint32        // max change per cycle
	MinWeight         uint32        // minimum weight (never below)
	MinActiveEndpoints int          // never remove all endpoints
	CooldownDuration  time.Duration // hold after change
	HysteresisThreshold float64    // score must change by this much to trigger weight change
}

// configForClass returns safety config for a service class.
func configForClass(class ai_routerpb.ServiceClass) safetyConfig {
	switch class {
	case ai_routerpb.ServiceClass_STREAM_HEAVY:
		return safetyConfig{
			MaxWeightDelta:     10,           // conservative for streams
			MinWeight:          5,
			MinActiveEndpoints: 1,
			CooldownDuration:   30 * time.Second,
			HysteresisThreshold: 0.08,
		}
	case ai_routerpb.ServiceClass_CONTROL_PLANE:
		return safetyConfig{
			MaxWeightDelta:     10,
			MinWeight:          20,           // never drain control plane
			MinActiveEndpoints: 1,
			CooldownDuration:   30 * time.Second,
			HysteresisThreshold: 0.10,
		}
	case ai_routerpb.ServiceClass_DEPLOYMENT_SENSITIVE:
		return safetyConfig{
			MaxWeightDelta:     15,
			MinWeight:          1,
			MinActiveEndpoints: 1,
			CooldownDuration:   20 * time.Second,
			HysteresisThreshold: 0.06,
		}
	default: // STATELESS_UNARY
		return safetyConfig{
			MaxWeightDelta:     20,
			MinWeight:          1,
			MinActiveEndpoints: 1,
			CooldownDuration:   15 * time.Second,
			HysteresisThreshold: 0.05,
		}
	}
}

// validate enforces safety invariants on a proposed routing policy.
// Returns the validated policy and a list of reasons for any changes made.
func (sv *safetyValidator) validate(
	proposed *ai_routerpb.RoutingPolicy,
	classifications map[string]ai_routerpb.ServiceClass,
) (clamps []string) {
	if proposed == nil {
		return nil
	}

	sv.mu.Lock()
	defer sv.mu.Unlock()

	now := time.Now()

	for svcName, sp := range proposed.Services {
		class := classifications[svcName]
		cfg := configForClass(class)

		prev := sv.previousWeights[svcName]
		if prev == nil {
			prev = make(map[string]uint32)
		}

		// Invariant 1: Never remove all endpoints.
		activeCount := 0
		var highestEp string
		var highestWeight uint32
		for ep, w := range sp.Weights {
			if w > 0 {
				activeCount++
			}
			if w > highestWeight {
				highestWeight = w
				highestEp = ep
			}
		}
		if activeCount == 0 && highestEp != "" {
			sp.Weights[highestEp] = cfg.MinWeight
			clamps = append(clamps, fmt.Sprintf(
				"safety: restored %s/%s to min weight %d (all endpoints were zero)",
				svcName, highestEp, cfg.MinWeight))
		}

		for ep, newW := range sp.Weights {
			key := svcName + "/" + ep

			// Invariant 2: Minimum weight per class.
			if newW < cfg.MinWeight {
				sp.Weights[ep] = cfg.MinWeight
				clamps = append(clamps, fmt.Sprintf(
					"safety: clamped %s min weight %d→%d (%s)",
					key, newW, cfg.MinWeight, class))
				newW = cfg.MinWeight
			}

			// Invariant 3: Max weight delta per cycle.
			if oldW, ok := prev[ep]; ok {
				delta := absDiff(newW, oldW)
				if delta > cfg.MaxWeightDelta {
					if newW > oldW {
						sp.Weights[ep] = oldW + cfg.MaxWeightDelta
					} else {
						sp.Weights[ep] = oldW - cfg.MaxWeightDelta
						if sp.Weights[ep] < cfg.MinWeight {
							sp.Weights[ep] = cfg.MinWeight
						}
					}
					clamps = append(clamps, fmt.Sprintf(
						"safety: delta clamped %s %d→%d (max ±%d)",
						key, oldW, sp.Weights[ep], cfg.MaxWeightDelta))
				}
			}

			// Invariant 4: Cooldown — if we changed recently, hold.
			if last, ok := sv.lastChange[key]; ok {
				if now.Sub(last) < cfg.CooldownDuration {
					if oldW, ok := prev[ep]; ok {
						sp.Weights[ep] = oldW // hold previous weight
					}
				}
			}

			// Track if weight actually changed (for cooldown).
			finalW := sp.Weights[ep]
			if oldW, ok := prev[ep]; ok && finalW != oldW {
				sv.lastChange[key] = now
			}
		}

		// Update previous weights for next cycle.
		newPrev := make(map[string]uint32, len(sp.Weights))
		for ep, w := range sp.Weights {
			newPrev[ep] = w
		}
		sv.previousWeights[svcName] = newPrev
	}

	return clamps
}

// computeConfidence returns a confidence score (0.0-1.0) based on data quality.
func computeConfidence(endpoints map[string]*endpointMetrics, node *nodeMetrics) float64 {
	if len(endpoints) == 0 {
		return 0
	}

	score := 0.5 // base confidence

	// Bonus for having endpoint metrics.
	withLatency := 0
	withRPS := 0
	for _, ep := range endpoints {
		if ep.LatencyP99 > 0 {
			withLatency++
		}
		if ep.RPS > 0 {
			withRPS++
		}
	}
	if withLatency > 0 {
		score += 0.15
	}
	if withRPS > 0 {
		score += 0.15
	}

	// Bonus for node metrics.
	if node != nil && !node.Stale {
		score += 0.1
	}

	// Bonus for having multiple data points (more endpoints = more confidence).
	if len(endpoints) >= 3 {
		score += 0.1
	}

	return math.Min(score, 1.0)
}

func absDiff(a, b uint32) uint32 {
	if a > b {
		return a - b
	}
	return b - a
}
