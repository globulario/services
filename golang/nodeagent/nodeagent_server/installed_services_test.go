package main

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/plan/versionutil"
)

func TestComputeInstalledServicesDeterministic(t *testing.T) {
	tmp := t.TempDir()
	oldBase := versionutil.BaseDir()
	versionutil.SetBaseDir(tmp)
	t.Cleanup(func() { versionutil.SetBaseDir(oldBase) })
	t.Setenv("GLOBULAR_SERVICES_DIR", tmp)

	writeMarker(t, filepath.Join(tmp, "gateway"), "1.0.0")
	writeConfig(t, filepath.Join(tmp, "gateway.json"), map[string]interface{}{
		"Name":        "gateway",
		"Version":     "1.0.0",
		"PublisherID": "globular",
		"Config": map[string]string{
			"foo": "bar",
		},
	})

	installed1, hash1, err := ComputeInstalledServices(context.Background())
	if err != nil {
		t.Fatalf("ComputeInstalledServices returned error: %v", err)
	}
	if len(installed1) != 1 {
		t.Fatalf("expected 1 installed service, got %d", len(installed1))
	}
	key := ServiceKey{PublisherID: "globular", ServiceName: "gateway"}
	if installed1[key].Version != "1.0.0" {
		t.Fatalf("expected version 1.0.0, got %q", installed1[key].Version)
	}

	installed2, hash2, err := ComputeInstalledServices(context.Background())
	if err != nil {
		t.Fatalf("second ComputeInstalledServices returned error: %v", err)
	}
	if hash1 == "" || hash2 == "" {
		t.Fatalf("expected non-empty hashes, got %q and %q", hash1, hash2)
	}
	if hash1 != hash2 {
		t.Fatalf("hash should be deterministic; got %q and %q", hash1, hash2)
	}
	if len(installed2) != len(installed1) {
		t.Fatalf("installed map size changed between runs")
	}
}

func TestAppliedHashOrderIndependent(t *testing.T) {
	first := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svcA"}: {PublisherID: "p1", ServiceName: "svcA", Version: "1"},
		{PublisherID: "p2", ServiceName: "svcB"}: {PublisherID: "p2", ServiceName: "svcB", Version: "2"},
	}
	second := map[ServiceKey]InstalledServiceInfo{}
	second[ServiceKey{PublisherID: "p2", ServiceName: "svcB"}] = InstalledServiceInfo{PublisherID: "p2", ServiceName: "svcB", Version: "2"}
	second[ServiceKey{PublisherID: "p1", ServiceName: "svcA"}] = InstalledServiceInfo{PublisherID: "p1", ServiceName: "svcA", Version: "1"}

	if h1, h2 := computeAppliedServicesHash(first), computeAppliedServicesHash(second); h1 != h2 {
		t.Fatalf("expected hashes to match regardless of insertion order; got %q and %q", h1, h2)
	}
}

func TestAppliedHashChangesWhenVersionChanges(t *testing.T) {
	tmp := t.TempDir()
	oldBase := versionutil.BaseDir()
	versionutil.SetBaseDir(tmp)
	t.Cleanup(func() { versionutil.SetBaseDir(oldBase) })
	t.Setenv("GLOBULAR_SERVICES_DIR", tmp)

	writeMarker(t, filepath.Join(tmp, "gateway"), "1.0.0")
	writeConfig(t, filepath.Join(tmp, "gateway.json"), map[string]interface{}{
		"Name":        "gateway",
		"Version":     "1.0.0",
		"PublisherID": "globular",
	})

	_, hash1, err := ComputeInstalledServices(context.Background())
	if err != nil {
		t.Fatalf("ComputeInstalledServices returned error: %v", err)
	}

	writeMarker(t, filepath.Join(tmp, "gateway"), "2.0.0")
	_, hash2, err := ComputeInstalledServices(context.Background())
	if err != nil {
		t.Fatalf("ComputeInstalledServices after version change returned error: %v", err)
	}
	if hash1 == hash2 {
		t.Fatalf("hash should change when version changes")
	}
}

func TestAppliedHashChangesWhenPublisherDiffers(t *testing.T) {
	first := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svcA"}: {PublisherID: "p1", ServiceName: "svcA", Version: "1"},
	}
	second := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p2", ServiceName: "svcA"}: {PublisherID: "p2", ServiceName: "svcA", Version: "1"},
	}

	if h1, h2 := computeAppliedServicesHash(first), computeAppliedServicesHash(second); h1 == h2 {
		t.Fatalf("hash should differ when publisher differs even if name+version match")
	}
}

func writeMarker(t *testing.T, dir, version string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "version"), []byte(version), 0o644); err != nil {
		t.Fatalf("write version marker: %v", err)
	}
}

func writeConfig(t *testing.T, path string, data map[string]interface{}) {
	t.Helper()
	enc, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := os.WriteFile(path, enc, 0o644); err != nil {
		t.Fatalf("write config %s: %v", path, err)
	}
}
