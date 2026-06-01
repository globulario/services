package main

import (
	"math"
	"time"

	"github.com/globulario/services/golang/ai_router/ai_routerpb"
)

// scoringWeights defines per-signal weights for the scoring model.
type scoringWeights struct {
	CPU        float64
	LatencyP99 float64
	ErrorRate  float64
	Anomaly    float64 // Phase 3: from ai_watcher
	Reliability float64 // Phase 6: from ai_memory
}

// defaultWeights returns conservative scoring weights for Phase 0.5.
func defaultWeights() scoringWeights {
	return scoringWeights{
		CPU:        0.25,
		LatencyP99: 0.25,
		ErrorRate:  0.30,
		Anomaly:    0.10,
		Reliability: 0.10,
	}
}

// scoringResult holds the computed score and breakdown for one endpoint.
type scoringResult struct {
	Service    string
	Instance   string
	Score      float64            // 0.0 (healthy) to 1.0 (unhealthy)
	Weight     uint32             // computed weight (1-100)
	Components map[string]float64 // per-signal breakdown
	Reasons    []string           // human-readable explanation
}

// scoreEndpoints computes scores for all endpoints given their metrics.
func scoreEndpoints(
	endpoints map[string]*endpointMetrics,
	node *nodeMetrics,
	classifications map[string]ai_routerpb.ServiceClass,
	weights scoringWeights,
	anomalies *anomalyTracker,
) []scoringResult {

	// Find max latency across all endpoints for normalization.
	var maxLatency float64
	for _, ep := range endpoints {
		if ep.LatencyP99 > maxLatency {
			maxLatency = ep.LatencyP99
		}
	}
	if maxLatency == 0 {
		maxLatency = 1 // avoid divide by zero
	}

	var results []scoringResult
	for key, ep := range endpoints {
		_ = key

		components := make(map[string]float64)

		// CPU component (node-level, shared across all endpoints).
		cpuScore := 0.5 // neutral if stale
		if node != nil && !node.Stale {
			cpuScore = clamp(node.CPUUsage, 0, 1)
		}
		components["cpu"] = cpuScore

		// Latency component (endpoint-level).
		latScore := 0.0
		if maxLatency > 0 {
			latScore = clamp(ep.LatencyP99/maxLatency, 0, 1)
		}
		components["latency_p99"] = latScore

		// Error rate component (endpoint-level).
		errScore := clamp(ep.ErrorRate, 0, 1)
		components["error_rate"] = errScore

		// Anomaly component (Phase 3 — from ai_watcher security events).
		anomalyScore := 0.0
		if anomalies != nil {
			anomalyScore = anomalies.getAnomalyScore(ep.Service)
		}
		components["anomaly"] = anomalyScore

		// Reliability component (Phase 6 — default 0 for now, meaning "fully reliable").
		reliabilityPenalty := 0.0
		components["reliability"] = reliabilityPenalty

		// Weighted sum.
		score := weights.CPU*cpuScore +
			weights.LatencyP99*latScore +
			weights.ErrorRate*errScore +
			weights.Anomaly*anomalyScore +
			weights.Reliability*reliabilityPenalty

		score = clamp(score, 0, 1)

		// Convert score → weight (lower score = higher weight).
		weight := uint32(math.Round(100 * (1 - score)))
		if weight < 1 {
			weight = 1 // never zero unless explicitly draining
		}

		// Build human-readable reasons for non-trivial scores.
		var reasons []string
		if cpuScore > 0.7 {
			reasons = append(reasons, "high CPU usage")
		}
		if latScore > 0.7 {
			reasons = append(reasons, "high latency")
		}
		if errScore > 0.1 {
			reasons = append(reasons, "elevated error rate")
		}
		if anomalyScore > 0.3 {
			reasons = append(reasons, "security anomaly detected")
		}
		if len(reasons) == 0 {
			reasons = append(reasons, "healthy")
		}

		results = append(results, scoringResult{
			Service:    ep.Service,
			Instance:   ep.Instance,
			Score:      score,
			Weight:     weight,
			Components: components,
			Reasons:    reasons,
		})
	}

	return results
}

// smoothScore applies exponential smoothing to prevent flapping.
// Returns: α * current + (1-α) * previous
func smoothScore(current, previous, alpha float64) float64 {
	if previous == 0 {
		return current // no history, use current
	}
	return alpha*current + (1-alpha)*previous
}

// clamp restricts a value to [min, max].
func clamp(v, min, max float64) float64 {
	if v < min {
		return min
	}
	if v > max {
		return max
	}
	return v
}

// scoringInterval is how often the dry-run scoring loop runs.
const scoringInterval = 5 * time.Second
