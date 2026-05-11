package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/ai_memory/ai_memorypb"
	"github.com/globulario/services/golang/opsknowledge"
)

func TestOpsKnowledgeEntryToMemory_StampsSeedMetadata(t *testing.T) {
	e := opsknowledge.Entry{
		ID:    "ops.test.fixture",
		Type:  "REFERENCE",
		Title: "fixture",
		Tags:  []string{"day-1", "test"},
	}
	mem := opsKnowledgeEntryToMemory(e, "abc123", "v1.2.3", "globular.internal")

	if mem.GetId() != "ops.test.fixture" {
		t.Errorf("Id mismatch: %s", mem.GetId())
	}
	if mem.GetProject() != opsKnowledgeProject {
		t.Errorf("Project mismatch: %s", mem.GetProject())
	}
	if mem.GetType() != ai_memorypb.MemoryType_REFERENCE {
		t.Errorf("Type mismatch: %v", mem.GetType())
	}
	if mem.GetAgentId() != opsKnowledgeSeederAgent {
		t.Errorf("AgentId mismatch: %s", mem.GetAgentId())
	}
	if mem.GetClusterId() != "globular.internal" {
		t.Errorf("ClusterId mismatch: %s", mem.GetClusterId())
	}

	md := mem.GetMetadata()
	if md["source"] != "seed" {
		t.Errorf("metadata.source must be seed, got %q", md["source"])
	}
	if md["immutable"] != "true" {
		t.Errorf("metadata.immutable must be true, got %q", md["immutable"])
	}
	if md["seed_sha256"] != "abc123" {
		t.Errorf("metadata.seed_sha256 mismatch: %s", md["seed_sha256"])
	}
	if md["seed_version"] != "v1.2.3" {
		t.Errorf("metadata.seed_version mismatch: %s", md["seed_version"])
	}

	// Auto-stamps the seed tag.
	if !containsString(mem.GetTags(), "seed") {
		t.Errorf("tags must include 'seed', got %v", mem.GetTags())
	}
}

func TestOpsKnowledgeEntryToMemory_DefaultTypeIsReference(t *testing.T) {
	e := opsknowledge.Entry{
		ID:    "ops.test.fixture",
		Type:  "BOGUS_TYPE_NOT_IN_PROTO",
		Title: "fixture",
		Tags:  []string{"day-1"},
	}
	mem := opsKnowledgeEntryToMemory(e, "h", "v", "c")
	if mem.GetType() != ai_memorypb.MemoryType_REFERENCE {
		t.Errorf("unknown type must default to REFERENCE, got %v", mem.GetType())
	}
}

func TestOpsKnowledgeEntryToMemory_DoesNotDuplicateSeedTag(t *testing.T) {
	e := opsknowledge.Entry{
		ID:   "ops.test.fixture",
		Tags: []string{"day-1", "seed", "extra"},
	}
	mem := opsKnowledgeEntryToMemory(e, "h", "v", "c")
	count := 0
	for _, t := range mem.GetTags() {
		if t == "seed" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("seed tag must appear exactly once, got %d in %v", count, mem.GetTags())
	}
}

func TestReadBundleSeedVersion_MissingManifestFallback(t *testing.T) {
	prev := opsKnowledgeBundlePath
	t.Cleanup(func() { setBundlePathForTest(prev) })
	setBundlePathForTest(filepath.Join(t.TempDir(), "no-such"))
	if got := readBundleSeedVersion(); got != "auto-seeded" {
		t.Errorf("missing manifest must fall back, got %q", got)
	}
}

func TestReadBundleSeedVersion_AppendsBuildID(t *testing.T) {
	dir := t.TempDir()
	prev := opsKnowledgeBundlePath
	t.Cleanup(func() { setBundlePathForTest(prev) })
	setBundlePathForTest(dir)

	manifest := `{"version":"1.0.0","build_id":"abcdef1234567890"}`
	if err := os.WriteFile(filepath.Join(dir, "manifest.json"), []byte(manifest), 0o644); err != nil {
		t.Fatalf("write manifest: %v", err)
	}
	got := readBundleSeedVersion()
	want := "1.0.0+abcdef12"
	if got != want {
		t.Errorf("readBundleSeedVersion = %q, want %q", got, want)
	}
}

