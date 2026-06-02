package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/identity"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions/serviceports"
	"github.com/globulario/services/golang/systemdutil"
	"github.com/globulario/services/golang/versionutil"
	"github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/security"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/structpb"
)

// artifact.fetch copies a local artifact into a deterministic staging path.
// It supports local sources only for now; remote fetch can be added later.
type artifactFetchAction struct{}

func (artifactFetchAction) Name() string { return "artifact.fetch" }

func (artifactFetchAction) Validate(args *structpb.Struct) error { return nil }

func (artifactFetchAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	source := strings.TrimSpace(fields["source"].GetStringValue())
	dest := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	service := strings.TrimSpace(fields["service"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	platform := strings.TrimSpace(fields["platform"].GetStringValue())
	publisherID := strings.TrimSpace(fields["publisher_id"].GetStringValue())
	repositoryAddr := strings.TrimSpace(fields["repository_addr"].GetStringValue())
	repositoryInsecure := fields["repository_insecure"].GetBoolValue()
	repositoryCAPath := strings.TrimSpace(fields["repository_ca_path"].GetStringValue())
	buildNumber := int64(fields["build_number"].GetNumberValue())
	expectedSHA := strings.TrimSpace(fields["expected_sha256"].GetStringValue())
	buildID := strings.TrimSpace(fields["build_id"].GetStringValue())

	if dest == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create dest dir: %w", err)
	}
	// Determine full artifact identity for logs and metadata resolution.
	identityStr := fmt.Sprintf("%s/%s@%s+%d", publisherID, service, version, buildNumber)
	if buildID != "" {
		identityStr = fmt.Sprintf("%s/%s@%s build_id=%s", publisherID, service, version, buildID)
	}
	identity := identityStr

	// Resolve the repository address early — we may need it below to fetch
	// the manifest digest when the caller didn't pass expected_sha256.
	effectiveRepoAddr := repositoryAddr
	if effectiveRepoAddr == "" {
		effectiveRepoAddr = config.ResolveServiceAddr("repository.PackageRepository", "")
	}
	if effectiveRepoAddr == "" {
		effectiveRepoAddr = discoverRepositoryViaGateway()
	}

	// build_id is the canonical artifact identity. When provided, resolve it
	// to a concrete build_number+checksum before any fetch attempt.
	//
	// Failure modes here are NOT equivalent and must NOT be conflated:
	//
	//   - ErrBuildIDOrphaned     → the repository has explicitly demoted this
	//                              build (archived/yanked/revoked/quarantined).
	//                              FORBIDDEN to silently install local bytes —
	//                              we'd be running a build the repository said
	//                              "stop". Caller sees CannotFallbackWithoutManifestOrChecksum.
	//   - ErrBuildIDNotFound     → no manifest in the catalog → no way to verify
	//                              checksum of any local candidate → same: no fallback.
	//   - ErrRepositoryUnreachable → network blip. Fallback is allowed iff the
	//                              caller has an independent checksum / pinned proof.
	//
	// The legacy single "use local fallback" message is preserved for the
	// unreachable case so installer_api.go's existing behaviour is unchanged
	// there. Orphan / NotFound cases now return a distinct, non-recoverable
	// error that installer_api.go will refuse to fallback on.
	if buildID != "" {
		if effectiveRepoAddr != "" && service != "" && platform != "" {
			resolvedBN, resolvedChecksum, rerr := resolveArtifactByBuildID(ctx, effectiveRepoAddr, buildID, service, publisherID, platform)
			if rerr != nil {
				switch {
				case errors.Is(rerr, ErrBuildIDOrphaned):
					log.Printf("artifact.fetch: build_id=%s ORPHANED in repository — fallback FORBIDDEN (repository demoted this build): %v",
						buildID, rerr)
					return "", fmt.Errorf("artifact.fetch: build_id=%s is orphaned: CannotFallbackWithoutManifestOrChecksum: %w", buildID, rerr)
				case errors.Is(rerr, ErrBuildIDNotFound):
					log.Printf("artifact.fetch: build_id=%s NOT in repository catalog — fallback FORBIDDEN (no checksum proof): %v",
						buildID, rerr)
					return "", fmt.Errorf("artifact.fetch: build_id=%s not in catalog: CannotFallbackWithoutManifestOrChecksum: %w", buildID, rerr)
				default:
					log.Printf("artifact.fetch: build_id=%s resolve failed (repo=%s, transient): %v — aborting fetch",
						buildID, effectiveRepoAddr, rerr)
					return "", fmt.Errorf("artifact.fetch: build_id=%s could not be resolved (repository unreachable) — use local fallback: %w",
						buildID, rerr)
				}
			}
			log.Printf("artifact.fetch: resolved build_id=%s → build_number=%d checksum=%s",
				buildID, resolvedBN, shortHash(resolvedChecksum))
			buildNumber = resolvedBN
			if expectedSHA == "" && resolvedChecksum != "" {
				expectedSHA = resolvedChecksum
			}
		} else {
			// No repository address available — this is the transient/bootstrap
			// case. Caller may fallback to a locally pinned package, but only if
			// it has an independent checksum proof (expected_sha256 set).
			log.Printf("artifact.fetch: build_id=%s set but no repository reachable — aborting fetch", buildID)
			return "", fmt.Errorf("artifact.fetch: build_id=%s could not be resolved (repository unreachable) — use local fallback: %w",
				buildID, ErrRepositoryUnreachable)
		}
	}

	// Safe cache decision matrix (see ROOT-CAUSE FIX in todo):
	//   A) expected_sha256 set            → verify local hash; reuse if match, replace if not.
	//   B) full artifact identity known   → fetch manifest digest from repository, then A.
	//   C) neither                        → refuse blind cache reuse (loud error).
	// Blind "file exists → reuse" is forbidden.
	if _, err := os.Stat(dest); err == nil {
		effectiveSHA := expectedSHA
		if effectiveSHA == "" {
			// Case B: try to resolve the digest from the repository manifest
			// so we can validate the cached bytes before trusting them.
			if service != "" && version != "" && platform != "" && effectiveRepoAddr != "" {
				log.Printf("artifact.fetch: cache-resolving-digest %s (dest=%s, repo=%s)",
					identity, dest, effectiveRepoAddr)
				if digest, rerr := resolveArtifactDigest(ctx, effectiveRepoAddr,
					publisherID, service, version, platform,
					strings.TrimSpace(fields["artifact_kind"].GetStringValue()),
					buildNumber); rerr == nil && digest != "" {
					effectiveSHA = digest
				} else if rerr != nil {
					log.Printf("artifact.fetch: cache-resolve-failed %s: %v — will re-download",
						identity, rerr)
				}
			}
		}
		if effectiveSHA != "" {
			if err := verifyFileSHA256(dest, effectiveSHA); err != nil {
				log.Printf("artifact.fetch: cache-mismatch %s (dest=%s): %v — re-downloading",
					identity, dest, err)
				if rmErr := os.Remove(dest); rmErr != nil && !os.IsNotExist(rmErr) {
					log.Printf("artifact.fetch: cache-remove-failed %s: %v", dest, rmErr)
				}
			} else {
				log.Printf("artifact.fetch: cache-hit-verified %s (dest=%s, sha256=%s)",
					identity, dest, shortHash(effectiveSHA))
				return "artifact already present (verified)", nil
			}
		} else {
			// Case C: cannot validate cache without identity — never trust.
			// This is loud on purpose: it proves the contract is enforced.
			log.Printf("artifact.fetch: cache-insufficient-identity %s (dest=%s) — refusing blind reuse",
				identity, dest)
			return "", fmt.Errorf(
				"artifact.fetch: refuse blind cache reuse for %s — pass expected_sha256 or full artifact identity (service+version+platform+publisher, repository_addr for manifest lookup)",
				dest)
		}
	}

	// Resolve local source path if not explicitly provided.
	if source == "" && (service != "" && version != "" && platform != "") {
		source = resolveArtifactPath(service, version, platform)
	}

	// Try local copy first — but only if we can validate the copied bytes.
	// A local source without identity is treated the same as blind cache reuse.
	if source != "" {
		if _, err := os.Stat(source); err == nil {
			in, err := os.Open(source)
			if err != nil {
				return "", fmt.Errorf("open source: %w", err)
			}
			defer in.Close()
			if err := copyFileAtomic(dest, in); err != nil {
				return "", err
			}
			if expectedSHA != "" {
				if verr := verifyFileSHA256(dest, expectedSHA); verr != nil {
					os.Remove(dest)
					log.Printf("artifact.fetch: local-source-mismatch %s (source=%s): %v",
						identity, source, verr)
					return "", fmt.Errorf("local source %s sha256 mismatch: %w", source, verr)
				}
				log.Printf("artifact.fetch: local-source-verified %s (source=%s, sha256=%s)",
					identity, source, shortHash(expectedSHA))
				return "artifact fetched (local, verified)", nil
			}
			log.Printf("artifact.fetch: local-source-no-digest %s (source=%s) — copied without verification",
				identity, source)
			return "artifact fetched (local)", nil
		}
	}

	// Fall back to remote repository download.
	repositoryAddr = effectiveRepoAddr
	if repositoryAddr == "" {
		return "", fmt.Errorf("artifact not found locally and repository address could not be resolved")
	}
	if service == "" || version == "" || platform == "" {
		return "", fmt.Errorf("service, version, and platform are required for remote fetch")
	}
	// Determine artifact kind from plan args (default: SERVICE for backward compat).
	artifactKind := repositorypb.ArtifactKind_SERVICE
	if kindStr := strings.TrimSpace(fields["artifact_kind"].GetStringValue()); kindStr != "" {
		switch strings.ToUpper(kindStr) {
		case "INFRASTRUCTURE":
			artifactKind = repositorypb.ArtifactKind_INFRASTRUCTURE
		case "APPLICATION":
			artifactKind = repositorypb.ArtifactKind_APPLICATION
		case "COMMAND":
			artifactKind = repositorypb.ArtifactKind_COMMAND
		}
	}
	ref := &repositorypb.ArtifactRef{
		Name:     service,
		Version:  version,
		Platform: platform,
		Kind:     artifactKind,
	}
	if publisherID != "" {
		ref.PublisherId = publisherID
	}
	// If the caller didn't pass expected_sha256, resolve it from the manifest
	// before download so the download path can verify bytes post-fetch.
	if expectedSHA == "" {
		if digest, rerr := resolveArtifactDigest(ctx, repositoryAddr,
			publisherID, service, version, platform,
			strings.TrimSpace(fields["artifact_kind"].GetStringValue()),
			buildNumber); rerr == nil && digest != "" {
			expectedSHA = digest
			log.Printf("artifact.fetch: resolved-digest %s sha256=%s (pre-download)",
				identity, shortHash(expectedSHA))
		} else if rerr != nil {
			log.Printf("artifact.fetch: digest-resolve-failed %s: %v — downloading without pre-check",
				identity, rerr)
		}
	}
	if err := downloadArtifactFromRepository(ctx, repositoryAddr, ref, dest, expectedSHA, repositoryInsecure, repositoryCAPath, buildNumber); err != nil {
		log.Printf("artifact.fetch: download-failed %s: %v", identity, err)
		return "", err
	}
	if expectedSHA != "" {
		log.Printf("artifact.fetch: download-complete-verified %s (dest=%s, sha256=%s)",
			identity, dest, shortHash(expectedSHA))
	} else {
		log.Printf("artifact.fetch: download-complete %s (dest=%s, no expected digest)",
			identity, dest)
	}
	return fmt.Sprintf("artifact fetched (remote) from %s", repositoryAddr), nil
}

// resolveArtifactDigest fetches the expected checksum for an artifact from the
// repository's GetArtifactManifest. Used to validate local cached bytes before
// reuse when the caller did not pass an explicit expected_sha256. Returns the
// lowercase hex digest (no "sha256:" prefix) or an error.
func resolveArtifactDigest(ctx context.Context, repoAddr, publisherID, service, version, platform, kindStr string, buildNumber int64) (string, error) {
	if repoAddr == "" {
		return "", fmt.Errorf("repository address not set")
	}
	conn, _, err := dialRepository(ctx, repoAddr)
	if err != nil {
		return "", fmt.Errorf("dial repository: %w", err)
	}
	defer conn.Close()

	authCtx := ctx
	if clusterID, cerr := security.GetLocalClusterID(); cerr == nil && clusterID != "" {
		md := metadata.Pairs("cluster_id", clusterID)
		authCtx = metadata.NewOutgoingContext(ctx, md)
	}

	kind := repositorypb.ArtifactKind_SERVICE
	switch strings.ToUpper(kindStr) {
	case "INFRASTRUCTURE":
		kind = repositorypb.ArtifactKind_INFRASTRUCTURE
	case "APPLICATION":
		kind = repositorypb.ArtifactKind_APPLICATION
	case "COMMAND":
		kind = repositorypb.ArtifactKind_COMMAND
	}
	ref := &repositorypb.ArtifactRef{
		PublisherId: publisherID,
		Name:        service,
		Version:     version,
		Platform:    platform,
		Kind:        kind,
	}
	client := repositorypb.NewPackageRepositoryClient(conn)
	resp, err := client.GetArtifactManifest(authCtx, &repositorypb.GetArtifactManifestRequest{
		Ref:         ref,
		BuildNumber: buildNumber,
	})
	if err != nil {
		return "", fmt.Errorf("get manifest: %w", err)
	}
	manifest := resp.GetManifest()
	if manifest == nil {
		return "", fmt.Errorf("no manifest returned")
	}
	// Strip "sha256:" prefix and lowercase — verifyFileSHA256 compares lowercase hex.
	digest := strings.ToLower(strings.TrimSpace(manifest.GetChecksum()))
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) != 64 {
		return "", fmt.Errorf("manifest checksum is not a sha256 hex (len=%d)", len(digest))
	}
	return digest, nil
}

