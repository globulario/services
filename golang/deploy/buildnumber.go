package deploy

import (
	"context"
	"fmt"
	"strings"

	"github.com/globulario/services/golang/repository/repository_client"
	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/versionutil"
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

// LatestStableVersion returns the highest published STABLE version for a package.
// Local deploys must anchor to an existing release version rather than minting a
// new semver from a workstation.
func LatestStableVersion(ctx context.Context, client *repository_client.Repository_Service_Client, publisher, name, platform string) (string, error) {
	manifests, err := client.GetArtifactVersions(publisher, name, platform)
	if err != nil {
		return "", fmt.Errorf("list artifact versions: %w", err)
	}
	return selectLatestStableVersion(manifests)
}

// StableVersionExists reports whether version is already present in the STABLE
// published history for the package.
func StableVersionExists(ctx context.Context, client *repository_client.Repository_Service_Client, publisher, name, version, platform string) (bool, error) {
	manifests, err := client.GetArtifactVersions(publisher, name, platform)
	if err != nil {
		return false, fmt.Errorf("list artifact versions: %w", err)
	}
	return stableVersionExists(manifests, version), nil
}

func selectLatestStableVersion(manifests []*repopb.ArtifactManifest) (string, error) {
	best := ""
	for _, m := range manifests {
		if m == nil || !manifestIsStable(m) {
			continue
		}
		ref := m.GetRef()
		if ref == nil {
			continue
		}
		v := strings.TrimSpace(ref.GetVersion())
		if v == "" {
			continue
		}
		if best == "" {
			best = v
			continue
		}
		cmp, err := versionutil.Compare(v, best)
		if err != nil {
			continue
		}
		if cmp > 0 {
			best = v
		}
	}
	if best == "" {
		return "", fmt.Errorf("no published STABLE version found")
	}
	return best, nil
}

func stableVersionExists(manifests []*repopb.ArtifactManifest, version string) bool {
	want := strings.TrimSpace(version)
	if want == "" {
		return false
	}
	for _, m := range manifests {
		if m == nil || !manifestIsStable(m) {
			continue
		}
		ref := m.GetRef()
		if ref == nil {
			continue
		}
		if versionutil.Equal(ref.GetVersion(), want) {
			return true
		}
	}
	return false
}

func manifestIsStable(m *repopb.ArtifactManifest) bool {
	if m == nil {
		return false
	}
	ch := m.GetChannel()
	return ch == repopb.ArtifactChannel_CHANNEL_UNSET || ch == repopb.ArtifactChannel_STABLE
}
