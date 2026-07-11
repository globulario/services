package main

// D1a — no NodeAssignment may be accepted until EVERY semantic in that structure
// is consumed by the resolver. Both the dormant explicit-placement GRANT and the
// dormant per-node version override are unconsumed today, so ANY non-empty
// NodeAssignments is HARD-REJECTED at the write gate (the canonical
// applyServiceRelease choke point) — accepted-but-ignored operator intent is the
// dangerous state. Version override is explicitly out of D1 scope and stays
// rejected even after grants are enabled.

import (
	"testing"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestValidateServiceReleaseSpec_RejectsAnyNodeAssignment(t *testing.T) {
	const nid = "681710ee-6966-5df3-b155-3cef8b4e1a96"
	cases := map[string]*cluster_controllerpb.ServiceReleaseSpec{
		"explicit placement grant": {
			PublisherID: "core@globular.io", ServiceName: "media",
			NodeAssignments: []*cluster_controllerpb.NodeAssignment{
				{NodeID: nid, Placement: cluster_controllerpb.NodeAssignmentPlacementGrant},
			},
		},
		"bare version override (out of D1 scope, still rejected)": {
			PublisherID: "core@globular.io", ServiceName: "media",
			NodeAssignments: []*cluster_controllerpb.NodeAssignment{
				{NodeID: nid, Version: "1.2.272"},
			},
		},
		"nil-only entry": {
			PublisherID: "core@globular.io", ServiceName: "media",
			NodeAssignments: []*cluster_controllerpb.NodeAssignment{nil},
		},
	}
	for name, spec := range cases {
		err := validateServiceReleaseSpec(spec)
		if err == nil {
			t.Errorf("%s: any non-empty node_assignments must be rejected (accepted-but-ignored intent is the dangerous state)", name)
			continue
		}
		if status.Code(err) != codes.InvalidArgument {
			t.Errorf("%s: rejection must be InvalidArgument, got %v", name, status.Code(err))
		}
	}
}

func TestValidateServiceReleaseSpec_AcceptsNoAssignments(t *testing.T) {
	cases := map[string]*cluster_controllerpb.ServiceReleaseSpec{
		"nil spec":          nil,
		"empty spec":        {PublisherID: "core@globular.io", ServiceName: "media"},
		"empty assignments": {PublisherID: "core@globular.io", ServiceName: "media", NodeAssignments: []*cluster_controllerpb.NodeAssignment{}},
	}
	for name, spec := range cases {
		if err := validateServiceReleaseSpec(spec); err != nil {
			t.Errorf("%s: must be accepted (no node_assignments present), got %v", name, err)
		}
	}
}