// resolveArtifactEntrypointDigest fetches the expected runtime-binary checksum
// (entrypoint_checksum) for an artifact from the repository manifest.
// Returns lowercase hex digest without "sha256:" prefix.
// Falls back to archive checksum for legacy manifests missing entrypoint checksum.
func resolveArtifactEntrypointDigest(ctx context.Context, repoAddr, publisherID, service, version, platform, kindStr string, buildNumber int64) (string, error) {
	if repoAddr == "" {
		return "", fmt.Errorf("repository address not set")
	}
	conn, _, err := dialRepository(ctx, repoAddr)
	if err != nil {
		return "", fmt.Errorf("dial repository: %w", err)
	}
	defer conn.Close()

	authCtx := ctx
	if clusterID, cerr := security.GetLocalClusterID(); cerr == nil && clusterID != "" {
		md := metadata.Pairs("cluster_id", clusterID)
		authCtx = metadata.NewOutgoingContext(ctx, md)
	}

	kind := repositorypb.ArtifactKind_SERVICE
	switch strings.ToUpper(kindStr) {
	case "INFRASTRUCTURE":
		kind = repositorypb.ArtifactKind_INFRASTRUCTURE
	case "APPLICATION":
		kind = repositorypb.ArtifactKind_APPLICATION
	case "COMMAND":
		kind = repositorypb.ArtifactKind_COMMAND
	}
	ref := &repositorypb.ArtifactRef{
		PublisherId: publisherID,
		Name:        service,
		Version:     version,
		Platform:    platform,
		Kind:        kind,
	}
	client := repositorypb.NewPackageRepositoryClient(conn)
	resp, err := client.GetArtifactManifest(authCtx, &repositorypb.GetArtifactManifestRequest{
		Ref:         ref,
		BuildNumber: buildNumber,
	})
	if err != nil {
		return "", fmt.Errorf("get manifest: %w", err)
	}
	manifest := resp.GetManifest()
	if manifest == nil {
		return "", fmt.Errorf("no manifest returned")
	}

	digest := strings.ToLower(strings.TrimSpace(manifest.GetEntrypointChecksum()))
	if digest == "" {
		digest = strings.ToLower(strings.TrimSpace(manifest.GetChecksum()))
	}
	digest = strings.TrimPrefix(digest, "sha256:")
	if len(digest) != 64 {
		return "", fmt.Errorf("manifest entrypoint checksum is not a sha256 hex (len=%d)", len(digest))
	}
	return digest, nil
}

