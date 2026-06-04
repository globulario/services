// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.installer_api
// @awareness file_role=controller_intent_to_node_local_install_apply_bridge_with_artifact_extraction
// @awareness implements=globular.platform:intent.node_agent.is_executor_not_cluster_brain
// @awareness implements=globular.platform:intent.controller.apply_package_release_must_carry_expected_sha256
// @awareness enforces=globular.platform:invariant.controller.apply_package_release_requires_manifest_checksum
// @awareness risk=critical
package main

// installer_api.go — the leaf-work executor for controller-issued
// install/apply intent. Receives ApplyPackageRelease dispatches
// carrying expected_sha256 from manifest.entrypoint_checksum;
// the install MUST verify the downloaded artifact's sha256 matches
// before any file is moved into place.
//
// Aliasing desired_hash (convergence identity) into ExpectedSha256
// (binary integrity) was incident
// node_agent.install_package_aliases_convergence_hash_into_expected_sha256
// — every dispatched install failed verify because the convergence
// hash never matches a binary sha256. Keep the two fields distinct;
// they describe different invariants.

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
	"sort"
	"strings"
	"time"

	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/installed_state"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
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

// InstallPackage installs a package artifact from local disk.
// Packages are distributed via the gateway's /join/packages/ endpoint and
// stored in /var/lib/globular/packages/ before the node-agent starts.
// The repository/MinIO path has been removed — local disk is the sole source.
func (srv *NodeAgentServer) InstallPackage(ctx context.Context, name, kind, repositoryAddr, desiredVersion string, buildID string, expectedSHA256 string) error {
	platform := runtime.GOOS + "_" + runtime.GOARCH
	version := strings.TrimSpace(desiredVersion)
	if version == "" {
		if bomVersion, bomErr := resolveVersionFromReleaseIndexFunc(name); bomErr == nil && strings.TrimSpace(bomVersion) != "" {
			version = strings.TrimSpace(bomVersion)
			log.Printf("installer-api: resolved %s version from release-index: %s", name, version)
		}
		if version == "" {
			return fmt.Errorf("resolve package version for %s (%s): no desired version provided and release-index lookup failed", name, kind)
		}
	}

	artifactPath := fmt.Sprintf("/var/lib/globular/staging/%s/%s/latest.artifact",
		defaultPublisherID, name)

	// Find the .tgz package on disk. If not present, fetch it from the repository
	// so that `pkg publish` + `services desired set` converges without requiring
	// the operator to manually stage every package on every node.
	localPath := srv.findLocalPackage(name, version, platform)
	if localPath == "" {
		repoAddr := srv.discoverRepositoryAddr()
		if repoAddr == "" {
			return fmt.Errorf("install %s: package not found in local dirs %v (version=%s) and repository address unavailable", name, localPackageDirs, version)
		}
		log.Printf("installer-api: %s@%s not found locally, downloading from repository %s", name, version, repoAddr)
		// expectedSHA256 here is the manifest's entrypoint_checksum (the
		// binary digest), per INC-2026-0014's dispatch contract. It is
		// passed through but NOT used for bundle verification —
		// DownloadArtifactToDir resolves the bundle digest from the
		// manifest itself. See invariant
		// install_package.hash_schemas_must_not_alias.
		dlPath, dlErr := actions.DownloadArtifactToDir(ctx, repoAddr, defaultPublisherID, name, version, platform, kind, expectedSHA256, "/var/lib/globular/packages")
		if dlErr != nil {
			return fmt.Errorf("install %s: not in local dirs and repository download failed: %w", name, dlErr)
		}
		localPath = dlPath
		log.Printf("installer-api: downloaded %s@%s to %s", name, version, dlPath)
	}
	log.Printf("installer-api: installing %s from local package %s", name, localPath)

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
	var installErr error
	switch strings.ToUpper(kind) {
	case "INFRASTRUCTURE":
		installErr = srv.installInfra(ctx, name, version, artifactPath)
	default: // SERVICE, COMMAND
		installErr = srv.installPayload(ctx, name, version, artifactPath)
	}
	if installErr != nil {
		return installErr
	}

	// Project T: persist the manifest's entrypoint binary filename as a
	// sidecar so the verifier/drift logic can resolve the installed binary
	// path without inferring it from the package name. Pre-fix, packages
	// whose name uses hyphens but whose entrypoint uses underscores (e.g.
	// scylla-manager → scylla_manager) caused the verifier to look at the
	// wrong path and the drift reconciler to dispatch reinstall forever.
	if entrypoint := readArtifactManifestEntrypoint(artifactPath); entrypoint != "" {
		if err := versionutil.WriteEntrypoint(name, entrypoint); err != nil {
			log.Printf("installer-api: warn write entrypoint sidecar for %s: %v", name, err)
		}
	}

	// Write entrypoint_checksum to the etcd installed-state record. Without
	// this, the heartbeat compares the binary hash against an empty checksum
	// and reports hash_drift, and the runtime verifier has no hash to produce
	// a VERIFIED verdict (UNVERIFIED finding). ApplyPackageRelease does this
	// on the normal reconciler path; here we mirror that behaviour for packages
	// installed via the join workflow.
	srv.writeInstalledStateChecksum(ctx, name, strings.ToUpper(kind), version, buildID)
	return nil
}

