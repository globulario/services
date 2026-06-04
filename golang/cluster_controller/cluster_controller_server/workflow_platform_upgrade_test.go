package main

// workflow_platform_upgrade_test.go — pins the per-(node, package)
// decision contract for the platform.upgrade workflow.
//
// Contract (verbatim from operator intent, 2026-06-04 conversation):
//
//   for each (node × BOM-package):
//     if profile mismatch:                       skip (profile_skip)
//     if not currently installed on this node:   skip (not_installed)
//     if installed_version == BOM_version:       skip (up_to_date)
//     if installed_version >  BOM_version:       skip (skip_downgrade)
//     if BOM_version not in local repository:    skip (missing_in_repo)
//     if installed_version <  BOM_version:       upgrade
//
// This contract is the explicit fix for the v1.2.155-v1.2.159 incidents
// where the old direct-etcd-write platform-upgrade CLI bypassed every
// gate above and bulk-applied ServiceDesiredVersion for the entire BOM,
// undoing operator removals and creating 28 fresh DesiredBuildIdOrphaned
// findings.

import (
	"strings"
	"testing"
)

// fixedResolver returns a LocalBuildIDResolver backed by a static table.
// Used to simulate the local repository's authoritative view of which
// (name, version) tuples are actually installable.
func fixedResolver(table map[string]string) LocalBuildIDResolver {
	return func(name, version string) string {
		return table[name+"@"+version]
	}
}

// ── Test 1: profile_skip ─────────────────────────────────────────────
func TestEvaluate_ProfileMismatch_SkipsWithProfileSkip(t *testing.T) {
	nodes := []NodeView{{
		NodeID:            "node-a",
		Profiles:          []string{"control-plane", "core"},
		InstalledVersions: map[string]string{"some-svc": "1.0.0"},
	}}
	bom := []BOMPackage{{
		Name: "some-svc", Kind: "service", Version: "1.0.0",
		Profiles: []string{"compute"}, // no overlap with node profiles
	}}
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom, fixedResolver(nil))
	if len(audit) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(audit))
	}
	if audit[0].Action != "profile_skip" {
		t.Errorf("Action = %q, want profile_skip", audit[0].Action)
	}
	if len(upgrades) != 0 {
		t.Errorf("expected 0 upgrades, got %d", len(upgrades))
	}
}

// ── Test 2: not_installed → SKIP (respect operator removal) ─────────
// This is the v1.2.159 regression fix: do not silently re-install a
// package the operator removed.
func TestEvaluate_NotInstalled_RespectsRemoval(t *testing.T) {
	nodes := []NodeView{{
		NodeID:            "node-a",
		Profiles:          []string{"core"},
		InstalledVersions: map[string]string{}, // empty: not installed
	}}
	bom := []BOMPackage{{
		Name: "echo", Kind: "service", Version: "1.2.151",
		Profiles: []string{"core"},
	}}
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom, fixedResolver(nil))
	if len(audit) != 1 {
		t.Fatalf("expected 1 audit entry, got %d", len(audit))
	}
	if audit[0].Action != "not_installed" {
		t.Errorf("Action = %q, want not_installed (operator removal must be preserved)",
			audit[0].Action)
	}
	if len(upgrades) != 0 {
		t.Errorf("upgrades must be empty when package was operator-removed; got %d", len(upgrades))
	}
}

// ── Test 3: up_to_date → skip ────────────────────────────────────────
func TestEvaluate_UpToDate_Skips(t *testing.T) {
	nodes := []NodeView{{
		NodeID:            "node-a",
		Profiles:          []string{"core"},
		InstalledVersions: map[string]string{"mcp": "1.2.151"},
	}}
	bom := []BOMPackage{{
		Name: "mcp", Kind: "service", Version: "1.2.151",
		Profiles: []string{"core"},
	}}
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom,
		fixedResolver(map[string]string{"mcp@1.2.151": "abc-build-id"}))
	if audit[0].Action != "up_to_date" {
		t.Errorf("Action = %q, want up_to_date", audit[0].Action)
	}
	if len(upgrades) != 0 {
		t.Errorf("expected 0 upgrades when up-to-date; got %d", len(upgrades))
	}
}

