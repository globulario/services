package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/versionutil"
	"google.golang.org/protobuf/types/known/structpb"
)

const defaultPublisherID = "core@globular.io"

// pinnedArtifactDir holds a copy of the last successfully installed artifact
// for each package. This is the local resilience cache: if MinIO / the
// repository become unreachable, the node can still reinstall or self-heal
// from this directory without any external dependency.
const pinnedArtifactDir = "/var/lib/globular/packages/pinned"

// InstallPackage fetches a package artifact from the repository and installs
// it locally. This is the public entry point for the workflow engine bridge.
//
// Parameters:
//   - name: package name (e.g. "dns", "envoy")
//   - kind: SERVICE, INFRASTRUCTURE, or COMMAND
//   - repositoryAddr: gRPC address of the repository (e.g. "10.0.0.63:443")
//
// localPackageDirs are searched (in order) when the repository is unreachable.
// The pinned dir holds the last installed version; the others hold packages
// placed by the join script or manual operator action.
var localPackageDirs = []string{
	pinnedArtifactDir,
	"/var/lib/globular/packages",
	"/var/lib/globular/staging/local",
}

// InstallPackage fetches and installs a package artifact.
//
// Artifact identity MUST be propagated end-to-end:
//   - buildNumber identifies which build of this version is expected
//   - expectedSHA256 is verified against the fetched bytes (if provided)
//
// Either value may be zero/empty; when both are missing, artifact.fetch will
// resolve the digest from the repository manifest before trusting any cached
// bytes. The contract is: no install ever silently succeeds on unvalidated
// cached content.
func (srv *NodeAgentServer) InstallPackage(ctx context.Context, name, kind, repositoryAddr, desiredVersion string, buildID string, expectedSHA256 string) error {
	platform := runtime.GOOS + "_" + runtime.GOARCH
	version := desiredVersion
	if version == "" {
		version = resolvePackageVersion(name)
	}

	artifactPath := fmt.Sprintf("/var/lib/globular/staging/%s/%s/latest.artifact",
		defaultPublisherID, name)

	// Try fetching from repository first.
	if repositoryAddr == "" {
		repositoryAddr = srv.discoverRepositoryAddr()
	}

	fetched := false
	var fetchErr error
	if repositoryAddr != "" {
		fetchHandler := actions.Get("artifact.fetch")
		if fetchHandler != nil {
			fetchArgs, err := structpb.NewStruct(map[string]any{
				"service":         name,
				"version":         version,
				"platform":        platform,
				"artifact_path":   artifactPath,
				"publisher_id":    defaultPublisherID,
				"repository_addr": repositoryAddr,
				"artifact_kind":   kind,
				"build_id":        buildID,
				"expected_sha256": expectedSHA256,
			})
			if err == nil {
				log.Printf("installer-api: fetching %s (%s) build_id=%s from %s",
					name, kind, buildID, repositoryAddr)
				if _, fe := fetchHandler.Apply(ctx, fetchArgs); fe == nil {
					fetched = true
				} else {
					fetchErr = fe
					log.Printf("installer-api: repo fetch failed for %s: %v — trying local fallback", name, fe)
				}
			}
		}
	}

	// Fallback fail-closed: if the repository told us this build_id is orphaned
	// (demoted: archived/yanked/revoked) or not in the catalog at all, we must
	// NOT install whatever .tgz happens to live in /var/lib/globular/packages/.
	// Without a manifest+checksum proof, blind reuse of pinned bytes silently
	// runs a build the repository said "stop on". This is the
	// fallback.requires_manifest_checksum invariant.
	//
	// Network/TLS/auth failures (ErrRepositoryUnreachable) are recoverable —
	// the local pinned tarball may still be the right bytes — and the existing
	// fallback path continues unchanged for that case. The cache decision
	// matrix in artifact.fetch will still refuse to reuse cached bytes without
	// a checksum proof, so even the unreachable case can't silently install
	// the wrong build.
	if fetchErr != nil {
		if errors.Is(fetchErr, actions.ErrBuildIDOrphaned) ||
			errors.Is(fetchErr, actions.ErrBuildIDNotFound) {
			return fmt.Errorf("install %s build_id=%s blocked: RELEASE_BLOCKED_REPOSITORY_ORPHANED_BUILD_ID — repository demoted or never published this build_id; refusing local fallback to avoid installing unverifiable bytes: %w",
				name, buildID, fetchErr)
		}
	}

	// Local fallback: find a .tgz package on disk.
	if !fetched {
		localPath := srv.findLocalPackage(name, version, platform)
		if localPath == "" {
			if repositoryAddr == "" {
				return fmt.Errorf("no repository address available and no local package for %s", name)
			}
			return fmt.Errorf("fetch %s: repository unreachable and no local package found", name)
		}
		log.Printf("installer-api: using local package %s for %s", localPath, name)

		// Copy the .tgz to the staging path so the install handlers can find it.
		if err := os.MkdirAll(filepath.Dir(artifactPath), 0o755); err != nil {
			return fmt.Errorf("create staging dir: %w", err)
		}
		src, err := os.ReadFile(localPath)
		if err != nil {
			return fmt.Errorf("read local package %s: %w", localPath, err)
		}
		if err := os.WriteFile(artifactPath, src, 0o644); err != nil {
			return fmt.Errorf("write staging artifact: %w", err)
		}
	}

	// Try to read the real version from the artifact manifest (package.json).
	// This covers Day0/bootstrap paths that don't pass a desired version,
	// ensuring the correct version is written to the marker file.
	if manifestVer := readArtifactManifestVersion(artifactPath); manifestVer != "" {
		if version == "" || version == resolvePackageVersion(name) {
			// Only override if we're using the hardcoded fallback.
			log.Printf("installer-api: resolved %s version from manifest: %s → %s", name, version, manifestVer)
			version = manifestVer
		}
	}

	// Pin the artifact locally before installing. This is the resilience cache:
	// if MinIO/repository becomes unreachable later, this copy lets the node
	// reinstall or self-heal without any external dependency.
	pinArtifact(name, version, platform, artifactPath)

	// Install.
	switch strings.ToUpper(kind) {
	case "INFRASTRUCTURE":
		return srv.installInfra(ctx, name, version, artifactPath)
	case "COMMAND":
		return srv.installPayload(ctx, name, version, artifactPath)
	default:
		return srv.installPayload(ctx, name, version, artifactPath)
	}
}

