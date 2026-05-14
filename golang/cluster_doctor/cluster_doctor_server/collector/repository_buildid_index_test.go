package collector

// repository_buildid_index_test.go — Regression for the v1.2.48 doctor gap.
//
// Failure mode this test pins:
//
//   • repository.desired_build_ids_resolve consults Snapshot.RepositoryBuildIDIndex
//     to decide whether a desired build_id is "resolvable in the repository."
//   • The collector built the index from ListArtifacts results without filtering
//     publish_state. ListArtifacts returns ALL rows to admin callers (the doctor
//     authenticates with the service mTLS cert and reaches the admin code path
//     in artifact_handlers.go ListArtifacts), so YANKED / REVOKED / ARCHIVED /
//     CORRUPTED build_ids leaked into the index.
//   • A desired pin to a demoted build_id then looked "resolved" → the rule
//     stayed silent on a real orphan. Observed live on storage (build_id
//     801c0043-…) and node-agent (build_id fe08cd6a-…) after the v1.2.48
//     deploy on 2026-05-14.
//
// The fix filters the index through repopb.IsInstallableByPin so only
// PUBLISHED and DEPRECATED rows pass through. This test exercises that
// predicate directly against the canonical PublishState values so the next
// time someone adds a new lifecycle state they will see this test fail
// before the rule goes silent in prod.

import (
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestBuildIDIndex_OnlyInstallableStatesAdmitted(t *testing.T) {
	// One artifact per terminal-ish publish_state. Only PUBLISHED and
	// DEPRECATED are installable by pin (IsInstallableByPin contract);
	// the rest are demoted in one way or another and MUST NOT pass.
	cases := []struct {
		state repopb.PublishState
		bid   string
		want  bool
	}{
		{repopb.PublishState_PUBLISHED, "bid-published", true},
		{repopb.PublishState_DEPRECATED, "bid-deprecated", true},

		{repopb.PublishState_YANKED, "bid-yanked", false},
		{repopb.PublishState_REVOKED, "bid-revoked", false},
		{repopb.PublishState_ARCHIVED, "bid-archived", false},
		{repopb.PublishState_QUARANTINED, "bid-quarantined", false},
		{repopb.PublishState_CORRUPTED, "bid-corrupted", false},

		{repopb.PublishState_STAGING, "bid-staging", false},
		{repopb.PublishState_VERIFIED, "bid-verified", false},
		{repopb.PublishState_ORPHANED, "bid-orphaned", false},
		{repopb.PublishState_FAILED, "bid-failed", false},
	}

	in := make([]*repopb.ArtifactManifest, 0, len(cases))
	for _, c := range cases {
		in = append(in, &repopb.ArtifactManifest{
			BuildId:      c.bid,
			PublishState: c.state,
		})
	}

	got := buildIDIndexFromManifests(in)

	for _, c := range cases {
		if _, present := got[c.bid]; present != c.want {
			t.Errorf("state=%s build_id=%s present=%v want=%v — demoted states must not pollute the index", c.state, c.bid, present, c.want)
		}
	}
}

func TestBuildIDIndex_NilAndEmptyBuildIDIgnored(t *testing.T) {
	// Defensive: nil entries and rows without a build_id must not panic
	// or appear in the index. The collector iterates ListArtifacts which
	// can legitimately contain incomplete rows during a publish-in-flight.
	in := []*repopb.ArtifactManifest{
		nil,
		{PublishState: repopb.PublishState_PUBLISHED}, // empty build_id
		{BuildId: "bid-1", PublishState: repopb.PublishState_PUBLISHED},
	}
	got := buildIDIndexFromManifests(in)
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 entry, got %d: %+v", len(got), got)
	}
	if !got["bid-1"] {
		t.Errorf("bid-1 missing from index: %+v", got)
	}
}

func TestBuildIDIndex_DocumentedOrphans_StayOutOfIndex(t *testing.T) {
	// Documents the v1.2.48 live incident: storage build_id 801c0043-… and
	// node-agent build_id fe08cd6a-… were demoted in the repository while
	// desired state still pinned them. If they had been YANKED but the
	// admin-visible ListArtifacts surfaced them, the pre-fix collector
	// would have admitted them and the rule would have stayed silent.
	//
	// The fix: regardless of admin visibility, demoted rows do not enter
	// the index. This test pins that.
	in := []*repopb.ArtifactManifest{
		{BuildId: "801c0043-0b34-41f9-b933-cbcf19ec5aa9", PublishState: repopb.PublishState_YANKED},
		{BuildId: "fe08cd6a-ae2b-498d-826f-3b5aac95e26f", PublishState: repopb.PublishState_REVOKED},
		{BuildId: "bid-good", PublishState: repopb.PublishState_PUBLISHED},
	}
	got := buildIDIndexFromManifests(in)
	if got["801c0043-0b34-41f9-b933-cbcf19ec5aa9"] {
		t.Error("YANKED storage build_id leaked into index — the live v1.2.48 bug")
	}
	if got["fe08cd6a-ae2b-498d-826f-3b5aac95e26f"] {
		t.Error("REVOKED node-agent build_id leaked into index — the live v1.2.48 bug")
	}
	if !got["bid-good"] {
		t.Error("PUBLISHED build_id must remain in index")
	}
}
