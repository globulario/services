package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Project O.3 state-path migration tests.
//
// Spec:
//   1. old exists, new missing → migration moves file, content preserved.
//   2. both exist               → canonical wins, old left in place, warn.
//   3. neither exists           → no-op, no error.
//   4. canonical parent missing → parent created safely.
//   5. repeated startup         → idempotent, no duplicate move.

func TestMigrateLegacyStatePath_OldExistsNewMissing(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "clustercontroller", "state.json")
	canonical := filepath.Join(root, "cluster-controller", "state.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	want := []byte(`{"cluster_id":"globular.internal"}`)
	if err := os.WriteFile(legacy, want, 0o600); err != nil {
		t.Fatal(err)
	}

	MigrateLegacyStatePathOnce(canonical, legacy)

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy file should be removed after rename; err=%v", err)
	}
	got, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("canonical should exist after migration: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content not preserved\n got=%s\nwant=%s", got, want)
	}
}

func TestMigrateLegacyStatePath_BothExist_CanonicalWins(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "clustercontroller", "state.json")
	canonical := filepath.Join(root, "cluster-controller", "state.json")
	_ = os.MkdirAll(filepath.Dir(legacy), 0o755)
	_ = os.MkdirAll(filepath.Dir(canonical), 0o755)
	canonicalContent := []byte(`{"cluster_id":"new"}`)
	legacyContent := []byte(`{"cluster_id":"old"}`)
	if err := os.WriteFile(canonical, canonicalContent, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, legacyContent, 0o600); err != nil {
		t.Fatal(err)
	}

	MigrateLegacyStatePathOnce(canonical, legacy)

	got, _ := os.ReadFile(canonical)
	if string(got) != string(canonicalContent) {
		t.Errorf("canonical should win; got=%s want=%s", got, canonicalContent)
	}
	leg, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatalf("legacy must be left in place for operator review: %v", err)
	}
	if string(leg) != string(legacyContent) {
		t.Errorf("legacy content must not be overwritten; got=%s want=%s", leg, legacyContent)
	}
}

func TestMigrateLegacyStatePath_NeitherExists(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, "cluster-controller", "state.json")
	legacy := filepath.Join(root, "clustercontroller", "state.json")

	MigrateLegacyStatePathOnce(canonical, legacy)

	for _, p := range []string{canonical, legacy} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("no path should exist after no-op migration; saw err=%v for %s", err, p)
		}
	}
}

func TestMigrateLegacyStatePath_ParentDirCreated(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "clustercontroller", "state.json")
	canonical := filepath.Join(root, "deep", "nested", "cluster-controller", "state.json")
	if err := os.MkdirAll(filepath.Dir(legacy), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacy, []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	MigrateLegacyStatePathOnce(canonical, legacy)

	if _, err := os.Stat(canonical); err != nil {
		t.Fatalf("canonical not created under missing parent dir: %v", err)
	}
	parent, err := os.Stat(filepath.Dir(canonical))
	if err != nil {
		t.Fatalf("parent not stat-able: %v", err)
	}
	if parent.Mode().Perm() != 0o750 {
		t.Errorf("parent should be 0750; got %v", parent.Mode().Perm())
	}
}

func TestMigrateLegacyStatePath_Idempotent(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "clustercontroller", "state.json")
	canonical := filepath.Join(root, "cluster-controller", "state.json")
	_ = os.MkdirAll(filepath.Dir(legacy), 0o755)
	want := []byte(`{"cluster_id":"x"}`)
	_ = os.WriteFile(legacy, want, 0o600)

	for i := 0; i < 3; i++ {
		MigrateLegacyStatePathOnce(canonical, legacy)
	}

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy still present after first move; err=%v", err)
	}
	got, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(want) {
		t.Errorf("content corrupted across repeated runs; got=%s want=%s", got, want)
	}
}

func TestMigrateLegacyStatePath_EmptyArgs_NoOp(t *testing.T) {
	MigrateLegacyStatePathOnce("", "/tmp/x")
	MigrateLegacyStatePathOnce("/tmp/x", "")
	MigrateLegacyStatePathOnce("/tmp/x", "/tmp/x") // same path
	// Should not panic or error.
}
