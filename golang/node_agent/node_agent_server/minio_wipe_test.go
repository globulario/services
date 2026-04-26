package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/globulario/services/golang/config"
)

// ── transitionAuthorizesWipe ──────────────────────────────────────────────────

func TestTransitionAuthorizesWipe_NilTransition_NotAuthorized(t *testing.T) {
	state := &config.ObjectStoreDesiredState{Generation: 2}
	ok, reason := transitionAuthorizesWipe(nil, state, "10.0.0.1")
	if ok {
		t.Fatalf("expected not authorized for nil transition, got true")
	}
	if reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestTransitionAuthorizesWipe_GenerationMismatch_NotAuthorized(t *testing.T) {
	// Transition is for gen 1 but desired is gen 2 (stale record).
	transition := &config.TopologyTransition{
		Generation:    1,
		IsDestructive: true,
		Approved:      true,
		AffectedPaths: map[string]string{"10.0.0.1": "/data"},
		CreatedAt:     time.Now(),
	}
	state := &config.ObjectStoreDesiredState{
		Generation: 2,
		NodePaths:  map[string]string{"10.0.0.1": "/data"},
	}
	ok, reason := transitionAuthorizesWipe(transition, state, "10.0.0.1")
	if ok {
		t.Fatalf("expected not authorized for generation mismatch, got true; reason: %s", reason)
	}
}

func TestTransitionAuthorizesWipe_NotDestructive_NotAuthorized(t *testing.T) {
	transition := &config.TopologyTransition{
		Generation:    3,
		IsDestructive: false,
		Approved:      true,
		AffectedPaths: map[string]string{"10.0.0.1": "/data"},
		CreatedAt:     time.Now(),
	}
	state := &config.ObjectStoreDesiredState{
		Generation: 3,
		NodePaths:  map[string]string{"10.0.0.1": "/data"},
	}
	ok, _ := transitionAuthorizesWipe(transition, state, "10.0.0.1")
	if ok {
		t.Fatal("expected not authorized for non-destructive transition")
	}
}

func TestTransitionAuthorizesWipe_NotApproved_NotAuthorized(t *testing.T) {
	transition := &config.TopologyTransition{
		Generation:    3,
		IsDestructive: true,
		Approved:      false, // operator did not confirm --i-understand-data-reset
		AffectedPaths: map[string]string{"10.0.0.1": "/data"},
		CreatedAt:     time.Now(),
	}
	state := &config.ObjectStoreDesiredState{
		Generation: 3,
		NodePaths:  map[string]string{"10.0.0.1": "/data"},
	}
	ok, _ := transitionAuthorizesWipe(transition, state, "10.0.0.1")
	if ok {
		t.Fatal("expected not authorized when Approved=false")
	}
}

func TestTransitionAuthorizesWipe_NodeNotInPlan_NotAuthorized(t *testing.T) {
	// This node is not in the wipe plan (another node's path changed).
	transition := &config.TopologyTransition{
		Generation:    3,
		IsDestructive: true,
		Approved:      true,
		AffectedPaths: map[string]string{"10.0.0.2": "/data"}, // only node-2 affected
		CreatedAt:     time.Now(),
	}
	state := &config.ObjectStoreDesiredState{
		Generation: 3,
		NodePaths:  map[string]string{"10.0.0.1": "/data", "10.0.0.2": "/data"},
	}
	ok, _ := transitionAuthorizesWipe(transition, state, "10.0.0.1")
	if ok {
		t.Fatal("expected not authorized when node not in affected paths")
	}
}

func TestTransitionAuthorizesWipe_PathMismatch_NotAuthorized(t *testing.T) {
	// Transition says /old but desired says /new — stale record.
	transition := &config.TopologyTransition{
		Generation:    3,
		IsDestructive: true,
		Approved:      true,
		AffectedPaths: map[string]string{"10.0.0.1": "/data/old"},
		CreatedAt:     time.Now(),
	}
	state := &config.ObjectStoreDesiredState{
		Generation: 3,
		NodePaths:  map[string]string{"10.0.0.1": "/data/new"}, // path already changed again
	}
	ok, reason := transitionAuthorizesWipe(transition, state, "10.0.0.1")
	if ok {
		t.Fatalf("expected not authorized for path mismatch, got true; reason: %s", reason)
	}
}

func TestTransitionAuthorizesWipe_StandaloneToDistributed_Authorized(t *testing.T) {
	// Standalone → distributed with approved transition.
	transition := &config.TopologyTransition{
		Generation:    2,
		IsDestructive: true,
		Approved:      true,
		AffectedPaths: map[string]string{
			"10.0.0.1": "/var/lib/globular/minio",
			"10.0.0.2": "/var/lib/globular/minio",
		},
		Reasons:   []string{"standalone → distributed transition"},
		CreatedAt: time.Now(),
	}
	state := &config.ObjectStoreDesiredState{
		Generation: 2,
		Mode:       config.ObjectStoreModeDistributed,
		NodePaths: map[string]string{
			"10.0.0.1": "/var/lib/globular/minio",
			"10.0.0.2": "/var/lib/globular/minio",
		},
	}
	ok, _ := transitionAuthorizesWipe(transition, state, "10.0.0.1")
	if !ok {
		t.Fatal("expected authorized for standalone→distributed with approved transition")
	}
	ok2, _ := transitionAuthorizesWipe(transition, state, "10.0.0.2")
	if !ok2 {
		t.Fatal("expected authorized for node-2 in same transition")
	}
}

func TestTransitionAuthorizesWipe_DistributedPathChange_Authorized(t *testing.T) {
	// distributed → distributed path change with approved transition.
	transition := &config.TopologyTransition{
		Generation:    5,
		IsDestructive: true,
		Approved:      true,
		AffectedPaths: map[string]string{"10.0.0.3": "/data/ssd"},
		Reasons:       []string{"node path change /data/old → /data/ssd"},
		CreatedAt:     time.Now(),
	}
	state := &config.ObjectStoreDesiredState{
		Generation: 5,
		Mode:       config.ObjectStoreModeDistributed,
		NodePaths: map[string]string{
			"10.0.0.1": "/data",
			"10.0.0.2": "/data",
			"10.0.0.3": "/data/ssd",
		},
	}
	ok, _ := transitionAuthorizesWipe(transition, state, "10.0.0.3")
	if !ok {
		t.Fatal("expected authorized for distributed→distributed path change with approved transition")
	}
	// Other nodes not in the wipe plan should NOT be wiped.
	ok2, _ := transitionAuthorizesWipe(transition, state, "10.0.0.1")
	if ok2 {
		t.Fatal("node-1 not in wipe plan should not be authorized")
	}
}

// ── clearMinioSysForModeChange filesystem tests ───────────────────────────────

func TestClearMinioSysForModeChange_SingleDrive_Idempotent(t *testing.T) {
	dir := t.TempDir()
	dataDir := filepath.Join(dir, "data")
	minioSys := filepath.Join(dataDir, ".minio.sys")

	// Create .minio.sys
	if err := os.MkdirAll(minioSys, 0o755); err != nil {
		t.Fatal(err)
	}
	// Verify it exists
	if _, err := os.Stat(minioSys); err != nil {
		t.Fatal(".minio.sys should exist before wipe")
	}

	srv := &NodeAgentServer{}
	state := &config.ObjectStoreDesiredState{
		DrivesPerNode: 1,
		NodePaths:     map[string]string{"10.0.0.1": dir},
	}
	srv.clearMinioSysForModeChange(state, "10.0.0.1")

	if _, err := os.Stat(minioSys); !os.IsNotExist(err) {
		t.Fatal(".minio.sys should be gone after wipe")
	}

	// Call again — must be idempotent (no panic when already absent).
	srv.clearMinioSysForModeChange(state, "10.0.0.1")
}

func TestClearMinioSysForModeChange_MultiDrive_WipesAll(t *testing.T) {
	dir := t.TempDir()

	// Create .minio.sys in data1 and data2
	for _, d := range []string{"data1", "data2"} {
		sys := filepath.Join(dir, d, ".minio.sys")
		if err := os.MkdirAll(sys, 0o755); err != nil {
			t.Fatal(err)
		}
	}

	srv := &NodeAgentServer{}
	state := &config.ObjectStoreDesiredState{
		DrivesPerNode: 2,
		NodePaths:     map[string]string{"10.0.0.1": dir},
	}
	srv.clearMinioSysForModeChange(state, "10.0.0.1")

	for _, d := range []string{"data1", "data2"} {
		sys := filepath.Join(dir, d, ".minio.sys")
		if _, err := os.Stat(sys); !os.IsNotExist(err) {
			t.Fatalf(".minio.sys in %s should be gone after wipe", d)
		}
	}
}

// ── manual-edit override (Item 6) ────────────────────────────────────────────
//
// Verifies that atomicWriteIfChanged always restores the desired content when
// on-disk content was manually edited. This is the mechanism that enforces
// "etcd desired state wins over manual edits".

func TestAtomicWriteIfChanged_OverwritesManualEdit(t *testing.T) {
	path := filepath.Join(t.TempDir(), "minio.env")

	// Write desired content first.
	desiredContent := []byte("MINIO_VOLUMES=https://10.0.0.1:9000/data/data{1...2}\nMINIO_SITE_NAME=globular\n")
	if _, err := atomicWriteIfChanged(path, desiredContent, 0o640); err != nil {
		t.Fatal(err)
	}

	// Simulate manual edit (operator edits the file directly).
	manualContent := []byte("MINIO_VOLUMES=https://10.0.0.1:9000/data/manualEdit\nMINIO_SITE_NAME=hacked\n")
	if err := os.WriteFile(path, manualContent, 0o640); err != nil {
		t.Fatal(err)
	}

	// Reconcile call: atomicWriteIfChanged sees content differs → overwrites.
	changed, err := atomicWriteIfChanged(path, desiredContent, 0o640)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed=true when manual edit present")
	}
	// Verify desired content restored.
	got, _ := os.ReadFile(path)
	if string(got) != string(desiredContent) {
		t.Errorf("expected desired content restored, got: %q", got)
	}
}

// ── atomicWriteIfChanged ──────────────────────────────────────────────────────

func TestAtomicWriteIfChanged_WritesWhenDifferent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.env")
	changed, err := atomicWriteIfChanged(path, []byte("NEW=value\n"), 0o640)
	if err != nil {
		t.Fatal(err)
	}
	if !changed {
		t.Error("expected changed=true for new file")
	}
	got, _ := os.ReadFile(path)
	if string(got) != "NEW=value\n" {
		t.Errorf("unexpected content: %q", got)
	}
}

func TestAtomicWriteIfChanged_NoWriteWhenSame(t *testing.T) {
	path := filepath.Join(t.TempDir(), "test.env")
	content := []byte("SAME=value\n")
	if _, err := atomicWriteIfChanged(path, content, 0o640); err != nil {
		t.Fatal(err)
	}
	// Second call with same content should return changed=false.
	changed, err := atomicWriteIfChanged(path, content, 0o640)
	if err != nil {
		t.Fatal(err)
	}
	if changed {
		t.Error("expected changed=false when content unchanged")
	}
}
