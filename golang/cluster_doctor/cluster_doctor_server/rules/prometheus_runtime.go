package rules

import (
	"fmt"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

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

	if age, ok := snap.PromMetrics["controller_loop_heartbeat_age"]; ok && age > 180 {
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

	if age, ok := snap.PromMetrics["node_heartbeat_age_max"]; ok && age > 120 {
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
				"timestamp":        snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if mismatches, ok := snap.PromMetrics["drift_kind_mismatch"]; ok && mismatches > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("desired.kind_mismatch", "cluster", "controller"),
			InvariantID: "desired.kind_mismatch",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     fmt.Sprintf("Desired-state kind mismatch blocked %d dispatch(es) — SERVICE desired but INFRASTRUCTURE in repo", int(mismatches)),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "kind_mismatch", map[string]string{
				"total_blocked": fmt.Sprintf("%.0f", mismatches),
				"timestamp":    snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if opens, ok := snap.PromMetrics["reconcile_circuit_open"]; ok && opens > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("cluster.reconcile_circuit_open", "cluster", "controller"),
			InvariantID: "cluster.reconcile_circuit_open",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     fmt.Sprintf("Reconcile circuit breaker opened %d time(s) — periodic reconcile suspended due to repeated failures", int(opens)),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "circuit_breaker", map[string]string{
				"total_opens": fmt.Sprintf("%.0f", opens),
				"timestamp":  snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if rejected, ok := snap.PromMetrics["workflow_dispatch_rejected"]; ok && rejected > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("workflow.backend_pressure", "cluster", "workflow"),
			InvariantID: "workflow.backend_pressure",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "workflow",
			Summary:     fmt.Sprintf("Workflow health gate rejected %d dispatch(es) — backend under pressure", int(rejected)),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "backend_pressure", map[string]string{
				"total_rejected": fmt.Sprintf("%.0f", rejected),
				"timestamp":     snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
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
				"timestamp":   snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}

	if blocked, ok := snap.PromMetrics["release_transient_blocked"]; ok && blocked > 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("release.blocked_workflow_unavailable", "cluster", "controller"),
			InvariantID: "release.blocked_workflow_unavailable",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     fmt.Sprintf("%.0f release(s) blocked in transient retry backoff — workflow service was unreachable during last dispatch attempt", blocked),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "release_transient_blocked", map[string]string{
				"blocked_releases": fmt.Sprintf("%.0f", blocked),
				"timestamp":       snap.PromTS.UTC().Format(time.RFC3339),
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

	if hasApplied && hasEvents && xdsEvents > 0 && xdsApplied == 0 {
		findings = append(findings, Finding{
			FindingID:   FindingID("xds.no_applies", "cluster", "controller"),
			InvariantID: "xds.no_applies",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "control_plane",
			EntityRef:   "controller",
			Summary:     fmt.Sprintf("xDS config: %.0f event(s) received but no config has been applied — rendered config hash may be stuck or xDS renderer is not producing output", xdsEvents),
			Evidence: []*cluster_doctorpb.Evidence{kvEvidence("prometheus", "xds_generation", map[string]string{
				"events_total":  fmt.Sprintf("%.0f", xdsEvents),
				"applied_total": fmt.Sprintf("%.0f", xdsApplied),
				"timestamp":     snap.PromTS.UTC().Format(time.RFC3339),
			})},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
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
