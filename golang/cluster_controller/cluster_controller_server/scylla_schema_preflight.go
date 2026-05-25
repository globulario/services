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
// Group0 is populated whenever Layer 3 executes (table found, even if stale).
type DDLPreflightResult struct {
	OK        bool
	Reason    DDLPreflightReason
	Details   map[string]string
	Group0    *Group0View // non-nil when group0 table was successfully queried
	CheckedAt time.Time
}

func (r DDLPreflightResult) withDetail(key, value string) DDLPreflightResult {
	if r.Details == nil {
		r.Details = map[string]string{}
	}
	r.Details[key] = value
	return r
}

func (r DDLPreflightResult) withGroup0(v Group0View) DDLPreflightResult {
	r.Group0 = &v
	return r
}

func preflightOK() DDLPreflightResult {
	return DDLPreflightResult{OK: true, Reason: DDLPreflightOK, CheckedAt: time.Now()}
}

func preflightFail(reason DDLPreflightReason) DDLPreflightResult {
	return DDLPreflightResult{OK: false, Reason: reason, CheckedAt: time.Now()}
}

// localInfo is the result of querying system.local.
type localInfo struct {
	HostID        string
	SchemaVersion string
}

// group0Member is a row from system.raft_group0_members.
// CanVote is nil when the can_vote column is not present in this Scylla version.
type group0Member struct {
	ServerID string
	CanVote  *bool
}

// peerSchemaRow is a row from system.peers used for schema agreement checking
// and Group 0 gossip cross-referencing.
type peerSchemaRow struct {
	Peer          string
	SchemaVersion string
	HostID        string // for Group 0 cross-reference; empty in old Scylla versions
}

