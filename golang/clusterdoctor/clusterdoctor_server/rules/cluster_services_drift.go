package rules

import (
	"fmt"

	clusterdoctorpb "github.com/globulario/services/golang/clusterdoctor/clusterdoctorpb"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/collector"
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
		if desired == "" || desired == applied {
			continue
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("cluster.services.drift", nodeID, desired),
			InvariantID: "cluster.services.drift",
			Severity:    clusterdoctorpb.Severity_SEVERITY_WARN,
			Category:    "drift",
			EntityRef:   nodeID,
			Summary:     fmt.Sprintf("Node %s services state hash mismatch (desired ≠ applied)", nodeID),
			Evidence: []*clusterdoctorpb.Evidence{
				kvEvidence("clustercontroller", "GetClusterHealthV1", map[string]string{
					"node_id":       nodeID,
					"desired_hash":  desired,
					"applied_hash":  applied,
				}),
			},
			Remediation: []*clusterdoctorpb.RemediationStep{
				step(1, "Trigger reconciliation for node "+nodeID+" to converge desired state", "globular doctor drift"),
				step(2, "Inspect current plan to understand what changes are pending", ""),
				step(3, "Check for failed previous plans that may have left state partially applied", "globular doctor node "+nodeID),
			},
			InvariantStatus: clusterdoctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
