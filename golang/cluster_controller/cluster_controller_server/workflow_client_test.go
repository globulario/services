package main

import "testing"

// The defect this file ratchets: the controller resolved ONE workflow address at
// boot, dialled it, assigned srv.workflowClient and never re-resolved. Every
// caller guarded with `if srv.workflowClient == nil`, which stays false once the
// field is set — so after that instance died the guard kept passing and dispatch
// went into a dead connection. A nil-check that only proves "we dialled
// something once" is not a liveness check.
//
// The failover policy is the substance of the fix, so it lives in a pure
// function and is tested here without a cluster. The dial/cache path needs a
// live registry and is exercised in integration, not here.

func TestOrderWorkflowCandidates_PrefersRunningLocalThenRunningRemote(t *testing.T) {
	got := orderWorkflowCandidates([]wfCandidate{
		{addr: "10.0.0.9:10220", local: false, running: false},
		{addr: "10.0.0.8:10220", local: false, running: true},
		{addr: "10.0.0.63:10220", local: true, running: true},
		{addr: "10.0.0.20:10220", local: true, running: false},
	})
	want := []string{
		"10.0.0.63:10220", // running + local — best
		"10.0.0.8:10220",  // running + remote — failover target
		"10.0.0.20:10220", // not running + local — last resort
		"10.0.0.9:10220",  // not running + remote
	}
	if len(got) != len(want) {
		t.Fatalf("got %d candidates %v, want %d %v", len(got), got, len(want), want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("position %d = %q, want %q (full order: %v)", i, got[i], want[i], got)
		}
	}
}

// A remote RUNNING instance must outrank a local DOWN one. This is the ordering
// that makes failover work: preferring locality over health would keep sending
// dispatch at the dead local instance, which is the original bug wearing a
// different hat.
func TestOrderWorkflowCandidates_HealthOutranksLocality(t *testing.T) {
	got := orderWorkflowCandidates([]wfCandidate{
		{addr: "local-down:10220", local: true, running: false},
		{addr: "remote-up:10220", local: false, running: true},
	})
	if len(got) == 0 || got[0] != "remote-up:10220" {
		t.Fatalf("first candidate = %v, want remote-up:10220 — a running remote must beat a down local", got)
	}
}

func TestOrderWorkflowCandidates_Deduplicates(t *testing.T) {
	got := orderWorkflowCandidates([]wfCandidate{
		{addr: "10.0.0.63:10220", local: true, running: true},
		{addr: "10.0.0.63:10220", local: true, running: true},
		{addr: "10.0.0.63:10220", local: false, running: false},
	})
	if len(got) != 1 {
		t.Fatalf("got %v, want exactly one entry — the same address must not be dialled twice", got)
	}
}

func TestOrderWorkflowCandidates_EmptyMeansUnreachable(t *testing.T) {
	// No candidates must yield no addresses, so getWorkflowClient returns nil and
	// callers see "unreachable" — never a stale client left over from boot.
	if got := orderWorkflowCandidates(nil); len(got) != 0 {
		t.Fatalf("got %v, want empty", got)
	}
}

// The override seam: tests inject a fake through srv.workflowClient, and
// getWorkflowClient must return it untouched rather than trying to resolve a
// real instance. Production leaves the field nil so resolution always runs.
func TestGetWorkflowClient_ExplicitOverrideWins(t *testing.T) {
	fake := &fakeDriftWorkflowClient{}
	srv := &server{workflowClient: fake}
	if got := srv.getWorkflowClient(); got != fake {
		t.Fatalf("getWorkflowClient() = %v, want the injected override %v", got, fake)
	}
}
