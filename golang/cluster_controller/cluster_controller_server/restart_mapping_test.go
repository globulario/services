package main

import (
	"testing"
)

// TestRestartActionsForChangedConfigs_NoChange verifies that no restart actions
// are emitted when rendered content is identical to the stored hashes.
func TestRestartActionsForChangedConfigs_NoChange(t *testing.T) {
	rendered := map[string]string{
		"/var/lib/globular/etcd/etcd.yaml": "name: node1\ndata-dir: /var/lib/globular/etcd\n",
	}
	// Compute the hashes as they would be stored after a dispatch.
	stored := HashRenderedConfigs(rendered)

	actions := restartActionsForChangedConfigs(stored, rendered)
	if len(actions) != 0 {
		t.Errorf("expected no restart actions when content unchanged, got %d: %v", len(actions), actions)
	}
}

// TestRestartActionsForChangedConfigs_Changed verifies that restart actions are
// emitted for the renderer whose output file changed.
func TestRestartActionsForChangedConfigs_Changed(t *testing.T) {
	original := "name: node1\n"
	updated := "name: node2\n"

	// Simulate old stored hash (from previous dispatch).
	oldRendered := map[string]string{
		"/var/lib/globular/etcd/etcd.yaml": original,
	}
	stored := HashRenderedConfigs(oldRendered)

	// New render has different content.
	newRendered := map[string]string{
		"/var/lib/globular/etcd/etcd.yaml": updated,
	}

	actions := restartActionsForChangedConfigs(stored, newRendered)
	if len(actions) == 0 {
		t.Fatal("expected restart actions for changed etcd config, got none")
	}
	// The etcd renderer's restartUnits should appear.
	found := false
	for _, a := range actions {
		if a.GetUnitName() == "globular-etcd.service" && a.GetAction() == "restart" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected restart action for globular-etcd.service, got actions: %v", actions)
	}
}

// TestRestartActionsForChangedConfigs_NewFile verifies that when a config file
// appears for the first time (no stored hash), NO restart is triggered.
// The service starts fresh via enable/start actions; an extra restart is not needed.
func TestRestartActionsForChangedConfigs_NewFile(t *testing.T) {
	newRendered := map[string]string{
		"/var/lib/globular/minio/minio.env": "MINIO_VOLUMES=/var/lib/globular/minio/data\n",
	}

	// No previously stored hashes (first dispatch).
	actions := restartActionsForChangedConfigs(nil, newRendered)
	if len(actions) != 0 {
		t.Errorf("expected no restart actions on first render (initial write), got %d: %v", len(actions), actions)
	}
}

// TestRestartActionsForChangedConfigs_MultipleRenderers verifies that changing
// two renderers' outputs produces restart actions for both, not each other's.
func TestRestartActionsForChangedConfigs_MultipleRenderers(t *testing.T) {
	old := map[string]string{
		"/var/lib/globular/etcd/etcd.yaml":  "name: node1\n",
		"/var/lib/globular/minio/minio.env": "MINIO_VOLUMES=/path\n",
	}
	stored := HashRenderedConfigs(old)

	newRendered := map[string]string{
		"/var/lib/globular/etcd/etcd.yaml":  "name: node2\n",   // changed
		"/var/lib/globular/minio/minio.env": "MINIO_VOLUMES=/path\n", // unchanged
	}

	actions := restartActionsForChangedConfigs(stored, newRendered)
	// Should have restart for etcd but NOT minio.
	etcdRestart := false
	minioRestart := false
	for _, a := range actions {
		if a.GetAction() != "restart" {
			continue
		}
		switch a.GetUnitName() {
		case "globular-etcd.service":
			etcdRestart = true
		case "globular-minio.service":
			minioRestart = true
		}
	}
	if !etcdRestart {
		t.Error("expected restart for globular-etcd.service")
	}
	if minioRestart {
		t.Error("unexpected restart for globular-minio.service (content unchanged)")
	}
}

// TestHashRenderedConfigs_Deterministic verifies that the same content always
// produces the same hash.
func TestHashRenderedConfigs_Deterministic(t *testing.T) {
	content := map[string]string{"path": "content"}
	h1 := HashRenderedConfigs(content)
	h2 := HashRenderedConfigs(content)
	if h1["path"] != h2["path"] {
		t.Errorf("hashing is not deterministic: %q vs %q", h1["path"], h2["path"])
	}
}

// TestHashRenderedConfigs_DifferentContent verifies that different content
// produces different hashes.
func TestHashRenderedConfigs_DifferentContent(t *testing.T) {
	h1 := HashRenderedConfigs(map[string]string{"path": "content-a"})
	h2 := HashRenderedConfigs(map[string]string{"path": "content-b"})
	if h1["path"] == h2["path"] {
		t.Error("different content produced identical hash")
	}
}

// TestHashRenderedConfigs_Empty verifies nil is returned for empty input.
func TestHashRenderedConfigs_Empty(t *testing.T) {
	if h := HashRenderedConfigs(nil); h != nil {
		t.Errorf("expected nil for nil input, got %v", h)
	}
	if h := HashRenderedConfigs(map[string]string{}); h != nil {
		t.Errorf("expected nil for empty input, got %v", h)
	}
}
