package rules

import (
	"fmt"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// This file implements the ScyllaDB invariants of the infrastructure truth
// plane. They consume the GetInfraProbe RPC results stored in
// snap.InfraProbes[nodeId] (Phase 1). The probe already did the hard work
// (parse config, attest, observe runtime, derive lifecycle); these rules project
// its structured violations and lifecycle into operator-facing doctor findings.
//
// Two cross-cutting rules from the awareness graph shape this file:
//   - cluster_doctor.rule_goes_silent_when_source_errors: a rule must NOT emit
//     "healthy" (i.e. stay silent) when its source errored. The
//     scylla.probe_required_when_installed rule exists precisely so a missing or
//     failed probe on an installed node becomes a visible finding.
//   - infra.repair_must_target_owner_desired_state_not_manual_file_edit: every
//     remediation points at the config owner, never "hand-edit scylla.yaml".

const scyllaRepairOwner = "Fix the ScyllaDB config owner (post-install renderer + controller-provided desired state); do NOT hand-edit /etc/scylla/scylla.yaml — a render overwrites it."

// scyllaProbeFor returns the scylladb probe result for a node, or nil.
func scyllaProbeFor(snap *collector.Snapshot, nid string) *cluster_controllerpb.InfraProbeResult {
	resp, ok := snap.InfraProbes[nid]
	if !ok || resp == nil {
		return nil
	}
	for _, r := range resp.GetResults() {
		if r.GetComponent() == "scylladb" {
			return r
		}
	}
	return nil
}

// scyllaInstalledFromInventory reports whether the node's inventory/health
// indicates ScyllaDB is installed — used to decide whether a missing probe is a
// problem (installed) or acceptable silence (not installed). This is independent
// of the probe so it still works when the probe itself is absent.
func scyllaInstalledFromInventory(snap *collector.Snapshot, nid string) bool {
	if inv := snap.Inventories[nid]; inv != nil {
		for _, u := range inv.GetUnits() {
			if strings.Contains(strings.ToLower(u.GetName()), "scylla-server") {
				return true
			}
		}
	}
	if health := snap.NodeHealths[nid]; health != nil {
		for name := range health.GetInstalledVersions() {
			if strings.Contains(strings.ToLower(name), "scylladb") {
				return true
			}
		}
	}
	return false
}

func sevFromString(s string) cluster_doctorpb.Severity {
	switch strings.ToUpper(strings.TrimSpace(s)) {
	case "CRITICAL":
		return cluster_doctorpb.Severity_SEVERITY_CRITICAL
	case "ERROR":
		return cluster_doctorpb.Severity_SEVERITY_ERROR
	case "WARN":
		return cluster_doctorpb.Severity_SEVERITY_WARN
	case "INFO":
		return cluster_doctorpb.Severity_SEVERITY_INFO
	default:
		return cluster_doctorpb.Severity_SEVERITY_WARN
	}
}

// ── scylla.loopback_forbidden (CRITICAL, node) ───────────────────────────────

type scyllaLoopbackForbidden struct{}

func (scyllaLoopbackForbidden) ID() string       { return "scylla.loopback_forbidden" }
func (scyllaLoopbackForbidden) Category() string { return "scylla" }
func (scyllaLoopbackForbidden) Scope() string    { return "node" }

func (scyllaLoopbackForbidden) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := scyllaProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		for _, v := range probe.GetViolations() {
			if v.GetId() != "scylla.loopback_forbidden" {
				continue
			}
			findings = append(findings, Finding{
				FindingID:   FindingID("scylla.loopback_forbidden", nid, v.GetEvidence()),
				InvariantID: "scylla.loopback_forbidden",
				Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
				Category:    "scylla",
				EntityRef:   nid,
				Summary:     fmt.Sprintf("Node %s ScyllaDB config has a loopback cluster address: %s", nid, v.GetMessage()),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("node_agent", "GetInfraProbe", map[string]string{
						"node_id":  nid,
						"evidence": v.GetEvidence(),
						"summary":  probe.GetSummary(),
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, v.GetRemediation(), ""),
					step(2, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component scylladb", ""),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}

// ── scylla.config_valid (ERROR, node) ────────────────────────────────────────

type scyllaConfigValid struct{}

func (scyllaConfigValid) ID() string       { return "scylla.config_valid" }
func (scyllaConfigValid) Category() string { return "scylla" }
func (scyllaConfigValid) Scope() string    { return "node" }

func (scyllaConfigValid) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := scyllaProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		for _, v := range probe.GetViolations() {
			if v.GetId() != "scylla.config_valid" {
				continue
			}
			findings = append(findings, Finding{
				FindingID:   FindingID("scylla.config_valid", nid, v.GetEvidence()),
				InvariantID: "scylla.config_valid",
				Severity:    sevFromString(v.GetSeverity()),
				Category:    "scylla",
				EntityRef:   nid,
				Summary:     fmt.Sprintf("Node %s ScyllaDB rendered config is invalid: %s", nid, v.GetMessage()),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("node_agent", "GetInfraProbe", map[string]string{
						"node_id":  nid,
						"evidence": v.GetEvidence(),
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, v.GetRemediation(), ""),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}

// ── scylla.join_not_stalled (CRITICAL, node) ─────────────────────────────────

type scyllaJoinNotStalled struct{}

func (scyllaJoinNotStalled) ID() string       { return "scylla.join_not_stalled" }
func (scyllaJoinNotStalled) Category() string { return "scylla" }
func (scyllaJoinNotStalled) Scope() string    { return "node" }

func (scyllaJoinNotStalled) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := scyllaProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		lc := probe.GetLifecycle()
		if lc.GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_STALLED {
			continue
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("scylla.join_not_stalled", nid, lc.GetStateLabel()),
			InvariantID: "scylla.join_not_stalled",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "scylla",
			EntityRef:   nid,
			Summary:     fmt.Sprintf("Node %s ScyllaDB join is STALLED: %s", nid, lc.GetBlockingReason()),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetInfraProbe", map[string]string{
					"node_id":         nid,
					"lifecycle":       lc.GetStateLabel(),
					"blocking_reason": lc.GetBlockingReason(),
					"daemon_active":   fmt.Sprintf("%t", probe.GetDaemonActive()),
					"summary":         probe.GetSummary(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "A STALLED join cannot recover on its own — it needs the owner fixed, not more time.", ""),
				step(2, scyllaRepairOwner, ""),
				step(3, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component scylladb", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// ── scylla.peers_match_expected (ERROR, cluster) ─────────────────────────────

type scyllaPeersMatchExpected struct{}

func (scyllaPeersMatchExpected) ID() string       { return "scylla.peers_match_expected" }
func (scyllaPeersMatchExpected) Category() string { return "scylla" }
func (scyllaPeersMatchExpected) Scope() string    { return "cluster" }

func (scyllaPeersMatchExpected) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := scyllaProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		// Only meaningful when the daemon is up and we expect a real ring.
		if !probe.GetDaemonActive() || len(probe.GetExpectedPeers()) <= 1 {
			continue
		}
		if probe.GetPeersMatch() {
			continue
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("scylla.peers_match_expected", nid, ""),
			InvariantID: "scylla.peers_match_expected",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "scylla",
			EntityRef:   nid,
			Summary: fmt.Sprintf("Node %s ScyllaDB observed peers %v do not cover expected cluster members %v",
				nid, probe.GetObservedPeers(), probe.GetExpectedPeers()),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetInfraProbe", map[string]string{
					"node_id":        nid,
					"expected_peers": strings.Join(probe.GetExpectedPeers(), ","),
					"observed_peers": strings.Join(probe.GetObservedPeers(), ","),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Check gossip/connectivity between this node and the missing peers; verify seeds are reachable.", ""),
				step(2, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component scylladb", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// ── scylla.probe_required_when_installed (ERROR, node) ────────────────────────
// Silence is acceptable ONLY when ScyllaDB is not installed. An installed node
// with no probe data (dial failure, source error, or — separately — an old
// binary that doesn't implement GetInfraProbe) must produce a finding so a
// failed source is never read as "healthy".

type scyllaProbeRequiredWhenInstalled struct{}

func (scyllaProbeRequiredWhenInstalled) ID() string       { return "scylla.probe_required_when_installed" }
func (scyllaProbeRequiredWhenInstalled) Category() string { return "scylla" }
func (scyllaProbeRequiredWhenInstalled) Scope() string    { return "node" }

func (scyllaProbeRequiredWhenInstalled) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		if scyllaProbeFor(snap, nid) != nil {
			continue // we have probe data — other rules judge it
		}
		if !scyllaInstalledFromInventory(snap, nid) {
			continue // not installed → silence is correct
		}

		// Installed but no probe. Distinguish "old binary" (capability gap,
		// expected during rollout → WARN) from a real source failure (ERROR).
		sev := cluster_doctorpb.Severity_SEVERITY_ERROR
		reason := "node-agent returned no infra probe (dial failure or source error)"
		if snap.InfraProbeCapabilityMissing[nid] {
			sev = cluster_doctorpb.Severity_SEVERITY_WARN
			reason = "node-agent binary predates GetInfraProbe (capability missing) — upgrade the node-agent to enable infra truth-plane visibility"
		}

		findings = append(findings, Finding{
			FindingID:   FindingID("scylla.probe_required_when_installed", nid, ""),
			InvariantID: "scylla.probe_required_when_installed",
			Severity:    sev,
			Category:    "scylla",
			EntityRef:   nid,
			Summary:     fmt.Sprintf("Node %s has ScyllaDB installed but produced no infra probe: %s", nid, reason),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetInfraProbe", map[string]string{
					"node_id":             nid,
					"capability_missing":  fmt.Sprintf("%t", snap.InfraProbeCapabilityMissing[nid]),
					"collector_had_error": fmt.Sprintf("%t", snap.HadError("node_agent@"+nid, "GetInfraProbe")),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Ensure the node-agent is reachable and up to date so it can answer GetInfraProbe.", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}
