// @awareness namespace=globular.platform
// @awareness component=cluster_controller.convergence_classifier
// @awareness file_role=per_node_per_package_convergence_classification
// @awareness implements=globular.platform:intent.local_success_not_global_convergence
// @awareness implements=globular.platform:intent.service.installation_is_not_runtime_truth
// @awareness implements=globular.platform:intent.state.repository_desired_installed_runtime
// @awareness risk=critical
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

const (
	// During bootstrap/rejoin we require fresher runtime observations.
	runtimeFreshnessBootstrap = 60 * time.Second
	// During steady-state we tolerate older heartbeats.
	runtimeFreshnessSteady = 3 * time.Minute
	// Per node/package/version runtime-repair cooldown.
	runtimeRepairCooldown = 45 * time.Second
)

type RuntimeConvergence string

const (
	RuntimeConverged RuntimeConvergence = "converged"
	RuntimeInactive  RuntimeConvergence = "inactive"
	RuntimeFailed    RuntimeConvergence = "failed"
	RuntimeMissing   RuntimeConvergence = "missing"
	RuntimeUnknown   RuntimeConvergence = "unknown"
	RuntimeStale     RuntimeConvergence = "stale"
	RuntimeNotNeeded RuntimeConvergence = "not_needed"
)

type PackageConvergence struct {
	VersionOK      bool
	HashOK         bool
	BuildIDOK      bool
	RuntimeNeeded  bool
	RuntimeOK      bool
	RuntimeState   RuntimeConvergence
	RepairRequired bool
	Reason         string
}

var runtimeRepairCooldownByTarget sync.Map // key -> time.Time

func runtimeProofRequiredForKind(pkgKind, pkgName string) bool {
	kind := strings.ToUpper(strings.TrimSpace(pkgKind))
	if kind == "COMMAND" {
		return false
	}
	if skipRuntimeCheck(pkgName) {
		return false
	}
	return kind == "SERVICE" || kind == "INFRASTRUCTURE" || kind == "APPLICATION"
}

func runtimeUnitForPackage(pkgName, pkgKind string) string {
	name := strings.TrimSpace(pkgName)
	kind := strings.ToUpper(strings.TrimSpace(pkgKind))
	if kind == "SERVICE" || kind == "APPLICATION" {
		name = canonicalServiceName(name)
	}
	if name == "" {
		return ""
	}
	return packageToUnit(name)
}

func runtimeFreshnessThreshold(node *nodeState) time.Duration {
	if node == nil {
		return runtimeFreshnessSteady
	}
	switch node.BootstrapPhase {
	case BootstrapAdmitted, BootstrapInfraPreparing, BootstrapEtcdJoining, BootstrapEtcdReady, BootstrapXdsReady, BootstrapEnvoyReady, BootstrapStorageJoining:
		return runtimeFreshnessBootstrap
	default:
		return runtimeFreshnessSteady
	}
}

func runtimeStatusFresh(node *nodeState, now time.Time) (bool, string) {
	if node == nil {
		return false, "runtime status unknown (node state unavailable)"
	}
	if node.LastSeen.IsZero() {
		return false, "runtime status unknown (no heartbeat)"
	}
	age := now.Sub(node.LastSeen)
	thr := runtimeFreshnessThreshold(node)
	if age > thr {
		return false, fmt.Sprintf("runtime status stale (%s > %s)", age.Round(time.Second), thr)
	}
	return true, ""
}

