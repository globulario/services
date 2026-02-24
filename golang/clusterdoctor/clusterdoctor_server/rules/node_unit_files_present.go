package rules

import (
	"fmt"

	clusterdoctorpb "github.com/globulario/services/golang/clusterdoctor/clusterdoctorpb"
	"github.com/globulario/services/golang/clusterdoctor/clusterdoctor_server/collector"
)

type nodeUnitFilesPresent struct{}

func (nodeUnitFilesPresent) ID() string       { return "node.systemd.unit_files_present" }
func (nodeUnitFilesPresent) Category() string { return "systemd" }
func (nodeUnitFilesPresent) Scope() string    { return "node" }

func (nodeUnitFilesPresent) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		inv, ok := snap.Inventories[nodeID]
		if !ok {
			continue
		}

		for _, u := range inv.GetUnits() {
			if NormalizeUnitState(u.GetState()) != UnitStateNotFound {
				continue
			}
			findings = append(findings, Finding{
				FindingID:   FindingID("node.systemd.unit_files_present", nodeID, u.GetName()),
				InvariantID: "node.systemd.unit_files_present",
				Severity:    clusterdoctorpb.Severity_SEVERITY_ERROR,
				Category:    "systemd",
				EntityRef:   fmt.Sprintf("%s/%s", nodeID, u.GetName()),
				Summary:     fmt.Sprintf("Unit file %s not found on node %s", u.GetName(), nodeID),
				Evidence: []*clusterdoctorpb.Evidence{
					kvEvidence("nodeagent", "GetInventory", map[string]string{
						"node_id":   nodeID,
						"unit_name": u.GetName(),
						"raw_state": u.GetState(),
					}),
				},
				Remediation: []*clusterdoctorpb.RemediationStep{
					step(1, fmt.Sprintf("Reinstall the package providing unit %s", u.GetName()), ""),
					step(2, "Check repository connectivity and package availability", ""),
					step(3, "Trigger reconciliation to re-deploy the service", "globular doctor node "+nodeID),
				},
				InvariantStatus: clusterdoctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}