// writeInstalledStateChecksum computes the SHA256 of the installed binary and
// writes it to the etcd installed-state record as entrypoint_checksum. Called
// from the join-workflow install path to prevent hash_drift and UNVERIFIED.
// Best-effort: failures are logged but never block the install.
func (srv *NodeAgentServer) writeInstalledStateChecksum(ctx context.Context, name, kind, version, buildID string) {
	path := installedBinaryPath(name, kind)
	hash, err := cachedSha256(path)
	if err != nil || hash == "" {
		log.Printf("installer-api: skip entrypoint_checksum for %s (%s): binary not hashable: %v", name, kind, err)
		return
	}

	// Read-modify-write: preserve all fields written by package.report_state
	// (Checksum/artifact hash, Platform, BuildNumber, etc.) and only add
	// entrypoint_checksum. A full replace would silently clear those fields.
	pkg, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, kind, name)
	now := time.Now().Unix()
	if pkg == nil {
		pkg = &node_agentpb.InstalledPackage{
			NodeId:        srv.nodeID,
			Name:          name,
			Version:       version,
			Kind:          kind,
			Status:        "installed",
			InstalledUnix: now,
			BuildId:       buildID,
		}
	}
	pkg.UpdatedUnix = now
	if pkg.Metadata == nil {
		pkg.Metadata = make(map[string]string)
	}
	pkg.Metadata["entrypoint_checksum"] = hash

	// Stamp the canonical install receipt. installed_state.metadata is the
	// sole authority for expected unit-file/binary/config content (see
	// docs/architecture/retire-systemd-sidecars.md). Best-effort: if the
	// receipt cannot be computed (binary missing, unit file missing) we
	// still write the entrypoint_checksum so the rest of the install
	// proof chain holds — the missing receipt fields will surface as
	// installed_state_missing_or_unproven at heartbeat time, which is
	// the correct fail-closed behaviour.
	unitPath := filepath.Join("/etc/systemd/system", "globular-"+name+".service")
	receiptOpts := ReceiptOpts{
		BinaryPath:  path,
		InstalledBy: "node-agent.installer-api",
	}
	if _, statErr := os.Stat(unitPath); statErr == nil {
		receiptOpts.UnitFilePath = unitPath
	}
	if rerr := StampInstallReceipt(pkg, receiptOpts); rerr != nil {
		log.Printf("installer-api: install receipt skipped for %s: %v (entrypoint_checksum still committed)", name, rerr)
	}

	if werr := installed_state.WriteInstalledPackage(ctx, pkg); werr != nil {
		log.Printf("installer-api: write installed-state for %s: %v (non-fatal)", name, werr)
		return
	}
	log.Printf("installer-api: stored entrypoint_checksum for %s: %s", name, hash[:16])
}

