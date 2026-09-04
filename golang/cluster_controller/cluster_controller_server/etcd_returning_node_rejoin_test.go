package main

import (
	"testing"
)

// A node that finished its join, lost its etcd ring entry while it was down, and
// came back without one must be recognised and repaired.
//
// WHAT WAS MISSING. The repair chain behind EtcdJoinRejoinRequired was fully
// implemented and wired — reconcileEtcdAutoRejoin calls MemberAdd behind the
// never-rejoin-the-leader guard and validateEtcdRejoinPreconditions, then
// dispatchEtcdWipeAndRejoin runs the node agent's wipe-etcd-and-rejoin workflow.
// Nothing ever entered it for this shape of node:
//
//   - classifyStuckEtcdJoin requires BootstrapPhase == etcd_joining, so it only
//     sees a node that NEVER finished joining;
//   - the "member disappeared" cooldown lived inside the EtcdJoinVerified branch,
//     so it only saw a node whose phase was still verified.
//
// A node loses the verified phase whenever its record is re-created —
// removeStaleNodesLocked deletes the record of a node whose identity another
// claimant took over, and the next heartbeat re-creates it at EtcdJoinNone — and
// it is workload_ready, not etcd_joining. It therefore matched neither detector.
//
// Measured on the 5-node simulation, 1.2.357, 2026-09-03, after
// authority/node-clone-identity-collision: node-5 sat at etcd_join_phase="",
// bootstrap_phase=workload_ready, globular-etcd inactive, absent from the ring,
// while the controller logged "renderEtcdConfig: … is not an etcd member yet —
// skipping etcd.yaml render until MemberAdd puts it in the ring" every ~15s
// indefinitely. Ring stayed at 4 of 5.

func returningNode(phase EtcdJoinPhase, bootstrap BootstrapPhase, ips ...string) *nodeState {
	if len(ips) == 0 {
		ips = []string{"10.10.0.15"}
	}
	return &nodeState{
		NodeID:         "node-5-id",
		EtcdJoinPhase:  phase,
		BootstrapPhase: bootstrap,
		Identity:       storedIdentity{Hostname: "node-5", Ips: ips},
	}
}

// etcdActive expresses "this node's etcd is running" the way nodeHasEtcdRunning
// reads it — through the reported unit list, not a synthetic flag.
func etcdActive() []unitStatusRecord {
	return []unitStatusRecord{{Name: "globular-etcd.service", State: "active"}}
}

// ringWithout builds a member set that does NOT contain the node's peer URL.
func ringWithout() map[string]bool {
	return map[string]bool{
		"https://10.10.0.11:2380": true,
		"https://10.10.0.12:2380": true,
		"https://10.10.0.13:2380": true,
		"https://10.10.0.14:2380": true,
	}
}

func TestNodeReturnedWithoutMembership_RecognisesTheReturningNode(t *testing.T) {
	node := returningNode(EtcdJoinNone, BootstrapWorkloadReady)
	if !nodeReturnedWithoutMembership(node, ringWithout()) {
		t.Fatal("a bootstrapped node absent from the ring with etcd down must be recognised; " +
			"without this it matches no detector and never reaches rejoin_required")
	}
}

func TestNodeReturnedWithoutMembership_Guards(t *testing.T) {
	cases := []struct {
		name string
		node *nodeState
		ring map[string]bool
		want bool
	}{
		{
			name: "nil node",
			node: nil, ring: ringWithout(), want: false,
		},
		{
			// A node still joining belongs to classifyStuckEtcdJoin, which applies
			// its own longer threshold. Claiming it here would race the join script.
			name: "still mid-join is left to the stuck-join classifier",
			node: returningNode(EtcdJoinNone, BootstrapEtcdJoining),
			ring: ringWithout(), want: false,
		},
		{
			name: "already a ring member",
			node: returningNode(EtcdJoinNone, BootstrapWorkloadReady),
			ring: map[string]bool{"https://10.10.0.15:2380": true}, want: false,
		},
		{
			// Phases with their own handling must not be hijacked.
			name: "verified is handled by its own branch",
			node: returningNode(EtcdJoinVerified, BootstrapWorkloadReady),
			ring: ringWithout(), want: false,
		},
		{
			name: "already rejoin_required",
			node: returningNode(EtcdJoinRejoinRequired, BootstrapWorkloadReady),
			ring: ringWithout(), want: false,
		},
		{
			name: "repair already in progress",
			node: returningNode(EtcdJoinRejoinInProgress, BootstrapWorkloadReady),
			ring: ringWithout(), want: false,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := nodeReturnedWithoutMembership(tc.node, tc.ring); got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
		})
	}
}

// The cooldown must hold: a transient etcd restart or a brief partition must not
// trigger a destructive repair.
func TestConfirmVanishedEtcdMember_CooldownHoldsForTwoCycles(t *testing.T) {
	node := returningNode(EtcdJoinNone, BootstrapWorkloadReady)
	peers := []*nodeState{{NodeID: "peer-1", EtcdJoinPhase: EtcdJoinVerified, Units: etcdActive()}}

	for cycle := 1; cycle <= 2; cycle++ {
		if next := confirmVanishedEtcdMember(node, peers); next != "" {
			t.Fatalf("cycle %d: expected no verdict during cooldown, got %q", cycle, next)
		}
	}
	if next := confirmVanishedEtcdMember(node, peers); next != EtcdJoinRejoinRequired {
		t.Fatalf("third consecutive cycle must confirm rejoin_required, got %q", next)
	}
	if node.EtcdMissingCycles != 0 {
		t.Fatalf("the counter must reset once a verdict is reached, got %d", node.EtcdMissingCycles)
	}
}

// The quorum gate must hold: with no other healthy member, the node must NOT be
// routed to a data-directory wipe. Wiping the last surviving member is the
// unrecoverable case — same lesson as etcd.auto_rejoin_leader_guard_fails_open.
func TestConfirmVanishedEtcdMember_NoHealthyPeersDoesNotTriggerAWipe(t *testing.T) {
	node := returningNode(EtcdJoinNone, BootstrapWorkloadReady)
	lonely := []*nodeState{{NodeID: "peer-1", EtcdJoinPhase: EtcdJoinVerified}} // etcd not active

	var next EtcdJoinPhase
	for cycle := 1; cycle <= 3; cycle++ {
		next = confirmVanishedEtcdMember(node, lonely)
	}
	if next == EtcdJoinRejoinRequired {
		t.Fatal("with no other healthy etcd member the node must not be routed to a wipe; " +
			"it must fall back to the ordinary join flow")
	}
	if next != EtcdJoinNone {
		t.Fatalf("expected a reset to the ordinary join flow, got %q", next)
	}
}

// etcdMemberVanished is the single definition both detectors share, so they
// cannot drift apart.
func TestEtcdMemberVanished(t *testing.T) {
	absent := returningNode(EtcdJoinVerified, BootstrapWorkloadReady)
	if !etcdMemberVanished(absent, ringWithout()) {
		t.Fatal("absent from the ring with etcd down is the vanished shape")
	}
	running := returningNode(EtcdJoinVerified, BootstrapWorkloadReady)
	running.Units = etcdActive()
	if etcdMemberVanished(running, ringWithout()) {
		t.Fatal("a node whose etcd is running has not vanished")
	}
	present := returningNode(EtcdJoinVerified, BootstrapWorkloadReady)
	if etcdMemberVanished(present, map[string]bool{"https://10.10.0.15:2380": true}) {
		t.Fatal("a node present in the ring has not vanished")
	}
	if etcdMemberVanished(nil, ringWithout()) {
		t.Fatal("a nil node cannot have vanished")
	}
}
