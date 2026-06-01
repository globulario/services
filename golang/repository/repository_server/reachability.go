package main

// reachability.go — Shared reachability engine (PR 4).
//
// This is the single source of truth for "is this artifact safe to remove?"
// All destructive operations (GC, archive, uninstall safety, revoke) MUST
// route through this engine instead of each implementing their own policy.
//
// Model
// ─────
//
//   Roots (what anchors artifacts — nothing reachable from a root can be GCed):
//     • ExplicitRoots  — build_ids held by desired state, installed state,
//                        workflow runs, or rollback pins. These come from the
//                        caller (controller, node-agent, admin CLI) via etcd
//                        queries. The repository engine is intentionally blind
//                        to etcd — callers provide the set.
//     • RetentionRoots — the last RetentionWindow PUBLISHED builds per
//                        (publisher, name, platform) series. Applied locally
//                        from the repository catalog. Ensures recent history
//                        is preserved without external coordination.
//
//   Expansion (what becomes reachable once a root is reached):
//     • Forward hard_dep edges: if artifact A is reachable and A hard_deps B,
//       then B is reachable too. B must not be deleted while A exists.
//     • Transitively: the expansion is exhaustive (BFS until stable).
//
//   Note on reverse deps: reverse dependency queries ("who depends on X?") are
//   NOT a separate root source. They are a special form of the expansion check:
//   "is X in any reachable artifact's hard_deps?" — answered by inspecting the
//   ReachableSet or calling IsBlockedByDependents.
//
// Usage
// ─────
//
//   // Caller collects explicit roots from etcd (desired state, installed state…)
//   explicitRoots := map[string]bool{"<build_id>": true, …}
//
//   rs := ComputeReachable(catalog, explicitRoots, DefaultReachabilityConfig())
//
//   if rs.Contains(someManifest) {
//       // cannot delete
//   }
//   if rs.BlockedByDependents(target, catalog) {
//       // something reachable depends on target — cannot delete
//   }

import (
	"sort"
	"strings"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
)

const defaultRetentionWindow = 3

// ReachabilityConfig controls the retention-window policy.
type ReachabilityConfig struct {
	// RetentionWindow is the number of most recent PUBLISHED builds per
	// (publisher, name, platform) series that are automatically considered roots.
	// 0 uses the default (3).
	RetentionWindow int
}

// DefaultReachabilityConfig returns the standard production configuration.
func DefaultReachabilityConfig() ReachabilityConfig {
	return ReachabilityConfig{RetentionWindow: defaultRetentionWindow}
}

// ReachableSet is the result of ComputeReachable.
// An artifact is reachable if it must be kept — it is either a root itself
// or transitively required by a root via hard_dep edges.
type ReachableSet struct {
	buildIDs map[string]bool // reachable build_ids
	keys     map[string]bool // reachable artifact storage keys
}

// Contains returns true if the manifest is in the reachable set.
func (rs ReachableSet) Contains(m *repopb.ArtifactManifest) bool {
	if m == nil {
		return false
	}
	if rs.buildIDs[m.GetBuildId()] {
		return true
	}
	return rs.keys[artifactKeyWithBuild(m.GetRef(), m.GetBuildNumber())]
}

// ContainsBuildID returns true if the given build_id is reachable.
func (rs ReachableSet) ContainsBuildID(buildID string) bool {
	return rs.buildIDs[buildID]
}

// Size returns the number of reachable artifacts.
func (rs ReachableSet) Size() int {
	return len(rs.keys)
}

// BlockedByDependents returns true if any reachable artifact in the catalog
// has targetName in its hard_deps. This is the reverse-dep safety check for
// destructive operations: "can I delete/archive artifact named targetName?"
//
// This must be called with the full catalog (not just the reachable subset)
// because the question is whether any reachable artifact would break.
func (rs ReachableSet) BlockedByDependents(targetName string, catalog []*repopb.ArtifactManifest) bool {
	target := canonicalName(targetName)
	for _, m := range catalog {
		if !rs.Contains(m) {
			continue
		}
		for _, dep := range m.GetHardDeps() {
			if canonicalName(dep.GetName()) == target {
				return true
			}
		}
	}
	return false
}

// ── Engine ────────────────────────────────────────────────────────────────