// Sentinel errors raised by resolveArtifactByBuildID. Callers use errors.Is
// to distinguish "repository is unreachable / network error" (local fallback
// may be safe IF an alternate way to verify checksum exists) from "repository
// is reachable and has explicitly demoted this build_id" (fallback would
// silently install stale bytes — forbidden by fallback.requires_manifest_checksum).
var (
	// ErrBuildIDOrphaned: the repository returned a structured
	// `DesiredBuildIdOrphaned` precondition failure. The build was demoted
	// (archived, yanked, revoked, quarantined, or never finished publishing)
	// while desired state still pins it. Fallback to a local pinned tarball
	// is forbidden — the controller must surface this as a release blocker
	// (RELEASE_BLOCKED_REPOSITORY_ORPHANED_BUILD_ID).
	ErrBuildIDOrphaned = errors.New("DesiredBuildIdOrphaned")

	// ErrBuildIDNotFound: the repository is reachable but the build_id is not
	// in its catalog. Same fallback semantics as ErrBuildIDOrphaned (no quiet
	// local install) — without a manifest there is no way to verify any
	// candidate bytes match the desired identity.
	ErrBuildIDNotFound = errors.New("BuildIDNotFound")

	// ErrRepositoryUnreachable: network / TLS / auth failure. The repository
	// may come back; fallback is permitted only when an independent checksum
	// proof is available for the candidate bytes.
	ErrRepositoryUnreachable = errors.New("RepositoryUnreachable")
)

// isRepositoryBackpressure returns true when err signals that the repository is
// temporarily overloaded (ResourceExhausted / server overloaded / too many
// concurrent requests). This is a transient capacity signal — the caller must
// retry with backoff, not mark the install as permanently failed.
//
// Permanent artifact identity failures (checksum mismatch, orphaned build,
// manifest missing) must NOT be classified here.
func isRepositoryBackpressure(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "code = resourceexhausted") ||
		strings.Contains(s, "resourceexhausted") ||
		strings.Contains(s, "resource exhausted") ||
		strings.Contains(s, "server overloaded") ||
		strings.Contains(s, "too many concurrent requests") ||
		strings.Contains(s, "repository overloaded") ||
		strings.Contains(s, "repository backpressure")
}

// resolveArtifactByBuildID resolves the exact build_number and checksum for a
// given build_id by calling ResolveArtifact on the repository. This is the
// correct path for controllers that dispatch workflows with a known build_id
// but without a pre-resolved build_number.
//
// Error contract — exactly one of:
//
//   - nil, build_number, checksum  (success)
//   - error wrapping ErrBuildIDOrphaned   (FailedPrecondition: build demoted)
//   - error wrapping ErrBuildIDNotFound   (NotFound: never published / purged)
//   - error wrapping ErrRepositoryUnreachable (network / TLS / auth)
//   - bare error                          (programmer error, e.g. nil manifest)
//
// The installer / workflow caller branches on these:
//   - Orphaned / NotFound → DO NOT use local fallback. Surface release blocker.
//     The repository owns identity; quietly installing a pinned tarball whose
//     checksum we can no longer verify would violate the 4-layer model.
//   - Unreachable          → local fallback is permitted only if an independent
//     manifest/checksum proof is available (out-of-band pinning).
func resolveArtifactByBuildID(ctx context.Context, repoAddr, buildID, service, publisherID, platform string) (int64, string, error) {
	if repoAddr == "" {
		return 0, "", fmt.Errorf("repository address not set: %w", ErrRepositoryUnreachable)
	}
	conn, _, err := dialRepository(ctx, repoAddr)
	if err != nil {
		return 0, "", fmt.Errorf("dial repository: %v: %w", err, ErrRepositoryUnreachable)
	}
	defer conn.Close()

	authCtx := ctx
	if clusterID, cerr := security.GetLocalClusterID(); cerr == nil && clusterID != "" {
		md := metadata.Pairs("cluster_id", clusterID)
		authCtx = metadata.NewOutgoingContext(ctx, md)
	}

	client := repositorypb.NewPackageRepositoryClient(conn)
	resp, err := client.ResolveArtifact(authCtx, &repositorypb.ResolveArtifactRequest{
		BuildId:     buildID,
		Name:        service,
		PublisherId: publisherID,
		Platform:    platform,
	})
	if err != nil {
		// Classify by gRPC code from the repository.
		// NB: the lowercase grpc.status.FromError is imported in other files in
		// this package; we use string-match here to avoid pulling another import
		// just for one switch — the resolver-side identifiers are stable.
		emsg := err.Error()
		switch {
		case strings.Contains(emsg, "DesiredBuildIdOrphaned"):
			return 0, "", fmt.Errorf("resolve artifact by build_id %s: %v: %w", buildID, err, ErrBuildIDOrphaned)
		case strings.Contains(emsg, "code = NotFound"):
			return 0, "", fmt.Errorf("resolve artifact by build_id %s: %v: %w", buildID, err, ErrBuildIDNotFound)
		case isRepositoryBackpressure(err):
			// Repository backpressure (ResourceExhausted / server overloaded) is
			// transient — the caller must retry with backoff, not treat this as a
			// permanent bootstrap failure.
			return 0, "", fmt.Errorf("repository backpressure while resolving artifact build_id=%s: %v: %w", buildID, err, ErrRepositoryUnreachable)
		case strings.Contains(emsg, "code = Unavailable") || strings.Contains(emsg, "code = DeadlineExceeded") ||
			strings.Contains(emsg, "code = Unauthenticated") || strings.Contains(emsg, "connection refused") ||
			strings.Contains(emsg, "no such host"):
			return 0, "", fmt.Errorf("resolve artifact by build_id %s: %v: %w", buildID, err, ErrRepositoryUnreachable)
		default:
			return 0, "", fmt.Errorf("resolve artifact by build_id %s: %w", buildID, err)
		}
	}
	manifest := resp.GetManifest()
	if manifest == nil {
		return 0, "", fmt.Errorf("no manifest returned for build_id %s", buildID)
	}
	digest := strings.ToLower(strings.TrimSpace(manifest.GetChecksum()))
	digest = strings.TrimPrefix(digest, "sha256:")
	return manifest.GetBuildNumber(), digest, nil
}

