package upstream

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// GitIndexSource implements ReleaseSource for Git repositories.
// Works with any Git server: GitHub, GitLab, Gitea, Forgejo, bare repos.
//
// The provider clones/fetches the repo into a deterministic cache directory
// and reads release-index.json from the working tree. Artifacts are resolved
// via asset_url (HTTP), artifact_base_url + asset_path (HTTP), or directly
// from the checkout tree (local files inside the repo).
//
// Cache layout:
//
//	{CacheDir}/repo.git          — bare clone
//	{CacheDir}/worktree/{tag}    — sparse checkout for reading index
//
// Credentials:
//   - HTTPS repos: AuthToken is injected via GIT_ASKPASS helper
//   - SSH repos: handled by the system SSH agent or credentials_ref
//
// All git commands have a 60s timeout. Credentials are never logged.
type GitIndexSource struct{}

func (s *GitIndexSource) Type() string { return TypeGitIndex }

const (
	gitCmdTimeout = 60 * time.Second
)

// per-source locks to prevent concurrent clone/fetch races.
var (
	gitLocksMu sync.Mutex
	gitLocks   = map[string]*sync.Mutex{}
)

func getGitLock(cacheDir string) *sync.Mutex {
	gitLocksMu.Lock()
	defer gitLocksMu.Unlock()
	if gitLocks[cacheDir] == nil {
		gitLocks[cacheDir] = &sync.Mutex{}
	}
	return gitLocks[cacheDir]
}

// ListReleases lists Git tags from the remote repo.
func (s *GitIndexSource) ListReleases(ctx context.Context, opts SourceOpts) ([]ReleaseRef, error) {
	if err := requireGitBinary(); err != nil {
		return nil, err
	}
	if opts.RepoURL == "" {
		return nil, fmt.Errorf("GIT_INDEX: repo_url is required")
	}

	// Use ls-remote to list tags without cloning.
	args := []string{"ls-remote", "--tags", "--refs", opts.RepoURL}
	out, err := runGitCmd(ctx, "", opts, args...)
	if err != nil {
		return nil, fmt.Errorf("GIT_INDEX: list tags: %w", err)
	}

	var refs []ReleaseRef
	for _, line := range strings.Split(strings.TrimSpace(out), "\n") {
		if line == "" {
			continue
		}
		parts := strings.Fields(line)
		if len(parts) < 2 {
			continue
		}
		refName := parts[1]
		tag := strings.TrimPrefix(refName, "refs/tags/")
		refs = append(refs, ReleaseRef{Tag: tag, Name: tag})
	}
	return refs, nil
}

