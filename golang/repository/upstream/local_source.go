// @awareness namespace=globular.platform
// @awareness component=platform_repository.upstream.local_source
// @awareness file_role=local_filesystem_release_source_with_strict_path_traversal_protection
// @awareness implements=globular.platform:intent.upstream_release_streams.must_be_provider_neutral
// @awareness enforces=globular.platform:invariant.repository.upstream_local_paths_must_reject_traversal
// @awareness risk=critical
package upstream

// local_source.go — LOCAL_DIR provider for air-gapped / USB / offline
// release streams.
//
// Security-critical: this provider reads files from operator-supplied
// paths. The two guards that MUST stay in place:
//
//  1. validateLocalRoot insists on an absolute path that resolves to
//     an existing directory. A relative or empty root would let
//     subsequent joins escape the intended sandbox.
//
//  2. safeJoin refuses absolute paths, refuses any `..` segment, and
//     evaluates symlinks before checking containment. A naïve
//     filepath.Join + HasPrefix check would let a symlink inside the
//     root point outside it, silently letting an operator-uploaded
//     archive read arbitrary host files.
//
// HTTPS asset_url is rejected — LOCAL_DIR serves local files only.
// The HTTP family of sources is reachable through HTTPIndexSource;
// LOCAL_DIR must never become a back-door HTTP client.

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// LocalDirSource implements ReleaseSource for local filesystem directories.
// Used for air-gapped / USB / offline release streams.
//
// All paths are resolved relative to opts.LocalRoot with strict path traversal
// protection: the resolved path must remain under LocalRoot after cleaning,
// symlink evaluation, and normalization.
type LocalDirSource struct{}

func (s *LocalDirSource) Type() string { return TypeLocalDir }

// ListReleases lists release directories under local_root.
// Expects directory structure: {local_root}/releases/{tag}/release-index.json
// or uses index_path_template to discover tags.
func (s *LocalDirSource) ListReleases(ctx context.Context, opts SourceOpts) ([]ReleaseRef, error) {
	root, err := validateLocalRoot(opts.LocalRoot)
	if err != nil {
		return nil, err
	}

	// Try to list directories under releases/
	releasesDir := filepath.Join(root, "releases")
	entries, readErr := os.ReadDir(releasesDir)
	if readErr != nil {
		return nil, fmt.Errorf("LOCAL_DIR: cannot list releases at %s: %w", releasesDir, readErr)
	}

	var refs []ReleaseRef
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		tag := e.Name()
		// Verify release-index.json exists for this tag.
		indexPath := filepath.Join(releasesDir, tag, "release-index.json")
		if _, statErr := os.Stat(indexPath); statErr != nil {
			continue
		}
		refs = append(refs, ReleaseRef{Tag: tag, Name: tag})
	}
	return refs, nil
}

// GetReleaseIndex reads release-index.json from the local filesystem.
func (s *LocalDirSource) GetReleaseIndex(ctx context.Context, opts SourceOpts, tag string) ([]byte, error) {
	path, err := resolveLocalPath(opts.LocalRoot, opts.IndexPathTemplate, tag)
	if err != nil {
		return nil, fmt.Errorf("LOCAL_DIR: %w", err)
	}
	data, readErr := os.ReadFile(path)
	if readErr != nil {
		if os.IsNotExist(readErr) {
			// Definitive absence (no index for this tag) — provider-neutral
			// NotFound, not a transient read failure. Callers must not retry.
			return nil, fmt.Errorf("LOCAL_DIR: release-index.json for tag %q not found: %w", tag, ErrNotFound)
		}
		return nil, fmt.Errorf("LOCAL_DIR: read release-index.json: %w", readErr)
	}
	return data, nil
}

