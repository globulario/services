package main

import (
	"strings"
	"testing"
	"time"
)

func TestNodeIsPreparedForMinioJoin(t *testing.T) {
	tests := []struct {
		name string
		node *nodeState
		want bool
	}{
		{
			name: "prepared: core profile with minio unit",
			node: &nodeState{
				Profiles:       []string{"core"},
				Identity:       storedIdentity{Ips: []string{"10.0.0.5"}},
				Units:          []unitStatusRecord{{Name: "globular-minio.service", State: "inactive"}},
				BootstrapPhase: BootstrapStorageJoining,
			},
			want: true,
		},
		{
			name: "prepared: storage profile",
			node: &nodeState{
				Profiles:       []string{"storage"},
				Identity:       storedIdentity{Ips: []string{"10.0.0.5"}},
				Units:          []unitStatusRecord{{Name: "globular-minio.service", State: "inactive"}},
				BootstrapPhase: BootstrapWorkloadReady,
			},
			want: true,
		},
		{
			name: "not prepared: no minio profile",
			node: &nodeState{
				Profiles:       []string{"gateway"},
				Identity:       storedIdentity{Ips: []string{"10.0.0.5"}},
				Units:          []unitStatusRecord{{Name: "globular-minio.service", State: "inactive"}},
				BootstrapPhase: BootstrapWorkloadReady,
			},
			want: false,
		},
		{
			name: "not prepared: no unit",
			node: &nodeState{
				Profiles:       []string{"core"},
				Identity:       storedIdentity{Ips: []string{"10.0.0.5"}},
				BootstrapPhase: BootstrapWorkloadReady,
			},
			want: false,
		},
		{
			name: "not prepared: mid-join",
			node: &nodeState{
				Profiles:       []string{"core"},
				Identity:       storedIdentity{Ips: []string{"10.0.0.5"}},
				Units:          []unitStatusRecord{{Name: "globular-minio.service", State: "inactive"}},
				BootstrapPhase: BootstrapWorkloadReady,
				MinioJoinPhase: MinioJoinPoolUpdated,
			},
			want: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := nodeIsPreparedForMinioJoin(tt.node)
			if got != tt.want {
				t.Errorf("nodeIsPreparedForMinioJoin() = %v, want %v", got, tt.want)
			}
		})
	}
}

// TestMinioJoin_HappyPath tests: none → prepared → pool_updated → started → verified.
func TestMinioJoin_HappyPath(t *testing.T) {
	mgr := newMinioPoolManager()
	state := &controllerState{MinioCredentials: generateMinioCredentials()}

	node := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "core-1", Ips: []string{"10.0.0.5"}},
		Profiles:       []string{"core"},
		Units:          []unitStatusRecord{{Name: "globular-minio.service", State: "inactive"}},
		BootstrapPhase: BootstrapStorageJoining,
	}
	nodes := []*nodeState{node}

	// none → prepared
	dirty := mgr.reconcileMinioJoinPhases(nodes, state)
	if !dirty || node.MinioJoinPhase != MinioJoinPrepared {
		t.Fatalf("expected prepared, got %s", node.MinioJoinPhase)
	}

	// prepared → pool_updated (IP appended)
	dirty = mgr.reconcileMinioJoinPhases(nodes, state)
	if !dirty || node.MinioJoinPhase != MinioJoinPoolUpdated {
		t.Fatalf("expected pool_updated, got %s", node.MinioJoinPhase)
	}
	if len(state.MinioPoolNodes) != 1 || state.MinioPoolNodes[0] != "10.0.0.5" {
		t.Fatalf("expected pool to contain 10.0.0.5, got %v", state.MinioPoolNodes)
	}

	// pool_updated: minio not running → stays
	dirty = mgr.reconcileMinioJoinPhases(nodes, state)
	if dirty {
		t.Fatal("expected no change — minio not active")
	}

	// pool_updated → started: minio active
	node.Units = []unitStatusRecord{{Name: "globular-minio.service", State: "active"}}
	dirty = mgr.reconcileMinioJoinPhases(nodes, state)
	if !dirty || node.MinioJoinPhase != MinioJoinStarted {
		t.Fatalf("expected started, got %s", node.MinioJoinPhase)
	}

	// started → verified: after 30s
	node.MinioJoinStartedAt = time.Now().Add(-35 * time.Second)
	dirty = mgr.reconcileMinioJoinPhases(nodes, state)
	if !dirty || node.MinioJoinPhase != MinioJoinVerified {
		t.Fatalf("expected verified, got %s", node.MinioJoinPhase)
	}
}

