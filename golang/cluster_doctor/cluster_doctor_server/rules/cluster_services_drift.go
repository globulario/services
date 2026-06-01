// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules
// @awareness file_role=cluster_services_desired_vs_running_drift_rule
// @awareness enforces=globular.platform:invariant.state.runtime_not_desired
// @awareness risk=high
package rules

import (
	"fmt"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type clusterServicesDrift struct{}

func (clusterServicesDrift) ID() string       { return "cluster.services.drift" }
func (clusterServicesDrift) Category() string { return "drift" }
func (clusterServicesDrift) Scope() string    { return "cluster" }

func (clusterServicesDrift) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for nodeID, nh := range snap.NodeHealths {
		desired := nh.GetDesiredServicesHash()
		applied := nh.GetAppliedServicesHash()
		if desired == "" || desired == "services:none" || desired == applied {
			continue
		}

		canPriv := nh.GetCanApplyPrivileged()

		summary := fmt.Sprintf("Node %s services state hash mismatch (desired ≠ applied)", nodeID)
		if !canPriv {
			summary = fmt.Sprintf("Node %s services state hash mismatch — node lacks privilege for systemd operations", nodeID)
		}

		driftAge := snap.NodeDriftAge[nodeID]
		sev := driftSeverity(driftAge)

		ageDesc := ""
		if driftAge > 0 {
			ageDesc = fmt.Sprintf(" (drift age: %s)", driftAge.Round(time.Second))
		}

		evidence := []*cluster_doctorpb.Evidence{
			kvEvidence("cluster_controller", "GetClusterHealthV1", map[string]string{
				"node_id":              nodeID,
				"desired_hash":         desired,
				"applied_hash":         applied,
				"can_apply_privileged": fmt.Sprintf("%v", canPriv),
				"drift_age_seconds":    fmt.Sprintf("%.0f", driftAge.Seconds()),
			}),
		}

		remediation := []*cluster_doctorpb.RemediationStep{
			step(1, "Trigger reconciliation for node "+nodeID+" to converge desired state", "globular doctor drift"),
			step(2, "Inspect current plan to understand what changes are pending", ""),
			step(3, "Check for failed previous plans that may have left state partially applied", "globular doctor node "+nodeID),
		}

		if !canPriv {
			remediation = []*cluster_doctorpb.RemediationStep{
				step(1, "Node lacks privilege for systemd operations. Ensure the globular user has sudoers rules for systemctl.", ""),
				actionStep(
					2,
					"Restart the node agent to pick up the updated sudo permissions",
					fmt.Sprintf("globular doctor remediate %s --step 1",
						FindingID("cluster.services.drift", nodeID, desired)),
					systemctlRestartAction("globular-node-agent.service", nodeID),
				),
			}
		}

		findings = append(findings, Finding{
			FindingID:       FindingID("cluster.services.drift", nodeID, desired),
			InvariantID:     "cluster.services.drift",
			Severity:        sev,
			Category:        "drift",
			EntityRef:       nodeID,
			Summary:         summary + ageDesc,
			Evidence:        evidence,
			Remediation:     remediation,
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// driftSeverity escalates severity based on how long the drift has persisted.
//   - Unknown age (0) or < 2 min → WARN (transient, may self-heal)
//   - 2–5 min → WARN (still recent)
//   - > 5 min → ERROR (convergence loop should have fixed this)
func driftSeverity(age time.Duration) cluster_doctorpb.Severity {
	const errorThreshold = 5 * time.Minute
	if age > errorThreshold {
		return cluster_doctorpb.Severity_SEVERITY_ERROR
	}
	return cluster_doctorpb.Severity_SEVERITY_WARN
}
