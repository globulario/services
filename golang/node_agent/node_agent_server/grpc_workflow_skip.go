// @awareness namespace=globular.platform
// @awareness component=platform_node_agent.grpc_workflow_skip
// @awareness file_role=privileged_install_skip_decision_with_evidence_based_short_circuit
// @awareness enforces=globular.platform:invariant.destructive_actions.require_explicit_guard
// @awareness enforces=globular.platform:invariant.node_agent.install_skip_must_refresh_runtime_proof
// @awareness risk=critical
package main

// grpc_workflow_skip.go — decides whether an install workflow
// step can be skipped because the desired state is already
// present on disk. The decision MUST be evidence-based:
//
//   - the installed binary's sha256 matches manifest
//     entrypoint_checksum, OR
//   - the systemd unit is active AND the recorded
//     installed_revision matches the dispatch's target version
//
// Skipping on any weaker signal (timestamp, version string only,
// "looks installed") re-introduces the
// service.old_pid_after_upgrade class of bug where an old PID
// keeps serving while the controller thinks the upgrade
// completed. The skip path is privileged — operators rely on
// the workflow status reflecting reality, not optimistic
// inference.

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/globulario/services/golang/node_agent/node_agentpb"
	"github.com/globulario/services/golang/versionutil"
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
// DRIFT TRAP — this is a hardcoded mirror of an external source of truth.
// The authoritative source is packages/registry.yaml, projected into
// packages/metadata/<name>/specs/*_cmd.yaml (one file per command package),
// and the controller's KindCommand entries in component_catalog.go. Every
// time a new *_cmd.yaml ships, this map must be edited by hand or the
// node-agent silently treats the new package as a missing systemd unit and
// reinstalls on every reconcile.
//
// TestCommandAndSkipUnitListsMatchSpecs (installed_services_drift_test.go)
// walks packages/metadata/*/specs/ at test time and fails when this map
// drifts. That catches the next omission at CI time, but the structural fix
// is to derive this set from a shared catalog package both binaries import —
// tracked as the meta-principle code_must_not_mirror_external_enumerations.
var commandPackages = map[string]bool{
	"restic":       true,
	"rclone":       true,
	"ffmpeg":       true,
	"sctool":       true,
	"mc":           true,
	"etcdctl":      true,
	"globular-cli": true,
	"sha256sum":    true,
	"yt-dlp":       true,
	"claude":       true,
	"codex":        true,
}

var commandBinaryExistsFunc = commandBinaryExists
var commandBinaryPathFunc = commandBinaryPath
var binaryChecksumFunc = cachedSha256

// servicePolicyDirPresentFunc is the injection seam for the RBAC policy-marker
// check in canSkipInstallPackage (overridden in tests). Production uses the real
// filesystem probe.
var servicePolicyDirPresentFunc = servicePolicyDirPresent

// isCommandPackage reports whether pkgName is a binary-only (no unit) package.
func isCommandPackage(pkgName string) bool {
	return commandPackages[pkgName]
}