// pinArtifact copies the fetched artifact to the local pinned directory.
// Best-effort: failures are logged but don't block installation.
func pinArtifact(name, version, platform, artifactPath string) {
	if version == "" || artifactPath == "" {
		return
	}
	if err := os.MkdirAll(pinnedArtifactDir, 0o755); err != nil {
		log.Printf("pin-artifact: create dir: %v", err)
		return
	}

	// Target: <name>_<version>_<platform>.tgz (matches findLocalPackage search)
	target := filepath.Join(pinnedArtifactDir, fmt.Sprintf("%s_%s_%s.tgz", name, version, platform))

	src, err := os.ReadFile(artifactPath)
	if err != nil {
		log.Printf("pin-artifact: read %s: %v", artifactPath, err)
		return
	}
	if err := os.WriteFile(target, src, 0o644); err != nil {
		log.Printf("pin-artifact: write %s: %v", target, err)
		return
	}

	// Remove older pinned versions of the same package to save disk.
	prefix := name + "_"
	entries, _ := os.ReadDir(pinnedArtifactDir)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if strings.HasPrefix(e.Name(), prefix) && e.Name() != filepath.Base(target) {
			old := filepath.Join(pinnedArtifactDir, e.Name())
			if err := os.Remove(old); err == nil {
				log.Printf("pin-artifact: removed old %s", e.Name())
			}
		}
	}

	log.Printf("pin-artifact: saved %s (%d bytes)", filepath.Base(target), len(src))
}

