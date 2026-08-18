package main

import (
	"path/filepath"
	"testing"
	"time"
)

// A join token's MaxUses bounds how many DISTINCT NODES it admits. It does not
// bound attempts.
//
// Observed 2026-08-18 on the 5-node simulation: the join script died at phase
// [2.3] "Generating service certificate" when sign_ca_certificate answered
// non-2xx, leaving service.key with no service.crt. The controller had already
// charged the use at authorization time, so the retry found the budget spent —
// and because cleanupJoinStateLocked deletes an exhausted token, the attempt
// after that got "join token not found" instead. The node was permanently
// unjoinable, and wiping its state did not help: the exhaustion lived on the
// controller, not the node. node-5 and the cold-boot nodes 2 and 3 were
// stranded this way.
//
// These tests pin both halves: a retrying node is not charged twice, and a
// different node still cannot get in on a spent budget.

func admissionTestServer(t *testing.T) *server {
	t.Helper()
	state := newControllerState()
	statePath := filepath.Join(t.TempDir(), "state.json")
	srv := newServer(defaultClusterControllerConfig(), "", statePath, state, nil)
	srv.setLeader(true, "test", "127.0.0.1:1234")
	return srv
}

func seedToken(srv *server, token string, maxUses int) *joinTokenRecord {
	jt := &joinTokenRecord{
		Token:     token,
		MaxUses:   maxUses,
		Uses:      0,
		ExpiresAt: time.Now().Add(time.Hour),
	}
	srv.state.JoinTokens[token] = jt
	return jt
}

func nodeIdentity(host string, ips ...string) storedIdentity {
	return storedIdentity{Hostname: host, Ips: ips}
}

// TestJoinAdmissionKey_StableAcrossIPOrderAndCase proves the retry predicate
// keys on the node, not on the wire formatting of its identity. A node whose
// agent reports its IPs in a different order across attempts is the same node.
func TestJoinAdmissionKey_StableAcrossIPOrderAndCase(t *testing.T) {
	a := joinAdmissionKey(nodeIdentity("node-5", "10.10.0.15", "192.168.1.5"))
	b := joinAdmissionKey(nodeIdentity("NODE-5", "192.168.1.5", "10.10.0.15", "10.10.0.15"))
	if a != b {
		t.Errorf("admission key not stable across IP order/case/duplicates:\n  a=%q\n  b=%q", a, b)
	}
	if a == "" {
		t.Error("admission key is empty for a fully-identified node")
	}
}

// TestJoinAdmissionKey_EmptyIdentityIsNotAKey pins the fail-closed direction: a
// caller with no stable identity must never match someone else's admission and
// ride in on their token use.
func TestJoinAdmissionKey_EmptyIdentityIsNotAKey(t *testing.T) {
	if got := joinAdmissionKey(storedIdentity{}); got != "" {
		t.Errorf("joinAdmissionKey(empty) = %q, want \"\" so it is treated as a new admission", got)
	}
	srv := admissionTestServer(t)
	seedToken(srv, "tok", 1)
	srv.state.JoinRequests["existing"] = &joinRequestRecord{
		RequestID:      "existing",
		Token:          "tok",
		Identity:       nodeIdentity("node-5", "10.10.0.15"),
		LifecyclePhase: JoinPhaseAuthorized,
	}
	if got := srv.priorJoinAdmissionLocked("tok", storedIdentity{}); got != nil {
		t.Errorf("an identity-less request matched an existing admission (%s) — it must not", got.RequestID)
	}
}