// packageUnit returns the systemd unit name for pkgName, or "" for command packages.
func packageUnit(pkgName string) string {
	switch pkgName {
	case "scylladb":
		return "scylla-server.service"
	case "keepalived":
		return "keepalived.service"
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
	pkgName, pkgKind, desiredVersion, desiredHash, expectedSha256, buildID string,
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
	buildIDMatches := false
	if buildID != "" && existing.GetBuildId() != buildID {
		return installSkipDeniedVersion, fmt.Sprintf(
			"install-package %s: build_id %s != desired %s",
			pkgName, existing.GetBuildId(), buildID)
	} else if buildID != "" {
		buildIDMatches = true
	}

	unit := packageUnit(pkgName)

	// Checksum check: for managed services/infra, installed_state checksum must match
	// when desired hash is provided, unless the exact build_id already matches.
	// The top-level Checksum field is a weak legacy signal: older writers used it
	// for convergence hash, newer writers use metadata.entrypoint_checksum for
	// binary proof. build_id + entrypoint proof below decides the safe skip.
	if unit != "" && !buildIDMatches && normalizedHash(desiredHash) != "" && normalizedHash(existing.GetChecksum()) != normalizedHash(desiredHash) {
		return installSkipDeniedVersion, fmt.Sprintf(
			"install-package %s: checksum %s != desired %s",
			pkgName, normalizedHash(existing.GetChecksum()), normalizedHash(desiredHash))
	}

	if runtimeManagedOutsideInstall(pkgName) && entrypointProofOptional(pkgName) && normalizedHash(expectedSha256) == "" {
		return installSkipAllowed, fmt.Sprintf(
			"install-package %s: binary-less package at %s has separately reconciled runtime, skipping",
			pkgName, desiredVersion)
	}

	// Command packages have no systemd unit — binary presence is sufficient proof.
	if unit == "" {
		if !commandBinaryExistsFunc(pkgName) {
			return installSkipDeniedUnitGone, fmt.Sprintf(
				"install-package %s: command binary missing; reinstalling", pkgName)
		}

		if normalizedHash(expectedSha256) != "" {
			path := commandBinaryPathFunc(pkgName)
			if path == "" {
				return installSkipDeniedUnitGone, fmt.Sprintf(
					"install-package %s: command binary path missing; reinstalling", pkgName)
			}
			actual, err := binaryChecksumFunc(path)
			if err != nil || normalizedHash(actual) != normalizedHash(expectedSha256) {
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
		entrypointChecksum := normalizedHash(existing.GetMetadata()["entrypoint_checksum"])
		expectedEntrypoint := normalizedHash(expectedSha256)
		switch {
		case expectedEntrypoint != "" && entrypointChecksum == "":
			return installSkipDeniedVersion, fmt.Sprintf(
				"install-package %s: %s active at %s but entrypoint_checksum missing — reapplying to register proof",
				pkgName, unit, desiredVersion)
		case expectedEntrypoint != "" && entrypointChecksum != expectedEntrypoint:
			return installSkipDeniedVersion, fmt.Sprintf(
				"install-package %s: %s active at %s but entrypoint_checksum %s != expected %s — reapplying",
				pkgName, unit, desiredVersion, entrypointChecksum, expectedEntrypoint)
		case expectedEntrypoint == "" && entrypointChecksum == "" && !entrypointProofOptional(pkgName):
			return installSkipDeniedVersion, fmt.Sprintf(
				"install-package %s: %s active at %s but entrypoint_checksum missing — reapplying to register proof",
				pkgName, unit, desiredVersion)
		}

		// Policy-presence precondition: registered RBAC permission mappings are
		// runtime proof too (invariant node_agent.install_skip_must_refresh_
		// runtime_proof). A SERVICE whose ActionPolicyDir/{name}/ marker is
		// absent was installed out-of-band by the Day-0 globular-installer —
		// install_payload (the SOLE deployer of permissions.generated.json)
		// never ran, so its authz resolver has zero mappings and denies every
		// role-based RPC (v1.2.267 empty-resolver incident: repository
		// GetRepositoryStatus / ListRepositoryFindings PermissionDenied,
		// degrading cluster-doctor). Deny the skip so the install path runs and
		// deploys policy. install_payload creates the marker unconditionally, so
		// this denies at most once per out-of-band install (no reinstall loop).
		if strings.EqualFold(pkgKind, "SERVICE") && !servicePolicyDirPresentFunc(pkgName) {
			return installSkipDeniedVersion, fmt.Sprintf(
				"install-package %s: %s active at %s but RBAC policy dir absent (installed out-of-band) — reinstalling to deploy policy",
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

func entrypointProofOptional(pkgName string) bool {
	entrypoint := strings.ToLower(strings.TrimSpace(versionutil.ReadEntrypoint(pkgName)))
	return entrypoint == "none" || entrypoint == "noop"
}

func runtimeManagedOutsideInstall(pkgName string) bool {
	switch strings.ToLower(strings.TrimSpace(pkgName)) {
	case "keepalived":
		return true
	default:
		return false
	}
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

func installedBinaryChecksumForSkip(pkg *node_agentpb.InstalledPackage) string {
	if pkg == nil {
		return ""
	}
	if md := pkg.GetMetadata(); md != nil {
		if entrypoint := strings.TrimSpace(md["entrypoint_checksum"]); entrypoint != "" {
			return entrypoint
		}
	}
	return pkg.GetChecksum()
}

func commandBinaryPath(name string) string {
	bin := strings.TrimSuffix(name, "-cmd")
	for _, dir := range []string{globularBinDir, "/usr/local/bin"} {
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

// ──────────────────────────────────────────────────────────────────────────
// Phase 27 — runtime-proof refresh for the skip path.
//
// canSkipInstallPackage decides "the on-disk binary matches the desired
// version AND the unit is active." That is necessary but NOT sufficient
// proof of convergence: the running PID may still be the OLD binary (e.g.
// the binary was swapped on disk by `globular pkg build` or by a partial
// install, but the systemd unit was never restarted to load the new
// bytes). Without that runtime-proof check, the skip path returns
// SUCCEEDED while the running service is still old, AND because the
// service self-registers its /globular/services/<id>/config record only
// on startup, the etcd config record stays at the old version too.
//
// Phase 23 surfaced this as failure_mode
// node_agent.install_skip_without_service_config_update. The invariant
// pair is node_agent.install_skip_must_refresh_runtime_proof.
//
// runtimeBinaryProvider abstracts /proc reading so tests can inject a
// fake without touching the host filesystem. Production code uses
// DiscoverRunningBinaries (process_fingerprint.go); the var-of-func
// pattern matches the existing isActive/isLoaded injection style in
// canSkipInstallPackage above.
type runtimeBinaryProvider func() map[string]RunningBinary

var runtimeBinariesFunc runtimeBinaryProvider = DiscoverRunningBinaries

// runtimeProofVerdict classifies what the runtime check learned about a
// service whose canSkipInstallPackage said "skip allowed."
type runtimeProofVerdict int

const (
	// runtimeProofMatches — the running PID's exe checksum matches the
	// expected sha256 (or no expected sha256 was supplied / no PID was
	// found to match against). Skip is safe; return SUCCEEDED.
	runtimeProofMatches runtimeProofVerdict = iota

	// runtimeProofStale — the running PID's exe checksum does NOT match
	// the expected sha256. The binary on disk is correct but the running
	// service is still the OLD binary. Skip is NOT safe — caller must
	// restart the unit (so the new binary loads + self-registers + etcd
	// config record updates) before returning success.
	runtimeProofStale

	// runtimeProofNoRunningPID — no globular-bin process was found for
	// this service name. Could mean the unit is mid-restart or the
	// service crashed. Skip is NOT safe; caller should not claim success.
	runtimeProofNoRunningPID
)

// verifyRunningBinaryMatchesExpected reads the running PID's binary
// checksum for pkgName and compares against expectedSha256. Returns
// runtimeProofMatches if expectedSha256 is empty (no opinion to enforce),
// or if the running checksum matches. Returns runtimeProofStale on
// mismatch (the actual checksum is included in the reason string for
// log diagnostics). Returns runtimeProofNoRunningPID if no globular-bin
// process was discovered for pkgName.
//
// MUST be called from the skip-allowed branch after canSkipInstallPackage
// returns installSkipAllowed, for any package that has a systemd unit
// (command packages are stateless — the on-disk binary check in
// canSkipInstallPackage already covers them).
//
// This is the runtime-proof refresh contract from
// invariant.node_agent.install_skip_must_refresh_runtime_proof. The fix
// is bounded: it does NOT force a reinstall (the on-disk binary is
// already correct); it only catches the case where the running PID is
// behind the on-disk binary.
func verifyRunningBinaryMatchesExpected(pkgName, expectedSha256 string) (runtimeProofVerdict, string) {
	exp := normalizedHash(expectedSha256)
	if exp == "" {
		// No opinion from the manifest side — nothing to verify against.
		// canSkipInstallPackage's on-disk + active-unit checks are the
		// strongest evidence available. Skip is allowed.
		return runtimeProofMatches, fmt.Sprintf(
			"runtime-proof: %s no expected_sha256 — installed-state proof is sufficient",
			pkgName)
	}

	running := runtimeBinariesFunc()
	rb, ok := running[pkgName]
	if !ok {
		return runtimeProofNoRunningPID, fmt.Sprintf(
			"runtime-proof: %s no running globular-bin PID found — cannot verify; skip not safe",
			pkgName)
	}
	got := normalizedHash(rb.Checksum)
	if got == "" {
		// Process was found but we couldn't read its checksum.
		return runtimeProofNoRunningPID, fmt.Sprintf(
			"runtime-proof: %s running PID=%d but no checksum available — cannot verify; skip not safe",
			pkgName, rb.PID)
	}
	if got != exp {
		return runtimeProofStale, fmt.Sprintf(
			"runtime-proof: %s on-disk binary matches expected but running PID=%d checksum %s != expected %s — unit restart required",
			pkgName, rb.PID, got, exp)
	}
	return runtimeProofMatches, fmt.Sprintf(
		"runtime-proof: %s running PID=%d binary checksum matches expected", pkgName, rb.PID)
}
