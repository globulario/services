package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

const (
	// leaderLivenessInterval is how often the watchdog checks heartbeat processing.
	leaderLivenessInterval = 30 * time.Second

	// leaderLivenessThreshold is how long without a processed heartbeat before
	// the leader considers itself potentially degraded.
	leaderLivenessThreshold = 2 * time.Minute

	// leaderLivenessConsecutiveFailures is how many consecutive failed checks
	// are required before triggering resignation (hysteresis).
	leaderLivenessConsecutiveFailures = 2

	// leaderLivenessWarmup is the grace period after becoming leader before
	// the watchdog starts checking. Prevents false positives during startup
	// when no nodes have reported yet.
	leaderLivenessWarmup = 3 * time.Minute
)

var (
	leaderSelfResignTotal = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "leader_self_resign_total",
		Help:      "Total number of times the leader resigned due to liveness failure.",
	})

	leaderLivenessChecksFailed = promauto.NewCounter(prometheus.CounterOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "leader_liveness_checks_failed_total",
		Help:      "Total liveness checks that detected no recent heartbeat processing.",
	})

	leaderLastHeartbeatProcessed = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "last_heartbeat_processed_unix",
		Help:      "Unix timestamp of the last successfully processed heartbeat.",
	})

	leaderLivenessDegraded = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "globular",
		Subsystem: "controller",
		Name:      "leader_liveness_degraded",
		Help:      "1 when liveness is degraded but no safe successor exists, 0 otherwise.",
	})
)

// startLeaderLivenessCheck starts a background goroutine that monitors whether
// the leader is actually processing heartbeats. If the leader holds the etcd
// lease but does no useful work for too long, and a safe successor exists, it
// auto-resigns via resignCh.
//
// This check is based entirely on actual work performed (heartbeat processing),
// NOT on etcd lease health. etcd keepalive only proves the process is alive,
// not functional.
func (srv *server) startLeaderLivenessCheck(ctx context.Context) {
	ticker := time.NewTicker(leaderLivenessInterval)
	safeGo("leader-liveness-check", func() {
		defer ticker.Stop()

		consecutiveFailures := 0
		leaderSince := time.Time{} // tracks when we became leader for warmup
		degradedEmitted := false   // prevents spamming degraded events

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}

			// Export last heartbeat timestamp as a Prometheus gauge on every tick.
			if lastNano := srv.lastHeartbeatProcessed.Load(); lastNano > 0 {
				leaderLastHeartbeatProcessed.Set(float64(time.Unix(0, lastNano).Unix()))
			}

			// Only the leader needs self-checking.
			if !srv.isLeader() {
				consecutiveFailures = 0
				leaderSince = time.Time{}
				degradedEmitted = false
				leaderLivenessDegraded.Set(0)
				continue
			}

			// Track when we first observed ourselves as leader (for warmup).
			if leaderSince.IsZero() {
				leaderSince = time.Now()
			}

			// Warmup grace: don't check until nodes have had time to report.
			if time.Since(leaderSince) < leaderLivenessWarmup {
				continue
			}

			// Count expected heartbeat sources: currently enrolled active nodes
			// that should be sending heartbeats.
			selfNodeID := srv.findSelfNodeID()
			expectedCount, expectedNodes := srv.countExpectedHeartbeatSources(selfNodeID)

			// If no nodes are expected to heartbeat, there's nothing to check.
			// Don't accumulate failures — this prevents single-node clusters
			// and fully-unreachable clusters from churning leadership.
			if expectedCount == 0 {
				if consecutiveFailures > 0 {
					log.Printf("leader-liveness-check: no expected heartbeat sources, resetting failure counter (was %d)", consecutiveFailures)
				}
				consecutiveFailures = 0
				leaderLivenessDegraded.Set(0)
				degradedEmitted = false
				continue
			}

			// Check: has a heartbeat been successfully processed recently?
			lastNano := srv.lastHeartbeatProcessed.Load()
			var elapsed time.Duration
			if lastNano > 0 {
				elapsed = time.Since(time.Unix(0, lastNano))
			}

			if lastNano == 0 || elapsed > leaderLivenessThreshold {
				leaderLivenessChecksFailed.Inc()
				consecutiveFailures++

				if lastNano == 0 {
					log.Printf("leader-liveness-check: no heartbeat ever processed (expected_nodes=%d, consecutive_failures=%d/%d)",
						expectedCount, consecutiveFailures, leaderLivenessConsecutiveFailures)
				} else {
					log.Printf("leader-liveness-check: no heartbeat for %s (threshold=%s, expected_nodes=%d, consecutive_failures=%d/%d)",
						elapsed.Truncate(time.Second), leaderLivenessThreshold,
						expectedCount, consecutiveFailures, leaderLivenessConsecutiveFailures)
				}
			} else {
				// Healthy — reset hysteresis counter.
				if consecutiveFailures > 0 {
					log.Printf("leader-liveness-check: heartbeat recovered (last=%s ago), resetting failure counter",
						elapsed.Truncate(time.Second))
				}
				consecutiveFailures = 0
				leaderLivenessDegraded.Set(0)
				degradedEmitted = false
				continue
			}

			// Hysteresis: only consider resignation after N consecutive failures.
			if consecutiveFailures < leaderLivenessConsecutiveFailures {
				continue
			}

			// Check for a safe successor before resigning.
			successorID := srv.findLivenessSafeSuccessor(selfNodeID, expectedNodes)

			hostname, _ := os.Hostname()
			leaderID, _ := srv.leaderID.Load().(string)
			reason := fmt.Sprintf("no heartbeat processed for %d consecutive checks (%s intervals, threshold %s, expected_nodes=%d)",
				consecutiveFailures, leaderLivenessInterval, leaderLivenessThreshold, expectedCount)

			if successorID == "" {
				// No safe successor — do NOT resign. Emit warning and wait.
				leaderLivenessDegraded.Set(1)
				if !degradedEmitted {
					log.Printf("WARN: leader-liveness-check: liveness degraded but no safe successor exists, retaining leadership — %s", reason)
					srv.emitClusterEvent("controller.leader_liveness_degraded", map[string]interface{}{
						"severity":            "WARNING",
						"node_id":             hostname,
						"leader_id":           leaderID,
						"reason":              reason,
						"expected_nodes":      expectedCount,
						"successor_count":     0,
						"last_heartbeat_unix": srv.lastHeartbeatProcessed.Load(),
						"timestamp":           time.Now().UTC().Format(time.RFC3339),
					})
					degradedEmitted = true
				}
				// Reset failure counter — don't let it grow unbounded while
				// stuck without a successor. It will re-accumulate if the
				// condition persists.
				consecutiveFailures = 0
				continue
			}

			// Safe successor exists — resign.
			log.Printf("CRITICAL: leader-liveness-check: self-resigning leadership (successor=%s) — %s", successorID, reason)

			// Emit structured event BEFORE resigning so it goes out
			// while we still have leader context.
			srv.emitClusterEvent("controller.leader_self_resign", map[string]interface{}{
				"severity":            "CRITICAL",
				"node_id":             hostname,
				"leader_id":           leaderID,
				"reason":              reason,
				"last_heartbeat_unix": srv.lastHeartbeatProcessed.Load(),
				"successor_id":        successorID,
				"timestamp":           time.Now().UTC().Format(time.RFC3339),
			})

			leaderSelfResignTotal.Inc()

			// Signal the leader election goroutine to resign.
			select {
			case srv.resignCh <- struct{}{}:
				log.Printf("leader-liveness-check: resign signal sent")
			default:
				log.Printf("leader-liveness-check: resign already in progress")
			}

			// Reset state — the ticker continues but will skip checks
			// because isLeader() will return false after resignation.
			consecutiveFailures = 0
			leaderSince = time.Time{}
			degradedEmitted = false
			leaderLivenessDegraded.Set(0)
		}
	})
}

