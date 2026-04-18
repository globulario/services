package main

import (
	"strings"
	"testing"
)

// ── Identity-copy enforcement ────────────────────────────────────────────────

// TestValidateRepairPlan_RejectsForbiddenIdentityCopy proves that a repair
// plan containing identity-copy actions is rejected. This is not just
// "structurally impossible" — it is actively validated and rejected.
func TestValidateRepairPlan_RejectsForbiddenIdentityCopy(t *testing.T) {
	cases := []struct {
		name   string
		plan   map[string]any
		reject bool
	}{
		{
			name:   "clean plan — no identity fields",
			plan:   map[string]any{"action": "reinstall", "node_id": "abc"},
			reject: false,
		},
		{
			name:   "copy_private_key=true rejected",
			plan:   map[string]any{"copy_private_key": true},
			reject: true,
		},
		{
			name:   "copy_ca_key=true rejected",
			plan:   map[string]any{"copy_ca_key": true},
			reject: true,
		},
		{
			name:   "copy_node_id=true rejected",
			plan:   map[string]any{"copy_node_id": true},
			reject: true,
		},
		{
			name:   "copy_identity=true rejected",
			plan:   map[string]any{"copy_identity": true},
			reject: true,
		},
		{
			name:   "clone_certs=true rejected",
			plan:   map[string]any{"clone_certs": true},
			reject: true,
		},
		{
			name:   "clone_keys=true rejected",
			plan:   map[string]any{"clone_keys": true},
			reject: true,
		},
		{
			name:   "copy_private_key=false allowed (explicit false)",
			plan:   map[string]any{"copy_private_key": false},
			reject: false,
		},
		{
			name:   "copy_private_key=nil allowed",
			plan:   map[string]any{"copy_private_key": nil},
			reject: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := ValidateRepairPlan(tc.plan)
			if tc.reject && err == nil {
				t.Error("expected rejection for forbidden identity-copy field, got nil")
			}
			if !tc.reject && err != nil {
				t.Errorf("expected clean plan to pass, got error: %v", err)
			}
		})
	}
}

// ── Priority ordering ────────────────────────────────────────────────────────

// TestRepairPriorityClass verifies the default repair priority ordering:
// controller (0) → node-agent (1) → infrastructure (2) → services (3).
func TestRepairPriorityClass(t *testing.T) {
	cases := []struct {
		name     string
		kind     string
		expected int
	}{
		{"cluster-controller", "SERVICE", 0},
		{"node-agent", "SERVICE", 1},
		{"envoy", "INFRASTRUCTURE", 2},
		{"xds", "INFRASTRUCTURE", 2},
		{"etcdctl", "COMMAND", 2},
		{"dns", "SERVICE", 3},
		{"rbac", "SERVICE", 3},
		{"ai-memory", "SERVICE", 3},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := repairPriorityClass(tc.name, tc.kind)
			if got != tc.expected {
				t.Errorf("repairPriorityClass(%q, %q) = %d, want %d", tc.name, tc.kind, got, tc.expected)
			}
		})
	}
}

// ── Identity integrity classification ────────────────────────────────────────

// TestIdentityCorruptBlocksRepair verifies that the classifier blocks repair
// when identity integrity is corrupt.
func TestIdentityCorruptBlocksRepair(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"broken-node": {InstalledVersions: map[string]string{}},
		},
	}

	diagnosis := map[string]any{
		"identity_integrity_status": "corrupt",
		"node_id":                  "broken-node",
	}

	_, err := srv.repairClassify(nil, "broken-node", diagnosis)
	if err == nil {
		t.Fatal("expected classifier to block repair when identity is corrupt")
	}
	if !strings.Contains(err.Error(), "CORRUPT") {
		t.Errorf("error should mention CORRUPT, got: %v", err)
	}
}

// TestIdentitySuspectRotatesCerts verifies that suspect identity triggers
// rotate_certs in the repair plan.
func TestIdentitySuspectRotatesCerts(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"suspect-node": {InstalledVersions: map[string]string{"dns": ""}},
		},
	}

	diagnosis := map[string]any{
		"identity_integrity_status": "suspect",
		"node_id":                  "suspect-node",
	}

	plan, err := srv.repairClassify(nil, "suspect-node", diagnosis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan["identity_action"] != "rotate_certs" {
		t.Errorf("expected identity_action=rotate_certs, got %v", plan["identity_action"])
	}
}

// ── Reference validation ─────────────────────────────────────────────────────

// TestValidateReference_RejectsAbsentNode verifies that a non-existent
// reference node is rejected.
func TestValidateReference_RejectsAbsentNode(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{},
	}
	err := srv.validateReferenceNode(nil, "nonexistent")
	if err == nil {
		t.Fatal("expected error for non-existent reference node")
	}
}

// TestValidateReference_RejectsRepairingNode verifies that a reference
// node in "repairing" phase is rejected.
func TestValidateReference_RejectsRepairingNode(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"ref": {BootstrapPhase: "repairing", AppliedServicesHash: "something"},
		},
	}
	err := srv.validateReferenceNode(nil, "ref")
	if err == nil {
		t.Fatal("expected error for repairing reference node")
	}
}

// ── Classifier controller repair detection ───────────────────────────────────

// TestClassifier_DetectsOldController verifies that the classifier flags
// controller_repair_required when the broken node's controller is below
// minSafeReconcileVersion.
func TestClassifier_DetectsOldController(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"old-node": {
				InstalledVersions: map[string]string{
					"cluster-controller": "0.0.8", // below minSafeReconcileVersion
				},
			},
		},
	}

	diagnosis := map[string]any{
		"identity_integrity_status": "clean",
	}

	plan, err := srv.repairClassify(nil, "old-node", diagnosis)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if plan["controller_repair_required"] != true {
		t.Error("expected controller_repair_required=true for old controller")
	}
}

