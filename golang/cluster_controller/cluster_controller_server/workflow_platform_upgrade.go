// @awareness namespace=globular.platform
// @awareness component=platform_cluster_controller.workflow_platform_upgrade
// @awareness file_role=per_node_per_package_upgrade_decision_logic
// @awareness implements=globular.platform:intent.workflow.source_of_operational_truth
// @awareness implements=globular.platform:intent.controller.decides_but_does_not_execute_leaf_work
// @awareness risk=high
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	cluster_controllerpb "github.com/globulario/services/golang/cluster_controller/cluster_controllerpb"
	"github.com/globulario/services/golang/config"
	"github.com/globulario/services/golang/repository/repository_client"
	repositorypb "github.com/globulario/services/golang/repository/repositorypb"
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

// RunPlatformUpgradeWorkflow dispatches the platform.upgrade workflow
// via the centralized WorkflowService. This is the controller-side entry
// point for callers that want to trigger an upgrade run from server code
// (the CLI goes through the workflow service directly — see
// globularcli/upgrade_cmds.go — and reaches the same actor handlers via
// the default router registered in reconcile_runtime.go).
func (srv *server) RunPlatformUpgradeWorkflow(ctx context.Context, releaseTag string, dryRun bool) (map[string]any, error) {
	if releaseTag == "" {
		return nil, fmt.Errorf("release_tag required")
	}

	router := engine.NewRouter()
	engine.RegisterPlatformUpgradeControllerActions(router, srv.platformUpgradeControllerConfig())

	inputs := map[string]any{
		"cluster_id":  srv.cfg.ClusterDomain,
		"release_tag": releaseTag,
		"dry_run":     dryRun,
	}
	corrID := fmt.Sprintf("platform-upgrade-%s-%d", releaseTag, time.Now().Unix())

	resp, err := srv.executeWorkflowCentralized(ctx, "platform.upgrade", corrID, inputs, router)
	if err != nil {
		return nil, err
	}
	if resp.Status == "FAILED" {
		return nil, fmt.Errorf("platform.upgrade workflow failed: %s", resp.Error)
	}
	return map[string]any{
		"status":     resp.Status,
		"run_id":     resp.RunId,
		"release_tag": releaseTag,
		"dry_run":    dryRun,
	}, nil
}

// platformUpgradeControllerConfig returns the engine.PlatformUpgradeControllerConfig
// the workflow router wires up. It binds the actor handlers to this
// controller's snapshot/resolver implementations.
//
// Source of truth for the BOM is the LOCAL repository's PUBLISHED
// artifacts — not a separate release-index.json fetched from upstream.
// Rationale: Dave's framing on 2026-06-04 — "when you upgrade you got a
// bunch of service from upstream (.tar.gz) so you simply need to publish
// received service it's the repository responsibility to discard existing
// package... and then see if the service must be install (it's the
// cluster controller responsibility)". The repository is authoritative
// for what's actually installable; that's the same set the workflow
// must reason about.
func (srv *server) platformUpgradeControllerConfig() engine.PlatformUpgradeControllerConfig {
	return engine.PlatformUpgradeControllerConfig{
		Evaluate:         srv.platformUpgradeEvaluate,
		DispatchUpgrades: srv.platformUpgradeDispatch,
		Audit:            srv.platformUpgradeAudit,
	}
}

// platformUpgradeEvaluate fetches the local repository's PUBLISHED
// artifacts (the de-facto BOM), snapshots node state, and computes
// per-(node, package) decisions via the pure evaluateUpgradeDecisions
// function.
func (srv *server) platformUpgradeEvaluate(ctx context.Context, releaseTag string) ([]engine.UpgradeDecision, []engine.UpgradeDecision, error) {
	bom, resolver, err := srv.fetchLocalRepositoryBOM(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("fetch local BOM: %w", err)
	}

	nodes := srv.snapshotNodesForUpgrade()
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom, resolver)

	log.Printf("platform_upgrade.evaluate: tag=%s bom_packages=%d nodes=%d decisions=%d upgrades=%d",
		releaseTag, len(bom), len(nodes), len(audit), len(upgrades))
	return audit, upgrades, nil
}

