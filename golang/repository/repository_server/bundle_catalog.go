package main

// bundle_catalog.go — ListBundles handler for the Repository PackageRepository service.
//
// Retrieves the bundle inventory from the Resource service (local_resource.Bundles)
// and returns lightweight BundleSummary records for the admin UI catalog.

import (
	"context"
	"log/slog"
	"strings"

	"github.com/globulario/services/golang/plan/versionutil"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// ListBundles returns a summary of all bundles stored in the repository.
// It queries the Resource service for the bundle index and maps each record
// to a BundleSummary. If the Resource service is unreachable or the index is
// empty, an empty list is returned without error so the UI can show an
// appropriate empty-state message.
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
