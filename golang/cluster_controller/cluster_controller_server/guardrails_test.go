package main

import (
	"context"
	"encoding/json"
	"testing"
)

// TestSchemaGuardRepairRequired verifies that schemaGuardStatus with
// repair_required=true deserializes correctly.
func TestSchemaGuardRepairRequired(t *testing.T) {
	raw := `{
		"keyspace": "dns",
		"strategy": "SimpleStrategy",
		"current_rf": 3,
		"required_rf": 3,
		"violation": false,
		"repair_required": true,
		"repair_required_since_unix": 1746000000,
		"updated_at_unix": 1746000001
	}`
	var st schemaGuardStatus
	if err := json.Unmarshal([]byte(raw), &st); err != nil {
		t.Fatalf("unexpected unmarshal error: %v", err)
	}
	if !st.RepairRequired {
		t.Error("expected RepairRequired=true, got false")
	}
	if st.RepairRequiredSinceUnix != 1746000000 {
		t.Errorf("expected RepairRequiredSinceUnix=1746000000, got %d", st.RepairRequiredSinceUnix)
	}
	if st.Violation {
		t.Error("expected Violation=false after successful ALTER, got true")
	}

	// Round-trip
	b, err := json.Marshal(st)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var st2 schemaGuardStatus
	if err := json.Unmarshal(b, &st2); err != nil {
		t.Fatalf("re-unmarshal failed: %v", err)
	}
	if st2.RepairRequired != st.RepairRequired {
		t.Errorf("round-trip: RepairRequired mismatch: got %v", st2.RepairRequired)
	}
}

// TestProjectionRFEnforcement verifies that projectionReplicationFactor
// returns correct values for various cluster sizes.
func TestProjectionRFEnforcement(t *testing.T) {
	cases := []struct {
		hosts int
		want  int
	}{
		{1, 1},
		{2, 2},
		{3, 3},
		{5, 3},
		{10, 3},
	}
	for _, tc := range cases {
		got := projectionReplicationFactor(tc.hosts)
		if got != tc.want {
			t.Errorf("projectionReplicationFactor(%d) = %d, want %d", tc.hosts, got, tc.want)
		}
	}
}

// TestTopologyPreflightStorageQuorum verifies that removing a storage node below
// 3 is blocked, and that removing a non-critical node is allowed.
func TestTopologyPreflightStorageQuorum(t *testing.T) {
	srv := &server{
		state: &controllerState{
			Nodes: map[string]*nodeState{
				"n1": {NodeID: "n1", Status: "ready", Profiles: []string{"core", "storage", "control-plane"}},
				"n2": {NodeID: "n2", Status: "ready", Profiles: []string{"core", "storage", "control-plane"}},
				"n3": {NodeID: "n3", Status: "ready", Profiles: []string{"core", "storage", "control-plane"}},
			},
		},
	}

	// Removing n3 would leave exactly 2 storage nodes — below minimum 3.
	violations := srv.topologyPreflightForRemove("n3")
	if len(violations) == 0 {
		t.Fatal("expected storage_quorum violation when removing a node from 3-node cluster")
	}
	found := false
	for _, v := range violations {
		if v.Kind == "storage_quorum" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected storage_quorum violation, got: %v", violations)
	}

	// Add a 4th storage node — removing n3 now leaves 3 (fine).
	srv.state.Nodes["n4"] = &nodeState{
		NodeID: "n4", Status: "ready", Profiles: []string{"core", "storage"},
	}
	violations2 := srv.topologyPreflightForRemove("n3")
	for _, v := range violations2 {
		if v.Kind == "storage_quorum" {
			t.Errorf("unexpected storage_quorum violation with 4 nodes: %s", v.Message)
		}
	}
}

