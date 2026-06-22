package rules

import (
	"fmt"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// This file implements the etcd invariants of the infrastructure truth plane
// (Phase 2). They consume the GetInfraProbe RPC results stored in
// snap.InfraProbes[nodeId]. The probe already did the hard work (parse etcd.yaml,
// attest, observe member list/status/alarms via the native v3 API, derive
// lifecycle); these rules project its structured violations and lifecycle into
// operator-facing doctor findings.
//
// Two cross-cutting rules from the awareness graph shape this file:
//   - cluster_doctor.rule_goes_silent_when_source_errors: a rule must NOT emit
//     "healthy" (i.e. stay silent) when its source errored. The
//     etcd.probe_required_when_installed rule exists precisely so a missing or
//     failed probe on an installed node becomes a visible finding.
//   - infra.repair_must_target_owner_desired_state_not_manual_file_edit: every
//     remediation points at the config owner, never "hand-edit etcd.yaml".

const etcdRepairOwner = "Fix the etcd config owner (cluster-controller reconcileServiceConfigs renderer + controller-provided membership/routable IP); do NOT hand-edit /var/lib/globular/config/etcd.yaml — a render overwrites it."

// etcdInfraProbeFor returns the etcd probe result for a node, or nil.
func etcdInfraProbeFor(snap *collector.Snapshot, nid string) *cluster_controllerpb.InfraProbeResult {
	resp, ok := snap.InfraProbes[nid]
	if !ok || resp == nil {
		return nil
	}
	for _, r := range resp.GetResults() {
		if r.GetComponent() == "etcd" {
			return r
		}
	}
	return nil
}

// etcdInstalledFromInventory reports whether the node's inventory/health
// indicates etcd is installed — used to decide whether a missing probe is a
// problem (installed) or acceptable silence (not installed). Independent of the
// probe so it still works when the probe itself is absent.
func etcdInstalledFromInventory(snap *collector.Snapshot, nid string) bool {
	if inv := snap.Inventories[nid]; inv != nil {
		for _, u := range inv.GetUnits() {
			if strings.Contains(strings.ToLower(u.GetName()), "globular-etcd") {
				return true
			}
		}
	}
	if health := snap.NodeHealths[nid]; health != nil {
		for name := range health.GetInstalledVersions() {
			if strings.Contains(strings.ToLower(name), "etcd") {
				return true
			}
		}
	}
	return false
}

// ── etcd.loopback_forbidden (CRITICAL, node) ─────────────────────────────────

type etcdInfraLoopbackForbidden struct{}

func (etcdInfraLoopbackForbidden) ID() string       { return "etcd.loopback_forbidden" }
func (etcdInfraLoopbackForbidden) Category() string { return "etcd" }
func (etcdInfraLoopbackForbidden) Scope() string    { return "node" }

func (etcdInfraLoopbackForbidden) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := etcdInfraProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		for _, v := range probe.GetViolations() {
			if v.GetId() != "etcd.loopback_forbidden" {
				continue
			}
			findings = append(findings, Finding{
				FindingID:   FindingID("etcd.loopback_forbidden", nid, v.GetEvidence()),
				InvariantID: "etcd.loopback_forbidden",
				Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
				Category:    "etcd",
				EntityRef:   nid,
				Summary:     fmt.Sprintf("Node %s etcd config advertises a loopback/unspecified cluster address: %s", nid, v.GetMessage()),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("node_agent", "GetInfraProbe", map[string]string{
						"node_id":  nid,
						"evidence": v.GetEvidence(),
						"summary":  probe.GetSummary(),
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, v.GetRemediation(), ""),
					step(2, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component etcd", ""),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}

// ── etcd.config_valid (ERROR, node) ──────────────────────────────────────────

type etcdInfraConfigValid struct{}

func (etcdInfraConfigValid) ID() string       { return "etcd.config_valid" }
func (etcdInfraConfigValid) Category() string { return "etcd" }
func (etcdInfraConfigValid) Scope() string    { return "node" }

func (etcdInfraConfigValid) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := etcdInfraProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		for _, v := range probe.GetViolations() {
			if v.GetId() != "etcd.config_valid" {
				continue
			}
			findings = append(findings, Finding{
				FindingID:   FindingID("etcd.config_valid", nid, v.GetEvidence()),
				InvariantID: "etcd.config_valid",
				Severity:    sevFromString(v.GetSeverity()),
				Category:    "etcd",
				EntityRef:   nid,
				Summary:     fmt.Sprintf("Node %s etcd rendered config is invalid: %s", nid, v.GetMessage()),
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

// ── etcd.join_not_stalled (CRITICAL, node) ───────────────────────────────────

type etcdInfraJoinNotStalled struct{}

func (etcdInfraJoinNotStalled) ID() string       { return "etcd.join_not_stalled" }
func (etcdInfraJoinNotStalled) Category() string { return "etcd" }
func (etcdInfraJoinNotStalled) Scope() string    { return "node" }

func (etcdInfraJoinNotStalled) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := etcdInfraProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		lc := probe.GetLifecycle()
		if lc.GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_STALLED {
			continue
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("etcd.join_not_stalled", nid, lc.GetStateLabel()),
			InvariantID: "etcd.join_not_stalled",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "etcd",
			EntityRef:   nid,
			Summary:     fmt.Sprintf("Node %s etcd is STALLED: %s", nid, lc.GetBlockingReason()),
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
				step(1, "A STALLED etcd member cannot recover on its own — it needs the owner fixed (or a controller-driven wipe+rejoin for CORRUPT), not more time.", ""),
				step(2, etcdRepairOwner, ""),
				step(3, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component etcd", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// ── etcd.peers_match_expected (ERROR, cluster) ───────────────────────────────

type etcdInfraPeersMatchExpected struct{}

func (etcdInfraPeersMatchExpected) ID() string       { return "etcd.peers_match_expected" }
func (etcdInfraPeersMatchExpected) Category() string { return "etcd" }
func (etcdInfraPeersMatchExpected) Scope() string    { return "cluster" }

func (etcdInfraPeersMatchExpected) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := etcdInfraProbeFor(snap, nid)
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
			FindingID:   FindingID("etcd.peers_match_expected", nid, ""),
			InvariantID: "etcd.peers_match_expected",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "etcd",
			EntityRef:   nid,
			Summary: fmt.Sprintf("Node %s etcd observed members %v do not cover expected cluster members %v",
				nid, probe.GetObservedPeers(), probe.GetExpectedPeers()),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetInfraProbe", map[string]string{
					"node_id":        nid,
					"expected_peers": strings.Join(probe.GetExpectedPeers(), ","),
					"observed_peers": strings.Join(probe.GetObservedPeers(), ","),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Check peer connectivity (port 2380) between this node and the missing members; verify the controller re-rendered initial-cluster after the last MemberAdd.", ""),
				step(2, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component etcd", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// ── etcd.probe_required_when_installed (ERROR, node) ──────────────────────────
// Silence is acceptable ONLY when etcd is not installed. An installed node with
// no probe data (dial failure, source error, or an old binary that doesn't
// implement GetInfraProbe) must produce a finding so a failed source is never
// read as "healthy".

type etcdInfraProbeRequiredWhenInstalled struct{}

func (etcdInfraProbeRequiredWhenInstalled) ID() string       { return "etcd.probe_required_when_installed" }
func (etcdInfraProbeRequiredWhenInstalled) Category() string { return "etcd" }
func (etcdInfraProbeRequiredWhenInstalled) Scope() string    { return "node" }

func (etcdInfraProbeRequiredWhenInstalled) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		if etcdInfraProbeFor(snap, nid) != nil {
			continue // we have probe data — other rules judge it
		}
		if !etcdInstalledFromInventory(snap, nid) {
			continue // not installed → silence is correct
		}

		// Installed but no probe. The shared builder distinguishes "could not
		// observe" (collector error / capability gap → WARN + UNKNOWN) from
		// "observed nothing despite a complete harvest" (ERROR + FAIL).
		findings = append(findings, infraProbeRequiredFinding(snap, "etcd", "etcd", nid))
	}
	return findings
}
