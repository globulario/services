package deploy

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// LatestBuildInfo holds the result of querying the latest build.
type LatestBuildInfo struct {
	BuildNumber int64
	BuildID     string
	Checksum    string
}

// QueryLatestBuild queries the repository for the latest build of an artifact.
// Returns zero-value LatestBuildInfo if no builds exist yet.
func QueryLatestBuild(ctx context.Context, client *repository_client.Repository_Service_Client, publisher, name, version, platform string) (*LatestBuildInfo, error) {
	ref := &repopb.ArtifactRef{
		PublisherId: publisher,
		Name:        name,
		Version:     version,
		Platform:    platform,
		Kind:        repopb.ArtifactKind_SERVICE,
	}

	manifest, err := client.GetArtifactManifest(ref, 0)
	if err != nil {
		// No artifact exists yet — first publish.
		return &LatestBuildInfo{}, nil
	}
	if manifest == nil {
		return &LatestBuildInfo{}, nil
	}
	return &LatestBuildInfo{
		BuildNumber: manifest.GetBuildNumber(),
		BuildID:     manifest.GetBuildId(),
		Checksum:    manifest.GetChecksum(),
	}, nil
}

// NextBuildNumber returns the next build number for a service.
// Deprecated: build_number is display-only. Repository allocates build_id on upload.
func NextBuildNumber(ctx context.Context, client *repository_client.Repository_Service_Client, publisher, name, version, platform string) (int64, string, error) {
	info, err := QueryLatestBuild(ctx, client, publisher, name, version, platform)
	if err != nil {
		return 0, "", fmt.Errorf("query latest build: %w", err)
	}
	return info.BuildNumber + 1, info.Checksum, nil
}
