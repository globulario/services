package rules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
)

func TestArtifactLayoutDriftLocal_UnexpectedEntry_Warns(t *testing.T) {
	td := t.TempDir()
	if err := os.MkdirAll(filepath.Join(td, "pki"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(td, "etcd"), 0o755); err != nil {
		t.Fatal(err)
	}
	// A genuinely-unknown name (NOT a catalog service) — this is the
	// truly-unknown drift path. (Using a real service name here would now be
	// reclassified as an uninstalled-service cleanup candidate, not unknown.)
	if err := os.MkdirAll(filepath.Join(td, "totally-unknown-svc"), 0o755); err != nil {
		t.Fatal(err)
	}

	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	findings := (artifactLayoutDriftLocal{}).Evaluate(nil, testConfig())
	if len(findings) != 1 {
		t.Fatalf("expected 1 finding, got %d: %+v", len(findings), findings)
	}
	if findings[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Fatalf("expected WARN, got %v", findings[0].Severity)
	}
}

func TestArtifactLayoutDriftLocal_AllowlistOnly_Silent(t *testing.T) {
	td := t.TempDir()
	for _, d := range []string{"pki", "etcd", "config", "services", "repository"} {
		if err := os.MkdirAll(filepath.Join(td, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}

	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	findings := (artifactLayoutDriftLocal{}).Evaluate(nil, testConfig())
	if len(findings) != 0 {
		t.Fatalf("expected 0 findings, got %d: %+v", len(findings), findings)
	}
}

func TestArtifactLayoutDriftLocal_InstallTransactionsPlatformState_Silent(t *testing.T) {
	td := t.TempDir()
	for _, d := range []string{"pki", "etcd", "install-transactions"} {
		if err := os.MkdirAll(filepath.Join(td, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(td, "install-transactions", "ownership.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}

	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	findings := (artifactLayoutDriftLocal{}).Evaluate(nil, testConfig())
	for _, f := range findings {
		if strings.Contains(f.Summary, "install-transactions") {
			t.Fatalf("install transaction state must be platform state, not layout drift: %s", f.Summary)
		}
	}
}

// ─────────────────────────────────────────────────────────────────────────────
// v1.2.117 — required tests for the canonical runtime dir model.
// See docs/intent/service_runtime_paths_must_be_canonical.yaml and the
// claude_v1_2_117_artifact_layout_drift_plan.md handoff.
// ─────────────────────────────────────────────────────────────────────────────

func withArtifactStateRoot(t *testing.T, root string) {
	t.Helper()
	original := artifactStateRootPath
	artifactStateRootPath = root
	t.Cleanup(func() { artifactStateRootPath = original })
}

func mkDir(t *testing.T, root, name string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", name, err)
	}
}

func mkFileInDir(t *testing.T, root, dir, fname string) {
	t.Helper()
	mkDir(t, root, dir)
	if err := os.WriteFile(filepath.Join(root, dir, fname), []byte("x"), 0o644); err != nil {
		t.Fatalf("write %s/%s: %v", dir, fname, err)
	}
}

func snapshotWithInstalled(installedNames ...string) *collector.Snapshot {
	inv := &node_agentpb.Inventory{}
	for _, n := range installedNames {
		inv.Components = append(inv.Components, &node_agentpb.InstalledComponent{Name: n})
	}
	return &collector.Snapshot{
		Inventories: map[string]*node_agentpb.Inventory{"fake-node": inv},
	}
}

func findingByInvariant(findings []Finding, invariantID string) *Finding {
	for i := range findings {
		if findings[i].InvariantID == invariantID {
			return &findings[i]
		}
	}
	return nil
}

func summaryContains(t *testing.T, f *Finding, want ...string) {
	t.Helper()
	if f == nil {
		t.Fatalf("expected finding, got nil")
	}
	for _, w := range want {
		if !strings.Contains(f.Summary, w) {
			t.Errorf("summary missing %q; got: %s", w, f.Summary)
		}
	}
}

// Required Test 1 — canonical installed runtime dir produces no drift.
func TestLayoutDrift_LegitimateInstalledServiceRuntimeDir_NoDrift(t *testing.T) {
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "packages")
	mkDir(t, root, "etcd")
	mkDir(t, root, "ai-executor")
	mkDir(t, root, "cluster-doctor")
	mkDir(t, root, "node-agent")

	snap := snapshotWithInstalled("ai-executor", "cluster-doctor", "node-agent")
	findings := artifactLayoutDriftLocal{}.Evaluate(snap, testConfig())
	if len(findings) != 0 {
		t.Errorf("expected no findings for canonical installed dirs; got %d: %+v", len(findings), findings)
	}
}

// Required Test 2 — unknown non-empty dir produces a WARN finding.
func TestLayoutDrift_UnknownNonEmptyDir_ProducesWarn(t *testing.T) {
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "packages")
	mkFileInDir(t, root, "mystery-service", "data.bin")

	findings := artifactLayoutDriftLocal{}.Evaluate(snapshotWithInstalled(), testConfig())
	f := findingByInvariant(findings, "artifact.layout_drift_local")
	if f == nil {
		t.Fatalf("expected layout_drift_local finding; got %+v", findings)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("severity = %v, want WARN", f.Severity)
	}
	summaryContains(t, f, "unknown", "mystery-service")
}

// Required Test 3 — empty legacy alias of installed service is INFO cleanup,
// not WARN.
func TestLayoutDrift_EmptyLegacyAliasOfInstalledService_IsCleanupCandidate(t *testing.T) {
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "packages")
	mkDir(t, root, "cluster-doctor")
	mkDir(t, root, "cluster_doctor")
	mkDir(t, root, "clusterdoctor")

	snap := snapshotWithInstalled("cluster-doctor")
	findings := artifactLayoutDriftLocal{}.Evaluate(snap, testConfig())
	for _, f := range findings {
		if f.Severity == cluster_doctorpb.Severity_SEVERITY_WARN {
			t.Errorf("unexpected WARN for empty legacy alias: %s", f.Summary)
		}
	}
	var info *Finding
	for i := range findings {
		if findings[i].Severity == cluster_doctorpb.Severity_SEVERITY_INFO {
			info = &findings[i]
			break
		}
	}
	if info == nil {
		t.Fatalf("expected INFO cleanup-candidate finding; got %+v", findings)
	}
	summaryContains(t, info, "cleanup-candidate", "cluster_doctor", "clusterdoctor")
}

// ─────────────────────────────────────────────────────────────────────────────
// Catalog-aware reclassification (the torrent-orphan class).
// A dir whose name matches a component-catalog service that is NOT installed on
// this node is an uninstalled-service runtime dir, recognized via the catalog —
// NOT "unknown" drift, and NOT silenced by expanding a static allowlist
// (forbidden.fix.layout_drift_by_expanding_allowlist_only). It is downgraded in
// severity (empty → INFO cleanup; data-bearing → accurate WARN), never hidden.
// See invariant doctor.layout_drift_must_reflect_real_risk.
// ─────────────────────────────────────────────────────────────────────────────

// Empty catalog-known dir for an uninstalled service → INFO cleanup, not unknown WARN.
func TestLayoutDrift_CatalogKnownUninstalledEmptyDir_IsCleanupNotUnknown(t *testing.T) {
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "packages")
	mkDir(t, root, "torrent") // catalog-known (media-server), not installed, empty

	findings := artifactLayoutDriftLocal{}.Evaluate(snapshotWithInstalled(), testConfig())
	for _, f := range findings {
		if strings.Contains(f.Summary, "unknown") && strings.Contains(f.Summary, "torrent") {
			t.Fatalf("catalog-known uninstalled empty dir must not be 'unknown'; got: %s", f.Summary)
		}
		if f.Severity == cluster_doctorpb.Severity_SEVERITY_WARN && strings.Contains(f.Summary, "torrent") {
			t.Fatalf("empty uninstalled-service dir must not WARN; got: %s", f.Summary)
		}
	}
	var info *Finding
	for i := range findings {
		if findings[i].Severity == cluster_doctorpb.Severity_SEVERITY_INFO {
			info = &findings[i]
			break
		}
	}
	if info == nil {
		t.Fatalf("expected INFO cleanup-candidate; got %+v", findings)
	}
	summaryContains(t, info, "cleanup-candidate", "torrent")
}

// Non-empty catalog-known dir for an uninstalled service → WARN orphan-data with
// an accurate message (not "unknown"). Data-bearing orphans must not be
// auto-classified as safe cleanup.
func TestLayoutDrift_CatalogKnownUninstalledNonEmptyDir_IsOrphanDataWarn(t *testing.T) {
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "packages")
	mkFileInDir(t, root, "torrent", "downloads.db") // catalog-known, not installed, data present

	findings := artifactLayoutDriftLocal{}.Evaluate(snapshotWithInstalled(), testConfig())
	f := findingByInvariant(findings, "artifact.layout_drift_local")
	if f == nil {
		t.Fatalf("expected artifact.layout_drift_local finding; got %+v", findings)
	}
	if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("severity = %v, want WARN", f.Severity)
	}
	if strings.Contains(f.Summary, "unknown") {
		t.Errorf("data-bearing catalog-known orphan must not be 'unknown'; got: %s", f.Summary)
	}
	summaryContains(t, f, "not installed", "torrent")
}

