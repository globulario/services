package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/security"
	"github.com/globulario/services/golang/versionutil"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
)

// ReleaseResolver resolves a ServiceReleaseSpec version policy (exact pin or channel)
// to an exact version string, its SHA256 artifact digest, and the build number.
// It contacts the repository service to confirm the artifact exists and retrieve its manifest.
type ReleaseResolver struct {
	RepositoryAddr string // gRPC endpoint for repository service, e.g. "localhost:10101"
	InstallPolicy  *cluster_controllerpb.InstallPolicySpec // optional consumer install policy
	ArtifactKind   repositorypb.ArtifactKind // default SERVICE; set to INFRASTRUCTURE for infra releases
}

// ResolvedArtifact holds the full identity of a resolved artifact.
type ResolvedArtifact struct {
	Version     string
	Digest      string // SHA256 lowercase hex
	BuildNumber int64
}

// Resolve returns the full artifact identity for a ServiceReleaseSpec.
//
// Resolution rules:
//   - If spec.Version is non-empty: calls GetArtifactManifest to confirm existence and get digest.
//   - If spec.Version is empty: uses getLatestPublished to pick the latest PUBLISHED artifact
//     (highest semver, then highest build_number), then confirms via GetArtifactManifest.
//
// Only PUBLISHED artifacts are considered for latest resolution.
// Amendment 4: asserts that manifest.Checksum is a 64-char lowercase hex string (SHA256).
func (r *ReleaseResolver) Resolve(ctx context.Context, spec *cluster_controllerpb.ServiceReleaseSpec) (*ResolvedArtifact, error) {
	// Load install policy from governed storage if not injected.
	if r.InstallPolicy == nil {
		r.InstallPolicy = LoadInstallPolicy()
	}

	if spec == nil {
		return nil, fmt.Errorf("spec is nil")
	}
	if strings.TrimSpace(spec.PublisherID) == "" {
		return nil, fmt.Errorf("spec.publisher_id is required")
	}
	if strings.TrimSpace(spec.ServiceName) == "" {
		return nil, fmt.Errorf("spec.service_name is required")
	}

	addr := r.RepositoryAddr
	if addr == "" {
		addr = "localhost"
	}

	// Direct gRPC connection to the repository service, bypassing Envoy.
	// Envoy strips custom gRPC metadata (token, cluster_id) which breaks auth.
	repoClient, cc, err := r.dialRepositoryDirect(ctx, addr)
	if err != nil {
		return nil, fmt.Errorf("connect to repository %s: %w", addr, err)
	}
	defer cc.Close()

	authCtx := r.buildAuthContext(ctx)

	version := strings.TrimSpace(spec.Version)
	buildNumber := spec.BuildNumber

	if version == "" {
		// Channel field is deprecated and functionally ignored.
		if ch := strings.TrimSpace(spec.Channel); ch != "" {
			slog.Warn("spec.channel is deprecated and functionally ignored; resolution picks the latest published version",
				"channel", ch, "service", spec.ServiceName)
		}
		// Resolve latest PUBLISHED artifact: highest semver, then highest build_number.
		resolved, err := r.getLatestPublished(authCtx, repoClient, spec, "")
		if err != nil {
			return nil, fmt.Errorf("resolve latest version for %s/%s: %w", spec.PublisherID, spec.ServiceName, err)
		}
		version = resolved.version
		buildNumber = resolved.buildNumber
	} else if buildNumber == 0 {
		slog.Info("release-resolver: resolving latest build for pinned version",
			"service", spec.ServiceName, "version", version)
		// Version is pinned but build_number is unspecified — resolve to the
		// highest published build_number for that version. Otherwise
		// GetArtifactManifest would treat 0 as a literal lookup key and
		// return the oldest (build-0) artifact, preventing rebuild rollouts.
		// Normalize the version for comparison against artifact metadata.
		targetVersion := version
		if cv, err := versionutil.Canonical(targetVersion); err == nil {
			targetVersion = cv
		}
		resolved, err := r.getLatestPublished(authCtx, repoClient, spec, targetVersion)
		if err != nil {
			return nil, fmt.Errorf("resolve latest build for %s/%s@%s: %w", spec.PublisherID, spec.ServiceName, version, err)
		}
		buildNumber = resolved.buildNumber
		slog.Info("release-resolver: picked latest build",
			"service", spec.ServiceName, "version", version, "build_number", buildNumber)
	}

	// Normalize version to canonical semver.
	if cv, err := versionutil.Canonical(version); err == nil {
		version = cv
	}

	// Default platform to linux_amd64 when unspecified — artifacts are always
	// published with a platform, so an empty platform produces a key mismatch.
	platform := strings.TrimSpace(spec.Platform)
	if platform == "" {
		platform = "linux_amd64"
	}

	// Fetch manifest to confirm existence and retrieve digest.
	kind := r.ArtifactKind
	if kind == repositorypb.ArtifactKind_ARTIFACT_KIND_UNSPECIFIED {
		kind = repositorypb.ArtifactKind_SERVICE
	}
	ref := &repositorypb.ArtifactRef{
		PublisherId: spec.PublisherID,
		Name:        spec.ServiceName,
		Version:     version,
		Platform:    platform,
		Kind:        kind,
	}
	manifestResp, err := repoClient.GetArtifactManifest(authCtx, &repositorypb.GetArtifactManifestRequest{
		Ref:         ref,
		BuildNumber: buildNumber,
	})
	if err != nil {
		return nil, fmt.Errorf("get artifact manifest for %s/%s@%s build %d: %w",
			spec.PublisherID, spec.ServiceName, version, buildNumber, err)
	}
	manifest := manifestResp.GetManifest()
	if manifest == nil {
		return nil, fmt.Errorf("no manifest returned for %s/%s@%s build %d",
			spec.PublisherID, spec.ServiceName, version, buildNumber)
	}

	// Amendment 4: assert checksum is SHA256 hex (64 hex chars).
	checksum := normalizeSHA256(manifest.GetChecksum())
	if err := assertSHA256Hex(checksum, spec.PublisherID, spec.ServiceName, version); err != nil {
		return nil, err
	}

	return &ResolvedArtifact{
		Version:     version,
		Digest:      checksum,
		BuildNumber: manifest.GetBuildNumber(),
	}, nil
}

