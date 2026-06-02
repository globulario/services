// @awareness namespace=globular.platform
// @awareness component=platform_repository.upstream.github_source
// @awareness file_role=github_releases_api_implementation_of_release_source
// @awareness implements=globular.platform:intent.upstream_release_streams.must_be_provider_neutral
// @awareness enforces=globular.platform:invariant.repository.upstream_credentials_must_be_redacted_in_audit_logs
// @awareness risk=high
package upstream

// github.go — GitHub Releases API implementation of ReleaseSource.
// GitHub-specific knowledge MUST stay confined here: every other
// upstream consumer talks to the ReleaseSource interface, not the
// GitHub API.
//
// Token discipline: AuthToken is sent as a Bearer header to
// api.github.com endpoints (REST API) and also forwarded to the
// asset BrowserDownloadURL for private-repo asset fetches. Both
// paths share the same token; do NOT log the URL or header. Asset
// URLs returned to callers must be redacted via parse.go's
// RedactAssetURL before they appear in any operator log.
//
// All HTTP requests are bounded by githubTimeout (15s for API,
// httpTimeout 30s for asset download). Response bodies are capped
// at githubMaxResponse (1 MiB) for API responses and maxIndexBytes
// (10 MiB) for release-index.json — a malformed/oversized response
// must not exhaust process memory.

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// GitHubRelease represents a subset of the GitHub Release API response.
type GitHubRelease struct {
	TagName     string        `json:"tag_name"`
	Name        string        `json:"name"`
	Prerelease  bool          `json:"prerelease"`
	Draft       bool          `json:"draft"`
	Assets      []GitHubAsset `json:"assets"`
	PublishedAt string        `json:"published_at"`
}

// GitHubAsset represents a downloadable asset within a GitHub Release.
type GitHubAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Size               int64  `json:"size"`
}

const (
	githubAPIBase     = "https://api.github.com"
	githubMaxResponse = 1 << 20 // 1 MiB
	githubTimeout     = 15 * time.Second
	httpTimeout       = 30 * time.Second
	maxIndexBytes     = 10 << 20 // 10 MiB
)

// ── GitHubSource implements ReleaseSource ───────────────────────────────────

// GitHubSource provides release-index and artifact access via GitHub Releases API.
type GitHubSource struct{}

func (s *GitHubSource) Type() string { return TypeGitHubRelease }

func (s *GitHubSource) ListReleases(ctx context.Context, opts SourceOpts) ([]ReleaseRef, error) {
	if opts.Owner == "" || opts.Repo == "" {
		return nil, fmt.Errorf("GITHUB_RELEASE requires owner and repo")
	}
	url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=30", githubAPIBase, opts.Owner, opts.Repo)
	data, err := doGitHubRequest(url, opts.AuthToken)
	if err != nil {
		return nil, err
	}
	var releases []GitHubRelease
	if err := json.Unmarshal(data, &releases); err != nil {
		return nil, fmt.Errorf("decode GitHub releases: %w", err)
	}
	var refs []ReleaseRef
	for _, r := range releases {
		if r.Draft {
			continue
		}
		if r.Prerelease && !opts.IncludePrereleases {
			continue
		}
		refs = append(refs, ReleaseRef{
			Tag:        r.TagName,
			Name:       r.Name,
			Prerelease: r.Prerelease,
		})
	}
	return refs, nil
}

func (s *GitHubSource) GetReleaseIndex(ctx context.Context, opts SourceOpts, tag string) ([]byte, error) {
	if opts.Owner != "" && opts.Repo != "" {
		// Use GitHub API to find release-index.json asset.
		release, err := FetchReleaseByTag(opts.Owner, opts.Repo, tag, opts.AuthToken)
		if err != nil {
			return nil, err
		}
		asset, assetErr := FindReleaseIndexAsset(release)
		if assetErr != nil {
			return nil, assetErr
		}
		return httpGet(asset.BrowserDownloadURL, opts.AuthToken)
	}
	// Fallback: use index_url template.
	if opts.IndexURL == "" {
		return nil, fmt.Errorf("GITHUB_RELEASE: no owner/repo and no index_url")
	}
	url := strings.ReplaceAll(opts.IndexURL, "{tag}", tag)
	return httpGet(url, opts.AuthToken)
}

func (s *GitHubSource) OpenArtifact(ctx context.Context, opts SourceOpts, ref ArtifactRef) (io.ReadCloser, ArtifactMeta, error) {
	if ref.AssetURL == "" {
		return nil, ArtifactMeta{}, fmt.Errorf("GITHUB_RELEASE: asset_url is required")
	}
	return httpOpen(ref.AssetURL, opts.AuthToken)
}

// ── Standalone functions (backward compat) ──────────────────────────────────

