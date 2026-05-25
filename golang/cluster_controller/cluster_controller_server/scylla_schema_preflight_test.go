package main

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// fakeQuerier is a test double for scyllaPreflightQuerier.
type fakeQuerier struct {
	localHostID        string
	localSchemaVersion string
	localErr           error
	peers              []peerSchemaRow
	peersErr           error
	members            []group0Member
	membersErr         error
	raftStateMembers   []group0Member
	raftStateErr       error
}

func (f *fakeQuerier) queryLocalInfo(_ context.Context) (localInfo, error) {
	if f.localErr != nil {
		return localInfo{}, f.localErr
	}
	return localInfo{HostID: f.localHostID, SchemaVersion: f.localSchemaVersion}, nil
}

func (f *fakeQuerier) queryPeerSchemas(_ context.Context) ([]peerSchemaRow, error) {
	return f.peers, f.peersErr
}

func (f *fakeQuerier) queryGroup0Members(_ context.Context) ([]group0Member, error) {
	return f.members, f.membersErr
}

func (f *fakeQuerier) queryRaftStateFallback(_ context.Context) ([]group0Member, error) {
	if f.raftStateErr != nil {
		return nil, f.raftStateErr
	}
	return f.raftStateMembers, nil
}

// healthyQuerier returns a fully-healthy fakeQuerier: local node + two peers,
// all agreeing on schema, three live Group 0 voters — all in gossip.
func healthyQuerier() *fakeQuerier {
	return &fakeQuerier{
		localHostID:        "uuid-local",
		localSchemaVersion: "aaaa-1111",
		peers: []peerSchemaRow{
			{Peer: "10.0.0.8", SchemaVersion: "aaaa-1111", HostID: "uuid-peer-a"},
			{Peer: "10.0.0.20", SchemaVersion: "aaaa-1111", HostID: "uuid-peer-b"},
		},
		members: []group0Member{
			{ServerID: "uuid-local", CanVote: boolPtr(true)},
			{ServerID: "uuid-peer-a", CanVote: boolPtr(true)},
			{ServerID: "uuid-peer-b", CanVote: boolPtr(true)},
		},
	}
}

// ── Original Phase D tests (updated for new interface) ───────────────────────

func TestCanScyllaDDLProceed_HealthyCluster(t *testing.T) {
	r := canScyllaDDLProceedWith(context.Background(), healthyQuerier())
	if !r.OK {
		t.Fatalf("healthy cluster must produce OK, got reason=%q finding=%q",
			r.Reason, Group0FindingText(r))
	}
	if r.Reason != DDLPreflightOK {
		t.Fatalf("expected reason=%q, got %q", DDLPreflightOK, r.Reason)
	}
	if r.Group0 == nil {
		t.Fatal("healthy result must carry a Group0View")
	}
}

func TestCanScyllaDDLProceed_ScyllaUnavailable_LocalQueryFails(t *testing.T) {
	q := healthyQuerier()
	q.localErr = errors.New("connection refused")
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("query error on system.local must block DDL")
	}
	if r.Reason != DDLPreflightScyllaUnavailable {
		t.Fatalf("expected %q, got %q", DDLPreflightScyllaUnavailable, r.Reason)
	}
}

func TestCanScyllaDDLProceed_ScyllaUnavailable_EmptySchemaVersion(t *testing.T) {
	q := healthyQuerier()
	q.localSchemaVersion = ""
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("empty schema_version in system.local must block DDL")
	}
	if r.Reason != DDLPreflightScyllaUnavailable {
		t.Fatalf("expected %q, got %q", DDLPreflightScyllaUnavailable, r.Reason)
	}
}

func TestCanScyllaDDLProceed_SchemaAgreementUnavailable_PeerQueryFails(t *testing.T) {
	q := healthyQuerier()
	q.peersErr = errors.New("timeout")
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("peer query failure must block DDL")
	}
	if r.Reason != DDLPreflightSchemaAgreementUnavailable {
		t.Fatalf("expected %q, got %q", DDLPreflightSchemaAgreementUnavailable, r.Reason)
	}
}

func TestCanScyllaDDLProceed_SchemaAgreementUnavailable_PeerDisagrees(t *testing.T) {
	q := healthyQuerier()
	q.peers = []peerSchemaRow{
		{Peer: "10.0.0.8", SchemaVersion: "aaaa-1111", HostID: "uuid-peer-a"},
		{Peer: "10.0.0.20", SchemaVersion: "bbbb-2222", HostID: "uuid-peer-b"}, // stale
	}
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("schema disagreement must block DDL")
	}
	if r.Reason != DDLPreflightSchemaAgreementUnavailable {
		t.Fatalf("expected %q, got %q", DDLPreflightSchemaAgreementUnavailable, r.Reason)
	}
	if r.Details["first_disagreeing_peer"] != "10.0.0.20" {
		t.Errorf("expected disagreeing peer=10.0.0.20, got %q", r.Details["first_disagreeing_peer"])
	}
}

