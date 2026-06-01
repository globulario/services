// @awareness namespace=globular.platform
// @awareness component=platform_repository.upstream
// @awareness file_role=upstream_sync_execution
// @awareness implements=globular.platform:intent.upstream_release_streams.must_be_provider_neutral
// @awareness risk=high
package main

// repository_source_upstream.go — UpstreamRepositorySource wraps upstream.ReleaseSource
// for on-demand artifact resolution. Reuses existing release-index parsing,
// provider construction, and credential resolution from sync_from_upstream.go.
//
// Open() returns a streaming io.ReadCloser — the resolver handles
// materialization to local POSIX CAS. No buffering of full artifacts here.

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	uppkg "github.com/globulario/services/golang/repository/upstream"
)

// UpstreamRepositorySource resolves artifacts from a configured upstream provider
// (GitHub Releases, HTTP index, local directory, or git index).
//
// It does not duplicate GitHub logic — it reuses uppkg.ReleaseSource directly.
// GitHub source only works when the release contains a release-index.json.
type UpstreamRepositorySource struct {
	srv      *server
	src      *repopb.UpstreamSource
	provider uppkg.ReleaseSource
	opts     uppkg.SourceOpts
}

func newUpstreamSource(srv *server, src *repopb.UpstreamSource, provider uppkg.ReleaseSource, opts uppkg.SourceOpts) *UpstreamRepositorySource {
	return &UpstreamRepositorySource{srv: srv, src: src, provider: provider, opts: opts}
}

func (s *UpstreamRepositorySource) Name() string {
	if s.src != nil {
		return s.src.GetName()
	}
	return "upstream"
}

func (s *UpstreamRepositorySource) Type() string {
	if s.src == nil {
		return "UPSTREAM"
	}
	return uppkg.MapProtoType(int32(s.src.GetType()))
}

func (s *UpstreamRepositorySource) Priority() int { return 30 }

func (s *UpstreamRepositorySource) Health(ctx context.Context) SourceHealth {
	if s.src == nil || !s.src.GetEnabled() {
		return SourceHealth{Available: false, Reason: "upstream source disabled or not configured"}
	}
	if s.provider == nil {
		return SourceHealth{Available: false, Reason: "provider not initialized"}
	}
	return SourceHealth{Available: true}
}

// Open resolves and streams one artifact from this upstream source.
//
// Resolution order:
//  1. If req.AssetURL or req.AssetPath is set (from manifest.UpstreamImport),
//     call provider.OpenArtifact directly — no index fetch needed.
//  2. Otherwise fetch release-index.json and find the matching entry.
//
// Returns ErrArtifactNotFound if the release has no release-index.json or
// no entry matching the request.
func (s *UpstreamRepositorySource) Open(ctx context.Context, req ArtifactRequest) (*ArtifactCandidate, error) {
	if s.src == nil || !s.src.GetEnabled() {
		return nil, fmt.Errorf("%w: %s", ErrSourceUnavailable, s.Name())
	}
	if req.AssetURL != "" || req.AssetPath != "" {
		return s.openDirect(ctx, req)
	}
	return s.openViaIndex(ctx, req)
}

func (s *UpstreamRepositorySource) openDirect(ctx context.Context, req ArtifactRequest) (*ArtifactCandidate, error) {
	ref := uppkg.ArtifactRef{
		AssetURL:   req.AssetURL,
		AssetPath:  req.AssetPath,
		Filename:   req.Filename,
		ReleaseTag: req.ReleaseTag,
		Name:       req.Name,
		Version:    req.Version,
		Platform:   req.Platform,
		Sha256:     req.Sha256,
	}
	rc, meta, err := s.provider.OpenArtifact(ctx, s.opts, ref)
	if err != nil {
		return nil, fmt.Errorf("%w: %s open artifact: %v", ErrArtifactNotFound, s.Name(), err)
	}
	size := req.SizeBytes
	if size == 0 && meta.ContentLength > 0 {
		size = meta.ContentLength
	}
	provenanceURL := req.AssetURL
	if provenanceURL == "" {
		provenanceURL = req.AssetPath
	}
	return &ArtifactCandidate{
		SourceName:    s.Name(),
		SourceType:    s.Type(),
		Reader:        rc,
		SizeBytes:     size,
		Sha256:        req.Sha256,
		ProvenanceURL: provenanceURL,
	}, nil
}

