// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.node_inventory_complete
// @awareness file_role=doctor_rule_classifying_per_node_inventory_completeness_before_treating_other_findings_as_authoritative
// @awareness implements=globular.platform:intent.runtime_observation_must_not_mutate_desired
// @awareness implements=globular.platform:intent.runtime_health.requires_live_observation
// @awareness risk=critical
package rules

// node_inventory_complete.go — DIAGNOSTIC ONLY. Establishes
// whether a node's inventory snapshot is complete enough for
// other rules to draw conclusions from. A partial inventory
// MUST NOT be treated as authoritative absence — that's the
// fm.industry.missing_inventory_misclassified_as_down failure
// mode, which has cascaded into false-quorum-loss findings
// before.
//
// This rule is foundational: other rules consult it to decide
// whether to fire or to downgrade their severity. Adding any
// auto-recovery here (e.g. "re-trigger collector if incomplete")
// would mask the underlying collector bug rather than expose it.

import (
	"fmt"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type nodeInventoryComplete struct{}

func (nodeInventoryComplete) ID() string       { return "node.inventory.complete" }
func (nodeInventoryComplete) Category() string { return "inventory" }
func (nodeInventoryComplete) Scope() string    { return "node" }

func (nodeInventoryComplete) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		inv, ok := snap.Inventories[nodeID]

		// If inventory RPC failed entirely, skip (DataError is already recorded in header).
		if !ok {
			continue
		}

		componentCount := len(inv.GetComponents())
		unitCount := len(inv.GetUnits())

		// Primary signal: explicit InventoryComplete flag (if the field exists).
		// The current nodeagent Inventory proto does not yet have this field,
		// so we fall back to array emptiness as a heuristic.
		// TODO: promote to explicit flag once nodeagent adds inventory_complete bool.
		incomplete := componentCount == 0 || unitCount == 0

		if !incomplete {
			continue
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("node.inventory.complete", nodeID, nodeID),
			InvariantID: "node.inventory.complete",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "inventory",
			EntityRef:   nodeID,
			Summary: fmt.Sprintf("Node %s inventory incomplete (components=%d, units=%d)",
				nodeID, componentCount, unitCount),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetInventory", map[string]string{
					"node_id":         nodeID,
					"component_count": fmt.Sprintf("%d", componentCount),
					"unit_count":      fmt.Sprintf("%d", unitCount),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				actionStep(
					1,
					"Restart node-agent to force a fresh inventory scan",
					fmt.Sprintf("globular doctor remediate %s --step 0",
						FindingID("node.inventory.complete", nodeID, nodeID)),
					systemctlRestartAction("globular-node-agent.service", nodeID),
				),
				step(2, "Check node agent logs for scan errors: journalctl -u globular-node-agent -n 200", ""),
				step(3, "If scan fails repeatedly, the node-agent may be missing privileges for systemctl list-units", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