// TestTopologyPreflightControllerPlacement verifies that removing the last
// control-plane node is blocked.
func TestTopologyPreflightControllerPlacement(t *testing.T) {
	srv := &server{
		state: &controllerState{
			Nodes: map[string]*nodeState{
				"cp1": {NodeID: "cp1", Status: "ready", Profiles: []string{"control-plane", "core", "storage"}},
				"s1":  {NodeID: "s1", Status: "ready", Profiles: []string{"core", "storage"}},
				"s2":  {NodeID: "s2", Status: "ready", Profiles: []string{"core", "storage"}},
				"s3":  {NodeID: "s3", Status: "ready", Profiles: []string{"core", "storage"}},
			},
		},
	}

	// cp1 is the only control-plane node — removing it must be blocked.
	violations := srv.topologyPreflightForRemove("cp1")
	found := false
	for _, v := range violations {
		if v.Kind == "controller_placement" || v.Kind == "ingress_participant" {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected controller_placement or ingress_participant violation, got: %v", violations)
	}
}

// TestIngressSpecExplicitDisabled verifies that ingressDesiredSpec with
// ExplicitDisabled=false does not become true after JSON round-trip, and
// that ExplicitDisabled=true survives round-trip.
// This exercises the controller-side hold-safe vs stop-keepalived distinction.
func TestIngressSpecExplicitDisabled(t *testing.T) {
	// Case: disabled with explicit_disabled=false should NOT stop keepalived.
	spec := ingressDesiredSpec{
		Version:          "v1",
		Mode:             ingressModeDisabled,
		ExplicitDisabled: false,
	}
	b, err := json.Marshal(spec)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var spec2 ingressDesiredSpec
	if err := json.Unmarshal(b, &spec2); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if spec2.ExplicitDisabled {
		t.Error("ExplicitDisabled=false should not become true after round-trip")
	}
	if spec2.Mode != ingressModeDisabled {
		t.Errorf("Mode mismatch: got %v", spec2.Mode)
	}

	// Case: disabled with explicit_disabled=true should stop keepalived.
	spec3 := ingressDesiredSpec{
		Version:          "v1",
		Mode:             ingressModeDisabled,
		ExplicitDisabled: true,
	}
	b2, err := json.Marshal(spec3)
	if err != nil {
		t.Fatalf("marshal failed: %v", err)
	}
	var spec4 ingressDesiredSpec
	if err := json.Unmarshal(b2, &spec4); err != nil {
		t.Fatalf("unmarshal failed: %v", err)
	}
	if !spec4.ExplicitDisabled {
		t.Error("ExplicitDisabled=true should survive round-trip")
	}
}

// Case 05: critical state registry completeness and writer validation.

func TestCriticalStateRegistry_AllEntriesComplete(t *testing.T) {
	for _, rec := range criticalStateRegistry {
		if rec.Key == "" {
			t.Errorf("registry entry has empty Key: %+v", rec)
		}
		if rec.Owner == "" {
			t.Errorf("registry entry %q has empty Owner", rec.Key)
		}
		if rec.SchemaVersion == "" {
			t.Errorf("registry entry %q has empty SchemaVersion", rec.Key)
		}
		if rec.DoctorInvariant == "" {
			t.Errorf("registry entry %q has empty DoctorInvariant", rec.Key)
		}
		if rec.GuardedBy == "" {
			t.Errorf("registry entry %q has empty GuardedBy", rec.Key)
		}
	}
}

func TestValidateCriticalKeyWrite_OwnerAllowed(t *testing.T) {
	for _, rec := range criticalStateRegistry {
		if err := ValidateCriticalKeyWrite(rec.Key, rec.Owner); err != nil {
			t.Errorf("owner write should be allowed for %q: %v", rec.Key, err)
		}
	}
}

func TestValidateCriticalKeyWrite_NonOwnerRejected(t *testing.T) {
	for _, rec := range criticalStateRegistry {
		if rec.IsPrefix {
			continue // prefix keys use range checks, skip for exact-match test
		}
		if err := ValidateCriticalKeyWrite(rec.Key, "rogue-writer"); err == nil {
			t.Errorf("expected rejection for non-owner write to %q", rec.Key)
		}
	}
}

func TestValidateCriticalKeyWrite_UnknownKeyAllowed(t *testing.T) {
	if err := ValidateCriticalKeyWrite("/globular/unknown/key", "anyone"); err != nil {
		t.Errorf("unknown key should pass through: %v", err)
	}
}

func TestDriftTopologyPreflight_BlocksUnsafeCluster(t *testing.T) {
	srv := &server{
		state: &controllerState{
			Nodes: map[string]*nodeState{
				"n1": {NodeID: "n1", Status: "ready", Profiles: []string{"storage", "control-plane"}},
				"n2": {NodeID: "n2", Status: "ready", Profiles: []string{"storage"}},
			},
		},
	}
	violations := srv.driftTopologyPreflight(context.Background())
	if len(violations) == 0 {
		t.Fatal("expected drift topology preflight violations")
	}
	foundStorage := false
	for _, v := range violations {
		if v.Kind == "storage_quorum" {
			foundStorage = true
			break
		}
	}
	if !foundStorage {
		t.Fatalf("expected storage_quorum violation, got: %+v", violations)
	}
}
