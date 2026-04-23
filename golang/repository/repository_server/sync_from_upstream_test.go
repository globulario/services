package main

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestProcessSyncEntrySkipsExistingDigestWithDifferentBuildNumber(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "workflow",
		Version:     "1.0.53",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}
	seedPublishedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         ref,
		BuildNumber: 1,
		BuildId:     "019d0001-0000-7000-8000-000000000001",
		Checksum:    "sha256:same-content",
		SizeBytes:   100,
	})

	result := srv.processSyncEntry(
		context.Background(),
		&releaseIndexEntry{
			Name:          "workflow",
			Publisher:     "core@globular.io",
			Version:       "1.0.53",
			BuildID:       "67",
			Platform:      "linux_amd64",
			PackageDigest: "sha256:same-content",
			AssetURL:      "https://example.invalid/workflow.tgz",
		},
		&repopb.UpstreamSource{Name: "test-source"},
		"v1.0.53",
		false,
	)

	if result.GetStatus() != repopb.UpstreamSyncStatus_SYNC_SKIPPED {
		t.Fatalf("expected SYNC_SKIPPED, got %s: %s", result.GetStatus().String(), result.GetDetail())
	}
	if result.GetDetail() == "" {
		t.Fatal("expected detail explaining the existing artifact")
	}
}
