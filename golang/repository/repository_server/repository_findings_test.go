package main

// repository_findings_test.go — Phase F Part 4 repository-side findings tests.

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// fakeLedgerWithRows lets us drive ListRepositoryFindings without a real Scylla
// session by injecting a manifestLedger that returns a controlled row set.
// The test installs the fake into srv.scylla and seeds storage so blob checks
// align with the row metadata.

func TestListRepositoryFindings_PublishedMissingBlob(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Seed a published row + manifest, then delete the blob.
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: "sha256:abcd", SizeBytes: 100,
	})
	key := artifactKeyWithBuild(ref, 1)
	if err := srv.Storage().Remove(ctx, binaryStorageKey(key)); err != nil {
		t.Fatalf("delete blob: %v", err)
	}

	// Install a fake ledger so ListRepositoryFindings has a row to scan.
	srv.scylla = &fakeLedger{
		rows: map[string]*manifestRow{key: {
			ArtifactKey:  key,
			PublisherID:  "core@globular.io", Name: "echo",
			Version: "1.0.0", Platform: "linux_amd64",
			BuildNumber: 1, Checksum: "sha256:abcd", SizeBytes: 100,
			PublishState: repopb.PublishState_PUBLISHED.String(),
		}},
	}

	resp, err := srv.ListRepositoryFindings(ctx, &repopb.ListRepositoryFindingsRequest{})
	if err != nil {
		t.Fatalf("ListRepositoryFindings: %v", err)
	}
	if len(resp.GetFindings()) == 0 {
		t.Fatal("expected at least one finding for missing blob")
	}
	found := false
	for _, f := range resp.GetFindings() {
		if f.GetKind() == repopb.RepositoryFindingKind_REPO_FIND_PUBLISHED_MISSING_BLOB {
			found = true
			if f.GetSeverity() != repopb.RepositoryFindingSeverity_REPO_FIND_CRITICAL {
				t.Errorf("missing-blob severity: got %s, want CRITICAL", f.GetSeverity())
			}
			if f.GetRecommendedCommand() == "" {
				t.Error("recommended_command must be populated")
			}
			break
		}
	}
	if !found {
		t.Fatal("expected REPO_FIND_PUBLISHED_MISSING_BLOB in response")
	}
}

func TestListRepositoryFindings_PublishedUnsignedRequired(t *testing.T) {
	srv := newTestServer(t)
	srv.signaturePolicy.SetPolicyForTest(strictPolicy())
	ctx := context.Background()

	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: "sha256:abcd", SizeBytes: 100,
	})
	key := artifactKeyWithBuild(ref, 1)
	srv.scylla = &fakeLedger{
		rows: map[string]*manifestRow{key: {
			ArtifactKey:  key,
			PublisherID:  "core@globular.io", Name: "echo",
			Version: "1.0.0", Platform: "linux_amd64",
			BuildNumber: 1, Checksum: "sha256:abcd", SizeBytes: 100,
			PublishState: repopb.PublishState_PUBLISHED.String(),
		}},
	}
	resp, err := srv.ListRepositoryFindings(ctx, &repopb.ListRepositoryFindingsRequest{})
	if err != nil {
		t.Fatalf("ListRepositoryFindings: %v", err)
	}
	found := false
	for _, f := range resp.GetFindings() {
		if f.GetKind() == repopb.RepositoryFindingKind_REPO_FIND_PUBLISHED_UNSIGNED_REQUIRED {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected REPO_FIND_PUBLISHED_UNSIGNED_REQUIRED")
	}
}

func TestListRepositoryFindings_RevokedInstallableCoherence(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io", Name: "echo",
		Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: ref, BuildNumber: 1, BuildId: "v1",
		Checksum: "sha256:abcd", SizeBytes: 100,
	})
	key := artifactKeyWithBuild(ref, 1)
	// Pipeline state REVOKED, but we synthesize a row whose publish_state
	// is still PUBLISHED — the incoherent state the rule must catch.
	_ = srv.transitionArtifactState(ctx, key, PipelineRevoked, "test_revoke", "", ArtifactStateFields{
		BlobKey: binaryStorageKey(key), Checksum: "sha256:abcd", SizeBytes: 100,
	})
	srv.scylla = &fakeLedger{
		rows: map[string]*manifestRow{key: {
			ArtifactKey:   key,
			PublisherID:   "core@globular.io", Name: "echo",
			Version: "1.0.0", Platform: "linux_amd64",
			BuildNumber:   1, Checksum: "sha256:abcd", SizeBytes: 100,
			PublishState:  repopb.PublishState_PUBLISHED.String(),
			ArtifactState: string(PipelineRevoked),
		}},
	}
	resp, err := srv.ListRepositoryFindings(ctx, &repopb.ListRepositoryFindingsRequest{})
	if err != nil {
		t.Fatalf("ListRepositoryFindings: %v", err)
	}
	found := false
	for _, f := range resp.GetFindings() {
		if f.GetKind() == repopb.RepositoryFindingKind_REPO_FIND_REVOKED_INSTALLABLE {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected REPO_FIND_REVOKED_INSTALLABLE for stale publish_state=PUBLISHED + pipeline=REVOKED")
	}
}

