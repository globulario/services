package main

import (
	"context"
	"errors"
	"testing"
)

// fakeQuerier is a test double for scyllaPreflightQuerier.
type fakeQuerier struct {
	localSchema    string
	localSchemaErr error
	peers          []peerSchemaRow
	peersErr       error
	members        []group0Member
	membersErr     error
}

func (f *fakeQuerier) queryLocalSchema(_ context.Context) (string, error) {
	return f.localSchema, f.localSchemaErr
}
func (f *fakeQuerier) queryPeerSchemas(_ context.Context) ([]peerSchemaRow, error) {
	return f.peers, f.peersErr
}
func (f *fakeQuerier) queryGroup0Members(_ context.Context) ([]group0Member, error) {
	return f.members, f.membersErr
}

// healthyQuerier returns a fully-healthy fakeQuerier: one local node, two
// agreeing peers, three live Group 0 voters.
func healthyQuerier() *fakeQuerier {
	return &fakeQuerier{
		localSchema: "aaaa-1111",
		peers: []peerSchemaRow{
			{Peer: "10.0.0.8", SchemaVersion: "aaaa-1111"},
			{Peer: "10.0.0.20", SchemaVersion: "aaaa-1111"},
		},
		members: []group0Member{
			{ServerID: "uuid-a", CanVote: true, Voter: true},
			{ServerID: "uuid-b", CanVote: true, Voter: true},
			{ServerID: "uuid-c", CanVote: true, Voter: true},
		},
	}
}

func TestCanScyllaDDLProceed_HealthyCluster(t *testing.T) {
	r := canScyllaDDLProceedWith(context.Background(), healthyQuerier())
	if !r.OK {
		t.Fatalf("healthy cluster must produce OK, got reason=%q", r.Reason)
	}
	if r.Reason != DDLPreflightOK {
		t.Fatalf("expected reason=%q, got %q", DDLPreflightOK, r.Reason)
	}
}

func TestCanScyllaDDLProceed_ScyllaUnavailable_LocalQueryFails(t *testing.T) {
	q := healthyQuerier()
	q.localSchemaErr = errors.New("connection refused")
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
	q.localSchema = ""
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
		{Peer: "10.0.0.8", SchemaVersion: "aaaa-1111"},
		{Peer: "10.0.0.20", SchemaVersion: "bbbb-2222"}, // stale peer
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
// rule: when the Raft Group 0 table is absent (pre-Raft Scylla), DDL is blocked
// rather than assumed safe.
func TestCanScyllaDDLProceed_Unknown_Group0TableMissing(t *testing.T) {
	q := healthyQuerier()
	q.membersErr = ErrGroup0TableMissing
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("missing Group 0 table must block DDL (fail closed)")
	}
	if r.Reason != DDLPreflightUnknown {
		t.Fatalf("expected %q, got %q", DDLPreflightUnknown, r.Reason)
	}
}

// TestCanScyllaDDLProceed_Unknown_Group0QueryError covers an unexpected query
// failure (auth error, I/O error) — also blocked, fail closed.
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

// TestCanScyllaDDLProceed_Group0StaleVoter verifies that a voter with
// can_vote=false (wiped/removed node still in Raft membership) blocks DDL.
func TestCanScyllaDDLProceed_Group0StaleVoter(t *testing.T) {
	q := healthyQuerier()
	q.members = []group0Member{
		{ServerID: "uuid-a", CanVote: true, Voter: true},
		{ServerID: "uuid-dead", CanVote: false, Voter: true}, // stale voter
		{ServerID: "uuid-c", CanVote: true, Voter: true},
	}
	r := canScyllaDDLProceedWith(context.Background(), q)
	if r.OK {
		t.Fatal("stale voter must block DDL")
	}
	if r.Reason != DDLPreflightGroup0StaleVoter {
		t.Fatalf("expected %q, got %q", DDLPreflightGroup0StaleVoter, r.Reason)
	}
	if r.Details["first_stale_voter"] != "uuid-dead" {
		t.Errorf("expected first_stale_voter=uuid-dead, got %q", r.Details["first_stale_voter"])
	}
	if r.Details["stale_voter_count"] != "1" {
		t.Errorf("expected stale_voter_count=1, got %q", r.Details["stale_voter_count"])
	}
}
