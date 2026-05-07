package main

// globular:tested_by lkg_expansion

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/globular_service/lkg"
)

func setupXDSLKGDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	lkg.OverrideBaseDir(dir)
	t.Cleanup(func() { lkg.OverrideBaseDir("/var/lib/globular") })
	return dir
}

func validXDSConfigJSON(endpoints ...string) []byte {
	if len(endpoints) == 0 {
		endpoints = []string{"10.0.0.1:2379", "10.0.0.2:2379"}
	}
	cfg := xdsDesiredConfig{
		EtcdEndpoints: endpoints,
		SyncInterval:  5,
	}
	b, _ := json.Marshal(cfg)
	return b
}

// TestEtcdOutageServesFromLKG verifies that when xds/config.json is absent
// (simulating an etcd outage preventing the controller from delivering a new
// config), loadXDSConfigWithLKGPath falls back to the LKG record, restores the
// file atomically, and returns a valid config — not an error or empty state.
//
// Invariant: runtime.last_known_good_required_for_critical_consumers
func TestEtcdOutageServesFromLKG(t *testing.T) {
	dir := setupXDSLKGDir(t)
	configPath := filepath.Join(dir, "xds", "config.json")

	// Step 1: write a valid config file and call the loader — this stores it in LKG.
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(configPath, validXDSConfigJSON(), 0o644); err != nil {
		t.Fatalf("write initial config: %v", err)
	}
	cfg, source, err := loadXDSConfigWithLKGPath(configPath)
	if err != nil {
		t.Fatalf("initial load failed: %v", err)
	}
	if source != "file" {
		t.Errorf("initial load source = %q, want %q", source, "file")
	}
	if len(cfg.EtcdEndpoints) == 0 {
		t.Fatal("initial config has empty etcd_endpoints")
	}

	// Step 2: remove the config file (simulate etcd outage — controller cannot deliver new config).
	if err := os.Remove(configPath); err != nil {
		t.Fatalf("remove config: %v", err)
	}

	// Step 3: load again — must serve from LKG and restore the file.
	cfg2, source2, err2 := loadXDSConfigWithLKGPath(configPath)
	if err2 != nil {
		t.Fatalf("LKG load failed: %v", err2)
	}
	if source2 != "lkg" {
		t.Errorf("LKG load source = %q, want %q", source2, "lkg")
	}
	if cfg2 == nil || len(cfg2.EtcdEndpoints) == 0 {
		t.Error("LKG config has empty etcd_endpoints — state was reset to empty")
	}

	// Step 4: verify file was atomically restored (xds binary will find it).
	if _, statErr := os.Stat(configPath); os.IsNotExist(statErr) {
		t.Error("config file was not restored from LKG — xds binary would fail to start")
	}
}

// TestCorruptLKGRejectedPriorStateRetained verifies that a corrupt LKG record
// (checksum mismatch) is rejected: loadXDSConfigWithLKGPath returns ErrCorrupt
// and does NOT overwrite the valid file on disk with corrupt state.
//
// Invariant: runtime.last_known_good_required_for_critical_consumers
func TestCorruptLKGRejectedPriorStateRetained(t *testing.T) {
	dir := setupXDSLKGDir(t)
	configPath := filepath.Join(dir, "xds", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	// Write a valid config file and let it be stored in LKG.
	original := validXDSConfigJSON()
	if err := os.WriteFile(configPath, original, 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	if _, _, err := loadXDSConfigWithLKGPath(configPath); err != nil {
		t.Fatalf("initial load: %v", err)
	}

	// Corrupt the LKG file directly (simulate bit-flip or partial write).
	lkgFilePath := filepath.Join(dir, "xds", "config-last-known-good.json")
	if err := os.WriteFile(lkgFilePath, []byte("{corrupt json!!!}"), 0o644); err != nil {
		t.Fatalf("corrupt LKG: %v", err)
	}

	// Remove the config file to force the LKG path.
	if err := os.Remove(configPath); err != nil {
		t.Fatalf("remove config: %v", err)
	}

	// Attempt to load: LKG is corrupt, must return error.
	cfg, _, err := loadXDSConfigWithLKGPath(configPath)
	if err == nil {
		t.Error("expected error on corrupt LKG, got nil")
	}
	if cfg != nil {
		t.Error("expected nil config on corrupt LKG, got non-nil — corrupt state was applied")
	}

	// Verify the file was NOT restored with corrupt content.
	if _, statErr := os.Stat(configPath); !os.IsNotExist(statErr) {
		data, _ := os.ReadFile(configPath)
		t.Errorf("file should not exist after corrupt LKG rejection, but found: %s", data)
	}
}

// TestAtomicLKGWriteSurvivesRestart verifies that the LKG write is atomic
// and survives a simulated process restart: after storing a valid config in
// LKG and then loading with a missing file (new process, no file on disk),
// the restored config is correct and the file is written atomically.
//
// Invariant: runtime.last_known_good_required_for_critical_consumers
func TestAtomicLKGWriteSurvivesRestart(t *testing.T) {
	dir := setupXDSLKGDir(t)
	configPath := filepath.Join(dir, "xds", "config.json")
	if err := os.MkdirAll(filepath.Dir(configPath), 0o750); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	endpoints := []string{"10.10.0.11:2379", "10.10.0.12:2379", "10.10.0.13:2379"}
	if err := os.WriteFile(configPath, validXDSConfigJSON(endpoints...), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// First "process": load once to commit to LKG.
	cfg1, source1, err1 := loadXDSConfigWithLKGPath(configPath)
	if err1 != nil || source1 != "file" {
		t.Fatalf("pre-restart load: err=%v source=%q", err1, source1)
	}
	if len(cfg1.EtcdEndpoints) != len(endpoints) {
		t.Fatalf("pre-restart endpoint count = %d, want %d", len(cfg1.EtcdEndpoints), len(endpoints))
	}

	// Simulate restart: delete the config file (as if node rebooted without disk persistence).
	if err := os.Remove(configPath); err != nil {
		t.Fatalf("remove config: %v", err)
	}

	// Second "process": load without file — must restore from LKG.
	cfg2, source2, err2 := loadXDSConfigWithLKGPath(configPath)
	if err2 != nil {
		t.Fatalf("post-restart load: %v", err2)
	}
	if source2 != "lkg" {
		t.Errorf("post-restart source = %q, want %q", source2, "lkg")
	}
	if cfg2 == nil {
		t.Fatal("post-restart config is nil")
	}
	if len(cfg2.EtcdEndpoints) != len(endpoints) {
		t.Errorf("post-restart endpoint count = %d, want %d", len(cfg2.EtcdEndpoints), len(endpoints))
	}
	for i, ep := range endpoints {
		if i >= len(cfg2.EtcdEndpoints) || cfg2.EtcdEndpoints[i] != ep {
			t.Errorf("endpoint[%d]: got %q, want %q", i, cfg2.EtcdEndpoints[i], ep)
		}
	}

	// Verify atomic restore: .tmp file must not linger.
	if _, err := os.Stat(configPath + ".tmp"); !os.IsNotExist(err) {
		t.Error("stale .tmp file left after atomic restore — rename was not called")
	}

	// Verify the restored file is valid JSON readable by xds binary.
	restored, readErr := os.ReadFile(configPath)
	if readErr != nil {
		t.Fatalf("restored file unreadable: %v", readErr)
	}
	var check xdsDesiredConfig
	if err := json.Unmarshal(restored, &check); err != nil {
		t.Fatalf("restored file is invalid JSON: %v", err)
	}
	if len(check.EtcdEndpoints) == 0 {
		t.Error("restored file has empty etcd_endpoints")
	}
}
