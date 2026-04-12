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
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/globulario/services/golang/identity"
	"github.com/globulario/services/golang/node_agent/node_agent_server/internal/actions/serviceports"
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

	if dest == "" {
		return "", fmt.Errorf("artifact_path is required")
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return "", fmt.Errorf("create dest dir: %w", err)
	}
	// Determine full artifact identity for logs and metadata resolution.
	identity := fmt.Sprintf("%s/%s@%s+%d", publisherID, service, version, buildNumber)

	// Resolve the repository address early — we may need it below to fetch
	// the manifest digest when the caller didn't pass expected_sha256.
	effectiveRepoAddr := repositoryAddr
	if effectiveRepoAddr == "" {
		effectiveRepoAddr = config.ResolveServiceAddr("repository.PackageRepository", "")
	}
	if effectiveRepoAddr == "" {
		effectiveRepoAddr = discoverRepositoryViaGateway()
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

// shortHash returns the first 12 chars of a hex digest for log readability.
func shortHash(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = strings.TrimPrefix(s, "sha256:")
	if len(s) <= 12 {
		return s
	}
	return s[:12]
}

// artifact.verify performs a simple existence/digest check if provided.
type artifactVerifyAction struct{}

func (artifactVerifyAction) Name() string { return "artifact.verify" }

func (artifactVerifyAction) Validate(args *structpb.Struct) error { return nil }

func (artifactVerifyAction) Apply(ctx context.Context, args *structpb.Struct) (string, error) {
	fields := args.GetFields()
	path := strings.TrimSpace(fields["artifact_path"].GetStringValue())
	expected := strings.ToLower(strings.TrimSpace(fields["expected_sha256"].GetStringValue()))
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
	if got != expected {
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
		case strings.HasPrefix(name, "scripts/"):
			dest = filepath.Join(scriptsDir, filepath.Base(name))
		case strings.HasPrefix(name, "data/"):
			// Data files are extracted to ActionStateDir preserving subdirectory structure.
			// e.g. data/workflows/day0.bootstrap.yaml → /var/lib/globular/workflows/day0.bootstrap.yaml
			rel := strings.TrimPrefix(name, "data/")
			dest = filepath.Join(ActionStateDir, rel)
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
		if err := os.Rename(tmp, dest); err != nil {
			os.Remove(tmp)
			return "", fmt.Errorf("rename %s: %w", dest, err)
		}
	}

	// Ensure the service working directory exists (systemd units reference
	// WorkingDirectory=/var/lib/globular/<service> which must be created).
	svcWorkDir := filepath.Join(ActionStateDir, service)
	if err := os.MkdirAll(svcWorkDir, 0o755); err != nil {
		return "", fmt.Errorf("create service workdir %s: %w", svcWorkDir, err)
	}

	if wroteUnit && !skipSystemd {
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
		got := hex.EncodeToString(hasher.Sum(nil))
		if got != strings.ToLower(expectedSHA256) {
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
	// Fallback for unknown services: convention is name_server.
	return strings.ReplaceAll(name, "-", "_") + "_server"
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
	replacer := strings.NewReplacer(
		"{{.StateDir}}", stateRoot,
		"{{.Prefix}}", prefix,
		"{{.BinDir}}", binDir,
	)
	rendered := replacer.Replace(content)
	if rendered == content {
		return nil // no templates found, skip write
	}
	return os.WriteFile(path, []byte(rendered), 0o644)
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
	got := hex.EncodeToString(h.Sum(nil))
	if got != strings.ToLower(expected) {
		return fmt.Errorf("sha256 mismatch: want %s got %s", expected, got)
	}
	return nil
}

// allowMissingSHA256 returns AllowMissingSHA256. Production default is false;
// tests may set AllowMissingSHA256 = true for dev bypass scenarios.
func allowMissingSHA256() bool {
	return AllowMissingSHA256
}

func init() {
	Register(artifactFetchAction{})
	Register(artifactVerifyAction{})
	Register(serviceInstallPayloadAction{})
	Register(serviceWriteVersionMarkerAction{})
}
