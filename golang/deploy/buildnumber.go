package deploy

import (
	"context"
	"fmt"

	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

// QueryLatestBuild queries the repository for the latest build number of an artifact.
// Returns 0 if no builds exist yet.
func QueryLatestBuild(ctx context.Context, client *repository_client.Repository_Service_Client, publisher, name, version, platform string) (int64, string, error) {
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
		return 0, "", nil
	}
	if manifest == nil {
		return 0, "", nil
	}
	return manifest.GetBuildNumber(), manifest.GetChecksum(), nil
}

// NextBuildNumber returns the next build number for a service.
func NextBuildNumber(ctx context.Context, client *repository_client.Repository_Service_Client, publisher, name, version, platform string) (int64, string, error) {
	current, checksum, err := QueryLatestBuild(ctx, client, publisher, name, version, platform)
	if err != nil {
		return 0, "", fmt.Errorf("query latest build: %w", err)
	}
	return current + 1, checksum, nil
}