// TestMinioJoin_PoolOrderStable tests that pool order is append-only.
func TestMinioJoin_PoolOrderStable(t *testing.T) {
	mgr := newMinioPoolManager()
	state := &controllerState{MinioCredentials: generateMinioCredentials()}

	node1 := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "node-1", Ips: []string{"10.0.0.1"}},
		Profiles:       []string{"core"},
		Units:          []unitStatusRecord{{Name: "globular-minio.service", State: "active"}},
		BootstrapPhase: BootstrapWorkloadReady,
	}
	node2 := &nodeState{
		NodeID:         "n2",
		Identity:       storedIdentity{Hostname: "node-2", Ips: []string{"10.0.0.2"}},
		Profiles:       []string{"core"},
		Units:          []unitStatusRecord{{Name: "globular-minio.service", State: "inactive"}},
		BootstrapPhase: BootstrapStorageJoining,
	}
	nodes := []*nodeState{node1, node2}

	// First reconcile: both nodes get processed.
	mgr.reconcileMinioJoinPhases(nodes, state)
	mgr.reconcileMinioJoinPhases(nodes, state) // prepared → pool_updated

	if len(state.MinioPoolNodes) != 2 {
		t.Fatalf("expected 2 nodes in pool, got %d", len(state.MinioPoolNodes))
	}
	// Order should be stable (n1 first, n2 second — order of processing).
	if state.MinioPoolNodes[0] != "10.0.0.1" {
		t.Errorf("expected first pool entry 10.0.0.1, got %s", state.MinioPoolNodes[0])
	}
	if state.MinioPoolNodes[1] != "10.0.0.2" {
		t.Errorf("expected second pool entry 10.0.0.2, got %s", state.MinioPoolNodes[1])
	}

	// Adding node3 should append, not reorder.
	node3 := &nodeState{
		NodeID:         "n3",
		Identity:       storedIdentity{Hostname: "node-3", Ips: []string{"10.0.0.3"}},
		Profiles:       []string{"storage"},
		Units:          []unitStatusRecord{{Name: "globular-minio.service", State: "inactive"}},
		BootstrapPhase: BootstrapStorageJoining,
	}
	nodes = append(nodes, node3)
	mgr.reconcileMinioJoinPhases(nodes, state)
	mgr.reconcileMinioJoinPhases(nodes, state)

	if len(state.MinioPoolNodes) != 3 {
		t.Fatalf("expected 3 nodes in pool, got %d", len(state.MinioPoolNodes))
	}
	if state.MinioPoolNodes[2] != "10.0.0.3" {
		t.Errorf("expected third pool entry 10.0.0.3, got %s", state.MinioPoolNodes[2])
	}
}

// TestMinioJoin_Timeout tests that a stuck join times out.
func TestMinioJoin_Timeout(t *testing.T) {
	mgr := newMinioPoolManager()
	state := &controllerState{MinioCredentials: generateMinioCredentials()}

	node := &nodeState{
		NodeID:             "n1",
		Identity:           storedIdentity{Hostname: "slow-minio", Ips: []string{"10.0.0.5"}},
		Profiles:           []string{"core"},
		Units:              []unitStatusRecord{{Name: "globular-minio.service", State: "inactive"}},
		BootstrapPhase:     BootstrapStorageJoining,
		MinioJoinPhase:     MinioJoinPoolUpdated,
		MinioJoinStartedAt: time.Now().Add(-minioJoinTimeout - time.Minute),
	}
	nodes := []*nodeState{node}

	dirty := mgr.reconcileMinioJoinPhases(nodes, state)
	if !dirty || node.MinioJoinPhase != MinioJoinFailed {
		t.Fatalf("expected failed, got %s", node.MinioJoinPhase)
	}
}

// TestMinioJoin_AlreadyInPool tests that a node already in the pool fast-forwards.
func TestMinioJoin_AlreadyInPool(t *testing.T) {
	mgr := newMinioPoolManager()
	state := &controllerState{
		MinioPoolNodes:   []string{"10.0.0.5"},
		MinioCredentials: generateMinioCredentials(),
	}

	node := &nodeState{
		NodeID:         "n1",
		Identity:       storedIdentity{Hostname: "existing", Ips: []string{"10.0.0.5"}},
		Profiles:       []string{"core"},
		Units:          []unitStatusRecord{{Name: "globular-minio.service", State: "active"}},
		BootstrapPhase: BootstrapWorkloadReady,
	}
	nodes := []*nodeState{node}

	dirty := mgr.reconcileMinioJoinPhases(nodes, state)
	if !dirty {
		t.Fatal("expected dirty")
	}
	if node.MinioJoinPhase != MinioJoinVerified {
		t.Fatalf("expected verified (already in pool and running), got %s", node.MinioJoinPhase)
	}
	// Pool should still have exactly 1 entry (no duplicate).
	if len(state.MinioPoolNodes) != 1 {
		t.Fatalf("expected 1 pool entry (no dup), got %d", len(state.MinioPoolNodes))
	}
}

