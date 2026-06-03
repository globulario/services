// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.unit_receipt_drift
// @awareness file_role=doctor_visibility_for_install_receipt_authority_states
// @awareness implements=globular.platform:intent.runtime_observation_must_not_mutate_desired
// @awareness enforces=globular.platform:invariant.state.unknown_must_not_default_to_healthy
// @awareness risk=high
package rules

// unit_receipt_drift.go — surfaces the two drift classes produced by
// the node-agent's authority-resolution path in
// server.go:checkUnitHashDrift after the sidecar-retirement refactor
// (commits 91671230 / 47d7a541, 2026-06-03):
//
//	unit_file_drift                      installed_state.metadata.unit_file_sha256
//	                                     disagrees with the unit file on disk.
//	                                     Service is still running; classify as
//	                                     WARN so the release pipeline's re-
//	                                     install path can handle convergence
//	                                     without raising a quorum-loss signal.
//
//	installed_state_missing_or_unproven  No receipt and no sidecar — there is
//	                                     no authority anywhere. Classify as
//	                                     CRITICAL and FAIL the invariant
//	                                     (`state.unknown_must_not_default_to_
//	                                     healthy`): unknown state must not
//	                                     render as green. This is the fail-
//	                                     closed half of the authority model.
//
// Backwards compatibility: live nodes running pre-refactor node-agent
// (or any stale inventory still cached at the controller) may report
// the legacy "hash_drift" state. Treat it as an alias for
// unit_file_drift so the upgrade window doesn't go dark.
//
// MUST NOT mutate desired state. This rule reads only Layer 4 (runtime)
// from the 4-layer truth model and converts it into operator-visible
// findings.

import (
	"fmt"
	"strings"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
)

// State strings produced by node-agent's checkUnitHashDrift (server.go).
// Pinned here so a rename on either side fails a regression test rather
// than silently going dark. Sidecar legacy state string is included for
// backward compatibility with stale inventories.
const (
	UnitStateUnitFileDrift     = "unit_file_drift"
	UnitStateInstalledMissing  = "installed_state_missing_or_unproven"
	UnitStateLegacyHashDrift   = "hash_drift" // legacy: pre-sidecar-retirement
)

// IsRunningButDrifted returns true for unit states that mean
// "service is still active, but its unit file content differs from
// the install receipt." Objectstore topology / physical overlap rules
// MUST treat these as participating (not as down), because the release
// pipeline's re-install path will heal them without a quorum loss.
//
// Exported so the objectstore rules can share the same recognition set
// without duplicating string literals.
func IsRunningButDrifted(state string) bool {
	switch normalizeUnitState(state) {
	case UnitStateUnitFileDrift, UnitStateLegacyHashDrift:
		return true
	}
	return false
}

// IsReceiptMissing returns true when the unit state signals the
// fail-closed condition: no receipt and no legacy sidecar to migrate
// from. Caller must treat this as CRITICAL.
func IsReceiptMissing(state string) bool {
	return normalizeUnitState(state) == UnitStateInstalledMissing
}

func normalizeUnitState(s string) string {
	return strings.ToLower(strings.TrimSpace(s))
}

type unitReceiptDrift struct{}

func (unitReceiptDrift) ID() string       { return "unit_receipt_drift" }
func (unitReceiptDrift) Category() string { return "drift" }
func (unitReceiptDrift) Scope() string    { return "node" }

func (unitReceiptDrift) Evaluate(snap *collector.Snapshot, _ Config) []Finding {
	if snap == nil {
		return nil
	}
	var findings []Finding
	for nodeID, inv := range snap.Inventories {
		if inv == nil {
			continue
		}
		for _, u := range inv.GetUnits() {
			state := normalizeUnitState(u.GetState())
			unitName := strings.TrimSpace(u.GetName())
			if unitName == "" {
				continue
			}
			switch {
			case IsReceiptMissing(state):
				findings = append(findings, receiptMissingFinding(nodeID, unitName, u.GetState()))
			case IsRunningButDrifted(state):
				findings = append(findings, unitFileDriftFinding(nodeID, unitName, u.GetState()))
			}
		}
	}
	return findings
}

