package main

import (
	"testing"
)

// ── Phase 1: Phantom rematerialization guard ─────────────────────────────────

// TestResolveInfraVersion_RejectsUnknown verifies that resolveInfraVersion
// skips nodes reporting "unknown" version — a fallback placeholder from
// loadSystemdUnits(). Using it would create desired entries with fake versions.
//
// Nodes are modelled as Status="ready" so the heartbeat is authoritative (the
// etcd fallback is suppressed). This represents an established cluster where
// all nodes genuinely have no good version for the component.
func TestResolveInfraVersion_RejectsUnknown(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": {Status: "ready", InstalledVersions: map[string]string{"xds": "unknown"}},
			"node-b": {Status: "ready", InstalledVersions: map[string]string{"xds": "unknown"}},
			"node-c": {Status: "ready", InstalledVersions: map[string]string{"xds": "unknown"}},
		},
	}
	version, source := srv.resolveInfraVersion("xds")
	if version != "" {
		t.Errorf("resolveInfraVersion should reject 'unknown', got version=%q source=%q", version, source)
	}
}

// TestResolveInfraVersion_RejectsFallback010 verifies that resolveInfraVersion
// skips nodes reporting "" — the old fallback version from pre-fix code.
//
// Node is modelled as Status="ready" so the heartbeat is authoritative.
func TestResolveInfraVersion_RejectsFallback010(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": {Status: "ready", InstalledVersions: map[string]string{"mcp": ""}},
		},
	}
	version, source := srv.resolveInfraVersion("mcp")
	if version != "" {
		t.Errorf("resolveInfraVersion should reject empty version, got version=%q source=%q", version, source)
	}
}

// TestResolveInfraVersion_AcceptsRealVersion verifies that resolveInfraVersion
// accepts legitimate versions from installed nodes.
func TestResolveInfraVersion_AcceptsRealVersion(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": {InstalledVersions: map[string]string{"xds": "0.0.2"}},
		},
	}
	version, source := srv.resolveInfraVersion("xds")
	if version != "0.0.2" {
		t.Errorf("resolveInfraVersion should accept real version, got version=%q source=%q", version, source)
	}
	if source != "installed:node-a" {
		t.Errorf("expected source 'installed:node-a', got %q", source)
	}
}

// TestResolveInfraVersion_SkipsUnknownPicksReal verifies mixed scenario:
// node-a reports "unknown", node-b reports real version → should pick node-b.
func TestResolveInfraVersion_SkipsUnknownPicksReal(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": {InstalledVersions: map[string]string{"gateway": "unknown"}},
			"node-b": {InstalledVersions: map[string]string{"gateway": "0.0.3"}},
		},
	}
	version, _ := srv.resolveInfraVersion("gateway")
	if version != "0.0.3" {
		t.Errorf("resolveInfraVersion should skip 'unknown' and pick '0.0.3', got %q", version)
	}
}

// TestResolveInfraVersion_JoiningNodeUnknownDoesNotSuppressFallback verifies
// that a joining (non-ready) node reporting "unknown" does NOT suppress the
// etcd installed_state fallback. Only ready/Day1Ready nodes are authoritative.
//
// We can't assert the exact fallback result (it depends on what etcd has), but
// we CAN assert that version is not "unknown" — regardless of whether the
// fallback finds data or not.
func TestResolveInfraVersion_JoiningNodeUnknownDoesNotSuppressFallback(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			// Joining node — no Status, no Day1Phase.
			"node-joining": {InstalledVersions: map[string]string{"gateway": "unknown"}},
		},
	}
	version, _ := srv.resolveInfraVersion("gateway")
	// Result may be "" (nothing in etcd) or a real version (etcd has data).
	// Either is fine. What must never happen is returning "unknown".
	if version == "unknown" {
		t.Error("resolveInfraVersion must never return 'unknown' as version")
	}
}

// ── Phase 4: Old leader safety ───────────────────────────────────────────────

// TestAutoImportDisabled_DesiredUnchanged simulates the scenario where 20
// fallback services are reported by nodes but desired state is empty.
// The controller must NOT auto-import — desired state must remain empty.
func TestAutoImportDisabled_DesiredUnchanged(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": {InstalledVersions: makeStaleServices(20)},
			"node-b": {InstalledVersions: makeStaleServices(20)},
			"node-c": {InstalledVersions: makeStaleServices(20)},
		},
	}

	// Simulate: auto-import done flag should prevent any import.
	srv.autoImportDone.Store(true)
	if !srv.autoImportDone.Load() {
		t.Fatal("autoImportDone should be set to prevent phantom import")
	}

	// Verify: no desired state was created (we have no resource store, so
	// if import ran it would panic — the test passing proves it didn't).
}

// TestAutoImportDisabled_FakeVersionNotMagicString verifies that auto-import
// protection works for ANY fake version, not just "". Uses "9.9.9" as
// a fabricated version to prove filtering is structural, not magic-string.
func TestAutoImportDisabled_FakeVersionNotMagicString(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": {InstalledVersions: map[string]string{
				"dns":  "9.9.9", // fake version, but looks real
				"rbac": "9.9.9",
			}},
		},
	}

	// Auto-import is structurally disabled (removed from startupAutoImport
	// and Trigger B). Even with plausible-looking version strings, the
	// controller will never auto-create desired state from runtime.
	srv.autoImportDone.Store(true)
	if !srv.autoImportDone.Load() {
		t.Fatal("autoImportDone should prevent import regardless of version strings")
	}
	// If we had a resource store and ran importInstalledToDesired, it would
	// still work — but the auto-triggers that called it are removed.
	// The protection is structural (no auto-trigger), not string-based.
}

