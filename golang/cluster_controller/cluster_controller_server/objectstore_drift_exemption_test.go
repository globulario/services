package main

// Regression for the workflow.drift_stuck CRITICAL false-positive: a held
// minio/sidekick on a storage-profile node (ObjectStoreIntent.Member=true) that
// has NOT been admitted to DesiredObjectStoreMembers was reported as
// missing_package by reconcileScanDrift. The release workflow short-circuits with
// 0 targets for such a package, so the item cycled forever (35+ cycles →
// CRITICAL). The package is unadmitted/held, not absent
// (meta.absence_scope_must_be_explicit).
//
// The scanner exempts a package iff:
//   isObjectStoreTopologyGated(svc) && !nodeIsExplicitObjectStoreMember(node, desiredMembers)
//
// These tests pin that decision directly. The exemption must be NARROW
// (meta.silence_is_not_valid_for_unexpected): it must never swallow a genuinely
// missing package on an admitted member, nor a non-gated package.

import (
	"testing"

	"github.com/globulario/services/golang/workflow/workflowpb"
)

// driftExempt mirrors the exact guard inserted in reconcileScanDrift's
// missing_package emission block.
func driftExempt(svc string, node *nodeState, desiredMembers []ObjectStoreMember) bool {
	return isObjectStoreTopologyGated(svc) && !nodeIsExplicitObjectStoreMember(node, desiredMembers)
}

func TestIsObjectStoreTopologyGated(t *testing.T) {
	gated := []string{"minio", "sidekick"}
	for _, p := range gated {
		if !isObjectStoreTopologyGated(p) {
			t.Errorf("%q must be objectstore-topology-gated", p)
		}
	}
	notGated := []string{"scylladb", "etcd", "mcp", "keepalived", "globular-codex", ""}
	for _, p := range notGated {
		if isObjectStoreTopologyGated(p) {
			t.Errorf("%q must NOT be objectstore-topology-gated (only minio/sidekick are)", p)
		}
	}
}

func TestDriftExemption_HeldMemberIsExempt(t *testing.T) {
	// Storage-eligible node (intent.Member=true) NOT admitted to the pool.
	held := &nodeState{
		NodeID:            "globule-nuc",
		Profiles:          []string{"core", "storage"},
		ObjectStoreIntent: &ObjectStoreIntent{Member: true},
	}
	// v2 mode: another node is admitted, this one is held out.
	desired := []ObjectStoreMember{{NodeID: "globule-ryzen"}}

	if !driftExempt("minio", held, desired) {
		t.Error("held minio on an unadmitted storage node must be EXEMPT (unadmitted/held, not missing)")
	}
	if !driftExempt("sidekick", held, desired) {
		t.Error("held sidekick on an unadmitted storage node must be EXEMPT")
	}
}

func TestDriftExemption_AdmittedMemberStillDrifts(t *testing.T) {
	// Node admitted to the pool — a missing minio here is REAL drift.
	admitted := &nodeState{
		NodeID:            "globule-ryzen",
		Profiles:          []string{"core", "storage"},
		ObjectStoreIntent: &ObjectStoreIntent{Member: true},
	}
	desired := []ObjectStoreMember{{NodeID: "globule-ryzen"}}

	if driftExempt("minio", admitted, desired) {
		t.Error("missing minio on an ADMITTED member must NOT be exempt — it is real drift")
	}
}

func TestDriftExemption_NonGatedPackageStillDrifts(t *testing.T) {
	// A held node missing a non-gated package (e.g. scylladb) is still real drift;
	// the objectstore exemption must not leak to other packages.
	held := &nodeState{
		NodeID:            "globule-nuc",
		Profiles:          []string{"core", "storage"},
		ObjectStoreIntent: &ObjectStoreIntent{Member: true},
	}
	desired := []ObjectStoreMember{{NodeID: "globule-ryzen"}}

	if driftExempt("scylladb", held, desired) {
		t.Error("non-gated package must NOT be exempted by the objectstore rule")
	}
}

func TestDriftExemption_LegacyModeStillDrifts(t *testing.T) {
	// Legacy mode (nil desiredMembers): profile-derived membership governs, so a
	// storage node IS an explicit member and a missing minio is real drift.
	node := &nodeState{
		NodeID:            "globule-legacy",
		Profiles:          []string{"core", "storage"},
		ObjectStoreIntent: &ObjectStoreIntent{Member: true},
	}
	if driftExempt("minio", node, nil) {
		t.Error("legacy mode (nil desiredMembers) treats storage nodes as members — must NOT be exempt")
	}
}

func TestClearResolvedDriftItemsClearsWithheldTopologyRows(t *testing.T) {
	current := map[string]map[string]bool{
		"missing_package": {
			"dns@node-a": true,
		},
	}
	items := []*workflowpb.DriftUnresolved{
		{DriftType: "missing_package", EntityRef: "dns@node-a"},
		{DriftType: "missing_package", EntityRef: "minio@node-b"},
		{DriftType: "missing_package", EntityRef: "sidekick@node-b"},
	}
	var cleared []string
	clearResolvedDriftItems(current, items, func(driftType, entityRef string) {
		cleared = append(cleared, driftType+"/"+entityRef)
	})

	if len(cleared) != 2 {
		t.Fatalf("cleared %d rows, want 2: %v", len(cleared), cleared)
	}
	if cleared[0] != "missing_package/minio@node-b" || cleared[1] != "missing_package/sidekick@node-b" {
		t.Errorf("cleared=%v, want withheld minio and sidekick rows", cleared)
	}
}