// TestCanScyllaDDLProceed_Unknown_Group0TableMissing covers the fail-closed
// rule: when both raft_group0_members and raft_state are absent, DDL is
// blocked. The raft_state fallback (ScyllaDB 2025.x+) is tried first; only
// when it also returns ErrGroup0TableMissing is DDLPreflightUnknown returned.
func TestCanScyllaDDLProceed_Unknown_Group0TableMissing(t *testing.T) {
	q := healthyQuerier()
	q.membersErr = ErrGroup0TableMissing
	q.raftStateErr = ErrGroup0TableMissing // both tables absent — full unknown
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("missing Group 0 table must block DDL (fail closed)")
	}
	if r.Reason != DDLPreflightUnknown {
		t.Fatalf("expected %q, got %q", DDLPreflightUnknown, r.Reason)
	}
	// Finding text must communicate the "cannot be proven" message.
	ft := Group0FindingText(r)
	if !strings.Contains(ft, "cannot be proven") {
		t.Errorf("finding text must say 'cannot be proven', got: %s", ft)
	}
}

// TestCanScyllaDDLProceed_Unknown_Group0QueryError covers an unexpected query
// failure — also blocked, fail closed.
func TestCanScyllaDDLProceed_Unknown_Group0QueryError(t *testing.T) {
	q := healthyQuerier()
	q.membersErr = errors.New("read timeout")
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("Group 0 query error must block DDL (fail closed)")
	}
	if r.Reason != DDLPreflightUnknown {
		t.Fatalf("expected %q, got %q", DDLPreflightUnknown, r.Reason)
	}
}

// TestCanScyllaDDLProceed_Group0Unavailable_EmptyMembers handles the edge case
// where the table exists but is empty — abnormal and treated as unavailable.
func TestCanScyllaDDLProceed_Group0Unavailable_EmptyMembers(t *testing.T) {
	q := healthyQuerier()
	q.members = []group0Member{}
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("empty Group 0 members must block DDL")
	}
	if r.Reason != DDLPreflightGroup0Unavailable {
		t.Fatalf("expected %q, got %q", DDLPreflightGroup0Unavailable, r.Reason)
	}
}

// TestCanScyllaDDLProceed_Group0StaleVoter_CanVoteFalse verifies that a voter
// present in gossip but with can_vote=false blocks DDL.
func TestCanScyllaDDLProceed_Group0StaleVoter_CanVoteFalse(t *testing.T) {
	q := healthyQuerier()
	q.members = []group0Member{
		{ServerID: "uuid-local", CanVote: boolPtr(true)},
		{ServerID: "uuid-peer-a", CanVote: boolPtr(false)}, // stale via can_vote
		{ServerID: "uuid-peer-b", CanVote: boolPtr(true)},
	}
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("can_vote=false voter must block DDL")
	}
	if r.Reason != DDLPreflightGroup0StaleVoter {
		t.Fatalf("expected %q, got %q", DDLPreflightGroup0StaleVoter, r.Reason)
	}
	if r.Details["first_stale_voter"] != "uuid-peer-a" {
		t.Errorf("expected first_stale_voter=uuid-peer-a, got %q", r.Details["first_stale_voter"])
	}
	if r.Details["stale_voter_count"] != "1" {
		t.Errorf("expected stale_voter_count=1, got %q", r.Details["stale_voter_count"])
	}
}

// ── Phase D.1 tests — Group 0 gossip cross-referencing ───────────────────────

