package main

// staging_sweeper_test.go — D5a: the orphan-temp sweeper reaps crash-leaked
// .tmp.<uuid> files and NEVER deletes in-use content (committed blobs, committed
// manifests, or a recent/in-flight temp).

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/globulario/services/golang/storage_backend"
)

func writeAged(t *testing.T, root, rel, data string, age time.Duration) string {
	t.Helper()
	p := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}
	mt := time.Now().Add(-age)
	if err := os.Chtimes(p, mt, mt); err != nil {
		t.Fatal(err)
	}
	return p
}

// Test #3 (removes orphans) + #4 (never deletes in-use): old orphan temp goes;
// committed blob, committed manifest, and a recent in-flight temp all stay.
func TestSweepOrphanTempBlobs_RemovesOldOrphansOnly(t *testing.T) {
	root := t.TempDir()
	srv := &server{localStorage: storage_backend.NewOSStorage(root)}

	blob := writeAged(t, root, "artifacts/pkg%1.0.0%linux%1.bin", "BINARY", 2*time.Hour)
	manifest := writeAged(t, root, "artifacts/pkg%1.0.0%linux%1.manifest.json", "{}", 2*time.Hour)
	oldOrphan := writeAged(t, root, "artifacts/pkg%1.0.0%linux%1.bin.tmp.f47ac10b-58cc-4372-a567-0e02b2c3d479", "HALF", 2*time.Hour)
	recentTemp := writeAged(t, root, "artifacts/pkg%1.0.0%linux%1.bin.tmp.aaaaaaaa-bbbb-cccc-dddd-eeeeeeeeeeee", "INFLIGHT", 1*time.Minute)

	removed := srv.sweepOrphanTempBlobs(context.Background(), time.Now())
	if removed != 1 {
		t.Fatalf("expected exactly 1 orphan removed, got %d", removed)
	}
	if _, err := os.Stat(oldOrphan); !os.IsNotExist(err) {
		t.Fatal("old orphan temp must be removed")
	}
	for _, p := range []string{blob, manifest, recentTemp} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("in-use/recent file must be preserved: %s (%v)", p, err)
		}
	}
}

// Test #4 hardened: a directory of only committed blobs/manifests (even very old)
// is never touched — the temp pattern, not age, is the gate against committed content.
func TestSweepOrphanTempBlobs_NeverTouchesCommittedContent(t *testing.T) {
	root := t.TempDir()
	srv := &server{localStorage: storage_backend.NewOSStorage(root)}
	blob := writeAged(t, root, "artifacts/x.bin", "B", 3*time.Hour)
	manifest := writeAged(t, root, "artifacts/x.manifest.json", "{}", 3*time.Hour)

	if removed := srv.sweepOrphanTempBlobs(context.Background(), time.Now()); removed != 0 {
		t.Fatalf("committed blobs/manifests must never be swept, removed=%d", removed)
	}
	for _, p := range []string{blob, manifest} {
		if _, err := os.Stat(p); err != nil {
			t.Fatalf("committed file was removed: %s", p)
		}
	}
}
