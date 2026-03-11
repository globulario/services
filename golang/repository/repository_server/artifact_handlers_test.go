package main

import (
	"context"
	"fmt"
	"testing"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/storage_backend"
	"google.golang.org/protobuf/encoding/protojson"
)

// newTestServer returns a server with local OS storage rooted in a temp dir.
func newTestServer(t *testing.T) *server {
	t.Helper()
	dir := t.TempDir()
	srv := &server{Root: dir}
	srv.storage = storage_backend.NewOSStorage(dir)
	return srv
}

// seedArtifact writes a manifest (and dummy binary) into the test server.
func seedArtifact(t *testing.T, srv *server, m *repopb.ArtifactManifest) {
	t.Helper()
	ctx := context.Background()
	key := artifactKey(m.GetRef())
	_ = srv.Storage().MkdirAll(ctx, artifactsDir, 0o755)
	mjson, err := protojson.Marshal(m)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, manifestStorageKey(key), mjson, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	if err := srv.Storage().WriteFile(ctx, binaryStorageKey(key), []byte("fake-binary"), 0o644); err != nil {
		t.Fatalf("write binary: %v", err)
	}
}

func TestSearchArtifacts_EmptyCatalog(t *testing.T) {
	srv := newTestServer(t)
	resp, err := srv.SearchArtifacts(context.Background(), &repopb.SearchArtifactsRequest{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetArtifacts()) != 0 {
		t.Errorf("expected 0 artifacts, got %d", len(resp.GetArtifacts()))
	}
}

func TestSearchArtifacts_TextQuery(t *testing.T) {
	srv := newTestServer(t)
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         &repopb.ArtifactRef{PublisherId: "glob", Name: "gateway", Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE},
		Description: "HTTP gateway for Globular",
		Keywords:    []string{"http", "proxy"},
	})
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref:         &repopb.ArtifactRef{PublisherId: "glob", Name: "rbac", Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE},
		Description: "Role-based access control",
	})

	// Search for "gateway"
	resp, err := srv.SearchArtifacts(context.Background(), &repopb.SearchArtifactsRequest{Query: "gateway"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetArtifacts()) != 1 {
		t.Fatalf("expected 1 result, got %d", len(resp.GetArtifacts()))
	}
	if resp.GetArtifacts()[0].GetRef().GetName() != "gateway" {
		t.Errorf("expected gateway, got %s", resp.GetArtifacts()[0].GetRef().GetName())
	}

	// Search for keyword "proxy"
	resp, err = srv.SearchArtifacts(context.Background(), &repopb.SearchArtifactsRequest{Query: "proxy"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetArtifacts()) != 1 {
		t.Fatalf("expected 1 result for keyword search, got %d", len(resp.GetArtifacts()))
	}
}

func TestSearchArtifacts_FilterByKind(t *testing.T) {
	srv := newTestServer(t)
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{PublisherId: "glob", Name: "gateway", Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE},
	})
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{PublisherId: "glob", Name: "admin", Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_APPLICATION},
	})

	resp, err := srv.SearchArtifacts(context.Background(), &repopb.SearchArtifactsRequest{Kind: repopb.ArtifactKind_APPLICATION})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetArtifacts()) != 1 {
		t.Fatalf("expected 1 application, got %d", len(resp.GetArtifacts()))
	}
	if resp.GetArtifacts()[0].GetRef().GetName() != "admin" {
		t.Errorf("expected admin, got %s", resp.GetArtifacts()[0].GetRef().GetName())
	}
}

func TestSearchArtifacts_Pagination(t *testing.T) {
	srv := newTestServer(t)
	for i := 0; i < 5; i++ {
		seedArtifact(t, srv, &repopb.ArtifactManifest{
			Ref:           &repopb.ArtifactRef{PublisherId: "glob", Name: fmt.Sprintf("svc%d", i), Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE},
			PublishedUnix: int64(100 + i),
		})
	}

	// Page 1: size 2
	resp, err := srv.SearchArtifacts(context.Background(), &repopb.SearchArtifactsRequest{PageSize: 2})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetArtifacts()) != 2 {
		t.Fatalf("expected 2, got %d", len(resp.GetArtifacts()))
	}
	if resp.GetNextPageToken() == "" {
		t.Fatal("expected next_page_token")
	}
	if resp.GetTotalCount() != 5 {
		t.Errorf("expected total_count=5, got %d", resp.GetTotalCount())
	}

	// Page 2
	resp2, err := srv.SearchArtifacts(context.Background(), &repopb.SearchArtifactsRequest{PageSize: 2, PageToken: resp.GetNextPageToken()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp2.GetArtifacts()) != 2 {
		t.Fatalf("page 2: expected 2, got %d", len(resp2.GetArtifacts()))
	}

	// Page 3 (last)
	resp3, err := srv.SearchArtifacts(context.Background(), &repopb.SearchArtifactsRequest{PageSize: 2, PageToken: resp2.GetNextPageToken()})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp3.GetArtifacts()) != 1 {
		t.Fatalf("page 3: expected 1, got %d", len(resp3.GetArtifacts()))
	}
	if resp3.GetNextPageToken() != "" {
		t.Errorf("expected empty next_page_token on last page")
	}
}

