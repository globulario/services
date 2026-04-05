package rules

import (
	"fmt"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
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

		evidence := []*cluster_doctorpb.Evidence{
			kvEvidence("cluster_controller", "GetClusterHealthV1", map[string]string{
				"node_id":              nodeID,
				"desired_hash":         desired,
				"applied_hash":         applied,
				"can_apply_privileged": fmt.Sprintf("%v", canPriv),
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
			Severity:        cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:        "drift",
			EntityRef:       nodeID,
			Summary:         summary,
			Evidence:        evidence,
			Remediation:     remediation,
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
