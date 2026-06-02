package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
)

// installSkipResult is the decision returned by canSkipInstallPackage.
type installSkipResult int

const (
	// installSkipAllowed — package is at the desired version and runtime is active (or binary-only).
	// Safe to return SUCCEEDED without reinstalling.
	installSkipAllowed installSkipResult = iota

	// installSkipDeniedNoRecord — no installed-state record found for the exact kind.
	// Must install.
	installSkipDeniedNoRecord

	// installSkipDeniedVersion — installed version or build_id does not match desired.
	// Must install.
	installSkipDeniedVersion

	// installSkipDeniedInactive — version matches but the systemd unit is loaded yet inactive.
	// Caller should try supervisor.Start before falling back to full reinstall.
	installSkipDeniedInactive

	// installSkipDeniedUnitGone — version matches but the systemd unit file is missing entirely.
	// Must reinstall.
	installSkipDeniedUnitGone
)

// commandPackages is the set of binary-only packages that have no systemd unit.
// Skip is safe for these when version (and optional build_id) match because
// there is no runtime state to check.
//
// Must stay in sync with KindCommand entries in component_catalog.go (controller).
// The node-agent cannot import the controller's catalog, so this list is maintained
// manually. Add entries here whenever a new COMMAND-kind package is added to the catalog.
var commandPackages = map[string]bool{
	"restic":      true,
	"rclone":      true,
	"ffmpeg":      true,
	"sctool":      true,
	"mc":          true,
	"etcdctl":     true,
	"globular-cli": true,
	"cli":          true,
	"sha256sum":    true,
	"yt-dlp":       true,
	"claude":       true,
}

var commandBinaryExistsFunc = commandBinaryExists
var commandBinaryPathFunc = commandBinaryPath
var binaryChecksumFunc = cachedSha256

// isCommandPackage reports whether pkgName is a binary-only (no unit) package.
func isCommandPackage(pkgName string) bool {
	return commandPackages[pkgName]
}

// packageUnit returns the systemd unit name for pkgName, or "" for command packages.
func packageUnit(pkgName string) string {
	switch pkgName {
	case "scylladb":
		return "scylla-server.service"
	case "scylla-manager":
		return "globular-scylla-manager.service"
	case "scylla-manager-agent":
		return "globular-scylla-manager-agent.service"
	}
	if isCommandPackage(pkgName) {
		return ""
	}
	return "globular-" + pkgName + ".service"
}