// TestPriorJoinAdmission_MatchesSameNodeRetry is the core of the fix: the same
// node coming back under the same token is recognised as continuing its
// admission.
func TestPriorJoinAdmission_MatchesSameNodeRetry(t *testing.T) {
	srv := admissionTestServer(t)
	seedToken(srv, "tok", 1)
	srv.state.JoinRequests["first"] = &joinRequestRecord{
		RequestID:      "first",
		Token:          "tok",
		Identity:       nodeIdentity("node-5", "10.10.0.15"),
		LifecyclePhase: JoinPhaseAuthorized,
	}

	got := srv.priorJoinAdmissionLocked("tok", nodeIdentity("node-5", "10.10.0.15"))
	if got == nil {
		t.Fatal("a retrying node was not recognised as holding a prior admission")
	}
	if got.RequestID != "first" {
		t.Errorf("matched %q, want the original admission %q", got.RequestID, "first")
	}
}

// TestPriorJoinAdmission_DifferentNodeIsNotARetry is the security half. Once the
// budget is spent, a DIFFERENT node must still be refused — the fix must not
// turn a single-use token into an unlimited one.
func TestPriorJoinAdmission_DifferentNodeIsNotARetry(t *testing.T) {
	srv := admissionTestServer(t)
	seedToken(srv, "tok", 1)
	srv.state.JoinRequests["first"] = &joinRequestRecord{
		RequestID:      "first",
		Token:          "tok",
		Identity:       nodeIdentity("node-5", "10.10.0.15"),
		LifecyclePhase: JoinPhaseAuthorized,
	}

	if got := srv.priorJoinAdmissionLocked("tok", nodeIdentity("node-9", "10.10.0.19")); got != nil {
		t.Errorf("a different node matched an existing admission (%s) — a spent token must not admit it", got.RequestID)
	}
}

// TestPriorJoinAdmission_OtherTokenIsNotARetry — an admission granted by one
// token must not excuse a second token's budget.
func TestPriorJoinAdmission_OtherTokenIsNotARetry(t *testing.T) {
	srv := admissionTestServer(t)
	srv.state.JoinRequests["first"] = &joinRequestRecord{
		RequestID:      "first",
		Token:          "tok-a",
		Identity:       nodeIdentity("node-5", "10.10.0.15"),
		LifecyclePhase: JoinPhaseAuthorized,
	}
	if got := srv.priorJoinAdmissionLocked("tok-b", nodeIdentity("node-5", "10.10.0.15")); got != nil {
		t.Errorf("an admission under tok-a matched a request under tok-b (%s)", got.RequestID)
	}
}

// TestPriorJoinAdmission_TerminalRecordIsNotLive — a rejected or removed node is
// not mid-join, so it does not keep a free retry slot open forever.
func TestPriorJoinAdmission_TerminalRecordIsNotLive(t *testing.T) {
	for _, phase := range []JoinLifecyclePhase{
		JoinPhaseRejected, JoinPhaseRemoved, JoinPhaseRemoving,
		JoinPhaseQuarantined, JoinPhaseStaleGhost,
	} {
		srv := admissionTestServer(t)
		srv.state.JoinRequests["done"] = &joinRequestRecord{
			RequestID:      "done",
			Token:          "tok",
			Identity:       nodeIdentity("node-5", "10.10.0.15"),
			LifecyclePhase: phase,
		}
		if got := srv.priorJoinAdmissionLocked("tok", nodeIdentity("node-5", "10.10.0.15")); got != nil {
			t.Errorf("phase %s counted as a live admission (%s)", phase, got.RequestID)
		}
	}
}

// TestCleanupJoinState_KeepsExhaustedTokenWhileNodeIsStillJoining pins the
// second half of the original failure. Deleting the token out from under a node
// that is mid-join turns every retry into "join token not found" — an error that
// reads as operator mistake and hides that the node already holds an admission.
func TestCleanupJoinState_KeepsExhaustedTokenWhileNodeIsStillJoining(t *testing.T) {
	srv := admissionTestServer(t)
	jt := seedToken(srv, "tok", 1)
	jt.Uses = 1 // exhausted
	srv.state.JoinRequests["inflight"] = &joinRequestRecord{
		RequestID:      "inflight",
		Token:          "tok",
		Identity:       nodeIdentity("node-5", "10.10.0.15"),
		RequestedAt:    time.Now(),
		Status:         "pending",
		LifecyclePhase: JoinPhaseAuthorized,
	}

	srv.cleanupJoinStateLocked(time.Now())

	if _, ok := srv.state.JoinTokens["tok"]; !ok {
		t.Error("exhausted token was deleted while a node was still mid-join — its retries will now fail with \"token not found\"")
	}
}