// platformUpgradeDispatch upserts the canonical desired-state record for
// each gated upgrade decision so the release reconciler picks up the drift
// and runs release.apply.package using the canonical
// ServiceRelease/<publisher>/<name> ID.
//
// Earlier versions of this handler called RunPackageReleaseWorkflow directly
// with a synthetic releaseID ("platform-upgrade-<tag>-<ts>-<kind>-<name>").
// The first step of release.apply.package — controller.release.mark_resolved
// — tries to load the ServiceRelease record by that releaseName; the
// synthetic name doesn't match any existing record so the lookup fails with
// "ServiceRelease ... not found" and EVERY per-package workflow died at
// step 1. platform-upgrade reported RUN_STATUS_SUCCEEDED, but the audit log
// listed 21+ dispatched packages and zero actually upgraded.
//
// Why upsertOne is not a "bypass" of workflow authority:
//   - platform_upgrade.evaluate has already gated each (node, package)
//     decision against profile match, installed-state, semver, and repo
//     resolvability. Only the gated decisions reach dispatch.
//   - upsertOne IS the canonical typed RPC that "globular services desired
//     set" calls — it goes through the full path: ServiceDesiredVersion +
//     bridge to ServiceRelease / InfrastructureRelease, audit log, version
//     regression guard, observability blackout refusal.
//   - The reconciler watches ServiceDesiredVersion for drift and dispatches
//     release.apply.package with the canonical releaseID
//     "<ResourceType>/<publisher>/<name>" that mark_resolved expects.
//
// failure_mode.controller.platform_upgrade_bypassed_workflow_authority — the
// older direct-etcd-write CLI that prompted the warning bypassed the
// controller entirely and bulk-applied for the entire BOM without gates.
// This handler is the opposite shape: gated decisions, going through the
// controller's typed API.
func (srv *server) platformUpgradeDispatch(ctx context.Context, releaseTag string, upgrades []engine.UpgradeDecision) error {
	// Group by (package_name, package_kind, bom_version, build_id). One
	// desired-state record per package; the candidate nodes set is implicit
	// from profiles + heartbeat at reconcile time.
	type key struct {
		name, kind, version, buildID string
	}
	groups := map[key][]string{}
	for _, u := range upgrades {
		k := key{u.PackageName, u.PackageKind, u.BOMVersion, u.LocalBuildID}
		groups[k] = append(groups[k], u.NodeID)
	}

	var firstErr error
	dispatched := 0
	for k, nodes := range groups {
		// Resolve the artifact manifest from the repository to obtain
		// the build_number. upsertOne stores it on ServiceDesiredVersion so
		// the reconciler dispatches with the right build identity (Phase 2).
		manifest, mErr := srv.resolveArtifactForUpgrade(ctx, k.name, k.version, k.buildID)
		if mErr != nil {
			log.Printf("platform_upgrade.dispatch: skip %s@%s — manifest resolve failed: %v",
				k.name, k.version, mErr)
			if firstErr == nil {
				firstErr = mErr
			}
			continue
		}

		desired := &cluster_controllerpb.DesiredService{
			ServiceId:   k.name,
			Version:     k.version,
			BuildNumber: manifest.GetBuildNumber(),
			BuildId:     k.buildID,
		}
		// allowRegression=false: a platform upgrade must not silently regress a
		// version — automatic rollback is forbidden (deployment.automatic_rollback_is_forbidden).
		if err := srv.upsertOne(ctx, desired, false); err != nil {
			log.Printf("platform_upgrade.dispatch: upsertOne FAILED for %s@%s: %v",
				k.name, k.version, err)
			if firstErr == nil {
				firstErr = err
			}
			continue
		}
		dispatched++
		log.Printf("platform_upgrade.dispatch: upserted desired %s@%s build_id=%s nodes=%d",
			k.name, k.version, k.buildID, len(nodes))
	}

	log.Printf("platform_upgrade.dispatch: tag=%s groups=%d dispatched=%d",
		releaseTag, len(groups), dispatched)
	if firstErr != nil {
		return fmt.Errorf("platform_upgrade.dispatch had failures (first error): %w", firstErr)
	}
	return nil
}