// canSkipInstallPackage decides whether an install-package request can be safely
// short-circuited.
//
// Inputs:
//   - pkgName, pkgKind  — from the install request
//   - desiredVersion, buildID — target version; buildID may be ""
//   - existing — installed-state record retrieved with the EXACT requested kind
//     (nil if not found)
//   - isActive, isLoaded — injectable systemctl wrappers (unit, error)
//
// Returns (result, reason) where reason is a log-friendly string.
//
// Cross-kind lookup is explicitly NOT performed here: the caller must
// query with the exact pkgKind and pass what it gets.
func canSkipInstallPackage(
	ctx context.Context,
	pkgName, pkgKind, desiredVersion, desiredHash, buildID string,
	existing *node_agentpb.InstalledPackage,
	isActive func(context.Context, string) (bool, error),
	isLoaded func(context.Context, string) (bool, error),
) (installSkipResult, string) {
	if existing == nil {
		return installSkipDeniedNoRecord, fmt.Sprintf(
			"install-package %s: no installed-state record for kind %s", pkgName, pkgKind)
	}

	if existing.GetVersion() != desiredVersion {
		return installSkipDeniedVersion, fmt.Sprintf(
			"install-package %s: installed %s != desired %s",
			pkgName, existing.GetVersion(), desiredVersion)
	}

	// build_id check: when requested, exact match is required.
	if buildID != "" && existing.GetBuildId() != buildID {
		return installSkipDeniedVersion, fmt.Sprintf(
			"install-package %s: build_id %s != desired %s",
			pkgName, existing.GetBuildId(), buildID)
	}

	unit := packageUnit(pkgName)

	// Checksum check: for managed services/infra, installed_state checksum must match
	// when desired hash is provided. Command packages can prove checksum directly
	// from the local binary below.
	if unit != "" && normalizedHash(desiredHash) != "" && normalizedHash(existing.GetChecksum()) != normalizedHash(desiredHash) {
		return installSkipDeniedVersion, fmt.Sprintf(
			"install-package %s: checksum %s != desired %s",
			pkgName, normalizedHash(existing.GetChecksum()), normalizedHash(desiredHash))
	}

	// Command packages have no systemd unit — binary presence is sufficient proof.
	if unit == "" {
		if !commandBinaryExistsFunc(pkgName) {
			return installSkipDeniedUnitGone, fmt.Sprintf(
				"install-package %s: command binary missing; reinstalling", pkgName)
		}

		if normalizedHash(desiredHash) != "" {
			path := commandBinaryPathFunc(pkgName)
			if path == "" {
				return installSkipDeniedUnitGone, fmt.Sprintf(
					"install-package %s: command binary path missing; reinstalling", pkgName)
			}
			actual, err := binaryChecksumFunc(path)
			if err != nil || normalizedHash(actual) != normalizedHash(desiredHash) {
				return installSkipDeniedVersion, fmt.Sprintf(
					"install-package %s: command binary checksum mismatch; reinstalling", pkgName)
			}
		}

		return installSkipAllowed, fmt.Sprintf(
			"install-package %s: command package at %s converged, skipping", pkgName, desiredVersion)
	}

	// Runtime proof: unit must be active.
	active, err := isActive(ctx, unit)
	if err == nil && active {
		// Also require entrypoint_checksum to be present. Without it the runtime
		// verifier produces no verdict (UNVERIFIED finding) and the heartbeat
		// reports hash_drift. Force a re-apply so ApplyPackageRelease writes the
		// checksum — the reconciler must not skip a package that is running but
		// unverified, even when version and build_id match.
		if existing.GetMetadata()["entrypoint_checksum"] == "" {
			return installSkipDeniedVersion, fmt.Sprintf(
				"install-package %s: %s active at %s but entrypoint_checksum missing — reapplying to register proof",
				pkgName, unit, desiredVersion)
		}
		return installSkipAllowed, fmt.Sprintf(
			"install-package %s: %s active at %s, skipping", pkgName, unit, desiredVersion)
	}

	// Unit exists but is not active — check whether the unit file is loaded.
	loaded, _ := isLoaded(ctx, unit)
	if loaded {
		return installSkipDeniedInactive, fmt.Sprintf(
			"install-package %s: installed_state matches but %s is inactive; attempting repair",
			pkgName, unit)
	}

	return installSkipDeniedUnitGone, fmt.Sprintf(
		"install-package %s: installed_state matches but %s is missing; reinstalling",
		pkgName, unit)
}

func normalizedHash(hash string) string {
	h := strings.ToLower(strings.TrimSpace(hash))
	h = strings.TrimPrefix(h, "sha256:")
	return h
}

// buildIDSkipChecksumOK reports whether the build_id idempotency guard may
// skip reinstall given the installed binary checksum and the expected binary
// checksum from the manifest. Returns false (must reinstall) when both hashes
// are present and they differ — this indicates the binary was replaced
// out-of-band (e.g. globular deploy or a local build) with a different binary
// that happens to carry the same version/build_id.
// Returns true (skip allowed) when either hash is absent (no opinion) or when
// the hashes agree.
func buildIDSkipChecksumOK(installedChecksum, expectedSha256 string) bool {
	exp := normalizedHash(expectedSha256)
	got := normalizedHash(installedChecksum)
	if exp == "" || got == "" {
		return true // no opinion from one side — allow skip
	}
	return exp == got
}

func commandBinaryPath(name string) string {
	bin := strings.TrimSuffix(name, "-cmd")
	for _, dir := range []string{"/usr/local/bin", "/usr/lib/globular/bin"} {
		path := filepath.Join(dir, bin)
		if st, err := os.Stat(path); err == nil && !st.IsDir() {
			return path
		}
	}
	if p, err := exec.LookPath(bin); err == nil {
		return p
	}
	return ""
}
