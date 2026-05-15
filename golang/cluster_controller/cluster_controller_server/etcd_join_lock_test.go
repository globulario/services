package main

// etcd_join_lock_test.go — Pin the join-in-progress lock schema and its
// effect on stale-member eviction.
//
// The lock closes a structural race observed live on 2026-05-14: the
// gateway-served join script runs `etcdctl member add` BEFORE the joining
// node's node-agent has heartbeated, so for ~10–30s the controller's
// desired set does NOT contain the new node. removeStaleMembers' eviction
// loop then classified the freshly-added member as stale and removed it.
//
// The fix: the join script writes /globular/etcd_joins/<sanitized_hostname>
// with a leased TTL before member-add. The controller's eviction loop reads
// this prefix and skips any member.Name listed in it.

import (
	"testing"
)

// ── Lock-key parsing ──────────────────────────────────────────────────────

func TestParseEtcdJoinLockKeys_HappyPath(t *testing.T) {
	keys := []string{
		etcdJoinsInProgressPrefix + "globule-nuc",
		etcdJoinsInProgressPrefix + "globule-dell",
	}
	got := parseEtcdJoinLockKeys(keys)
	if !got["globule-nuc"] || !got["globule-dell"] {
		t.Errorf("expected both names in lock set, got %v", got)
	}
	if len(got) != 2 {
		t.Errorf("expected exactly 2 entries, got %d: %v", len(got), got)
	}
}

func TestParseEtcdJoinLockKeys_DropsMalformed(t *testing.T) {
	// Lock entries that don't match the schema must NOT cause spurious
	// "in-progress" claims. A malformed entry that bypassed validation
	// would let any operator stamp /globular/etcd_joins/anything and
	// pin a real member forever. Defense-in-depth at the read side.
	keys := []string{
		etcdJoinsInProgressPrefix,             // bare prefix, no hostname
		etcdJoinsInProgressPrefix + "   ",     // whitespace-only suffix
		"/some/other/prefix/globule-nuc",      // outside our prefix
		"",                                    // empty key
		etcdJoinsInProgressPrefix + "valid",   // real entry
	}
	got := parseEtcdJoinLockKeys(keys)
	if len(got) != 1 || !got["valid"] {
		t.Errorf("expected exactly one entry {valid:true}, got %v", got)
	}
}

func TestParseEtcdJoinLockKeys_Empty(t *testing.T) {
	// Empty etcd response (no join in flight) must yield an empty map,
	// not a nil map — callers iterate without nil-checking.
	got := parseEtcdJoinLockKeys(nil)
	if got == nil {
		t.Fatal("must return non-nil map for nil input")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map, got %v", got)
	}
}

// ── Nil-client guard ─────────────────────────────────────────────────────

func TestJoinInProgressMembers_NilClientFailsOpen(t *testing.T) {
	// A manager constructed without a real etcd client (e.g. before
	// bootstrap completes, or during test setup) must return an empty
	// set rather than panicking or returning nil. Empty = "no join in
	// flight" = preserve the existing removeStaleMembers behavior.
	mgr := &etcdMemberManager{client: nil}
	got := mgr.joinInProgressMembers(nil)
	if got == nil {
		t.Fatal("must return non-nil map even with nil client")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map for nil client, got %v", got)
	}
}

func TestJoinInProgressMembers_NilReceiver(t *testing.T) {
	// Defensive: a nil *etcdMemberManager must not panic. Production
	// callers always pass a real pointer, but the bootstrap path
	// briefly has nil and the controller restart-loop has been bitten
	// by panics from nil receivers before.
	var mgr *etcdMemberManager
	got := mgr.joinInProgressMembers(nil)
	if got == nil {
		t.Fatal("must return non-nil map for nil receiver")
	}
	if len(got) != 0 {
		t.Errorf("expected empty map for nil receiver, got %v", got)
	}
}
