package rules

import (
	"fmt"

	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
)

type nodeUnitsRunning struct{}

func (nodeUnitsRunning) ID() string       { return "node.systemd.units_running" }
func (nodeUnitsRunning) Category() string { return "systemd" }
func (nodeUnitsRunning) Scope() string    { return "node" }

func (nodeUnitsRunning) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		inv, ok := snap.Inventories[nodeID]
		if !ok {
			continue
		}

		for _, u := range inv.GetUnits() {
			state := NormalizeUnitState(u.GetState())
			if state != UnitStateFailed && state != UnitStateInactive {
				continue
			}

			severity := cluster_doctorpb.Severity_SEVERITY_WARN
			if state == UnitStateFailed {
				severity = cluster_doctorpb.Severity_SEVERITY_ERROR
			}

			findings = append(findings, Finding{
				FindingID:   FindingID("node.systemd.units_running", nodeID, u.GetName()),
				InvariantID: "node.systemd.units_running",
				Severity:    severity,
				Category:    "systemd",
				EntityRef:   fmt.Sprintf("%s/%s", nodeID, u.GetName()),
				Summary: fmt.Sprintf("Unit %s on node %s is %s (expected active)",
					u.GetName(), nodeID, state),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("node_agent", "GetInventory", map[string]string{
						"node_id":          nodeID,
						"unit_name":        u.GetName(),
						"raw_state":        u.GetState(),
						"normalized_state": state.String(),
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, fmt.Sprintf("Restart unit: systemctl restart %s", u.GetName()), ""),
					step(2, fmt.Sprintf("Check journal: journalctl -u %s -n 100", u.GetName()), ""),
					step(3, "If repeatedly failing, check unit file and dependencies", ""),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}