// restampReceiptOnInstallSkip re-stamps the canonical install receipt
// for a package whose install workflow short-circuited because the
// package is already at the desired version with an active unit (the
// installSkipAllowed branch in grpc_workflow.go).
//
// Without this call, packages whose receipt was seeded from a
// legacy_sidecar migration would never gain a canonical installed_by:
// the install path that proves on-disk content matches the desired
// version is exactly the path that should re-stamp the receipt with
// canonical provenance. INFRASTRUCTURE/wrapper packages (envoy,
// keepalived, etc.) hit this case repeatedly because their install
// frequently short-circuits — same unit content survives reboots
// and apply-desired sweeps. Live regression observed 2026-06-03 on
// globular-envoy.service.
//
// Best-effort: failures are logged but never affect the skip's
// SUCCEEDED verdict. The skip itself already proved the package is
// correctly installed; stamping is forensic metadata, not a
// precondition.
//
// Attribution "node-agent.grpc_workflow.install_skip_restamp"
// distinguishes skip-path stamps from normal installer-api stamps
// in audit / forensic queries.
func (srv *NodeAgentServer) restampReceiptOnInstallSkip(ctx context.Context, name, kind, version, buildID string) {
	kindU := strings.ToUpper(kind)
	binPath := installedBinaryPath(name, kindU)
	hash, err := cachedSha256(binPath)
	if err != nil || hash == "" {
		// Wrapper packages (bin/noop entrypoint, baseline-provided
		// binaries) may legitimately have no hashable binary at the
		// canonical path. Skip silently rather than block — the skip
		// path already returned SUCCEEDED to the caller.
		log.Printf("install-skip-restamp: %s/%s binary not hashable at %s: %v (skipping restamp)", kindU, name, binPath, err)
		return
	}
	pkg, _ := installed_state.GetInstalledPackage(ctx, srv.nodeID, kindU, name)
	if pkg == nil {
		log.Printf("install-skip-restamp: %s/%s has no existing installed_state row; nothing to restamp", kindU, name)
		return
	}
	unitPath := filepath.Join("/etc/systemd/system", "globular-"+name+".service")
	unitPathArg := ""
	if fi, statErr := os.Stat(unitPath); statErr == nil && !fi.IsDir() {
		unitPathArg = unitPath
	}
	if !stampSkipPathReceipt(pkg, unitPathArg, binPath, hash) {
		// Stamp returned an error (e.g. declared file unreadable). Do
		// NOT write a partial-stamp pkg — that would be the same kind
		// of half-baked receipt that caused the original drift.
		return
	}
	if werr := installed_state.WriteInstalledPackage(ctx, pkg); werr != nil {
		log.Printf("install-skip-restamp: write installed-state for %s/%s: %v (non-fatal)", kindU, name, werr)
		return
	}
	log.Printf("install-skip-restamp: re-stamped canonical receipt for %s/%s @ %s (legacy_sidecar superseded if present)", kindU, name, version)
}