// GetReleaseIndex fetches/updates the Git repo cache and reads release-index.json.
func (s *GitIndexSource) GetReleaseIndex(ctx context.Context, opts SourceOpts, tag string) ([]byte, error) {
	if err := requireGitBinary(); err != nil {
		return nil, err
	}
	if opts.RepoURL == "" {
		return nil, fmt.Errorf("GIT_INDEX: repo_url is required")
	}
	if opts.CacheDir == "" {
		return nil, fmt.Errorf("GIT_INDEX: cache_dir is required — set repository data directory")
	}

	lock := getGitLock(opts.CacheDir)
	lock.Lock()
	defer lock.Unlock()

	bareDir := filepath.Join(opts.CacheDir, "repo.git")

	// Clone or fetch.
	if _, err := os.Stat(filepath.Join(bareDir, "HEAD")); os.IsNotExist(err) {
		// Initial bare clone.
		if mkErr := os.MkdirAll(opts.CacheDir, 0o755); mkErr != nil {
			return nil, fmt.Errorf("GIT_INDEX: create cache dir: %w", mkErr)
		}
		if _, cloneErr := runGitCmd(ctx, "", opts, "clone", "--bare", "--no-tags", opts.RepoURL, bareDir); cloneErr != nil {
			return nil, fmt.Errorf("GIT_INDEX: clone %s: %w", RedactAssetURL(opts.RepoURL), cloneErr)
		}
		// Fetch all tags after clone.
		if _, fetchErr := runGitCmd(ctx, bareDir, opts, "fetch", "origin", "+refs/tags/*:refs/tags/*"); fetchErr != nil {
			// Non-fatal: clone succeeded, tags may be fetched next time.
		}
	} else {
		// Fetch updates.
		if _, fetchErr := runGitCmd(ctx, bareDir, opts, "fetch", "origin", "--prune"); fetchErr != nil {
			return nil, fmt.Errorf("GIT_INDEX: fetch %s: %w", RedactAssetURL(opts.RepoURL), fetchErr)
		}
		// Fetch tags.
		runGitCmd(ctx, bareDir, opts, "fetch", "origin", "+refs/tags/*:refs/tags/*")
	}

	// Read the file from the bare repo at the requested tag/ref.
	template := opts.IndexPathTemplate
	if template == "" {
		template = "releases/{tag}/release-index.json"
	}
	filePath := strings.ReplaceAll(template, "{tag}", tag)

	// Try tag first, then branch.
	ref := tag
	if opts.Branch != "" {
		// Check if tag exists; if not, try branch.
		if _, err := runGitCmd(ctx, bareDir, opts, "rev-parse", "--verify", "refs/tags/"+tag); err != nil {
			ref = "origin/" + opts.Branch
		}
	}

	data, err := runGitCmd(ctx, bareDir, opts, "show", ref+":"+filePath)
	if err != nil {
		return nil, fmt.Errorf("GIT_INDEX: release-index.json not found at %s:%s — %w", ref, filePath, err)
	}

	return []byte(data), nil
}

// OpenArtifact resolves and opens a package artifact.
// Resolution order:
//  1. ref.AssetURL (HTTP) — delegate to HTTP download
//  2. opts.ArtifactBaseURL + ref.AssetPath — compose HTTP URL
//  3. ref.AssetPath from Git checkout — read from bare repo
func (s *GitIndexSource) OpenArtifact(ctx context.Context, opts SourceOpts, ref ArtifactRef) (io.ReadCloser, ArtifactMeta, error) {
	// HTTP-based artifact resolution (preferred).
	if ref.AssetURL != "" {
		return httpOpen(ref.AssetURL, opts.AuthToken)
	}
	if opts.ArtifactBaseURL != "" && (ref.AssetPath != "" || ref.Filename != "") {
		url := resolveHTTPAssetURL(opts, ref)
		if url != "" {
			return httpOpen(url, opts.AuthToken)
		}
	}

	// Fallback: read from Git repo (for small repos with blobs in Git).
	if ref.AssetPath != "" && opts.CacheDir != "" {
		return s.openFromGitRepo(ctx, opts, ref)
	}

	return nil, ArtifactMeta{}, fmt.Errorf("GIT_INDEX: cannot resolve artifact for %s — set asset_url or artifact_base_url", ref.Name)
}

// openFromGitRepo reads an artifact from the bare Git repo cache.
func (s *GitIndexSource) openFromGitRepo(ctx context.Context, opts SourceOpts, ref ArtifactRef) (io.ReadCloser, ArtifactMeta, error) {
	if err := requireGitBinary(); err != nil {
		return nil, ArtifactMeta{}, err
	}

	bareDir := filepath.Join(opts.CacheDir, "repo.git")

	// Validate path safety.
	if strings.Contains(ref.AssetPath, "..") {
		return nil, ArtifactMeta{}, fmt.Errorf("GIT_INDEX: path traversal rejected in asset_path %q", ref.AssetPath)
	}
	if filepath.IsAbs(ref.AssetPath) {
		return nil, ArtifactMeta{}, fmt.Errorf("GIT_INDEX: absolute asset_path rejected %q", ref.AssetPath)
	}

	// Determine the Git ref to read from.
	gitRef := ref.ReleaseTag
	if gitRef == "" && opts.Branch != "" {
		gitRef = "origin/" + opts.Branch
	}
	if gitRef == "" {
		gitRef = "HEAD"
	}

	data, err := runGitCmd(ctx, bareDir, opts, "show", gitRef+":"+ref.AssetPath)
	if err != nil {
		return nil, ArtifactMeta{}, fmt.Errorf("GIT_INDEX: artifact %q not found at %s:%s — %w",
			ref.Name, gitRef, ref.AssetPath, err)
	}

	return io.NopCloser(strings.NewReader(data)), ArtifactMeta{
		ContentLength: int64(len(data)),
		ContentType:   "application/octet-stream",
	}, nil
}