// classifyPackageConvergence compares the controller's desired state against
// the node-agent's installed state and emits a convergence verdict.
// The desiredHash here must come from lookupServiceReleaseBuildID — never
// from a locally recomputed value. Hash schema parity is critical.
//
// Phase 38 (THE root-cause fix): the function now ALSO verifies the
// `desiredEntrypointChecksum` parameter against
// installed.Metadata["entrypoint_checksum"]. Pre-Phase-38, every gate that
// called this function (release-workflow skip-node, drift-reconciler via a
// separate but parallel check, release-pipeline, bootstrap, etc.) could
// declare a package "converged" on (version, hash, buildId, runtime) match
// without ever verifying the binary actually on disk. That hole is exactly
// how a node-agent install can claim convergence with the OLD binary still
// on disk: the convergence-committer writes the new buildId from a
// ConvergenceResultV1 message, the systemd unit is "active" with the old
// bytes still loaded, every classifyPackageConvergence check passes, and
// the system silently stays at the wrong binary forever.
//
// Contract for the new check:
//   - both empty → no opinion (legacy artifact without recorded proof;
//     the verifier surfaces this via runtime_identity_unproven)
//   - desired empty / installed present → no opinion (cannot compare)
//   - desired present / installed empty → no opinion (same)
//   - both present + equal (sha256:-prefix-stripping, case-insensitive) → OK
//   - both present + differ → RepairRequired, reason
//     "installed entrypoint_checksum X != desired Y"
//
//globular:enforces infra.desired_hash_consistency
//globular:enforces convergence.requires_entrypoint_checksum_match
//globular:expects_hash_schema infra_desired_hash
//globular:expects_hash_schema service_desired_hash
//globular:state_transition DESIRED -> INSTALLED
//globular:risk convergence.classification_error
func classifyPackageConvergence(
	node *nodeState,
	pkgName, pkgKind string,
	desiredVersion, desiredHash, desiredBuildID, desiredEntrypointChecksum string,
	installed *node_agentpb.InstalledPackage,
	now time.Time,
) PackageConvergence {
	pc := PackageConvergence{
		RuntimeNeeded: runtimeProofRequiredForKind(pkgKind, pkgName),
		RuntimeState:  RuntimeUnknown,
	}

	if installed == nil {
		pc.RepairRequired = true
		pc.Reason = "not installed"
		if !pc.RuntimeNeeded {
			pc.RuntimeState = RuntimeNotNeeded
		}
		return pc
	}

	gotVersion := strings.TrimSpace(installed.GetVersion())
	wantVersion := strings.TrimSpace(desiredVersion)
	if wantVersion == "" || gotVersion == wantVersion {
		pc.VersionOK = true
	} else {
		pc.RepairRequired = true
		pc.Reason = fmt.Sprintf("installed version %s != desired %s", gotVersion, wantVersion)
		return pc
	}

	gotHash := normalizeDesiredHash(installed.GetChecksum())
	wantHash := normalizeDesiredHash(desiredHash)
	if wantHash == "" || gotHash == wantHash {
		pc.HashOK = true
	} else {
		pc.RepairRequired = true
		pc.Reason = fmt.Sprintf("installed checksum %s != desired %s", gotHash, wantHash)
		return pc
	}

	gotBuild := strings.TrimSpace(installed.GetBuildId())
	wantBuild := strings.TrimSpace(desiredBuildID)
	if wantBuild == "" || gotBuild == wantBuild {
		pc.BuildIDOK = true
	} else {
		pc.RepairRequired = true
		pc.Reason = fmt.Sprintf("installed build_id %s != desired %s", gotBuild, wantBuild)
		return pc
	}

	// Phase 38 — entrypoint_checksum hard binary proof.
	// Comes after version/hash/buildId because those pass first in normal
	// healthy convergence; the entrypoint mismatch only matters when
	// upstream gates say "converged" but the binary on disk is wrong.
	gotEntry := normalizeEntrypointChecksum(installed.GetMetadata()["entrypoint_checksum"])
	wantEntry := normalizeEntrypointChecksum(desiredEntrypointChecksum)
	if wantEntry != "" && gotEntry != "" && wantEntry != gotEntry {
		pc.RepairRequired = true
		pc.Reason = fmt.Sprintf("installed entrypoint_checksum %s != desired %s (buildId matched but on-disk binary is wrong)",
			shortEntrypoint(gotEntry), shortEntrypoint(wantEntry))
		return pc
	}

	if !pc.RuntimeNeeded {
		pc.RuntimeOK = true
		pc.RuntimeState = RuntimeNotNeeded
		pc.Reason = "runtime not needed"
		return pc
	}

	fresh, freshnessReason := runtimeStatusFresh(node, now)
	if !fresh {
		pc.RepairRequired = true
		pc.RuntimeState = RuntimeStale
		pc.Reason = freshnessReason
		return pc
	}

	unit := runtimeUnitForPackage(pkgName, pkgKind)
	if unit == "" {
		pc.RepairRequired = true
		pc.RuntimeState = RuntimeUnknown
		pc.Reason = "runtime unit unknown"
		return pc
	}

	for _, u := range node.Units {
		if !strings.EqualFold(strings.TrimSpace(u.Name), unit) {
			continue
		}
		state := strings.ToLower(strings.TrimSpace(u.State))
		switch state {
		case "active":
			pc.RuntimeOK = true
			pc.RuntimeState = RuntimeConverged
			pc.Reason = fmt.Sprintf("%s active", unit)
			return pc
		case "failed":
			pc.RuntimeState = RuntimeFailed
		case "inactive":
			pc.RuntimeState = RuntimeInactive
		case "":
			pc.RuntimeState = RuntimeUnknown
		default:
			pc.RuntimeState = RuntimeUnknown
			log.Printf("classifyPackageConvergence: unexpected systemd state=%q for unit=%s on node=%s",
				state, unit, node.NodeID)
		}
		pc.RepairRequired = true
		pc.Reason = fmt.Sprintf("%s state=%s", unit, state)
		return pc
	}

	pc.RepairRequired = true
	pc.RuntimeState = RuntimeMissing
	pc.Reason = fmt.Sprintf("%s missing", unit)
	return pc
}

