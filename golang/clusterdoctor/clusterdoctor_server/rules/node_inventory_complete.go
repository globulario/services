package rules

import (
	"fmt"

	clusterdoctorpb "github.com/globulario/services/golang/clusterdoctor/clusterdoctorpb"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/collector"
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
			Severity:    clusterdoctorpb.Severity_SEVERITY_ERROR,
			Category:    "inventory",
			EntityRef:   nodeID,
			Summary: fmt.Sprintf("Node %s inventory incomplete (components=%d, units=%d)",
				nodeID, componentCount, unitCount),
			Evidence: []*clusterdoctorpb.Evidence{
				kvEvidence("nodeagent", "GetInventory", map[string]string{
					"node_id":         nodeID,
					"component_count": fmt.Sprintf("%d", componentCount),
					"unit_count":      fmt.Sprintf("%d", unitCount),
				}),
			},
			Remediation: []*clusterdoctorpb.RemediationStep{
				step(1, "Re-trigger inventory scan on the node agent", ""),
				step(2, "Restart the node agent if scan has stalled: systemctl restart globular-nodeagent", ""),
				step(3, "Check node agent logs for scan errors: journalctl -u globular-nodeagent -n 200", ""),
			},
			InvariantStatus: clusterdoctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
