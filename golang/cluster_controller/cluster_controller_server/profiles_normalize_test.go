package main

import (
	"reflect"
	"testing"
	"time"
)

func TestEnforceFoundingProfiles_QuorumOnly(t *testing.T) {
	got := enforceFoundingProfiles([]string{"core"}, 0)
	want := []string{"control-plane", "core", "storage"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("enforceFoundingProfiles(core, 0) = %v, want %v", got, want)
	}
	for _, profile := range got {
		if profile == "media-server" {
			t.Fatal("enforceFoundingProfiles must not grant media-server")
		}
	}
}

func TestEnforceFoundingProfiles_PreservesExplicitMediaServer(t *testing.T) {
	got := enforceFoundingProfiles([]string{"media-server"}, 0)
	want := []string{"control-plane", "core", "media-server", "storage"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("enforceFoundingProfiles(media-server, 0) = %v, want %v", got, want)
	}
}

func TestEnforceStorageQuorumLocked_DoesNotReAddMediaServer(t *testing.T) {
	srv := newTestServer(t, newControllerState())
	now := time.Now()
	srv.state.Nodes = map[string]*nodeState{
		"n1": {
			NodeID:   "n1",
			LastSeen: now,
			Profiles: []string{"core", "control-plane", "storage"},
			Identity: storedIdentity{Hostname: "n1"},
		},
		"n2": {
			NodeID:   "n2",
			LastSeen: now.Add(-time.Minute),
			Profiles: []string{"core", "control-plane"},
			Identity: storedIdentity{Hostname: "n2"},
		},
	}

	if !srv.enforceStorageQuorumLocked() {
		t.Fatal("expected storage quorum repair to promote a node")
	}
	got := normalizeProfiles(srv.state.Nodes["n2"].Profiles)
	want := []string{"control-plane", "core", "storage"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("repaired profiles = %v, want %v", got, want)
	}
	for _, profile := range got {
		if profile == "media-server" {
			t.Fatal("storage quorum repair must not re-add media-server")
		}
	}
}