// FetchLatestRelease queries the GitHub API for the latest release.
func FetchLatestRelease(owner, repo string, includePrerelease bool, authToken string) (*GitHubRelease, error) {
	if includePrerelease {
		url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=10", githubAPIBase, owner, repo)
		data, err := doGitHubRequest(url, authToken)
		if err != nil {
			return nil, err
		}
		var releases []GitHubRelease
		if err := json.Unmarshal(data, &releases); err != nil {
			return nil, fmt.Errorf("decode GitHub releases list: %w", err)
		}
		for i := range releases {
			if !releases[i].Draft {
				return &releases[i], nil
			}
		}
		return nil, fmt.Errorf("no non-draft releases found for %s/%s", owner, repo)
	}
	url := fmt.Sprintf("%s/repos/%s/%s/releases/latest", githubAPIBase, owner, repo)
	data, err := doGitHubRequest(url, authToken)
	if err != nil {
		return nil, err
	}
	var release GitHubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("decode GitHub latest release: %w", err)
	}
	return &release, nil
}

// FetchReleaseByTag queries a specific release by tag name.
func FetchReleaseByTag(owner, repo, tag string, authToken string) (*GitHubRelease, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/releases/tags/%s", githubAPIBase, owner, repo, tag)
	data, err := doGitHubRequest(url, authToken)
	if err != nil {
		return nil, err
	}
	var release GitHubRelease
	if err := json.Unmarshal(data, &release); err != nil {
		return nil, fmt.Errorf("decode GitHub release for tag %q: %w", tag, err)
	}
	return &release, nil
}

// FindReleaseIndexAsset finds the release-index.json asset in a GitHub Release.
func FindReleaseIndexAsset(release *GitHubRelease) (*GitHubAsset, error) {
	if release == nil {
		return nil, fmt.Errorf("release is nil")
	}
	for i, a := range release.Assets {
		if a.Name == "release-index.json" {
			return &release.Assets[i], nil
		}
	}
	return nil, fmt.Errorf("GitHub release %q has no release-index.json asset", release.TagName)
}

// doGitHubRequest performs an authenticated GET to the GitHub API.
func doGitHubRequest(url, authToken string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build GitHub request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}

	client := &http.Client{Timeout: githubTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GitHub API unreachable: %w — check network connectivity", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK:
		// success
	case http.StatusNotFound:
		if authToken == "" {
			return nil, fmt.Errorf("GitHub release not found (404) — if this is a private repo, set credentials_ref")
		}
		return nil, fmt.Errorf("GitHub release not found (404) — verify the repo, tag, and token permissions")
	case http.StatusForbidden:
		remaining := resp.Header.Get("X-RateLimit-Remaining")
		reset := resp.Header.Get("X-RateLimit-Reset")
		if remaining == "0" {
			return nil, fmt.Errorf("GitHub API rate limited — resets at %s; use credentials_ref for higher limits", reset)
		}
		return nil, fmt.Errorf("GitHub API forbidden (403) — check token permissions")
	case http.StatusUnauthorized:
		return nil, fmt.Errorf("GitHub API unauthorized (401) — credentials_ref token may be expired or invalid")
	default:
		return nil, fmt.Errorf("GitHub API returned %d", resp.StatusCode)
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, githubMaxResponse+1))
	if err != nil {
		return nil, fmt.Errorf("read GitHub response: %w", err)
	}
	if len(data) > githubMaxResponse {
		return nil, fmt.Errorf("GitHub response exceeds %d bytes", githubMaxResponse)
	}
	return data, nil
}

// ── Shared HTTP helpers ─────────────────────────────────────────────────────

// httpGet fetches a URL with optional Bearer auth. Returns the response body.
func httpGet(url, authToken string) ([]byte, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("build request for %s: %w", RedactAssetURL(url), err)
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("GET %s: %w", RedactAssetURL(url), err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GET %s returned %d", RedactAssetURL(url), resp.StatusCode)
	}
	data, err := io.ReadAll(io.LimitReader(resp.Body, int64(maxIndexBytes)+1))
	if err != nil {
		return nil, fmt.Errorf("read %s: %w", RedactAssetURL(url), err)
	}
	if len(data) > maxIndexBytes {
		return nil, fmt.Errorf("response from %s exceeds %d bytes", RedactAssetURL(url), maxIndexBytes)
	}
	return data, nil
}

// httpOpen opens a streaming HTTP connection. Caller must close the reader.
func httpOpen(url, authToken string) (io.ReadCloser, ArtifactMeta, error) {
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		return nil, ArtifactMeta{}, fmt.Errorf("build request: %w", err)
	}
	if authToken != "" {
		req.Header.Set("Authorization", "Bearer "+authToken)
	}
	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, ArtifactMeta{}, fmt.Errorf("GET %s: %w", RedactAssetURL(url), err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return nil, ArtifactMeta{}, fmt.Errorf("GET %s returned %d", RedactAssetURL(url), resp.StatusCode)
	}
	meta := ArtifactMeta{
		ContentLength: resp.ContentLength,
		ContentType:   resp.Header.Get("Content-Type"),
	}
	return resp.Body, meta, nil
}