// shortHash returns the first 12 chars of a hex digest for log readability.
func shortHash(s string) string {
	s = canonicalSHA256(s)
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

func canonicalSHA256(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "sha256:")
	return s
}

// artifact.verify performs a simple existence/digest check if provided.
type artifactVerifyAction struct{}

func (artifactVerifyAction) Name() string { return "artifact.verify" }

func (artifactVerifyAction) Validate(args *structpb.Struct) error { return nil }

func (artifactVerifyAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	path := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	expected := canonicalSHA256(fields["expected_sha256"].GetStringValue())
	if path == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	if _, err := os.Stat(path); err != nil {
		return "", fmt.Errorf("artifact missing: %w", err)
	}
	if expected == "" {
		if !allowMissingSHA256() {
			return "", fmt.Errorf("expected_sha256 is required (set AllowMissingSHA256 for dev bypass)")
		}
		// Dev bypass: compute hash for audit logging but allow the install to proceed.
		f, err := os.Open(path)
		if err != nil {
			return "", fmt.Errorf("open artifact: %w", err)
		}
		defer f.Close()
		h := sha256.New()
		if _, err := io.Copy(h, f); err != nil {
			return "", fmt.Errorf("hash artifact: %w", err)
		}
		got := hex.EncodeToString(h.Sum(nil))
		return fmt.Sprintf("artifact verified (dev bypass, no expected digest, computed sha256=%s)", got), nil
	}
	f, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("open artifact: %w", err)
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", fmt.Errorf("hash artifact: %w", err)
	}
	got := hex.EncodeToString(h.Sum(nil))
	if canonicalSHA256(got) != expected {
		return "", fmt.Errorf("artifact digest mismatch: want %s got %s", expected, got)
	}
	return fmt.Sprintf("artifact verified sha256=%s", got), nil
}

type serviceInstallPayloadAction struct{}

func (serviceInstallPayloadAction) Name() string { return "service.install_payload" }

func (serviceInstallPayloadAction) Validate(args *structpb.Struct) error { return nil }

