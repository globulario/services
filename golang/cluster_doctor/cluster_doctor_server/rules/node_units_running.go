package rules

import (
	"fmt"
	"strings"

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

			findingID := FindingID("node.systemd.units_running", nodeID, u.GetName())

			// Step 1: if the unit is Globular-managed, emit a structured
			// SYSTEMCTL_RESTART action (LOW risk, auto-executable). For
			// non-managed units the executor refuses anyway — keep them
			// text-only so operators remediate manually.
			var step1 *cluster_doctorpb.RemediationStep
			if strings.HasPrefix(u.GetName(), "globular-") {
				step1 = actionStep(
					1,
					fmt.Sprintf("Restart unit: systemctl restart %s", u.GetName()),
					fmt.Sprintf("globular doctor remediate %s --step 0", findingID),
					systemctlRestartAction(u.GetName(), nodeID),
				)
			} else {
				step1 = step(1, fmt.Sprintf("Restart unit: systemctl restart %s", u.GetName()), "")
			}

			findings = append(findings, Finding{
				FindingID:   findingID,
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
					step1,
					step(2, fmt.Sprintf("Check journal: journalctl -u %s -n 100", u.GetName()), ""),
					step(3, "If repeatedly failing, check unit file and dependencies", ""),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}
