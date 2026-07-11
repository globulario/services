package main

// D1a — explicit per-node placement grants are HARD-REJECTED until the resolver
// consumes them (D1b). Accepted-but-ignored operator intent is the dangerous
// state, so a ServiceReleaseSpec carrying a placement=GRANT node_assignment must
// be refused at the write gate — not persisted, not warned, not kept for later.

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestRejectUnconsumedGrants_RejectsGrant(t *testing.T) {
	spec := &cluster_controllerpb.ServiceReleaseSpec{
		PublisherID: "core@globular.io",
		ServiceName: "media",
		NodeAssignments: []*cluster_controllerpb.NodeAssignment{
			{
				NodeID:    "681710ee-6966-5df3-b155-3cef8b4e1a96",
				Placement: cluster_controllerpb.NodeAssignmentPlacementGrant,
			},
		},
	}
	err := rejectUnconsumedGrants(spec)
	if err == nil {
		t.Fatal("a GRANT node_assignment must be rejected (accepted-but-ignored intent is the dangerous state)")
	}
	if status.Code(err) != codes.InvalidArgument {
		t.Fatalf("grant rejection must be InvalidArgument, got %v (%v)", status.Code(err), err)
	}
}

func TestRejectUnconsumedGrants_AcceptsNonGrant(t *testing.T) {
	cases := map[string]*cluster_controllerpb.ServiceReleaseSpec{
		"nil spec":   nil,
		"empty spec": {PublisherID: "core@globular.io", ServiceName: "media"},
		"bare version override (no grant)": {
			PublisherID: "core@globular.io",
			ServiceName: "media",
			NodeAssignments: []*cluster_controllerpb.NodeAssignment{
				// Placement == "" — a legacy per-node version override, NOT an
				// explicit placement grant. D1a rejects only GRANT.
				{NodeID: "681710ee-6966-5df3-b155-3cef8b4e1a96", Version: "1.2.272"},
			},
		},
	}
	for name, spec := range cases {
		if err := rejectUnconsumedGrants(spec); err != nil {
			t.Errorf("%s: must be accepted (no grant present), got %v", name, err)
		}
	}
}