func packageRuntimeHealthyOnNode(node *nodeState, pkgName, pkgKind string) (bool, string) {
	// Build an artificial installed record so classifyPackageConvergence performs
	// runtime-only checks without version/hash/build/entrypoint gates.
	pc := classifyPackageConvergence(
		node,
		pkgName,
		pkgKind,
		"",
		"",
		"",
		"", // Phase 38: no entrypoint check for runtime-only health probe
		&node_agentpb.InstalledPackage{Version: "runtime-check"},
		time.Now(),
	)
	return pc.RuntimeOK, pc.Reason
}

// normalizeEntrypointChecksum trims whitespace, strips an optional sha256:
// prefix, and lowercases — so equality is robust to operator-visible
// formatting variance from manifests / package.json / node-agent reports.
func normalizeEntrypointChecksum(s string) string {
	v := strings.ToLower(strings.TrimSpace(s))
	return strings.TrimPrefix(v, "sha256:")
}

// shortEntrypoint returns the first 16 hex chars of a normalized checksum
// for log-friendly display. Sha256 collision probability at 8+ hex chars is
// negligible for our key/log identification purposes.
func shortEntrypoint(s string) string {
	v := normalizeEntrypointChecksum(s)
	if len(v) > 16 {
		return v[:16]
	}
	return v
}

func runtimeRepairCooldownKey(nodeID, pkgName, pkgKind, desiredVersion, desiredHash, desiredBuildID string) string {
	return strings.ToLower(strings.TrimSpace(nodeID) + "|" + strings.TrimSpace(pkgKind) + "|" + strings.TrimSpace(pkgName) +
		"|" + strings.TrimSpace(desiredVersion) + "|" + normalizeDesiredHash(desiredHash) + "|" + strings.TrimSpace(desiredBuildID))
}

func shouldDispatchRuntimeRepair(key string, now time.Time) (bool, time.Duration) {
	if v, ok := runtimeRepairCooldownByTarget.Load(key); ok {
		last := v.(time.Time)
		if elapsed := now.Sub(last); elapsed < runtimeRepairCooldown {
			return false, runtimeRepairCooldown - elapsed
		}
	}
	runtimeRepairCooldownByTarget.Store(key, now)
	return true, 0
}

func normalizeDesiredHash(hash string) string {
	h := strings.ToLower(strings.TrimSpace(hash))
	h = strings.TrimPrefix(h, "sha256:")
	return h
}

