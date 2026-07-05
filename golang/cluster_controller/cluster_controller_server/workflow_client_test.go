package main

import (
	"reflect"
	"testing"
)

// TestOrderWorkflowCandidates pins the failover preference: a RUNNING instance
// on the LOCAL node is tried first, then running remotes, then non-running as a
// last resort — so the controller reaches a healthy workflow instance and rolls
// over when one dies, without ever depending on the mesh.
func TestOrderWorkflowCandidates(t *testing.T) {
	in := []wfCandidate{
		{addr: "10.0.0.9:12100", local: false, running: false},
		{addr: "10.0.0.8:12100", local: false, running: true},
		{addr: "10.0.0.63:12100", local: true, running: false},
		{addr: "10.0.0.63:12100", local: true, running: false}, // duplicate — must collapse
		{addr: "10.0.0.20:12100", local: true, running: true},
	}
	got := orderWorkflowCandidates(in)
	want := []string{
		"10.0.0.20:12100", // running + local
		"10.0.0.8:12100",  // running + remote
		"10.0.0.63:12100", // non-running + local
		"10.0.0.9:12100",  // non-running + remote
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("orderWorkflowCandidates order = %v, want %v", got, want)
	}
}

// TestOrderWorkflowCandidates_Empty returns nil for no candidates.
func TestOrderWorkflowCandidates_Empty(t *testing.T) {
	if got := orderWorkflowCandidates(nil); got != nil {
		t.Fatalf("expected nil for no candidates, got %v", got)
	}
}