// TestCleanupJoinState_ReclaimsExhaustedTokenOnceNothingIsJoining — the delay is
// not a leak. Once the admission reaches a terminal phase the token is dropped
// as before.
func TestCleanupJoinState_ReclaimsExhaustedTokenOnceNothingIsJoining(t *testing.T) {
	srv := admissionTestServer(t)
	jt := seedToken(srv, "tok", 1)
	jt.Uses = 1
	srv.state.JoinRequests["done"] = &joinRequestRecord{
		RequestID:      "done",
		Token:          "tok",
		Identity:       nodeIdentity("node-5", "10.10.0.15"),
		RequestedAt:    time.Now(),
		LifecyclePhase: JoinPhaseRemoved,
	}

	srv.cleanupJoinStateLocked(time.Now())

	if _, ok := srv.state.JoinTokens["tok"]; ok {
		t.Error("exhausted token was retained even though nothing is joining under it")
	}
}

// TestCleanupJoinState_ExpiredTokenStillReclaimed — the live-admission hold
// applies only to the exhaustion path. Expiry still collects unconditionally,
// so a token cannot be pinned open indefinitely.
func TestCleanupJoinState_ExpiredTokenStillReclaimed(t *testing.T) {
	srv := admissionTestServer(t)
	jt := seedToken(srv, "tok", 5)
	jt.ExpiresAt = time.Now().Add(-time.Hour)
	srv.state.JoinRequests["inflight"] = &joinRequestRecord{
		RequestID:      "inflight",
		Token:          "tok",
		Identity:       nodeIdentity("node-5", "10.10.0.15"),
		RequestedAt:    time.Now(),
		LifecyclePhase: JoinPhaseAuthorized,
	}

	srv.cleanupJoinStateLocked(time.Now())

	if _, ok := srv.state.JoinTokens["tok"]; ok {
		t.Error("an EXPIRED token was retained — only exhaustion waits on live admissions")
	}
}

// ── end-to-end through the v2 authorization handler ──────────────────────────

// TestRequestJoinAuthorization_RetryDoesNotConsumeASecondUse is the test that
// would have caught the original bug. The same node authorizes twice — exactly
// what the installer does when phase [2.3] fails and the join script re-runs —
// and the token must be charged once, not twice.
func TestRequestJoinAuthorization_RetryDoesNotConsumeASecondUse(t *testing.T) {
	srv := newJoinAuthServer(t)
	identity := NodePlanIdentity{Hostname: "node-retry-01", IPs: []string{"10.0.9.1"}}

	first, err := srv.requestJoinAuthorizationCore(&JoinAuthorizationRequest{
		JoinToken: "tok-v2", Identity: identity, Nonce: "n1",
	})
	if err != nil {
		t.Fatalf("first authorization: %v", err)
	}
	if !first.Allowed {
		t.Fatalf("first authorization denied: %q", first.DeniedReason)
	}
	afterFirst := srv.state.JoinTokens["tok-v2"].Uses
	if afterFirst != 1 {
		t.Fatalf("uses after first authorization = %d, want 1", afterFirst)
	}

	second, err := srv.requestJoinAuthorizationCore(&JoinAuthorizationRequest{
		JoinToken: "tok-v2", Identity: identity, Nonce: "n2",
	})
	if err != nil {
		t.Fatalf("retry authorization: %v", err)
	}
	if !second.Allowed {
		t.Fatalf("retry authorization denied: %q", second.DeniedReason)
	}
	if got := srv.state.JoinTokens["tok-v2"].Uses; got != 1 {
		t.Errorf("uses after retry = %d, want 1 — the same node retrying must not be charged twice", got)
	}
}

