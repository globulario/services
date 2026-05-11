package rules

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/globulario/services/golang/cluster_doctor/cluster_doctor_server/collector"
	cluster_doctorpb "github.com/globulario/services/golang/cluster_doctor/cluster_doctorpb"
	"github.com/globulario/services/golang/opsknowledge"
)

// withBundleDir swaps the package-level current-bundle path for the
// duration of one test, restoring it on cleanup.
func withBundleDir(t *testing.T, dir string) {
	t.Helper()
	prev := awarenessBundleCurrentPath
	awarenessBundleCurrentPath = dir
	t.Cleanup(func() { awarenessBundleCurrentPath = prev })
}

// validSeedYAML returns a minimal-but-valid ops-knowledge file with one
// entry plus its canonical hash.
func validSeedYAML(t *testing.T) (string, string) {
	t.Helper()
	yaml := `schema_version: 1
file_kind: stage
metadata:
  title: Test
  description: Test fixture
entries:
  - id: ops.test.fixture
    type: REFERENCE
    title: Test fixture
    tags: [day-1, test]
    applies_when:
      cluster_phases: [day-1]
    content: "test content for the fixture"
    provenance:
      source: seed
      immutable: true
`
	tmp := filepath.Join(t.TempDir(), "fixture.yaml")
	if err := os.WriteFile(tmp, []byte(yaml), 0o644); err != nil {
		t.Fatalf("write fixture: %v", err)
	}
	f, err := opsknowledge.LoadFile(tmp)
	if err != nil {
		t.Fatalf("load fixture: %v", err)
	}
	hash, err := opsknowledge.HashEntry(f.Entries[0])
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	return yaml, hash
}

// makeBundle builds a minimal bundle directory layout with a manifest
// and ops-knowledge payload. Returns the bundle dir.
func makeBundle(t *testing.T, yaml, hash string, mutateYAML func(string) string, opsKnowledgeEntries []bundleOpsKnowledgeManifest) string {
	t.Helper()
	dir := t.TempDir()

	if mutateYAML != nil {
		yaml = mutateYAML(yaml)
	}
	if yaml != "" {
		opsDir := filepath.Join(dir, "ops-knowledge", "stages")
		if err := os.MkdirAll(opsDir, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
		if err := os.WriteFile(filepath.Join(opsDir, "fixture.yaml"), []byte(yaml), 0o644); err != nil {
			t.Fatalf("write yaml: %v", err)
		}
	}

	if opsKnowledgeEntries == nil {
		opsKnowledgeEntries = []bundleOpsKnowledgeManifest{{
			ID:         "ops.test.fixture",
			FilePath:   "stages/fixture.yaml",
			Type:       "REFERENCE",
			Title:      "Test fixture",
			SeedSHA256: hash,
		}}
	}
	manifest := bundleManifest{
		Name:                "globular-awareness-bundle",
		BuildID:             "test-build-id",
		Version:             "test-1.0.0",
		OpsKnowledgeEntries: opsKnowledgeEntries,
	}
	mb, err := json.Marshal(manifest)
	if err != nil {
		t.Fatalf("marshal manifest: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), mb, 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	return dir
}

func TestOpsKnowledgeSeedIntegrity_NoBundle_Silent(t *testing.T) {
	withBundleDir(t, filepath.Join(t.TempDir(), "does-not-exist"))
	got := opsKnowledgeSeedIntegrity{}.Evaluate(&collector.Snapshot{}, Config{})
	if len(got) != 0 {
		t.Fatalf("missing bundle dir must produce 0 findings, got %d", len(got))
	}
}

func TestOpsKnowledgeSeedIntegrity_Healthy_NoFindings(t *testing.T) {
	yaml, hash := validSeedYAML(t)
	withBundleDir(t, makeBundle(t, yaml, hash, nil, nil))
	got := opsKnowledgeSeedIntegrity{}.Evaluate(&collector.Snapshot{}, Config{})
	if len(got) != 0 {
		t.Fatalf("healthy bundle must produce 0 findings, got %d: %+v", len(got), got)
	}
}

func TestOpsKnowledgeSeedIntegrity_NoEntries_Warn(t *testing.T) {
	withBundleDir(t, makeBundle(t, "", "", nil, []bundleOpsKnowledgeManifest{}))
	got := opsKnowledgeSeedIntegrity{}.Evaluate(&collector.Snapshot{}, Config{})
	if len(got) != 1 {
		t.Fatalf("expected exactly 1 finding for empty seed, got %d", len(got))
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN, got %v", got[0].Severity)
	}
	if !strings.Contains(got[0].Summary, "operational-knowledge") {
		t.Errorf("summary should reference operational-knowledge: %q", got[0].Summary)
	}
}

func TestOpsKnowledgeSeedIntegrity_DriftedHash_Error(t *testing.T) {
	yaml, hash := validSeedYAML(t)
	// Mutate the YAML's content but keep the manifest's declared hash —
	// the recompute should diverge.
	mutated := strings.Replace(yaml, "test content for the fixture", "MUTATED CONTENT", 1)
	withBundleDir(t, makeBundle(t, mutated, hash, nil, nil))
	got := opsKnowledgeSeedIntegrity{}.Evaluate(&collector.Snapshot{}, Config{})
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for drift, got %d", len(got))
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("expected ERROR, got %v", got[0].Severity)
	}
	if !strings.Contains(got[0].Summary, "drifted") {
		t.Errorf("summary should call out drift: %q", got[0].Summary)
	}
}

func TestOpsKnowledgeSeedIntegrity_MissingFile_Error(t *testing.T) {
	yaml, hash := validSeedYAML(t)
	dir := makeBundle(t, yaml, hash, nil, nil)
	// Remove the file the manifest expects.
	if err := os.Remove(filepath.Join(dir, "ops-knowledge", "stages", "fixture.yaml")); err != nil {
		t.Fatalf("remove: %v", err)
	}
	withBundleDir(t, dir)
	got := opsKnowledgeSeedIntegrity{}.Evaluate(&collector.Snapshot{}, Config{})
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for missing file, got %d", len(got))
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("expected ERROR, got %v", got[0].Severity)
	}
	if !strings.Contains(got[0].Summary, "missing") {
		t.Errorf("summary should call out missing: %q", got[0].Summary)
	}
}

func TestOpsKnowledgeSeedIntegrity_AiMemoryMissing_Warn(t *testing.T) {
	yaml, hash := validSeedYAML(t)
	withBundleDir(t, makeBundle(t, yaml, hash, nil, nil))
	// ai-memory queried (non-nil map) but does not have the entry the
	// manifest declares — auto-seeder hasn't run yet.
	snap := &collector.Snapshot{
		OpsKnowledgeMemoryEntries: map[string]string{},
	}
	got := opsKnowledgeSeedIntegrity{}.Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for ai-memory missing, got %d", len(got))
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_WARN {
		t.Errorf("expected WARN for memory-missing-only, got %v", got[0].Severity)
	}
	if !strings.Contains(got[0].Summary, "ai-memory diverges") {
		t.Errorf("summary should call out ai-memory: %q", got[0].Summary)
	}
	if got[0].EntityRef != "ai_memory" {
		t.Errorf("EntityRef should be ai_memory, got %q", got[0].EntityRef)
	}
}

func TestOpsKnowledgeSeedIntegrity_AiMemoryDrifted_Error(t *testing.T) {
	yaml, hash := validSeedYAML(t)
	withBundleDir(t, makeBundle(t, yaml, hash, nil, nil))
	snap := &collector.Snapshot{
		OpsKnowledgeMemoryEntries: map[string]string{
			"ops.test.fixture": "WRONG_HASH",
		},
	}
	got := opsKnowledgeSeedIntegrity{}.Evaluate(snap, Config{})
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for memory drift, got %d", len(got))
	}
	if got[0].Severity != cluster_doctorpb.Severity_SEVERITY_ERROR {
		t.Errorf("expected ERROR for memory drift, got %v", got[0].Severity)
	}
	if !strings.Contains(got[0].Summary, "drifted") {
		t.Errorf("summary should call out drift: %q", got[0].Summary)
	}
}

