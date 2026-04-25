package main

import (
	"context"
	"fmt"

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
	pkgName, pkgKind, desiredVersion, buildID string,
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

	// build_id check: only enforce when both sides are non-empty.
	if buildID != "" && existing.GetBuildId() != "" && existing.GetBuildId() != buildID {
		return installSkipDeniedVersion, fmt.Sprintf(
			"install-package %s: build_id %s != desired %s",
			pkgName, existing.GetBuildId(), buildID)
	}

	// Command packages have no systemd unit — binary presence is sufficient proof.
	unit := packageUnit(pkgName)
	if unit == "" {
		return installSkipAllowed, fmt.Sprintf(
			"install-package %s: command package at %s, skipping", pkgName, desiredVersion)
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
