package main

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// startFakeScyllaREST starts a test HTTP server simulating the ScyllaDB REST API.
// Returns the server and sets scyllaRESTPort so callScyllaRemoveNode hits it.
// Caller must call cleanup() when done.
func startFakeScyllaREST(handler http.HandlerFunc) (host string, cleanup func()) {
	srv := httptest.NewServer(handler)
	parts := strings.SplitN(strings.TrimPrefix(srv.URL, "http://"), ":", 2)
	scyllaRESTPort = parts[1]
	return parts[0], srv.Close
}

func TestCriticalScyllaKeyspacesIncludesRepository(t *testing.T) {
	found := false
	for _, ks := range criticalScyllaKeyspaces {
		if ks == "repository" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("criticalScyllaKeyspaces must include repository for RF policy enforcement")
	}
}

// TestTryRemoveScyllaGhostVoters_SkipsWhenFewHealthyVoters verifies that
// Scylla group0 ghost-voter auto-removal is blocked when too few healthy voters
// would remain after removal.
func TestTryRemoveScyllaGhostVoters_SkipsWhenFewHealthyVoters(t *testing.T) {
	called := false
	host, cleanup := startFakeScyllaREST(func(w http.ResponseWriter, r *http.Request) {
		called = true
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	// 2 total voters: 1 stale + 1 healthy → healthyVoters=1 < 3 → skip
	preflight := DDLPreflightResult{
		OK:     false,
		Reason: DDLPreflightGroup0StaleVoter,
		Group0: &Group0View{
			TotalVoters: 2,
			StaleVoters: 1,
			Voters: []Group0Voter{
				{ServerID: "ghost-uuid", StaleReason: "not_in_gossip"},
				{ServerID: "live-uuid", InGossip: true},
			},
		},
	}
	tryRemoveScyllaGhostVoters(context.Background(), preflight, host)
	if called {
		t.Fatal("removenode REST call must not be made when healthy voters < 3")
	}
}

// TestTryRemoveScyllaGhostVoters_RemovesNotInGossipOnly verifies that only
// not_in_gossip voters are auto-removed; can_vote=false voters are left alone.
func TestTryRemoveScyllaGhostVoters_RemovesNotInGossipOnly(t *testing.T) {
	var removed []string
	host, cleanup := startFakeScyllaREST(func(w http.ResponseWriter, r *http.Request) {
		removed = append(removed, r.URL.Query().Get("host_id"))
		w.WriteHeader(http.StatusOK)
	})
	defer cleanup()

	cv := false
	preflight := DDLPreflightResult{
		OK:     false,
		Reason: DDLPreflightGroup0StaleVoter,
		Group0: &Group0View{
			TotalVoters: 5,
			StaleVoters: 2,
			Voters: []Group0Voter{
				{ServerID: "ghost-uuid", StaleReason: "not_in_gossip"},
				{ServerID: "suspended-uuid", StaleReason: "can_vote=false", CanVote: &cv, InGossip: true},
				{ServerID: "live-1", InGossip: true},
				{ServerID: "live-2", InGossip: true},
				{ServerID: "live-3", InGossip: true},
			},
		},
	}
	tryRemoveScyllaGhostVoters(context.Background(), preflight, host)

	if len(removed) != 1 || removed[0] != "ghost-uuid" {
		t.Fatalf("expected only ghost-uuid to be removed; got %v", removed)
	}
}

// TestCallScyllaRemoveNode_PropagatesRestError verifies that a non-2xx HTTP
// status from the ScyllaDB REST API is returned as an error.
func TestCallScyllaRemoveNode_PropagatesRestError(t *testing.T) {
	host, cleanup := startFakeScyllaREST(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "node not found in ring", http.StatusNotFound)
	})
	defer cleanup()

	err := callScyllaRemoveNode(context.Background(), host, "some-uuid")
	if err == nil {
		t.Fatal("expected error on 404 response")
	}
	if !strings.Contains(err.Error(), "404") {
		t.Fatalf("expected 404 in error, got: %v", err)
	}
}

// TestCallScyllaRemoveNode_SendsCorrectRequest verifies the URL and method.
func TestCallScyllaRemoveNode_SendsCorrectRequest(t *testing.T) {
	var gotMethod, gotPath, gotHostID string
	host, cleanup := startFakeScyllaREST(func(w http.ResponseWriter, r *http.Request) {
		gotMethod = r.Method
		gotPath = r.URL.Path
		gotHostID = r.URL.Query().Get("host_id")
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "ok")
	})
	defer cleanup()

	if err := callScyllaRemoveNode(context.Background(), host, "dead-uuid-1234"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gotMethod != http.MethodPost {
		t.Errorf("expected POST, got %s", gotMethod)
	}
	if gotPath != "/storage_service/remove_node" {
		t.Errorf("expected /storage_service/remove_node, got %s", gotPath)
	}
	if gotHostID != "dead-uuid-1234" {
		t.Errorf("expected host_id=dead-uuid-1234, got %s", gotHostID)
	}
}
