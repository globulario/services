package main

import (
	"testing"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/workflow"
)

// mockEmitter captures emitted events for test assertions.
type mockEmitter struct {
	events []map[string]interface{}
}

func (m *mockEmitter) emitClusterEvent(eventType string, data map[string]interface{}) {
	data["_type"] = eventType
	m.events = append(m.events, data)
}

func (m *mockEmitter) getWorkflowRecorder() *workflow.Recorder {
	return nil
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
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "test-node", Ips: []string{"10.0.0.20"}},
		Profiles:           []string{"core", "gateway"},
		BootstrapPhase:     BootstrapAdmitted,
		LastSeen:           time.Now(),
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// admitted → infra_preparing (immediate)
	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty {
		t.Fatal("expected dirty")
	}
	if node.BootstrapPhase != BootstrapInfraPreparing {
		t.Fatalf("expected infra_preparing, got %s", node.BootstrapPhase)
	}

	// infra_preparing: no etcd unit yet → stays
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if dirty {
		t.Fatal("expected no change — etcd unit not present")
	}

	// infra_preparing → etcd_joining: etcd unit appears
	node.Units = []unitStatusRecord{{Name: "globular-etcd.service", State: "inactive"}}
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty || node.BootstrapPhase != BootstrapEtcdJoining {
		t.Fatalf("expected etcd_joining, got %s", node.BootstrapPhase)
	}

	// etcd_joining: wait for EtcdJoinPhase verified
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if dirty {
		t.Fatal("expected no change — etcd join not verified")
	}

	// etcd_joining → etcd_ready
	node.EtcdJoinPhase = EtcdJoinVerified
	// Runtime-aware bootstrap gating requires required infra runtime to be
	// healthy before advancing past envoy_ready.
	for i := range node.Units {
		if node.Units[i].Name == "globular-etcd.service" {
			node.Units[i].State = "active"
		}
	}
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty || node.BootstrapPhase != BootstrapEtcdReady {
		t.Fatalf("expected etcd_ready, got %s", node.BootstrapPhase)
	}

	// etcd_ready: wait for xDS active
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if dirty {
		t.Fatal("expected no change — xds not active")
	}

	// etcd_ready → xds_ready
	node.Units = append(node.Units, unitStatusRecord{Name: "globular-xds.service", State: "active"})
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty || node.BootstrapPhase != BootstrapXdsReady {
		t.Fatalf("expected xds_ready, got %s", node.BootstrapPhase)
	}

	// xds_ready: wait for envoy active
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if dirty {
		t.Fatal("expected no change — envoy not active")
	}

	// xds_ready → envoy_ready
	node.Units = append(node.Units, unitStatusRecord{Name: "globular-envoy.service", State: "active"})
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty || node.BootstrapPhase != BootstrapEnvoyReady {
		t.Fatalf("expected envoy_ready, got %s", node.BootstrapPhase)
	}

	// envoy_ready → storage_joining (core profile includes MinIO)
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty || node.BootstrapPhase != BootstrapStorageJoining {
		t.Fatalf("expected storage_joining, got %s", node.BootstrapPhase)
	}

	// storage_joining → workload_ready: MinIO join verified
	// (core+gateway has minio but NOT scylladb — scylladb is for control-plane/storage profiles)
	node.MinioJoinPhase = MinioJoinVerified
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty || node.BootstrapPhase != BootstrapWorkloadReady {
		t.Fatalf("expected workload_ready, got %s", node.BootstrapPhase)
	}

	// Verify events were emitted for each transition.
	if len(emitter.events) < 6 {
		t.Fatalf("expected at least 6 events, got %d", len(emitter.events))
	}
}