// ── Test 4: skip_downgrade → never go backwards ──────────────────────
// installed_version 1.2.152 > BOM_version 1.2.151 → skip.
func TestEvaluate_SkipDowngrade_NeverGoesBackwards(t *testing.T) {
	nodes := []NodeView{{
		NodeID:            "node-a",
		Profiles:          []string{"core"},
		InstalledVersions: map[string]string{"cluster-controller": "1.2.152"},
	}}
	bom := []BOMPackage{{
		Name: "cluster-controller", Kind: "service", Version: "1.2.151",
		Profiles: []string{"core"},
	}}
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom,
		fixedResolver(map[string]string{"cluster-controller@1.2.151": "old-bid"}))
	if audit[0].Action != "skip_downgrade" {
		t.Errorf("Action = %q, want skip_downgrade", audit[0].Action)
	}
	if len(upgrades) != 0 {
		t.Errorf("must never produce downgrade upgrade; got %d", len(upgrades))
	}
}

// ── Test 5: upgrade → BOM > installed AND resolver has the version ──
func TestEvaluate_UpgradeDispatched(t *testing.T) {
	nodes := []NodeView{{
		NodeID:            "node-a",
		Profiles:          []string{"core", "control-plane"},
		InstalledVersions: map[string]string{"cluster-controller": "1.2.152"},
	}}
	bom := []BOMPackage{{
		Name: "cluster-controller", Kind: "service", Version: "1.2.153",
		Profiles: []string{"control-plane"},
	}}
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom,
		fixedResolver(map[string]string{"cluster-controller@1.2.153": "a2180517-1da1-4cf4-af47-3f5f155c7007"}))
	if audit[0].Action != "upgrade" {
		t.Errorf("Action = %q, want upgrade", audit[0].Action)
	}
	if len(upgrades) != 1 {
		t.Fatalf("expected 1 upgrade, got %d", len(upgrades))
	}
	u := upgrades[0]
	if u.LocalBuildID != "a2180517-1da1-4cf4-af47-3f5f155c7007" {
		t.Errorf("upgrade must carry LOCAL repo build_id, not BOM's; got %q", u.LocalBuildID)
	}
	if u.BOMVersion != "1.2.153" || u.InstalledVersion != "1.2.152" {
		t.Errorf("decision metadata wrong: %+v", u)
	}
}

// ── Test 6: missing_in_repo → upgrade refused (orphan-prevention) ───
// BOM > installed but local repo has no resolvable build_id → refuse.
// This is the v1.2.155-v1.2.159 orphan-prevention guard.
func TestEvaluate_MissingInRepo_RefusesToDispatch(t *testing.T) {
	nodes := []NodeView{{
		NodeID:            "node-a",
		Profiles:          []string{"core"},
		InstalledVersions: map[string]string{"some-svc": "1.0.0"},
	}}
	bom := []BOMPackage{{
		Name: "some-svc", Kind: "service", Version: "2.0.0",
		Profiles: []string{"core"},
	}}
	// Resolver returns "" — local repo has no installable artifact
	// for some-svc@2.0.0.
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom, fixedResolver(nil))
	if audit[0].Action != "missing_in_repo" {
		t.Errorf("Action = %q, want missing_in_repo (refuses orphan)", audit[0].Action)
	}
	if len(upgrades) != 0 {
		t.Fatalf("must not dispatch upgrade when local repo cannot serve it; got %d", len(upgrades))
	}
}

// ── Test 7: native-version equal → up_to_date ────────────────────────
// Non-semver versions (minio RELEASE.X) fall back to string equality.
func TestEvaluate_NativeVersionEqual_IsUpToDate(t *testing.T) {
	nodes := []NodeView{{
		NodeID:   "node-a",
		Profiles: []string{"storage"},
		InstalledVersions: map[string]string{
			"minio": "RELEASE.2025-09-07T16-13-09Z",
		},
	}}
	bom := []BOMPackage{{
		Name: "minio", Kind: "infrastructure",
		Version:  "RELEASE.2025-09-07T16-13-09Z",
		Profiles: []string{"storage"},
	}}
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom, fixedResolver(nil))
	if audit[0].Action != "up_to_date" {
		t.Errorf("Action = %q, want up_to_date for matching native version", audit[0].Action)
	}
	if len(upgrades) != 0 {
		t.Errorf("must not dispatch when native versions match; got %d", len(upgrades))
	}
}

