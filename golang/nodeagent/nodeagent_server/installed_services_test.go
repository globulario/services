package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
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

// TestAppliedHashNotAffectedByConfig verifies that config differences do not change
// the P2 applied services hash (config excluded until P3).
func TestAppliedHashNotAffectedByConfig(t *testing.T) {
	withDigest := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svcA"}: {PublisherID: "p1", ServiceName: "svcA", Version: "1.0.0", ConfigDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	withoutDigest := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svcA"}: {PublisherID: "p1", ServiceName: "svcA", Version: "1.0.0"},
	}
	if h1, h2 := computeAppliedServicesHash(withDigest), computeAppliedServicesHash(withoutDigest); h1 == h2 {
		t.Fatalf("hash should differ when config digest differs")
	}
}

// TestAppliedHashCanonicalFormat verifies the exact canonical string format so that
// controller and node-agent can be independently validated to produce the same hash
// for a single service: SHA256("<publisher_id>/<canonical_service_name>=<version>;").
func TestAppliedHashCanonicalFormat(t *testing.T) {
	installed := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "pub", ServiceName: "gateway"}: {PublisherID: "pub", ServiceName: "gateway", Version: "1.0.0"},
	}
	got := computeAppliedServicesHash(installed)

	// Manually compute expected: "pub/gateway=1.0.0@-;"
	raw := "pub/gateway=1.0.0@-;"
	sum := sha256.Sum256([]byte(raw))
	want := hex.EncodeToString(sum[:])

	if got != want {
		t.Fatalf("applied hash format mismatch\n  got:  %q\n  want: %q\n  (raw string: %q)", got, want, raw)
	}
}

func TestConfigDigestChangeChangesHash(t *testing.T) {
	first := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svcA"}: {PublisherID: "p1", ServiceName: "svcA", Version: "1", ConfigDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	second := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svcA"}: {PublisherID: "p1", ServiceName: "svcA", Version: "1", ConfigDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	if h1, h2 := computeAppliedServicesHash(first), computeAppliedServicesHash(second); h1 == h2 {
		t.Fatalf("hash should change when config digest differs")
	}
}

func TestInvalidConfigDigestReturnsError(t *testing.T) {
	tmp := t.TempDir()
	oldBase := versionutil.BaseDir()
	versionutil.SetBaseDir(tmp)
	t.Cleanup(func() { versionutil.SetBaseDir(oldBase) })
	t.Setenv("GLOBULAR_SERVICES_DIR", tmp)

	writeMarker(t, filepath.Join(tmp, "gateway"), "1.0.0")
	writeConfigDigest(t, filepath.Join(tmp, "gateway"), "not-hex")

	_, _, err := ComputeInstalledServices(context.Background())
	if err == nil {
		t.Fatalf("expected error for invalid config digest marker")
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

func writeConfigDigest(t *testing.T, dir, digest string) {
	t.Helper()
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", dir, err)
	}
	if err := os.WriteFile(filepath.Join(dir, "config.sha256"), []byte(digest), 0o644); err != nil {
		t.Fatalf("write config digest: %v", err)
	}
}