// countExpectedHeartbeatSources returns the number of currently enrolled active
// nodes that are expected to be sending heartbeats, along with their IDs.
//
// A node is an expected heartbeat source if ALL of:
//   - not self (leader doesn't heartbeat to itself)
//   - has reported at least once (node.LastSeen is non-zero)
//   - not unreachable (already marked stale by health monitor)
//   - not blocked
//
// No "drained" or "retired" status exists yet in the codebase, but if added
// they should be excluded here as well.
func (srv *server) countExpectedHeartbeatSources(selfNodeID string) (int, []string) {
	srv.lock("leader-liveness:count-expected")
	defer srv.unlock()

	var count int
	var ids []string
	for id, node := range srv.state.Nodes {
		if id == selfNodeID {
			continue
		}
		if node == nil || node.LastSeen.IsZero() {
			continue
		}
		if node.Status == "unreachable" || node.Status == "blocked" {
			continue
		}
		count++
		ids = append(ids, id)
	}
	return count, ids
}

// findLivenessSafeSuccessor checks whether at least one follower is viable to
// take over leadership. Returns the ID of the first viable successor, or ""
// if none exist.
//
// A safe successor for liveness failover must satisfy:
//   - fresh heartbeat (within heartbeatStaleThreshold)
//   - not unreachable or blocked
//   - has the cluster-controller unit running (present in unit inventory)
//
// This intentionally does NOT check version/checksum match. A running
// controller at any version can lead — availability is prioritized over
// upgrade uniformity. Version convergence is handled separately by the
// self-update reconciler.
func (srv *server) findLivenessSafeSuccessor(selfNodeID string, candidateIDs []string) string {
	srv.lock("leader-liveness:find-successor")
	defer srv.unlock()

	for _, id := range candidateIDs {
		node := srv.state.Nodes[id]
		if node == nil {
			continue
		}

		// Predicate 1: fresh heartbeat.
		if time.Since(node.LastSeen) >= heartbeatStaleThreshold {
			continue
		}

		// Predicate 2: not unreachable/blocked.
		if node.Status == "unreachable" || node.Status == "blocked" {
			continue
		}

		// Predicate 3: controller unit is present (running or at least installed).
		if !hasControllerUnit(node) {
			continue
		}

		return id
	}
	return ""
}

// hasControllerUnit checks whether the node has a cluster-controller systemd
// unit in its unit inventory, indicating the controller binary is installed.
func hasControllerUnit(node *nodeState) bool {
	for _, u := range node.Units {
		name := canonicalServiceName(u.Name)
		if name == "cluster-controller" {
			return true
		}
	}
	return false
}
