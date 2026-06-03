// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.node_units_running
// @awareness file_role=doctor_rule_classifying_per_node_systemd_unit_running_state_vs_desired
// @awareness implements=globular.platform:intent.runtime_observation_must_not_mutate_desired
// @awareness implements=globular.platform:intent.runtime_health.requires_live_observation
// @awareness risk=high
package rules

// node_units_running.go — DIAGNOSTIC ONLY. Per-node Layer-4
// runtime observation: which systemd units are active vs the
// installed-state record. Findings here drive the
// installed_state_runtime_mismatch correlation in the
// companion rule.
//
// MUST NOT restart units. Restart authority belongs to the
// node-agent action handlers (under the systemd unit
// allowlist); a doctor rule that restarts units bypasses the
// allowlist and the audit trail. Surface the gap; the operator
// (or the auto-heal pipeline through its approval gates)
// decides whether to restart.

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

type nodeUnitsRunning struct{}

func (nodeUnitsRunning) ID() string       { return "node.systemd.units_running" }
func (nodeUnitsRunning) Category() string { return "systemd" }
func (nodeUnitsRunning) Scope() string    { return "node" }

func (nodeUnitsRunning) Evaluate(snap *collector.Snapshot, cfg Config) []Finding {
	var findings []Finding

	// keepalived is ingress-gated. When /globular/ingress/v1/spec is
	// "disabled" the unit must be installed-but-inactive — see the
	// installed_state_runtime_mismatch rule for the full rationale. This
	// secondary check exists because nodeUnitsRunning iterates the
	// inventory directly and would otherwise emit a parallel WARN on the
	// same package, doubling the dashboard noise on every healthy Day-0
	// cluster.
	ingressDisabled := IngressIsDisabled(snap)

	for _, node := range snap.Nodes {
		nodeID := node.GetNodeId()
		inv, ok := snap.Inventories[nodeID]
		if !ok {
			continue
		}

		// Minio non-member check is per-node; compute once per node.
		minioNonMember := nodeIsMinioNonMember(nodeID, snap)

		for _, u := range inv.GetUnits() {
			state := NormalizeUnitState(u.GetState())
			if state != UnitStateFailed && state != UnitStateInactive {
				continue
			}
			if u.GetName() == "keepalived.service" && ingressDisabled {
				continue
			}
			// minio and sidekick are inactive by design on non-member nodes.
			if isMinioOrSidekickUnit(u.GetName()) && minioNonMember {
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
