package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/gocql/gocql"
)

// DDLPreflightReason is the machine-readable outcome of a DDL preflight check.
// All non-OK values block DDL. DDLPreflightUnknown is fail-closed: when Group 0
// state cannot be determined, DDL is blocked to prevent a silent Raft deadlock.
type DDLPreflightReason string

const (
	DDLPreflightOK                         DDLPreflightReason = "ok"
	DDLPreflightUnknown                    DDLPreflightReason = "unknown"
	DDLPreflightScyllaUnavailable          DDLPreflightReason = "scylla_unavailable"
	DDLPreflightGroup0Unavailable          DDLPreflightReason = "group0_unavailable"
	DDLPreflightGroup0StaleVoter           DDLPreflightReason = "group0_stale_voter"
	DDLPreflightSchemaAgreementUnavailable DDLPreflightReason = "schema_agreement_unavailable"
)

// DDLPreflightResult carries the outcome of CanScyllaDDLProceed.
type DDLPreflightResult struct {
	OK        bool
	Reason    DDLPreflightReason
	Details   map[string]string
	CheckedAt time.Time
}

func (r DDLPreflightResult) withDetail(key, value string) DDLPreflightResult {
	if r.Details == nil {
		r.Details = map[string]string{}
	}
	r.Details[key] = value
	return r
}

func preflightOK() DDLPreflightResult {
	return DDLPreflightResult{OK: true, Reason: DDLPreflightOK, CheckedAt: time.Now()}
}

func preflightFail(reason DDLPreflightReason) DDLPreflightResult {
	return DDLPreflightResult{OK: false, Reason: reason, CheckedAt: time.Now()}
}

// group0Member is a row from system.raft_group0_members.
type group0Member struct {
	ServerID string
	CanVote  bool
	Voter    bool
}

// peerSchemaRow is a row from system.peers.
type peerSchemaRow struct {
	Peer          string
	SchemaVersion string
}

// scyllaPreflightQuerier abstracts CQL queries used by the DDL preflight check,
// enabling unit tests to inject fakes without a real Scylla connection.
type scyllaPreflightQuerier interface {
	queryLocalSchema(ctx context.Context) (schemaVersion string, err error)
	queryPeerSchemas(ctx context.Context) ([]peerSchemaRow, error)
	// queryGroup0Members returns ErrGroup0TableMissing when the
	// system.raft_group0_members table does not exist.
	queryGroup0Members(ctx context.Context) ([]group0Member, error)
}

// ErrGroup0TableMissing is returned by queryGroup0Members when the Raft Group 0
// members table is not present (pre-Raft Scylla or test cluster).
var ErrGroup0TableMissing = fmt.Errorf("system.raft_group0_members table not found")

// CanScyllaDDLProceed checks whether Scylla schema mutations (CREATE KEYSPACE,
// ALTER KEYSPACE) can complete right now. It must be called before any DDL.
//
// Design rule: fail closed. If Group 0 state is unknown or unhealthy, DDL is
// blocked. A silent Raft deadlock is worse than a skipped keyspace creation.
//
// Checks (in order):
//  1. Session is non-nil and system.local is queryable (Scylla reachable).
//  2. All peers agree on schema_version (DDL can propagate).
//  3. system.raft_group0_members has no stale voters.
//     Table missing → DDLPreflightUnknown (BLOCKS DDL, fail closed).
func CanScyllaDDLProceed(ctx context.Context, session *gocql.Session) DDLPreflightResult {
	if session == nil {
		return preflightFail(DDLPreflightScyllaUnavailable).withDetail("reason", "nil session")
	}
	return canScyllaDDLProceedWith(ctx, &gocqlQuerier{session: session})
}

