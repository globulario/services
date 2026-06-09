// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.node_reachable
// @awareness file_role=doctor_rule_classifying_per_node_agent_reachability_via_heartbeat_freshness
// @awareness implements=globular.platform:intent.runtime_observation_must_not_mutate_desired
// @awareness implements=globular.platform:intent.runtime_health.requires_live_observation
// @awareness risk=high
package rules

// node_reachable.go — DIAGNOSTIC ONLY. Per-node availability
// rule based on heartbeat freshness (NodeState.LastSeen).
// Stale heartbeat surfaces a finding; the operator decides
// whether to investigate or initiate removal.
//
// MUST NOT trigger removal. The asymmetry — "unreachable"
// surfaces a finding but never auto-evicts — is exactly what
// prevents network partitions and slow disks from cascading
// into destructive cluster shrink (see
// node_removal_requests.go on the controller side: removal
// requires an explicit, audited request record).

import (
	"fmt"
	"time"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type nodeReachable struct{}

func (nodeReachable) ID() string       { return "node.reachable" }
func (nodeReachable) Category() string { return "availability" }
func (nodeReachable) Scope() string    { return "node" }

func (nodeReachable) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding
	// cluster_controller unreachable → empty snap.Nodes is "unknown", not "no
	// nodes". Honors deadline_exceeded_must_not_drive_definitive_node_state: a
	// source error must not be read as nodes being absent or down.
	if snap.HadError("cluster_controller", "ListNodes") {
		return findings
	}
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
				// Step 1 is a structured action — restart the node-agent on
				// the unreachable node. LOW risk because the unit is
				// Globular-managed and restart is idempotent. If the node
				// is genuinely offline, the dialer returns a clean error.
				actionStep(
					1,
					"Restart globular-node-agent on "+nodeID,
					"globular doctor remediate "+FindingID("node.reachable", nodeID, nodeID)+" --step 0",
					systemctlRestartAction("globular-node-agent.service", nodeID),
				),
				step(2, "Check node agent logs: journalctl -u globular-node-agent -n 100", ""),
				step(3, "Verify network connectivity from controller to node "+nodeID, ""),
				step(4, "If node is permanently gone, remove it from the cluster", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
