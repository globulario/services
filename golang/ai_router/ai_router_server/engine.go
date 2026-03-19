package main

import (
	"context"
	"time"

	"github.com/globulario/services/golang/ai_router/ai_routerpb"
)

// scoringLoop runs the background scoring cycle every 5 seconds.
// In neutral/observe mode: computes scores, logs results, but doesn't cache
// the policy for xDS consumption.
// In active mode (Phase 1+): also stores the policy for GetRoutingPolicy.
func (srv *server) scoringLoop() {
	// Wait for service startup.
	time.Sleep(15 * time.Second)

	coll := newCollector()
	weights := defaultWeights()
	ticker := time.NewTicker(scoringInterval)
	defer ticker.Stop()

	var previousScores map[string]float64 // key → previous score for smoothing
	previousScores = make(map[string]float64)
	const smoothAlpha = 0.3

	cycle := uint64(0)
	for range ticker.C {
		cycle++
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)

		// Collect metrics.
		endpoints, node, err := coll.collectAll(ctx)
		cancel()
		if err != nil {
			logger.Warn("scoring: metrics collection failed", "cycle", cycle, "err", err)
			continue
		}

		srv.statsMu.Lock()
		srv.stats.LastMetricsAt = time.Now()
		srv.statsMu.Unlock()

		if len(endpoints) == 0 {
			logger.Debug("scoring: no endpoints found", "cycle", cycle)
			continue
		}

		// Score all endpoints.
		results := scoreEndpoints(endpoints, node, srv.classifications, weights)

		// Apply smoothing.
		for i := range results {
			key := results[i].Service + "/" + results[i].Instance
			if prev, ok := previousScores[key]; ok {
				results[i].Score = smoothScore(results[i].Score, prev, smoothAlpha)
				// Recompute weight from smoothed score.
				w := uint32(100 * (1 - results[i].Score))
				if w < 1 {
					w = 1
				}
				results[i].Weight = w
			}
			previousScores[key] = results[i].Score
		}

		// Log dry-run results.
		srv.modeMu.RLock()
		mode := srv.mode
		srv.modeMu.RUnlock()

		changed := 0
		for _, r := range results {
			if r.Score > 0.3 || cycle%12 == 0 { // log notable scores, or every minute for all
				logger.Info("scoring",
					"mode", mode.String(),
					"cycle", cycle,
					"service", r.Service,
					"instance", r.Instance,
					"score", round2(r.Score),
					"weight", r.Weight,
					"cpu", round2(r.Components["cpu"]),
					"latency", round2(r.Components["latency_p99"]),
					"errors", round2(r.Components["error_rate"]),
					"reasons", r.Reasons,
				)
			}
			if r.Weight < 90 {
				changed++
			}
		}

		// Build policy.
		policy := &ai_routerpb.RoutingPolicy{
			Services:     make(map[string]*ai_routerpb.ServicePolicy),
			Generation:   cycle,
			ComputedAtMs: time.Now().UnixMilli(),
			Mode:         mode,
		}

		for _, r := range results {
			sp := policy.Services[r.Service]
			if sp == nil {
				sp = &ai_routerpb.ServicePolicy{
					Weights:      make(map[string]uint32),
					ServiceClass: srv.classifications[r.Service],
					Confidence:   0.0, // Phase 0.5: dry-run, no confidence
					Reasons:      []string{"phase-0.5: dry-run scoring, not applied"},
				}
				policy.Services[r.Service] = sp
			}
			sp.Weights[r.Instance] = r.Weight
		}

		// In observe mode: cache the policy (readable via GetRoutingPolicy but
		// xDS watcher treats confidence=0 as passthrough).
		if mode == ai_routerpb.RouterMode_ROUTER_OBSERVE || mode == ai_routerpb.RouterMode_ROUTER_ACTIVE {
			srv.cachedPolicy.Store(policy)
		}

		srv.statsMu.Lock()
		srv.stats.PoliciesComputed++
		srv.stats.LastPolicyAt = time.Now()
		srv.statsMu.Unlock()

		if changed > 0 || cycle%12 == 0 {
			logger.Info("scoring_summary",
				"cycle", cycle,
				"mode", mode.String(),
				"endpoints", len(results),
				"would_change", changed,
				"cpu", round2(node.CPUUsage),
				"memory", round2(node.MemoryUsage),
			)
		}
	}
}

// round2 rounds to 2 decimal places for logging.
func round2(v float64) float64 {
	return float64(int(v*100+0.5)) / 100
}
