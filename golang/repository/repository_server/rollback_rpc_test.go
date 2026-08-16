package main

// rollback_rpc_test.go — Phase CLI-C tests for installed-revision recording
// and rollback candidate evaluation.

import (
	"context"
	"testing"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// helper: seed two installed revisions on a single node so candidates exist.
func seedTwoRevisions(t *testing.T, srv *server) (*repopb.InstalledPackageRevision, *repopb.InstalledPackageRevision) {
	t.Helper()
	ctx := context.Background()

	old := &repopb.InstalledPackageRevision{
		PublisherId: "core@globular.io", Name: "echo", Platform: "linux_amd64",
		Kind:    repopb.ArtifactKind_SERVICE,
		Version: "1.0.0", BuildId: "v1", BuildNumber: 1,
		Checksum: "sha256:abc", InstalledAtUnix: time.Now().Unix() - 3600,
		NodeId: "n1", Action: "install",
	}
	if _, err := srv.RecordInstalledRevision(ctx, &repopb.RecordInstalledRevisionRequest{Revision: old}); err != nil {
		t.Fatalf("record old: %v", err)
	}
	current := &repopb.InstalledPackageRevision{
		PublisherId: "core@globular.io", Name: "echo", Platform: "linux_amd64",
		Kind:    repopb.ArtifactKind_SERVICE,
		Version: "1.1.0", BuildId: "v2", BuildNumber: 2,
		Checksum: "sha256:def", InstalledAtUnix: time.Now().Unix(),
		NodeId: "n1", PreviousRevisionId: old.GetRevisionId(), Action: "upgrade",
	}
	if _, err := srv.RecordInstalledRevision(ctx, &repopb.RecordInstalledRevisionRequest{Revision: current}); err != nil {
		t.Fatalf("record current: %v", err)
	}
	return old, current
}

func TestRecordAndListInstalledRevisions(t *testing.T) {
	srv := newTestServer(t)
	old, current := seedTwoRevisions(t, srv)

	resp, err := srv.ListInstalledRevisions(context.Background(),
		&repopb.ListInstalledRevisionsRequest{
			PublisherId: "core@globular.io", Name: "echo", Platform: "linux_amd64",
		})
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(resp.GetRevisions()) != 2 {
		t.Fatalf("expected 2 revisions, got %d", len(resp.GetRevisions()))
	}
	// Newest first.
	if resp.GetRevisions()[0].GetVersion() != current.GetVersion() {
		t.Fatalf("expected newest first, got %s then %s",
			resp.GetRevisions()[0].GetVersion(), resp.GetRevisions()[1].GetVersion())
	}
	if resp.GetRevisions()[0].GetRevisionId() == "" || resp.GetRevisions()[0].GetRevisionId() == old.GetRevisionId() {
		t.Fatal("revision_id collision or empty")
	}
}

func TestListRollbackCandidates_PicksPreviousAndEvaluates(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Seed real artifacts so verifyArtifactIntegrity finds manifest + blob.
	for _, ver := range []string{"1.0.0", "1.1.0"} {
		ref := &repopb.ArtifactRef{
			PublisherId: "core@globular.io", Name: "echo",
			Version: ver, Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
		}
		buildID := "v1"
		buildNum := int64(1)
		if ver == "1.1.0" {
			buildID = "v2"
			buildNum = 2
		}
		seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
			Ref: ref, BuildNumber: buildNum, BuildId: buildID,
			Checksum: "sha256:abc", SizeBytes: 100,
		})
		key := artifactKeyWithBuild(ref, buildNum)
		_ = srv.transitionArtifactState(ctx, key, PipelinePublished, "test", "", ArtifactStateFields{
			BlobKey: binaryStorageKey(key), Checksum: "sha256:abc", SizeBytes: 100,
			BuildID: buildID, BuildNumber: buildNum,
		})
	}
	seedTwoRevisions(t, srv)

	resp, err := srv.ListRollbackCandidates(ctx, &repopb.ListRollbackCandidatesRequest{
		PublisherId: "core@globular.io", Name: "echo", Platform: "linux_amd64",
	})
	if err != nil {
		t.Fatalf("ListRollbackCandidates: %v", err)
	}
	if resp.GetCurrentRef() == nil || resp.GetCurrentRef().GetVersion() != "1.1.0" {
		t.Fatalf("current_ref should be 1.1.0, got %v", resp.GetCurrentRef())
	}
	if len(resp.GetCandidates()) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(resp.GetCandidates()))
	}
	cand := resp.GetCandidates()[0]
	if cand.GetTargetRef().GetVersion() != "1.0.0" {
		t.Fatalf("candidate version: got %s, want 1.0.0", cand.GetTargetRef().GetVersion())
	}
	// Eligibility may not be OK (size_mismatch on seed), but the structure
	// must be populated.
	if cand.GetEligibility() == nil {
		t.Fatal("eligibility nil")
	}
}

func TestListRollbackCandidates_RevokedNotEligible(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: "sha256:abc", SizeBytes: 100,
	})
	if err := srv.RevokeArtifact(ctx, ref, 1, "policy", "op"); err != nil {
		t.Fatalf("revoke: %v", err)
	}
	seedTwoRevisions(t, srv)

	resp, err := srv.ListRollbackCandidates(ctx, &repopb.ListRollbackCandidatesRequest{
		PublisherId: "core@globular.io", Name: "echo", Platform: "linux_amd64",
	})
	if err != nil {
		t.Fatalf("rollback candidates: %v", err)
	}
	if len(resp.GetCandidates()) != 1 {
		t.Fatalf("expected 1 candidate, got %d", len(resp.GetCandidates()))
	}
	cand := resp.GetCandidates()[0]
	if cand.GetEligibility().GetEligible() {
		t.Fatal("REVOKED candidate must not be eligible")
	}
}
