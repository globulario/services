package rules

import (
	"fmt"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// This file implements the MinIO invariants of the infrastructure truth plane
// (Phase 3). They consume the GetInfraProbe RPC results stored in
// snap.InfraProbes[nodeId]. The probe already did the hard work (parse minio.env,
// attest topology against the ObjectStoreDesiredState, observe the native health
// endpoints, derive lifecycle); these rules project its structured violations and
// lifecycle into operator-facing doctor findings.
//
// These are DISTINCT from the objectstore* rules: those consume the desired
// topology + disk candidates from etcd, whereas these consume the node-agent's
// per-node truth-plane probe (rendered minio.env + live /minio/health/* endpoints).
//
// Two cross-cutting rules from the awareness graph shape this file:
//   - cluster_doctor.rule_goes_silent_when_source_errors: minio.probe_required_when_installed
//     exists so a missing/failed probe on an installed node becomes a finding.
//   - infra.repair_must_target_owner_desired_state_not_manual_file_edit: every
//     remediation points at the config owner, never "hand-edit minio.env".

const minioRepairOwner = "Fix the MinIO config owner (controller-published ObjectStoreDesiredState + config.RenderMinioEnv); do NOT hand-edit /var/lib/globular/minio/minio.env — a render overwrites it and a wrong MINIO_VOLUMES topology risks a format.json reformat."

// minioInfraProbeFor returns the minio probe result for a node, or nil.
func minioInfraProbeFor(snap *collector.Snapshot, nid string) *cluster_controllerpb.InfraProbeResult {
	resp, ok := snap.InfraProbes[nid]
	if !ok || resp == nil {
		return nil
	}
	for _, r := range resp.GetResults() {
		if r.GetComponent() == "minio" {
			return r
		}
	}
	return nil
}

// minioInstalledFromInventory reports whether MinIO is installed on the node.
func minioInstalledFromInventory(snap *collector.Snapshot, nid string) bool {
	if inv := snap.Inventories[nid]; inv != nil {
		for _, u := range inv.GetUnits() {
			if strings.Contains(strings.ToLower(u.GetName()), "globular-minio") {
				return true
			}
		}
	}
	if health := snap.NodeHealths[nid]; health != nil {
		for name := range health.GetInstalledVersions() {
			if strings.Contains(strings.ToLower(name), "minio") {
				return true
			}
		}
	}
	return false
}

// minioProjectViolations emits a finding per probe violation matching invariantID.
func minioProjectViolations(snap *collector.Snapshot, invariantID string, sev cluster_doctorpb.Severity, summaryFmt string) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := minioInfraProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		for _, v := range probe.GetViolations() {
			if v.GetId() != invariantID {
				continue
			}
			severity := sev
			if sev == cluster_doctorpb.Severity_SEVERITY_UNKNOWN {
				severity = sevFromString(v.GetSeverity())
			}
			findings = append(findings, Finding{
				FindingID:   FindingID(invariantID, nid, v.GetEvidence()),
				InvariantID: invariantID,
				Severity:    severity,
				Category:    "minio",
				EntityRef:   nid,
				Summary:     fmt.Sprintf(summaryFmt, nid, v.GetMessage()),
				Evidence: []*cluster_doctorpb.Evidence{
					kvEvidence("node_agent", "GetInfraProbe", map[string]string{
						"node_id":  nid,
						"evidence": v.GetEvidence(),
						"summary":  probe.GetSummary(),
					}),
				},
				Remediation: []*cluster_doctorpb.RemediationStep{
					step(1, v.GetRemediation(), ""),
					step(2, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component minio", ""),
				},
				InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
			})
		}
	}
	return findings
}

// ── minio.loopback_forbidden (CRITICAL, node) ────────────────────────────────

type minioInfraLoopbackForbidden struct{}

func (minioInfraLoopbackForbidden) ID() string       { return "minio.loopback_forbidden" }
func (minioInfraLoopbackForbidden) Category() string { return "minio" }
func (minioInfraLoopbackForbidden) Scope() string    { return "node" }

func (minioInfraLoopbackForbidden) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	return minioProjectViolations(snap, "minio.loopback_forbidden",
		cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		"Node %s MinIO distributed volume advertises a non-routable host: %s")
}

// ── minio.topology_matches_desired (CRITICAL, node) ──────────────────────────
// The format.json blast-radius guard: split-brain standalone-in-cluster or a
// drive-count mismatch vs the desired topology.

type minioInfraTopologyMatchesDesired struct{}