func TestRecordAndListConfigReceipts(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	r := &repopb.PackageConfigReceipt{
		NodeId: "n1", PublisherId: "core@globular.io", Name: "echo",
		Platform: "linux_amd64", BuildNumber: 1, Path: "/etc/globular/echo.json",
		ConfigKind:    repopb.ConfigKind_CONFIG_OPERATOR_OVERRIDE,
		MergeStrategy: repopb.MergeStrategy_MERGE_PRESERVE,
		Action:        repopb.ConfigReceiptAction_CONFIG_RECEIPT_PRESERVED,
		ChecksumBefore: "sha256:abc", ChecksumAfter: "sha256:abc",
	}
	if _, err := srv.RecordConfigReceipt(ctx, &repopb.RecordConfigReceiptRequest{Receipt: r}); err != nil {
		t.Fatalf("RecordConfigReceipt: %v", err)
	}
	resp, err := srv.ListConfigReceipts(ctx, &repopb.ListConfigReceiptsRequest{
		PublisherId: "core@globular.io", Name: "echo", Platform: "linux_amd64",
	})
	if err != nil {
		t.Fatalf("ListConfigReceipts: %v", err)
	}
	if len(resp.GetReceipts()) != 1 {
		t.Fatalf("expected 1 receipt, got %d", len(resp.GetReceipts()))
	}
	if resp.GetReceipts()[0].GetAction() != repopb.ConfigReceiptAction_CONFIG_RECEIPT_PRESERVED {
		t.Errorf("action: got %s, want PRESERVED", resp.GetReceipts()[0].GetAction())
	}
}

func TestConfigReceipts_RedactsSecretPath(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()
	r := &repopb.PackageConfigReceipt{
		NodeId: "n1", PublisherId: "core@globular.io", Name: "echo",
		Platform: "linux_amd64", Path: "/var/lib/globular/secret.key",
		ConfigKind: repopb.ConfigKind_CONFIG_SECRET, Sensitive: true,
		Action: repopb.ConfigReceiptAction_CONFIG_RECEIPT_SKIPPED_SECRET,
	}
	if _, err := srv.RecordConfigReceipt(ctx, &repopb.RecordConfigReceiptRequest{Receipt: r}); err != nil {
		t.Fatalf("RecordConfigReceipt: %v", err)
	}
	resp, _ := srv.ListConfigReceipts(ctx, &repopb.ListConfigReceiptsRequest{
		PublisherId: "core@globular.io", Name: "echo", Platform: "linux_amd64",
	})
	if got := resp.GetReceipts()[0].GetPath(); got != "[REDACTED]" {
		t.Fatalf("expected [REDACTED] path, got %q", got)
	}
}

func TestConfigReceipts_ConflictFilterReturnsOnlyConflicts(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	// Mix of actions.
	for _, action := range []repopb.ConfigReceiptAction{
		repopb.ConfigReceiptAction_CONFIG_RECEIPT_PRESERVED,
		repopb.ConfigReceiptAction_CONFIG_RECEIPT_REPLACED,
		repopb.ConfigReceiptAction_CONFIG_RECEIPT_CONFLICT,
		repopb.ConfigReceiptAction_CONFIG_RECEIPT_CONFLICT,
	} {
		_, _ = srv.RecordConfigReceipt(ctx, &repopb.RecordConfigReceiptRequest{
			Receipt: &repopb.PackageConfigReceipt{
				NodeId: "n1", PublisherId: "core@globular.io", Name: "echo",
				Platform: "linux_amd64", Path: "/etc/globular/x.json",
				Action: action,
			},
		})
	}
	resp, _ := srv.ListConfigReceipts(ctx, &repopb.ListConfigReceiptsRequest{
		PublisherId:  "core@globular.io", Name: "echo", Platform: "linux_amd64",
		ActionFilter: repopb.ConfigReceiptAction_CONFIG_RECEIPT_CONFLICT,
	})
	if len(resp.GetReceipts()) != 2 {
		t.Fatalf("expected 2 CONFLICT receipts, got %d", len(resp.GetReceipts()))
	}
}
