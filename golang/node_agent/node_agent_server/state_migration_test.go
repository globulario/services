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
	// canonical has all fields populated — nothing to merge from legacy
	_ = os.WriteFile(canonical, []byte(`{"node_id":"new","join_token":"tok"}`), 0o600)
	_ = os.WriteFile(legacy, []byte(`{"node_id":"old","join_token":"old-tok"}`), 0o600)

	MigrateLegacyStatePathOnce(canonical, legacy)

	// canonical content must be preserved (canonical wins)
	s, err := loadNodeAgentState(canonical)
	if err != nil {
		t.Fatalf("canonical unreadable: %v", err)
	}
	if s.NodeID != "new" {
		t.Errorf("canonical node_id should win; got=%s", s.NodeID)
	}
	if s.JoinToken != "tok" {
		t.Errorf("canonical join_token should win; got=%s", s.JoinToken)
	}
	// legacy dir must be removed — no lingering layout_drift finding
	if _, err := os.Stat(filepath.Dir(legacy)); !os.IsNotExist(err) {
		t.Errorf("legacy dir must be removed after merge; err=%v", err)
	}
}

func TestMigrateLegacyStatePath_NodeAgent_BothExist_MergesEmptyFields(t *testing.T) {
	root := t.TempDir()
	legacy := filepath.Join(root, "nodeagent", "state.json")
	canonical := filepath.Join(root, "node-agent", "state.json")
	_ = os.MkdirAll(filepath.Dir(legacy), 0o755)
	_ = os.MkdirAll(filepath.Dir(canonical), 0o755)
	// canonical is missing join_token and controller_endpoint
	_ = os.WriteFile(canonical, []byte(`{"node_id":"new"}`), 0o600)
	_ = os.WriteFile(legacy, []byte(`{"node_id":"old","join_token":"tok","controller_endpoint":"ep"}`), 0o600)

	MigrateLegacyStatePathOnce(canonical, legacy)

	s, err := loadNodeAgentState(canonical)
	if err != nil {
		t.Fatalf("canonical unreadable: %v", err)
	}
	if s.NodeID != "new" {
		t.Errorf("canonical node_id should win; got=%s", s.NodeID)
	}
	if s.JoinToken != "tok" {
		t.Errorf("empty join_token should be filled from legacy; got=%s", s.JoinToken)
	}
	if s.ControllerEndpoint != "ep" {
		t.Errorf("empty controller_endpoint should be filled from legacy; got=%s", s.ControllerEndpoint)
	}
	if _, err := os.Stat(filepath.Dir(legacy)); !os.IsNotExist(err) {
		t.Errorf("legacy dir must be removed after merge; err=%v", err)
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
