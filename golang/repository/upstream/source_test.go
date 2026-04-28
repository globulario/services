package upstream

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ── Provider selection tests ────────────────────────────────────────────────

func TestNewSource_GitHub(t *testing.T) {
	src, err := NewSource(TypeGitHubRelease)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Type() != TypeGitHubRelease {
		t.Fatalf("expected %s, got %s", TypeGitHubRelease, src.Type())
	}
}

func TestNewSource_HTTPIndex(t *testing.T) {
	src, err := NewSource(TypeHTTPIndex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Type() != TypeHTTPIndex {
		t.Fatalf("expected %s, got %s", TypeHTTPIndex, src.Type())
	}
}

func TestNewSource_LocalDir(t *testing.T) {
	src, err := NewSource(TypeLocalDir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Type() != TypeLocalDir {
		t.Fatalf("expected %s, got %s", TypeLocalDir, src.Type())
	}
}

func TestNewSource_GitIndex_Unimplemented(t *testing.T) {
	_, err := NewSource(TypeGitIndex)
	if err == nil {
		t.Fatal("expected error for unimplemented GIT_INDEX")
	}
	if !errors.Is(err, ErrProviderUnimplemented) {
		t.Fatalf("expected ErrProviderUnimplemented, got: %v", err)
	}
}

func TestNewSource_Unknown(t *testing.T) {
	_, err := NewSource("MAGIC_SOURCE")
	if err == nil {
		t.Fatal("expected error for unknown type")
	}
}

func TestNewSource_Empty(t *testing.T) {
	_, err := NewSource("")
	if err == nil {
		t.Fatal("expected error for empty type")
	}
}

func TestMapProtoType(t *testing.T) {
	tests := []struct {
		val  int32
		want string
	}{
		{1, TypeGitHubRelease},
		{2, TypeHTTPIndex},
		{3, TypeGitIndex},
		{4, TypeLocalDir},
		{0, ""},
		{99, ""},
	}
	for _, tt := range tests {
		got := MapProtoType(tt.val)
		if got != tt.want {
			t.Errorf("MapProtoType(%d) = %q, want %q", tt.val, got, tt.want)
		}
	}
}

// ── HTTP_INDEX tests ────────────────────────────────────────────────────────

func TestHTTPIndex_ListReleases_Unsupported(t *testing.T) {
	src := &HTTPIndexSource{}
	_, err := src.ListReleases(context.Background(), SourceOpts{})
	if !errors.Is(err, ErrListUnsupported) {
		t.Fatalf("expected ErrListUnsupported, got: %v", err)
	}
}

func TestHTTPIndex_ResolveArtifactBaseURL(t *testing.T) {
	opts := SourceOpts{ArtifactBaseURL: "https://repo.local/artifacts"}
	ref := ArtifactRef{AssetPath: "packages/echo_1.0.84_linux_amd64.tgz", Name: "echo"}
	url := resolveHTTPAssetURL(opts, ref)
	want := "https://repo.local/artifacts/packages/echo_1.0.84_linux_amd64.tgz"
	if url != want {
		t.Fatalf("got %q, want %q", url, want)
	}
}

func TestHTTPIndex_AssetURLOverridesAssetPath(t *testing.T) {
	opts := SourceOpts{ArtifactBaseURL: "https://repo.local/artifacts"}
	ref := ArtifactRef{
		AssetURL:  "https://cdn.example.com/direct/echo.tgz",
		AssetPath: "packages/echo.tgz",
		Name:      "echo",
	}
	url := resolveHTTPAssetURL(opts, ref)
	if url != "https://cdn.example.com/direct/echo.tgz" {
		t.Fatalf("asset_url should override asset_path: got %q", url)
	}
}

func TestHTTPIndex_FallbackToFilename(t *testing.T) {
	opts := SourceOpts{ArtifactBaseURL: "https://repo.local/packages"}
	ref := ArtifactRef{Filename: "echo_1.0.84_linux_amd64.tgz", Name: "echo"}
	url := resolveHTTPAssetURL(opts, ref)
	want := "https://repo.local/packages/echo_1.0.84_linux_amd64.tgz"
	if url != want {
		t.Fatalf("got %q, want %q", url, want)
	}
}

// ── LOCAL_DIR tests ─────────────────────────────────────────────────────────

func TestLocalDir_GetReleaseIndex(t *testing.T) {
	root := t.TempDir()
	releaseDir := filepath.Join(root, "releases", "v1.0.84")
	os.MkdirAll(releaseDir, 0o755)
	indexContent := `{"schema_version":"globular.repository.index/v1","release_tag":"v1.0.84","packages":[]}`
	os.WriteFile(filepath.Join(releaseDir, "release-index.json"), []byte(indexContent), 0o644)

	src := &LocalDirSource{}
	data, err := src.GetReleaseIndex(context.Background(), SourceOpts{
		LocalRoot:         root,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}, "v1.0.84")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != indexContent {
		t.Fatalf("content mismatch: %q", string(data))
	}
}

func TestLocalDir_ListReleases(t *testing.T) {
	root := t.TempDir()
	// Create two release dirs, one without index (should be skipped).
	os.MkdirAll(filepath.Join(root, "releases", "v1.0.82"), 0o755)
	os.WriteFile(filepath.Join(root, "releases", "v1.0.82", "release-index.json"), []byte(`{}`), 0o644)
	os.MkdirAll(filepath.Join(root, "releases", "v1.0.83"), 0o755)
	os.WriteFile(filepath.Join(root, "releases", "v1.0.83", "release-index.json"), []byte(`{}`), 0o644)
	os.MkdirAll(filepath.Join(root, "releases", "empty-dir"), 0o755) // no index

	src := &LocalDirSource{}
	refs, err := src.ListReleases(context.Background(), SourceOpts{LocalRoot: root})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(refs) != 2 {
		t.Fatalf("expected 2 releases, got %d", len(refs))
	}
}

func TestLocalDir_OpenArtifact_AssetPath(t *testing.T) {
	root := t.TempDir()
	pkgDir := filepath.Join(root, "packages")
	os.MkdirAll(pkgDir, 0o755)
	os.WriteFile(filepath.Join(pkgDir, "echo.tgz"), []byte("fake-archive"), 0o644)

	src := &LocalDirSource{}
	rc, meta, err := src.OpenArtifact(context.Background(), SourceOpts{LocalRoot: root},
		ArtifactRef{AssetPath: "packages/echo.tgz", Name: "echo"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()
	if meta.ContentLength != 12 { // len("fake-archive")
		t.Fatalf("expected content length 12, got %d", meta.ContentLength)
	}
}

func TestLocalDir_RejectsPathTraversal_DotDot(t *testing.T) {
	root := t.TempDir()
	src := &LocalDirSource{}
	_, _, err := src.OpenArtifact(context.Background(), SourceOpts{LocalRoot: root},
		ArtifactRef{AssetPath: "../../../etc/passwd", Name: "exploit"})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Fatalf("expected traversal error, got: %v", err)
	}
}

func TestLocalDir_RejectsAbsoluteAssetPath(t *testing.T) {
	root := t.TempDir()
	src := &LocalDirSource{}
	_, _, err := src.OpenArtifact(context.Background(), SourceOpts{LocalRoot: root},
		ArtifactRef{AssetPath: "/etc/passwd", Name: "exploit"})
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Fatalf("expected traversal error, got: %v", err)
	}
}

func TestLocalDir_RejectsNonAbsoluteRoot(t *testing.T) {
	src := &LocalDirSource{}
	_, err := src.GetReleaseIndex(context.Background(), SourceOpts{
		LocalRoot:         "relative/path",
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}, "v1.0.84")
	if err == nil {
		t.Fatal("expected error for non-absolute root")
	}
}

func TestLocalDir_RejectsHTTPAssetURL(t *testing.T) {
	root := t.TempDir()
	src := &LocalDirSource{}
	_, _, err := src.OpenArtifact(context.Background(), SourceOpts{LocalRoot: root},
		ArtifactRef{AssetURL: "https://evil.com/payload.tgz", Name: "exploit"})
	if err == nil {
		t.Fatal("expected error for HTTP URL in LOCAL_DIR")
	}
}

func TestLocalDir_ResolvesFileURL(t *testing.T) {
	root := t.TempDir()
	os.WriteFile(filepath.Join(root, "test.tgz"), []byte("data"), 0o644)

	src := &LocalDirSource{}
	rc, _, err := src.OpenArtifact(context.Background(), SourceOpts{LocalRoot: root},
		ArtifactRef{AssetURL: "file://test.tgz", Name: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	rc.Close()
}

// ── SafeJoin tests ──────────────────────────────────────────────────────────

func TestSafeJoin_ValidPath(t *testing.T) {
	root := t.TempDir()
	os.MkdirAll(filepath.Join(root, "sub"), 0o755)
	os.WriteFile(filepath.Join(root, "sub", "file.txt"), []byte("ok"), 0o644)

	path, err := safeJoin(root, "sub/file.txt")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.HasPrefix(path, root) {
		t.Fatalf("path %q should be under root %q", path, root)
	}
}

func TestSafeJoin_RejectsDotDot(t *testing.T) {
	root := t.TempDir()
	_, err := safeJoin(root, "../escape")
	if err == nil {
		t.Fatal("expected error for ..")
	}
}

func TestSafeJoin_RejectsAbsolute(t *testing.T) {
	root := t.TempDir()
	_, err := safeJoin(root, "/etc/passwd")
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}
