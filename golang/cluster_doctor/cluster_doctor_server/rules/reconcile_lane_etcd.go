package rules

import (
	"fmt"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type reconcileLaneStatusEtcd struct{}

func (reconcileLaneStatusEtcd) ID() string       { return "reconcile.lane_status_etcd" }
func (reconcileLaneStatusEtcd) Category() string { return "control_plane" }
func (reconcileLaneStatusEtcd) Scope() string    { return "cluster" }

func (reconcileLaneStatusEtcd) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	// If Prometheus lane metrics are present, let prometheus_runtime own these
	// findings to avoid duplicates.
	if snap.PromMetrics != nil {
		if _, ok := snap.PromMetrics["reconcile_lane_blocked_cluster"]; ok {
			return nil
		}
	}
	var findings []Finding
	for lane, raw := range snap.ReconcileLaneStatus {
		phase, _ := raw["phase"].(string)
		lastErr, _ := raw["last_error"].(string)
		switch phase {
		case "TIMEOUT":
			findings = append(findings, Finding{
				FindingID:   FindingID("reconcile.lane_timeout", "cluster", lane),
				InvariantID: "reconcile.lane_timeout",
				Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
				Category:    "control_plane",
				EntityRef:   lane,
				Summary:     fmt.Sprintf("Reconcile lane %s timed out: %s", lane, lastErr),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("etcd", "Get(/globular/controller/reconcile/lanes/<lane>)", map[string]string{
						"lane":       lane,
						"phase":      phase,
						"last_error": lastErr,
					}),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		case "BLOCKED":
			sev := cluster_doctorpb.Severity_SEVERITY_WARN
			invariant := "reconcile.lane_blocked"
			if lane == "cluster_reconcile" {
				sev = cluster_doctorpb.Severity_SEVERITY_CRITICAL
				invariant = "reconcile.critical_lane_blocked"
			}
			findings = append(findings, Finding{
				FindingID:   FindingID(invariant, "cluster", lane),
				InvariantID: invariant,
				Severity:    sev,
				Category:    "control_plane",
				EntityRef:   lane,
				Summary:     fmt.Sprintf("Reconcile lane %s is blocked: %s", lane, lastErr),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("etcd", "Get(/globular/controller/reconcile/lanes/<lane>)", map[string]string{
						"lane":       lane,
						"phase":      phase,
						"last_error": lastErr,
					}),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}