// Installed catalog service runtime dir → silent (caught by installed-state, not 5b).
func TestLayoutDrift_CatalogKnownInstalledDir_Silent(t *testing.T) {
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "packages")
	mkFileInDir(t, root, "torrent", "state.db")

	snap := snapshotWithInstalled("torrent")
	findings := artifactLayoutDriftLocal{}.Evaluate(snap, testConfig())
	for _, f := range findings {
		if strings.Contains(f.Summary, "torrent") {
			t.Fatalf("installed torrent runtime dir must be silent; got: %s", f.Summary)
		}
	}
}

// Reference-safety: a catalog-known uninstalled dir actively pinned by a systemd
// unit must NOT be flagged (deletion would be undone; fix the unit instead).
func TestLayoutDrift_CatalogKnownUninstalledDir_NotFlaggedWhenPinned(t *testing.T) {
	td := t.TempDir()
	for _, d := range []string{"pki", "etcd"} {
		mkDir(t, td, d)
	}
	mkFileInDir(t, td, "torrent", "data.bin") // non-empty → would be orphan-data WARN if unpinned
	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	stubUnitDir(t, "[Service]\nWorkingDirectory=-"+filepath.Join(td, "torrent")+"\nExecStart=/bin/true\n")

	findings := (artifactLayoutDriftLocal{}).Evaluate(snapshotWithInstalled(), testConfig())
	for _, f := range findings {
		if strings.Contains(f.Summary, "torrent") {
			t.Fatalf("systemd-pinned catalog dir must NOT be flagged; got: %s", f.Summary)
		}
	}
}