// TestBootstrap_SkipEtcd tests that a node without etcd profile skips
// etcd_joining and etcd_ready phases.
func TestBootstrap_SkipEtcd(t *testing.T) {
	emitter := &mockEmitter{}
	// "gateway" profile has no etcd, but has xds and envoy.
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "gw-node"},
		Profiles:           []string{"gateway"},
		BootstrapPhase:     BootstrapAdmitted,
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// admitted → infra_preparing
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapInfraPreparing {
		t.Fatalf("expected infra_preparing, got %s", node.BootstrapPhase)
	}

	// infra_preparing: no etcd profile → skips etcd phases.
	// Gateway has xds profile, so should land on etcd_ready (waiting for xds).
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapEtcdReady {
		t.Fatalf("expected etcd_ready (skip etcd join), got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_SkipEnvoy tests that a node with only "storage" profile skips
// etcd, xds, and envoy phases (none of those are in the storage profile).
func TestBootstrap_SkipEnvoy(t *testing.T) {
	emitter := &mockEmitter{}
	// "storage" profile runs MinIO but not etcd/xds/envoy.
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "storage-node", Ips: []string{"10.0.0.5"}},
		Profiles:           []string{"storage"},
		BootstrapPhase:     BootstrapAdmitted,
		LastSeen:           time.Now(),
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// admitted → infra_preparing
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapInfraPreparing {
		t.Fatalf("expected infra_preparing, got %s", node.BootstrapPhase)
	}

	// infra_preparing: no etcd/xds/envoy profile → skip to storage_joining
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapStorageJoining {
		t.Fatalf("expected storage_joining (no etcd/xds/envoy), got %s", node.BootstrapPhase)
	}

	// storage_joining: both MinIO and Scylla join verified → workload_ready
	// (storage profile has both minio and scylladb)
	node.MinioJoinPhase = MinioJoinVerified
	node.ScyllaJoinPhase = ScyllaJoinVerified
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapWorkloadReady {
		t.Fatalf("expected workload_ready, got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_Timeout tests that a stuck phase transitions to bootstrap_failed.
func TestBootstrap_Timeout(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "slow-node", Ips: []string{"10.0.0.5"}},
		Profiles:           []string{"core"},
		BootstrapPhase:     BootstrapInfraPreparing,
		BootstrapStartedAt: time.Now().Add(-bootstrapPhaseTimeout - time.Minute),
	}
	nodes := []*nodeState{node}

	// Should timeout and fail.
	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
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
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "fail-node", Ips: []string{"10.0.0.5"}},
		Profiles:           []string{"core"},
		BootstrapPhase:     BootstrapEtcdJoining,
		BootstrapStartedAt: time.Now(),
		EtcdJoinPhase:      EtcdJoinFailed,
		EtcdJoinError:      "quorum lost",
	}
	nodes := []*nodeState{node}

	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
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

	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
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

	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
	if dirty {
		t.Fatal("expected no change for workload_ready node")
	}
}

// TestBootstrap_FailedNodeAutoRetries tests that failed nodes are auto-retried
// by resetting to admitted so the phase machine re-evaluates.
func TestBootstrap_FailedNodeAutoRetries(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:         "n1",
		Profiles:       []string{"core"},
		BootstrapPhase: BootstrapFailed,
		BootstrapError: "something broke",
	}
	nodes := []*nodeState{node}

	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty {
		t.Fatal("expected dirty: failed nodes auto-retry by resetting to admitted")
	}
	// After auto-retry, the phase is reset and immediately advanced to infra_preparing.
	if node.BootstrapPhase == BootstrapFailed {
		t.Fatalf("expected phase reset from failed, got %s", node.BootstrapPhase)
	}
	if node.BootstrapError != "" {
		t.Fatalf("expected error cleared on retry, got %q", node.BootstrapError)
	}
}