// ── Git command execution ───────────────────────────────────────────────────

// requireGitBinary checks that git is available on PATH.
func requireGitBinary() error {
	_, err := exec.LookPath("git")
	if err != nil {
		return fmt.Errorf("GIT_INDEX: git binary not found on PATH — install git to use GIT_INDEX sources")
	}
	return nil
}

// runGitCmd executes a git command with timeout and credential handling.
// Never logs credentials. Returns stdout as string.
func runGitCmd(ctx context.Context, workDir string, opts SourceOpts, args ...string) (string, error) {
	cmdCtx, cancel := context.WithTimeout(ctx, gitCmdTimeout)
	defer cancel()

	cmd := exec.CommandContext(cmdCtx, "git", args...)
	if workDir != "" {
		cmd.Dir = workDir
	}

	// Inject credentials via environment, never as command args.
	cmd.Env = append(os.Environ(),
		"GIT_TERMINAL_PROMPT=0",                // never prompt
		"GIT_SSH_COMMAND=ssh -o BatchMode=yes",  // non-interactive SSH
	)

	// HTTPS token auth via GIT_ASKPASS.
	if opts.AuthToken != "" && !strings.HasPrefix(opts.RepoURL, "ssh://") && !strings.Contains(opts.RepoURL, "@") {
		askpass := createAskpassScript(opts.AuthToken)
		if askpass != "" {
			cmd.Env = append(cmd.Env, "GIT_ASKPASS="+askpass)
			defer os.Remove(askpass)
		}
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		errMsg := strings.TrimSpace(stderr.String())
		// Redact any credentials from error output.
		if opts.AuthToken != "" {
			errMsg = strings.ReplaceAll(errMsg, opts.AuthToken, "[REDACTED]")
		}
		if cmdCtx.Err() == context.DeadlineExceeded {
			return "", fmt.Errorf("git command timed out after %s: %s", gitCmdTimeout, redactGitError(errMsg))
		}
		return "", fmt.Errorf("git %s failed: %s", args[0], redactGitError(errMsg))
	}

	return stdout.String(), nil
}

// createAskpassScript writes a temporary script that outputs the auth token.
// The script is deleted after the git command completes.
func createAskpassScript(token string) string {
	f, err := os.CreateTemp("", "git-askpass-*.sh")
	if err != nil {
		return ""
	}
	// The script prints the token when asked for a password.
	// This avoids embedding the token in the repo URL.
	fmt.Fprintf(f, "#!/bin/sh\necho '%s'\n", strings.ReplaceAll(token, "'", "'\\''"))
	f.Close()
	os.Chmod(f.Name(), 0o700)
	return f.Name()
}

// redactGitError removes common credential-containing patterns from git errors.
func redactGitError(msg string) string {
	// Remove URLs with embedded credentials.
	if strings.Contains(msg, "@") {
		// Simple redaction: replace user:pass@host patterns.
		for _, prefix := range []string{"https://", "http://", "ssh://"} {
			if idx := strings.Index(msg, prefix); idx >= 0 {
				rest := msg[idx+len(prefix):]
				if atIdx := strings.Index(rest, "@"); atIdx >= 0 {
					msg = msg[:idx+len(prefix)] + "[REDACTED]@" + rest[atIdx+1:]
				}
			}
		}
	}
	return msg
}