func (s *UpstreamRepositorySource) openViaIndex(ctx context.Context, req ArtifactRequest) (*ArtifactCandidate, error) {
	tag := req.ReleaseTag
	if tag == "" {
		tag = s.src.GetLastSyncedTag()
	}
	if tag == "" {
		return nil, fmt.Errorf("%w: no release tag for upstream source %s", ErrArtifactNotFound, s.Name())
	}

	idxBytes, err := s.provider.GetReleaseIndex(ctx, s.opts, tag)
	if err != nil {
		return nil, fmt.Errorf("%w: %s fetch release-index %s: %v", ErrArtifactNotFound, s.Name(), tag, err)
	}
	if len(idxBytes) == 0 {
		return nil, fmt.Errorf("%w: GitHub release %s has no release-index.json", ErrArtifactNotFound, tag)
	}

	idx, parseErr := parseReleaseIndex(idxBytes)
	if parseErr != nil {
		return nil, fmt.Errorf("%w: %s parse release-index %s: %v", ErrSourceMisconfigured, s.Name(), tag, parseErr)
	}

	var aliasRec *releaseBuildAliasRecord
	if req.BuildNumber > 0 {
		ref := &repopb.ArtifactRef{
			PublisherId: req.PublisherID,
			Name:        req.Name,
			Version:     req.Version,
			Platform:    req.Platform,
		}
		aliasRec, _ = s.srv.loadReleaseBuildAlias(ctx, ref, tag, req.BuildNumber)
	}

	n := s.findEntry(idx, req, aliasRec)
	if n == nil {
		return nil, fmt.Errorf("%w: %s release %s has no entry for %s/%s/%s",
			ErrArtifactNotFound, s.Name(), tag, req.Name, req.Version, req.Platform)
	}
	if n.Digest == "" {
		slog.Warn("repository-source: upstream entry has no checksum",
			"source", s.Name(), "name", n.Name, "version", n.Version)
		return nil, fmt.Errorf("%w: %s/%s has no checksum in release-index", ErrChecksumUnknown, req.Name, req.Version)
	}

	ref := uppkg.ArtifactRef{
		AssetURL:      n.AssetURL,
		AssetPath:     n.AssetPath,
		Filename:      n.Filename,
		ReleaseTag:    n.ReleaseTag,
		OriginRelease: n.OriginRelease,
		Name:          n.Name,
		Version:       n.Version,
		Platform:      n.Platform,
		Sha256:        n.Digest,
	}
	rc, meta, err := s.provider.OpenArtifact(ctx, s.opts, ref)
	if err != nil {
		return nil, fmt.Errorf("%w: %s open artifact %s: %v", ErrArtifactNotFound, s.Name(), n.Name, err)
	}

	size := int64(0)
	if meta.ContentLength > 0 {
		size = meta.ContentLength
	}

	provenanceURL := resolveProvenanceAssetURL(n)
	if provenanceURL == "" {
		provenanceURL = strings.ReplaceAll(s.src.GetIndexUrl(), "{tag}", tag)
	}

	return &ArtifactCandidate{
		SourceName:    s.Name(),
		SourceType:    s.Type(),
		Reader:        rc,
		SizeBytes:     size,
		Sha256:        n.Digest,
		ProvenanceURL: provenanceURL,
	}, nil
}

func (s *UpstreamRepositorySource) findEntry(idx *releaseIndex, req ArtifactRequest, aliasRec *releaseBuildAliasRecord) *normalizedEntry {
	for _, entry := range idx.Packages {
		if !strings.EqualFold(entry.Name, req.Name) {
			continue
		}
		if entry.Version != req.Version || entry.Platform != req.Platform {
			continue
		}
		n := normalizeReleaseEntry(entry, s.src)
		if req.BuildNumber > 0 && n.BuildNumber != req.BuildNumber {
			continue
		}
		if req.Sha256 != "" && n.Digest != "" && n.Digest != req.Sha256 {
			continue
		}
		if req.BuildID != "" && n.BuildID != req.BuildID {
			// Alias-aware match: allow upstream build_id when alias maps this
			// release_tag/build_number locator to the requested canonical build_id.
			if aliasRec == nil ||
				aliasRec.CanonicalBuildID != req.BuildID ||
				aliasRec.UpstreamBuildID == "" ||
				aliasRec.UpstreamBuildID != n.BuildID {
				continue
			}
		}
		return n
	}
	return nil
}

// loadUpstreamSources returns UpstreamRepositorySource wrappers for all enabled
// upstream sources registered in etcd.
func (srv *server) loadUpstreamSources(ctx context.Context) []*UpstreamRepositorySource {
	srcs, err := srv.scanAllUpstreamSources(ctx)
	if err != nil {
		slog.Warn("repository-source: cannot list upstream sources", "err", err)
		return nil
	}
	var out []*UpstreamRepositorySource
	for _, src := range srcs {
		if !src.GetEnabled() {
			continue
		}
		provType := uppkg.MapProtoType(int32(src.GetType()))
		provider, err := uppkg.NewSource(provType)
		if err != nil {
			slog.Warn("repository-source: cannot create provider", "source", src.GetName(), "err", err)
			continue
		}
		var authToken string
		if credRef := src.GetCredentialsRef(); credRef != "" {
			if tok, _ := resolveCredentialFromEtcd(ctx, credRef); tok != "" {
				authToken = tok
			}
		}
		out = append(out, newUpstreamSource(srv, src, provider, sourceOptsFromProto(src, authToken)))
	}
	return out
}