// artifactCandidate holds version + build_number for latest resolution.
type artifactCandidate struct {
	version     string
	buildNumber int64
}

// getLatestPublished resolves the latest PUBLISHED artifact for a service.
// Filters by publish_state == PUBLISHED, then picks highest semver,
// then highest build_number within that version.
//
// When versionFilter is non-empty, only candidates whose version matches
// (canonically) are considered — used to pick the latest build_number for a
// pinned version. When empty, all versions compete.
func (r *ReleaseResolver) getLatestPublished(ctx context.Context, client repositorypb.PackageRepositoryClient, spec *cluster_controllerpb.ServiceReleaseSpec, versionFilter string) (*artifactCandidate, error) {
	resp, err := client.ListArtifacts(ctx, &repositorypb.ListArtifactsRequest{})
	if err != nil {
		return nil, fmt.Errorf("list artifacts: %w", err)
	}
	artifacts := resp.GetArtifacts()
	if len(artifacts) == 0 {
		return nil, fmt.Errorf("no artifacts found for %s/%s", spec.PublisherID, spec.ServiceName)
	}

	// Collect candidates: only PUBLISHED, matching publisher/service/platform.
	// Apply install policy filtering if configured.
	policy := r.InstallPolicy
	verifiedCache := make(map[string]bool)
	var candidates []artifactCandidate
	for _, a := range artifacts {
		if a.GetRef() == nil {
			continue
		}
		// Only consider PUBLISHED artifacts.
		// PUBLISH_STATE_UNSPECIFIED is treated as PUBLISHED for legacy artifacts
		// that predate the publish state machine.
		ps := a.GetPublishState()

		// Always reject YANKED/QUARANTINED/REVOKED regardless of policy.
		if repositorypb.IsDownloadBlocked(ps) {
			continue
		}

		if ps != repositorypb.PublishState_PUBLISHED && ps != repositorypb.PublishState_PUBLISH_STATE_UNSPECIFIED {
			// DEPRECATED: skip if policy says so.
			if ps == repositorypb.PublishState_DEPRECATED && policy != nil && policy.BlockDeprecated {
				continue
			}
			if ps != repositorypb.PublishState_DEPRECATED {
				continue
			}
		}

		ref := a.GetRef()

		// Apply install policy namespace filtering.
		if policy != nil {
			pubID := ref.GetPublisherId()
			// Check blocked namespaces.
			if containsString(policy.BlockedNamespaces, pubID) {
				slog.Debug("artifact skipped (blocked namespace)", "publisher", pubID, "name", ref.GetName())
				continue
			}
			// Check allowed namespaces (if configured, only those are accepted).
			if len(policy.AllowedNamespaces) > 0 && !containsString(policy.AllowedNamespaces, pubID) {
				slog.Debug("artifact skipped (not in allowed namespaces)", "publisher", pubID, "name", ref.GetName())
				continue
			}
			// Check verified publishers only.
			if policy.VerifiedPublishersOnly {
				if !repositorypb.IsOfficialNamespace(pubID) {
					if verified, ok := verifiedCache[pubID]; ok {
						if !verified {
							continue
						}
					} else {
						verified := isVerifiedPublisher(ctx, client, pubID)
						verifiedCache[pubID] = verified
						if !verified {
							slog.Debug("artifact skipped (unverified publisher)", "publisher", pubID, "name", ref.GetName())
							continue
						}
					}
				}
			}
		}

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
		if versionFilter != "" {
			// Compare canonically so "0.0.1" matches "v0.0.1".
			cv := v
			if norm, err := versionutil.Canonical(cv); err == nil {
				cv = norm
			}
			if cv != versionFilter {
				continue
			}
		}
		candidates = append(candidates, artifactCandidate{
			version:     v,
			buildNumber: a.GetBuildNumber(),
		})
	}
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no published artifact found for %s/%s", spec.PublisherID, spec.ServiceName)
	}

	// Find highest semver, then highest build_number.
	best := candidates[0]
	for _, c := range candidates[1:] {
		cmp, err := versionutil.Compare(c.version, best.version)
		if err != nil {
			// Fallback: lexicographic.
			if c.version > best.version {
				best = c
			}
			continue
		}
		if cmp > 0 {
			best = c
		} else if cmp == 0 && c.buildNumber > best.buildNumber {
			best = c
		}
	}

	return &best, nil
}