// ── Test 8: native-version different + resolvable → upgrade ─────────
func TestEvaluate_NativeVersionDifferent_DispatchesIfResolvable(t *testing.T) {
	nodes := []NodeView{{
		NodeID:   "node-a",
		Profiles: []string{"storage"},
		InstalledVersions: map[string]string{
			"minio": "RELEASE.2025-08-13T08-35-41Z",
		},
	}}
	bom := []BOMPackage{{
		Name: "minio", Kind: "infrastructure",
		Version:  "RELEASE.2025-09-07T16-13-09Z",
		Profiles: []string{"storage"},
	}}
	resolver := fixedResolver(map[string]string{
		"minio@RELEASE.2025-09-07T16-13-09Z": "minio-bid-aaaa",
	})
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom, resolver)
	if audit[0].Action != "upgrade" {
		t.Errorf("Action = %q, want upgrade for forward native-version change", audit[0].Action)
	}
	if len(upgrades) != 1 {
		t.Fatalf("expected 1 upgrade, got %d", len(upgrades))
	}
	if upgrades[0].LocalBuildID != "minio-bid-aaaa" {
		t.Errorf("expected local build_id, got %q", upgrades[0].LocalBuildID)
	}
}

// ── Test 9: per-node iteration, deterministic order ─────────────────
// Multiple nodes × multiple packages should produce a stable, sorted
// audit so the workflow's safe_retry idempotency is real.
func TestEvaluate_MultiNodeMultiPackage_DeterministicOrder(t *testing.T) {
	nodes := []NodeView{
		{NodeID: "node-z", Profiles: []string{"core"},
			InstalledVersions: map[string]string{"svc-a": "1.0.0"}},
		{NodeID: "node-a", Profiles: []string{"core"},
			InstalledVersions: map[string]string{"svc-a": "1.0.0", "svc-b": "1.0.0"}},
	}
	bom := []BOMPackage{
		{Name: "svc-b", Kind: "service", Version: "1.0.0", Profiles: []string{"core"}},
		{Name: "svc-a", Kind: "service", Version: "1.0.0", Profiles: []string{"core"}},
	}
	audit, _ := evaluateUpgradeDecisions(nodes, bom, fixedResolver(nil))
	if len(audit) != 4 {
		t.Fatalf("expected 4 audit entries (2 nodes × 2 packages), got %d", len(audit))
	}
	// Expect sorted-by-node-then-package order:
	//   (node-a, svc-a), (node-a, svc-b), (node-z, svc-a), (node-z, svc-b)
	expected := []string{
		"node-a/svc-a",
		"node-a/svc-b",
		"node-z/svc-a",
		"node-z/svc-b",
	}
	for i, exp := range expected {
		got := audit[i].NodeID + "/" + audit[i].PackageName
		if got != exp {
			t.Errorf("audit[%d] = %q, want %q (order must be deterministic for idempotency)", i, got, exp)
		}
	}
}

// ── Test 10: per-node decision (operator removed on one node only) ──
// node-a has the package installed, node-b doesn't. Only node-a gets the
// upgrade — node-b's removal is respected.
func TestEvaluate_PerNodeDecision_RespectsRemovalOnSpecificNode(t *testing.T) {
	nodes := []NodeView{
		{NodeID: "node-a", Profiles: []string{"core"},
			InstalledVersions: map[string]string{"my-svc": "1.0.0"}},
		{NodeID: "node-b", Profiles: []string{"core"},
			InstalledVersions: map[string]string{}}, // removed on node-b
	}
	bom := []BOMPackage{{
		Name: "my-svc", Kind: "service", Version: "1.1.0",
		Profiles: []string{"core"},
	}}
	resolver := fixedResolver(map[string]string{"my-svc@1.1.0": "new-bid"})
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom, resolver)
	if len(audit) != 2 {
		t.Fatalf("expected 2 audit entries, got %d", len(audit))
	}
	if audit[0].NodeID != "node-a" || audit[0].Action != "upgrade" {
		t.Errorf("node-a should be upgraded; got %+v", audit[0])
	}
	if audit[1].NodeID != "node-b" || audit[1].Action != "not_installed" {
		t.Errorf("node-b should be not_installed (operator removed); got %+v", audit[1])
	}
	if len(upgrades) != 1 {
		t.Fatalf("expected exactly 1 upgrade (only node-a); got %d", len(upgrades))
	}
}