// ComputeReachable computes the reachable set from the catalog.
//
// catalog       — all manifests known to the repository (any state).
// explicitRoots — build_ids anchored externally (desired/installed/workflow/pins).
//                 May be nil or empty; retention-window roots still apply.
// cfg           — retention-window policy.
func ComputeReachable(
	catalog []*repopb.ArtifactManifest,
	explicitRoots map[string]bool,
	cfg ReachabilityConfig,
) ReachableSet {
	window := cfg.RetentionWindow
	if window <= 0 {
		window = defaultRetentionWindow
	}

	// Index: build_id → manifest, key → manifest.
	byBuildID := make(map[string]*repopb.ArtifactManifest, len(catalog))
	byKey := make(map[string]*repopb.ArtifactManifest, len(catalog))
	// Index: canonical name → manifests (for hard_dep forward expansion).
	byName := make(map[string][]*repopb.ArtifactManifest, len(catalog))

	for _, m := range catalog {
		if id := m.GetBuildId(); id != "" {
			byBuildID[id] = m
		}
		key := artifactKeyWithBuild(m.GetRef(), m.GetBuildNumber())
		byKey[key] = m
		name := canonicalName(m.GetRef().GetName())
		byName[name] = append(byName[name], m)
	}

	// ── Phase 1: Retention-window roots (no hard_dep expansion) ────────────
	//
	// Retention roots protect the N most recent PUBLISHED builds per series.
	// These are "available for rollback / download" roots, not "actively running"
	// roots. They do NOT expand via hard_deps — retaining a recent build does not
	// require retaining all of its transitive dependencies indefinitely.

	reachableBuildIDs := make(map[string]bool, window*len(byName)+len(explicitRoots))
	reachableKeys := make(map[string]bool, window*len(byName)+len(explicitRoots))

	markReachable := func(m *repopb.ArtifactManifest) {
		if m == nil {
			return
		}
		key := artifactKeyWithBuild(m.GetRef(), m.GetBuildNumber())
		reachableKeys[key] = true
		if id := m.GetBuildId(); id != "" {
			reachableBuildIDs[id] = true
		}
	}

	for _, candidates := range retentionGroups(catalog) {
		sort.Slice(candidates, func(i, j int) bool {
			return candidates[i].GetBuildNumber() > candidates[j].GetBuildNumber()
		})
		for i, m := range candidates {
			if i >= window {
				break
			}
			markReachable(m)
		}
	}

	// ── Phase 2: Explicit roots with transitive hard_dep expansion ────────
	//
	// Explicit roots represent actively-running or actively-desired artifacts
	// (desired state, installed state, workflow refs, rollback pins).
	// These DO expand transitively: if A is explicitly rooted and A hard_deps B,
	// then B must be kept — deleting B would break A.

	// Queue starts with all explicit-root manifests.
	queue := make([]*repopb.ArtifactManifest, 0, len(explicitRoots))
	enqueueForExpansion := func(m *repopb.ArtifactManifest) {
		if m == nil {
			return
		}
		key := artifactKeyWithBuild(m.GetRef(), m.GetBuildNumber())
		if reachableKeys[key] {
			return // already visited
		}
		markReachable(m)
		queue = append(queue, m)
	}

	for id := range explicitRoots {
		if m, ok := byBuildID[id]; ok {
			enqueueForExpansion(m)
		} else if m, ok := byKey[id]; ok {
			enqueueForExpansion(m)
		}
	}

	// BFS: expand along hard_dep edges from explicit roots only.
	for len(queue) > 0 {
		m := queue[0]
		queue = queue[1:]
		for _, dep := range m.GetHardDeps() {
			depName := canonicalName(dep.GetName())
			publisherFilter := strings.ToLower(strings.TrimSpace(dep.GetPublisherId()))
			for _, candidate := range byName[depName] {
				if publisherFilter != "" &&
					strings.ToLower(candidate.GetRef().GetPublisherId()) != publisherFilter {
					continue
				}
				enqueueForExpansion(candidate)
			}
		}
	}

	return ReachableSet{
		buildIDs: reachableBuildIDs,
		keys:     reachableKeys,
	}
}

// retentionGroups groups manifests by (publisher, name, platform) series.
// Only PUBLISHED manifests are considered for retention anchoring.
func retentionGroups(catalog []*repopb.ArtifactManifest) map[string][]*repopb.ArtifactManifest {
	groups := make(map[string][]*repopb.ArtifactManifest)
	for _, m := range catalog {
		// Only anchor PUBLISHED artifacts in the retention window.
		if m.GetPublishState() != repopb.PublishState_PUBLISHED {
			continue
		}
		ref := m.GetRef()
		series := ref.GetPublisherId() + "%" + ref.GetName() + "%" + ref.GetPlatform()
		groups[series] = append(groups[series], m)
	}
	return groups
}