func TestOpsKnowledgeSeedIntegrity_AiMemoryHealthy_NoFindings(t *testing.T) {
	yaml, hash := validSeedYAML(t)
	withBundleDir(t, makeBundle(t, yaml, hash, nil, nil))
	snap := &collector.Snapshot{
		OpsKnowledgeMemoryEntries: map[string]string{
			"ops.test.fixture": hash, // matches manifest
		},
	}
	got := opsKnowledgeSeedIntegrity{}.Evaluate(snap, Config{})
	if len(got) != 0 {
		t.Fatalf("healthy bundle + healthy memory must produce 0 findings, got %d: %+v", len(got), got)
	}
}

func TestOpsKnowledgeSeedIntegrity_NilMemoryMap_BundleOnly(t *testing.T) {
	yaml, hash := validSeedYAML(t)
	withBundleDir(t, makeBundle(t, yaml, hash, nil, nil))
	// nil OpsKnowledgeMemoryEntries means the collector did not query
	// ai-memory — fall back to bundle-only verification.
	snap := &collector.Snapshot{}
	got := opsKnowledgeSeedIntegrity{}.Evaluate(snap, Config{})
	if len(got) != 0 {
		t.Fatalf("nil memory map must skip drift check, got %d findings", len(got))
	}
}

func TestOpsKnowledgeSeedIntegrity_BothBundleAndMemoryDrift_TwoFindings(t *testing.T) {
	yaml, hash := validSeedYAML(t)
	mutated := strings.Replace(yaml, "test content for the fixture", "MUTATED", 1)
	withBundleDir(t, makeBundle(t, mutated, hash, nil, nil))
	snap := &collector.Snapshot{
		OpsKnowledgeMemoryEntries: map[string]string{
			"ops.test.fixture": "WRONG_HASH",
		},
	}
	got := opsKnowledgeSeedIntegrity{}.Evaluate(snap, Config{})
	if len(got) != 2 {
		t.Fatalf("bundle+memory both drifted should produce 2 findings, got %d", len(got))
	}
	// One bundle finding, one memory finding.
	var bundleFinding, memFinding *Finding
	for i := range got {
		if got[i].EntityRef == "ai_memory" {
			memFinding = &got[i]
		} else {
			bundleFinding = &got[i]
		}
	}
	if bundleFinding == nil || memFinding == nil {
		t.Fatalf("expected one bundle + one memory finding, got %+v", got)
	}
}

func TestOpsKnowledgeSeedIntegrity_BadManifest_Unknown(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte("{not json"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	withBundleDir(t, dir)
	got := opsKnowledgeSeedIntegrity{}.Evaluate(&collector.Snapshot{}, Config{})
	if len(got) != 1 {
		t.Fatalf("expected 1 finding for bad manifest, got %d", len(got))
	}
	if got[0].InvariantStatus != cluster_doctorpb.InvariantStatus_INVARIANT_UNKNOWN {
		t.Errorf("expected INVARIANT_UNKNOWN, got %v", got[0].InvariantStatus)
	}
	if got[0].CheckError == "" {
		t.Errorf("CheckError must be populated when the verdict is indeterminate")
	}
}