// stampSkipPathReceipt is the pure, testable inner helper used by
// restampReceiptOnInstallSkip. It mutates pkg.Metadata with the
// canonical receipt fields produced by StampInstallReceipt, anchored
// to the unit + binary paths the caller already validated. Returns
// false when Stamp rejects the inputs (typically because a declared
// file path is unreadable); the caller MUST NOT write pkg to etcd in
// that case — a partial receipt would be the same anti-pattern as
// the legacy_sidecar drift this fix exists to clear.
//
// hash is pre-computed by the caller (the binary sha256) and stored
// as entrypoint_checksum, which is what the heartbeat's verification
// pass compares against /proc/PID/exe.
//
// Timestamps: the skip path runs because canSkipInstallPackage
// determined the package is ALREADY at the desired version with
// the same on-disk bytes — no actual apply happened. The restamp
// is metadata-only and MUST NOT advance pkg.InstalledUnix or
// pkg.UpdatedUnix. Advancing UpdatedUnix to wall clock here causes
// the verifier (max(installedUnix, updatedUnix) → ApplyTime) to
// treat every restamp as a fresh apply, then any process whose
// start time predates the restamp fires service.old_pid_after_upgrade
// — the same INC-2026-0016 class of bug that the proof writer
// already guards against. Live regression observed 2026-06-03
// on envoy + torrent after commit 72ecf067 added this helper.
//
// metadata.installed_at IS updated by StampInstallReceipt (separate
// forensic field, not consumed by the verifier's ApplyTime
// calculation), so the audit trail of when the restamp ran is
// preserved without misleading the verifier.
func stampSkipPathReceipt(pkg *node_agentpb.InstalledPackage, unitPath, binaryPath, hash string) bool {
	if pkg == nil || strings.TrimSpace(hash) == "" {
		return false
	}
	if pkg.Metadata == nil {
		pkg.Metadata = make(map[string]string)
	}
	pkg.Metadata["entrypoint_checksum"] = hash
	opts := ReceiptOpts{
		BinaryPath:  binaryPath,
		InstalledBy: "node-agent.grpc_workflow.install_skip_restamp",
	}
	if unitPath != "" {
		opts.UnitFilePath = unitPath
	}
	if err := StampInstallReceipt(pkg, opts); err != nil {
		log.Printf("install-skip-restamp: stamp rejected for %s/%s: %v (not writing partial receipt)", pkg.GetKind(), pkg.GetName(), err)
		return false
	}
	return true
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
//
// Version contract:
//   - An EXPLICIT version (e.g. "1.2.115") is honored exactly. The wildcard
//     fallback is FORBIDDEN — returning an older local archive when the caller
//     requested a specific version silently installs the wrong binary and
//     declares success. See INC-2026-0012 follow-up: v1.2.110 was installed
//     when v1.2.115 was requested because the wildcard fired on cache miss.
//   - An EMPTY ("") or dev ("0.0.0-dev") version is the only signal that the
//     caller has no authoritative version and the Day-1 bootstrap wildcard
//     fallback is permitted.
func (srv *NodeAgentServer) findLocalPackage(name, version, platform string) string {
	candidates := []string{
		fmt.Sprintf("%s_%s_%s.tgz", name, version, platform),
		fmt.Sprintf("%s_%s.tgz", name, version),
		fmt.Sprintf("%s.tgz", name),
	}
	explicitVersion := isExplicitVersion(version)
	for _, dir := range localPackageDirs {
		for _, c := range candidates {
			path := filepath.Join(dir, c)
			if _, err := os.Stat(path); err == nil {
				return path
			}
		}
		if explicitVersion {
			// Caller passed a specific version — refuse to substitute another
			// version's archive. See repository.imported_package_must_be_local_install_resolvable.
			continue
		}
		// Day-1 degraded fallback: caller has no authoritative version (empty
		// or 0.0.0-dev). Accept the newest local versioned artifact for this
		// platform so bootstrap can progress when repository resolution is
		// unavailable but a Day-0-staged package exists on disk.
		if wildcard := findLocalPackageAnyVersion(dir, name, platform); wildcard != "" {
			return wildcard
		}
	}
	return ""
}

// isExplicitVersion reports whether version is a real authoritative version
// requirement (vs an empty/dev placeholder that signals "caller has no version").
// Only "" and "0.0.0-dev" are non-explicit; anything else binds the install
// to an exact local archive match.
func isExplicitVersion(version string) bool {
	v := strings.TrimSpace(version)
	if v == "" {
		return false
	}
	if v == "0.0.0-dev" {
		return false
	}
	return true
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
	addr = strings.TrimSpace(addr)
	if addr == "" {
		return false
	}
	if config.IsLoopbackEndpoint(addr) {
		return true
	}
	host, _, err := splitHostPort(addr)
	if err != nil || host == "" {
		host = addr
	}
	if localIP != "" && strings.TrimSpace(host) == strings.TrimSpace(localIP) {
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

// readArtifactManifestEntrypoint reads the `entrypoint` field from a
// staged artifact's package.json manifest. The artifact is a .tgz
// containing a top-level package.json with at least {"entrypoint": "..."}.
// Returns empty string when the entrypoint cannot be determined; callers
// should fall back to the legacy name-inferred path.
//
// Project T: introduced so the installer can persist the manifest-declared
// binary filename and the verifier can resolve the installed binary path
// from the package's own metadata instead of inferring it from the
// package name.
func readArtifactManifestEntrypoint(artifactPath string) string {
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
		name := filepath.Clean(hdr.Name)
		if name == "package.json" || name == "./package.json" {
			data, err := io.ReadAll(io.LimitReader(tr, 32*1024))
			if err != nil {
				return ""
			}
			var manifest struct {
				Entrypoint string `json:"entrypoint"`
			}
			if err := json.Unmarshal(data, &manifest); err != nil {
				return ""
			}
			return strings.TrimSpace(manifest.Entrypoint)
		}
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