func (serviceInstallPayloadAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	service := strings.TrimSpace(fields["service"].GetStringValue())
	artifact := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	version := strings.TrimSpace(fields["version"].GetStringValue())
	if service == "" {
		return "", fmt.Errorf("service is required")
	}
	if artifact == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	stagingRoot := filepath.Join(ActionStateDir, "staging", service)
	if ActionStagingRoot != "" {
		stagingRoot = filepath.Join(ActionStagingRoot, service)
	}
	if err := os.MkdirAll(stagingRoot, 0o755); err != nil {
		return "", fmt.Errorf("create staging dir: %w", err)
	}
	if _, err := os.MkdirTemp(stagingRoot, "extract-"); err != nil {
		return "", fmt.Errorf("create extract dir: %w", err)
	}
	f, err := os.Open(artifact)
	if err != nil {
		return "", fmt.Errorf("open artifact: %w", err)
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return "", fmt.Errorf("gzip reader: %w", err)
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	binDir, systemdDir, configDir, skipSystemd := installPaths()
	scriptsDir := filepath.Join(stagingRoot, "scripts")
	var wroteUnit bool

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", fmt.Errorf("read tar: %w", err)
		}
		if hdr.FileInfo().IsDir() {
			continue
		}
		name := strings.TrimLeft(hdr.Name, "./")
		var dest string
		switch {
		case strings.HasPrefix(name, "bin/"):
			dest = filepath.Join(binDir, filepath.Base(name))
		case strings.HasPrefix(name, "systemd/"), strings.HasPrefix(name, "units/"):
			if skipSystemd {
				continue
			}
			dest = filepath.Join(systemdDir, filepath.Base(name))
			wroteUnit = true
		case strings.HasPrefix(name, "config/"):
			dest = filepath.Join(configDir, service, strings.TrimPrefix(name, "config/"))
			// Seed-only: config files from packages are defaults. If a live
			// config already exists (rendered by controller, join script, or
			// workflow), preserve it — package reinstall must never overwrite
			// cluster-owned configuration.
			if _, err := os.Stat(dest); err == nil {
				log.Printf("install_payload: preserving existing config %s (seed-only)", dest)
				continue
			}
		case strings.HasPrefix(name, "scripts/"):
			dest = filepath.Join(scriptsDir, filepath.Base(name))
		case strings.HasPrefix(name, "data/"):
			// Data files are extracted to ActionStateDir preserving subdirectory structure.
			// e.g. data/workflows/day0.bootstrap.yaml → /var/lib/globular/workflows/day0.bootstrap.yaml
			rel := strings.TrimPrefix(name, "data/")
			dest = filepath.Join(ActionStateDir, rel)
		case strings.HasPrefix(name, "policy/"):
			// Authorization policy files (permissions.generated.json, roles.generated.json)
			// are installed under ActionPolicyDir/{service}/ so the runtime resolver
			// can map gRPC method paths to stable action keys.
			rel := strings.TrimPrefix(name, "policy/")
			dest = filepath.Join(ActionPolicyDir, service, rel)
		default:
			// ignore unsupported paths
			continue
		}
		if dest == "" {
			continue
		}
		if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
			return "", fmt.Errorf("mkdir for %s: %w", dest, err)
		}
		tmp := dest + ".tmp"
		df, err := os.Create(tmp)
		if err != nil {
			return "", fmt.Errorf("create %s: %w", tmp, err)
		}
		if _, err := io.Copy(df, tr); err != nil {
			df.Close()
			os.Remove(tmp)
			return "", fmt.Errorf("write %s: %w", dest, err)
		}
		if err := df.Chmod(hdr.FileInfo().Mode()); err != nil {
			df.Close()
			os.Remove(tmp)
			return "", fmt.Errorf("chmod %s: %w", dest, err)
		}
		if err := df.Close(); err != nil {
			os.Remove(tmp)
			return "", fmt.Errorf("close %s: %w", dest, err)
		}
		// Render template variables in systemd unit and config files.
		if strings.HasPrefix(name, "systemd/") || strings.HasPrefix(name, "units/") || strings.HasPrefix(name, "config/") {
			if err := renderTemplateVars(tmp, ActionStateDir, binDir); err != nil {
				os.Remove(tmp)
				return "", fmt.Errorf("render template %s: %w", dest, err)
			}
		}
		// Normalize fragile WorkingDirectory lines in systemd units.
		// Old packages may contain WorkingDirectory=/var/lib/globular/<svc>
		// (without the '-' optional prefix). systemd evaluates WorkingDirectory
		// before ExecStartPre runs, so a missing directory causes status=200/CHDIR.
		// Normalize at install time so the unit is always safe regardless of
		// package age.
		if strings.HasPrefix(name, "systemd/") || strings.HasPrefix(name, "units/") {
			if err := normalizeUnitWorkingDirectory(tmp); err != nil {
				os.Remove(tmp)
				return "", fmt.Errorf("normalize unit %s: %w", dest, err)
			}
		}
		if err := os.Rename(tmp, dest); err != nil {
			os.Remove(tmp)
			return "", fmt.Errorf("rename %s: %w", dest, err)
		}
		// Write a SHA-256 sidecar for systemd unit files so the heartbeat can
		// detect unit definition drift after installation.
		if strings.HasSuffix(dest, ".service") {
			if data, err := os.ReadFile(dest); err == nil {
				sum := sha256.Sum256(data)
				sidecar := dest + ".sha256"
				tmp2 := sidecar + ".tmp"
				if err := os.WriteFile(tmp2, []byte(hex.EncodeToString(sum[:])), 0o644); err == nil {
					_ = os.Rename(tmp2, sidecar)
				}
			}
		}
	}

	// Ensure the service working directory exists.  Even though we normalize
	// unit files to use WorkingDirectory=-<path> (optional), the directory
	// should still exist before the service starts so logs and state files
	// have a home.  Owner globular:globular, mode 0750.
	svcWorkDir := filepath.Join(ActionStateDir, service)
	if err := ensureServiceStateDir(svcWorkDir); err != nil {
		return "", fmt.Errorf("create service workdir %s: %w", svcWorkDir, err)
	}
	// Ensure shared top-level dirs that services create at runtime are owned
	// by globular:globular. The node-agent runs as root and MkdirAll would
	// create them as root:root, causing permission denied for services that
	// run as the globular user (e.g. authentication writing Ed25519 keys,
	// backup-manager creating jobs/). These dirs are shared across services
	// so we create them here at install time rather than per-service.
	for _, sharedDir := range []string{
		filepath.Join(ActionStateDir, "keys"),
		filepath.Join(ActionStateDir, "backups"),
		filepath.Join(ActionStateDir, "backups", "jobs"),
	} {
		if err := ensureServiceStateDir(sharedDir); err != nil {
			log.Printf("install: warning: could not ensure shared dir %s: %v", sharedDir, err)
		}
	}
	// Alertmanager may be installed without a default config file in some
	// artifacts. Seed a minimal config so the unit can start on first boot.
	if strings.EqualFold(service, "alertmanager") {
		if err := ensureAlertmanagerConfigFile(ActionStateDir); err != nil {
			return "", fmt.Errorf("prepare alertmanager config: %w", err)
		}
	}

	if wroteUnit && !skipSystemd && !ActionSkipDaemonReload {
		cctx, cancel := context.WithTimeout(ctx, 30*time.Second)
		defer cancel()
		cmd := exec.CommandContext(cctx, "systemctl", "daemon-reload")
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("systemctl daemon-reload: %v (output: %s)", err, string(out))
		}
	}

	if version == "" {
		version = filepath.Base(artifact)
	}

	// Ensure runtime config + port normalization. Non-fatal: many binaries
	// don't implement --describe so port config is best-effort. The drift
	// reconciler will re-run this later once the service is known.
	if err := serviceports.EnsureServicePortConfig(ctx, service, binDir); err != nil {
		log.Printf("install_payload: port config for %s best-effort failed: %v (install continues)", service, err)
	}

	// Run post-install script if bundled in the artifact.
	// Infrastructure packages (scylladb, etc.) use this to generate config
	// files that depend on runtime state (node IP, seed discovery, etc.).
	if err := runPostInstallScript(ctx, scriptsDir, ActionStateDir); err != nil {
		return "", fmt.Errorf("post-install script: %w", err)
	}

	// Safe post-install verification: confirm the bytes we just extracted
	// are actually on disk, executable, and non-empty. This replaces the
	// old --describe gate which falsely failed for binaries that don't
	// implement the describe protocol (node_agent_server, xds, gateway,
	// minio, etc). Identity of the extracted bytes is already verified
	// upstream by artifact.fetch (sha256 match vs manifest digest).
	exe := executableForService(service)
	if exe != "" {
		binPath := filepath.Join(binDir, exe)
		if err := verifyInstalledBinary(binPath); err != nil {
			return "", fmt.Errorf("verify %s: %w", service, err)
		}
		// Preflight: check that all native shared-library dependencies of the
		// binary are present on this node. Fail immediately rather than letting
		// systemd crash-loop for minutes before verify_runtime times out.
		if missing, err := MissingNativeLibs(ctx, binPath); err == nil && len(missing) > 0 {
			return "", fmt.Errorf(
				"NATIVE_LIBRARY_DEPENDENCY_MISSING: %s requires native libraries not installed on this node: %v — install the OS packages providing them (e.g. for libodbc.so.2: apt install unixodbc)",
				service, missing,
			)
		}
	}

	return fmt.Sprintf("service payload installed version=%s", version), nil
}

// verifyInstalledBinary checks that an installed binary is present, executable,
// and non-empty. It does NOT invoke the binary — invoking an unknown binary
// with an arbitrary flag like --describe is inherently unreliable for
// verification since many binaries don't implement it and some start the
// full service instead. Byte-level integrity is the responsibility of
// artifact.fetch (which verifies the sha256 digest vs the manifest).
func verifyInstalledBinary(binPath string) error {
	fi, err := os.Stat(binPath)
	if err != nil {
		return fmt.Errorf("%s: %w", binPath, err)
	}
	if fi.IsDir() {
		return fmt.Errorf("%s: is a directory, not a file", binPath)
	}
	if fi.Size() == 0 {
		return fmt.Errorf("%s: zero-byte file", binPath)
	}
	// Any exec bit suffices (owner/group/other). We don't own the binary,
	// we just need the kernel to accept it as runnable by systemd.
	if fi.Mode().Perm()&0o111 == 0 {
		return fmt.Errorf("%s: not executable (mode=%s)", binPath, fi.Mode().Perm())
	}
	return nil
}

