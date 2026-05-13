package main

import (
	"context"
	"strings"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

func TestEnsureReleaseBuildAliasAndLoad(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "workflow",
		Version:     "1.0.53",
		Platform:    "linux_amd64",
		Kind:        repopb.ArtifactKind_SERVICE,
	}

	err := srv.ensureReleaseBuildAlias(
		context.Background(),
		ref,
		"v1.0.53",
		67,
		"upstream-bid-67",
		"canonical-bid-1",
		"sha256:abcd",
		"v1.0.53",
		"globulario-github",
	)
	if err != nil {
		t.Fatalf("ensureReleaseBuildAlias: %v", err)
	}

	rec, err := srv.loadReleaseBuildAlias(context.Background(), ref, "v1.0.53", 67)
	if err != nil {
		t.Fatalf("loadReleaseBuildAlias: %v", err)
	}
	if rec == nil {
		t.Fatal("expected alias record, got nil")
	}
	if rec.CanonicalBuildID != "canonical-bid-1" {
		t.Fatalf("canonical_build_id=%q", rec.CanonicalBuildID)
	}
	if rec.UpstreamBuildID != "upstream-bid-67" {
		t.Fatalf("upstream_build_id=%q", rec.UpstreamBuildID)
	}
}

func TestEnsureReleaseBuildAliasRejectsCanonicalConflict(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "workflow",
		Version:     "1.0.53",
		Platform:    "linux_amd64",
	}
	ctx := context.Background()

	if err := srv.ensureReleaseBuildAlias(ctx, ref, "v1.0.53", 67, "upstream-a", "canonical-a", "sha256:abcd", "v1.0.53", "src"); err != nil {
		t.Fatalf("initial alias write failed: %v", err)
	}

	err := srv.ensureReleaseBuildAlias(ctx, ref, "v1.0.53", 67, "upstream-a", "canonical-b", "sha256:abcd", "v1.0.53", "src")
	if err == nil {
		t.Fatal("expected alias conflict error, got nil")
	}
}

func TestAliasStorageKey_UsesLegacyBuildAliasLocator(t *testing.T) {
	ref := &repopb.ArtifactRef{
		PublisherId: "core@globular.io",
		Name:        "workflow",
		Version:     "1.0.53",
		Platform:    "linux/amd64",
	}
	got := aliasStorageKey(ref, "v1.0.53", 67)
	wantSuffix := "artifacts/aliases/core@globular.io/workflow/1.0.53/linux_amd64/v1.0.53/67.json"
	if !strings.HasSuffix(got, wantSuffix) {
		t.Fatalf("aliasStorageKey=%q, want suffix %q", got, wantSuffix)
	}
}
