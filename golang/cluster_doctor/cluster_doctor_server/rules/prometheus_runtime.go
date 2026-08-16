package rules

import (
	"fmt"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// heartbeatEpochFloorYear separates a REAL past heartbeat from the
// unset-timestamp artifact.
//
// A heartbeat metric that was never set reads 0, so `time() - 0` yields an age
// equal to the whole Unix epoch (~1.78e9s ≈ 56 years) and the IMPLIED heartbeat
// instant lands on 1970. That is absence of evidence, not stale evidence.
//
// This used to be an upper age CUTOFF (30 days): any age beyond it was dropped
// entirely. That silently hid genuinely stale evidence — a node dead 31, 100 or
// 500 days is exactly what an operator most needs to see, and its finding simply
// vanished. The discriminator is not "how old is it" but "does the implied
// instant look like the epoch". There is deliberately no maximum age.
//
// Defense-in-depth only: collector/prometheus.go already excludes unset series
// with a PromQL `> 0` filter. This catches a zero-based age arriving by some
// other path. (repair.runtime_evidence_stale_or_conflicting)
const heartbeatEpochFloorYear = 2000

// heartbeatAgeIsRealEvidence reports whether a Prometheus-derived age describes
// a real past heartbeat. now is the scrape instant the age was derived against.
func heartbeatAgeIsRealEvidence(age float64, now time.Time) bool {
	if age <= 0 {
		// Zero or negative age: either the sample is absent/unset or the implied
		// instant is in the future. Neither is past staleness evidence.
		return false
	}
	if now.IsZero() {
		now = time.Now()
	}
	implied := now.Add(-time.Duration(age * float64(time.Second)))
	return implied.Year() >= heartbeatEpochFloorYear
}

// promRuntime checks Prometheus-fed control plane signals (if present).
// Scope: cluster.
type promRuntime struct{}

func (promRuntime) ID() string       { return "prometheus.runtime_signals" }
func (promRuntime) Category() string { return "observability" }
func (promRuntime) Scope() string    { return "cluster" }

func (promRuntime) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap.PromMetrics == nil {
		return nil
	}

	var findings []Finding

	if age, ok := snap.PromMetrics["controller_loop_heartbeat_age"]; ok && age > 180 && heartbeatAgeIsRealEvidence(age, snap.PromTS) {
		// Suppress when Prometheus only scrapes a non-leader controller instance.
		// Non-leaders never update loop_heartbeat_unix (the heartbeat loop is
		// gated on holding the leader lease), so their heartbeat age is always
		// stale — this is expected, not a stall.
		// reconcile_dropped_not_leader > 0 means the scraped instance(s) have
		// explicitly dropped ticks for not_leader, confirming they are followers.
		// The real leader's freshness is authoritative; check etcd reconcile lanes
		// (updated_at_unix in /globular/controller/reconcile/lanes/) if needed.
		nonLeaderDrops, hasDrops := snap.PromMetrics["reconcile_dropped_not_leader"]
		isNonLeaderScrape := hasDrops && nonLeaderDrops > 0
		if !isNonLeaderScrape {
			findings = append(findings, Finding{
				FindingID:   FindingID("prom.controller_stalled", "cluster", "controller"),
				InvariantID: "prometheus.controller_stalled",
				Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
				Category:    "control_plane",
				EntityRef:   "controller",
				Summary:     fmt.Sprintf("Controller reconcile loop stalled for %.0fs (Prometheus)", age),
				Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "controller_loop", map[string]string{
					"age_seconds": fmt.Sprintf("%.0f", age),
					"timestamp":   snap.PromTS.UTC().Format(time.RFC3339),
				})},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}

	if age, ok := snap.PromMetrics["workflow_oldest_active_age"]; ok && age > 900 {
		findings = append(findings, Finding{
			FindingID:   FindingID("prom.workflow_stuck", "cluster", "workflow"),
			InvariantID: "prometheus.workflow_stuck",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "control_plane",
			EntityRef:   "workflow",
			Summary:     fmt.Sprintf("Oldest active workflow stuck for %.0fs (Prometheus)", age),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "workflow_oldest_active_age", map[string]string{
				"age_seconds": fmt.Sprintf("%.0f", age),
				"timestamp":   snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if runs, ok := snap.PromMetrics["workflow_active_runs"]; ok && runs > 0 {
		age := snap.PromMetrics["workflow_oldest_active_age"]
		findings = append(findings, Finding{
			FindingID:   FindingID("prom.workflow_active", "cluster", "workflow"),
			InvariantID: "prometheus.workflow_active",
			Severity:    cluster_doctorpb.Severity_SEVERITY_INFO,
			Category:    "control_plane",
			EntityRef:   "workflow",
			Summary:     fmt.Sprintf("%d workflow run(s) active; oldest age %.0fs (Prometheus)", int(runs), age),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "workflow_active", map[string]string{
				"active_runs":        fmt.Sprintf("%.0f", runs),
				"oldest_age_seconds": fmt.Sprintf("%.0f", age),
				"timestamp":          snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_PASS,
		})
	}

	if age, ok := snap.PromMetrics["node_heartbeat_age_max"]; ok && age > 120 && heartbeatAgeIsRealEvidence(age, snap.PromTS) {
		findings = append(findings, Finding{
			FindingID:   FindingID("prom.node_heartbeats_stale", "cluster", "nodes"),
			InvariantID: "prometheus.node_heartbeats_stale",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "infrastructure",
			EntityRef:   "nodes",
			Summary:     fmt.Sprintf("Max node heartbeat age %.0fs (Prometheus)", age),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "node_heartbeat_age", map[string]string{
				"max_age_seconds": fmt.Sprintf("%.0f", age),
				"timestamp":       snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if leader, ok := snap.PromMetrics["etcd_has_leader"]; ok && leader < 1 {
		findings = append(findings, Finding{
			FindingID:   FindingID("prom.etcd_no_leader", "cluster", "etcd"),
			InvariantID: "prometheus.etcd_no_leader",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "infrastructure",
			EntityRef:   "etcd",
			Summary:     "Prometheus reports etcd leader down",
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "etcd", map[string]string{
				"has_leader": fmt.Sprintf("%.0f", leader),
				"timestamp":  snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	// ── Storm protection signals (Phase A-D) ────────────────────────────

	if loops, ok := snap.PromMetrics["apply_loop_detected"]; ok && loops > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("cluster.apply_loop_detected", "cluster", "controller"),
			InvariantID: "cluster.apply_loop_detected",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     fmt.Sprintf("Apply-loop detection triggered %d time(s) — packages quarantined from auto-dispatch", int(loops)),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "apply_loop", map[string]string{
				"total_quarantines": fmt.Sprintf("%.0f", loops),
				"timestamp":         snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if mismatches, ok := snap.PromMetrics["drift_kind_mismatch_rate_15m"]; ok && mismatches > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("desired.kind_mismatch", "cluster", "controller"),
			InvariantID: "desired.kind_mismatch",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     fmt.Sprintf("Desired-state kind mismatch blocked %d dispatch(es) — SERVICE desired but INFRASTRUCTURE in repo", int(mismatches)),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "kind_mismatch", map[string]string{
				"total_blocked": fmt.Sprintf("%.0f", mismatches),
				"timestamp":     snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	// reconcile_circuit_open is a current-state gauge (1 = breaker open now,
	// 0 = closed) — NOT the raw monotonic _total counter, which only ever grows
	// and would fire this CRITICAL forever after a single transient open. The
	// finding auto-clears the moment reconcile recovers (gauge → 0). See
	// diagnostics.must_measure_reality and the workflow_circuit_open rule below.
	if open, ok := snap.PromMetrics["reconcile_circuit_open"]; ok && open > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("cluster.reconcile_circuit_open", "cluster", "controller"),
			InvariantID: "cluster.reconcile_circuit_open",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     "Reconcile circuit breaker is OPEN — periodic reconcile suspended until repeated failures clear",
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "circuit_breaker", map[string]string{
				"circuit_open": fmt.Sprintf("%.0f", open),
				"timestamp":    snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	// workflow.backend_pressure — distinguish transient vs sustained.
	//
	// The previous shape consumed the raw counter
	// (workflow_dispatch_rejected). A monotonic counter is non-zero
	// forever after a single transient blip (e.g. one rejection
	// during a workflow_server restart), so the finding fired on every
	// sweep and the incident never auto-cleared. This shape consumes
	// rate-over-window metrics instead:
	//
	//   transient (1 ≤ rate_5m, rate_15m ≤ pressure_sustained_threshold) →
	//       INFO + INVARIANT_PASS (visible but doesn't open an incident)
	//   sustained (rate_15m > pressure_sustained_threshold)             →
	//       WARN + INVARIANT_FAIL (opens an incident, operator-actionable)
	//   absent (rate_5m == 0 and rate_15m == 0)                        →
	//       no finding emitted; existing OPEN auto-clears via absent_scans
	rate5m, rate5mOK := snap.PromMetrics["workflow_dispatch_rejected_rate_5m"]
	rate15m, rate15mOK := snap.PromMetrics["workflow_dispatch_rejected_rate_15m"]
	total, _ := snap.PromMetrics["workflow_dispatch_rejected"]
	pressureSustainedThreshold := 5.0 // rejections in 15m before we call it "sustained"

	if rate5mOK && rate15mOK && (rate5m > 0 || rate15m > 0) {
		// "Sustained" means rejections are STILL happening — not that a lot
		// happened at some point in the last 15m. A bring-up burst (e.g. the
		// workflow health-gate correctly rejecting dispatches while the backend
		// warms up) inflates rate_15m but stops; once rate_5m hits 0 the
		// pressure has resolved and is auto-clearing, so it must NOT be reported
		// as SUSTAINED (WARN). Require ongoing rejections (rate_5m > 0) before
		// escalating; a high rate_15m with rate_5m == 0 is transient (INFO).
		sustained := rate15m > pressureSustainedThreshold && rate5m > 0
		sev := cluster_doctorpb.Severity_SEVERITY_INFO
		status := cluster_doctorpb.InvariantStatus_INVARIANT_PASS
		summary := fmt.Sprintf("Workflow dispatch pressure transient — %.0f rejection(s) in last 5m, %.0f in last 15m (auto-clear when both rates hit 0)",
			rate5m, rate15m)
		if sustained {
			sev = cluster_doctorpb.Severity_SEVERITY_WARN
			status = cluster_doctorpb.InvariantStatus_INVARIANT_FAIL
			summary = fmt.Sprintf("Workflow dispatch pressure SUSTAINED — %.0f rejection(s) in last 15m (threshold %.0f) — backend under sustained pressure",
				rate15m, pressureSustainedThreshold)
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("workflow.backend_pressure", "cluster", "workflow"),
			InvariantID: "workflow.backend_pressure",
			Severity:    sev,
			Category:    "control_plane",
			EntityRef:   "workflow",
			Summary:     summary,
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "backend_pressure", map[string]string{
				"prometheus_query_5m":  "sum(increase(globular_controller_workflow_dispatch_rejected_total[5m]))",
				"prometheus_query_15m": "sum(increase(globular_controller_workflow_dispatch_rejected_total[15m]))",
				"rate_5m":              fmt.Sprintf("%.2f", rate5m),
				"rate_15m":             fmt.Sprintf("%.2f", rate15m),
				"sustained_threshold":  fmt.Sprintf("%.0f", pressureSustainedThreshold),
				"sustained":            fmt.Sprintf("%v", sustained),
				"total_rejected_ever":  fmt.Sprintf("%.0f", total),
				"window":               "5m,15m",
				"last_observed":        snap.PromTS.UTC().Format(time.RFC3339),
				"auto_clear_condition": "rate_5m == 0 AND rate_15m == 0",
			})},
			InvariantStatus: status,
		})
	}

	// ── Day-1 resilience signals (Phase 2-4) ────────────────────────────

	if open, ok := snap.PromMetrics["workflow_circuit_open"]; ok && open > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("workflow.dispatch_circuit_open", "cluster", "workflow"),
			InvariantID: "workflow.dispatch_circuit_open",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "control_plane",
			EntityRef:   "workflow",
			Summary:     "Workflow dispatch circuit breaker is OPEN — all workflow dispatches are blocked until backend recovers",
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "circuit_open", map[string]string{
				"circuit_open": fmt.Sprintf("%.0f", open),
				"timestamp":    snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	// release.blocked_workflow_unavailable — `release_transient_blocked`
	// is currently scraped as a gauge that records the count of releases
	// in transient-retry backoff at *some* point. If the metric is sticky
	// (never decremented when the workflow recovers), this finding fires
	// forever after one workflow flap. Treat as advisory (INFO + PASS)
	// when the value is small (1–N), elevate to WARN + FAIL only when the
	// stuck count crosses a threshold we'd actually act on. The signal is
	// still recorded as evidence so an operator can trace the history.
	if blocked, ok := snap.PromMetrics["release_transient_blocked"]; ok && blocked > 0 {
		const stuckReleaseThreshold = 3.0
		sev := cluster_doctorpb.Severity_SEVERITY_INFO
		status := cluster_doctorpb.InvariantStatus_INVARIANT_PASS
		summary := fmt.Sprintf("%.0f release(s) recorded as transient-blocked (workflow flapped) — advisory; clears when the controller restarts or re-evaluates the gauge", blocked)
		if blocked >= stuckReleaseThreshold {
			sev = cluster_doctorpb.Severity_SEVERITY_WARN
			status = cluster_doctorpb.InvariantStatus_INVARIANT_FAIL
			summary = fmt.Sprintf("%.0f release(s) blocked in transient retry backoff — threshold %.0f exceeded — workflow service was unreachable during last dispatch attempt", blocked, stuckReleaseThreshold)
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("release.blocked_workflow_unavailable", "cluster", "controller"),
			InvariantID: "release.blocked_workflow_unavailable",
			Severity:    sev,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     summary,
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "release_transient_blocked", map[string]string{
				"prometheus_query":     "globular_controller_release_transient_blocked",
				"blocked_releases":     fmt.Sprintf("%.0f", blocked),
				"stuck_threshold":      fmt.Sprintf("%.0f", stuckReleaseThreshold),
				"last_observed":        snap.PromTS.UTC().Format(time.RFC3339),
				"auto_clear_condition": "gauge drops to 0 (controller re-evaluates after workflow recovery)",
				"classification":       map[bool]string{true: "real-sustained", false: "stale-or-low"}[blocked >= stuckReleaseThreshold],
			})},
			InvariantStatus: status,
		})
	}

	if outdated, ok := snap.PromMetrics["controller_leader_outdated"]; ok && outdated > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("controller_leader_outdated", "cluster", "controller"),
			InvariantID: "controller_leader_outdated",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     "Controller leader is behind the target build and must hand off leadership to self-update",
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "controller_leader_outdated", map[string]string{
				"value":     fmt.Sprintf("%.0f", outdated),
				"timestamp": snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if noSafe, ok := snap.PromMetrics["controller_no_safe_successor"]; ok && noSafe > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("controller_no_safe_successor", "cluster", "controller"),
			InvariantID: "controller_no_safe_successor",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     "Controller leader cannot resign: no follower is currently a safe successor at target build",
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "controller_no_safe_successor", map[string]string{
				"value":     fmt.Sprintf("%.0f", noSafe),
				"timestamp": snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	// ── Reconcile lane isolation signals (Phase: starvation prevention) ──────

	// Fire on RECENT timeouts, not on the lifetime total.
	//
	// This finding's own wording is "timed out N time(s)" — an event-recency
	// diagnostic, not a statement about current lane health. It used to read
	// the monotonic lifetime counter and fire on `> 0`, which a counter can
	// never undo: one transient timeout became a permanent ERROR. Observed
	// 2026-08-13 — a single cluster_reconcile timeout at 00:34:44 during a
	// certification run held this at ERROR indefinitely while the lane's own
	// record said {"phase":"OK","running":false}. The two disagreed because
	// they answer different questions, and the rule was answering neither.
	//
	// The 15m window mirrors workflow_dispatch_rejected_rate_15m above and
	// auto-clears, so the finding retires itself the way the condition that
	// raised it does. The lifetime total is preserved as evidence rather than
	// reset — deleting telemetry to clear a doctor finding would destroy the
	// history that makes "has this lane ever timed out" answerable.
	//
	// Gating on lane phase != OK was considered and rejected: it would change
	// what the finding MEANS (current health rather than recent events) while
	// keeping its text, which is how a diagnostic quietly starts lying.
	if recent, ok := snap.PromMetrics["reconcile_lane_timeouts_cluster_15m"]; ok && recent > 0 {
		lifetime := snap.PromMetrics["reconcile_lane_timeouts_cluster"]
		findings = append(findings, Finding{
			FindingID:   FindingID("reconcile.lane_timeout", "cluster", "cluster_reconcile"),
			InvariantID: "reconcile.lane_timeout",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "control_plane",
			EntityRef:   "cluster_reconcile",
			Summary:     fmt.Sprintf("Reconcile lane cluster_reconcile timed out %.0f time(s) in the last 15m — lane execution exceeded timeout and was marked degraded", recent),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "reconcile_lane_timeouts_total", map[string]string{
				"lane":             "cluster_reconcile",
				"timeouts_15m":     fmt.Sprintf("%.0f", recent),
				"timeouts_ever":    fmt.Sprintf("%.0f", lifetime),
				"prometheus_query": `sum(increase(globular_controller_reconcile_lane_timeouts_total{lane="cluster_reconcile"}[15m]))`,
				"auto_clear":       "no lane timeout within the 15m window",
				"timestamp":        snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if blocked, ok := snap.PromMetrics["reconcile_lane_blocked_cluster"]; ok && blocked > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("reconcile.critical_lane_blocked", "cluster", "cluster_reconcile"),
			InvariantID: "reconcile.critical_lane_blocked",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "control_plane",
			EntityRef:   "cluster_reconcile",
			Summary:     "Critical reconcile lane cluster_reconcile is currently blocked/degraded",
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "reconcile_blocked_phase", map[string]string{
				"phase":     "cluster_reconcile",
				"blocked":   fmt.Sprintf("%.0f", blocked),
				"timestamp": snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if blocked, ok := snap.PromMetrics["reconcile_lane_blocked_projections"]; ok && blocked > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("reconcile.lane_blocked", "cluster", "projections"),
			InvariantID: "reconcile.lane_blocked",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "projections",
			Summary:     "Projection reconcile lane is currently blocked/degraded (isolated lane; other lanes should remain healthy)",
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "reconcile_blocked_phase", map[string]string{
				"phase":     "projections",
				"blocked":   fmt.Sprintf("%.0f", blocked),
				"timestamp": snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if blocked, ok := snap.PromMetrics["reconcile_lane_blocked_release_bridge"]; ok && blocked > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("reconcile.lane_blocked", "cluster", "release_bridge"),
			InvariantID: "reconcile.lane_blocked",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "release_bridge",
			Summary:     "Release bridge lane is currently blocked/degraded",
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "reconcile_blocked_phase", map[string]string{
				"phase":     "release_bridge",
				"blocked":   fmt.Sprintf("%.0f", blocked),
				"timestamp": snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if blocked, ok := snap.PromMetrics["reconcile_lane_blocked_drift"]; ok && blocked > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("reconcile.lane_blocked", "cluster", "drift_reconcile"),
			InvariantID: "reconcile.lane_blocked",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "drift_reconcile",
			Summary:     "Drift reconcile lane is currently blocked/degraded",
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "reconcile_blocked_phase", map[string]string{
				"phase":     "drift_reconcile",
				"blocked":   fmt.Sprintf("%.0f", blocked),
				"timestamp": snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	// ── Dependency cache watch health (Phase A-C depcache) ──────────────

	if inactive, ok := snap.PromMetrics["depcache_watch_inactive"]; ok && inactive > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("depcache.watch_inactive", "cluster", "controller"),
			InvariantID: "depcache.watch_inactive",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     fmt.Sprintf("%.0f dependency cache watch(es) are inactive — etcd invalidation events are not being received; cache may serve stale data", inactive),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "depcache_watch", map[string]string{
				"inactive_watches": fmt.Sprintf("%.0f", inactive),
				"timestamp":        snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if errors5m, ok := snap.PromMetrics["depcache_watch_errors_5m"]; ok && errors5m > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("depcache.watch_errors", "cluster", "controller"),
			InvariantID: "depcache.watch_errors",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     fmt.Sprintf("%.0f dependency cache watch error(s) in the last 5 minutes — etcd connectivity may be degraded", errors5m),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "depcache_watch_errors", map[string]string{
				"errors_5m": fmt.Sprintf("%.0f", errors5m),
				"timestamp": snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	// ── xDS config generation tracking (Phase F) ─────────────────────────

	xdsApplied, hasApplied := snap.PromMetrics["xds_config_applied_total"]
	xdsEvents, hasEvents := snap.PromMetrics["xds_config_events_total"]
	xdsLastUnix, hasLastUnix := snap.PromMetrics["xds_last_applied_unix"]

	// Investigated 2026-05-21 (single-node ryzen cluster): the rule
	// previously fired WARN + INVARIANT_FAIL whenever events_total > 0 and
	// applied_total == 0. That comparison is semantically wrong:
	//
	//   - xds_config_events_total increments on every ClusterNetwork /
	//     ServiceDesiredVersion *watch event* (reconcile_runtime.go:187,202).
	//     Every deploy bumps it.
	//   - xds_config_applied_total only increments when the controller's
	//     own node plan injects a `restart globular-xds.service` action
	//     because the rendered config hash changed
	//     (reconcile_nodes.go:1189). That is a NARROW signal — it does
	//     NOT mean "xDS pushed config to Envoy"; xDS pushes its own
	//     snapshots over gRPC continuously, independent of this counter.
	//
	// On a stable cluster (xds binary unchanged, rendered config hash
	// stable) applied_total stays at 0 forever while events_total grows.
	// That is normal, not drift.
	//
	// Until we have a real "xDS apply path stuck" signal, surface this
	// shape as INFO + INVARIANT_PASS so operators see the counters but
	// no incident is opened. The Summary explicitly says "advisory" so
	// the next reader doesn't mistake it for a real failure.
	//
	// Real evidence trail for a future, narrower rule:
	//   - globular-xds.service systemd state == active
	//   - globular-envoy.service systemd state == active
	//   - xds_snapshot_push_failures_total == 0 (TBD; metric does not exist yet)
	//   - elapsed since last successful xDS gRPC push > N seconds
	//
	// If xDS is *intentionally* disabled (no ClusterNetwork resource
	// configured), the events_total never increments and we never enter
	// this branch — no further policy needed.
	if hasApplied && hasEvents && xdsEvents > 0 && xdsApplied == 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("xds.no_applies", "cluster", "controller"),
			InvariantID: "xds.no_applies",
			Severity:    cluster_doctorpb.Severity_SEVERITY_INFO,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     fmt.Sprintf("xDS counter advisory — %.0f reconcile event(s), 0 controller-driven xds restarts. This is normal on a stable cluster (xds service pushes snapshots over gRPC independent of this counter). Real apply-path-stuck signal needs a different metric (TODO).", xdsEvents),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "xds_generation", map[string]string{
				"events_total":                            fmt.Sprintf("%.0f", xdsEvents),
				"applied_total":                           fmt.Sprintf("%.0f", xdsApplied),
				"classification":                          "advisory-metric-mismatch",
				"events_metric_meaning":                   "watch events that *may* require xDS rebuild (every deploy)",
				"applied_metric_meaning":                  "controller injected globular-xds.service restart action because rendered hash changed",
				"why_zero_is_normal":                      "xds service pushes snapshots over gRPC continuously — does not increment this counter",
				"recommended_evidence_for_real_xds_stuck": "globular-xds.service systemd state + xds gRPC push failure metric (TODO)",
				"timestamp":                               snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_PASS,
		})
	} else if hasApplied && xdsApplied > 0 && hasLastUnix && xdsLastUnix > 0 {
		age := snap.PromTS.Unix() - int64(xdsLastUnix)
		findings = append(findings, Finding{
			FindingID:   FindingID("xds.last_applied", "cluster", "controller"),
			InvariantID: "xds.last_applied",
			Severity:    cluster_doctorpb.Severity_SEVERITY_INFO,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     fmt.Sprintf("xDS config applied %.0f time(s); last apply %ds ago", xdsApplied, age),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "xds_generation", map[string]string{
				"applied_total":    fmt.Sprintf("%.0f", xdsApplied),
				"last_apply_age_s": fmt.Sprintf("%d", age),
				"timestamp":        snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_PASS,
		})
	}

	return findings
}