// Required Test 4 — cluster-doctor aliases all map to the same canonical.
func TestLayoutDrift_ClusterDoctorAliasesAreDeterministic(t *testing.T) {
	for _, name := range []string{"clusterdoctor", "cluster-doctor", "cluster_doctor"} {
		if got := CanonicalRuntimeDir(name); got != "cluster-doctor" {
			t.Errorf("CanonicalRuntimeDir(%q) = %q, want %q", name, got, "cluster-doctor")
		}
	}
	aliases := AllRuntimeDirAliases("cluster-doctor")
	want := map[string]bool{"cluster-doctor": true, "cluster_doctor": true, "clusterdoctor": true}
	if len(aliases) != len(want) {
		t.Fatalf("AllRuntimeDirAliases length = %d, want %d (%v)", len(aliases), len(want), aliases)
	}
	for _, a := range aliases {
		if !want[a] {
			t.Errorf("unexpected alias: %q", a)
		}
	}
}

// Required Test 5 — node-agent aliases all map to the same canonical.
func TestLayoutDrift_NodeAgentAliasesAreDeterministic(t *testing.T) {
	for _, name := range []string{"nodeagent", "node-agent", "node_agent"} {
		if got := CanonicalRuntimeDir(name); got != "node-agent" {
			t.Errorf("CanonicalRuntimeDir(%q) = %q, want %q", name, got, "node-agent")
		}
	}
}