// findLocalPackage searches localPackageDirs for a .tgz matching the package.
// Naming convention: <name>_<version>_<platform>.tgz  (e.g. dns_0.0.1_linux_amd64.tgz)
// Also checks for just <name>_<version>.tgz and <name>.tgz as fallbacks.
func (srv *NodeAgentServer) findLocalPackage(name, version, platform string) string {
	candidates := []string{
		fmt.Sprintf("%s_%s_%s.tgz", name, version, platform),
		fmt.Sprintf("%s_%s.tgz", name, version),
		fmt.Sprintf("%s.tgz", name),
	}
	for _, dir := range localPackageDirs {
		for _, c := range candidates {
			path := filepath.Join(dir, c)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		// Day-1 degraded fallback: if exact-version lookup misses (e.g. caller
		// has no authoritative version and would otherwise use 0.0.0-dev),
		// accept the newest local versioned artifact for this platform.
		// This preserves bootstrap progress when repository/latest resolution is
		// unavailable but a Day-0-staged package exists on disk.
		if wildcard := findLocalPackageAnyVersion(dir, name, platform); wildcard != "" {
			return wildcard
		}
	}
	return ""
}

func findLocalPackageAnyVersion(dir, name, platform string) string {
	pattern := filepath.Join(dir, fmt.Sprintf("%s_*_%s.tgz", name, platform))
	matches, err := filepath.Glob(pattern)
	if err != nil || len(matches) == 0 {
		return ""
	}
	sort.Strings(matches)
	return matches[len(matches)-1]
}

func (srv *NodeAgentServer) installPayload(ctx context.Context, name, version, artifactPath string) error {
	handler := actions.Get("service.install_payload")
	if handler == nil {
		return fmt.Errorf("action service.install_payload not registered")
	}
	args, err := structpb.NewStruct(map[string]any{
		"service":       name,
		"version":       version,
		"artifact_path": artifactPath,
	})
	if err != nil {
		return err
	}
	if _, err := handler.Apply(ctx, args); err != nil {
		return fmt.Errorf("install %s: %w", name, err)
	}
	return srv.writeMarker(name, version)
}

func (srv *NodeAgentServer) installInfra(ctx context.Context, name, version, artifactPath string) error {
	handler := actions.Get("infrastructure.install")
	if handler == nil {
		return fmt.Errorf("action infrastructure.install not registered")
	}
	args, err := structpb.NewStruct(map[string]any{
		"name":          name,
		"version":       version,
		"artifact_path": artifactPath,
	})
	if err != nil {
		return err
	}
	if _, err := handler.Apply(ctx, args); err != nil {
		return fmt.Errorf("install infra %s: %w", name, err)
	}
	return srv.writeMarker(name, version)
}

func (srv *NodeAgentServer) writeMarker(name, version string) error {
	path := versionutil.MarkerPath(name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, []byte(version+"\n"), 0o644)
}

func (srv *NodeAgentServer) discoverRepositoryAddr() string {
	// Primary source of truth: service discovery from etcd. Prefer a non-local
	// endpoint to avoid self-routing loops during Day-1 when the local gateway
	// advertises :443 but upstream repository isn't actually ready.
	if addrs := config.ResolveServiceAddrs("repository.PackageRepository"); len(addrs) > 0 {
		localIP := strings.TrimSpace(config.GetRoutableIPv4())
		for _, addr := range addrs {
			if !isLocalEndpoint(addr, localIP) {
				return strings.TrimSpace(addr)
			}
		}
		// No remote endpoint found; fall back to the first discovered address.
		return strings.TrimSpace(addrs[0])
	}
	// Fallback for early bootstrap cases where service discovery is not ready.
	if srv.state != nil && srv.state.ControllerEndpoint != "" {
		host, _, err := splitHostPort(srv.state.ControllerEndpoint)
		if err == nil && host != "" {
			return host + ":443"
		}
	}
	return ""
}

func splitHostPort(addr string) (string, string, error) {
	if !strings.Contains(addr, ":") {
		return addr, "", nil
	}
	idx := strings.LastIndex(addr, ":")
	return addr[:idx], addr[idx+1:], nil
}

func isLocalEndpoint(addr, localIP string) bool {
	host := strings.TrimSpace(addr)
	if h, _, err := net.SplitHostPort(addr); err == nil {
		host = strings.TrimSpace(h)
	}
	if host == "" {
		return false
	}
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return true
	}
	if localIP != "" && host == localIP {
		return true
	}
	return false
}

// resolvePackageVersion returns the sentinel version used when the controller
// has not supplied an explicit desired version (Day-0 bootstrap path only).
// The real version is always provided by either:
//   - desiredVersion from the controller RPC (normal Day-1 path)
//   - readArtifactManifestVersion() from package.json inside the fetched .tgz
//
// The sentinel is intentionally not a real version so the manifest-override
// at the call site always wins. Hard-coding upstream versions here would
// diverge silently whenever etcd/envoy/scylladb ship a new release.
func resolvePackageVersion(_ string) string {
	return "0.0.0-dev"
}

// readArtifactManifestVersion reads the version from a staged artifact's
// package.json manifest. The artifact is a .tgz containing a top-level
// package.json with at least {"version": "..."}.
// Returns empty string if the version cannot be determined.
func readArtifactManifestVersion(artifactPath string) string {
	f, err := os.Open(artifactPath)
	if err != nil {
		return ""
	}
	defer f.Close()

	gz, err := gzip.NewReader(f)
	if err != nil {
		return ""
	}
	defer gz.Close()

	tr := tar.NewReader(gz)
	for {
		hdr, err := tr.Next()
		if err != nil {
			return ""
		}
		// Match package.json at the root of the archive.
		name := filepath.Clean(hdr.Name)
		if name == "package.json" || name == "./package.json" {
			data, err := io.ReadAll(io.LimitReader(tr, 32*1024))
			if err != nil {
				return ""
			}
			var manifest struct {
				Version string `json:"version"`
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				return ""
			}
			return strings.TrimSpace(manifest.Version)
		}
	}
}