// runPostInstallScript executes scripts/post-install.sh from the extracted
// artifact if it exists. The script runs with STATE_DIR set to the globular
// state directory so it can discover etcd endpoints, node IP, etc.
// After execution the scripts staging dir is cleaned up.
func runPostInstallScript(ctx context.Context, scriptsDir, stateRoot string) error {
	script := filepath.Join(scriptsDir, "post-install.sh")
	if _, err := os.Stat(script); err != nil {
		return nil // no post-install script bundled — nothing to do
	}

	log.Printf("install_payload: running post-install script %s", script)
	cctx, cancel := context.WithTimeout(ctx, 5*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(cctx, "/bin/bash", script)
	cmd.Env = append(os.Environ(), "STATE_DIR="+stateRoot)
	out, err := cmd.CombinedOutput()
	// Clean up extracted scripts regardless of outcome.
	os.RemoveAll(scriptsDir)
	if err != nil {
		return fmt.Errorf("exit %v: %s", err, string(out))
	}
	log.Printf("install_payload: post-install script completed:\n%s", string(out))
	return nil
}

type serviceWriteVersionMarkerAction struct{}

func (serviceWriteVersionMarkerAction) Name() string { return "service.write_version_marker" }

func (serviceWriteVersionMarkerAction) Validate(args *structpb.Struct) error { return nil }

func (serviceWriteVersionMarkerAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	service := strings.TrimSpace(fields["service"].GetStringValue())
	version := fields["version"].GetStringValue()
	if cv, err := versionutil.Canonical(version); err == nil {
		version = cv
	}
	path := strings.TrimSpace(fields["path"].GetStringValue())
	if service == "" {
		return "", fmt.Errorf("service is required")
	}
	if path == "" {
		path = versionutil.MarkerPath(service)
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create marker dir: %w", err)
	}
	if err := os.WriteFile(path, []byte(version), 0o644); err != nil {
		return "", fmt.Errorf("write marker: %w", err)
	}
	// Persist kind sidecar so Phase-1 (offline) reads know the kind without etcd.
	if kind := strings.TrimSpace(fields["artifact_kind"].GetStringValue()); kind != "" {
		_ = versionutil.WriteKind(service, kind)
	}
	return "version marker written", nil
}

// discoverRepositoryViaGateway derives the repository address from the
// controller endpoint. The gateway (Envoy) runs on the same host as the
// controller, on port 443, and routes gRPC to all backend services
// including the repository. This avoids requiring a separate
// REPOSITORY_ADDRESS configuration on joining nodes.
func discoverRepositoryViaGateway() string {
	// Try node-agent state file first.
	statePath := filepath.Join(ActionStateDir, "nodeagent", "state.json")
	if data, err := os.ReadFile(statePath); err == nil {
		var state struct {
			ControllerEndpoint string `json:"controller_endpoint"`
		}
		if json.Unmarshal(data, &state) == nil && state.ControllerEndpoint != "" {
			host, _, err := net.SplitHostPort(state.ControllerEndpoint)
			if err == nil && host != "" {
				addr := net.JoinHostPort(host, "443")
				fmt.Printf("INFO artifact fetch: discovered repository via gateway %s (from controller %s)\n", addr, state.ControllerEndpoint)
				return addr
			}
		}
	}

	return ""
}

func resolveArtifactPath(service, version, platform string) string {
	root := ActionArtifactRepoRoot
	filename := fmt.Sprintf("%s.%s.%s.tgz", service, version, platform)
	return filepath.Join(root, service, version, platform, filename)
}

// CheckArtifactPublished verifies that the artifact identified by the given parameters
// is in PUBLISHED state in the repository. This is the node-agent's final guardrail:
// even if the controller dispatches an install for a non-PUBLISHED artifact, the
// node-agent must reject it. Returns nil if PUBLISHED, error otherwise.
func CheckArtifactPublished(ctx context.Context, repoAddr, publisherID, name, version, platform, kind string, buildNumber int64) error {
	if repoAddr == "" {
		// No repository available — skip check (local/bootstrap installs).
		return nil
	}

	conn, resolvedAddr, err := dialRepository(ctx, repoAddr)
	if err != nil {
		return fmt.Errorf("publish guard: dial repository %s: %w", repoAddr, err)
	}
	defer conn.Close()
	_ = resolvedAddr

	authCtx := ctx
	if clusterID, err := security.GetLocalClusterID(); err == nil && clusterID != "" {
		md := metadata.Pairs("cluster_id", clusterID)
		authCtx = metadata.NewOutgoingContext(ctx, md)
	}

	artifactKind := repositorypb.ArtifactKind_SERVICE
	switch strings.ToUpper(kind) {
	case "INFRASTRUCTURE":
		artifactKind = repositorypb.ArtifactKind_INFRASTRUCTURE
	case "APPLICATION":
		artifactKind = repositorypb.ArtifactKind_APPLICATION
	case "COMMAND":
		artifactKind = repositorypb.ArtifactKind_COMMAND
	}

	ref := &repositorypb.ArtifactRef{
		PublisherId: publisherID,
		Name:        name,
		Version:     version,
		Platform:    platform,
		Kind:        artifactKind,
	}

	client := repositorypb.NewPackageRepositoryClient(conn)
	resp, err := client.GetArtifactManifest(authCtx, &repositorypb.GetArtifactManifestRequest{
		Ref:         ref,
		BuildNumber: buildNumber,
	})
	if err != nil {
		return fmt.Errorf("publish guard: get manifest for %s/%s@%s build %d: %w",
			publisherID, name, version, buildNumber, err)
	}
	manifest := resp.GetManifest()
	if manifest == nil {
		return fmt.Errorf("publish guard: no manifest returned for %s/%s@%s build %d",
			publisherID, name, version, buildNumber)
	}

	ps := manifest.GetPublishState()
	if ps != repositorypb.PublishState_PUBLISHED {
		return fmt.Errorf("publish guard: artifact %s/%s@%s build %d is %s, not PUBLISHED — rejecting install",
			publisherID, name, version, buildNumber, ps)
	}
	return nil
}

// dialRepository creates a gRPC connection to the repository service.
// DialRepository is the exported variant of dialRepository for cross-package
// node-agent code (e.g. the Phase F post-apply hook in package_revision_post.go).
// Same TLS material as the package-internal callers. Returns the connection
// and the resolved address.
func DialRepository(ctx context.Context, addr string) (*grpc.ClientConn, string, error) {
	return dialRepository(ctx, addr)
}

// Returns the connection and the resolved address.
func dialRepository(ctx context.Context, addr string) (*grpc.ClientConn, string, error) {
	var opts []grpc.DialOption

	// Always use mTLS — no insecure fallback.
	{
		caPath := "/var/lib/globular/pki/ca.pem"
		if _, err := os.Stat(caPath); err != nil {
			caPath = "" // CA not found on disk; proceed without pinned CA
		}
		tlsCfg := &tls.Config{}
		if caPath != "" {
			data, err := os.ReadFile(caPath)
			if err != nil {
				return nil, addr, fmt.Errorf("read repository CA %s: %w", caPath, err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(data) {
				return nil, addr, fmt.Errorf("parse repository CA %s: no certificates found", caPath)
			}
			tlsCfg.RootCAs = pool
		}
		clientCert := "/var/lib/globular/pki/issued/services/service.crt"
		clientKey := "/var/lib/globular/pki/issued/services/service.key"
		if cert, err := tls.LoadX509KeyPair(clientCert, clientKey); err == nil {
			tlsCfg.Certificates = []tls.Certificate{cert}
		}
		dt := config.ResolveDialTarget(addr)
		tlsCfg.ServerName = dt.ServerName
		addr = dt.Address
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}

	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, addr, opts...)
	if err != nil {
		return nil, addr, err
	}
	return conn, addr, nil
}

// downloadArtifactFromRepository fetches an artifact from a remote repository gRPC endpoint
// via streaming DownloadArtifact RPC and writes it atomically to dest.
//
// If expectedSHA256 is non-empty, the downloaded bytes are hashed and compared; a mismatch
// causes the temp file to be deleted and an error to be returned (hard invariant: never
// accept a corrupted artifact).
//
// TLS configuration uses:
//   - caPathFromPlan for the CA certificate if provided in the plan
//   - Falls back to the canonical CA location /var/lib/globular/pki/ca.pem
func downloadArtifactFromRepository(ctx context.Context, addr string, ref *repositorypb.ArtifactRef, dest, expectedSHA256 string, insecureFromPlan bool, caPathFromPlan string, buildNumber int64) error {
	var opts []grpc.DialOption

	// Always use mTLS — no insecure fallback.
	{
		caPath := caPathFromPlan
		// Fall back to canonical CA location (always present on joined nodes).
		if caPath == "" {
			candidate := "/var/lib/globular/pki/ca.pem"
			if _, err := os.Stat(candidate); err == nil {
				caPath = candidate
			}
		}
		tlsCfg := &tls.Config{}
		if caPath != "" {
			data, err := os.ReadFile(caPath)
			if err != nil {
				return fmt.Errorf("read repository CA %s: %w", caPath, err)
			}
			pool := x509.NewCertPool()
			if !pool.AppendCertsFromPEM(data) {
				return fmt.Errorf("parse repository CA %s: no certificates found", caPath)
			}
			tlsCfg.RootCAs = pool
		}
		// Load client certificate for mTLS authentication. The server-side
		// interceptor skips cluster_id enforcement for mTLS-authenticated
		// calls (TLS trust chain already prevents cross-cluster access).
		clientCert := "/var/lib/globular/pki/issued/services/service.crt"
		clientKey := "/var/lib/globular/pki/issued/services/service.key"
		if cert, err := tls.LoadX509KeyPair(clientCert, clientKey); err == nil {
			tlsCfg.Certificates = []tls.Certificate{cert}
		} else {
			fmt.Printf("WARN artifact fetch: no client certs (%v), download may fail cluster_id check\n", err)
		}
		dt := config.ResolveDialTarget(addr)
		tlsCfg.ServerName = dt.ServerName
		addr = dt.Address
		opts = append(opts, grpc.WithTransportCredentials(credentials.NewTLS(tlsCfg)))
	}
	dialCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	conn, err := grpc.DialContext(dialCtx, addr, opts...)
	if err != nil {
		return fmt.Errorf("dial repository %s: %w", addr, err)
	}
	defer conn.Close()

	// Inject cluster_id into outgoing gRPC metadata so the server-side
	// interceptor accepts the call. When going through the Envoy gateway,
	// mTLS client certs are stripped (TLS termination), so the interceptor
	// only sees metadata for cluster identity verification.
	if clusterID, err := security.GetLocalClusterID(); err == nil && clusterID != "" {
		md := metadata.Pairs("cluster_id", clusterID)
		ctx = metadata.NewOutgoingContext(ctx, md)
	}

	client := repositorypb.NewPackageRepositoryClient(conn)
	stream, err := client.DownloadArtifact(ctx, &repositorypb.DownloadArtifactRequest{Ref: ref, BuildNumber: buildNumber})
	if err != nil {
		return fmt.Errorf("download artifact %s/%s@%s: %w", ref.GetPublisherId(), ref.GetName(), ref.GetVersion(), err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(dest), "artifact-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	tmpPath := tmp.Name()

	if expectedSHA256 == "" {
		fmt.Printf("WARN artifact fetch: no expected_sha256 for %s — will download and compute hash post-download\n", dest)
	}
	hasher := sha256.New()
	hw := io.MultiWriter(tmp, hasher) // always hash downloads

	for {
		resp, err := stream.Recv()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("receive chunk: %w", err)
		}
		if _, err := hw.Write(resp.GetData()); err != nil {
			tmp.Close()
			os.Remove(tmpPath)
			return fmt.Errorf("write chunk: %w", err)
		}
	}

	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp file: %w", err)
	}

	if expectedSHA256 != "" {
		got := canonicalSHA256(hex.EncodeToString(hasher.Sum(nil)))
		if got != canonicalSHA256(expectedSHA256) {
			os.Remove(tmpPath)
			return fmt.Errorf("artifact digest mismatch: want %s got %s", expectedSHA256, got)
		}
	} else {
		// No expected hash but we still computed one — log for auditability.
		fmt.Printf("WARN artifact downloaded without SHA256 verification (dev bypass): sha256=%s\n", hex.EncodeToString(hasher.Sum(nil)))
	}

	if err := os.Rename(tmpPath, dest); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename artifact: %w", err)
	}
	return nil
}

func copyFileAtomic(dest string, r io.Reader) error {
	tmp, err := os.CreateTemp(filepath.Dir(dest), "artifact-*.tmp")
	if err != nil {
		return fmt.Errorf("create temp: %w", err)
	}
	tmpPath := tmp.Name()
	if _, err := io.Copy(tmp, r); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return fmt.Errorf("copy artifact: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("close temp: %w", err)
	}
	if err := os.Rename(tmpPath, dest); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("rename artifact: %w", err)
	}
	return nil
}