// receiptMissingFinding emits the fail-closed CRITICAL finding when a
// unit has no authority anywhere — neither installed_state.metadata
// receipt nor legacy sidecar. The invariant
// `state.unknown_must_not_default_to_healthy` demands this is not
// silently downgraded to a green/no-opinion verdict.
func receiptMissingFinding(nodeID, unitName, observedState string) Finding {
	return Finding{
		FindingID:   FindingID("unit_receipt_drift.missing", nodeID, unitName),
		InvariantID: "unit_receipt_drift.installed_state_missing_or_unproven",
		Severity:    cluster_doctorpb.Severity_SEVERITY_CRITICAL,
		Category:    "convergence",
		EntityRef:   fmt.Sprintf("%s/%s", nodeID, unitName),
		Summary: fmt.Sprintf(
			"Unit %s on node %s has no authoritative installed_state receipt "+
				"(installed_state.metadata absent AND no legacy .sha256 sidecar). "+
				"Runtime proof is untrusted — fail-closed.",
			unitName, nodeID),
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("cluster_controller", "GetInventory", map[string]string{
				"node_id":        nodeID,
				"unit":           unitName,
				"observed_state": observedState,
				"authority":      "none",
				"failure_class":  "fail_closed_unknown_state",
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, fmt.Sprintf("Re-run the install path for the package owning %s so a canonical receipt is stamped.", unitName), "globular services apply-desired"),
			step(2, fmt.Sprintf("Verify installed_state.metadata has installed_by + unit_file_sha256 after reinstall: globular nodeagent get installed-package --unit %s", unitName), ""),
			step(3, "Confirm the doctor finding clears on the next sweep.", "globular cluster get-doctor-report"),
		},
	}
}

// unitFileDriftFinding emits the WARN finding when the unit is still
// running but the on-disk unit file content has drifted from the
// installed_state.metadata.unit_file_sha256 receipt. This is the
// pre-reinstall window — the release pipeline's re-install path will
// converge it; the operator just needs visibility.
//
// Legacy "hash_drift" also lands here so pre-refactor inventories do
// not go dark.
func unitFileDriftFinding(nodeID, unitName, observedState string) Finding {
	authority := "installed_state.metadata"
	if normalizeUnitState(observedState) == UnitStateLegacyHashDrift {
		authority = "legacy_sidecar (pre-refactor inventory)"
	}
	return Finding{
		FindingID:   FindingID("unit_receipt_drift.unit_file", nodeID, unitName),
		InvariantID: "unit_receipt_drift.unit_file_drift",
		Severity:    cluster_doctorpb.Severity_SEVERITY_WARN,
		Category:    "drift",
		EntityRef:   fmt.Sprintf("%s/%s", nodeID, unitName),
		Summary: fmt.Sprintf(
			"Unit %s on node %s: installed_state expected unit hash differs from "+
				"on-disk systemd unit hash. Service is still running; release "+
				"pipeline re-install will heal — surfaced for operator visibility.",
			unitName, nodeID),
		InvariantStatus: cluster_doctorpb.InvariantStatus_INVARIANT_FAIL,
		Evidence: []*cluster_doctorpb.Evidence{
			kvEvidence("cluster_controller", "GetInventory", map[string]string{
				"node_id":        nodeID,
				"unit":           unitName,
				"observed_state": observedState,
				"authority":      authority,
				"drift_class":    "unit_file_content_differs_from_receipt",
			}),
		},
		Remediation: []*cluster_doctorpb.RemediationStep{
			step(1, fmt.Sprintf("Dispatch re-install for the package owning %s so the unit file is rewritten from the artifact and the receipt is re-stamped.", unitName), "globular services apply-desired"),
			step(2, fmt.Sprintf("If drift persists, inspect: diff /etc/systemd/system/%s vs the artifact's systemd/%s", unitName, unitName), ""),
		},
	}
}
