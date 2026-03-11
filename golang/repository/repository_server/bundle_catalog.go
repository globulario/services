package main

// bundle_catalog.go — ListBundles handler for the Repository PackageRepository service.
//
// Deprecated: This handler serves legacy bundle summaries via the Resource service.
// The modern artifact catalog (ListArtifacts, SearchArtifacts) supersedes this.
// Retained for backward compatibility with older CLI versions and admin UI fallback.

import (
	"context"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/plan/versionutil"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ListBundles returns a summary of all bundles stored in the repository.
//
// Deprecated: Use ListArtifacts or SearchArtifacts instead. This method queries
// the legacy Resource service bundle index and will be removed in a future version.
func (srv *server) ListBundles(ctx context.Context, req *repopb.ListBundlesRequest) (*repopb.ListBundlesResponse, error) {
	resourceClient, err := srv.getResourceClient()
	if err != nil {
		slog.Warn("ListBundles: resource client unavailable", "err", err)
		return &repopb.ListBundlesResponse{}, nil
	}

	bundles, err := resourceClient.GetPackageBundles("")
	if err != nil {
		slog.Warn("ListBundles: GetPackageBundles failed", "err", err)
		return &repopb.ListBundlesResponse{}, nil
	}

	prefix := strings.ToLower(req.GetPrefix())
	summaries := make([]*repopb.BundleSummary, 0, len(bundles))
	for _, b := range bundles {
		if b.PackageDescriptor == nil {
			continue
		}
		name := b.PackageDescriptor.Name
		if prefix != "" && !strings.HasPrefix(strings.ToLower(name), prefix) {
			continue
		}
		ver := b.PackageDescriptor.Version
		if cv, err := versionutil.Canonical(ver); err == nil {
			ver = cv
		}
		summaries = append(summaries, &repopb.BundleSummary{
			Name:          name,
			Version:       ver,
			Platform:      b.Plaform,
			PublisherId:   b.PackageDescriptor.PublisherID,
			ServiceId:     b.PackageDescriptor.Id,
			SizeBytes:     int64(b.Size),
			PublishedUnix: b.Modified,
			Sha256:        b.Checksum,
		})
	}

	slog.Debug("ListBundles", "count", len(summaries))
	return &repopb.ListBundlesResponse{Bundles: summaries}, nil
}