// Required Test 6 — model recognizes ai-executor, ai-memory, dns, event, file.
func TestLayoutDrift_KnownServicesIncluded(t *testing.T) {
	known := []string{"ai-executor", "ai-memory", "dns", "event", "file"}
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "packages")
	for _, s := range known {
		mkDir(t, root, s)
	}
	snap := snapshotWithInstalled(known...)
	findings := artifactLayoutDriftLocal{}.Evaluate(snap, testConfig())
	for _, f := range findings {
		if f.Severity == cluster_doctorpb.Severity_SEVERITY_WARN {
			t.Errorf("WARN for installed services should not fire: %s", f.Summary)
		}
	}
}

// Required Test 7 — layout rule must remain layout-only; it does not enforce
// nor suppress permission invariants like etcd 0700. Permission rules are
// separate. Here we verify the rule is silent on platform base entries and
// does not produce findings about etcd permissions either way.
func TestLayoutDrift_PlatformBaseSilentDoesNotSuppressOtherInvariants(t *testing.T) {
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "etcd")
	mkDir(t, root, "packages")
	findings := artifactLayoutDriftLocal{}.Evaluate(snapshotWithInstalled(), testConfig())
	if len(findings) != 0 {
		t.Errorf("layout rule must remain layout-only; got %d findings: %+v",
			len(findings), findings)
	}
}

// Required Test 8 — non-empty duplicate legacy alias requires operator review
// (WARN), distinct from the empty-cleanup case.
func TestLayoutDrift_NonEmptyDuplicateAliasProducesDuplicateWarn(t *testing.T) {
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "packages")
	mkDir(t, root, "node-agent")
	mkFileInDir(t, root, "node_agent", "leftover.db")

	snap := snapshotWithInstalled("node-agent")
	findings := artifactLayoutDriftLocal{}.Evaluate(snap, testConfig())
	dup := findingByInvariant(findings, "service.runtime_dir_name_must_be_canonical")
	if dup == nil {
		t.Fatalf("expected service.runtime_dir_name_must_be_canonical finding; got %+v", findings)
	}
	if dup.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("severity = %v, want WARN", dup.Severity)
	}
	summaryContains(t, dup, "node_agent")
}