func TestBootstrapDoesNotAdvanceOnInstalledOnly(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "joiner"},
		Profiles:           []string{"core", "gateway"},
		BootstrapPhase:     BootstrapEnvoyReady,
		BootstrapStartedAt: time.Now(),
		LastSeen:           time.Now(),
		// envoy active but minio inactive -> runtime mismatch
		Units: []unitStatusRecord{
			{Name: "globular-envoy.service", State: "active"},
			{Name: "globular-minio.service", State: "inactive"},
			{Name: "globular-sidekick.service", State: "active"},
			{Name: "globular-node-exporter.service", State: "active"},
			{Name: "globular-prometheus.service", State: "active"},
			{Name: "globular-alertmanager.service", State: "active"},
			{Name: "globular-gateway.service", State: "active"},
			{Name: "globular-xds.service", State: "active"},
			{Name: "globular-etcd.service", State: "active"},
		},
	}
	nodes := []*nodeState{node}

	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
	if dirty {
		t.Fatal("expected no phase change when required infra runtime is unhealthy")
	}
	if node.BootstrapPhase != BootstrapEnvoyReady {
		t.Fatalf("expected phase to stay envoy_ready, got %s", node.BootstrapPhase)
	}
	if node.BlockedReason != "day1_infra_runtime_blocked" {
		t.Fatalf("expected day1_infra_runtime_blocked, got %q", node.BlockedReason)
	}
	if node.BootstrapError == "" {
		t.Fatal("expected bootstrap error detail for runtime block")
	}

	// Runtime heals => block clears and phase advances.
	for i := range node.Units {
		if node.Units[i].Name == "globular-minio.service" {
			node.Units[i].State = "active"
		}
	}
	node.MinioJoinPhase = MinioJoinVerified
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty {
		t.Fatal("expected phase change after runtime converges")
	}
	if node.BlockedReason != "" {
		t.Fatalf("expected blocked reason cleared, got %q", node.BlockedReason)
	}
	if node.BootstrapPhase == BootstrapEnvoyReady {
		t.Fatal("expected bootstrap to advance after runtime recovery")
	}
}

