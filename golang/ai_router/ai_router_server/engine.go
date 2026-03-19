package main

import (
	"context"
	"time"

	"github.com/globulario/services/golang/ai_router/ai_routerpb"
)

// scoringLoop runs the background scoring cycle every 5 seconds.
// Computes per-endpoint scores, applies safety invariants, and caches
// the routing policy for the xDS watcher to consume.
func (srv *server) scoringLoop() {
	// Wait for service startup and Prometheus to be reachable.
	time.Sleep(15 * time.Second)

	coll := newCollector()
	weights := defaultWeights()
	safety := newSafetyValidator()
	ticker := time.NewTicker(scoringInterval)
	defer ticker.Stop()

	previousScores := make(map[string]float64)
	const smoothAlpha = 0.3

	cycle := uint64(0)
	for range ticker.C {
		cycle++
		ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)

		// Collect metrics from Prometheus.
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

		// Score all endpoints (includes anomaly signals from ai_watcher).
		results := scoreEndpoints(endpoints, node, srv.classifications, weights, srv.anomalies)

		// Apply exponential smoothing to prevent flapping.
		for i := range results {
			key := results[i].Service + "/" + results[i].Instance
			if prev, ok := previousScores[key]; ok {
				results[i].Score = smoothScore(results[i].Score, prev, smoothAlpha)
				w := uint32(100 * (1 - results[i].Score))
				if w < 1 {
					w = 1
				}
				results[i].Weight = w
			}
			previousScores[key] = results[i].Score
		}

		// Read current mode.
		srv.modeMu.RLock()
		mode := srv.mode
		srv.modeMu.RUnlock()

		// Compute data quality confidence.
		confidence := computeConfidence(endpoints, node)

		// Build policy from scoring results.
		policy := &ai_routerpb.RoutingPolicy{
			Services:     make(map[string]*ai_routerpb.ServicePolicy),
			Generation:   cycle,
			ComputedAtMs: time.Now().UnixMilli(),
			Mode:         mode,
		}

		changed := 0
		for _, r := range results {
			sp := policy.Services[r.Service]
			if sp == nil {
				class := srv.classifications[r.Service]
				sp = &ai_routerpb.ServicePolicy{
					Weights:      make(map[string]uint32),
					ServiceClass: class,
					Confidence:   float32(confidence),
					Reasons:      []string{},
				}
				policy.Services[r.Service] = sp
			}
			sp.Weights[r.Instance] = r.Weight
			if r.Weight < 90 {
				changed++
				sp.Reasons = append(sp.Reasons, r.Reasons...)
			}
		}

		// Apply stability controls (outlier detection, circuit breakers, retries).
		for svcName, sp := range policy.Services {
			avgErr := computeAvgErrorRate(endpoints, svcName)
			applyStabilityControls(sp, sp.ServiceClass, avgErr)
		}

		// Apply safety invariants (max delta, min weight, cooldown).
		clamps := safety.validate(policy, srv.classifications)

		// Check for drain conditions and manage active drains.
		for svcName, sp := range policy.Services {
			class := srv.classifications[svcName]
			for ep, w := range sp.Weights {
				if shouldDrain(w, class) {
					reason := "low_score"
					if srv.anomalies.getAnomalyScore(svcName) > 0.3 {
						reason = "security_anomaly"
					}
					if srv.drains.startDrain(svcName, ep, class, reason) {
						sp.Reasons = append(sp.Reasons, "drain started: "+ep+" ("+reason+")")
					}
				} else if w >= 50 && srv.drains.isDraining(svcName, ep) {
					// Endpoint recovered — cancel drain.
					srv.drains.cancelDrain(svcName, ep)
					sp.Reasons = append(sp.Reasons, "drain cancelled: "+ep+" (recovered)")
				}
			}
		}

		// Apply active drains to policy (set weight=0, add drain entries).
		drainEvents := srv.drains.applyDrains(policy)
		for _, c := range clamps {
			logger.Info("safety", "clamp", c, "cycle", cycle)
		}

		// Log notable scores.
		for _, r := range results {
			if r.Score > 0.3 || cycle%12 == 0 {
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
		}

		// Cache policy based on mode.
		switch mode {
		case ai_routerpb.RouterMode_ROUTER_ACTIVE:
			// Active: cache with real confidence — xDS watcher applies weights.
			srv.cachedPolicy.Store(policy)
			srv.statsMu.Lock()
			srv.stats.PoliciesApplied++
			srv.statsMu.Unlock()

		case ai_routerpb.RouterMode_ROUTER_OBSERVE:
			// Observe: cache for visibility via GetRoutingPolicy, but
			// confidence tells xDS watcher not to apply.
			for _, sp := range policy.Services {
				sp.Confidence = 0 // signal: don't apply
				sp.Reasons = append(sp.Reasons, "observe mode: computed but not applied")
			}
			srv.cachedPolicy.Store(policy)

		default:
			// Neutral: don't cache.
		}

		srv.statsMu.Lock()
		srv.stats.PoliciesComputed++
		srv.stats.LastPolicyAt = time.Now()
		srv.statsMu.Unlock()

		// Publish drain events.
		for _, de := range drainEvents {
			logger.Info("drain_event", "event", de, "cycle", cycle)
			if srv.anomalies != nil {
				srv.anomalies.publishRoutingEvent("routing.drain.completed", map[string]interface{}{
					"detail": de,
					"cycle":  cycle,
				})
			}
		}

		if changed > 0 || len(clamps) > 0 || len(drainEvents) > 0 || cycle%12 == 0 {
			logger.Info("scoring_summary",
				"cycle", cycle,
				"mode", mode.String(),
				"confidence", round2(confidence),
				"endpoints", len(results),
				"would_change", changed,
				"safety_clamps", len(clamps),
				"active_drains", srv.drains.activeDrains(),
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
