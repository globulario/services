package main

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// mockEmitter captures emitted events for test assertions.
type mockEmitter struct {
	events []map[string]interface{}
}

func (m *mockEmitter) emitClusterEvent(eventType string, data map[string]interface{}) {
	data["_type"] = eventType
	m.events = append(m.events, data)
}

func TestBootstrapPhaseReady(t *testing.T) {
	tests := []struct {
		phase BootstrapPhase
		want  bool
	}{
		{BootstrapNone, true},
		{BootstrapWorkloadReady, true},
		{BootstrapStorageJoining, true},
		{BootstrapAdmitted, false},
		{BootstrapInfraPreparing, false},
		{BootstrapEtcdJoining, false},
		{BootstrapEtcdReady, false},
		{BootstrapXdsReady, false},
		{BootstrapEnvoyReady, false},
		{BootstrapFailed, false},
	}
	for _, tt := range tests {
		if got := bootstrapPhaseReady(tt.phase); got != tt.want {
			t.Errorf("bootstrapPhaseReady(%q) = %v, want %v", tt.phase, got, tt.want)
		}
	}
}

// TestBootstrap_FullPath_CoreGateway tests the happy path for a node with
// core + gateway profiles (etcd + xds + envoy).
func TestBootstrap_FullPath_CoreGateway(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "test-node", Ips: []string{"10.0.0.20"}},
		Profiles:       []string{"core", "gateway"},
		BootstrapPhase: BootstrapAdmitted,
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// admitted → infra_preparing (immediate)
	dirty := reconcileBootstrapPhases(nodes, emitter)
	if !dirty {
		t.Fatal("expected dirty")
	}
	if node.BootstrapPhase != BootstrapInfraPreparing {
		t.Fatalf("expected infra_preparing, got %s", node.BootstrapPhase)
	}

	// infra_preparing: no etcd unit yet → stays
	dirty = reconcileBootstrapPhases(nodes, emitter)
	if dirty {
		t.Fatal("expected no change — etcd unit not present")
	}

	// infra_preparing → etcd_joining: etcd unit appears
	node.Units = []unitStatusRecord{{Name: "globular-etcd.service", State: "inactive"}}
	dirty = reconcileBootstrapPhases(nodes, emitter)
	if !dirty || node.BootstrapPhase != BootstrapEtcdJoining {
		t.Fatalf("expected etcd_joining, got %s", node.BootstrapPhase)
	}

	// etcd_joining: wait for EtcdJoinPhase verified
	dirty = reconcileBootstrapPhases(nodes, emitter)
	if dirty {
		t.Fatal("expected no change — etcd join not verified")
	}

	// etcd_joining → etcd_ready
	node.EtcdJoinPhase = EtcdJoinVerified
	dirty = reconcileBootstrapPhases(nodes, emitter)
	if !dirty || node.BootstrapPhase != BootstrapEtcdReady {
		t.Fatalf("expected etcd_ready, got %s", node.BootstrapPhase)
	}

	// etcd_ready: wait for xDS active
	dirty = reconcileBootstrapPhases(nodes, emitter)
	if dirty {
		t.Fatal("expected no change — xds not active")
	}

	// etcd_ready → xds_ready
	node.Units = append(node.Units, unitStatusRecord{Name: "globular-xds.service", State: "active"})
	dirty = reconcileBootstrapPhases(nodes, emitter)
	if !dirty || node.BootstrapPhase != BootstrapXdsReady {
		t.Fatalf("expected xds_ready, got %s", node.BootstrapPhase)
	}

	// xds_ready: wait for envoy active
	dirty = reconcileBootstrapPhases(nodes, emitter)
	if dirty {
		t.Fatal("expected no change — envoy not active")
	}

	// xds_ready → envoy_ready
	node.Units = append(node.Units, unitStatusRecord{Name: "globular-envoy.service", State: "active"})
	dirty = reconcileBootstrapPhases(nodes, emitter)
	if !dirty || node.BootstrapPhase != BootstrapEnvoyReady {
		t.Fatalf("expected envoy_ready, got %s", node.BootstrapPhase)
	}

	// envoy_ready → workload_ready (immediate)
	dirty = reconcileBootstrapPhases(nodes, emitter)
	if !dirty || node.BootstrapPhase != BootstrapWorkloadReady {
		t.Fatalf("expected workload_ready, got %s", node.BootstrapPhase)
	}

	// Verify events were emitted for each transition.
	if len(emitter.events) < 5 {
		t.Fatalf("expected at least 5 events, got %d", len(emitter.events))
	}
}