// TestRequestJoinAuthorization_DistinctNodesEachConsumeAUse is the counterpart:
// the budget must still count nodes.
func TestRequestJoinAuthorization_DistinctNodesEachConsumeAUse(t *testing.T) {
	srv := newJoinAuthServer(t)

	for i, id := range []NodePlanIdentity{
		{Hostname: "node-a", IPs: []string{"10.0.9.10"}},
		{Hostname: "node-b", IPs: []string{"10.0.9.11"}},
		{Hostname: "node-c", IPs: []string{"10.0.9.12"}},
	} {
		resp, err := srv.requestJoinAuthorizationCore(&JoinAuthorizationRequest{
			JoinToken: "tok-v2", Identity: id, Nonce: id.Hostname,
		})
		if err != nil {
			t.Fatalf("authorization for %s: %v", id.Hostname, err)
		}
		if !resp.Allowed {
			t.Fatalf("authorization for %s denied: %q", id.Hostname, resp.DeniedReason)
		}
		if got, want := srv.state.JoinTokens["tok-v2"].Uses, i+1; got != want {
			t.Errorf("after %s: uses = %d, want %d — each distinct node costs one use", id.Hostname, got, want)
		}
	}
}

// TestRequestJoinAuthorization_ExhaustedTokenStillRefusesANewNode proves the fix
// did not turn the budget into a suggestion.
func TestRequestJoinAuthorization_ExhaustedTokenStillRefusesANewNode(t *testing.T) {
	srv := newJoinAuthServer(t)
	srv.state.JoinTokens["tok-v2"].MaxUses = 1

	if _, err := srv.requestJoinAuthorizationCore(&JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-first", IPs: []string{"10.0.9.20"}},
		Nonce:     "n1",
	}); err != nil {
		t.Fatalf("first authorization: %v", err)
	}

	_, err := srv.requestJoinAuthorizationCore(&JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-second", IPs: []string{"10.0.9.21"}},
		Nonce:     "n2",
	})
	if err == nil {
		t.Fatal("a second, DIFFERENT node was admitted on a single-use token")
	}

	// ...and the node that already holds the admission can still retry.
	retry, rerr := srv.requestJoinAuthorizationCore(&JoinAuthorizationRequest{
		JoinToken: "tok-v2",
		Identity:  NodePlanIdentity{Hostname: "node-first", IPs: []string{"10.0.9.20"}},
		Nonce:     "n3",
	})
	if rerr != nil {
		t.Fatalf("the admitted node could not retry on its own exhausted token: %v", rerr)
	}
	if !retry.Allowed {
		t.Fatalf("retry denied for the already-admitted node: %q", retry.DeniedReason)
	}
}

// ── preflight: a node must not conflict with itself ──────────────────────────

func TestSameMachineIdentity(t *testing.T) {
	cases := []struct {
		name string
		a, b storedIdentity
		want bool
	}{
		{"same host and ip", nodeIdentity("node-5", "10.10.0.15"), nodeIdentity("node-5", "10.10.0.15"), true},
		{"same host, case differs", nodeIdentity("NODE-5", "10.10.0.15"), nodeIdentity("node-5", "10.10.0.15"), true},
		{"same host, one shared ip of several", nodeIdentity("node-5", "10.10.0.15", "172.17.0.2"), nodeIdentity("node-5", "192.168.1.9", "10.10.0.15"), true},
		{"same host, no shared ip — a real collision", nodeIdentity("node-5", "10.10.0.15"), nodeIdentity("node-5", "10.10.0.99"), false},
		{"different host, shared ip", nodeIdentity("node-5", "10.10.0.15"), nodeIdentity("node-9", "10.10.0.15"), false},
		{"empty hostname never matches", storedIdentity{}, storedIdentity{}, false},
		{"loopback does not count as a shared ip", nodeIdentity("node-5", "127.0.0.1"), nodeIdentity("node-5", "127.0.0.1"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := sameMachineIdentity(tc.a, tc.b); got != tc.want {
				t.Errorf("sameMachineIdentity() = %v, want %v", got, tc.want)
			}
		})
	}
}

