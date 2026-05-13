package main

import (
	"context"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestFindEntryAliasAwareBuildIDMatch(t *testing.T) {
	srv := newTestServer(t)
	src := &UpstreamRepositorySource{
		srv: srv,
		src: &repopb.UpstreamSource{Name: "test-upstream"},
	}

	req := ArtifactRequest{
		PublisherID: "core@globular.io",
		Name:        "workflow",
		Version:     "1.0.53",
		Platform:    "linux_amd64",
		BuildNumber: 67,
		BuildID:     "canonical-A",
		ReleaseTag:  "v1.0.53",
	}

	idx := &releaseIndex{
		ReleaseTag: "v1.0.53",
		Packages: []*releaseIndexEntry{
			{
				Publisher:       "core@globular.io",
				Name:            "workflow",
				Version:         "1.0.53",
				Platform:        "linux_amd64",
				BuildNumber:     67,
				BuildID:         "upstream-B",
				ArtifactSha256:  "sha256:same",
				PackageDigest:   "sha256:same",
			},
		},
	}

	ref := &repopb.ArtifactRef{
		PublisherId: req.PublisherID,
		Name:        req.Name,
		Version:     req.Version,
		Platform:    req.Platform,
	}
	if err := srv.ensureReleaseBuildAlias(context.Background(), ref, req.ReleaseTag, req.BuildNumber, "upstream-B", "canonical-A", "sha256:same", "v1.0.53", "test-upstream"); err != nil {
		t.Fatalf("write alias: %v", err)
	}

	aliasRec, err := srv.loadReleaseBuildAlias(context.Background(), ref, req.ReleaseTag, req.BuildNumber)
	if err != nil {
		t.Fatalf("load alias: %v", err)
	}
	got := src.findEntry(idx, req, aliasRec)
	if got == nil {
		t.Fatal("expected alias-aware match, got nil")
	}
	if got.BuildID != "upstream-B" {
		t.Fatalf("expected matched upstream build_id, got %q", got.BuildID)
	}
}

func TestFindEntryRejectsMismatchedBuildIDWithoutAlias(t *testing.T) {
	src := &UpstreamRepositorySource{
		srv: newTestServer(t),
		src: &repopb.UpstreamSource{Name: "test-upstream"},
	}
	req := ArtifactRequest{
		PublisherID: "core@globular.io",
		Name:        "workflow",
		Version:     "1.0.53",
		Platform:    "linux_amd64",
		BuildNumber: 67,
		BuildID:     "canonical-A",
		ReleaseTag:  "v1.0.53",
	}
	idx := &releaseIndex{
		ReleaseTag: "v1.0.53",
		Packages: []*releaseIndexEntry{
			{
				Publisher:       "core@globular.io",
				Name:            "workflow",
				Version:         "1.0.53",
				Platform:        "linux_amd64",
				BuildNumber:     67,
				BuildID:         "upstream-B",
				ArtifactSha256:  "sha256:same",
				PackageDigest:   "sha256:same",
			},
		},
	}
	if got := src.findEntry(idx, req, nil); got != nil {
		t.Fatalf("expected nil without alias, got build_id=%q", got.BuildID)
	}
}