// scyllaPreflightQuerier abstracts CQL queries used by the DDL preflight check,
// enabling unit tests to inject fakes without a real Scylla connection.
type scyllaPreflightQuerier interface {
	// queryLocalInfo returns the local node's host_id and schema_version from
	// system.local. Used for Layer 1 (reachability) and Group 0 cross-reference.
	queryLocalInfo(ctx context.Context) (localInfo, error)
	// queryPeerSchemas returns peer IP, schema_version, and host_id from
	// system.peers. Used for Layer 2 (agreement) and Group 0 cross-reference.
	queryPeerSchemas(ctx context.Context) ([]peerSchemaRow, error)
	// queryGroup0Members returns members from system.raft_group0_members.
	// Returns ErrGroup0TableMissing when the table does not exist.
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
//  1. system.local is queryable → Scylla reachable; yields local host_id.
//  2. system.peers schema_versions all agree with local → DDL can propagate.
//  3. system.raft_group0_members: cross-reference voter IDs against gossip;
//     detect stale voters (not in gossip or can_vote=false).
//     Table missing → DDLPreflightUnknown (BLOCKS DDL, fail closed).
func CanScyllaDDLProceed(ctx context.Context, session *gocql.Session) DDLPreflightResult {
	if session == nil {
		return preflightFail(DDLPreflightScyllaUnavailable).withDetail("reason", "nil session")
	}
	return canScyllaDDLProceedWith(ctx, &gocqlQuerier{session: session})
}

// canScyllaDDLProceedWith is the testable core of CanScyllaDDLProceed.
func canScyllaDDLProceedWith(ctx context.Context, q scyllaPreflightQuerier) DDLPreflightResult {
	// Layer 1: Scylla reachability and local host identity.
	local, err := q.queryLocalInfo(ctx)
	if err != nil {
		return preflightFail(DDLPreflightScyllaUnavailable).
			withDetail("query", "system.local").
			withDetail("error", err.Error())
	}
	if local.SchemaVersion == "" {
		return preflightFail(DDLPreflightScyllaUnavailable).
			withDetail("reason", "empty schema_version in system.local")
	}

	// Layer 2: Schema agreement — all peers must agree on schema_version.
	// Disagreement means a prior DDL has not propagated; issuing new DDL on
	// top of that risks split-brain schema state.
	peers, err := q.queryPeerSchemas(ctx)
	if err != nil {
		return preflightFail(DDLPreflightSchemaAgreementUnavailable).
			withDetail("query", "system.peers").
			withDetail("error", err.Error())
	}
	disagreements := 0
	var firstDisagreePeer, firstDisagreeVersion string
	for _, p := range peers {
		if p.SchemaVersion != "" && p.SchemaVersion != local.SchemaVersion {
			disagreements++
			if firstDisagreePeer == "" {
				firstDisagreePeer = p.Peer
				firstDisagreeVersion = p.SchemaVersion
			}
		}
	}
	if disagreements > 0 {
		return preflightFail(DDLPreflightSchemaAgreementUnavailable).
			withDetail("local_schema_version", local.SchemaVersion).
			withDetail("first_disagreeing_peer", firstDisagreePeer).
			withDetail("peer_schema_version", firstDisagreeVersion).
			withDetail("disagreement_count", fmt.Sprintf("%d", disagreements))
	}

	// Layer 3: Raft Group 0 voter health via gossip cross-reference.
	//
	// If system.raft_group0_members does not exist, we cannot verify voter
	// health → DDLPreflightUnknown → DDL blocked (fail closed). A single
	// dead voter in Raft membership causes a silent DDL hang that can freeze
	// the entire cluster's schema management.
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

	// Build the structured view. Cross-references member server_ids against
	// gossip (system.local.host_id + system.peers.host_id) to detect stale
	// voters — nodes that are still Raft voters but have been wiped/removed.
	view := collectGroup0View(members, local.HostID, peers)

	if view.TotalVoters == 0 {
		return preflightFail(DDLPreflightGroup0Unavailable).
			withDetail("reason", "raft_group0_members_empty").
			withGroup0(view)
	}

	if view.StaleVoters > 0 {
		return preflightFail(DDLPreflightGroup0StaleVoter).
			withDetail("stale_voter_count", fmt.Sprintf("%d", view.StaleVoters)).
			withDetail("first_stale_voter", firstStaleVoter(view)).
			withGroup0(view)
	}

	return preflightOK().withGroup0(view)
}

func firstStaleVoter(view Group0View) string {
	for _, v := range view.Voters {
		if v.IsStale() {
			return v.ServerID
		}
	}
	return ""
}

// gocqlQuerier implements scyllaPreflightQuerier against a real *gocql.Session.
type gocqlQuerier struct {
	session *gocql.Session
}

func (g *gocqlQuerier) queryLocalInfo(ctx context.Context) (localInfo, error) {
	var info localInfo
	err := g.session.Query(`SELECT host_id, schema_version FROM system.local`).
		WithContext(ctx).Consistency(gocql.One).Scan(&info.HostID, &info.SchemaVersion)
	return info, err
}

func (g *gocqlQuerier) queryPeerSchemas(ctx context.Context) ([]peerSchemaRow, error) {
	iter := g.session.Query(`SELECT peer, schema_version, host_id FROM system.peers`).
		WithContext(ctx).Consistency(gocql.One).Iter()
	var rows []peerSchemaRow
	var peer, sv, hostID string
	for iter.Scan(&peer, &sv, &hostID) {
		rows = append(rows, peerSchemaRow{Peer: peer, SchemaVersion: sv, HostID: hostID})
	}
	return rows, iter.Close()
}

// queryGroup0Members first tries to fetch server_id and can_vote. If can_vote
// does not exist in this Scylla version, it falls back to server_id only.
// If the table itself is absent, it returns ErrGroup0TableMissing.
func (g *gocqlQuerier) queryGroup0Members(ctx context.Context) ([]group0Member, error) {
	iter := g.session.Query(`SELECT server_id, can_vote FROM system.raft_group0_members`).
		WithContext(ctx).Consistency(gocql.One).Iter()
	var members []group0Member
	var serverID string
	var canVote bool
	for iter.Scan(&serverID, &canVote) {
		cv := canVote
		members = append(members, group0Member{ServerID: serverID, CanVote: &cv})
	}
	if err := iter.Close(); err != nil {
		msg := err.Error()
		if isGroup0TableMissingErr(msg) {
			return nil, ErrGroup0TableMissing
		}
		if strings.Contains(msg, "can_vote") || strings.Contains(msg, "Undefined name") {
			// can_vote column absent (older Scylla) — retry with server_id only.
			return g.queryGroup0MembersBasic(ctx)
		}
		return nil, err
	}
	return members, nil
}

func (g *gocqlQuerier) queryGroup0MembersBasic(ctx context.Context) ([]group0Member, error) {
	iter := g.session.Query(`SELECT server_id FROM system.raft_group0_members`).
		WithContext(ctx).Consistency(gocql.One).Iter()
	var members []group0Member
	var serverID string
	for iter.Scan(&serverID) {
		members = append(members, group0Member{ServerID: serverID, CanVote: nil})
	}
	if err := iter.Close(); err != nil {
		if isGroup0TableMissingErr(err.Error()) {
			return nil, ErrGroup0TableMissing
		}
		return nil, err
	}
	return members, nil
}

func isGroup0TableMissingErr(msg string) bool {
	return strings.Contains(msg, "unconfigured table") ||
		strings.Contains(msg, "table not found") ||
		(strings.Contains(msg, "raft_group0_members") &&
			strings.Contains(msg, "does not exist"))
}

func boolPtr(b bool) *bool { return &b }