// TestJoinPreflight_NodeDoesNotConflictWithItself is the second half of the
// stranding bug. Once the first attempt registered the node, its own retry was
// refused as "hostname already present" — a conflict with the record it had
// just created.
func TestJoinPreflight_NodeDoesNotConflictWithItself(t *testing.T) {
	srv := admissionTestServer(t)
	srv.state.Nodes["n5"] = &nodeState{
		NodeID:             "n5",
		Identity:           nodeIdentity("node-5", "10.10.0.15"),
		JoinLifecyclePhase: JoinPhaseBootstrapping, // its own first attempt created this
	}

	jr := &joinRequestRecord{Identity: nodeIdentity("node-5", "10.10.0.15")}
	ok, reason := srv.evaluateJoinPreflightLocked(jr)
	if !ok {
		t.Errorf("a node mid-join was blocked from retrying by its own record: %s", reason)
	}
}

// TestJoinPreflight_ActiveMemberRejoinIsStillRefused pins the LIMIT of the
// self-exemption. approveJoinRecordLocked overwrites state.Nodes with a fresh
// converging/BootstrapAdmitted record, so letting an already-active member back
// through preflight would throw away its placement generation and runtime
// progress. The founding node auto-join-loops against exactly this path (96
// attempts observed 2026-08-18), and the correct outcome there is a cheap
// refusal — not a reset of the healthiest node in the cluster.
func TestJoinPreflight_ActiveMemberRejoinIsStillRefused(t *testing.T) {
	for _, phase := range []JoinLifecyclePhase{
		JoinPhaseActive, JoinPhaseAdmitted, JoinPhaseConverging, "",
	} {
		srv := admissionTestServer(t)
		srv.state.Nodes["n1"] = &nodeState{
			NodeID:             "n1",
			Identity:           nodeIdentity("node-1", "10.10.0.11"),
			JoinLifecyclePhase: phase,
		}

		jr := &joinRequestRecord{Identity: nodeIdentity("node-1", "10.10.0.11")}
		if ok, _ := srv.evaluateJoinPreflightLocked(jr); ok {
			t.Errorf("phase %q: an already-established member was allowed back through preflight — "+
				"approval would reset its node state", phase)
		}
	}
}

// TestJoinPreflight_DifferentMachineSameHostnameStillConflicts keeps the check
// doing its job: a second machine claiming an existing name shares no address
// with it and is still refused.
func TestJoinPreflight_DifferentMachineSameHostnameStillConflicts(t *testing.T) {
	srv := admissionTestServer(t)
	srv.state.Nodes["n5"] = &nodeState{
		NodeID:   "n5",
		Identity: nodeIdentity("node-5", "10.10.0.15"),
	}

	jr := &joinRequestRecord{Identity: nodeIdentity("node-5", "10.10.0.99")}
	if ok, _ := srv.evaluateJoinPreflightLocked(jr); ok {
		t.Error("a different machine claiming an existing hostname was allowed through preflight")
	}
}

// TestJoinPreflight_SameIPDifferentHostnameStillConflicts — the IP arm of the
// check must survive too.
func TestJoinPreflight_SameIPDifferentHostnameStillConflicts(t *testing.T) {
	srv := admissionTestServer(t)
	srv.state.Nodes["n5"] = &nodeState{
		NodeID:   "n5",
		Identity: nodeIdentity("node-5", "10.10.0.15"),
	}

	jr := &joinRequestRecord{Identity: nodeIdentity("node-9", "10.10.0.15")}
	if ok, _ := srv.evaluateJoinPreflightLocked(jr); ok {
		t.Error("a node reusing another node's IP was allowed through preflight")
	}
}
