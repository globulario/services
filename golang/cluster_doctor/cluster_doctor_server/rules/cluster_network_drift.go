package rules

import (
	"fmt"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type clusterNetworkDrift struct{}

func (clusterNetworkDrift) ID() string       { return "cluster.network.drift" }
func (clusterNetworkDrift) Category() string { return "drift" }
func (clusterNetworkDrift) Scope() string    { return "cluster" }

func (clusterNetworkDrift) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for nodeID, nh := range snap.NodeHealths {
		desired := nh.GetDesiredNetworkHash()
		applied := nh.GetAppliedNetworkHash()
		if desired == "" || desired == applied {
			continue
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("cluster.network.drift", nodeID, desired),
			InvariantID: "cluster.network.drift",
			Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
			Category:    "drift",
			EntityRef:   nodeID,
			Summary:     fmt.Sprintf("Node %s network state hash mismatch (desired ≠ applied)", nodeID),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_controller", "GetClusterHealthV1", map[string]string{
					"node_id":      nodeID,
					"desired_hash": desired,
					"applied_hash": applied,
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Trigger network reconciliation for node "+nodeID, ""),
				step(2, "Inspect cluster network config for recent changes", ""),
				step(3, "Check nodeagent logs for network apply errors: journalctl -u globular-nodeagent -n 200", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
