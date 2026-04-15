package rules

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

type subsystemStuck struct{}

func (subsystemStuck) ID() string       { return "subsystem.stuck" }
func (subsystemStuck) Category() string { return "subsystem" }
func (subsystemStuck) Scope() string    { return "node" }

func (subsystemStuck) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding

	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		shResp, ok := snap.SubsystemHealth[nid]
		if !ok {
			continue
		}
		hostname := node.GetIdentity().GetHostname()

		for _, sub := range shResp.GetSubsystems() {
			if sub.GetState() == node_agentpb.SubsystemState_SUBSYSTEM_STATE_HEALTHY ||
				sub.GetState() == node_agentpb.SubsystemState_SUBSYSTEM_STATE_STARTING ||
				sub.GetState() == node_agentpb.SubsystemState_SUBSYSTEM_STATE_UNSPECIFIED {
				continue
			}

			sev := cluster_doctorpb.Severity_SEVERITY_WARN
			if sub.GetState() == node_agentpb.SubsystemState_SUBSYSTEM_STATE_FAILED {
				sev = cluster_doctorpb.Severity_SEVERITY_ERROR
			}

			lastErr := sub.GetLastError()
			if lastErr == "" {
				lastErr = "(no error message)"
			}

			// Build evidence metadata.
			evidence := map[string]string{
				"node_id":     nid,
				"subsystem":   sub.GetName(),
				"state":       sub.GetState().String(),
				"error_count": fmt.Sprintf("%d", sub.GetErrorCount()),
				"last_error":  lastErr,
			}
			if sub.GetLastTick() != nil {
				evidence["last_tick"] = sub.GetLastTick().AsTime().String()
			}
			for k, v := range sub.GetMetadata() {
				evidence["meta."+k] = v
			}

			findings = append(findings, Finding{
				FindingID:   FindingID("subsystem.stuck", nid, sub.GetName()),
				InvariantID: "subsystem.stuck",
				Severity:    sev,
				Category:    "subsystem",
				EntityRef:   nid,
				Summary: fmt.Sprintf("Node %s (%s) subsystem %q is %s: %s (consecutive errors: %d)",
					hostname, nid, sub.GetName(),
					strings.ToLower(sub.GetState().String()),
					lastErr, sub.GetErrorCount()),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("node_agent", "GetSubsystemHealth", evidence),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					actionStep(
						1,
						fmt.Sprintf("Restart node agent on %s to recover %s subsystem", hostname, sub.GetName()),
						"globular doctor remediate "+FindingID("subsystem.stuck", nid, sub.GetName())+" --step 0",
						systemctlRestartAction("globular-node-agent.service", nid),
					),
					step(2, fmt.Sprintf("Check logs: journalctl -u globular-node-agent -n 100 --grep %s", sub.GetName()), ""),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}
