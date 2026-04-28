package upstream

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
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
	githubAPIBase    = "https://api.github.com"
	githubMaxResponse = 1 << 20 // 1 MiB
	githubTimeout    = 15 * time.Second
)

// FetchLatestRelease queries the GitHub API for the latest release.
// When includePrerelease is false, uses the /releases/latest endpoint which
// skips prereleases and drafts. When true, lists all releases and returns the
// first non-draft entry (which may be a prerelease).
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

	// Default: /releases/latest skips prereleases and drafts.
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
// Returns operator-friendly errors for common failure modes.
// Never logs or returns the auth token.
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
