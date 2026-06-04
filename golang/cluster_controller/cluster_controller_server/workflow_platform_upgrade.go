// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.workflow_platform_upgrade
// @awareness file_role=per_node_per_package_upgrade_decision_logic
// @awareness implements=globular.platform:intent.workflow.source_of_operational_truth
// @awareness implements=globular.platform:intent.controller.decides_but_does_not_execute_leaf_work
// @awareness risk=high
package main

import (
	"context"
	"sort"
	"strings"

	"github.com/globulario/services/golang/versionutil"
	"github.com/globulario/services/golang/workflow/engine"
)

// workflow_platform_upgrade.go — controller-side decision logic for the
// platform.upgrade workflow. The pure decision function
// evaluateUpgradeDecisions is the authoritative implementation of the
// per-(node, package) upgrade contract:
//
//   for each (node × BOM-package):
//     if profile mismatch:                       skip (profile_skip)
//     if not currently installed on this node:   skip (not_installed)
//     if installed_version >= BOM_version:       skip (up_to_date | skip_downgrade)
//     if upgrade needed but BOM version is not
//       in the LOCAL repository:                 skip (missing_in_repo)
//     else:                                      upgrade
//
// This is the workflow-native replacement for the old direct-etcd-write
// platform-upgrade CLI which bypassed every gate above and bulk-applied
// ServiceDesiredVersion for the entire BOM.
//
// The function is deliberately pure — it takes a snapshot of nodes,
// a list of BOM packages, and a local-repository build_id resolver,
// and returns deterministic decisions. The controller wires it into the
// platform.upgrade workflow's evaluate step via
// engine.PlatformUpgradeControllerConfig.

// BOMPackage is the minimal slice of a release-index.json entry the
// per-(node, package) decision function consumes. The caller is
// responsible for fetching the BOM (release-index.json for a tag) from
// the local cache, the upstream source, or GitHub — that's not part of
// the decision contract.
type BOMPackage struct {
	Name     string
	Kind     string
	Version  string
	Profiles []string
}

// NodeView is the minimal slice of a *nodeState the decision function
// consumes. Layer 3 (installed) is the ground truth for "what is the
// running cluster actually carrying for this package on this node."
// InstalledVersions is the heartbeat-reported map produced by node-agent.
type NodeView struct {
	NodeID            string
	Profiles          []string
	InstalledVersions map[string]string
}

// LocalBuildIDResolver returns the local repository's authoritative
// build_id for a (name, version) tuple, or "" if not present locally.
// The decision function uses this to fail-closed on "BOM wants to upgrade
// but the repo doesn't actually have an installable artifact" — that
// case was the source of the v1.2.155-v1.2.159 orphan storms.
type LocalBuildIDResolver func(name, version string) string

// evaluateUpgradeDecisions is the pure per-(node, package) decision
// function. It returns the full audit (every (node, package) pair,
// classified) plus the subset that should be dispatched as upgrades.
//
// Determinism: nodes and packages are iterated in sorted order so the
// output is stable for the same input. Two runs against the same state
// produce byte-identical results, which makes the workflow's
// idempotency guarantee real (safe_retry).
func evaluateUpgradeDecisions(
	nodes []NodeView,
	bom []BOMPackage,
	resolve LocalBuildIDResolver,
) (audit []engine.UpgradeDecision, upgrades []engine.UpgradeDecision) {
	// Sort by id/name for stable output.
	sortedNodes := append([]NodeView(nil), nodes...)
	sort.Slice(sortedNodes, func(i, j int) bool { return sortedNodes[i].NodeID < sortedNodes[j].NodeID })
	sortedBOM := append([]BOMPackage(nil), bom...)
	sort.Slice(sortedBOM, func(i, j int) bool { return sortedBOM[i].Name < sortedBOM[j].Name })

	for _, node := range sortedNodes {
		nodeProfiles := profileSet(node.Profiles)
		for _, pkg := range sortedBOM {
			d := engine.UpgradeDecision{
				NodeID:      node.NodeID,
				PackageName: pkg.Name,
				PackageKind: pkg.Kind,
				BOMVersion:  pkg.Version,
			}
			if installed := strings.TrimSpace(node.InstalledVersions[pkg.Name]); installed != "" {
				d.InstalledVersion = installed
			}

			// (1) Profile match.
			if !profilesIntersect(nodeProfiles, pkg.Profiles) {
				d.Action = "profile_skip"
				d.Reason = "node profiles do not overlap with package profiles"
				audit = append(audit, d)
				continue
			}

			// (2) Installed check — respect operator removals.
			// If a package is not currently installed on this node,
			// platform-upgrade does NOT auto-install it. Operator
			// removal (or never-installed) is preserved. Day-0 install
			// is a separate workflow that DOES create initial state.
			if d.InstalledVersion == "" {
				d.Action = "not_installed"
				d.Reason = "package not installed on this node; platform.upgrade preserves operator removals (use day0/install for first-install)"
				audit = append(audit, d)
				continue
			}

			// (3) Version comparison.
			// versionutil.Compare(a, b) returns the sign of (a - b):
			//   > 0  if a > b           → BOM > installed → upgrade direction
			//   == 0 if a == b           → up_to_date
			//   < 0  if a < b            → BOM < installed → never downgrade
			cmp, err := versionutil.Compare(pkg.Version, d.InstalledVersion)
			if err != nil {
				// Non-semver versions (native: minio RELEASE.X,
				// scylladb 2025.3.8, etc.). Strings can't be ordered
				// reliably. Fall back to string equality: equal means
				// up_to_date; different means treat-as-forward (the
				// operator chose this BOM; the resolver is the final
				// gate on whether the new version is actually serveable).
				if pkg.Version == d.InstalledVersion {
					cmp = 0
				} else {
					cmp = 1
				}
			}
			switch {
			case cmp == 0:
				d.Action = "up_to_date"
				audit = append(audit, d)
				continue
			case cmp < 0:
				d.Action = "skip_downgrade"
				d.Reason = "installed version is newer than BOM; never downgrade"
				audit = append(audit, d)
				continue
			}
			// cmp > 0 → BOM is newer → upgrade direction; fall through
			// to the resolver check below.

			// (4) Resolve the local repository's build_id for the BOM
			// version. Refuse to dispatch if the local repo doesn't
			// have it — that was the v1.2.155-v1.2.159 failure mode
			// (BOM build_ids != local build_ids → orphans).
			buildID := ""
			if resolve != nil {
				buildID = strings.TrimSpace(resolve(pkg.Name, pkg.Version))
			}
			if buildID == "" {
				d.Action = "missing_in_repo"
				d.Reason = "BOM version not resolvable in local repository; refusing to dispatch upgrade (orphan-prevention)"
				audit = append(audit, d)
				continue
			}

			d.LocalBuildID = buildID
			d.Action = "upgrade"
			audit = append(audit, d)
			upgrades = append(upgrades, d)
		}
	}
	return audit, upgrades
}

