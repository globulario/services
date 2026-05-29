package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Project O.3 state-path migration tests for node-agent. Mirrors the
// cluster-controller spec; behavior must be identical so the two services
// migrate symmetrically.

func TestMigrateLegacyStatePath_NodeAgent_OldExistsNewMissing(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "nodeagent", "state.json")
	canonical := filepath.Join(root, "node-agent", "state.json")
	_ = os.MkdirAll(filepath.Dir(legacy), 0o755)
	want := []byte(`{"node_id":"abc"}`)
	_ = os.WriteFile(legacy, want, 0o600)

	MigrateLegacyStatePathOnce(canonical, legacy)

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy should be removed after rename; err=%v", err)
	}
	got, err := os.ReadFile(canonical)
	if err != nil {
		t.Fatalf("canonical should exist: %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("content not preserved\n got=%s\nwant=%s", got, want)
	}
}

func TestMigrateLegacyStatePath_NodeAgent_BothExist_CanonicalWins(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "nodeagent", "state.json")
	canonical := filepath.Join(root, "node-agent", "state.json")
	_ = os.MkdirAll(filepath.Dir(legacy), 0o755)
	_ = os.MkdirAll(filepath.Dir(canonical), 0o755)
	_ = os.WriteFile(canonical, []byte(`{"node_id":"new"}`), 0o600)
	_ = os.WriteFile(legacy, []byte(`{"node_id":"old"}`), 0o600)

	MigrateLegacyStatePathOnce(canonical, legacy)

	got, _ := os.ReadFile(canonical)
	if string(got) != `{"node_id":"new"}` {
		t.Errorf("canonical should win; got=%s", got)
	}
	leg, err := os.ReadFile(legacy)
	if err != nil {
		t.Fatalf("legacy must be left for operator review: %v", err)
	}
	if string(leg) != `{"node_id":"old"}` {
		t.Errorf("legacy must not be overwritten; got=%s", leg)
	}
}

func TestMigrateLegacyStatePath_NodeAgent_NeitherExists(t *testing.T) {
	root := t.TempDir()
	canonical := filepath.Join(root, "node-agent", "state.json")
	legacy := filepath.Join(root, "nodeagent", "state.json")

	MigrateLegacyStatePathOnce(canonical, legacy)

	for _, p := range []string{canonical, legacy} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("no path should exist after no-op; saw err=%v for %s", err, p)
		}
	}
}

func TestMigrateLegacyStatePath_NodeAgent_Idempotent(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "nodeagent", "state.json")
	canonical := filepath.Join(root, "node-agent", "state.json")
	_ = os.MkdirAll(filepath.Dir(legacy), 0o755)
	want := []byte(`{"node_id":"x"}`)
	_ = os.WriteFile(legacy, want, 0o600)

	for i := 0; i < 3; i++ {
		MigrateLegacyStatePathOnce(canonical, legacy)
	}

	if _, err := os.Stat(legacy); !os.IsNotExist(err) {
		t.Errorf("legacy still present after first move; err=%v", err)
	}
	got, _ := os.ReadFile(canonical)
	if string(got) != string(want) {
		t.Errorf("content corrupted across repeated runs; got=%s want=%s", got, want)
	}
}