// platformUpgradeAudit best-effort writes the per-(node, package)
// decisions to etcd under /globular/platform_upgrade/runs/<tag>/<ts>.
func (srv *server) platformUpgradeAudit(ctx context.Context, releaseTag string, decisions []engine.UpgradeDecision) error {
	if srv.etcdClient == nil {
		return nil
	}
	ts := time.Now().UTC().Format(time.RFC3339Nano)
	key := fmt.Sprintf("/globular/platform_upgrade/runs/%s/%s", releaseTag, ts)
	body, err := json.Marshal(map[string]any{
		"release_tag":   releaseTag,
		"recorded_at":   ts,
		"decisions":     decisions,
		"decision_count": len(decisions),
	})
	if err != nil {
		return fmt.Errorf("marshal audit: %w", err)
	}
	if _, err := srv.etcdClient.Put(ctx, key, string(body)); err != nil {
		// Audit is best-effort; log and continue.
		log.Printf("platform_upgrade.audit: etcd write failed: %v", err)
		return nil
	}
	return nil
}

// snapshotNodesForUpgrade copies node profiles + installed_versions
// under the state lock, so the decision function operates on a stable
// view.
func (srv *server) snapshotNodesForUpgrade() []NodeView {
	srv.lock("platform_upgrade:snapshot")
	defer srv.unlock()

	out := make([]NodeView, 0, len(srv.state.Nodes))
	for _, n := range srv.state.Nodes {
		if n == nil {
			continue
		}
		installed := make(map[string]string, len(n.InstalledVersions))
		for k, v := range n.InstalledVersions {
			installed[k] = v
		}
		profiles := append([]string(nil), n.Profiles...)
		out = append(out, NodeView{
			NodeID:            n.NodeID,
			Profiles:          profiles,
			InstalledVersions: installed,
		})
	}
	return out
}

