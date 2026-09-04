package main

import (
	"context"
	"testing"

	"go.etcd.io/etcd/api/v3/etcdserverpb"
)

// The stale-member prune must never conclude "remove" from the controller's node
// records alone.
//
// WHAT HAPPENED. 2026-09-02, node-clone-identity-collision on the 5-node sim: a
// container was started carrying a copy of node-4's identity. The impostor took
// over node-4's node record, so the record's IP became the impostor's
// (10.10.0.15). The live etcd member for https://10.10.0.14:2380 — the REAL
// node-4, healthy, voting, untouched by the scenario — then matched no desired
// peer URL and no desired hostname, and this sweep removed it:
//
//	22:42:18  removeStaleNodesLocked: removing duplicate/stale node 35ac3821 (same host as 6400b443)
//	22:42:24  etcd member-remove: removed stale member node-4 (id=2263181996067307296, peer=[https://10.10.0.14:2380])
//	22:47:21  node-4 etcd: "rejected Raft message to mismatch member" → unit exited
//
// The ring went from 5 members / 5 healthy endpoints to 4 / 3, and node-4 never
// returned: the later re-add minted a NEW member id while node-4's data dir
// still held the old one.
//
// So classifyStaleMemberAction returns staleMemberCandidate, never "remove":
// removal additionally requires evidence from the member itself
// (memberIsResponding). That is the fail-closed shape
// etcd.auto_rejoin_leader_guard_fails_open was repaired into, and it honours
// delete_requires_explicit_intent_marker — a reconciler acting on inference has
// no explicit intent to delete a voter.

func testMember(name string, peerURLs ...string) *etcdserverpb.Member {
	return &etcdserverpb.Member{Name: name, PeerURLs: peerURLs}
}

func TestClassifyStaleMemberAction_UnclaimedLiveMemberIsOnlyACandidate(t *testing.T) {
	// The exact shape of the 2026-09-02 failure: every desired peer URL belongs
	// to some other node, and the impostor (…15) now holds node-4's record, so
	// nothing claims node-4's real peer URL or its hostname.
	desiredPeerURLs := map[string]bool{
		"https://10.10.0.11:2380": true,
		"https://10.10.0.12:2380": true,
		"https://10.10.0.13:2380": true,
		"https://10.10.0.15:2380": true,
	}

	action, _ := classifyStaleMemberAction(
		testMember("node-4", "https://10.10.0.14:2380"),
		desiredPeerURLs,
		map[string]bool{},
		map[string]string{},
		map[string]bool{},
	)
	if action != staleMemberCandidate {
		t.Fatalf("an unclaimed member must classify as a candidate, got %v", action)
	}
	// A candidate is not a decision. If this type ever gains a verdict that
	// authorises removal from records alone, the 2026-09-02 eviction is back.
	if staleMemberCandidate == staleMemberSkip || staleMemberCandidate == staleMemberUpdatePeerURL {
		t.Fatal("candidate must stay a distinct, non-authorising verdict")
	}
}

func TestClassifyStaleMemberAction_ProtectedCases(t *testing.T) {
	cases := []struct {
		name             string
		member           *etcdserverpb.Member
		desiredPeerURLs  map[string]bool
		desiredHostnames map[string]bool
		byHostname       map[string]string
		joinInProgress   map[string]bool
		want             staleMemberAction
	}{
		{
			name:   "unstarted member is left alone",
			member: testMember("", "https://10.10.0.14:2380"),
			want:   staleMemberSkip,
		},
		{
			name:           "day-1 join in flight is left alone",
			member:         testMember("node-6", "https://10.10.0.16:2380"),
			joinInProgress: map[string]bool{"node-6": true},
			want:           staleMemberSkip,
		},
		{
			name:            "member at a desired peer URL is left alone",
			member:          testMember("node-4", "https://10.10.0.14:2380"),
			desiredPeerURLs: map[string]bool{"https://10.10.0.14:2380": true},
			want:            staleMemberSkip,
		},
		{
			name:             "hostname fallback covers a node whose IP is momentarily unknown",
			member:           testMember("node-4", "https://10.10.0.14:2380"),
			desiredHostnames: map[string]bool{"node-4": true},
			want:             staleMemberSkip,
		},
		{
			name:       "same hostname at a different peer URL is updated, not removed",
			member:     testMember("node-4", "https://10.10.0.14:2380"),
			byHostname: map[string]string{"node-4": "https://10.10.0.44:2380"},
			want:       staleMemberUpdatePeerURL,
		},
		{
			name:       "same hostname already at the expected peer URL is left alone",
			member:     testMember("node-4", "https://10.10.0.14:2380"),
			byHostname: map[string]string{"node-4": "https://10.10.0.14:2380"},
			want:       staleMemberSkip,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			peerURLs := tc.desiredPeerURLs
			if peerURLs == nil {
				peerURLs = map[string]bool{}
			}
			hostnames := tc.desiredHostnames
			if hostnames == nil {
				hostnames = map[string]bool{}
			}
			byHost := tc.byHostname
			if byHost == nil {
				byHost = map[string]string{}
			}
			joining := tc.joinInProgress
			if joining == nil {
				joining = map[string]bool{}
			}
			got, expected := classifyStaleMemberAction(tc.member, peerURLs, hostnames, byHost, joining)
			if got != tc.want {
				t.Fatalf("got %v, want %v", got, tc.want)
			}
			if tc.want == staleMemberUpdatePeerURL && expected == "" {
				t.Fatal("an update verdict must carry the peer URL to update to")
			}
		})
	}
}

// memberIsResponding must fail closed: with nothing to ask, the answer is "not
// responding", so a genuine ghost stays prunable and the guard cannot wedge
// cleanup by defaulting to "alive".
func TestMemberIsResponding_FailsClosedWithoutEvidence(t *testing.T) {
	ctx := context.Background()
	var nilManager *etcdMemberManager
	if nilManager.memberIsResponding(ctx, testMember("node-4", "https://10.10.0.14:2380")) {
		t.Fatal("a nil manager cannot observe a member responding")
	}

	noClient := &etcdMemberManager{}
	if noClient.memberIsResponding(ctx, testMember("node-4", "https://10.10.0.14:2380")) {
		t.Fatal("a manager with no etcd client cannot observe a member responding")
	}
}
