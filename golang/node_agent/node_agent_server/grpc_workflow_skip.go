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
var commandPackages = map[string]bool{
	"restic":      true,
	"rclone":      true,
	"ffmpeg":      true,
	"sctool":      true,
	"mc":          true,
	"etcdctl":     true,
	"globular-cli": true,
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