// TestBootstrap_StorageOnlyNode tests a node with only storage profile
// (no etcd, no xds, no envoy) → skips to storage_joining, then workload_ready.
func TestBootstrap_StorageOnlyNode(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "storage-node"},
		Profiles:           []string{"storage"},
		BootstrapPhase:     BootstrapAdmitted,
		LastSeen:           time.Now(),
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// admitted → infra_preparing
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapInfraPreparing {
		t.Fatalf("expected infra_preparing, got %s", node.BootstrapPhase)
	}

	// infra_preparing: no etcd, no xds, no envoy → skips to storage_joining
	// (storage profile has MinIO, which needs join verification)
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapStorageJoining {
		t.Fatalf("expected storage_joining, got %s", node.BootstrapPhase)
	}

	// storage_joining: MinIO join not verified → stays
	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
	if dirty {
		t.Fatal("expected no change — minio join not verified")
	}

	// storage_joining → workload_ready: both MinIO and Scylla join verified
	// (storage profile has both minio and scylladb)
	node.MinioJoinPhase = MinioJoinVerified
	node.ScyllaJoinPhase = ScyllaJoinVerified
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty || node.BootstrapPhase != BootstrapWorkloadReady {
		t.Fatalf("expected workload_ready, got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_StorageJoin_CoreNode tests that a core node (which has MinIO)
// goes through storage_joining after envoy_ready.
func TestBootstrap_StorageJoin_CoreNode(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "core-node", Ips: []string{"10.0.0.5"}},
		Profiles:           []string{"core", "gateway"},
		BootstrapPhase:     BootstrapEnvoyReady,
		LastSeen:           time.Now(),
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// envoy_ready → storage_joining (core has MinIO)
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapStorageJoining {
		t.Fatalf("expected storage_joining, got %s", node.BootstrapPhase)
	}

	// MinIO join verified → workload_ready
	// (core+gateway has minio but NOT scylladb — scylladb only for control-plane/storage/scylla)
	node.MinioJoinPhase = MinioJoinVerified
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapWorkloadReady {
		t.Fatalf("expected workload_ready, got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_StorageJoin_ScyllaNode tests that a node with scylla profile
// waits for scylladb.service during storage_joining.
func TestBootstrap_StorageJoin_ScyllaNode(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "scylla-node"},
		Profiles:           []string{"scylla"},
		BootstrapPhase:     BootstrapAdmitted,
		LastSeen:           time.Now(),
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// admitted → infra_preparing → storage_joining (no etcd/xds/envoy, but has scylla)
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapInfraPreparing {
		t.Fatalf("expected infra_preparing, got %s", node.BootstrapPhase)
	}
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapStorageJoining {
		t.Fatalf("expected storage_joining, got %s", node.BootstrapPhase)
	}

	// ScyllaDB join not verified → stays in storage_joining
	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
	if dirty {
		t.Fatal("expected no change — scylla join not verified")
	}

	// ScyllaDB join verified → workload_ready
	node.ScyllaJoinPhase = ScyllaJoinVerified
	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty || node.BootstrapPhase != BootstrapWorkloadReady {
		t.Fatalf("expected workload_ready, got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_StorageJoin_Timeout tests that storage_joining times out.
func TestBootstrap_StorageJoin_Timeout(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "slow-storage"},
		Profiles:           []string{"storage"},
		BootstrapPhase:     BootstrapStorageJoining,
		BootstrapStartedAt: time.Now().Add(-bootstrapPhaseTimeout - time.Minute),
	}
	nodes := []*nodeState{node}

	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty {
		t.Fatal("expected dirty after timeout")
	}
	if node.BootstrapPhase != BootstrapFailed {
		t.Fatalf("expected bootstrap_failed, got %s", node.BootstrapPhase)
	}
	if node.BootstrapError == "" {
		t.Fatal("expected error message")
	}
}

// TestBootstrap_GatewayOnly_NoStorageJoin tests that a gateway-only node
// (no storage profile) skips storage_joining.
func TestBootstrap_GatewayOnly_NoStorageJoin(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "gw-only"},
		Profiles:           []string{"gateway"},
		BootstrapPhase:     BootstrapEnvoyReady,
		LastSeen:           time.Now(),
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// envoy_ready → workload_ready (no storage profile, skip storage_joining)
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapWorkloadReady {
		t.Fatalf("expected workload_ready (no storage), got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_DnsOnly_NoStorageJoin tests that a dns-only node
// skips all optional phases and goes straight to workload_ready.
func TestBootstrap_DnsOnly_NoStorageJoin(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "dns-only"},
		Profiles:           []string{"dns"},
		BootstrapPhase:     BootstrapAdmitted,
		BootstrapStartedAt: time.Now(),
	}
	nodes := []*nodeState{node}

	// admitted → infra_preparing
	reconcileBootstrapPhases(nodes, nil, emitter)
	// infra_preparing: no etcd, no xds, no envoy, no storage → workload_ready
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapWorkloadReady {
		t.Fatalf("expected workload_ready (dns-only, no storage), got %s", node.BootstrapPhase)
	}
}

// TestBootstrap_MinioSkip_Day1NonMember tests that a Day-1 storage node whose IP
// is not in the MinIO pool (pool non-empty) does not get blocked by the MinIO
// runtime check, allowing bootstrap to advance past envoy_ready.
func TestBootstrap_MinioSkip_Day1NonMember(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n2",
		Identity:           storedIdentity{Hostname: "day1-storage", Ips: []string{"10.0.0.99"}},
		Profiles:           []string{"core", "storage"},
		BootstrapPhase:     BootstrapEnvoyReady,
		BootstrapStartedAt: time.Now(),
		LastSeen:           time.Now(),
		EtcdJoinPhase:      EtcdJoinVerified,
		MinioJoinPhase:     MinioJoinNone, // not yet classified
		ScyllaJoinPhase:    ScyllaJoinVerified,
		Units: []unitStatusRecord{
			// minio is NOT active — node-agent topology gate is holding it
			{Name: "globular-minio.service", State: "inactive"},
			{Name: "scylla-server.service", State: "active"},
			{Name: "globular-etcd.service", State: "active"},
			{Name: "globular-xds.service", State: "active"},
			{Name: "globular-envoy.service", State: "active"},
		},
	}
	nodes := []*nodeState{node}
	// Pool is non-empty; this node's IP (10.0.0.99) is not in it.
	poolNodes := []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"}

	dirty := reconcileBootstrapPhases(nodes, poolNodes, emitter)
	if !dirty {
		t.Fatal("expected dirty=true when minio skipped via pool membership check")
	}
	// Must advance to storage_joining (has scylla profile), not block at envoy_ready.
	if node.BootstrapPhase != BootstrapStorageJoining {
		t.Fatalf("expected storage_joining, got %s (blocked: %s)", node.BootstrapPhase, node.BlockedDetails)
	}
}

// TestBootstrap_MinioSkip_NonMemberPhase tests that MinioJoinNonMember phase
// still skips the MinIO runtime check regardless of pool contents.
func TestBootstrap_MinioSkip_NonMemberPhase(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n3",
		Identity:           storedIdentity{Hostname: "non-member", Ips: []string{"10.0.0.99"}},
		Profiles:           []string{"core", "storage"},
		BootstrapPhase:     BootstrapEnvoyReady,
		BootstrapStartedAt: time.Now(),
		LastSeen:           time.Now(),
		EtcdJoinPhase:      EtcdJoinVerified,
		MinioJoinPhase:     MinioJoinNonMember,
		ScyllaJoinPhase:    ScyllaJoinVerified,
		Units: []unitStatusRecord{
			{Name: "globular-minio.service", State: "inactive"},
			{Name: "scylla-server.service", State: "active"},
			{Name: "globular-etcd.service", State: "active"},
			{Name: "globular-xds.service", State: "active"},
			{Name: "globular-envoy.service", State: "active"},
		},
	}
	nodes := []*nodeState{node}
	// Even with an empty pool, MinioJoinNonMember should skip the check.
	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty {
		t.Fatal("expected dirty=true")
	}
	if node.BootstrapPhase != BootstrapStorageJoining {
		t.Fatalf("expected storage_joining, got %s (blocked: %s)", node.BootstrapPhase, node.BlockedDetails)
	}
}

// TestBootstrap_MinioRequired_PoolMember tests that a node whose IP IS in the
// pool still requires MinIO to be active before advancing past envoy_ready.
func TestBootstrap_MinioRequired_PoolMember(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "pool-member", Ips: []string{"10.0.0.63"}},
		Profiles:           []string{"core", "storage"},
		BootstrapPhase:     BootstrapEnvoyReady,
		BootstrapStartedAt: time.Now(),
		EtcdJoinPhase:      EtcdJoinVerified,
		MinioJoinPhase:     MinioJoinVerified,
		ScyllaJoinPhase:    ScyllaJoinVerified,
		Units: []unitStatusRecord{
			{Name: "globular-minio.service", State: "inactive"}, // minio stopped
			{Name: "scylla-server.service", State: "active"},
			{Name: "globular-etcd.service", State: "active"},
			{Name: "globular-xds.service", State: "active"},
			{Name: "globular-envoy.service", State: "active"},
		},
	}
	nodes := []*nodeState{node}
	// This node IS in the pool — minio must be active.
	poolNodes := []string{"10.0.0.63", "10.0.0.8", "10.0.0.20"}

	dirty := reconcileBootstrapPhases(nodes, poolNodes, emitter)
	// Should be blocked: minio is inactive and node is a pool member.
	if node.BootstrapPhase != BootstrapEnvoyReady {
		t.Fatalf("expected to stay at envoy_ready (minio inactive), got %s", node.BootstrapPhase)
	}
	_ = dirty
}