// lookupResolvedEntrypointChecksum returns the resolved entrypoint
// checksum (BINARY sha256, NOT tarball digest) for the given package by
// reading the InfrastructureRelease/ServiceRelease/ApplicationRelease
// resource status. Returns "" when the resource is not found or has not
// been resolved yet — callers MUST treat empty as "no opinion" rather
// than "matches".
//
// Phase 38 — the controller-side proof of binary identity. This is
// what classifyPackageConvergence compares against installed_state's
// metadata["entrypoint_checksum"] to detect a lying installed_state
// (buildId matches but the binary on disk was never swapped).
func (srv *server) lookupResolvedEntrypointChecksum(ctx context.Context, publisherID, pkgName, installedKind string) string {
	if srv.resources == nil {
		return ""
	}
	kind := strings.ToUpper(strings.TrimSpace(installedKind))
	// Try the resource type that matches the installed kind first; fall
	// back across the other release types so node-agent's split-kind
	// records (both SERVICE and INFRASTRUCTURE registrations exist for
	// the same package) still resolve to the correct binary identity.
	// All seven proto ArtifactKind values are handled explicitly. Only
	// three Release resource types exist (ServiceRelease,
	// InfrastructureRelease, ApplicationRelease) — the kinds without a
	// dedicated release type map to the most semantically-similar one
	// FIRST, then fall back to the others.
	//
	// TestLookupResolvedEntrypointChecksumKindsExhaustive enforces that
	// every proto ArtifactKind has an explicit case here, satisfying
	// invariant:release_type_switch_must_have_default.
	candidates := []string{}
	switch kind {
	case "INFRASTRUCTURE", "SUBSYSTEM":
		// SUBSYSTEM cohabits the infrastructure layer; look at infra first.
		candidates = []string{"InfrastructureRelease", "ServiceRelease", "ApplicationRelease"}
	case "SERVICE", "AGENT":
		// AGENT is a single-node service; same resource type as SERVICE.
		candidates = []string{"ServiceRelease", "InfrastructureRelease", "ApplicationRelease"}
	case "APPLICATION":
		candidates = []string{"ApplicationRelease", "ServiceRelease", "InfrastructureRelease"}
	case "COMMAND", "AWARENESS_BUNDLE":
		// COMMAND and AWARENESS_BUNDLE have no installed daemon — no
		// runtime entrypoint to verify — but a release record may still
		// exist for audit. Check service first as the most common shape.
		candidates = []string{"ServiceRelease", "InfrastructureRelease", "ApplicationRelease"}
	default:
		log.Printf("lookupResolvedEntrypointChecksum: unknown ArtifactKind=%q for package=%s — proto added a kind without updating release_runtime_convergence.go (falling back to all release types)",
			kind, pkgName)
		candidates = []string{"ServiceRelease", "InfrastructureRelease", "ApplicationRelease"}
	}
	for _, rt := range candidates {
		items, _, err := srv.resources.List(ctx, rt, "")
		if err != nil {
			continue
		}
		for _, obj := range items {
			switch v := obj.(type) {
			case *cluster_controllerpb.ServiceRelease:
				if v != nil && v.Spec != nil && v.Status != nil {
					canon := canonicalServiceName(v.Spec.ServiceName)
					if (canon == pkgName || v.Spec.ServiceName == pkgName) && v.Spec.PublisherID == publisherID {
						if s := strings.TrimSpace(v.Status.ResolvedEntrypointChecksum); s != "" {
							return s
						}
					}
				}
			case *cluster_controllerpb.InfrastructureRelease:
				if v != nil && v.Spec != nil && v.Status != nil {
					if v.Spec.Component == pkgName && v.Spec.PublisherID == publisherID {
						if s := strings.TrimSpace(v.Status.ResolvedEntrypointChecksum); s != "" {
							return s
						}
					}
				}
			case *cluster_controllerpb.ApplicationRelease:
				if v != nil && v.Spec != nil && v.Status != nil {
					if v.Spec.AppName == pkgName && v.Spec.PublisherID == publisherID {
						if s := strings.TrimSpace(v.Status.ResolvedEntrypointChecksum); s != "" {
							return s
						}
					}
				}
			default:
				// Per meta.silence_is_not_valid_for_unexpected: silent
				// skip of an unknown release type would mean a future
				// type (e.g. ComputeRelease) silently produces "" for
				// resolved checksum, which the caller can't distinguish
				// from "not pinned yet." Log so the type's first
				// encounter is loud — the resources List filter is
				// already pre-narrowed by `rt` so this should not fire
				// in steady state.
				log.Printf("lookupResolvedEntrypointChecksum: unknown release item type %T for rt=%s package=%s — checksum lookup will return \"\" if no other release type matches",
					v, rt, pkgName)
			}
		}
	}
	return ""
}