func (minioInfraTopologyMatchesDesired) ID() string       { return "minio.topology_matches_desired" }
func (minioInfraTopologyMatchesDesired) Category() string { return "minio" }
func (minioInfraTopologyMatchesDesired) Scope() string    { return "node" }

func (minioInfraTopologyMatchesDesired) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	return minioProjectViolations(snap, "minio.topology_matches_desired",
		cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		"Node %s MinIO rendered topology diverges from desired (format.json risk): %s")
}

// ── minio.config_valid (ERROR, node) ─────────────────────────────────────────

type minioInfraConfigValid struct{}

func (minioInfraConfigValid) ID() string       { return "minio.config_valid" }
func (minioInfraConfigValid) Category() string { return "minio" }
func (minioInfraConfigValid) Scope() string    { return "node" }

func (minioInfraConfigValid) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	return minioProjectViolations(snap, "minio.config_valid",
		cluster_doctorpb.Severity_SEVERITY_UNKNOWN, // take severity from the violation
		"Node %s MinIO rendered config is invalid: %s")
}

// ── minio.not_stalled (CRITICAL, node) ───────────────────────────────────────

type minioInfraNotStalled struct{}

func (minioInfraNotStalled) ID() string       { return "minio.not_stalled" }
func (minioInfraNotStalled) Category() string { return "minio" }
func (minioInfraNotStalled) Scope() string    { return "node" }

func (minioInfraNotStalled) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := minioInfraProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		lc := probe.GetLifecycle()
		if lc.GetState() != cluster_controllerpb.InfraLifecycleState_INFRA_STALLED {
			continue
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("minio.not_stalled", nid, lc.GetStateLabel()),
			InvariantID: "minio.not_stalled",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "minio",
			EntityRef:   nid,
			Summary:     fmt.Sprintf("Node %s MinIO is STALLED: %s", nid, lc.GetBlockingReason()),
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
				step(1, "A STALLED MinIO member cannot recover on its own — its topology diverges from the pool; fix the owner, not more time.", ""),
				step(2, minioRepairOwner, ""),
				step(3, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component minio", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// ── minio.write_quorum (ERROR, cluster) ──────────────────────────────────────
// Live but the pool has lost write quorum — uploads fail. Distinct from the
// disk-topology-derived objectstore.write_quorum_lost: this is the live
// /minio/health/cluster signal.

type minioInfraWriteQuorum struct{}

func (minioInfraWriteQuorum) ID() string       { return "minio.write_quorum" }
func (minioInfraWriteQuorum) Category() string { return "minio" }
func (minioInfraWriteQuorum) Scope() string    { return "cluster" }

func (minioInfraWriteQuorum) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := minioInfraProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		rt := probe.GetRuntime()
		// Only meaningful once the server is live; "live && !write_quorum" is the
		// real blast-radius signal. A non-live server is covered by the lifecycle.
		if rt["live"] != "true" || rt["write_quorum"] != "false" {
			continue
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("minio.write_quorum", nid, ""),
			InvariantID: "minio.write_quorum",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "minio",
			EntityRef:   nid,
			Summary:     fmt.Sprintf("Node %s MinIO is live but the pool has lost write quorum — uploads will fail", nid),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetInfraProbe", map[string]string{
					"node_id":      nid,
					"live":         rt["live"],
					"write_quorum": rt["write_quorum"],
					"read_quorum":  rt["read_quorum"],
					"summary":      probe.GetSummary(),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Restore enough pool drives/peers to regain write quorum; check globular-minio.service on the offline pool nodes.", ""),
				step(2, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component minio", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// ── minio.probe_required_when_installed (ERROR, node) ─────────────────────────

type minioInfraProbeRequiredWhenInstalled struct{}

func (minioInfraProbeRequiredWhenInstalled) ID() string       { return "minio.probe_required_when_installed" }
func (minioInfraProbeRequiredWhenInstalled) Category() string { return "minio" }
func (minioInfraProbeRequiredWhenInstalled) Scope() string    { return "node" }

func (minioInfraProbeRequiredWhenInstalled) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		if minioInfraProbeFor(snap, nid) != nil {
			continue // we have probe data — other rules judge it
		}
		if !minioInstalledFromInventory(snap, nid) {
			continue // not installed → silence is correct
		}

		// Installed but no probe. The shared builder distinguishes "could not
		// observe" (collector error / capability gap → WARN + UNKNOWN) from
		// "observed nothing despite a complete harvest" (ERROR + FAIL).
		findings = append(findings, infraProbeRequiredFinding(snap, "minio", "MinIO", nid))
	}
	return findings
}
