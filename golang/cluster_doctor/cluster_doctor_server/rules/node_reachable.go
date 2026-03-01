package rules

import (
	"fmt"
	"time"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

type nodeReachable struct{}

func (nodeReachable) ID() string       { return "node.reachable" }
func (nodeReachable) Category() string { return "availability" }
func (nodeReachable) Scope() string    { return "node" }

func (nodeReachable) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding
	now := time.Now()

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		lastSeen := node.GetLastSeen().AsTime()
		age := now.Sub(lastSeen)
		unreachable := node.GetStatus() == "unreachable" || age > cfg.HeartbeatStale

		if !unreachable {
			continue
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("node.reachable", nodeID, nodeID),
			InvariantID: "node.reachable",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "availability",
			EntityRef:   nodeID,
			Summary: fmt.Sprintf("Node %s is unreachable (last seen %s ago, threshold %s)",
				nodeID, age.Round(time.Second), cfg.HeartbeatStale),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("cluster_controller", "ListNodes", map[string]string{
					"node_id":        nodeID,
					"last_seen":      lastSeen.UTC().String(),
					"age_sec":        fmt.Sprintf("%d", int(age.Seconds())),
					"status":         node.GetStatus(),
					"agent_endpoint": node.GetAgentEndpoint(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Verify node agent is running on the node", "globular doctor node "+nodeID),
				step(2, "Check node agent logs: journalctl -u globular-nodeagent -n 100", ""),
				step(3, "Verify network connectivity from controller to node "+nodeID, ""),
				step(4, "If node is permanently gone, remove it from the cluster", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
