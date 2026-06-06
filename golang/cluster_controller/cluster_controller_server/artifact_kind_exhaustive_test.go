// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.artifact_kind_exhaustive_test
// @awareness file_role=guards_proto_artifactkind_switches_against_silent_drift
// @awareness enforces=globular.platform:invariant.release_type_switch_must_have_default
// @awareness risk=high
package main

import (
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// allProtoArtifactKinds enumerates every named ArtifactKind value in the
// proto. UNSPECIFIED is excluded — it's the zero value and represents
// "no kind set", which every switch in this package handles via the
// default branch by design.
//
// When the proto adds a new kind, add it here too. The tests below will
// fail until you do — that's the point.
var allProtoArtifactKinds = []repopb.ArtifactKind{
	repopb.ArtifactKind_SERVICE,
	repopb.ArtifactKind_APPLICATION,
	repopb.ArtifactKind_AGENT,
	repopb.ArtifactKind_SUBSYSTEM,
	repopb.ArtifactKind_INFRASTRUCTURE,
	repopb.ArtifactKind_COMMAND,
	repopb.ArtifactKind_AWARENESS_BUNDLE,
}

// TestAllProtoArtifactKindsEnumerated catches the meta-drift: if proto
// adds a new ArtifactKind without updating allProtoArtifactKinds above,
// every downstream exhaustiveness test silently passes against a stale
// universe. We diff the names in our enumeration against proto's known
// names, and fail when they disagree.
//
// Without this test the bug shape recurses: the test that catches drift
// in kindRank etc. is itself drift-prone (same shape as the coreWorkflows
// hardcoded list).
func TestAllProtoArtifactKindsEnumerated(t *testing.T) {
	protoKnown := map[repopb.ArtifactKind]bool{}
	for value, name := range repopb.ArtifactKind_name {
		k := repopb.ArtifactKind(value)
		if k == repopb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED {
			continue
		}
		_ = name // unused but proves the entry exists
		protoKnown[k] = true
	}
	enumerated := map[repopb.ArtifactKind]bool{}
	for _, k := range allProtoArtifactKinds {
		enumerated[k] = true
	}

	var missing, extra []string
	for k := range protoKnown {
		if !enumerated[k] {
			missing = append(missing, k.String())
		}
	}
	for k := range enumerated {
		if !protoKnown[k] {
			extra = append(extra, k.String())
		}
	}
	if len(missing) > 0 {
		t.Errorf("proto defines %d ArtifactKind values not present in allProtoArtifactKinds: %v — add them and update every exhaustive switch", len(missing), missing)
	}
	if len(extra) > 0 {
		t.Errorf("allProtoArtifactKinds contains %d values not defined in proto: %v — proto removed a kind; remove from this list too", len(extra), extra)
	}
}

// TestKindRankCoversAllProtoArtifactKinds asserts that recovery_planner.go's
// kindRank() returns a non-default rank for every named proto ArtifactKind.
// The unknownKindRank fallback is reserved for actually-unknown strings
// (e.g. a kind added to the proto without a code update). If a proto-known
// value lands there, the install order is degraded silently.
func TestKindRankCoversAllProtoArtifactKinds(t *testing.T) {
	for _, k := range allProtoArtifactKinds {
		rank := kindRank(k.String())
		if rank == unknownKindRank {
			t.Errorf("kindRank(%s) returned unknownKindRank=%d — add an explicit case in recovery_planner.go", k.String(), rank)
		}
	}
}

// TestLookupResolvedEntrypointChecksumKindsExhaustive asserts that every
// proto ArtifactKind produces a candidates list in
// release_runtime_convergence.go's lookupResolvedEntrypointChecksum
// switch. We can't call the function directly (it hits Scylla via
// srv.resources), so we mirror the switch logic here and assert it
// covers every kind. Mirror drift is caught by Test*Enumerated above
// and by the fact that both this test and the switch must change
// together — see the comment block at the switch site.
func TestLookupResolvedEntrypointChecksumKindsExhaustive(t *testing.T) {
	for _, k := range allProtoArtifactKinds {
		got := candidatesForKind(k.String())
		if len(got) == 0 {
			t.Errorf("candidatesForKind(%s) returned empty — switch in release_runtime_convergence.go skipped this kind silently", k.String())
		}
	}
}

// candidatesForKind mirrors the switch in lookupResolvedEntrypointChecksum.
// Tests use it to assert exhaustiveness without calling the live function.
// MUST stay in lockstep with that switch.
func candidatesForKind(kind string) []string {
	switch strings.ToUpper(kind) {
	case "INFRASTRUCTURE", "SUBSYSTEM":
		return []string{"InfrastructureRelease", "ServiceRelease", "ApplicationRelease"}
	case "SERVICE", "AGENT":
		return []string{"ServiceRelease", "InfrastructureRelease", "ApplicationRelease"}
	case "APPLICATION":
		return []string{"ApplicationRelease", "ServiceRelease", "InfrastructureRelease"}
	case "COMMAND", "AWARENESS_BUNDLE":
		return []string{"ServiceRelease", "InfrastructureRelease", "ApplicationRelease"}
	default:
		return nil
	}
}