// TestCollectGroup0View_AllVotersInGossip verifies that when every voter's
// server_id appears in system.local or system.peers, the view has no stale
// voters and the IPs are resolved.
func TestCollectGroup0View_AllVotersInGossip(t *testing.T) {
	members := []group0Member{
		{ServerID: "uuid-local", CanVote: boolPtr(true)},
		{ServerID: "uuid-peer-a", CanVote: boolPtr(true)},
		{ServerID: "uuid-peer-b", CanVote: boolPtr(true)},
	}
	peers := []peerSchemaRow{
		{Peer: "10.0.0.8", HostID: "uuid-peer-a"},
		{Peer: "10.0.0.20", HostID: "uuid-peer-b"},
	}
	view := collectGroup0View(members, "uuid-local", peers)
	if view.StaleVoters != 0 {
		t.Errorf("expected 0 stale voters, got %d", view.StaleVoters)
	}
	if view.TotalVoters != 3 {
		t.Errorf("expected 3 total voters, got %d", view.TotalVoters)
	}
	// Peer IPs must be resolved.
	for _, v := range view.Voters {
		if v.ServerID == "uuid-peer-a" && v.PeerAddr != "10.0.0.8" {
			t.Errorf("uuid-peer-a: expected PeerAddr=10.0.0.8, got %q", v.PeerAddr)
		}
		if v.ServerID == "uuid-peer-b" && v.PeerAddr != "10.0.0.20" {
			t.Errorf("uuid-peer-b: expected PeerAddr=10.0.0.20, got %q", v.PeerAddr)
		}
		if v.ServerID == "uuid-local" && v.PeerAddr != "" {
			t.Errorf("local voter must have empty PeerAddr, got %q", v.PeerAddr)
		}
	}
}

// TestCollectGroup0View_VoterNotInGossip_Stale verifies that a voter whose
// server_id does not appear in system.local or system.peers is marked stale
// with reason "not_in_gossip". This is the primary dead-voter signal.
func TestCollectGroup0View_VoterNotInGossip_Stale(t *testing.T) {
	members := []group0Member{
		{ServerID: "uuid-local", CanVote: boolPtr(true)},
		{ServerID: "uuid-peer-a", CanVote: boolPtr(true)},
		{ServerID: "uuid-dead-nuc", CanVote: boolPtr(true)}, // wiped, not in gossip
	}
	peers := []peerSchemaRow{
		{Peer: "10.0.0.8", HostID: "uuid-peer-a"},
		// uuid-dead-nuc is absent from peers — it was wiped/removed.
	}
	view := collectGroup0View(members, "uuid-local", peers)
	if view.StaleVoters != 1 {
		t.Errorf("expected 1 stale voter, got %d", view.StaleVoters)
	}
	for _, v := range view.Voters {
		if v.ServerID == "uuid-dead-nuc" {
			if !v.IsStale() {
				t.Error("uuid-dead-nuc must be marked stale")
			}
			if v.StaleReason != "not_in_gossip" {
				t.Errorf("expected StaleReason=not_in_gossip, got %q", v.StaleReason)
			}
			if v.InGossip {
				t.Error("uuid-dead-nuc must not be InGossip")
			}
		}
	}
}

// TestCanScyllaDDLProceed_StaleVoterByGossip_Blocked verifies the end-to-end
// path: a voter absent from gossip → Group0StaleVoter → DDL blocked.
func TestCanScyllaDDLProceed_StaleVoterByGossip_Blocked(t *testing.T) {
	q := healthyQuerier()
	// Add a voter whose server_id is not in local or peers.
	q.members = append(q.members, group0Member{
		ServerID: "uuid-dead-nuc",
		CanVote:  boolPtr(true), // Scylla may still report can_vote=true for a wiped node
	})
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("voter absent from gossip must block DDL")
	}
	if r.Reason != DDLPreflightGroup0StaleVoter {
		t.Fatalf("expected %q, got %q; finding: %s", DDLPreflightGroup0StaleVoter, r.Reason, Group0FindingText(r))
	}
	if r.Group0 == nil {
		t.Fatal("Group0View must be populated when stale voter is detected")
	}
	if r.Group0.StaleVoters != 1 {
		t.Errorf("expected 1 stale voter in Group0View, got %d", r.Group0.StaleVoters)
	}
}

