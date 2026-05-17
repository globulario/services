package graph

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestFreshness_NoBuilds(t *testing.T) {
	g, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer g.Close()

	f := g.Freshness(context.Background(), "")
	if !f.Stale {
		t.Error("expected Stale=true when no graph builds exist")
	}
	if f.StaleReason == "" {
		t.Error("expected non-empty StaleReason")
	}
}

func TestFreshness_FreshGraph(t *testing.T) {
	g, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Insert a build record with current timestamp.
	if err := g.UpsertBuildRecord(ctx, "test-build-1", "/tmp/test", "abc123", "", BuildStats{}); err != nil {
		t.Fatalf("UpsertBuildRecord failed: %v", err)
	}

	f := g.Freshness(ctx, "")
	if f.Stale {
		t.Errorf("expected Stale=false for fresh graph, got StaleReason=%q", f.StaleReason)
	}
	if f.BuiltAt.IsZero() {
		t.Error("expected non-zero BuiltAt")
	}
}

func TestFreshness_KnowledgeNewerThanGraph(t *testing.T) {
	g, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Insert a build record 5 seconds in the past using the internal builds slice directly.
	past := time.Now().Add(-5 * time.Second).Unix()
	g.buildMu.Lock()
	g.builds = append(g.builds, &BuildRecord{
		ID:        "test-build-past",
		RepoRoot:  "/tmp/test",
		GitCommit: "abc123",
		CreatedAt: past,
	})
	g.buildMu.Unlock()

	// Create a temp docsDir with a knowledge file that is newer.
	docsDir := t.TempDir()
	knowledgeFile := filepath.Join(docsDir, "failure_modes.yaml")
	if err := os.WriteFile(knowledgeFile, []byte("failure_modes: []"), 0644); err != nil {
		t.Fatal(err)
	}
	// File was just created, so it's newer than the "past" build.

	f := g.Freshness(ctx, docsDir)
	if !f.Stale {
		t.Error("expected Stale=true when knowledge file is newer than graph build")
	}
	if f.StaleReason == "" {
		t.Error("expected non-empty StaleReason")
	}
}

func TestLatestBuildTime_Empty(t *testing.T) {
	g, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer g.Close()

	_, ok, err := g.LatestBuildTime(context.Background())
	if err != nil {
		t.Fatalf("LatestBuildTime error: %v", err)
	}
	if ok {
		t.Error("expected ok=false when no builds exist")
	}
}

func TestFreshness_HashIsComputedAndNonEmpty(t *testing.T) {
	g, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer g.Close()

	ctx := context.Background()
	if err := g.UpsertBuildRecord(ctx, "test-hash-build", "/tmp/test", "abc123", "", BuildStats{}); err != nil {
		t.Fatalf("UpsertBuildRecord failed: %v", err)
	}

	// Create a temp docsDir with at least one knowledge file.
	docsDir := t.TempDir()
	if err := os.WriteFile(filepath.Join(docsDir, "failure_modes.yaml"), []byte("failure_modes: []"), 0644); err != nil {
		t.Fatal(err)
	}

	f := g.Freshness(ctx, docsDir)
	if f.KnowledgeSourceHash == "" {
		t.Error("expected non-empty KnowledgeSourceHash when docsDir has knowledge files")
	}
	// Hash should be a 64-char hex SHA256.
	if len(f.KnowledgeSourceHash) != 64 {
		t.Errorf("KnowledgeSourceHash length = %d, want 64 (SHA256 hex)", len(f.KnowledgeSourceHash))
	}
}

func TestFreshness_StaleWhenMaxAgeExceeded(t *testing.T) {
	g, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer g.Close()

	ctx := context.Background()
	// Insert a build record that is 25 hours old.
	oldTs := time.Now().Add(-25 * time.Hour).Unix()
	g.buildMu.Lock()
	g.builds = append(g.builds, &BuildRecord{
		ID:        "old-build",
		RepoRoot:  "/r",
		GitCommit: "abc",
		CreatedAt: oldTs,
	})
	g.buildMu.Unlock()

	f := g.Freshness(ctx, "")
	if !f.Stale {
		t.Error("expected Stale=true when graph is 25 hours old")
	}
	if !f.MaxAgeExceeded {
		t.Error("expected MaxAgeExceeded=true when graph is 25 hours old")
	}
}

func TestFreshness_RebuildRecommendedWhenStale(t *testing.T) {
	g, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer g.Close()

	// No builds → stale.
	f := g.Freshness(context.Background(), "")
	if !f.RebuildRecommended {
		t.Error("expected RebuildRecommended=true when graph is stale")
	}
}

func TestLatestBuildTime_WithBuilds(t *testing.T) {
	g, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer g.Close()

	ctx := context.Background()
	now := time.Now().Unix()

	g.buildMu.Lock()
	for _, rec := range []struct {
		id        string
		createdAt int64
	}{
		{"b1", now - 100},
		{"b2", now},
	} {
		g.builds = append(g.builds, &BuildRecord{
			ID:        rec.id,
			RepoRoot:  "/r",
			GitCommit: "commit",
			CreatedAt: rec.createdAt,
		})
	}
	g.buildMu.Unlock()

	builtAt, ok, err := g.LatestBuildTime(ctx)
	if err != nil {
		t.Fatalf("LatestBuildTime error: %v", err)
	}
	if !ok {
		t.Fatal("expected ok=true")
	}
	if builtAt.Unix() != now {
		t.Errorf("expected latest build at %d, got %d", now, builtAt.Unix())
	}
}

// TestKnowledgeFiles_IncludesServicesYaml pins the canonical knowledge_files
// list so a regression that drops a graph-contributing YAML (e.g. services.yaml)
// from staleness tracking cannot land silently.
func TestKnowledgeFiles_IncludesServicesYaml(t *testing.T) {
	files := KnowledgeFiles()
	required := []string{
		"failure_modes.yaml",
		"invariants.yaml",
		"convergence_rules.yaml",
		"forbidden_fixes.yaml",
		"design_patterns.yaml",
		"patterns.yaml",
		"services.yaml",
	}
	have := make(map[string]bool, len(files))
	for _, f := range files {
		have[f] = true
	}
	for _, want := range required {
		if !have[want] {
			t.Errorf("KnowledgeFiles() missing %q — staleness check will not detect edits to this graph-contributing YAML",
				want)
		}
	}
}

// TestFreshness_StaleWhenServicesYamlNewerThanGraph: the regression that
// motivated this work — services.yaml is graph-contributing but the original
// canonical list omitted it, so edits did not mark the graph stale.
func TestFreshness_StaleWhenServicesYamlNewerThanGraph(t *testing.T) {
	g, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenMemory: %v", err)
	}
	defer g.Close()
	ctx := context.Background()

	past := time.Now().Add(-30 * time.Second).Unix()
	g.buildMu.Lock()
	g.builds = append(g.builds, &BuildRecord{
		ID:        "build-services-test",
		RepoRoot:  "/r",
		GitCommit: "commit",
		CreatedAt: past,
	})
	g.buildMu.Unlock()

	docsDir := t.TempDir()
	servicesPath := filepath.Join(docsDir, "services.yaml")
	if err := os.WriteFile(servicesPath, []byte("services:\n  - id: x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// File mtime is "now" — newer than the build at `past`.

	f := g.Freshness(ctx, docsDir)
	if !f.Stale {
		t.Errorf("expected Stale=true after editing services.yaml; got reason=%q", f.StaleReason)
	}
}