// containsString returns true if the slice contains the target (case-insensitive).
func containsString(slice []string, target string) bool {
	for _, s := range slice {
		if strings.EqualFold(s, target) {
			return true
		}
	}
	return false
}

// LoadInstallPolicy reads the install policy from the governed file path.
// Returns nil if no policy file exists.
func LoadInstallPolicy() *cluster_controllerpb.InstallPolicySpec {
	const policyPath = "/var/lib/globular/config/install-policy.json"
	data, err := os.ReadFile(policyPath)
	if err != nil {
		return nil
	}
	policy := &cluster_controllerpb.InstallPolicySpec{}
	if err := json.Unmarshal(data, policy); err != nil {
		slog.Warn("corrupt install policy file", "path", policyPath, "err", err)
		return nil
	}
	return policy
}

// isVerifiedPublisher checks if a publisher namespace has been claimed by a real owner.
func isVerifiedPublisher(ctx context.Context, client repositorypb.PackageRepositoryClient, publisherID string) bool {
	resp, err := client.GetNamespace(ctx, &repositorypb.GetNamespaceRequest{NamespaceId: publisherID})
	if err != nil || resp == nil || resp.GetNamespace() == nil {
		return false
	}
	owners := resp.GetNamespace().GetOwners()
	if len(owners) == 0 {
		return false
	}
	// Check that at least one owner is not "sa" (migration placeholder).
	for _, o := range owners {
		if o != "sa" {
			return true
		}
	}
	return false
}

// normalizeSHA256 strips common prefixes (e.g. "sha256:") and whitespace from
// a checksum string, returning the bare lowercase hex digest.
func normalizeSHA256(raw string) string {
	s := strings.TrimSpace(raw)
	s = strings.ToLower(s)
	// Strip "sha256:" or "SHA256:" prefix (common in OCI/Docker manifests).
	s = strings.TrimPrefix(s, "sha256:")
	return strings.TrimSpace(s)
}

// assertSHA256Hex returns an error if checksum is not a 64-character lowercase hex string.
// Amendment 4: fail fast at resolve time rather than propagating an ambiguous checksum.
func assertSHA256Hex(checksum, publisherID, serviceName, version string) error {
	checksum = normalizeSHA256(checksum)
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

// dialRepositoryDirect creates a direct gRPC connection to the repository service,
// bypassing Envoy which strips auth metadata.
func (r *ReleaseResolver) dialRepositoryDirect(ctx context.Context, addr string) (repositorypb.PackageRepositoryClient, *grpc.ClientConn, error) {
	// Resolve the repository port from the service config in etcd.
	repoCfg, err := config.GetServiceConfigurationsByName("repository.PackageRepository")
	if err == nil && repoCfg != nil {
		if port, ok := repoCfg["Port"]; ok {
			cfgAddr := fmt.Sprintf("%v", repoCfg["Address"])
			if cfgAddr == "" || cfgAddr == "localhost" || cfgAddr == "127.0.0.1" {
				cfgAddr = "127.0.0.1"
			}
			// Strip existing port if present
			if idx := strings.Index(cfgAddr, ":"); idx >= 0 {
				cfgAddr = cfgAddr[:idx]
			}
			addr = fmt.Sprintf("%s:%v", cfgAddr, port)
		}
	}

	// Fallback: use localhost:10003 if we can't resolve
	if !strings.Contains(addr, ":") {
		addr = addr + ":10003"
	}

	caPath := config.GetLocalCACertificate()
	if caPath == "" {
		return nil, nil, fmt.Errorf("no CA certificate found")
	}
	caPEM, err := os.ReadFile(caPath)
	if err != nil {
		return nil, nil, fmt.Errorf("read CA: %w", err)
	}
	pool := x509.NewCertPool()
	pool.AppendCertsFromPEM(caPEM)

	dt := config.ResolveDialTarget(addr)
	tlsCfg := &tls.Config{
		ServerName: dt.ServerName,
		RootCAs:    pool,
	}

	dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	cc, err := grpc.DialContext(dialCtx, dt.Address,
		grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("dial repository %s: %w", dt.Address, err)
	}

	return repositorypb.NewPackageRepositoryClient(cc), cc, nil
}

// buildAuthContext creates a gRPC context with authentication metadata
// for direct repository calls.
func (r *ReleaseResolver) buildAuthContext(ctx context.Context) context.Context {
	mac, err := config.GetMacAddress()
	if err != nil {
		return ctx
	}
	token, err := security.GetLocalToken(mac)
	if err != nil {
		return ctx
	}

	clusterID, _ := security.GetLocalClusterID()

	md := metadata.New(map[string]string{
		"token":      token,
		"mac":        mac,
		"cluster_id": clusterID,
	})
	return metadata.NewOutgoingContext(ctx, md)
}