// OpenArtifact opens a package artifact from the local filesystem.
// Resolution order:
//  1. ref.AssetPath — resolved relative to LocalRoot
//  2. ref.Filename — resolved relative to LocalRoot
//  3. ref.AssetURL with file:// scheme — resolved to absolute path under LocalRoot
//
// Absolute asset_url (http/https) is rejected — LOCAL_DIR only serves local files.
func (s *LocalDirSource) OpenArtifact(ctx context.Context, opts SourceOpts, ref ArtifactRef) (io.ReadCloser, ArtifactMeta, error) {
	root, err := validateLocalRoot(opts.LocalRoot)
	if err != nil {
		return nil, ArtifactMeta{}, err
	}

	var targetPath string
	switch {
	case ref.AssetPath != "":
		targetPath, err = safeJoin(root, ref.AssetPath)
	case ref.Filename != "":
		targetPath, err = safeJoin(root, ref.Filename)
	case strings.HasPrefix(ref.AssetURL, "file://"):
		filePath := strings.TrimPrefix(ref.AssetURL, "file://")
		targetPath, err = safeJoin(root, filePath)
	default:
		return nil, ArtifactMeta{}, fmt.Errorf("LOCAL_DIR: cannot resolve artifact for %s — no asset_path, filename, or file:// URL", ref.Name)
	}
	if err != nil {
		return nil, ArtifactMeta{}, fmt.Errorf("LOCAL_DIR: %w", err)
	}

	f, openErr := os.Open(targetPath)
	if openErr != nil {
		return nil, ArtifactMeta{}, fmt.Errorf("LOCAL_DIR: open artifact: %w", openErr)
	}

	info, statErr := f.Stat()
	if statErr != nil {
		f.Close()
		return nil, ArtifactMeta{}, fmt.Errorf("LOCAL_DIR: stat artifact: %w", statErr)
	}

	meta := ArtifactMeta{
		ContentLength: info.Size(),
		ContentType:   "application/gzip",
	}
	return f, meta, nil
}

// ── Path safety ─────────────────────────────────────────────────────────────

// validateLocalRoot ensures the local root is an absolute path to an existing directory.
func validateLocalRoot(root string) (string, error) {
	if root == "" {
		return "", fmt.Errorf("LOCAL_DIR: local_root is required")
	}
	if !filepath.IsAbs(root) {
		return "", fmt.Errorf("LOCAL_DIR: local_root must be an absolute path (got %q)", root)
	}
	cleaned := filepath.Clean(root)
	info, err := os.Stat(cleaned)
	if err != nil {
		return "", fmt.Errorf("LOCAL_DIR: local_root %q: %w", cleaned, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("LOCAL_DIR: local_root %q is not a directory", cleaned)
	}
	return cleaned, nil
}

// safeJoin joins a root and a relative path, then validates the result is
// under root after cleaning and symlink evaluation. Rejects path traversal.
func safeJoin(root, relative string) (string, error) {
	if filepath.IsAbs(relative) {
		return "", fmt.Errorf("path traversal rejected: absolute paths not allowed (%q)", relative)
	}
	if strings.Contains(relative, "..") {
		return "", fmt.Errorf("path traversal rejected: '..' not allowed in path (%q)", relative)
	}

	joined := filepath.Join(root, filepath.Clean(relative))

	// Evaluate symlinks to catch symlink-based traversal.
	resolved, err := filepath.EvalSymlinks(joined)
	if err != nil {
		// File may not exist yet — fall back to cleaned join.
		resolved = joined
	}

	resolvedRoot, _ := filepath.EvalSymlinks(root)
	if resolvedRoot == "" {
		resolvedRoot = root
	}

	if !strings.HasPrefix(resolved, resolvedRoot+string(filepath.Separator)) && resolved != resolvedRoot {
		return "", fmt.Errorf("path traversal rejected: resolved path %q is outside root %q", resolved, resolvedRoot)
	}

	return joined, nil
}

// resolveLocalPath resolves a release-index path from template + tag under root.
func resolveLocalPath(localRoot, template, tag string) (string, error) {
	root, err := validateLocalRoot(localRoot)
	if err != nil {
		return "", err
	}
	if template == "" {
		template = "releases/{tag}/release-index.json"
	}
	relative := strings.ReplaceAll(template, "{tag}", tag)
	return safeJoin(root, relative)
}