// TestBootstrap_MinioRequired_Day0EmptyPool tests that on Day-0 (empty pool)
// a storage node still requires MinIO active before advancing.
func TestBootstrap_MinioRequired_Day0EmptyPool(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "day0", Ips: []string{"10.0.0.63"}},
		Profiles:           []string{"core", "storage"},
		BootstrapPhase:     BootstrapEnvoyReady,
		BootstrapStartedAt: time.Now(),
		EtcdJoinPhase:      EtcdJoinVerified,
		MinioJoinPhase:     MinioJoinNone,
		ScyllaJoinPhase:    ScyllaJoinVerified,
		Units: []unitStatusRecord{
			{Name: "globular-minio.service", State: "inactive"},
			{Name: "scylla-server.service", State: "active"},
			{Name: "globular-etcd.service", State: "active"},
			{Name: "globular-xds.service", State: "active"},
			{Name: "globular-envoy.service", State: "active"},
		},
	}
	nodes := []*nodeState{node}
	// Day-0: pool is empty. MinIO is inactive and not a NonMember.
	// Must be blocked until MinIO becomes active.
	reconcileBootstrapPhases(nodes, nil, emitter)
	if node.BootstrapPhase != BootstrapEnvoyReady {
		t.Fatalf("expected to stay at envoy_ready on Day-0 with minio inactive, got %s", node.BootstrapPhase)
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
	if len(infra) != 2 { // etcd, envoy are infra; dns is now KindWorkload+ManagedUnit
		t.Fatalf("expected 2 infra actions, got %d", len(infra))
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
