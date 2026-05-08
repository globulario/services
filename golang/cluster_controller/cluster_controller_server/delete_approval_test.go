package main

// globular:tested_by intent_markers_tombstones

import (
	"encoding/json"
	"testing"
	"time"
)

// TestDeleteWithoutApprovalRestoresKey verifies that hasDeleteApprovalFromKVs
// returns false (indicating the guard must restore the key) when the approval
// set is empty, malformed, or missing required fields. A guard receiving false
// must restore the critical key — it must never treat absence as intent.
//
// Invariant: critical_state.deletion_requires_audited_intent
func TestDeleteWithoutApprovalRestoresKey(t *testing.T) {
	now := time.Now().Unix()

	// No approval keys at all.
	if hasDeleteApprovalFromKVs(nil, now) {
		t.Error("nil approval set: expected false (restore), got true (accept deletion)")
	}
	if hasDeleteApprovalFromKVs([][]byte{}, now) {
		t.Error("empty approval set: expected false (restore), got true (accept deletion)")
	}

	// Malformed JSON — not a valid record.
	if hasDeleteApprovalFromKVs([][]byte{[]byte("{bad json!!!")}, now) {
		t.Error("malformed JSON: expected false, got true")
	}

	// Missing actor_identity.
	noActor := criticalKeyDeleteApproval{
		Generation:     1,
		ActorIdentity:  "", // required
		Reason:         "planned",
		ApprovedAtUnix: now - 60,
	}
	data, _ := json.Marshal(noActor)
	if hasDeleteApprovalFromKVs([][]byte{data}, now) {
		t.Error("missing actor_identity: expected false, got true")
	}

	// Missing reason.
	noReason := criticalKeyDeleteApproval{
		Generation:     2,
		ActorIdentity:  "operator-alice",
		Reason:         "", // required
		ApprovedAtUnix: now - 60,
	}
	data2, _ := json.Marshal(noReason)
	if hasDeleteApprovalFromKVs([][]byte{data2}, now) {
		t.Error("missing reason: expected false, got true")
	}

	// Both actor and reason missing.
	empty := criticalKeyDeleteApproval{ApprovedAtUnix: now - 60}
	data3, _ := json.Marshal(empty)
	if hasDeleteApprovalFromKVs([][]byte{data3}, now) {
		t.Error("empty approval: expected false, got true")
	}
}

// TestDeleteWithValidApprovalAccepted verifies that hasDeleteApprovalFromKVs
// returns true (indicating the guard must not restore) when a valid, fresh
// approval record exists. A valid approval has non-empty actor and reason,
// and was written within the last 24 hours.
//
// Invariant: critical_state.deletion_requires_audited_intent
func TestDeleteWithValidApprovalAccepted(t *testing.T) {
	now := time.Now().Unix()

	valid := criticalKeyDeleteApproval{
		Generation:     7,
		ActorIdentity:  "operator-alice",
		Reason:         "scheduled maintenance window — removing objectstore topology",
		ApprovedAtUnix: now - 120, // 2 minutes ago — well within 24 h
	}
	data, _ := json.Marshal(valid)
	if !hasDeleteApprovalFromKVs([][]byte{data}, now) {
		t.Error("valid approval within 24h: expected true (accept deletion), got false (restore)")
	}

	// Multiple records — only one needs to be valid.
	badRecord := criticalKeyDeleteApproval{ActorIdentity: "", Reason: "", ApprovedAtUnix: now - 60}
	badData, _ := json.Marshal(badRecord)
	if !hasDeleteApprovalFromKVs([][]byte{badData, data}, now) {
		t.Error("mixed set with one valid record: expected true, got false")
	}
}

// TestStaleApprovalGenerationRejected verifies that approval records older
// than 24 hours are rejected. The guard must restore the key when only stale
// approvals exist — operator intent expires and the cluster self-heals.
//
// Invariant: critical_state.deletion_requires_audited_intent
func TestStaleApprovalGenerationRejected(t *testing.T) {
	now := time.Now().Unix()

	// 25 hours old — clearly stale.
	stale := criticalKeyDeleteApproval{
		Generation:     3,
		ActorIdentity:  "operator-bob",
		Reason:         "old maintenance window",
		ApprovedAtUnix: now - 90000, // 25 h
	}
	data, _ := json.Marshal(stale)
	if hasDeleteApprovalFromKVs([][]byte{data}, now) {
		t.Error("25h-old approval: expected false (restore), got true (accept)")
	}

	// Exactly 1 second past the 24h boundary — rejected.
	justExpired := criticalKeyDeleteApproval{
		Generation:     4,
		ActorIdentity:  "operator-carol",
		Reason:         "boundary test",
		ApprovedAtUnix: now - (approvalMaxAge + 1),
	}
	data2, _ := json.Marshal(justExpired)
	if hasDeleteApprovalFromKVs([][]byte{data2}, now) {
		t.Error("approval 1s past 24h boundary: expected false, got true")
	}

	// Exactly at the boundary (age == maxAge) — accepted.
	atBoundary := criticalKeyDeleteApproval{
		Generation:     5,
		ActorIdentity:  "operator-carol",
		Reason:         "boundary test",
		ApprovedAtUnix: now - approvalMaxAge,
	}
	data3, _ := json.Marshal(atBoundary)
	if !hasDeleteApprovalFromKVs([][]byte{data3}, now) {
		t.Error("approval exactly at 24h boundary: expected true, got false")
	}

	// 1 second within the window — valid.
	almostExpired := criticalKeyDeleteApproval{
		Generation:     6,
		ActorIdentity:  "operator-carol",
		Reason:         "boundary test",
		ApprovedAtUnix: now - (approvalMaxAge - 1),
	}
	data4, _ := json.Marshal(almostExpired)
	if !hasDeleteApprovalFromKVs([][]byte{data4}, now) {
		t.Error("approval 1s within 24h window: expected true, got false")
	}
}