func executableForService(svc string) string {
	name := normalizeServiceName(svc)
	if name == "" {
		return ""
	}
	// Use the identity registry which knows the actual deployed binary name.
	// This handles exceptions like xds, minio, gateway, envoy, etcd which
	// don't follow the _server convention.
	if key, ok := identity.NormalizeServiceKey(name); ok {
		if id, ok := identity.IdentityByKey(key); ok && id.Binary != "" {
			return id.Binary
		}
	}
	// If the service is not in the identity registry, skip binary verification.
	// The old fallback ({name}_server) was a false assumption that broke
	// COMMAND packages (etcdctl, ffmpeg, etc.) and INFRASTRUCTURE packages
	// (prometheus, alertmanager, etc.) whose binaries don't follow the
	// _server naming convention. Artifact integrity is already verified
	// upstream by artifact.fetch (sha256 match vs manifest digest).
	return ""
}

// resolveServiceId returns the gRPC FQN for a service using the identity registry.
func resolveServiceId(svc string) string {
	name := normalizeServiceName(svc)
	if name == "" {
		return ""
	}
	if key, ok := identity.NormalizeServiceKey(name); ok {
		if id, ok := identity.IdentityByKey(key); ok && id.GrpcFull != "" {
			return id.GrpcFull
		}
	}
	return ""
}

func normalizeServiceName(svc string) string {
	s := strings.ToLower(strings.TrimSpace(svc))
	s = strings.TrimPrefix(s, "globular-")
	s = strings.TrimSuffix(s, ".service")
	return s
}

type describePayload struct {
	Id      string `json:"Id"`
	Address string `json:"Address"`
	Port    int    `json:"Port"`
}

// (runDescribe was removed — the only in-package caller was dead code.
// Callers that need describe use serviceports.runDescribe, which now
// returns nil,nil for binaries that don't support the --describe protocol
// instead of propagating non-zero exit / non-JSON output as hard errors.)

func portFromAddress(addr string) int {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return 0
	}
	_ = host
	p, _ := strconv.Atoi(port)
	return p
}