// TestBootstrap_SkipEtcd tests that a node without etcd profile skips
// etcd_joining and etcd_ready phases.
func TestBootstrap_SkipEtcd(t *testing.T) {
	emitter := &mockEmitter{}
	// "gateway" profile has no etcd, but has xds and envoy.
	node := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "gw-node"},
		Profiles:       []string{"gateway"},
		BootstrapPhase: BootstrapAdmitted,
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// admitted → infra_preparing
	reconcileBootstrapPhases(nodes, emitter)
	if node.BootstrapPhase != BootstrapInfraPreparing {
		t.Fatalf("expected infra_preparing, got %s", node.BootstrapPhase)
	}

	// infra_preparing: no etcd profile → skips etcd phases.
	// Gateway has xds profile, so should land on etcd_ready (waiting for xds).
	reconcileBootstrapPhases(nodes, emitter)
	if node.BootstrapPhase != BootstrapEtcdReady {
		t.Fatalf("expected etcd_ready (skip etcd join), got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_SkipEnvoy tests that a node without gateway profile skips
// envoy phases and goes straight to workload_ready.
func TestBootstrap_SkipEnvoy(t *testing.T) {
	emitter := &mockEmitter{}
	// "control-plane" has etcd + xds but no envoy/gateway.
	node := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "cp-node", Ips: []string{"10.0.0.5"}},
		Profiles:       []string{"control-plane"},
		BootstrapPhase: BootstrapAdmitted,
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// admitted → infra_preparing
	reconcileBootstrapPhases(nodes, emitter)

	// infra_preparing → etcd_joining (has etcd profile, unit present)
	node.Units = []unitStatusRecord{{Name: "globular-etcd.service", State: "inactive"}}
	reconcileBootstrapPhases(nodes, emitter)
	if node.BootstrapPhase != BootstrapEtcdJoining {
		t.Fatalf("expected etcd_joining, got %s", node.BootstrapPhase)
	}

	// etcd_joining → etcd_ready
	node.EtcdJoinPhase = EtcdJoinVerified
	reconcileBootstrapPhases(nodes, emitter)
	if node.BootstrapPhase != BootstrapEtcdReady {
		t.Fatalf("expected etcd_ready, got %s", node.BootstrapPhase)
	}

	// etcd_ready → xds_ready (has xds profile via control-plane)
	node.Units = append(node.Units, unitStatusRecord{Name: "globular-xds.service", State: "active"})
	reconcileBootstrapPhases(nodes, emitter)
	if node.BootstrapPhase != BootstrapXdsReady {
		t.Fatalf("expected xds_ready, got %s", node.BootstrapPhase)
	}

	// xds_ready: no gateway profile → skip envoy → workload_ready
	reconcileBootstrapPhases(nodes, emitter)
	if node.BootstrapPhase != BootstrapWorkloadReady {
		t.Fatalf("expected workload_ready (no envoy), got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_Timeout tests that a stuck phase transitions to bootstrap_failed.
func TestBootstrap_Timeout(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "slow-node", Ips: []string{"10.0.0.5"}},
		Profiles:       []string{"core"},
		BootstrapPhase: BootstrapInfraPreparing,
		BootstrapStartedAt: time.Now().Add(-bootstrapPhaseTimeout - time.Minute),
	}
	nodes := []*nodeState{node}

	// Should timeout and fail.
	dirty := reconcileBootstrapPhases(nodes, emitter)
	if !dirty {
		t.Fatal("expected dirty after timeout")
	}
	if node.BootstrapPhase != BootstrapFailed {
		t.Fatalf("expected bootstrap_failed, got %s", node.BootstrapPhase)
	}
	if node.BootstrapError == "" {
		t.Fatal("expected error message after timeout")
	}
}

// TestBootstrap_EtcdJoinFailed tests that etcd join failure propagates to bootstrap failure.
func TestBootstrap_EtcdJoinFailed(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "fail-node", Ips: []string{"10.0.0.5"}},
		Profiles:       []string{"core"},
		BootstrapPhase: BootstrapEtcdJoining,
		BootstrapStartedAt: time.Now(),
		EtcdJoinPhase:  EtcdJoinFailed,
		EtcdJoinError:  "quorum lost",
	}
	nodes := []*nodeState{node}

	dirty := reconcileBootstrapPhases(nodes, emitter)
	if !dirty {
		t.Fatal("expected dirty")
	}
	if node.BootstrapPhase != BootstrapFailed {
		t.Fatalf("expected bootstrap_failed, got %s", node.BootstrapPhase)
	}
	if node.BootstrapError != "etcd join failed: quorum lost" {
		t.Fatalf("unexpected error: %q", node.BootstrapError)
	}
}