// profileSet returns a normalized set of profile strings.
func profileSet(profiles []string) map[string]struct{} {
	out := make(map[string]struct{}, len(profiles))
	for _, p := range profiles {
		p = strings.TrimSpace(strings.ToLower(p))
		if p != "" {
			out[p] = struct{}{}
		}
	}
	return out
}

// profilesIntersect reports whether the package's profile list overlaps
// with the node's profile set. Used as the "should this package ever run
// on this node" gate.
func profilesIntersect(nodeProfiles map[string]struct{}, packageProfiles []string) bool {
	if len(nodeProfiles) == 0 || len(packageProfiles) == 0 {
		// Defensive: if either side is empty, treat as no-intersection
		// rather than wildcard-match. An empty package.profiles list in
		// practice means "no profile gate declared" — which we treat
		// here as "skip rather than auto-install everywhere." Day-0
		// onboarding handles the explicit initial-install case via a
		// separate workflow.
		return false
	}
	for _, p := range packageProfiles {
		k := strings.TrimSpace(strings.ToLower(p))
		if k == "" {
			continue
		}
		if _, ok := nodeProfiles[k]; ok {
			return true
		}
	}
	return false
}

// platformUpgradeControllerConfig returns the engine.PlatformUpgradeControllerConfig
// the workflow router wires up. It binds the actor handlers to this
// controller's snapshot/resolver implementations.
//
// Wiring is deliberately deferred: a follow-up commit will route this
// config into RegisterPlatformUpgradeControllerActions wherever the
// controller builds its workflow router (see engine.NewRouter()).
// This file ships the decision logic + interface; the orchestration
// wiring lands once the decision contract is locked by tests.
func (srv *server) platformUpgradeControllerConfig() engine.PlatformUpgradeControllerConfig {
	_ = context.Background // keep import in place for follow-up wiring

	return engine.PlatformUpgradeControllerConfig{
		Evaluate: func(ctx context.Context, releaseTag string) ([]engine.UpgradeDecision, []engine.UpgradeDecision, error) {
			// Follow-up: wire BOM-fetch + node snapshot + resolver here.
			// For MVP, the decision logic itself is the contract;
			// wiring is a thin glue layer that:
			//   1. fetches release-index.json for releaseTag
			//   2. snapshots nodes from srv.state.Nodes
			//   3. binds a resolver to the local repository client
			//   4. calls evaluateUpgradeDecisions
			//   5. returns the audit + upgrade subset
			return nil, nil, nil
		},
		DispatchUpgrades: func(ctx context.Context, releaseTag string, upgrades []engine.UpgradeDecision) error {
			// Follow-up: per upgrade, dispatch release.apply.package via
			// srv.RunPackageReleaseWorkflow. The infrastructure for
			// release.apply.package dispatch already exists; this just
			// loops + invokes per upgrade decision.
			return nil
		},
		Audit: func(ctx context.Context, releaseTag string, decisions []engine.UpgradeDecision) error {
			// Best-effort: write to etcd or the audit trail. Defer to
			// follow-up — the workflow runs without this in MVP.
			return nil
		},
	}
}