// TestRenderMinioConfig_SingleNode tests standalone mode.
func TestRenderMinioConfig_SingleNode(t *testing.T) {
	creds := &minioCredentials{RootUser: "test-user", RootPassword: "test-pass"}
	ctx := &serviceConfigContext{
		Membership: &clusterMembership{
			Nodes: []memberNode{
				{NodeID: "n1", Hostname: "core-1", IP: "10.0.0.5", Profiles: []string{"core"}},
			},
		},
		CurrentNode:      &memberNode{NodeID: "n1", IP: "10.0.0.5", Profiles: []string{"core"}},
		MinioPoolNodes:   []string{"10.0.0.5"},
		MinioCredentials: creds,
	}

	content, ok := renderMinioConfig(ctx)
	if !ok {
		t.Fatal("expected renderMinioConfig to succeed")
	}
	if !strings.Contains(content, "MINIO_VOLUMES=/var/lib/globular/minio/data") {
		t.Error("single node should use local path")
	}
	if !strings.Contains(content, "MINIO_ROOT_USER=test-user") {
		t.Error("should use generated credentials")
	}
	if strings.Contains(content, "minioadmin") {
		t.Error("should not contain hardcoded minioadmin")
	}
}

// TestRenderMinioConfig_Distributed tests multi-node distributed mode.
func TestRenderMinioConfig_Distributed(t *testing.T) {
	creds := &minioCredentials{RootUser: "cluster-user", RootPassword: "cluster-pass"}
	ctx := &serviceConfigContext{
		Membership: &clusterMembership{
			Nodes: []memberNode{
				{NodeID: "n1", IP: "10.0.0.1", Profiles: []string{"core"}},
				{NodeID: "n2", IP: "10.0.0.2", Profiles: []string{"core"}},
			},
		},
		CurrentNode:      &memberNode{NodeID: "n1", IP: "10.0.0.1", Profiles: []string{"core"}},
		MinioPoolNodes:   []string{"10.0.0.1", "10.0.0.2"},
		MinioCredentials: creds,
	}

	content, ok := renderMinioConfig(ctx)
	if !ok {
		t.Fatal("expected success")
	}
	// Should contain both endpoints in order.
	if !strings.Contains(content, "https://10.0.0.1:9000") {
		t.Error("missing first endpoint")
	}
	if !strings.Contains(content, "https://10.0.0.2:9000") {
		t.Error("missing second endpoint")
	}
	// Order must be 10.0.0.1 before 10.0.0.2.
	idx1 := strings.Index(content, "10.0.0.1")
	idx2 := strings.Index(content, "10.0.0.2")
	if idx1 > idx2 {
		t.Error("pool order not preserved — 10.0.0.1 should come before 10.0.0.2")
	}
	if !strings.Contains(content, "MINIO_ROOT_USER=cluster-user") {
		t.Error("should use cluster credentials")
	}
}

// TestRenderMinioConfig_PoolExpansion tests that adding a third node works.
func TestRenderMinioConfig_PoolExpansion(t *testing.T) {
	ctx := &serviceConfigContext{
		Membership: &clusterMembership{
			Nodes: []memberNode{
				{NodeID: "n1", IP: "10.0.0.1", Profiles: []string{"core"}},
				{NodeID: "n2", IP: "10.0.0.2", Profiles: []string{"core"}},
				{NodeID: "n3", IP: "10.0.0.3", Profiles: []string{"storage"}},
			},
		},
		CurrentNode:      &memberNode{NodeID: "n1", IP: "10.0.0.1", Profiles: []string{"core"}},
		MinioPoolNodes:   []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
		MinioCredentials: &minioCredentials{RootUser: "u", RootPassword: "p"},
	}

	content, ok := renderMinioConfig(ctx)
	if !ok {
		t.Fatal("expected success")
	}
	// All 3 endpoints in order.
	if !strings.Contains(content, "https://10.0.0.3:9000") {
		t.Error("missing expansion node endpoint")
	}
	// Count endpoints.
	count := strings.Count(content, "https://")
	if count != 3 {
		t.Errorf("expected 3 endpoints, got %d", count)
	}
}

// TestGenerateMinioCredentials tests that credentials are random and non-empty.
func TestGenerateMinioCredentials(t *testing.T) {
	c1 := generateMinioCredentials()
	c2 := generateMinioCredentials()

	if c1.RootUser == "" || c1.RootPassword == "" {
		t.Fatal("credentials should not be empty")
	}
	if c1.RootUser == c2.RootUser {
		t.Fatal("two generated credentials should be different")
	}
	if !strings.HasPrefix(c1.RootUser, "gl-") {
		t.Errorf("root user should start with gl-, got %s", c1.RootUser)
	}
}
