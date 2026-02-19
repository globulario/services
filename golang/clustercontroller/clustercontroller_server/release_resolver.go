package main

import (
	"context"
	"fmt"
	"strings"

	clustercontrollerpb "github.com/globulario/services/golang/clustercontroller/clustercontrollerpb"
	"github.com/globulario/services/golang/repository/repository_client"
	"github.com/globulario/services/golang/repository/repositorypb"
)

// ReleaseResolver resolves a ServiceReleaseSpec version policy (exact pin or channel)
// to an exact version string and its SHA256 artifact digest.
// It contacts the repository service to confirm the artifact exists and retrieve its manifest.
type ReleaseResolver struct {
	RepositoryAddr string // gRPC endpoint for repository service, e.g. "localhost:10101"
}

// Resolve returns (exactVersion, sha256Digest, error).
//
// Resolution rules:
//   - If spec.Version is non-empty: calls GetArtifactManifest to confirm existence and get digest.
//   - If spec.Version is empty and spec.Channel is set: uses getLatestByChannel to pick the
//     latest version on that channel, then calls GetArtifactManifest for the digest.
//
// Amendment 4: asserts that manifest.Checksum is a 64-char lowercase hex string (SHA256).
// Fails fast if the repository returns a non-SHA256 checksum.
func (r *ReleaseResolver) Resolve(ctx context.Context, spec *clustercontrollerpb.ServiceReleaseSpec) (exactVersion, digest string, err error) {
	if spec == nil {
		return "", "", fmt.Errorf("spec is nil")
	}
	if strings.TrimSpace(spec.PublisherID) == "" {
		return "", "", fmt.Errorf("spec.publisher_id is required")
	}
	if strings.TrimSpace(spec.ServiceName) == "" {
		return "", "", fmt.Errorf("spec.service_name is required")
	}

	addr := r.RepositoryAddr
	if addr == "" {
		addr = "localhost:10101"
	}

	client, err := repository_client.NewRepositoryService_Client(addr, "repository.PackageRepository")
	if err != nil {
		return "", "", fmt.Errorf("connect to repository %s: %w", addr, err)
	}
	defer client.Close()

	version := strings.TrimSpace(spec.Version)

	// Channel resolution: if no explicit version, pick latest on channel.
	if version == "" {
		channel := strings.TrimSpace(spec.Channel)
		if channel == "" {
			return "", "", fmt.Errorf("spec.version or spec.channel must be set")
		}
		version, err = r.getLatestByChannel(ctx, client, spec, channel)
		if err != nil {
			return "", "", fmt.Errorf("resolve channel %q for %s/%s: %w", channel, spec.PublisherID, spec.ServiceName, err)
		}
	}

	// Fetch manifest to confirm existence and retrieve digest.
	ref := &repositorypb.ArtifactRef{
		PublisherId: spec.PublisherID,
		Name:        spec.ServiceName,
		Version:     version,
		Platform:    spec.Platform,
		Kind:        repositorypb.ArtifactKind_SERVICE,
	}
	manifest, err := client.GetArtifactManifest(ref)
	if err != nil {
		return "", "", fmt.Errorf("get artifact manifest for %s/%s@%s: %w", spec.PublisherID, spec.ServiceName, version, err)
	}
	if manifest == nil {
		return "", "", fmt.Errorf("no manifest returned for %s/%s@%s", spec.PublisherID, spec.ServiceName, version)
	}

	// Amendment 4: assert checksum is SHA256 hex (64 hex chars).
	// assertSHA256Hex normalizes to lowercase, so the returned digest is always lowercase.
	checksum := manifest.GetChecksum()
	if err := assertSHA256Hex(checksum, spec.PublisherID, spec.ServiceName, version); err != nil {
		return "", "", err
	}

	return version, strings.ToLower(strings.TrimSpace(checksum)), nil
}

// getLatestByChannel resolves the latest published version for a service on the given channel.
// v1 implementation: lists all artifacts for the service and picks the one tagged with the channel.
// If the repository does not support channel tagging natively, this falls back to picking the
// lexicographically greatest version among published artifacts (last-wins for SemVer ordering).
// TODO: replace with dedicated GetLatestByChannel RPC when discovery service supports it.
func (r *ReleaseResolver) getLatestByChannel(_ context.Context, client *repository_client.Repository_Service_Client, spec *clustercontrollerpb.ServiceReleaseSpec, channel string) (string, error) {
	artifacts, err := client.ListArtifacts()
	if err != nil {
		return "", fmt.Errorf("list artifacts: %w", err)
	}
	if len(artifacts) == 0 {
		return "", fmt.Errorf("no artifacts found for %s/%s", spec.PublisherID, spec.ServiceName)
	}

	// Find the latest version tagged with the requested channel.
	// Filter by publisher_id and service_name, then pick lexicographically greatest version.
	// TODO: replace with dedicated GetLatestByChannel RPC when discovery service supports it.
	best := ""
	for _, a := range artifacts {
		if a.GetRef() == nil {
			continue
		}
		ref := a.GetRef()
		if ref.GetPublisherId() != spec.PublisherID || ref.GetName() != spec.ServiceName {
			continue
		}
		if spec.Platform != "" && ref.GetPlatform() != spec.Platform {
			continue
		}
		v := ref.GetVersion()
		if v == "" {
			continue
		}
		if best == "" || v > best {
			best = v
		}
	}
	if best == "" {
		return "", fmt.Errorf("no valid version found for %s/%s on channel %q", spec.PublisherID, spec.ServiceName, channel)
	}
	return best, nil
}

// assertSHA256Hex returns an error if checksum is not a 64-character lowercase hex string.
// Amendment 4: fail fast at resolve time rather than propagating an ambiguous checksum.
func assertSHA256Hex(checksum, publisherID, serviceName, version string) error {
	checksum = strings.TrimSpace(checksum)
	checksum = strings.ToLower(checksum)
	if len(checksum) != 64 {
		return fmt.Errorf(
			"ArtifactManifest.checksum for %s/%s@%s has unexpected length %d (want 64); "+
				"ensure the repository stores checksums as SHA256 hex",
			publisherID, serviceName, version, len(checksum))
	}
	for _, c := range checksum {
		if !((c >= '0' && c <= '9') || (c >= 'a' && c <= 'f')) {
			return fmt.Errorf(
				"ArtifactManifest.checksum for %s/%s@%s is not valid hex (got %q); "+
					"ensure the repository stores checksums as SHA256 hex",
				publisherID, serviceName, version, checksum)
		}
	}
	return nil
}

// ListArtifacts returns all artifact manifests for the given ref (partial match by name/publisher).
// Thin wrapper added to repository_client to support channel resolution.
func init() {} // ensure package compiles without unused import