// canScyllaDDLProceedWith is the testable core of CanScyllaDDLProceed.
func canScyllaDDLProceedWith(ctx context.Context, q scyllaPreflightQuerier) DDLPreflightResult {
	// Layer 1: Scylla reachability — query system.local.
	localSchema, err := q.queryLocalSchema(ctx)
	if err != nil {
		return preflightFail(DDLPreflightScyllaUnavailable).
			withDetail("query", "system.local").
			withDetail("error", err.Error())
	}
	if localSchema == "" {
		return preflightFail(DDLPreflightScyllaUnavailable).
			withDetail("reason", "empty schema_version in system.local")
	}

	// Layer 2: Schema agreement — all peers must share the same schema_version.
	// Disagreement means a prior DDL has not propagated; issuing new DDL on top
	// of that can create split-brain schema state.
	peers, err := q.queryPeerSchemas(ctx)
	if err != nil {
		return preflightFail(DDLPreflightSchemaAgreementUnavailable).
			withDetail("query", "system.peers").
			withDetail("error", err.Error())
	}
	disagreements := 0
	var firstDisagreePeer, firstDisagreeVersion string
	for _, p := range peers {
		if p.SchemaVersion != "" && p.SchemaVersion != localSchema {
			disagreements++
			if firstDisagreePeer == "" {
				firstDisagreePeer = p.Peer
				firstDisagreeVersion = p.SchemaVersion
			}
		}
	}
	if disagreements > 0 {
		return preflightFail(DDLPreflightSchemaAgreementUnavailable).
			withDetail("local_schema_version", localSchema).
			withDetail("first_disagreeing_peer", firstDisagreePeer).
			withDetail("peer_schema_version", firstDisagreeVersion).
			withDetail("disagreement_count", fmt.Sprintf("%d", disagreements))
	}

	// Layer 3: Raft Group 0 voter health.
	//
	// If system.raft_group0_members does not exist, the cluster predates Raft-
	// managed schema or is a version that does not expose it. We cannot verify
	// voter health, so we return DDLPreflightUnknown — which BLOCKS DDL
	// (fail closed). A single dead voter causes a silent DDL hang.
	members, err := q.queryGroup0Members(ctx)
	if err != nil {
		if err == ErrGroup0TableMissing {
			return preflightFail(DDLPreflightUnknown).
				withDetail("reason", "raft_group0_members_table_missing")
		}
		return preflightFail(DDLPreflightUnknown).
			withDetail("reason", "group0_members_query_failed").
			withDetail("error", err.Error())
	}

	if len(members) == 0 {
		return preflightFail(DDLPreflightGroup0Unavailable).
			withDetail("reason", "raft_group0_members_empty")
	}

	staleVoters := 0
	var firstStale string
	for _, m := range members {
		// A voter with can_vote=false is a stale/dead voter that still holds a
		// Raft seat. It will block consensus and cause silent DDL hangs.
		if m.Voter && !m.CanVote {
			staleVoters++
			if firstStale == "" {
				firstStale = m.ServerID
			}
		}
	}
	if staleVoters > 0 {
		return preflightFail(DDLPreflightGroup0StaleVoter).
			withDetail("stale_voter_count", fmt.Sprintf("%d", staleVoters)).
			withDetail("first_stale_voter", firstStale)
	}

	return preflightOK()
}

// gocqlQuerier implements scyllaPreflightQuerier against a real *gocql.Session.
type gocqlQuerier struct {
	session *gocql.Session
}

func (g *gocqlQuerier) queryLocalSchema(ctx context.Context) (string, error) {
	var v string
	err := g.session.Query(`SELECT schema_version FROM system.local`).
		WithContext(ctx).Consistency(gocql.One).Scan(&v)
	return v, err
}

func (g *gocqlQuerier) queryPeerSchemas(ctx context.Context) ([]peerSchemaRow, error) {
	iter := g.session.Query(`SELECT peer, schema_version FROM system.peers`).
		WithContext(ctx).Consistency(gocql.One).Iter()
	var rows []peerSchemaRow
	var peer, sv string
	for iter.Scan(&peer, &sv) {
		rows = append(rows, peerSchemaRow{Peer: peer, SchemaVersion: sv})
	}
	return rows, iter.Close()
}

func (g *gocqlQuerier) queryGroup0Members(ctx context.Context) ([]group0Member, error) {
	iter := g.session.Query(`SELECT server_id, can_vote, voter FROM system.raft_group0_members`).
		WithContext(ctx).Consistency(gocql.One).Iter()
	var members []group0Member
	var serverID string
	var canVote, voter bool
	for iter.Scan(&serverID, &canVote, &voter) {
		members = append(members, group0Member{ServerID: serverID, CanVote: canVote, Voter: voter})
	}
	if err := iter.Close(); err != nil {
		// Translate "table not found" errors from gocql into ErrGroup0TableMissing
		// so callers don't need to inspect gocql error strings.
		msg := err.Error()
		if strings.Contains(msg, "unconfigured table") ||
			strings.Contains(msg, "table not found") ||
			strings.Contains(msg, "raft_group0_members") {
			return nil, ErrGroup0TableMissing
		}
		return nil, err
	}
	return members, nil
}