// ── Test 11: profilesIntersect helper edge cases ────────────────────
func TestProfilesIntersect(t *testing.T) {
	cases := []struct {
		node, pkg []string
		want      bool
		name      string
	}{
		{[]string{"core"}, []string{"core"}, true, "exact match"},
		{[]string{"control-plane", "core"}, []string{"core", "compute"}, true, "one overlap"},
		{[]string{"control-plane"}, []string{"compute"}, false, "no overlap"},
		{[]string{}, []string{"core"}, false, "empty node"},
		{[]string{"core"}, []string{}, false, "empty package"},
		{[]string{"Core"}, []string{"core"}, true, "case insensitive"},
		{[]string{" core "}, []string{"core"}, true, "whitespace trimmed"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := profilesIntersect(profileSet(tc.node), tc.pkg)
			if got != tc.want {
				t.Errorf("node=%v pkg=%v got=%v want=%v", tc.node, tc.pkg, got, tc.want)
			}
		})
	}
}

// ── Test 12: regression — the v1.2.159 scenario ─────────────────────
// Reproduces the v1.2.159 platform-upgrade mistake: BOM has 7 optional
// services with profiles matching the node, but they were removed in
// Part A so InstalledVersions is empty. The CORRECT behaviour is
// not_installed (preserve removal), not upgrade. The old direct-etcd
// CLI did the opposite.
func TestEvaluate_v1_2_159_OperatorRemovalRegression(t *testing.T) {
	nodes := []NodeView{{
		NodeID:   "ryzen",
		Profiles: []string{"control-plane", "core", "storage"},
		// Part A removed echo/catalog/blog/mail/sql/conversation/ldap.
		// InstalledVersions still has the rest at 1.2.151.
		InstalledVersions: map[string]string{
			"cluster-controller": "1.2.152",
			"node-agent":         "1.2.151",
			"repository":         "1.2.151",
			// echo, catalog, blog, mail, sql, conversation, ldap → ABSENT
		},
	}}
	bom := []BOMPackage{
		{Name: "echo", Kind: "service", Version: "1.2.151", Profiles: []string{"core"}},
		{Name: "catalog", Kind: "service", Version: "1.2.151", Profiles: []string{"core"}},
		{Name: "cluster-controller", Kind: "service", Version: "1.2.153", Profiles: []string{"control-plane"}},
		{Name: "node-agent", Kind: "service", Version: "1.2.151", Profiles: []string{"core"}},
		{Name: "repository", Kind: "service", Version: "1.2.151", Profiles: []string{"core"}},
	}
	resolver := fixedResolver(map[string]string{
		"cluster-controller@1.2.153": "a2180517-1da1-4cf4-af47-3f5f155c7007",
	})
	audit, upgrades := evaluateUpgradeDecisions(nodes, bom, resolver)

	byPkg := map[string]string{}
	for _, d := range audit {
		byPkg[d.PackageName] = d.Action
	}

	if byPkg["echo"] != "not_installed" {
		t.Errorf("echo regression: got %q, want not_installed", byPkg["echo"])
	}
	if byPkg["catalog"] != "not_installed" {
		t.Errorf("catalog regression: got %q, want not_installed", byPkg["catalog"])
	}
	if byPkg["node-agent"] != "up_to_date" {
		t.Errorf("node-agent: got %q, want up_to_date", byPkg["node-agent"])
	}
	if byPkg["repository"] != "up_to_date" {
		t.Errorf("repository: got %q, want up_to_date", byPkg["repository"])
	}
	if byPkg["cluster-controller"] != "upgrade" {
		t.Errorf("cluster-controller: got %q, want upgrade", byPkg["cluster-controller"])
	}

	if len(upgrades) != 1 {
		t.Fatalf("expected exactly 1 upgrade (cluster-controller); got %d", len(upgrades))
	}
	if upgrades[0].PackageName != "cluster-controller" {
		t.Errorf("the one upgrade should be cluster-controller; got %s", upgrades[0].PackageName)
	}
	for _, u := range upgrades {
		if strings.Contains("echo,catalog,blog,mail,sql,conversation,ldap", u.PackageName) {
			t.Errorf("operator-removed package re-added in upgrades: %s", u.PackageName)
		}
	}
}
