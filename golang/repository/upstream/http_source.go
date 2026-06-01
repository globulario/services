// @awareness namespace=globular.platform
// @awareness component=platform_repository.upstream
// @awareness file_role=http_artifact_upstream_implementation
// @awareness implements=globular.platform:intent.upstream_release_streams.must_be_provider_neutral
// @awareness risk=medium
package upstream

import (
	"context"
	"fmt"
	"io"
	"strings"
)

// HTTPIndexSource implements ReleaseSource for generic HTTP/HTTPS endpoints.
// The index_url template with {tag} substitution is the primary configuration.
// Artifacts are fetched from asset_url (absolute) or artifact_base_url + asset_path.
type HTTPIndexSource struct{}

func (s *HTTPIndexSource) Type() string { return TypeHTTPIndex }

// ListReleases is not supported for HTTP_INDEX — requires explicit tag.
func (s *HTTPIndexSource) ListReleases(ctx context.Context, opts SourceOpts) ([]ReleaseRef, error) {
	return nil, ErrListUnsupported
}

// GetReleaseIndex fetches release-index.json from the URL template.
func (s *HTTPIndexSource) GetReleaseIndex(ctx context.Context, opts SourceOpts, tag string) ([]byte, error) {
	if opts.IndexURL == "" {
		return nil, fmt.Errorf("HTTP_INDEX: index_url is required")
	}
	url := strings.ReplaceAll(opts.IndexURL, "{tag}", tag)
	return httpGet(url, opts.AuthToken)
}

// OpenArtifact opens a package artifact for streaming download.
// Resolution order:
//  1. ref.AssetURL (absolute URL) — used directly
//  2. opts.ArtifactBaseURL + ref.AssetPath — composed URL
func (s *HTTPIndexSource) OpenArtifact(ctx context.Context, opts SourceOpts, ref ArtifactRef) (io.ReadCloser, ArtifactMeta, error) {
	url := resolveHTTPAssetURL(opts, ref)
	if url == "" {
		return nil, ArtifactMeta{}, fmt.Errorf("HTTP_INDEX: cannot resolve artifact URL for %s (no asset_url and no artifact_base_url + asset_path)", ref.Name)
	}
	return httpOpen(url, opts.AuthToken)
}

// resolveHTTPAssetURL resolves the download URL for an artifact.
func resolveHTTPAssetURL(opts SourceOpts, ref ArtifactRef) string {
	// Prefer absolute asset_url.
	if ref.AssetURL != "" {
		return ref.AssetURL
	}
	// Compose from base + path.
	if opts.ArtifactBaseURL != "" && ref.AssetPath != "" {
		base := strings.TrimSuffix(opts.ArtifactBaseURL, "/")
		path := strings.TrimPrefix(ref.AssetPath, "/")
		return base + "/" + path
	}
	// Compose from base + filename.
	if opts.ArtifactBaseURL != "" && ref.Filename != "" {
		base := strings.TrimSuffix(opts.ArtifactBaseURL, "/")
		return base + "/" + ref.Filename
	}
	return ""
}
