package upstream

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// skipIfNoGit skips the test if git is not available.
func skipIfNoGit(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git binary not found — skipping GIT_INDEX test")
	}
}

// createTestBareRepo creates a bare Git repo with a release-index.json at a tagged ref.
// Returns the bare repo path.
func createTestBareRepo(t *testing.T, tag, indexContent string) string {
	t.Helper()
	dir := t.TempDir()
	workDir := filepath.Join(dir, "work")
	bareDir := filepath.Join(dir, "repo.git")

	// Create work repo.
	os.MkdirAll(workDir, 0o755)
	run(t, workDir, "git", "init", ".")
	run(t, workDir, "git", "config", "user.email", "test@test.com")
	run(t, workDir, "git", "config", "user.name", "Test")

	// Create release-index.json at releases/{tag}/
	relDir := filepath.Join(workDir, "releases", tag)
	os.MkdirAll(relDir, 0o755)
	os.WriteFile(filepath.Join(relDir, "release-index.json"), []byte(indexContent), 0o644)

	// Also add a test artifact.
	pkgDir := filepath.Join(workDir, "packages")
	os.MkdirAll(pkgDir, 0o755)
	os.WriteFile(filepath.Join(pkgDir, "echo_1.0.84_linux_amd64.tgz"), []byte("fake-archive-data"), 0o644)

	run(t, workDir, "git", "add", "-A")
	run(t, workDir, "git", "commit", "-m", "release "+tag)
	run(t, workDir, "git", "tag", tag)

	// Create bare clone.
	run(t, dir, "git", "clone", "--bare", workDir, bareDir)

	return bareDir
}

func run(t *testing.T, dir string, name string, args ...string) {
	t.Helper()
	cmd := exec.Command(name, args...)
	if dir != "" {
		cmd.Dir = dir
	}
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("%s %s failed: %v\n%s", name, strings.Join(args, " "), err, string(out))
	}
}

// ── GIT_INDEX provider tests ────────────────────────────────────────────────

func TestGitIndex_NewSource(t *testing.T) {
	src, err := NewSource(TypeGitIndex)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if src.Type() != TypeGitIndex {
		t.Fatalf("expected %s, got %s", TypeGitIndex, src.Type())
	}
}

func TestGitIndex_GetReleaseIndex_FromLocalBareRepo(t *testing.T) {
	skipIfNoGit(t)
	indexContent := `{"schema_version":"globular.repository.index/v1","release_tag":"v1.0.84","packages":[]}`
	bareDir := createTestBareRepo(t, "v1.0.84", indexContent)

	cacheDir := t.TempDir()
	src := &GitIndexSource{}
	data, err := src.GetReleaseIndex(context.Background(), SourceOpts{
		RepoURL:           bareDir,
		CacheDir:          cacheDir,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}, "v1.0.84")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(data) != indexContent {
		t.Fatalf("content mismatch: %q", string(data))
	}
}