// fetchLocalRepositoryBOM lists PUBLISHED artifacts from the local
// repository, picks the latest build_number per (publisher, name,
// platform, version), and returns both the BOM and a resolver bound to
// the same data.
func (srv *server) fetchLocalRepositoryBOM(ctx context.Context) ([]BOMPackage, LocalBuildIDResolver, error) {
	repoAddr := config.ResolveLocalServiceAddr("repository.PackageRepository")
	if repoAddr == "" {
		return nil, nil, fmt.Errorf("repository service not found in registry")
	}
	rc, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return nil, nil, fmt.Errorf("repository client: %w", err)
	}
	defer rc.Close()

	manifests, err := rc.ListArtifacts()
	if err != nil {
		return nil, nil, fmt.Errorf("ListArtifacts: %w", err)
	}

	// Group by (name, version) — pick the highest build_number for each
	// (PUBLISHED only, never YANKED/QUARANTINED/REVOKED). This is the
	// same selection rule release_resolver.getLatestPublished uses for
	// per-version pinning, kept in sync intentionally.
	type pkgKey struct{ name, version string }
	type pkgEntry struct {
		kind        string
		profiles    []string
		buildID     string
		buildNumber int64
	}
	picked := map[pkgKey]pkgEntry{}
	for _, m := range manifests {
		if m == nil || m.GetRef() == nil {
			continue
		}
		ps := m.GetPublishState()
		if repositorypb.IsDownloadBlocked(ps) {
			continue
		}
		if ps != repositorypb.PublishState_PUBLISHED && ps != repositorypb.PublishState_PUBLISH_STATE_UNSPECIFIED {
			continue
		}
		ref := m.GetRef()
		k := pkgKey{ref.GetName(), ref.GetVersion()}
		cur, exists := picked[k]
		if !exists || m.GetBuildNumber() > cur.buildNumber {
			picked[k] = pkgEntry{
				kind:        artifactKindToString(ref.GetKind()),
				profiles:    append([]string(nil), m.GetProfiles()...),
				buildID:     m.GetBuildId(),
				buildNumber: m.GetBuildNumber(),
			}
		}
	}

	// Collapse to one BOMPackage per name — among versions, take the
	// highest semver (or string-greater for native versions).
	type nameKey = string
	highest := map[nameKey]pkgKey{}
	for k := range picked {
		cur, ok := highest[k.name]
		if !ok {
			highest[k.name] = k
			continue
		}
		cmp, err := versionutil.Compare(k.version, cur.version)
		if err != nil {
			if k.version > cur.version {
				highest[k.name] = k
			}
			continue
		}
		if cmp > 0 {
			highest[k.name] = k
		}
	}

	bom := make([]BOMPackage, 0, len(highest))
	for name, k := range highest {
		e := picked[k]
		bom = append(bom, BOMPackage{
			Name:     name,
			Kind:     e.kind,
			Version:  k.version,
			Profiles: e.profiles,
		})
	}
	sort.Slice(bom, func(i, j int) bool { return bom[i].Name < bom[j].Name })

	resolver := LocalBuildIDResolver(func(name, version string) string {
		if e, ok := picked[pkgKey{name, version}]; ok {
			return e.buildID
		}
		return ""
	})
	return bom, resolver, nil
}

// resolveArtifactForUpgrade looks up the manifest for a (name, version,
// build_id) tuple via DescribePackage. release.apply.package needs the
// entrypoint_checksum + build_number from this manifest.
func (srv *server) resolveArtifactForUpgrade(ctx context.Context, name, version, buildID string) (*repositorypb.ArtifactManifest, error) {
	repoAddr := config.ResolveLocalServiceAddr("repository.PackageRepository")
	if repoAddr == "" {
		return nil, fmt.Errorf("repository service not found in registry")
	}
	rc, err := repository_client.NewRepositoryService_Client(repoAddr, "repository.PackageRepository")
	if err != nil {
		return nil, fmt.Errorf("repository client: %w", err)
	}
	defer rc.Close()

	manifests, err := rc.ListArtifacts()
	if err != nil {
		return nil, fmt.Errorf("ListArtifacts: %w", err)
	}
	for _, m := range manifests {
		if m == nil || m.GetRef() == nil {
			continue
		}
		if m.GetRef().GetName() != name {
			continue
		}
		if m.GetRef().GetVersion() != version {
			continue
		}
		if buildID != "" && m.GetBuildId() != buildID {
			continue
		}
		return m, nil
	}
	return nil, fmt.Errorf("artifact not found: %s@%s build_id=%s", name, version, buildID)
}

// artifactKindToString maps the proto enum to the lowercase kind
// strings used by release.apply.package and the package decision
// machinery ("service", "application", "infrastructure", ...).
func artifactKindToString(k repositorypb.ArtifactKind) string {
	switch k {
	case repositorypb.ArtifactKind_SERVICE:
		return "service"
	case repositorypb.ArtifactKind_APPLICATION:
		return "application"
	case repositorypb.ArtifactKind_INFRASTRUCTURE:
		return "infrastructure"
	case repositorypb.ArtifactKind_AGENT:
		return "agent"
	case repositorypb.ArtifactKind_SUBSYSTEM:
		return "subsystem"
	case repositorypb.ArtifactKind_COMMAND:
		return "command"
	case repositorypb.ArtifactKind_AWARENESS_BUNDLE:
		return "awareness_bundle"
	default:
		return ""
	}
}
