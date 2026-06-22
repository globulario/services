package rules

import (
	"fmt"
	"strconv"
	"strings"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// This file implements the Envoy invariants of the infrastructure truth plane
// (Phase 4). They consume the GetInfraProbe RPC results stored in
// snap.InfraProbes[nodeId]. Unlike the clustered components, Envoy is a per-node
// data plane fed by xDS — so the truth comes from the rendered bootstrap (can the
// ADS handshake happen) plus the live admin API (/ready, /stats: did CDS/LDS
// progress, are listeners active).
//
// These are DIAGNOSTIC ONLY and DISTINCT from the existing envoy.lds_wedge rule:
// that one is cluster-scoped and Prometheus-derived; these are per-node and read
// the node-agent's direct admin-API observation. They never restart/reload Envoy
// — for the LDS wedge the root cause is an upstream restart storm, so an
// auto-restart would deepen the wedge.

const envoyRepairOwner = "Repair the Envoy/xDS owner: the gateway/xDS control plane that writes the bootstrap and feeds dynamic config. Do NOT restart globular-envoy to clear an LDS wedge — the usual root cause is an upstream restart storm and a restart deepens it. See docs/awareness/reports/envoy_lds_cds_wedge.md."

// envoyInfraProbeFor returns the envoy probe result for a node, or nil.
func envoyInfraProbeFor(snap *collector.Snapshot, nid string) *cluster_controllerpb.InfraProbeResult {
	resp, ok := snap.InfraProbes[nid]
	if !ok || resp == nil {
		return nil
	}
	for _, r := range resp.GetResults() {
		if r.GetComponent() == "envoy" {
			return r
		}
	}
	return nil
}

// envoyInstalledFromInventory reports whether Envoy is installed on the node.
func envoyInstalledFromInventory(snap *collector.Snapshot, nid string) bool {
	if inv := snap.Inventories[nid]; inv != nil {
		for _, u := range inv.GetUnits() {
			if strings.Contains(strings.ToLower(u.GetName()), "globular-envoy") {
				return true
			}
		}
	}
	if health := snap.NodeHealths[nid]; health != nil {
		for name := range health.GetInstalledVersions() {
			if strings.Contains(strings.ToLower(name), "envoy") {
				return true
			}
		}
	}
	return false
}

func envoyRuntimeInt(probe *cluster_controllerpb.InfraProbeResult, key string) (int64, bool) {
	v, ok := probe.GetRuntime()[key]
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

// ── envoy.config_valid (ERROR/CRITICAL, node) ────────────────────────────────

type envoyInfraConfigValid struct{}

func (envoyInfraConfigValid) ID() string       { return "envoy.config_valid" }
func (envoyInfraConfigValid) Category() string { return "envoy" }
func (envoyInfraConfigValid) Scope() string    { return "node" }

func (envoyInfraConfigValid) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := envoyInfraProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		for _, v := range probe.GetViolations() {
			if v.GetId() != "envoy.config_valid" {
				continue
			}
			findings = append(findings, Finding{
				FindingID:   FindingID("envoy.config_valid", nid, v.GetEvidence()),
				InvariantID: "envoy.config_valid",
				Severity:    sevFromString(v.GetSeverity()),
				Category:    "envoy",
				EntityRef:   nid,
				Summary:     fmt.Sprintf("Node %s Envoy bootstrap is invalid: %s", nid, v.GetMessage()),
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

// ── envoy.lds_progress (CRITICAL, node) ──────────────────────────────────────
// The per-node, admin-API-derived LDS wedge: CDS applied at least one update but
// LDS update_attempt is still 0. Pins envoy.lds_progress_required_for_http_mesh_readiness
// and anchors failure_mode envoy.lds_update_attempt_zero_despite_cds_progress.

type envoyInfraLDSProgress struct{}

func (envoyInfraLDSProgress) ID() string       { return "envoy.lds_progress" }
func (envoyInfraLDSProgress) Category() string { return "envoy" }
func (envoyInfraLDSProgress) Scope() string    { return "node" }

func (envoyInfraLDSProgress) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := envoyInfraProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		cds, cdsOK := envoyRuntimeInt(probe, "cds_update_success")
		ldsAttempt, ldsOK := envoyRuntimeInt(probe, "lds_update_attempt")
		if !cdsOK || !ldsOK {
			continue // admin not reached / stats absent — probe_required covers visibility
		}
		// Cold init (CDS not yet applied) is not the wedge — too early to tell.
		if cds == 0 || ldsAttempt > 0 {
			continue
		}
		findings = append(findings, Finding{
			FindingID:   FindingID("envoy.lds_progress", nid, ""),
			InvariantID: "envoy.lds_progress_required_for_http_mesh_readiness",
			Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
			Category:    "envoy",
			EntityRef:   nid,
			Summary:     fmt.Sprintf("Node %s Envoy mesh WEDGED — CDS applied %d update(s) but LDS update_attempt is 0; port 443 will not bind, HTTP mesh is down", nid, cds),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetInfraProbe", map[string]string{
					"node_id":               nid,
					"cds_update_success":    fmt.Sprintf("%d", cds),
					"lds_update_attempt":    fmt.Sprintf("%d", ldsAttempt),
					"failure_mode_anchor":   "envoy.lds_update_attempt_zero_despite_cds_progress",
					"invariant_anchor":      "envoy.lds_progress_required_for_http_mesh_readiness",
					"auto_clear_condition":  "lds_update_attempt > 0",
					"do_not_auto_remediate": "true — restart loops deepen the wedge",
					"see_also":              "docs/awareness/reports/envoy_lds_cds_wedge.md",
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Do NOT restart globular-envoy — the LDS wedge is usually caused by an upstream restart storm that SIGTERMs Envoy before the LDS handshake completes.", ""),
				step(2, envoyRepairOwner, ""),
				step(3, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component envoy", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// ── envoy.listeners_active (ERROR, node) ─────────────────────────────────────
// CDS and LDS have both progressed, but no listener is active (rejected config or
// none delivered) — port 443 is not actually serving.

type envoyInfraListenersActive struct{}

func (envoyInfraListenersActive) ID() string       { return "envoy.listeners_active" }
func (envoyInfraListenersActive) Category() string { return "envoy" }
func (envoyInfraListenersActive) Scope() string    { return "node" }

func (envoyInfraListenersActive) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		probe := envoyInfraProbeFor(snap, nid)
		if probe == nil {
			continue
		}
		cds, cdsOK := envoyRuntimeInt(probe, "cds_update_success")
		ldsAttempt, ldsOK := envoyRuntimeInt(probe, "lds_update_attempt")
		active, activeOK := envoyRuntimeInt(probe, "active_listeners")
		if !cdsOK || !ldsOK || !activeOK {
			continue
		}
		// Only meaningful once both CDS and LDS have progressed (not the wedge,
		// not cold init) and yet no listener is active.
		if cds == 0 || ldsAttempt == 0 || active > 0 {
			continue
		}
		rejected, _ := envoyRuntimeInt(probe, "lds_update_rejected")
		findings = append(findings, Finding{
			FindingID:   FindingID("envoy.listeners_active", nid, ""),
			InvariantID: "envoy.listeners_active",
			Severity:    cluster_doctorpb.Severity_SEVERITY_ERROR,
			Category:    "envoy",
			EntityRef:   nid,
			Summary:     fmt.Sprintf("Node %s Envoy has attempted LDS but has 0 active listeners — port 443 is not serving (lds_rejected=%d)", nid, rejected),
			Evidence: []*cluster_doctorpb.Evidence{
				kvEvidence("node_agent", "GetInfraProbe", map[string]string{
					"node_id":             nid,
					"cds_update_success":  fmt.Sprintf("%d", cds),
					"lds_update_attempt":  fmt.Sprintf("%d", ldsAttempt),
					"lds_update_rejected": fmt.Sprintf("%d", rejected),
					"active_listeners":    fmt.Sprintf("%d", active),
				}),
			},
			Remediation: []*cluster_doctorpb.RemediationStep{
				step(1, "Check xDS (globular-xds) listener config — rejected LDS updates mean xDS is sending invalid listener resources.", ""),
				step(2, "Explain the stall: globular cluster infra explain-stall --node "+nid+" --component envoy", ""),
			},
			InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		})
	}
	return findings
}

// ── envoy.probe_required_when_installed (ERROR, node) ─────────────────────────

type envoyInfraProbeRequiredWhenInstalled struct{}

func (envoyInfraProbeRequiredWhenInstalled) ID() string       { return "envoy.probe_required_when_installed" }
func (envoyInfraProbeRequiredWhenInstalled) Category() string { return "envoy" }
func (envoyInfraProbeRequiredWhenInstalled) Scope() string    { return "node" }

func (envoyInfraProbeRequiredWhenInstalled) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	var findings []Finding
	for _, node := range snap.Nodes {
		nid := node.GetNodeId()
		if envoyInfraProbeFor(snap, nid) != nil {
			continue
		}
		if !envoyInstalledFromInventory(snap, nid) {
			continue
		}

		// Installed but no probe. The shared builder distinguishes "could not
		// observe" (collector error / capability gap → WARN + UNKNOWN) from
		// "observed nothing despite a complete harvest" (ERROR + FAIL).
		findings = append(findings, infraProbeRequiredFinding(snap, "envoy", "Envoy", nid))
	}
	return findings
}
