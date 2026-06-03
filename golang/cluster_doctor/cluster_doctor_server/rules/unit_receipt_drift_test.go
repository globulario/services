// @awareness namespace=globular.platform
// @awareness component=platform_cluster_doctor.rules.unit_receipt_drift_tests
// @awareness file_role=regression_tests_for_install_receipt_authority_visibility
// @awareness risk=high
package rules

import (
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

// inventoryWithUnit builds a single-unit Inventory for tests.
func inventoryWithUnit(unitName, state string) *node_agentpb.Inventory {
	return &node_agentpb.Inventory{
		Units: []*node_agentpb.UnitStatus{
			{Name: unitName, State: state},
		},
	}
}

// snapshotWith builds a snapshot with a single node and a single unit
// in the given state.
func snapshotWith(nodeID, unitName, state string) *collector.Snapshot {
	return &collector.Snapshot{
		Inventories: map[string]*node_agentpb.Inventory{
			nodeID: inventoryWithUnit(unitName, state),
		},
	}
}

// ── State-string contract ─────────────────────────────────────────────────────

// TestUnitStateConstantsMatchNodeAgentReturnValues pins the state
// strings produced by node-agent's checkUnitHashDrift in server.go.
// A rename on either side without updating this constant will fail
// this test, which is the point: the wire contract between node-agent
// and doctor depends on these exact strings.
func TestUnitStateConstantsMatchNodeAgentReturnValues(t *testing.T) {
	cases := map[string]string{
		"unit_file_drift":                     UnitStateUnitFileDrift,
		"installed_state_missing_or_unproven": UnitStateInstalledMissing,
		"hash_drift":                          UnitStateLegacyHashDrift,
	}
	for want, got := range cases {
		if got != want {
			t.Errorf("constant mismatch: got %q, want %q (node-agent emits this exact string)", got, want)
		}
	}
}

// ── unit_file_drift surfacing ─────────────────────────────────────────────────

// TestUnitReceiptDrift_SurfacesUnitFileDriftAsWarn proves the new
// state name (post sidecar retirement) is visible to the operator as
// a WARN finding rather than going dark.
func TestUnitReceiptDrift_SurfacesUnitFileDriftAsWarn(t *testing.T) {
	snap := snapshotWith("node-1", "globular-node-agent.service", "unit_file_drift")
	findings := unitReceiptDrift{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding for unit_file_drift, got %d", len(findings))
	}
	f := findings[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("severity = %v, want SEVERITY_WARN (service still running, release pipeline heals)", f.Severity)
	}
	if f.InvariantID != "unit_receipt_drift.unit_file_drift" {
		t.Errorf("invariant_id = %q, want unit_receipt_drift.unit_file_drift", f.InvariantID)
	}
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("status = %v, want INVARIANT_FAIL (drift must not pass silently)", f.InvariantStatus)
	}
}

// TestUnitReceiptDrift_LegacyHashDriftStillRecognized proves the
// pre-refactor state name is surfaced as WARN. Live nodes running the
// old node-agent (or cached stale inventories) still emit "hash_drift";
// the upgrade window must not go dark.
func TestUnitReceiptDrift_LegacyHashDriftStillRecognized(t *testing.T) {
	snap := snapshotWith("node-1", "globular-node-agent.service", "hash_drift")
	findings := unitReceiptDrift{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding for legacy hash_drift, got %d", len(findings))
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("legacy hash_drift severity = %v, want SEVERITY_WARN", findings[0].Severity)
	}
	authority := findings[0].Evidence[0].KeyValues["authority"]
	if authority != "legacy_sidecar (pre-refactor inventory)" {
		t.Errorf("legacy authority annotation missing; got %q", authority)
	}
}

// ── installed_state_missing_or_unproven fail-closed ───────────────────────────

// TestUnitReceiptDrift_FailClosedOnInstalledStateMissing is the
// regression for the invariant
// `state.unknown_must_not_default_to_healthy`. When a unit has no
// authority anywhere (no receipt, no sidecar), the doctor must NOT
// stay silent — it must surface a CRITICAL finding so the operator
// sees the unknown state.
func TestUnitReceiptDrift_FailClosedOnInstalledStateMissing(t *testing.T) {
	snap := snapshotWith("node-1", "globular-node-agent.service", "installed_state_missing_or_unproven")
	findings := unitReceiptDrift{}.Evaluate(snap, Config{})
	if len(findings) != 1 {
		t.Fatalf("expected exactly 1 finding for installed_state_missing_or_unproven, got %d", len(findings))
	}
	f := findings[0]
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("severity = %v, want SEVERITY_CRITICAL (fail-closed per state.unknown_must_not_default_to_healthy)", f.Severity)
	}
	if f.InvariantID != "unit_receipt_drift.installed_state_missing_or_unproven" {
		t.Errorf("invariant_id = %q", f.InvariantID)
	}
	if f.InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_FAIL {
		t.Errorf("status = %v, want INVARIANT_FAIL", f.InvariantStatus)
	}
	authority := f.Evidence[0].KeyValues["authority"]
	if authority != "none" {
		t.Errorf("authority annotation = %q, want \"none\" (fail-closed)", authority)
	}
	failClass := f.Evidence[0].KeyValues["failure_class"]
	if failClass != "fail_closed_unknown_state" {
		t.Errorf("failure_class = %q, want fail_closed_unknown_state", failClass)
	}
}

// ── healthy units do not fire ─────────────────────────────────────────────────