// TestCanScyllaDDLProceed_FindingTextIncludesVoterDetails verifies that the
// finding text for a stale voter includes the server_id and IP when resolved,
// and names the source table.
func TestCanScyllaDDLProceed_FindingTextIncludesVoterDetails(t *testing.T) {
	q := healthyQuerier()
	// Stale voter whose IP is resolvable via peers.
	q.peers = []peerSchemaRow{
		{Peer: "10.0.0.8", SchemaVersion: "aaaa-1111", HostID: "uuid-peer-a"},
		{Peer: "10.0.0.20", SchemaVersion: "aaaa-1111", HostID: "uuid-peer-b"},
		{Peer: "10.0.0.9", SchemaVersion: "aaaa-1111", HostID: "uuid-dead-hp"},
	}
	q.members = []group0Member{
		{ServerID: "uuid-local", CanVote: boolPtr(true)},
		{ServerID: "uuid-peer-a", CanVote: boolPtr(true)},
		{ServerID: "uuid-peer-b", CanVote: boolPtr(true)},
		{ServerID: "uuid-dead-hp", CanVote: boolPtr(false)}, // in gossip but can_vote=false
	}
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK || r.Reason != DDLPreflightGroup0StaleVoter {
		t.Fatalf("expected stale voter result, got reason=%q ok=%v", r.Reason, r.OK)
	}
	ft := Group0FindingText(r)
	// Must contain voter server_id.
	if !strings.Contains(ft, "uuid-dead-hp") {
		t.Errorf("finding text must contain stale voter server_id; got: %s", ft)
	}
	// Must contain IP (resolved from peers).
	if !strings.Contains(ft, "10.0.0.9") {
		t.Errorf("finding text must contain resolved peer IP; got: %s", ft)
	}
	// Must name the source table.
	if !strings.Contains(ft, "system.raft_group0_members") {
		t.Errorf("finding text must name source table; got: %s", ft)
	}
	// Must include remediation action.
	if !strings.Contains(ft, "removenode") && !strings.Contains(ft, "admin API") {
		t.Errorf("finding text must include remediation action; got: %s", ft)
	}
}

// TestCanScyllaDDLProceed_Unknown_FindingText verifies that when the Group 0
// table is missing, the finding text clearly says voter health cannot be proven.
func TestCanScyllaDDLProceed_Unknown_FindingText(t *testing.T) {
	q := healthyQuerier()
	q.membersErr = ErrGroup0TableMissing
	q.raftStateErr = ErrGroup0TableMissing // both tables absent
	r := canScyllaDDLProceedWith(context.Background(), q)
	ft := Group0FindingText(r)
	if !strings.Contains(ft, "cannot be proven") {
		t.Errorf("unknown finding must say 'cannot be proven'; got: %s", ft)
	}
	if !strings.Contains(ft, "upgrade") && !strings.Contains(ft, "manually") {
		t.Errorf("unknown finding must include guidance; got: %s", ft)
	}
}

// ── Regression: ScyllaDB 2025.x+ raft_state fallback ────────────────────────

// TestCanScyllaDDLProceed_RaftStateFallback_OK verifies that when
// system.raft_group0_members is absent (ScyllaDB 2025.x+), a healthy
// system.raft_state with a single CURRENT voter allows DDL to proceed.
// Regression for: DDL permanently blocked on ScyllaDB 2025.3.x clusters.
func TestCanScyllaDDLProceed_RaftStateFallback_OK(t *testing.T) {
	q := healthyQuerier()
	q.membersErr = ErrGroup0TableMissing // raft_group0_members absent
	q.raftStateMembers = []group0Member{
		{ServerID: "uuid-local", CanVote: boolPtr(true)},
	}
	r := canScyllaDDLProceedWith(context.Background(), q)
	if !r.OK {
		t.Fatalf("raft_state fallback with healthy voter must allow DDL, got reason=%q finding=%q",
			r.Reason, Group0FindingText(r))
	}
	if r.Reason != DDLPreflightOK {
		t.Fatalf("expected reason=%q, got %q", DDLPreflightOK, r.Reason)
	}
}

// TestCanScyllaDDLProceed_RaftStateFallback_BothMissing verifies fail-closed:
// when both raft_group0_members and raft_state are absent, DDL is blocked.
func TestCanScyllaDDLProceed_RaftStateFallback_BothMissing(t *testing.T) {
	q := healthyQuerier()
	q.membersErr = ErrGroup0TableMissing
	q.raftStateErr = ErrGroup0TableMissing
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("both tables absent must block DDL (fail closed)")
	}
	if r.Reason != DDLPreflightUnknown {
		t.Fatalf("expected %q, got %q", DDLPreflightUnknown, r.Reason)
	}
}

// TestCanScyllaDDLProceed_RaftStateFallback_StaleVoter verifies that a
// stale voter in raft_state (can_vote=false, not in gossip) blocks DDL.
func TestCanScyllaDDLProceed_RaftStateFallback_StaleVoter(t *testing.T) {
	q := healthyQuerier()
	q.membersErr = ErrGroup0TableMissing
	q.raftStateMembers = []group0Member{
		{ServerID: "uuid-local", CanVote: boolPtr(true)},
		{ServerID: "uuid-dead-nuc", CanVote: boolPtr(true)}, // not in gossip
	}
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("stale voter in raft_state must block DDL")
	}
	if r.Reason != DDLPreflightGroup0StaleVoter {
		t.Fatalf("expected %q, got %q", DDLPreflightGroup0StaleVoter, r.Reason)
	}
}
