package main

import (
	"context"
	"strings"
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
)

// Placement intent on the LEGACY RequestJoin path.
//
// The explicit-placement contract (docs/design/placement-explicit-node-assignment-contract.md)
// says profiles are operator intent, never inferred. The legacy path had no channel
// for that intent at all: RequestJoinRequest carried no profiles field, and the
// auto-approve branch called approveJoinRecordLocked(jr, nil), so every legacy join
// fell through to deduceProfiles() and was placed by hardware thresholds alone.
//
// That is not "the operator was overruled" — the operator was never heard. It bites
// hardest where several nodes report the SAME underlying hardware: five containers on
// one host each see the host's RAM/CPU/disk, clear every threshold, and are all placed
// control-plane+core+gateway+storage. A compose file asking for core,compute got
// control-plane,core,gateway, and five ScyllaDB instances came up where two were intended.
//
// These tests pin the contract in BOTH directions: declared intent wins, and a node
// that declares nothing still gets the deduced/default fallback unchanged.

func requestJoinWithProfiles(t *testing.T, srv *server, hostname string, profiles []string) (*cluster_controllerpb.RequestJoinResponse, error) {
	t.Helper()
	return srv.RequestJoin(context.Background(), &cluster_controllerpb.RequestJoinRequest{
		JoinToken: "tok-v2",
		Identity:  &cluster_controllerpb.NodeIdentity{Hostname: hostname, Ips: []string{"10.0.9.1"}},
		// Capabilities that clear EVERY deduction threshold, so a regression that
		// ignores requested_profiles produces the maximal deduced set and the
		// assertion below fails loudly rather than coincidentally passing.
		Capabilities: &cluster_controllerpb.NodeCapabilities{
			CpuCount: 32, RamBytes: 64 << 30, DiskBytes: 2 << 40, DiskFreeBytes: 1 << 40,
		},
		RequestedProfiles: profiles,
	})
}

func joinRecordFor(srv *server, hostname string) *joinRequestRecord {
	for _, jr := range srv.state.JoinRequests {
		if jr != nil && jr.Identity.Hostname == hostname {
			return jr
		}
	}
	return nil
}

// Declared intent must survive admission and beat hardware deduction.
func TestRequestJoin_DeclaredProfilesBeatHardwareDeduction(t *testing.T) {
	srv := newJoinAuthServer(t)
	if _, err := requestJoinWithProfiles(t, srv, "node-declared", []string{"core", "compute"}); err != nil {
		t.Fatalf("RequestJoin: %v", err)
	}
	jr := joinRecordFor(srv, "node-declared")
	if jr == nil {
		t.Fatal("join request not recorded")
	}
	want := normalizeProfiles([]string{"core", "compute"})
	if !sameStrings(jr.Profiles, want) {
		t.Fatalf("declared profiles discarded: got %v, want %v — hardware deduction must not override stated intent", jr.Profiles, want)
	}
	// The maximal deduced set is what a regression would produce; assert we did NOT get it.
	for _, deducedOnly := range []string{"control-plane", "gateway", "storage"} {
		for _, got := range jr.Profiles {
			if got == deducedOnly {
				t.Fatalf("profile %q was deduced from hardware despite an explicit request for %v", deducedOnly, want)
			}
		}
	}
}

// A node that declares nothing keeps the pre-existing deduced fallback.
func TestRequestJoin_NoDeclaredProfilesStillDeduces(t *testing.T) {
	srv := newJoinAuthServer(t)
	if _, err := requestJoinWithProfiles(t, srv, "node-silent", nil); err != nil {
		t.Fatalf("RequestJoin: %v", err)
	}
	jr := joinRecordFor(srv, "node-silent")
	if jr == nil {
		t.Fatal("join request not recorded")
	}
	if len(jr.Profiles) == 0 {
		t.Fatal("a node declaring no profiles must still receive the deduced/default set")
	}
	var sawDeduced bool
	for _, p := range jr.Profiles {
		if p == "control-plane" || p == "storage" || p == "gateway" {
			sawDeduced = true
		}
	}
	if !sawDeduced {
		t.Fatalf("hardware deduction fallback regressed: got %v from capabilities clearing every threshold", jr.Profiles)
	}
}

// A typo must be rejected at admission, not silently swapped for deduction.
func TestRequestJoin_UnknownProfileRejected(t *testing.T) {
	srv := newJoinAuthServer(t)
	_, err := requestJoinWithProfiles(t, srv, "node-typo", []string{"core", "storrage"})
	if err == nil {
		t.Fatal("unknown profile must be rejected, not silently replaced by hardware deduction")
	}
	if !strings.Contains(err.Error(), "storrage") {
		t.Fatalf("rejection must name the offending profile; got %v", err)
	}
	if jr := joinRecordFor(srv, "node-typo"); jr != nil && len(jr.Profiles) > 0 {
		t.Fatalf("rejected join must not be placed; got profiles %v", jr.Profiles)
	}
}