// TestUnitReceiptDrift_NoFindingsForActiveUnits proves the rule
// does not produce false-positive findings against healthy units.
func TestUnitReceiptDrift_NoFindingsForActiveUnits(t *testing.T) {
	snap := snapshotWith("node-1", "globular-node-agent.service", "active")
	findings := unitReceiptDrift{}.Evaluate(snap, Config{})
	if len(findings) != 0 {
		t.Errorf("active unit must not produce findings; got %d: %+v", len(findings), findings)
	}
}

// TestUnitReceiptDrift_NoFindingsForOtherStatesNotInScope verifies
// that state strings outside this rule's domain (failed, inactive,
// activating, missing, no_inventory) are NOT erroneously claimed.
// They are surfaced by other rules (nodeUnitsRunning,
// installedStateRuntimeMismatch).
func TestUnitReceiptDrift_NoFindingsForOtherStatesNotInScope(t *testing.T) {
	for _, state := range []string{"failed", "inactive", "activating", "missing", "no_inventory"} {
		snap := snapshotWith("node-1", "globular-node-agent.service", state)
		findings := unitReceiptDrift{}.Evaluate(snap, Config{})
		if len(findings) != 0 {
			t.Errorf("state %q must not produce unit_receipt_drift findings; got %d", state, len(findings))
		}
	}
}

// TestUnitReceiptDrift_NoFindingsForEmptyInventory proves the rule
// is safe against an empty snapshot — no panic, no findings.
func TestUnitReceiptDrift_NoFindingsForEmptyInventory(t *testing.T) {
	if got := (unitReceiptDrift{}).Evaluate(nil, Config{}); got != nil {
		t.Errorf("nil snapshot must not produce findings; got %v", got)
	}
	empty := &collector.Snapshot{Inventories: map[string]*node_agentpb.Inventory{}}
	if got := (unitReceiptDrift{}).Evaluate(empty, Config{}); len(got) != 0 {
		t.Errorf("empty inventory must not produce findings; got %v", got)
	}
}

// TestUnitReceiptDrift_MultipleNodesAggregated proves the rule emits
// one finding per (node, unit), not one collapsed cluster-wide finding.
// This matters because remediation is per-node — collapsing would lose
// the entityRef needed by the healer.
func TestUnitReceiptDrift_MultipleNodesAggregated(t *testing.T) {
	snap := &collector.Snapshot{
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": inventoryWithUnit("globular-foo.service", "unit_file_drift"),
			"node-2": inventoryWithUnit("globular-foo.service", "installed_state_missing_or_unproven"),
			"node-3": inventoryWithUnit("globular-foo.service", "active"),
		},
	}
	findings := unitReceiptDrift{}.Evaluate(snap, Config{})
	if len(findings) != 2 {
		t.Fatalf("expected 2 findings (one per drifted node), got %d", len(findings))
	}
	got := map[string]cluster_doctorpb.Severity{}
	for _, f := range findings {
		got[f.EntityRef] = f.Severity
	}
	if got["node-1/globular-foo.service"] != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("node-1 should be WARN; got %v", got["node-1/globular-foo.service"])
	}
	if got["node-2/globular-foo.service"] != cluster_doctorpb.Severity_SEVERITY_CRITICAL {
		t.Errorf("node-2 should be CRITICAL; got %v", got["node-2/globular-foo.service"])
	}
}

// ── shared helpers consumed by objectstore rules ──────────────────────────────

// TestIsRunningButDrifted_AcceptsBothLegacyAndNewNames proves the
// shared helper that objectstore_topology and objectstore_physical_
// overlap use to decide "service is still up" accepts both names.
// Without this, the upgrade-window inventory would be classified as
// not-active and trigger false quorum-loss findings.
func TestIsRunningButDrifted_AcceptsBothLegacyAndNewNames(t *testing.T) {
	for _, state := range []string{"unit_file_drift", "hash_drift"} {
		if !IsRunningButDrifted(state) {
			t.Errorf("IsRunningButDrifted(%q) = false; objectstore rules would treat this as down", state)
		}
	}
	if !IsRunningButDrifted("  UNIT_FILE_DRIFT  ") {
		t.Errorf("IsRunningButDrifted normalisation broken — mixed case rejected")
	}
}

// TestIsRunningButDrifted_RejectsTerminalStates proves the helper
// does NOT mistakenly classify failed / installed_state_missing as
// "still up". A false positive here would let the objectstore rules
// hide a quorum-loss event behind a green "drift" verdict.
func TestIsRunningButDrifted_RejectsTerminalStates(t *testing.T) {
	for _, state := range []string{
		"failed",
		"inactive",
		"missing",
		"no_inventory",
		"installed_state_missing_or_unproven",
		"",
	} {
		if IsRunningButDrifted(state) {
			t.Errorf("IsRunningButDrifted(%q) = true; would mask a quorum loss as drift", state)
		}
	}
}

// TestIsReceiptMissing_FailClosedHelper proves the helper recognises
// the fail-closed signal — independent of whitespace / case.
func TestIsReceiptMissing_FailClosedHelper(t *testing.T) {
	if !IsReceiptMissing("installed_state_missing_or_unproven") {
		t.Errorf("canonical fail-closed state not recognised")
	}
	if !IsReceiptMissing("  Installed_State_Missing_Or_Unproven  ") {
		t.Errorf("case/whitespace tolerance broken for fail-closed signal")
	}
	if IsReceiptMissing("unit_file_drift") {
		t.Errorf("unit_file_drift wrongly classified as receipt-missing — would force CRITICAL on a WARN-class drift")
	}
}
