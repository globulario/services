package main

import (
	"context"
	"encoding/json"
	"testing"
)

// ── TrustedPublisher struct logic ────────────────────────────────────────────

func TestTrustedPublisherStorageKey(t *testing.T) {
	key := trustedPublisherStorageKey("acme", "rel-001")
	want := "artifacts/.trusted-publishers/acme/rel-001.json"
	if key != want {
		t.Errorf("trustedPublisherStorageKey = %q, want %q", key, want)
	}
}

func TestTrustedPublisherDirKey(t *testing.T) {
	key := trustedPublisherDirKey("acme")
	want := "artifacts/.trusted-publishers/acme"
	if key != want {
		t.Errorf("trustedPublisherDirKey = %q, want %q", key, want)
	}
}

func TestTrustedPublisherJSONRoundTrip(t *testing.T) {
	tp := &TrustedPublisher{
		ID:              "rel-001",
		PublisherID:     "acme",
		PackageName:     "my-service",
		Provider:        "github-actions",
		RepositoryOwner: "acme-org",
		RepositoryName:  "my-service",
		BranchPattern:   "main",
		CreatedBy:       "dave",
		CreatedAt:       "2026-03-12T00:00:00Z",
	}

	data, err := json.Marshal(tp)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got TrustedPublisher
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.RepositoryOwner != "acme-org" {
		t.Errorf("RepositoryOwner = %q, want %q", got.RepositoryOwner, "acme-org")
	}
	if got.PackageName != "my-service" {
		t.Errorf("PackageName = %q, want %q", got.PackageName, "my-service")
	}
}

// ── matchesTrustedPublisherBySubject ─────────────────────────────────────────
//
// This method requires server storage to list relationships. We test it by
// writing trusted publisher JSON files directly to the test server's storage.

func seedTrustedPublisher(t *testing.T, srv *server, tp *TrustedPublisher) {
	t.Helper()
	ctx := context.Background()
	dirKey := trustedPublisherDirKey(tp.PublisherID)
	if err := srv.Storage().MkdirAll(ctx, dirKey, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.MarshalIndent(tp, "", "  ")
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	storageKey := trustedPublisherStorageKey(tp.PublisherID, tp.ID)
	if err := srv.Storage().WriteFile(ctx, storageKey, data, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func TestMatchesTrustedPublisher_ByRepositoryOwner(t *testing.T) {
	srv := newTestServer(t)
	seedTrustedPublisher(t, srv, &TrustedPublisher{
		ID:              "rel-001",
		PublisherID:     "acme",
		Provider:        "github-actions",
		RepositoryOwner: "acme-org",
		RepositoryName:  "build-pipeline",
	})

	ctx := context.Background()
	if !srv.matchesTrustedPublisherBySubject(ctx, "acme", "", "acme-org") {
		t.Error("expected match when subject matches RepositoryOwner")
	}
}

func TestMatchesTrustedPublisher_ByRepositoryName(t *testing.T) {
	srv := newTestServer(t)
	seedTrustedPublisher(t, srv, &TrustedPublisher{
		ID:              "rel-002",
		PublisherID:     "acme",
		Provider:        "github-actions",
		RepositoryOwner: "acme-org",
		RepositoryName:  "build-pipeline",
	})

	ctx := context.Background()
	if !srv.matchesTrustedPublisherBySubject(ctx, "acme", "", "build-pipeline") {
		t.Error("expected match when subject matches RepositoryName")
	}
}

func TestMatchesTrustedPublisher_NoMatch(t *testing.T) {
	srv := newTestServer(t)
	seedTrustedPublisher(t, srv, &TrustedPublisher{
		ID:              "rel-003",
		PublisherID:     "acme",
		Provider:        "github-actions",
		RepositoryOwner: "acme-org",
		RepositoryName:  "build-pipeline",
	})

	ctx := context.Background()
	if srv.matchesTrustedPublisherBySubject(ctx, "acme", "", "unknown-identity") {
		t.Error("expected no match for unrelated subject")
	}
}

func TestMatchesTrustedPublisher_PackageScoped(t *testing.T) {
	srv := newTestServer(t)
	seedTrustedPublisher(t, srv, &TrustedPublisher{
		ID:              "rel-004",
		PublisherID:     "acme",
		PackageName:     "my-service",
		Provider:        "github-actions",
		RepositoryOwner: "acme-org",
		RepositoryName:  "my-service-ci",
	})

	ctx := context.Background()

	// Should match correct package.
	if !srv.matchesTrustedPublisherBySubject(ctx, "acme", "my-service", "acme-org") {
		t.Error("expected match for correct package scope")
	}

	// Should NOT match wrong package.
	if srv.matchesTrustedPublisherBySubject(ctx, "acme", "other-service", "acme-org") {
		t.Error("expected no match for wrong package scope")
	}
}

func TestMatchesTrustedPublisher_EmptyStorage(t *testing.T) {
	srv := newTestServer(t)
	ctx := context.Background()

	if srv.matchesTrustedPublisherBySubject(ctx, "acme", "", "acme-org") {
		t.Error("expected no match on empty storage")
	}
}
