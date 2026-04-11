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

	return findings
}
