package main

import (
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// classifyStuckEtcdJoin tests
// ---------------------------------------------------------------------------

// Test 1: node stuck in etcd_joining after being permanently removed from the
// etcd cluster (WAL records its own removal → etcd crashes on start).
func TestClassifyStuckEtcdJoin_RemovedMember_Detected(t *testing.T) {
	node := makeNode("n1", "nuc", "10.0.0.8", []string{"core"}, []unitStatusRecord{
		{Name: "globular-etcd.service", State: "failed"}, // crashes immediately
	})
	node.BootstrapPhase = BootstrapEtcdJoining
	node.BootstrapStartedAt = time.Now().Add(-etcdStuckJoinThreshold - time.Minute)

	// Only ryzen is a named member — nuc is not.
	namedURLs := map[string]bool{"https://10.0.0.63:2380": true}

	if !classifyStuckEtcdJoin(node, namedURLs, time.Now()) {
		t.Fatal("expected stuck classification: removed member with crashed etcd")
	}
}

// Test 2: a node that just entered etcd_joining (within the threshold) must
// NOT be classified as stuck — it needs time to complete the join.
func TestClassifyStuckEtcdJoin_HealthyJoiningNode_NotClassified(t *testing.T) {
	node := makeNode("n1", "nuc", "10.0.0.8", []string{"core"}, []unitStatusRecord{
		{Name: "globular-etcd.service", State: "inactive"},
	})
	node.BootstrapPhase = BootstrapEtcdJoining
	node.BootstrapStartedAt = time.Now().Add(-1 * time.Minute) // well within threshold

	namedURLs := map[string]bool{}

	if classifyStuckEtcdJoin(node, namedURLs, time.Now()) {
		t.Fatal("healthy joining node within threshold should not be classified as stuck")
	}
}

// Test 3: ghost member scenario — MemberAdd was called by the join script but
// etcd never started, leaving an unnamed member in etcd. The ghost appears in
// existingPeerURLSet but NOT in namedMemberPeerURLSet.
// classifyStuckEtcdJoin uses namedURLs and should still fire.
func TestClassifyStuckEtcdJoin_GhostMember_Detected(t *testing.T) {
	node := makeNode("n1", "nuc", "10.0.0.8", []string{"core"}, []unitStatusRecord{
		{Name: "globular-etcd.service", State: "inactive"},
	})
	node.BootstrapPhase = BootstrapEtcdJoining
	node.BootstrapStartedAt = time.Now().Add(-etcdStuckJoinThreshold - time.Minute)

	// namedURLs is empty — ghost (unnamed) member is NOT here, but IS in
	// existingURLs. The function must use namedURLs so the ghost is detected.
	namedURLs := map[string]bool{}

	if !classifyStuckEtcdJoin(node, namedURLs, time.Now()) {
		t.Fatal("ghost member scenario should be classified as stuck (named URL set excludes ghost)")
	}
}

// Also verify that a node already in etcd as a named member is NOT classified.
func TestClassifyStuckEtcdJoin_AlreadyNamedMember_NotClassified(t *testing.T) {
	node := makeNode("n1", "nuc", "10.0.0.8", []string{"core"}, []unitStatusRecord{
		{Name: "globular-etcd.service", State: "active"},
	})
	node.BootstrapPhase = BootstrapEtcdJoining
	node.BootstrapStartedAt = time.Now().Add(-etcdStuckJoinThreshold - time.Minute)

	// Node IS in the named member list.
	namedURLs := map[string]bool{"https://10.0.0.8:2380": true}

	if classifyStuckEtcdJoin(node, namedURLs, time.Now()) {
		t.Fatal("node already in named member list should not be classified as stuck")
	}
}

// ---------------------------------------------------------------------------
// validateEtcdRejoinPreconditions tests
// ---------------------------------------------------------------------------

// Test 4: the wipe action must be refused when the node is the sole healthy
// etcd member — wiping its data dir would destroy cluster quorum.
func TestEtcdRejoin_RefuseIfSoleMember(t *testing.T) {
	node := makeNode("n1", "nuc", "10.0.0.8", []string{"core"}, nil)
	node.EtcdJoinPhase = EtcdJoinRejoinRequired
	node.LastSeen = time.Now()

	// Only this node in the cluster — no other healthy etcd peers.
	allNodes := []*nodeState{node}

	checks := validateEtcdRejoinPreconditions(node, allNodes)

	if checks.NotSoleMember {
		t.Fatal("expected NotSoleMember=false when node is the sole healthy etcd member")
	}
	if checks.Valid() {
		t.Fatal("expected preconditions to be invalid for sole member")
	}
	if checks.Error == nil {
		t.Fatal("expected a descriptive error")
	}
	if !strings.Contains(checks.Error.Error(), "quorum") {
		t.Fatalf("expected 'quorum' in error, got: %v", checks.Error)
	}
}

// ---------------------------------------------------------------------------
// markEtcdRejoinInProgress / workflow initiation tests
// ---------------------------------------------------------------------------

// Test 5: markEtcdRejoinInProgress transitions the node to rejoin_in_progress
// and clears the error, representing a backup-and-wipe workflow being dispatched.
// The state transition confirms the controller will not initiate a second wipe
// (it's now in rejoin_in_progress, not rejoin_required).
func TestEtcdRejoin_TransitionsToInProgress(t *testing.T) {
	ryzen := makeNode("n0", "ryzen", "10.0.0.63", []string{"core"}, []unitStatusRecord{
		{Name: "globular-etcd.service", State: "active"},
	})
	ryzen.EtcdJoinPhase = EtcdJoinVerified
	ryzen.LastSeen = time.Now()

	nuc := makeNode("n1", "nuc", "10.0.0.8", []string{"core"}, nil)
	nuc.EtcdJoinPhase = EtcdJoinRejoinRequired
	nuc.EtcdJoinError = "stuck in etcd_joining for 12m0s"
	nuc.LastSeen = time.Now()

	allNodes := []*nodeState{ryzen, nuc}

	if err := markEtcdRejoinInProgress(nuc, allNodes); err != nil {
		t.Fatalf("expected success (ryzen is a healthy peer), got: %v", err)
	}
	if nuc.EtcdJoinPhase != EtcdJoinRejoinInProgress {
		t.Fatalf("expected rejoin_in_progress, got %s", nuc.EtcdJoinPhase)
	}
	if nuc.EtcdJoinError != "" {
		t.Fatalf("expected error cleared on transition to in_progress, got %q", nuc.EtcdJoinError)
	}
}

// Verify markEtcdRejoinInProgress rejects a node not in rejoin_required state.
func TestEtcdRejoin_RefuseIfNotInRejoinState(t *testing.T) {
	nuc := makeNode("n1", "nuc", "10.0.0.8", []string{"core"}, nil)
	nuc.EtcdJoinPhase = EtcdJoinNone // wrong state
	nuc.LastSeen = time.Now()
	allNodes := []*nodeState{nuc}

	if err := markEtcdRejoinInProgress(nuc, allNodes); err == nil {
		t.Fatal("expected error for node not in rejoin_required state")
	}
}

// ---------------------------------------------------------------------------
// Bootstrap phase integration test
// ---------------------------------------------------------------------------

// Test 6: after the operator runs a repair (etcd rejoins successfully),
// the bootstrap phase machine must advance from etcd_joining to etcd_ready.
// Also verifies that while repair is pending (rejoin_required), the bootstrap
// timeout is suppressed and the timer is reset each cycle.
func TestBootstrap_EtcdRejoin_AdvancesAfterRecovery(t *testing.T) {
	emitter := &mockEmitter{}
	node := &nodeState{
		NodeID:   "n1",
		Identity: storedIdentity{Hostname: "nuc", Ips: []string{"10.0.0.8"}},
		Profiles: []string{"core"},
		Units: []unitStatusRecord{
			{Name: "globular-etcd.service", State: "inactive"},
		},
		BootstrapPhase: BootstrapEtcdJoining,
		// Stale timestamp — would normally trigger bootstrapPhaseTimeout.
		BootstrapStartedAt: time.Now().Add(-etcdStuckJoinThreshold - 2*time.Minute),
		EtcdJoinPhase:      EtcdJoinRejoinRequired,
		EtcdJoinError:      "stuck in etcd_joining for 12m0s: run repair",
	}
	nodes := []*nodeState{node}

	// First cycle with rejoin_required: bootstrap must NOT fail despite stale timer.
	dirty := reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty {
		t.Fatal("expected dirty=true: timer should be reset to suppress timeout")
	}
	if node.BootstrapPhase != BootstrapEtcdJoining {
		t.Fatalf("expected bootstrap to stay in etcd_joining (not timeout-fail), got %s", node.BootstrapPhase)
	}
	if time.Since(node.BootstrapStartedAt) > time.Second {
		t.Fatal("expected BootstrapStartedAt to be reset to ~now to suppress future timeouts")
	}
	if node.BootstrapError != node.EtcdJoinError {
		t.Fatalf("expected BootstrapError to mirror EtcdJoinError, got %q", node.BootstrapError)
	}

	// Simulate the operator completing the repair: etcd rejoins as a named member.
	node.EtcdJoinPhase = EtcdJoinVerified
	node.EtcdJoinError = ""
	node.Units = []unitStatusRecord{
		{Name: "globular-etcd.service", State: "active"},
	}

	dirty = reconcileBootstrapPhases(nodes, nil, emitter)
	if !dirty {
		t.Fatal("expected dirty=true after EtcdJoinVerified")
	}
	if node.BootstrapPhase != BootstrapEtcdReady {
		t.Fatalf("expected bootstrap to advance to etcd_ready after recovery, got %s", node.BootstrapPhase)
	}
}