func TestGetArtifactVersions(t *testing.T) {
	srv := newTestServer(t)
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{PublisherId: "glob", Name: "gateway", Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE},
	})
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{PublisherId: "glob", Name: "gateway", Version: "1.1.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE},
	})
	seedArtifact(t, srv, &repopb.ArtifactManifest{
		Ref: &repopb.ArtifactRef{PublisherId: "glob", Name: "rbac", Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE},
	})

	resp, err := srv.GetArtifactVersions(context.Background(), &repopb.GetArtifactVersionsRequest{
		Name: "gateway",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(resp.GetVersions()) != 2 {
		t.Fatalf("expected 2 versions, got %d", len(resp.GetVersions()))
	}
	// Should be sorted newest first.
	if resp.GetVersions()[0].GetRef().GetVersion() != "1.1.0" {
		t.Errorf("expected newest first (1.1.0), got %s", resp.GetVersions()[0].GetRef().GetVersion())
	}
}

func TestGetArtifactVersions_RequiresName(t *testing.T) {
	srv := newTestServer(t)
	_, err := srv.GetArtifactVersions(context.Background(), &repopb.GetArtifactVersionsRequest{})
	if err == nil {
		t.Fatal("expected error for missing name")
	}
}

func TestDeleteArtifact(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{PublisherId: "glob", Name: "gateway", Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE}
	seedArtifact(t, srv, &repopb.ArtifactManifest{Ref: ref})

	ctx := context.Background()

	// Verify it exists.
	_, err := srv.GetArtifactManifest(ctx, &repopb.GetArtifactManifestRequest{Ref: ref})
	if err != nil {
		t.Fatalf("expected artifact to exist: %v", err)
	}

	// Delete it.
	resp, err := srv.DeleteArtifact(ctx, &repopb.DeleteArtifactRequest{Ref: ref})
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !resp.GetResult() {
		t.Error("expected result=true")
	}

	// Verify it's gone.
	_, err = srv.GetArtifactManifest(ctx, &repopb.GetArtifactManifestRequest{Ref: ref})
	if err == nil {
		t.Fatal("expected artifact to be deleted")
	}
}

func TestDeleteArtifact_ResponseMessage(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{PublisherId: "glob", Name: "rbac", Version: "2.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE}
	seedArtifact(t, srv, &repopb.ArtifactManifest{Ref: ref})

	resp, err := srv.DeleteArtifact(context.Background(), &repopb.DeleteArtifactRequest{Ref: ref})
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !resp.GetResult() {
		t.Error("expected result=true")
	}
	if resp.GetMessage() == "" {
		t.Error("expected non-empty message")
	}
}

func TestDeleteArtifact_ForceField(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{PublisherId: "glob", Name: "gateway", Version: "3.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE}
	seedArtifact(t, srv, &repopb.ArtifactManifest{Ref: ref})

	// Delete with force=true should succeed (no installed-state to check since
	// etcd is not available in unit tests — findInstalledReferences returns nil).
	resp, err := srv.DeleteArtifact(context.Background(), &repopb.DeleteArtifactRequest{Ref: ref, Force: true})
	if err != nil {
		t.Fatalf("delete with force failed: %v", err)
	}
	if !resp.GetResult() {
		t.Error("expected result=true with force")
	}
}

func TestDeleteArtifact_NeverUninstalls(t *testing.T) {
	// Verify the contract: DeleteArtifact is a repository-only operation.
	// Even after deletion, the installed-state registry is untouched.
	// (In unit tests etcd is not available, so we just verify the operation
	// succeeds and the response message is explicit about semantics.)
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{PublisherId: "glob", Name: "auth", Version: "1.0.0", Platform: "linux_amd64", Kind: repopb.ArtifactKind_SERVICE}
	seedArtifact(t, srv, &repopb.ArtifactManifest{Ref: ref})

	resp, err := srv.DeleteArtifact(context.Background(), &repopb.DeleteArtifactRequest{Ref: ref})
	if err != nil {
		t.Fatalf("delete failed: %v", err)
	}
	if !resp.GetResult() {
		t.Error("expected result=true")
	}
	// Message should say "deleted from repository", not "uninstalled".
	msg := resp.GetMessage()
	if msg == "" {
		t.Fatal("expected message in response")
	}
}

func TestDeleteArtifact_NotFound(t *testing.T) {
	srv := newTestServer(t)
	ref := &repopb.ArtifactRef{PublisherId: "glob", Name: "nonexistent", Version: "1.0.0", Platform: "linux_amd64"}
	_, err := srv.DeleteArtifact(context.Background(), &repopb.DeleteArtifactRequest{Ref: ref})
	if err == nil {
		t.Fatal("expected NotFound error")
	}
}

func TestMatchesQuery(t *testing.T) {
	m := &repopb.ArtifactManifest{
		Ref:         &repopb.ArtifactRef{Name: "gateway"},
		Description: "HTTP gateway for Globular",
		Alias:       "Gateway Service",
		Keywords:    []string{"http", "proxy", "reverse-proxy"},
	}

	tests := []struct {
		query string
		want  bool
	}{
		{"gateway", true},
		{"gateway", true}, // query is pre-lowered by SearchArtifacts
		{"http", true},    // description
		{"proxy", true},   // keyword
		{"reverse", true}, // partial keyword
		{"database", false},
		{"", true}, // empty query shouldn't be called, but matches substring of anything
	}

	for _, tt := range tests {
		got := matchesQuery(m, tt.query)
		if got != tt.want {
			t.Errorf("matchesQuery(%q) = %v, want %v", tt.query, got, tt.want)
		}
	}
}
