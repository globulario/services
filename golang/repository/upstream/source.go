// @awareness namespace=globular.platform
// @awareness component=platform_repository.upstream.source_registry
// @awareness file_role=provider_neutral_release_source_interface_and_registry
// @awareness implements=globular.platform:intent.upstream_release_streams.must_be_provider_neutral
// @awareness risk=high
package upstream

// source.go — the ONLY abstraction the repository sync pipeline uses to
// reach an upstream release stream. No sync/import code path may import
// a concrete provider — they all flow through ReleaseSource. The
// constants TypeGitHubRelease/HTTPIndex/GitIndex/LocalDir are the
// closed enum of supported providers; NewSource MUST refuse anything
// outside the enum (no string-based fallback). Adding a 5th provider
// means adding a case here AND a new implementation file — never an
// inline conditional in the sync pipeline.

import (
	"context"
	"fmt"
	"io"
)

// ReleaseSource is the provider-neutral abstraction for upstream release streams.
// Implementations exist for GitHub Releases, HTTP indexes, local directories,
// and (future) Git indexes.
//
// The repository sync pipeline uses this interface exclusively — no provider-
// specific code leaks into the sync/import core. Controller and node-agent
// never interact with providers directly.
type ReleaseSource interface {
	// Type returns the provider type identifier (e.g. "GITHUB_RELEASE", "HTTP_INDEX").
	Type() string

	// ListReleases returns available release tags from the source.
	// Not all providers support listing — some return ErrListUnsupported.
	ListReleases(ctx context.Context, opts SourceOpts) ([]ReleaseRef, error)

	// GetReleaseIndex fetches the raw release-index.json bytes for a tag.
	GetReleaseIndex(ctx context.Context, opts SourceOpts, tag string) ([]byte, error)

	// OpenArtifact opens a package artifact for streaming download.
	// The ArtifactRef carries full context so providers can resolve paths.
	// The caller must close the returned ReadCloser.
	OpenArtifact(ctx context.Context, opts SourceOpts, ref ArtifactRef) (io.ReadCloser, ArtifactMeta, error)
}

// ErrListUnsupported is returned by providers that do not support ListReleases.
var ErrListUnsupported = fmt.Errorf("this provider does not support listing releases — use an explicit tag")

// ErrProviderUnimplemented is returned for provider types that are not yet implemented.
var ErrProviderUnimplemented = fmt.Errorf("provider type not yet implemented")

// SourceOpts carries provider-neutral configuration.
// Constructed from the proto UpstreamSource by the server-side mapping code.
type SourceOpts struct {
	// Common
	IndexURL          string // URL template with {tag} placeholder
	IndexPathTemplate string // Path template within repo/dir: "releases/{tag}/release-index.json"
	Platform          string

	// Credentials — provider interprets as appropriate.
	// AuthToken is for HTTP Bearer auth (GitHub, HTTP).
	// CredentialsRef is the raw ref for future SSH key support.
	AuthToken      string
	CredentialsRef string

	// GitHub-specific
	Owner              string
	Repo               string
	IncludePrereleases bool

	// Git-specific
	RepoURL  string
	Branch   string
	CacheDir string // per-source cache directory for Git clones

	// HTTP-specific
	ArtifactBaseURL string

	// Local-specific
	LocalRoot string
}

// ReleaseRef is a lightweight reference to a discovered release.
type ReleaseRef struct {
	Tag        string
	Name       string
	Prerelease bool
}

// ArtifactRef carries the full context needed to locate and verify a package artifact.
type ArtifactRef struct {
	AssetURL      string // absolute URL (HTTP/HTTPS) — used by GitHub, HTTP providers
	AssetPath     string // relative path within source — used by LOCAL_DIR, GIT_INDEX
	Filename      string // archive filename
	ReleaseTag    string // platform release tag
	OriginRelease string // release where artifact was originally built
	Name          string // package name
	Version       string // package version
	Platform      string // target platform
	Sha256        string // expected sha256 for verification
}

// ArtifactMeta is returned alongside the artifact stream.
type ArtifactMeta struct {
	ContentLength int64  // -1 if unknown
	ContentType   string // e.g. "application/gzip"
}

// ── Provider registry ───────────────────────────────────────────────────────

// SourceTypeID maps proto enum names to provider constructors.
// Using explicit string mapping avoids depending on proto in this package.
const (
	TypeGitHubRelease = "GITHUB_RELEASE"
	TypeHTTPIndex     = "HTTP_INDEX"
	TypeGitIndex      = "GIT_INDEX"
	TypeLocalDir      = "LOCAL_DIR"
)

// NewSource creates a ReleaseSource for the given provider type.
// Uses explicit enum mapping — does not accept arbitrary strings.
func NewSource(sourceType string) (ReleaseSource, error) {
	switch sourceType {
	case TypeGitHubRelease:
		return &GitHubSource{}, nil
	case TypeHTTPIndex:
		return &HTTPIndexSource{}, nil
	case TypeLocalDir:
		return &LocalDirSource{}, nil
	case TypeGitIndex:
		return &GitIndexSource{}, nil
	case "", "UPSTREAM_TYPE_UNSPECIFIED":
		return nil, fmt.Errorf("upstream source type is not set — specify a provider type")
	default:
		return nil, fmt.Errorf("unknown upstream source type %q", sourceType)
	}
}

// MapProtoType converts a proto enum int32 value to the canonical string type.
// This is the single mapping point — sync_from_upstream.go calls this instead
// of using src.GetType().String() directly.
func MapProtoType(protoValue int32) string {
	switch protoValue {
	case 1:
		return TypeGitHubRelease
	case 2:
		return TypeHTTPIndex
	case 3:
		return TypeGitIndex
	case 4:
		return TypeLocalDir
	default:
		return ""
	}
}