// Defensive: transient backup files are cleanup, not unknown WARN.
func TestLayoutDrift_TransientBackupFile_IsCleanupCandidate(t *testing.T) {
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "packages")
	if err := os.WriteFile(filepath.Join(root, "config.json.bak.1779932408243926936"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "day0-install.jsonl"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	findings := artifactLayoutDriftLocal{}.Evaluate(snapshotWithInstalled(), testConfig())
	for _, f := range findings {
		if f.Severity == cluster_doctorpb.Severity_SEVERITY_WARN {
			t.Errorf("transient files must not produce WARN: %s", f.Summary)
		}
	}
}

// Defensive: hidden entries are silent.
func TestLayoutDrift_HiddenDir_IsSilent(t *testing.T) {
	root := t.TempDir()
	withArtifactStateRoot(t, root)
	mkDir(t, root, "packages")
	mkDir(t, root, ".lock")
	findings := artifactLayoutDriftLocal{}.Evaluate(snapshotWithInstalled(), testConfig())
	if len(findings) != 0 {
		t.Errorf("hidden entries must be silent; got %d findings: %+v", len(findings), findings)
	}
}

// Predicate test: CanonicalRuntimeDir handles unknown names sanely.
func TestCanonicalRuntimeDir_NoMutationForUnknownNames(t *testing.T) {
	cases := map[string]string{
		"my_new_service": "my-new-service",
		"plain-name":     "plain-name",
		"":               "",
		"  spaces  ":     "spaces",
	}
	for in, want := range cases {
		got := CanonicalRuntimeDir(in)
		if got != want {
			t.Errorf("CanonicalRuntimeDir(%q) = %q, want %q", in, got, want)
		}
	}
}

// ── Reference-safety guard regression tests ──────────────────────────────────
//
// Background (2026-06-03): the rule flagged 6 empty underscore directories as
// cleanup candidates even though active systemd units pinned them via
// WorkingDirectory= and ExecStartPre `mkdir`. Deleting them would have been
// undone on the next service start, and the misleading finding distracted
// operators from the real fix (unit templates emit legacy underscore form).
//
// These tests pin the new pinned-paths guard: when a path under stateRoot is
// referenced by an active systemd unit, the rule must NOT classify it as a
// cleanup candidate even if it is empty and matches a known legacy alias.

// stubUnitDir creates a temp dir containing a fake systemd unit file with the
// given content, and points systemdUnitDirsForLayoutDrift at it for the
// duration of the test.
func stubUnitDir(t *testing.T, unitContent string) {
	t.Helper()
	td := t.TempDir()
	unitPath := filepath.Join(td, "globular-stub.service")
	if err := os.WriteFile(unitPath, []byte(unitContent), 0o644); err != nil {
		t.Fatal(err)
	}
	old := systemdUnitDirsForLayoutDrift
	systemdUnitDirsForLayoutDrift = []string{td}
	t.Cleanup(func() { systemdUnitDirsForLayoutDrift = old })
}

// TestArtifactLayoutDriftLocal_EmptyLegacyAlias_NotCleanupWhenWorkingDirectoryPins
// is the direct regression for the live 2026-06-03 false-positive: empty
// underscore dirs that the systemd unit declares as WorkingDirectory MUST NOT
// be flagged as cleanup candidates.
func TestArtifactLayoutDriftLocal_EmptyLegacyAlias_NotCleanupWhenWorkingDirectoryPins(t *testing.T) {
	td := t.TempDir()
	// Canonical dash dir (installed) + empty underscore alias.
	for _, d := range []string{"pki", "etcd", "ai-executor", "ai_executor"} {
		if err := os.MkdirAll(filepath.Join(td, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	// Fake systemd unit pins the underscore form as WorkingDirectory.
	stubUnitDir(t, "[Service]\nWorkingDirectory=-"+filepath.Join(td, "ai_executor")+"\nExecStart=/bin/true\n")

	snap := &collector.Snapshot{
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Components: []*node_agentpb.InstalledComponent{{Name: "ai-executor"}}},
		},
	}
	findings := (artifactLayoutDriftLocal{}).Evaluate(snap, testConfig())
	for _, f := range findings {
		if strings.Contains(f.Summary, "ai_executor") {
			t.Fatalf("ai_executor is systemd-pinned; must NOT appear in any finding. got: %q", f.Summary)
		}
	}
}

// TestArtifactLayoutDriftLocal_EmptyLegacyAlias_NotCleanupWhenExecStartPreMkdirPins
// covers the second Globular pattern: ExecStartPre runs `mkdir -p <path>`
// at every start to ensure the dir exists. If we deleted the dir, the next
// start would re-create it. Treat it as pinned.
func TestArtifactLayoutDriftLocal_EmptyLegacyAlias_NotCleanupWhenExecStartPreMkdirPins(t *testing.T) {
	td := t.TempDir()
	for _, d := range []string{"pki", "etcd", "ai-memory", "ai_memory"} {
		if err := os.MkdirAll(filepath.Join(td, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	stubUnitDir(t, "[Service]\n"+
		"ExecStartPre=+/bin/sh -c 'mkdir -p "+filepath.Join(td, "ai_memory")+" && chown globular:globular "+filepath.Join(td, "ai_memory")+"'\n"+
		"ExecStart=/bin/true\n")

	snap := &collector.Snapshot{
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Components: []*node_agentpb.InstalledComponent{{Name: "ai-memory"}}},
		},
	}
	findings := (artifactLayoutDriftLocal{}).Evaluate(snap, testConfig())
	for _, f := range findings {
		if strings.Contains(f.Summary, "ai_memory") {
			t.Fatalf("ai_memory is mkdir-pinned by ExecStartPre; must NOT appear in any finding. got: %q", f.Summary)
		}
	}
}

// TestArtifactLayoutDriftLocal_EmptyLegacyAlias_StillCleanupWhenUnreferenced
// is the inverse: a truly orphan empty underscore alias with no systemd
// reference MUST still be flagged as a cleanup candidate. The fix is a
// guard, not a wholesale silencing.
func TestArtifactLayoutDriftLocal_EmptyLegacyAlias_StillCleanupWhenUnreferenced(t *testing.T) {
	td := t.TempDir()
	for _, d := range []string{"pki", "etcd", "node-agent", "node_agent"} {
		if err := os.MkdirAll(filepath.Join(td, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	// Stub unit dir contains no reference to node_agent.
	stubUnitDir(t, "[Service]\nWorkingDirectory=-"+filepath.Join(td, "node-agent")+"\nExecStart=/bin/true\n")

	snap := &collector.Snapshot{
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Components: []*node_agentpb.InstalledComponent{{Name: "node-agent"}}},
		},
	}
	findings := (artifactLayoutDriftLocal{}).Evaluate(snap, testConfig())
	found := false
	for _, f := range findings {
		if strings.Contains(f.Summary, "node_agent") {
			found = true
			if f.Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
				t.Errorf("unreferenced empty legacy alias should be INFO cleanup; got severity=%v", f.Severity)
			}
		}
	}
	if !found {
		t.Errorf("unreferenced empty legacy alias 'node_agent' must still surface as cleanup candidate")
	}
}

// TestArtifactLayoutDriftLocal_NonEmptyLegacyAlias_NotDuplicateWhenPinned
// proves the guard ALSO applies to non-empty legacy aliases. If systemd
// pins the dir, its content is the service's real state (not a duplicate)
// regardless of the canonical-vs-alias name distinction.
func TestArtifactLayoutDriftLocal_NonEmptyLegacyAlias_NotDuplicateWhenPinned(t *testing.T) {
	td := t.TempDir()
	for _, d := range []string{"pki", "etcd", "backup-manager", "backup_manager"} {
		if err := os.MkdirAll(filepath.Join(td, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	// Put a file in the underscore dir so it is non-empty.
	if err := os.WriteFile(filepath.Join(td, "backup_manager", "jobs.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	stubUnitDir(t, "[Service]\nWorkingDirectory=-"+filepath.Join(td, "backup_manager")+"\nExecStart=/bin/true\n")

	snap := &collector.Snapshot{
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Components: []*node_agentpb.InstalledComponent{{Name: "backup-manager"}}},
		},
	}
	findings := (artifactLayoutDriftLocal{}).Evaluate(snap, testConfig())
	for _, f := range findings {
		if strings.Contains(f.Summary, "backup_manager") {
			t.Fatalf("non-empty pinned legacy alias must NOT be flagged as duplicate; got %q", f.Summary)
		}
	}
}

// TestArtifactLayoutDriftLocal_NonEmptyLegacyAlias_StillDuplicateWhenUnreferenced
// inverse: non-empty legacy alias with NO systemd reference still surfaces
// as WARN duplicate (operator review). The fix preserves this real-risk
// signal — it only silences false positives.
func TestArtifactLayoutDriftLocal_NonEmptyLegacyAlias_StillDuplicateWhenUnreferenced(t *testing.T) {
	td := t.TempDir()
	for _, d := range []string{"pki", "etcd", "ai-router", "ai_router"} {
		if err := os.MkdirAll(filepath.Join(td, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(td, "ai_router", "stale.db"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	stubUnitDir(t, "[Service]\nExecStart=/bin/true\n") // no references

	snap := &collector.Snapshot{
		Inventories: map[string]*node_agentpb.Inventory{
			"node-1": {Components: []*node_agentpb.InstalledComponent{{Name: "ai-router"}}},
		},
	}
	findings := (artifactLayoutDriftLocal{}).Evaluate(snap, testConfig())
	found := false
	for _, f := range findings {
		if strings.Contains(f.Summary, "ai_router") {
			found = true
			if f.Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
				t.Errorf("unreferenced non-empty legacy alias should be WARN duplicate; got severity=%v", f.Severity)
			}
		}
	}
	if !found {
		t.Errorf("unreferenced non-empty legacy alias 'ai_router' must still surface as duplicate")
	}
}

// TestArtifactLayoutDriftLocal_TransientFile_StillCleanupRegardlessOfUnits
// pins the policy for day0-install.jsonl: classified as cleanup-transient
// regardless of unit files. Systemd never references it, but we want a
// dedicated test so future refactors don't accidentally couple this rule
// to runtime references.
func TestArtifactLayoutDriftLocal_TransientFile_StillCleanupRegardlessOfUnits(t *testing.T) {
	td := t.TempDir()
	for _, d := range []string{"pki", "etcd"} {
		if err := os.MkdirAll(filepath.Join(td, d), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(td, "day0-install.jsonl"), []byte("{}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := artifactStateRootPath
	artifactStateRootPath = td
	t.Cleanup(func() { artifactStateRootPath = old })

	stubUnitDir(t, "[Service]\nExecStart=/bin/true\n")

	findings := (artifactLayoutDriftLocal{}).Evaluate(nil, testConfig())
	found := false
	for _, f := range findings {
		if strings.Contains(f.Summary, "day0-install.jsonl") {
			found = true
			if f.Severity != cluster_doctorpb.Severity_SEVERITY_INFO {
				t.Errorf("day0-install.jsonl should be INFO cleanup; got severity=%v", f.Severity)
			}
		}
	}
	if !found {
		t.Errorf("day0-install.jsonl must always be classified as cleanup-transient")
	}
}

// TestPathsPinnedBySystemdUnits_ParsesWorkingDirectoryAndMkdir is a unit
// test on the scanner helper itself. Covers: optional `-` prefix on
// WorkingDirectory, mkdir flag variants, ExecStartPre with quoted shell
// invocations, and paths outside stateRoot which must NOT appear.
func TestPathsPinnedBySystemdUnits_ParsesWorkingDirectoryAndMkdir(t *testing.T) {
	stateRoot := "/var/lib/globular"
	td := t.TempDir()
	unit := `[Unit]
Description=Stub

[Service]
WorkingDirectory=-/var/lib/globular/foo
ExecStartPre=+/bin/sh -c 'mkdir -p /var/lib/globular/bar && chown globular:globular /var/lib/globular/bar'
ExecStartPre=+/bin/mkdir -p /var/lib/globular/baz
ExecStart=/usr/lib/globular/bin/foo
# Paths outside stateRoot must NOT appear in the pinned set.
WorkingDirectory=/etc/somewhere/else
ExecStartPre=mkdir -p /tmp/elsewhere
`
	if err := os.WriteFile(filepath.Join(td, "globular-foo.service"), []byte(unit), 0o644); err != nil {
		t.Fatal(err)
	}
	old := systemdUnitDirsForLayoutDrift
	systemdUnitDirsForLayoutDrift = []string{td}
	t.Cleanup(func() { systemdUnitDirsForLayoutDrift = old })

	pinned := pathsPinnedBySystemdUnits(stateRoot)
	for _, want := range []string{
		"/var/lib/globular/foo",
		"/var/lib/globular/bar",
		"/var/lib/globular/baz",
	} {
		if !pinned[want] {
			t.Errorf("expected %q in pinned set; got %v", want, pinned)
		}
	}
	for _, bad := range []string{
		"/etc/somewhere/else",
		"/tmp/elsewhere",
	} {
		if pinned[bad] {
			t.Errorf("path %q outside stateRoot must NOT be in pinned set; got %v", bad, pinned)
		}
	}
}

// TestPathsPinnedBySystemdUnits_MissingUnitDirIsSafe proves the scanner
// fails open (returns empty set) when the configured unit dirs do not
// exist. Caller treats absence as "no opinion → may flag as cleanup,"
// which is correct: in a test or container without systemd, the rule
// should still emit its normal verdicts.
func TestPathsPinnedBySystemdUnits_MissingUnitDirIsSafe(t *testing.T) {
	old := systemdUnitDirsForLayoutDrift
	systemdUnitDirsForLayoutDrift = []string{"/nonexistent/path/should/not/exist/anywhere"}
	t.Cleanup(func() { systemdUnitDirsForLayoutDrift = old })

	pinned := pathsPinnedBySystemdUnits("/var/lib/globular")
	if len(pinned) != 0 {
		t.Errorf("expected empty pinned set for missing unit dir; got %v", pinned)
	}
}
