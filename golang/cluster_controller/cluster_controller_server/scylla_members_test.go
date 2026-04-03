package main

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestNodeIsPreparedForScyllaJoin(t *testing.T) {
	tests := []struct {
		name string
		node *nodeState
		want bool
	}{
		{
			name: "prepared: has scylla profile, unit, routable IP",
			node: &nodeState{
				Profiles:       []string{"scylla"},
				Identity:       storedIdentity{Ips: []string{"10.0.0.5"}},
				Units:          []unitStatusRecord{{Name: "scylla-server.service", State: "inactive"}},
				BootstrapPhase: BootstrapStorageJoining,
			},
			want: true,
		},
		{
			name: "not prepared: no scylla profile",
			node: &nodeState{
				Profiles:       []string{"core"},
				Identity:       storedIdentity{Ips: []string{"10.0.0.5"}},
				Units:          []unitStatusRecord{{Name: "scylla-server.service", State: "inactive"}},
				BootstrapPhase: BootstrapWorkloadReady,
			},
			want: false,
		},
		{
			name: "not prepared: no unit file",
			node: &nodeState{
				Profiles:       []string{"scylla"},
				Identity:       storedIdentity{Ips: []string{"10.0.0.5"}},
				BootstrapPhase: BootstrapStorageJoining,
			},
			want: false,
		},
		{
			name: "not prepared: localhost IP",
			node: &nodeState{
				Profiles:       []string{"scylla"},
				Identity:       storedIdentity{Ips: []string{"127.0.0.1"}},
				Units:          []unitStatusRecord{{Name: "scylla-server.service", State: "inactive"}},
				BootstrapPhase: BootstrapStorageJoining,
			},
			want: false,
		},
		{
			name: "not prepared: mid-join (configured)",
			node: &nodeState{
				Profiles:        []string{"scylla"},
				Identity:        storedIdentity{Ips: []string{"10.0.0.5"}},
				Units:           []unitStatusRecord{{Name: "scylla-server.service", State: "inactive"}},
				BootstrapPhase:  BootstrapStorageJoining,
				ScyllaJoinPhase: ScyllaJoinConfigured,
			},
			want: false,
		},
		{
			name: "not prepared: wrong bootstrap phase (infra_preparing)",
			node: &nodeState{
				Profiles:       []string{"scylla"},
				Identity:       storedIdentity{Ips: []string{"10.0.0.5"}},
				Units:          []unitStatusRecord{{Name: "scylla-server.service", State: "inactive"}},
				BootstrapPhase: BootstrapInfraPreparing,
			},
			want: false,
		},
		{
			name: "prepared: database profile works too",
			node: &nodeState{
				Profiles:       []string{"database"},
				Identity:       storedIdentity{Ips: []string{"10.0.0.5"}},
				Units:          []unitStatusRecord{{Name: "scylla-server.service", State: "inactive"}},
				BootstrapPhase: BootstrapWorkloadReady,
			},
			want: true,
		},
		{
			name: "prepared: failed phase allows retry",
			node: &nodeState{
				Profiles:        []string{"scylla"},
				Identity:        storedIdentity{Ips: []string{"10.0.0.5"}},
				Units:           []unitStatusRecord{{Name: "scylla-server.service", State: "inactive"}},
				BootstrapPhase:  BootstrapStorageJoining,
				ScyllaJoinPhase: ScyllaJoinFailed,
			},
			want: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeIsPreparedForScyllaJoin(tt.node)
			if got != tt.want {
				t.Errorf("nodeIsPreparedForScyllaJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestScyllaJoin_HappyPath tests: prepared → configured → started → verified.
func TestScyllaJoin_HappyPath(t *testing.T) {
	mgr := newScyllaClusterManager()

	node := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "scylla-node", Ips: []string{"10.0.0.5"}},
		Profiles:       []string{"scylla"},
		Units:          []unitStatusRecord{{Name: "scylla-server.service", State: "inactive"}},
		BootstrapPhase: BootstrapStorageJoining,
	}
	nodes := []*nodeState{node}

	// none → configured (prepared checks pass)
	dirty := mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if !dirty {
		t.Fatal("expected dirty")
	}
	if node.ScyllaJoinPhase != ScyllaJoinConfigured {
		t.Fatalf("expected configured, got %s", node.ScyllaJoinPhase)
	}

	// configured: service not active yet → stays
	dirty = mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if dirty {
		t.Fatal("expected no change — scylla not active")
	}

	// configured → started: service goes active
	node.Units = []unitStatusRecord{{Name: "scylla-server.service", State: "active"}}
	dirty = mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if !dirty || node.ScyllaJoinPhase != ScyllaJoinStarted {
		t.Fatalf("expected started, got %s", node.ScyllaJoinPhase)
	}

	// started → verified: after 30s heuristic
	node.ScyllaJoinStartedAt = time.Now().Add(-35 * time.Second)
	dirty = mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if !dirty || node.ScyllaJoinPhase != ScyllaJoinVerified {
		t.Fatalf("expected verified, got %s", node.ScyllaJoinPhase)
	}
}

// TestScyllaJoin_Timeout tests that a stuck join times out.
func TestScyllaJoin_Timeout(t *testing.T) {
	mgr := newScyllaClusterManager()

	node := &nodeState{
		NodeID:              "n1",
		Identity:            storedIdentity{Hostname: "slow-scylla", Ips: []string{"10.0.0.5"}},
		Profiles:            []string{"scylla"},
		Units:               []unitStatusRecord{{Name: "scylla-server.service", State: "inactive"}},
		BootstrapPhase:      BootstrapStorageJoining,
		ScyllaJoinPhase:     ScyllaJoinConfigured,
		ScyllaJoinStartedAt: time.Now().Add(-scyllaJoinTimeout - time.Minute),
	}
	nodes := []*nodeState{node}

	dirty := mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if !dirty {
		t.Fatal("expected dirty after timeout")
	}
	if node.ScyllaJoinPhase != ScyllaJoinFailed {
		t.Fatalf("expected failed, got %s", node.ScyllaJoinPhase)
	}
	if node.ScyllaJoinError == "" {
		t.Fatal("expected error message")
	}
}

// TestScyllaJoin_NonScyllaNodeSkipped tests that nodes without scylla profile are skipped.
func TestScyllaJoin_NonScyllaNodeSkipped(t *testing.T) {
	mgr := newScyllaClusterManager()

	node := &nodeState{
		NodeID:         "n1",
		Profiles:       []string{"core"},
		BootstrapPhase: BootstrapWorkloadReady,
	}
	nodes := []*nodeState{node}

	dirty := mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if dirty {
		t.Fatal("expected no change for non-scylla node")
	}
}

// TestScyllaJoin_VerifiedResetsOnStop tests that if ScyllaDB stops,
// the join phase resets to allow re-join.
func TestScyllaJoin_VerifiedResetsOnStop(t *testing.T) {
	mgr := newScyllaClusterManager()

	node := &nodeState{
		NodeID:          "n1",
		Identity:        storedIdentity{Hostname: "scylla-node", Ips: []string{"10.0.0.5"}},
		Profiles:        []string{"scylla"},
		Units:           []unitStatusRecord{{Name: "scylla-server.service", State: "inactive"}},
		BootstrapPhase:  BootstrapWorkloadReady,
		ScyllaJoinPhase: ScyllaJoinVerified,
	}
	nodes := []*nodeState{node}

	// ScyllaDB stopped → reset to none
	dirty := mgr.reconcileScyllaJoinPhases(context.Background(), nodes)
	if !dirty {
		t.Fatal("expected dirty after service stopped")
	}
	if node.ScyllaJoinPhase != ScyllaJoinNone {
		t.Fatalf("expected none after reset, got %s", node.ScyllaJoinPhase)
	}
}

// TestRenderScyllaConfig tests the ScyllaDB configuration renderer.
func TestRenderScyllaConfig(t *testing.T) {
	ctx := &serviceConfigContext{
		Membership: &clusterMembership{
			ClusterID: "test.globular.internal",
			Nodes: []memberNode{
				{NodeID: "n1", Hostname: "scylla-1", IP: "10.0.0.5", Profiles: []string{"scylla"}},
				{NodeID: "n2", Hostname: "scylla-2", IP: "10.0.0.6", Profiles: []string{"scylla"}},
				{NodeID: "n3", Hostname: "core-node", IP: "10.0.0.10", Profiles: []string{"core"}},
			},
		},
		CurrentNode: &memberNode{NodeID: "n1", Hostname: "scylla-1", IP: "10.0.0.5", Profiles: []string{"scylla"}},
		ClusterID:   "test.globular.internal",
	}

	content, ok := renderScyllaConfig(ctx)
	if !ok {
		t.Fatal("expected renderScyllaConfig to succeed")
	}

	// Verify key fields.
	if !strings.Contains(content, "cluster_name: 'test.globular.internal'") {
		t.Error("missing cluster_name")
	}
	if !strings.Contains(content, "listen_address: '10.0.0.5'") {
		t.Error("missing listen_address")
	}
	if !strings.Contains(content, "rpc_address: '10.0.0.5'") {
		t.Error("missing rpc_address")
	}
	// Seeds should include both scylla nodes but not the core node.
	if !strings.Contains(content, "10.0.0.5") || !strings.Contains(content, "10.0.0.6") {
		t.Error("seeds should include both scylla nodes")
	}
	if !strings.Contains(content, "seeds:") {
		t.Error("missing seed_provider")
	}
	if !strings.Contains(content, "native_transport_port: 9042") {
		t.Error("missing native_transport_port")
	}
}

// TestRenderScyllaConfig_NoProfile tests that non-scylla nodes get no config.
func TestRenderScyllaConfig_NoProfile(t *testing.T) {
	ctx := &serviceConfigContext{
		Membership: &clusterMembership{
			Nodes: []memberNode{
				{NodeID: "n1", Hostname: "core-node", IP: "10.0.0.10", Profiles: []string{"core"}},
			},
		},
		CurrentNode: &memberNode{NodeID: "n1", Hostname: "core-node", IP: "10.0.0.10", Profiles: []string{"core"}},
	}

	_, ok := renderScyllaConfig(ctx)
	if ok {
		t.Fatal("expected renderScyllaConfig to return false for non-scylla node")
	}
}

// TestRenderScyllaConfig_SingleNode tests single-node ScyllaDB config.
func TestRenderScyllaConfig_SingleNode(t *testing.T) {
	ctx := &serviceConfigContext{
		Membership: &clusterMembership{
			ClusterID: "globular.local",
			Nodes: []memberNode{
				{NodeID: "n1", Hostname: "db-1", IP: "10.0.0.5", Profiles: []string{"database"}},
			},
		},
		CurrentNode: &memberNode{NodeID: "n1", Hostname: "db-1", IP: "10.0.0.5", Profiles: []string{"database"}},
		ClusterID:   "globular.local",
	}

	content, ok := renderScyllaConfig(ctx)
	if !ok {
		t.Fatal("expected renderScyllaConfig to succeed for database profile")
	}
	// Single-node seeds should be just this node.
	if !strings.Contains(content, "seeds: '10.0.0.5'") {
		t.Errorf("single-node seeds wrong, got:\n%s", content)
	}
}
