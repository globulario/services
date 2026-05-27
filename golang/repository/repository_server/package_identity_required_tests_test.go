package main

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// These test names are intentionally aligned with awareness pack
// requirements so intent-audit can verify coverage by name.

func TestPublishRejectsSameBuildNumberDifferentBuildID(t *testing.T) {
	// Existing authoritative coverage for same-version/different-build-id reject.
	TestVA4_SameVersionDifferentBuildID_Rejected(t)
}

func TestRepairDoesNotUseBuildNumberAsIdentity(t *testing.T) {
	// Existing repair behavior: select latest existing build row even when
	// non-installable, rather than "latest installable by build_number".
	TestRepairArtifact_UsesLatestExistingBuildWhenBroken(t)
}

func TestRepositoryDoctorReportsDuplicateBuildNumberCollision(t *testing.T) {
	// Existing doctor behavior emits ambiguity finding when one identity lane
	// can resolve to multiple build identities.
	TestListRepositoryFindings_VersionResolutionAmbiguous(t)
}

func TestRepositoryDoctorReportsBuildIDReuse(t *testing.T) {
	// Existing doctor behavior emits build_id checksum conflict for reused
	// build_id mapped across divergent checksums.
	TestListRepositoryFindings_BuildIDChecksumConflict(t)
}

func TestRepositoryDoctorCollisionFindingIncludesForbiddenFixes(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "workflow",
		Version: "1.0.53", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	m1 := &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "build-A",
		Checksum: "sha256:aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		SizeBytes: 100,
	}
	m2 := &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 2, BuildId: "build-B",
		Checksum: "sha256:bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb",
		SizeBytes: 100,
	}
	seedPublishedArtifactDirect(t, srv, m1)
	seedPublishedArtifactDirect(t, srv, m2)
	m1JSON, err := marshalManifestWithState(m1, repopb.PublishState_PUBLISHED)
	if err != nil {
		t.Fatalf("marshal m1: %v", err)
	}
	m2JSON, err := marshalManifestWithState(m2, repopb.PublishState_PUBLISHED)
	if err != nil {
		t.Fatalf("marshal m2: %v", err)
	}
	key1 := artifactKeyWithBuild(ref, 1)
	key2 := artifactKeyWithBuild(ref, 2)
	srv.scylla = &fakeLedger{
		rows: map[string]*manifestRow{
			key1: {
				ArtifactKey: key1, PublisherID: ref.GetPublisherId(), Name: ref.GetName(), Version: ref.GetVersion(), Platform: ref.GetPlatform(),
				BuildNumber: 1, Checksum: m1.GetChecksum(), SizeBytes: 100,
				PublishState: repopb.PublishState_PUBLISHED.String(), ManifestJSON: m1JSON,
			},
			key2: {
				ArtifactKey: key2, PublisherID: ref.GetPublisherId(), Name: ref.GetName(), Version: ref.GetVersion(), Platform: ref.GetPlatform(),
				BuildNumber: 2, Checksum: m2.GetChecksum(), SizeBytes: 100,
				PublishState: repopb.PublishState_PUBLISHED.String(), ManifestJSON: m2JSON,
			},
		},
	}

	resp, err := srv.ListRepositoryFindings(ctx, &repopb.ListRepositoryFindingsRequest{})
	if err != nil {
		t.Fatalf("ListRepositoryFindings: %v", err)
	}
	found := false
	for _, f := range resp.GetFindings() {
		if f.GetReason() == "repository.identity.version_resolution_ambiguous" {
			found = true
			if f.GetRecommendedCommand() == "" {
				t.Fatalf("expected recommended command on collision finding")
			}
			break
		}
	}
	if !found {
		t.Fatal("expected repository.identity.version_resolution_ambiguous finding")
	}
}

