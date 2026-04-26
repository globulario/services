package main

import (
	"testing"

	"github.com/globulario/services/golang/config"
)

// ── nodeIPInPool ──────────────────────────────────────────────────────────────

func TestNodeIPInPool_MemberFirst(t *testing.T) {
	state := &config.ObjectStoreDesiredState{Nodes: []string{"10.0.0.1", "10.0.0.2"}}
	if !nodeIPInPool("10.0.0.1", state) {
		t.Error("expected true: first node is in pool")
	}
}

func TestNodeIPInPool_MemberSecond(t *testing.T) {
	state := &config.ObjectStoreDesiredState{Nodes: []string{"10.0.0.1", "10.0.0.2"}}
	if !nodeIPInPool("10.0.0.2", state) {
		t.Error("expected true: second node is in pool")
	}
}

func TestNodeIPInPool_NonMember(t *testing.T) {
	state := &config.ObjectStoreDesiredState{Nodes: []string{"10.0.0.1"}}
	if nodeIPInPool("10.0.0.8", state) {
		t.Error("expected false: 10.0.0.8 is not in pool")
	}
}

func TestNodeIPInPool_EmptyPool(t *testing.T) {
	state := &config.ObjectStoreDesiredState{Nodes: nil}
	if nodeIPInPool("10.0.0.1", state) {
		t.Error("expected false: pool is empty")
	}
}

func TestNodeIPInPool_NilState(t *testing.T) {
	if nodeIPInPool("10.0.0.1", nil) {
		t.Error("expected false: nil state")
	}
}

func TestNodeIPInPool_EmptyIP(t *testing.T) {
	state := &config.ObjectStoreDesiredState{Nodes: []string{"10.0.0.1"}}
	if nodeIPInPool("", state) {
		t.Error("expected false: empty nodeIP")
	}
}

func TestNodeIPInPool_ThreeNodePool(t *testing.T) {
	state := &config.ObjectStoreDesiredState{
		Nodes: []string{"10.0.0.1", "10.0.0.2", "10.0.0.3"},
	}
	for _, ip := range state.Nodes {
		if !nodeIPInPool(ip, state) {
			t.Errorf("expected true for pool member %s", ip)
		}
	}
	if nodeIPInPool("10.0.0.99", state) {
		t.Error("expected false for non-member 10.0.0.99")
	}
}

// ── Day-1 topology gate contract ─────────────────────────────────────────────
//
// These tests verify the gate predicate that reconcileMinioSystemdConfig uses
// to decide whether to render MinIO config and allow service startup.
// The actual systemctl calls in enforceMinioHeld cannot be unit-tested here,
// but the admission decision — which drives the whole gate — is pure and testable.

// TestTopologyGate_Day1NodeNotAdmitted verifies that a Day-1 storage-profile node
// whose IP is not yet in ObjectStoreDesiredState.Nodes is correctly identified
// as a non-member that should be held.
func TestTopologyGate_Day1NodeNotAdmitted(t *testing.T) {
	// Day-0 bootstrap node only: ryzen is the founding member.
	state := &config.ObjectStoreDesiredState{
		Mode:       config.ObjectStoreModeStandalone,
		Generation: 1,
		Nodes:      []string{"10.0.0.63"}, // ryzen only
	}
	nucIP := "10.0.0.8"
	if nodeIPInPool(nucIP, state) {
		t.Errorf("Day-1 nuc (%s) must not be admitted into the pool before apply-topology", nucIP)
	}
}

// TestTopologyGate_BootstrapNodeAllowed verifies that the bootstrap (Day-0) node
// itself is correctly identified as a pool member and is not held.
func TestTopologyGate_BootstrapNodeAllowed(t *testing.T) {
	state := &config.ObjectStoreDesiredState{
		Mode:       config.ObjectStoreModeStandalone,
		Generation: 1,
		Nodes:      []string{"10.0.0.63"},
	}
	if !nodeIPInPool("10.0.0.63", state) {
		t.Error("bootstrap node must be in pool and allowed to run MinIO")
	}
}

// TestTopologyGate_AfterApplyTopology verifies that after apply-topology adds the
// Day-1 node to the pool, it is correctly admitted.
func TestTopologyGate_AfterApplyTopology(t *testing.T) {
	// Simulate state after a successful apply-topology that expanded the pool.
	state := &config.ObjectStoreDesiredState{
		Mode:       config.ObjectStoreModeDistributed,
		Generation: 2,
		Nodes:      []string{"10.0.0.63", "10.0.0.8"}, // both admitted
	}
	if !nodeIPInPool("10.0.0.8", state) {
		t.Error("nuc must be allowed to run MinIO after apply-topology adds it to the pool")
	}
	if !nodeIPInPool("10.0.0.63", state) {
		t.Error("ryzen must remain allowed after pool expansion")
	}
}

// TestTopologyGate_MissingDesiredState verifies that a nil desired state
// results in the node being treated as non-member (gate blocks).
func TestTopologyGate_MissingDesiredState(t *testing.T) {
	// Pre-pool-formation: no desired state written yet.
	// reconcileMinioSystemdConfig returns early before the gate when state==nil,
	// but for completeness: if state were nil and we called nodeIPInPool, it must
	// return false (fail closed).
	if nodeIPInPool("10.0.0.63", nil) {
		t.Error("nil desired state must not admit any node (fail closed)")
	}
}

// TestTopologyGate_MultipleDay1NodesHeld verifies that multiple simultaneously
// joining Day-1 nodes are all held when only the bootstrap node is in the pool.
func TestTopologyGate_MultipleDay1NodesHeld(t *testing.T) {
	state := &config.ObjectStoreDesiredState{
		Mode:       config.ObjectStoreModeStandalone,
		Generation: 1,
		Nodes:      []string{"10.0.0.63"}, // bootstrap only
	}
	day1Nodes := []string{"10.0.0.8", "10.0.0.20"}
	for _, ip := range day1Nodes {
		if nodeIPInPool(ip, state) {
			t.Errorf("Day-1 node %s must not be admitted before apply-topology", ip)
		}
	}
}