// TestBootstrap_LegacyNode tests that a node with empty BootstrapPhase is
// not processed by the bootstrap state machine.
func TestBootstrap_LegacyNode(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "legacy"},
		Profiles:       []string{"core"},
		BootstrapPhase: BootstrapNone,
	}
	nodes := []*nodeState{node}

	dirty := reconcileBootstrapPhases(nodes, emitter)
	if dirty {
		t.Fatal("expected no change for legacy node")
	}
	if node.BootstrapPhase != BootstrapNone {
		t.Fatalf("expected empty phase unchanged, got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_WorkloadReadyNode tests that workload_ready nodes are skipped.
func TestBootstrap_WorkloadReadyNode(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:         "n1",
		Profiles:       []string{"core"},
		BootstrapPhase: BootstrapWorkloadReady,
	}
	nodes := []*nodeState{node}

	dirty := reconcileBootstrapPhases(nodes, emitter)
	if dirty {
		t.Fatal("expected no change for workload_ready node")
	}
}

// TestBootstrap_FailedNodeStays tests that failed nodes are not re-processed
// (require manual reset).
func TestBootstrap_FailedNodeStays(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:         "n1",
		Profiles:       []string{"core"},
		BootstrapPhase: BootstrapFailed,
		BootstrapError: "something broke",
	}
	nodes := []*nodeState{node}

	dirty := reconcileBootstrapPhases(nodes, emitter)
	if dirty {
		t.Fatal("expected no change for failed node")
	}
	if node.BootstrapPhase != BootstrapFailed {
		t.Fatalf("expected failed unchanged, got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_StorageOnlyNode tests a node with only storage profile
// (no etcd, no xds, no envoy) → fast path to workload_ready.
func TestBootstrap_StorageOnlyNode(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "storage-node"},
		Profiles:       []string{"storage"},
		BootstrapPhase: BootstrapAdmitted,
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// admitted → infra_preparing
	reconcileBootstrapPhases(nodes, emitter)
	if node.BootstrapPhase != BootstrapInfraPreparing {
		t.Fatalf("expected infra_preparing, got %s", node.BootstrapPhase)
	}

	// infra_preparing: no etcd, no xds, no envoy → skip all → workload_ready
	reconcileBootstrapPhases(nodes, emitter)
	if node.BootstrapPhase != BootstrapWorkloadReady {
		t.Fatalf("expected workload_ready (all skipped), got %s", node.BootstrapPhase)
	}
}

// TestFilterActionsByMaxTier tests tier-based filtering of unit actions.
func TestFilterActionsByMaxTier(t *testing.T) {
	actions := []*cluster_controllerpb.UnitAction{
		{UnitName: "globular-etcd.service", Action: "start"},
		{UnitName: "globular-dns.service", Action: "start"},
		{UnitName: "globular-event.service", Action: "start"},
		{UnitName: "globular-rbac.service", Action: "start"},
		{UnitName: "globular-envoy.service", Action: "start"},
	}

	infra := filterActionsByMaxTier(actions, TierInfrastructure)
	if len(infra) != 3 { // etcd, dns, envoy are infra
		t.Fatalf("expected 3 infra actions, got %d", len(infra))
	}
	for _, a := range infra {
		if getUnitTier(a.UnitName) != TierInfrastructure {
			t.Errorf("non-infra unit in filtered result: %s", a.UnitName)
		}
	}

	all := filterActionsByMaxTier(actions, TierWorkload)
	if len(all) != 5 {
		t.Fatalf("expected 5 actions for workload tier, got %d", len(all))
	}
}
