package main

// globular:tested_by topology_safety_drift_reconciler

import "testing"

// TestStaleMajorityTopologyRejectedWhileSafeActionsRun verifies that when
// topology violations are present, topology-affecting actions are blocked while
// safe actions (SERVICE, COMMAND) continue. The drift reconciler must not halt
// all processing when only topology-mutating actions are unsafe.
//
// Invariant: topology.reconciler_must_respect_safety_contract
func TestStaleMajorityTopologyRejectedWhileSafeActionsRun(t *testing.T) {
	violations := []topologySafetyViolation{
		{Kind: "objectstore_topology_mismatch", Message: "desired objectstore topology differs from controller pool"},
	}

	// SERVICE drift action — must always be safe, even under topology violations.
	serviceAction := driftAction{
		NodeID:     "node-1",
		PackageKey: "SERVICE/authentication",
		Kind:       "SERVICE",
		ActionKind: classifyDriftAction("SERVICE"),
	}
	if !driftActionSafe(serviceAction, violations) {
		t.Error("SERVICE drift action should be safe even when objectstore topology is mismatched")
	}

	// COMMAND drift action — also safe.
	commandAction := driftAction{
		NodeID:     "node-1",
		PackageKey: "COMMAND/globularcli",
		Kind:       "COMMAND",
		ActionKind: classifyDriftAction("COMMAND"),
	}
	if !driftActionSafe(commandAction, violations) {
		t.Error("COMMAND drift action should be safe even when topology is degraded")
	}

	// INFRASTRUCTURE drift action — must be blocked when the package owns
	// the violated topology dimension. MinIO is gated by objectstore topology
	// mismatch, not by a preferred storage node count.
	infraAction := driftAction{
		NodeID:     "node-1",
		PackageKey: "INFRASTRUCTURE/minio",
		Kind:       "INFRASTRUCTURE",
		ActionKind: classifyDriftAction("INFRASTRUCTURE"),
	}
	if driftActionSafe(infraAction, violations) {
		t.Error("INFRASTRUCTURE/minio drift action should be blocked when objectstore topology mismatch is present")
	}

	// INFRASTRUCTURE drift action — safe when no violations.
	if !driftActionSafe(infraAction, nil) {
		t.Error("INFRASTRUCTURE drift action should be safe when no topology violations exist")
	}
	if !driftActionSafe(infraAction, []topologySafetyViolation{}) {
		t.Error("INFRASTRUCTURE drift action should be safe when violation list is empty")
	}
}

// TestPerActionSafetyGateDeterministic verifies that classifyDriftAction is
// deterministic: the same kind always yields the same driftActionKind. This
// ensures the per-action gate produces consistent decisions across reconcile
// loops and is not dependent on any mutable state.
//
// Invariant: topology.reconciler_must_respect_safety_contract
func TestPerActionSafetyGateDeterministic(t *testing.T) {
	cases := []struct {
		kind     string
		expected driftActionKind
	}{
		// Standard service kinds — always safe.
		{"SERVICE", driftActionKindSafe},
		{"service", driftActionKindSafe}, // case-insensitive
		{"Service", driftActionKindSafe},
		{"COMMAND", driftActionKindSafe},
		{"command", driftActionKindSafe},
		// Infrastructure — topology-affecting.
		{"INFRASTRUCTURE", driftActionKindTopology},
		{"infrastructure", driftActionKindTopology},
		{"Infrastructure", driftActionKindTopology},
		// Unknown kinds default to safe (conservative: don't block valid updates).
		{"", driftActionKindSafe},
		{"UNKNOWN_KIND", driftActionKindSafe},
	}

	for _, tc := range cases {
		got := classifyDriftAction(tc.kind)
		if got != tc.expected {
			t.Errorf("classifyDriftAction(%q) = %q, want %q", tc.kind, got, tc.expected)
		}
		// Second call — must be identical (deterministic).
		got2 := classifyDriftAction(tc.kind)
		if got2 != got {
			t.Errorf("classifyDriftAction(%q) not deterministic: first=%q second=%q", tc.kind, got, got2)
		}
	}
}