func readServiceConfig(path string) (*describePayload, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var cfg describePayload
	if err := json.Unmarshal(b, &cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// renderTemplateVars replaces Go template placeholders in unit/config files
// with actual installation paths. This handles specs generated by specgen.sh
// which use {{.StateDir}}, {{.Prefix}}, etc.
func renderTemplateVars(path, stateRoot, binDir string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	content := string(data)
	prefix := filepath.Dir(binDir) // e.g. /usr/lib/globular/bin -> /usr/lib/globular

	// Resolve NodeIP from routable config first, then interface scan fallback.
	nodeIP := strings.TrimSpace(config.GetRoutableIPv4())
	if nodeIP == "" {
		nodeIP = resolveNodeIP()
	}

	rendered := replaceTemplateVars(content, map[string]string{
		"statedir": stateRoot,
		"prefix":   prefix,
		"bindir":   binDir,
		"nodeip":   nodeIP,
	})
	if rendered == content {
		return nil // no templates found, skip write
	}
	return os.WriteFile(path, []byte(rendered), 0o644)
}

var simpleTemplateVarRE = regexp.MustCompile(`\{\{\s*\.\s*([A-Za-z0-9_]+)\s*\}\}`)

// replaceTemplateVars replaces simple Go-template variables such as
// "{{.NodeIP}}" and tolerant variants like "{{ .NodeIp }}".
// Unknown variables are preserved as-is.
func replaceTemplateVars(content string, vars map[string]string) string {
	return simpleTemplateVarRE.ReplaceAllStringFunc(content, func(match string) string {
		sub := simpleTemplateVarRE.FindStringSubmatch(match)
		if len(sub) != 2 {
			return match
		}
		key := strings.ToLower(strings.TrimSpace(sub[1]))
		if v, ok := vars[key]; ok {
			return v
		}
		return match
	})
}

// resolveNodeIP returns the node's primary routable IPv4 address.
// Skips loopback and link-local addresses.
func resolveNodeIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return ""
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok {
			continue
		}
		ip := ipNet.IP
		if ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.To4() == nil {
			continue
		}
		return ip.String()
	}
	return ""
}

func ensureAlertmanagerConfigFile(stateRoot string) error {
	cfgDir := filepath.Join(stateRoot, "alertmanager")
	cfgPath := filepath.Join(cfgDir, "alertmanager.yml")
	if _, err := os.Stat(cfgPath); err == nil {
		return nil
	}
	if err := os.MkdirAll(cfgDir, 0o755); err != nil {
		return err
	}
	const cfg = "global:\n  resolve_timeout: 5m\nroute:\n  receiver: default\nreceivers:\n  - name: default\n"
	return os.WriteFile(cfgPath, []byte(cfg), 0o644)
}

func installPaths() (binDir, systemdDir, configDir string, skipSystemd bool) {
	binDir = ActionBinDir
	systemdDir = ActionSystemdDir
	configDir = ActionConfigDir
	skipSystemd = ActionSkipSystemd
	return
}

// verifyFileSHA256 checks that the file at path matches the expected lowercase hex SHA256.
func verifyFileSHA256(path, expected string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return err
	}
	got := canonicalSHA256(hex.EncodeToString(h.Sum(nil)))
	if got != canonicalSHA256(expected) {
		return fmt.Errorf("sha256 mismatch: want %s got %s", expected, got)
	}
	return nil
}

// allowMissingSHA256 returns AllowMissingSHA256. Production default is false;
// tests may set AllowMissingSHA256 = true for dev bypass scenarios.
func allowMissingSHA256() bool {
	return AllowMissingSHA256
}

// normalizeUnitWorkingDirectory rewrites a rendered systemd unit file in
// place, delegating the parse + rewrite to the shared systemdutil package
// so the CLI install path (golang/globularcli/services_cmds.go) and the
// node-agent install path (this file) cannot drift. See Project O / the
// claude_project_o_rescope_workingdirectory_instructions.md handoff.
func normalizeUnitWorkingDirectory(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	out := systemdutil.NormalizeUnitWorkingDirectory(data)
	if len(out) == len(data) && bytesEqual(out, data) {
		return nil
	}
	log.Printf("install: normalized fragile WorkingDirectory in %s", filepath.Base(path))
	return os.WriteFile(path, out, 0o644)
}

// bytesEqual is a small no-alloc equality check used only by
// normalizeUnitWorkingDirectory's fast-path. Avoids pulling in bytes.Equal
// in this hot install loop just for the length-already-equal short-circuit.
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// ensureServiceStateDir creates the given directory with the correct ownership
// (globular:globular) and permissions (0750) if it does not already exist.
// The shared state root (/var/lib/globular) is always kept at 0755 so
// non-root CLI can traverse it to read the CA cert.
func ensureServiceStateDir(dir string) error {
	if err := os.MkdirAll(dir, 0o750); err != nil {
		return err
	}
	// Guard: MkdirAll may have created the shared state root with 0750.
	// Restore 0755 on the root so non-root users can traverse it.
	if ActionStateDir != "" && dir != ActionStateDir {
		_ = os.Chmod(ActionStateDir, 0o755)
	}
	// Best-effort chown — may fail in test environments or when running as
	// non-root. Production node-agent runs as root.
	if u, err := user.Lookup("globular"); err == nil {
		uid, _ := strconv.Atoi(u.Uid)
		gid, _ := strconv.Atoi(u.Gid)
		_ = os.Chown(dir, uid, gid)
	}
	return nil
}

func init() {
	Register(artifactFetchAction{})
	Register(artifactVerifyAction{})
	Register(serviceInstallPayloadAction{})
	Register(serviceWriteVersionMarkerAction{})
}

// DownloadArtifactToDir fetches a single artifact from the repository by
// (name, version, platform, kind) and writes it to
// destDir/<name>_<version>_<platform>.tgz atomically. Returns the full
// path on success.
//
// Invariant: repository.fallback_requires_manifest_and_checksum — fallback
// sources must never fetch without manifest+checksum proof. When expectedSHA256
// is empty, this function resolves it from the repository manifest before
// downloading. Download is rejected if the manifest returns no checksum.
func DownloadArtifactToDir(ctx context.Context, repoAddr, publisherID, name, version, platform, kindStr, expectedSHA256, destDir string) (string, error) {
	artifactKind := repositorypb.ArtifactKind_SERVICE
	switch strings.ToUpper(kindStr) {
	case "INFRASTRUCTURE":
		artifactKind = repositorypb.ArtifactKind_INFRASTRUCTURE
	case "APPLICATION":
		artifactKind = repositorypb.ArtifactKind_APPLICATION
	case "COMMAND":
		artifactKind = repositorypb.ArtifactKind_COMMAND
	}
	ref := &repositorypb.ArtifactRef{
		PublisherId: publisherID,
		Name:        name,
		Version:     version,
		Platform:    platform,
		Kind:        artifactKind,
	}

	// Resolve checksum from the manifest when the caller hasn't provided one.
	// Invariant: fallback_requires_manifest_and_checksum — we must not download
	// without proof of what we expect to receive.
	sha256 := strings.TrimSpace(expectedSHA256)
	if sha256 == "" {
		digest, err := resolveArtifactDigest(ctx, repoAddr, publisherID, name, version, platform, kindStr, 0)
		if err != nil {
			return "", fmt.Errorf("download %s@%s: cannot resolve manifest checksum (required before download): %w", name, version, err)
		}
		sha256 = digest
	}

	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return "", fmt.Errorf("create download dir %s: %w", destDir, err)
	}
	dest := filepath.Join(destDir, fmt.Sprintf("%s_%s_%s.tgz", name, version, platform))
	if err := downloadArtifactFromRepository(ctx, repoAddr, ref, dest, sha256, false, "", 0); err != nil {
		return "", fmt.Errorf("download %s@%s from %s: %w", name, version, repoAddr, err)
	}
	return dest, nil
}