func TestGitIndex_ListReleases_FromLocalBareRepo(t *testing.T) {
	skipIfNoGit(t)
	bareDir := createTestBareRepo(t, "v1.0.84", `{"packages":[]}`)

	src := &GitIndexSource{}
	refs, err := src.ListReleases(context.Background(), SourceOpts{
		RepoURL: bareDir,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, r := range refs {
		if r.Tag == "v1.0.84" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected tag v1.0.84 in refs: %v", refs)
	}
}

func TestGitIndex_OpenArtifact_AssetPath_FromCheckout(t *testing.T) {
	skipIfNoGit(t)
	bareDir := createTestBareRepo(t, "v1.0.84", `{"packages":[]}`)

	cacheDir := t.TempDir()
	src := &GitIndexSource{}

	// First, get the index to populate the cache.
	_, err := src.GetReleaseIndex(context.Background(), SourceOpts{
		RepoURL:           bareDir,
		CacheDir:          cacheDir,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}, "v1.0.84")
	if err != nil {
		t.Fatalf("setup: %v", err)
	}

	// Now open an artifact from the Git repo.
	rc, meta, err := src.OpenArtifact(context.Background(), SourceOpts{
		RepoURL:  bareDir,
		CacheDir: cacheDir,
	}, ArtifactRef{
		AssetPath:  "packages/echo_1.0.84_linux_amd64.tgz",
		ReleaseTag: "v1.0.84",
		Name:       "echo",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	defer rc.Close()

	if meta.ContentLength != int64(len("fake-archive-data")) {
		t.Fatalf("expected content length %d, got %d", len("fake-archive-data"), meta.ContentLength)
	}
}

func TestGitIndex_OpenArtifact_AssetURL_HTTP(t *testing.T) {
	// When asset_url is an HTTP URL, GIT_INDEX delegates to HTTP download.
	// We can't test a real HTTP download here, but verify the code path
	// selects HTTP when asset_url is set.
	src := &GitIndexSource{}
	_, _, err := src.OpenArtifact(context.Background(), SourceOpts{}, ArtifactRef{
		AssetURL: "https://example.com/package.tgz",
		Name:     "test",
	})
	// This will fail because example.com is not reachable, but it should be
	// an HTTP error, not a "cannot resolve" error.
	if err == nil {
		t.Fatal("expected error (no real server)")
	}
	if strings.Contains(err.Error(), "cannot resolve") {
		t.Fatalf("should have attempted HTTP, got: %v", err)
	}
}

func TestGitIndex_OpenArtifact_ArtifactBaseURL(t *testing.T) {
	src := &GitIndexSource{}
	_, _, err := src.OpenArtifact(context.Background(), SourceOpts{
		ArtifactBaseURL: "https://artifacts.local",
	}, ArtifactRef{
		AssetPath: "packages/echo.tgz",
		Name:      "echo",
	})
	// Should attempt HTTP, not "cannot resolve".
	if err == nil {
		t.Fatal("expected error (no real server)")
	}
	if strings.Contains(err.Error(), "cannot resolve") {
		t.Fatalf("should have attempted HTTP via artifact_base_url, got: %v", err)
	}
}

func TestGitIndex_RejectsPathTraversal(t *testing.T) {
	skipIfNoGit(t)
	bareDir := createTestBareRepo(t, "v1.0.84", `{}`)
	cacheDir := t.TempDir()

	src := &GitIndexSource{}
	// Populate cache.
	src.GetReleaseIndex(context.Background(), SourceOpts{
		RepoURL:           bareDir,
		CacheDir:          cacheDir,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}, "v1.0.84")

	_, _, err := src.OpenArtifact(context.Background(), SourceOpts{
		RepoURL:  bareDir,
		CacheDir: cacheDir,
	}, ArtifactRef{
		AssetPath:  "../../../etc/passwd",
		ReleaseTag: "v1.0.84",
		Name:       "exploit",
	})
	if err == nil {
		t.Fatal("expected error for path traversal")
	}
	if !strings.Contains(err.Error(), "traversal") {
		t.Fatalf("expected traversal error, got: %v", err)
	}
}

func TestGitIndex_RejectsAbsoluteAssetPath(t *testing.T) {
	skipIfNoGit(t)
	bareDir := createTestBareRepo(t, "v1.0.84", `{}`)
	cacheDir := t.TempDir()

	src := &GitIndexSource{}
	src.GetReleaseIndex(context.Background(), SourceOpts{
		RepoURL:           bareDir,
		CacheDir:          cacheDir,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}, "v1.0.84")

	_, _, err := src.OpenArtifact(context.Background(), SourceOpts{
		RepoURL:  bareDir,
		CacheDir: cacheDir,
	}, ArtifactRef{
		AssetPath:  "/etc/passwd",
		ReleaseTag: "v1.0.84",
		Name:       "exploit",
	})
	if err == nil {
		t.Fatal("expected error for absolute path")
	}
}

func TestGitIndex_MissingTag(t *testing.T) {
	skipIfNoGit(t)
	bareDir := createTestBareRepo(t, "v1.0.84", `{}`)
	cacheDir := t.TempDir()

	src := &GitIndexSource{}
	_, err := src.GetReleaseIndex(context.Background(), SourceOpts{
		RepoURL:           bareDir,
		CacheDir:          cacheDir,
		IndexPathTemplate: "releases/{tag}/release-index.json",
	}, "v99.0.0")
	if err == nil {
		t.Fatal("expected error for missing tag")
	}
	if !strings.Contains(err.Error(), "not found") {
		t.Fatalf("expected 'not found' error, got: %v", err)
	}
}

func TestGitIndex_MissingRepoURL(t *testing.T) {
	src := &GitIndexSource{}
	_, err := src.GetReleaseIndex(context.Background(), SourceOpts{
		CacheDir: "/tmp/test",
	}, "v1.0.84")
	if err == nil {
		t.Fatal("expected error for missing repo_url")
	}
}

func TestGitIndex_MissingCacheDir(t *testing.T) {
	skipIfNoGit(t)
	src := &GitIndexSource{}
	_, err := src.GetReleaseIndex(context.Background(), SourceOpts{
		RepoURL: "/tmp/nonexistent",
	}, "v1.0.84")
	if err == nil {
		t.Fatal("expected error for missing cache_dir")
	}
}

func TestRequireGitBinary(t *testing.T) {
	// This test verifies the function doesn't panic. Whether git is
	// available depends on the environment.
	err := requireGitBinary()
	if err != nil {
		t.Skipf("git not available: %v", err)
	}
}

func TestRedactGitError(t *testing.T) {
	msg := "fatal: Authentication failed for 'https://user:secrettoken@github.com/org/repo.git/'"
	redacted := redactGitError(msg)
	if strings.Contains(redacted, "secrettoken") {
		t.Fatalf("token leaked in redacted error: %s", redacted)
	}
	if !strings.Contains(redacted, "[REDACTED]") {
		t.Fatalf("expected [REDACTED] in output: %s", redacted)
	}
}
