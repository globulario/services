package main

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions"
	"github.com/globulario/services/golang/versionutil"
	"google.golang.org/protobuf/types/known/structpb"
)

const defaultPublisherID = "core@globular.io"

// InstallPackage fetches a package artifact from the repository and installs
// it locally. This is the public entry point for the workflow engine bridge.
//
// Parameters:
//   - name: package name (e.g. "dns", "envoy")
//   - kind: SERVICE, INFRASTRUCTURE, or COMMAND
//   - repositoryAddr: gRPC address of the repository (e.g. "10.0.0.63:443")
//
// localPackageDirs are searched (in order) when the repository is unreachable.
// The installer script copies .tgz packages here before starting the workflow.
var localPackageDirs = []string{
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
				if _, err := fetchHandler.Apply(ctx, fetchArgs); err == nil {
					fetched = true
				} else {
					log.Printf("installer-api: repo fetch failed for %s: %v — trying local fallback", name, err)
				}
			}
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
	}
	return ""
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

// resolvePackageVersion returns the version for a given package name.
// Infrastructure packages have specific versions; services default to 0.0.1.
func resolvePackageVersion(name string) string {
	switch name {
	case "etcd":
		return "3.5.14"
	case "envoy":
		return "1.35.3"
	case "sidekick":
		return "7.0.0"
	case "prometheus":
		return "3.5.1"
	case "alertmanager":
		return "0.28.1"
	case "node-exporter":
		return "1.10.2"
	case "scylladb":
		return "2025.3.8"
	case "scylla-manager", "scylla-manager-agent":
		return "3.8.1"
	case "mcp":
		return "0.0.2"
	default:
		return "0.0.1"
	}
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
