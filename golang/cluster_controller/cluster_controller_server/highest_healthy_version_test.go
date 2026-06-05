package main

import (
	"testing"
	"time"
)

// TestHighestHealthyInstalledVersion_FreshAndStaleSplit pins
// meta.absence_scope_must_be_explicit + meta.authority_must_express_uncertainty
// for the version-regression guard. The previous single-string return
// conflated "no fresh node is running this service" (legitimate ""; caller
// safely allows downgrade) with "no node has a fresh heartbeat" (zero
// observability; caller MUST refuse). Now: (version, observable bool).
func TestHighestHealthyInstalledVersion_FreshAndStaleSplit(t *testing.T) {
	now := time.Now()
	freshAgo := now.Add(-2 * time.Minute)
	staleAgo := now.Add(-30 * time.Minute) // > 10min threshold

	srv := &server{state: newControllerState()}
	srv.state.Nodes = map[string]*nodeState{
		"fresh-with-svc": {
			NodeID:            "fresh-with-svc",
			LastSeen:          freshAgo,
			InstalledVersions: map[string]string{"echo": "1.2.3"},
		},
		"fresh-without-svc": {
			NodeID:            "fresh-without-svc",
			LastSeen:          freshAgo,
			InstalledVersions: map[string]string{"other": "9.9.9"},
		},
		"stale-with-newer": {
			NodeID:            "stale-with-newer",
			LastSeen:          staleAgo,
			InstalledVersions: map[string]string{"echo": "9.9.9"}, // newer but stale → ignored
		},
	}

	ver, observable := srv.highestHealthyInstalledVersion("echo")
	if !observable {
		t.Fatalf("observable = false; want true (fresh-with-svc + fresh-without-svc both report)")
	}
	if ver != "1.2.3" {
		t.Errorf("ver = %q; want 1.2.3 (stale 9.9.9 must be excluded — meta.absence_scope_must_be_explicit)", ver)
	}
}

// TestHighestHealthyInstalledVersion_AllStaleRefusesObservation pins the
// refuse-on-blackout gate: when every node is stale, the function must
// return observable=false so the caller knows it has zero current view
// of the cluster and refuses the downgrade-versus-not decision.
func TestHighestHealthyInstalledVersion_AllStaleRefusesObservation(t *testing.T) {
	staleAgo := time.Now().Add(-30 * time.Minute)

	srv := &server{state: newControllerState()}
	srv.state.Nodes = map[string]*nodeState{
		"stale-a": {
			NodeID:            "stale-a",
			LastSeen:          staleAgo,
			InstalledVersions: map[string]string{"echo": "1.0.0"},
		},
		"stale-b": {
			NodeID:            "stale-b",
			LastSeen:          staleAgo,
			InstalledVersions: map[string]string{"echo": "2.0.0"},
		},
	}

	ver, observable := srv.highestHealthyInstalledVersion("echo")
	if observable {
		t.Fatalf("observable = true; want false when every node is stale > 10min")
	}
	if ver != "" {
		t.Errorf("ver = %q; want empty when not observable", ver)
	}
}

// TestHighestHealthyInstalledVersion_FreshButNoServiceIsObservable pins
// the legitimate "" case: fresh heartbeats exist but no fresh node runs
// the queried service. observable=true means caller can safely allow a
// fresh deploy (no version to regress against). This contrasts with the
// all-stale case which returns observable=false.
func TestHighestHealthyInstalledVersion_FreshButNoServiceIsObservable(t *testing.T) {
	freshAgo := time.Now().Add(-2 * time.Minute)

	srv := &server{state: newControllerState()}
	srv.state.Nodes = map[string]*nodeState{
		"fresh": {
			NodeID:            "fresh",
			LastSeen:          freshAgo,
			InstalledVersions: map[string]string{"other": "1.0.0"},
		},
	}

	ver, observable := srv.highestHealthyInstalledVersion("echo")
	if !observable {
		t.Errorf("observable = false; want true (fresh node exists, just no echo)")
	}
	if ver != "" {
		t.Errorf("ver = %q; want empty (no fresh node runs echo)", ver)
	}
}
