// @awareness namespace=globular.platform
// @awareness component=platform_repository.upstream.parse_and_redact
// @awareness file_role=stdlib_only_helpers_for_repo_url_parsing_and_audit_log_token_redaction
// @awareness enforces=globular.platform:invariant.repository.upstream_credentials_must_be_redacted_in_audit_logs
// @awareness risk=high
package upstream

// parse.go — pure helpers shared by every upstream provider. Two
// load-bearing properties:
//
//  1. RedactAssetURL strips query parameters and userinfo from a URL
//     BEFORE it appears in any log or audit record. GitHub presigned
//     asset URLs carry tokens in the query string; logging the raw
//     URL leaks credentials to operators and (worse) into shipped
//     log archives. Every code path that logs an asset URL MUST go
//     through this helper.
//
//  2. ValidateIndexURLTemplate rejects index URL templates that lack
//     `{tag}`, contain unbalanced braces, or carry stray braces
//     outside the placeholder. A bad template can produce
//     ambiguous fetch URLs that select the wrong artifact.
//
// Package contract: stdlib-only. No repository_server / etcd / MinIO /
// ScyllaDB imports here — keeping the helper-set portable and
// individually testable is the whole reason this package exists.

import (
	"fmt"
	"net/url"
	"strings"
)

// ParseRepoURL extracts owner and repo from various GitHub URL formats:
//
//	"owner/repo"
//	"https://github.com/owner/repo"
//	"https://github.com/owner/repo.git"
//	"github.com/owner/repo"
func ParseRepoURL(repoURL string) (owner, repo string, err error) {
	s := strings.TrimSpace(repoURL)
	if s == "" {
		return "", "", fmt.Errorf("repo_url is empty")
	}

	// Try parsing as URL first.
	if strings.Contains(s, "://") || strings.HasPrefix(s, "github.com/") {
		if !strings.Contains(s, "://") {
			s = "https://" + s
		}
		u, parseErr := url.Parse(s)
		if parseErr != nil {
			return "", "", fmt.Errorf("invalid repo_url %q: %w", repoURL, parseErr)
		}
		if u.Host != "github.com" && u.Host != "www.github.com" {
			return "", "", fmt.Errorf("repo_url host must be github.com (got %q)", u.Host)
		}
		s = strings.TrimPrefix(u.Path, "/")
	}

	s = strings.TrimSuffix(s, ".git")
	s = strings.TrimSuffix(s, "/")

	parts := strings.Split(s, "/")
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", fmt.Errorf("repo_url must be \"owner/repo\" (got %q)", repoURL)
	}
	return parts[0], parts[1], nil
}

// RedactAssetURL strips query parameters from a URL for safe logging/audit.
// GitHub asset URLs may contain tokens in query params for private repos.
func RedactAssetURL(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err != nil {
		return "(invalid-url)"
	}
	u.RawQuery = ""
	u.Fragment = ""
	if u.User != nil {
		u.User = url.User("REDACTED")
	}
	return u.String()
}

// DeriveIndexURL builds the default release-index.json URL template for a
// GitHub repository. The {tag} placeholder is substituted at sync time.
func DeriveIndexURL(owner, repo string) string {
	return fmt.Sprintf("https://github.com/%s/%s/releases/download/{tag}/release-index.json", owner, repo)
}

// ValidateIndexURLTemplate validates an upstream index URL template.
// Rules:
// - must include "{tag}" placeholder
// - braces must be balanced
// - no stray braces outside "{tag}"
func ValidateIndexURLTemplate(raw string) error {
	s := strings.TrimSpace(raw)
	if s == "" {
		return fmt.Errorf("index_url is empty")
	}
	if !strings.Contains(s, "{tag}") {
		return fmt.Errorf("index_url must contain a {tag} placeholder")
	}
	if strings.Count(s, "{") != strings.Count(s, "}") {
		return fmt.Errorf("index_url has unbalanced braces")
	}
	withoutTag := strings.ReplaceAll(s, "{tag}", "")
	if strings.Contains(withoutTag, "{") || strings.Contains(withoutTag, "}") {
		return fmt.Errorf("index_url contains stray braces outside {tag}")
	}
	return nil
}
