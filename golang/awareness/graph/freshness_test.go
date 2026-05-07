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
		t.Fatalf("OpenInMemory: %v", err)
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
		t.Fatalf("OpenInMemory: %v", err)
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
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer g.Close()

	ctx := context.Background()

	// Insert a build record using the current UpsertBuildRecord call which uses time.Now().
	// We need to create the knowledge file AFTER the build record, so it's newer.
	// But since UpsertBuildRecord uses time.Now() internally, we can't directly set it to "past".
	// Instead, check what UpsertBuildRecord does with timestamps.
	// Looking at the query.go: it inserts created_at = excluded.created_at on conflict.
	// The created_at value comes from time.Now().Unix() inside UpsertBuildRecord.
	// So we insert first, then write knowledge files that are "newer" by modifying mtime.
	// We'll use the SQL directly to insert a past build.

	// Insert a build record 5 seconds in the past by manipulating DB directly.
	past := time.Now().Add(-5 * time.Second).Unix()
	_, dbErr := g.db.ExecContext(ctx,
		`INSERT INTO graph_builds (id, repo_root, git_commit, release_id, created_at, stats_json)
		 VALUES (?, ?, ?, ?, ?, ?)
		 ON CONFLICT(id) DO UPDATE SET created_at = excluded.created_at`,
		"test-build-past", "/tmp/test", "abc123", "", past, `{}`,
	)
	if dbErr != nil {
		t.Fatalf("direct insert failed: %v", dbErr)
	}

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
		t.Fatalf("OpenInMemory: %v", err)
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

func TestLatestBuildTime_WithBuilds(t *testing.T) {
	g, err := OpenMemory()
	if err != nil {
		t.Fatalf("OpenInMemory: %v", err)
	}
	defer g.Close()

	ctx := context.Background()
	now := time.Now().Unix()

	for _, rec := range []struct {
		id        string
		createdAt int64
	}{
		{"b1", now - 100},
		{"b2", now},
	} {
		_, dbErr := g.db.ExecContext(ctx,
			`INSERT INTO graph_builds (id, repo_root, git_commit, release_id, created_at, stats_json)
			 VALUES (?, ?, ?, ?, ?, ?)
			 ON CONFLICT(id) DO UPDATE SET created_at = excluded.created_at`,
			rec.id, "/r", "commit", "", rec.createdAt, `{}`,
		)
		if dbErr != nil {
			t.Fatalf("insert failed: %v", dbErr)
		}
	}

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
