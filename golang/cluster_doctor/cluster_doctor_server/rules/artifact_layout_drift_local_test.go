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
	if err := os.MkdirAll(filepath.Join(td, "authentication"), 0o755); err != nil {
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