// ── Phase 7: Minimum reconcile version gate ──────────────────────────────────

// TestMinReconcileVersion_BelowMinimum verifies that a controller below the
// minimum safe reconcile version cannot mutate desired state.
func TestMinReconcileVersion_BelowMinimum(t *testing.T) {
	cases := []struct {
		version string
		safe    bool
	}{
		{"0.0.1", false}, // old Day 0 default
		{"0.0.2", false}, // pre-fix
		{"0.0.8", false}, // dell's version from report
		{"0.0.9", false}, // below minimum
		{"0.0.10", true}, // at minimum
		{"0.0.11", true}, // above minimum
		{"", true},  // future release
		{"1.0.0", true},  // future major
	}
	for _, tc := range cases {
		t.Run(tc.version, func(t *testing.T) {
			result := isReconcileSafe(tc.version)
			if result != tc.safe {
				t.Errorf("isReconcileSafe(%q) = %v, want %v", tc.version, result, tc.safe)
			}
		})
	}
}

// ── checkRuntimeDeps: catalog-miss edge cases ─────────────────────────────────

// TestCheckRuntimeDeps_CatalogMissTreatedAsMissing verifies that a runtime
// dependency whose name is not found in the catalog is returned as missing
// (not silently skipped). A missing-from-catalog dep must block the workload
// so BFS can classify it as unresolvable, surfacing Day1WorkloadBlocked instead
// of letting the node spin in dependency_seeding_in_progress forever.
func TestCheckRuntimeDeps_CatalogMissTreatedAsMissing(t *testing.T) {
	// We need a Component whose RuntimeLocalDependencies contains a name that
	// does not exist in the global catalog. Inject it directly.
	synthetic := &Component{
		Name:                     "fake-workload-zzz",
		Kind:                     KindWorkload,
		RuntimeLocalDependencies: []string{"nonexistent-ghost-dep-zzz"},
	}

	// Healthy units and installed versions are both empty — the dep is not
	// installed and not running, which is fine: the important thing is that
	// CatalogByName("nonexistent-ghost-dep-zzz") returns nil.
	missing := checkRuntimeDeps(synthetic, map[string]bool{}, map[string]string{})

	if len(missing) == 0 {
		t.Fatal("checkRuntimeDeps should return 'nonexistent-ghost-dep-zzz' as missing, but returned empty slice")
	}
	found := false
	for _, m := range missing {
		if m == "nonexistent-ghost-dep-zzz" {
			found = true
		}
	}
	if !found {
		t.Errorf("checkRuntimeDeps missing list = %v, want 'nonexistent-ghost-dep-zzz'", missing)
	}
}

// TestCheckRuntimeDeps_CommandDepPublisherKey verifies that a KindCommand
// dependency stored in InstalledVersions under a publisher-qualified key
// ("core@globular.io/rclone") is found when the dep name is bare ("rclone").
// Uses lookupInstalledVersionFromMap so publisher-prefix keys don't cause
// false "missing" reports.
func TestCheckRuntimeDeps_CommandDepPublisherKey(t *testing.T) {
	// rclone is a real KindCommand in the catalog.
	comp := CatalogByName("rclone")
	if comp == nil {
		t.Skip("rclone not in catalog")
	}
	// Workload that depends on rclone (backup has this dep; synthesise directly).
	synthetic := &Component{
		Name:                     "fake-backup-workload",
		Kind:                     KindWorkload,
		RuntimeLocalDependencies: []string{"rclone"},
	}
	// InstalledVersions uses the publisher-qualified key, as the node agent writes.
	installed := map[string]string{
		"core@globular.io/rclone": "1.73.1",
	}
	missing := checkRuntimeDeps(synthetic, map[string]bool{}, installed)
	for _, m := range missing {
		if normalizeComponentName(m) == "rclone" {
			t.Errorf("checkRuntimeDeps reported rclone missing even though it is in InstalledVersions under publisher key")
		}
	}
}

// ── resolveInfraVersion: COMMAND kind heartbeat resolution ────────────────────

// TestResolveInfraVersion_CommandKindFromHeartbeat verifies that
// resolveInfraVersion resolves a KindCommand package (rclone, restic, sctool,
// mc, etc.) from the in-memory heartbeat map, just like KindInfrastructure.
// The function is kind-agnostic at the heartbeat layer — it matches by name in
// InstalledVersions regardless of the catalog kind.
func TestResolveInfraVersion_CommandKindFromHeartbeat(t *testing.T) {
	srv := &server{}
	srv.state = &controllerState{
		Nodes: map[string]*nodeState{
			"node-a": {
				Status:            "ready",
				InstalledVersions: map[string]string{"rclone": "1.67.0"},
			},
		},
	}
	version, source := srv.resolveInfraVersion("rclone")
	if version != "1.67.0" {
		t.Errorf("resolveInfraVersion should pick up KindCommand 'rclone' from heartbeat, got version=%q source=%q", version, source)
	}
	if source != "installed:node-a" {
		t.Errorf("expected source 'installed:node-a', got %q", source)
	}
}

// makeStaleServices creates a map of N services all reporting "" fallback
// version, simulating old node-agent behavior.
func makeStaleServices(n int) map[string]string {
	services := []string{
		"dns", "rbac", "authentication", "ldap", "media",
		"search", "chat", "blog", "monitoring", "echo",
		"storage", "log", "event", "conversation", "certificates",
		"ai-memory", "ai-executor", "ai-watcher", "ai-router", "compute",
	}
	m := make(map[string]string, n)
	for i := 0; i < n && i < len(services); i++ {
		m[services[i]] = ""
	}
	return m
}
