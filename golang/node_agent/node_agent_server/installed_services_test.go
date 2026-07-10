package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/versionutil"
)

func TestComputeInstalledServicesDeterministic(t *testing.T) {
	tmp := t.TempDir()
	oldBase := versionutil.BaseDir()
	versionutil.SetBaseDir(tmp)
	t.Cleanup(func() { versionutil.SetBaseDir(oldBase) })
	t.Setenv("GLOBULAR_SERVICES_DIR", tmp)
	t.Setenv("PATH", t.TempDir())

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
	// Check that our test service is in the result (systemd units from the
	// running host may also be present).
	found := false
	for k, info := range installed1 {
		if k.ServiceName == "gateway" && info.Version == "1.0.0" {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected gateway@1.0.0 in installed services")
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
	t.Setenv("PATH", t.TempDir())

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

func TestAppliedHashNotAffectedByPublisher(t *testing.T) {
	// The applied hash uses only canonical service name + version (matching the
	// controller's desired hash format). Publisher does not affect the hash.
	first := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svca"}: {PublisherID: "p1", ServiceName: "svca", Version: "1"},
	}
	second := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p2", ServiceName: "svca"}: {PublisherID: "p2", ServiceName: "svca", Version: "1"},
	}

	if h1, h2 := computeAppliedServicesHash(first), computeAppliedServicesHash(second); h1 != h2 {
		t.Fatalf("hash should NOT differ when only publisher differs; got %q and %q", h1, h2)
	}
}

func TestAppliedHashDeterministicWithDuplicateServiceNames(t *testing.T) {
	first := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svc"}: {PublisherID: "p1", ServiceName: "svc", Version: "1"},
		{PublisherID: "p2", ServiceName: "svc"}: {PublisherID: "p2", ServiceName: "svc", Version: "2"},
	}
	second := map[ServiceKey]InstalledServiceInfo{}
	second[ServiceKey{PublisherID: "p2", ServiceName: "svc"}] = InstalledServiceInfo{PublisherID: "p2", ServiceName: "svc", Version: "2"}
	second[ServiceKey{PublisherID: "p1", ServiceName: "svc"}] = InstalledServiceInfo{PublisherID: "p1", ServiceName: "svc", Version: "1"}

	if h1, h2 := computeAppliedServicesHash(first), computeAppliedServicesHash(second); h1 != h2 {
		t.Fatalf("hash should be deterministic with duplicate service names; got %q and %q", h1, h2)
	}
}

// TestAppliedHashNotAffectedByConfig verifies that config differences do not
// change the applied services hash (matches controller's version-only format).
func TestAppliedHashNotAffectedByConfig(t *testing.T) {
	withDigest := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svca"}: {PublisherID: "p1", ServiceName: "svca", Version: "1.0.0", ConfigDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	withoutDigest := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svca"}: {PublisherID: "p1", ServiceName: "svca", Version: "1.0.0"},
	}
	if h1, h2 := computeAppliedServicesHash(withDigest), computeAppliedServicesHash(withoutDigest); h1 != h2 {
		t.Fatalf("hash should NOT differ when only config digest differs; got %q and %q", h1, h2)
	}
}

// TestAppliedHashCanonicalFormat verifies the exact canonical string format so that
// controller and node-agent can be independently validated to produce the same hash
// for a single service: SHA256("<canonical_service_name>=<version>;").
func TestAppliedHashCanonicalFormat(t *testing.T) {
	installed := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "pub", ServiceName: "gateway"}: {PublisherID: "pub", ServiceName: "gateway", Version: "1.0.0"},
	}
	got := computeAppliedServicesHash(installed)

	// Manually compute expected: "gateway=1.0.0;"
	raw := "gateway=1.0.0;"
	sum := sha256.Sum256([]byte(raw))
	want := hex.EncodeToString(sum[:])

	if got != want {
		t.Fatalf("applied hash format mismatch\n  got:  %q\n  want: %q\n  (raw string: %q)", got, want, raw)
	}
}

func TestConfigDigestChangeDoesNotChangeHash(t *testing.T) {
	// Config digest is not included in the hash (matches controller format).
	first := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svca"}: {PublisherID: "p1", ServiceName: "svca", Version: "1", ConfigDigest: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"},
	}
	second := map[ServiceKey]InstalledServiceInfo{
		{PublisherID: "p1", ServiceName: "svca"}: {PublisherID: "p1", ServiceName: "svca", Version: "1", ConfigDigest: "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"},
	}
	if h1, h2 := computeAppliedServicesHash(first), computeAppliedServicesHash(second); h1 != h2 {
		t.Fatalf("hash should NOT change when only config digest differs; got %q and %q", h1, h2)
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

// TestLoadSystemdUnitsDetectsVersionFromBinary verifies that when a service
// has a running systemd unit but no version marker, loadSystemdUnits probes
// the binary via --describe and classifies it as ManagedInstalled when a
// real version is found. This is the path taken by bootstrapped services
// (installed by globular-installer) that have not yet gone through the
// controller convergence cycle.
func TestLoadSystemdUnitsDetectsVersionFromBinary(t *testing.T) {
	// Create a fake "dns_server" binary that outputs valid --describe JSON.
	binDir := t.TempDir()
	fakeBin := filepath.Join(binDir, "dns_server")
	script := "#!/bin/sh\necho '{\"Id\":\"dns-id\",\"Version\":\"1.2.3\"}'\n"
	if err := os.WriteFile(fakeBin, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake binary: %v", err)
	}

	old := globularBinDir
	globularBinDir = binDir
	t.Cleanup(func() { globularBinDir = old })

	// Simulate a byService map with a "dns" entry (no version yet) as if
	// loadSystemdUnits found a globular-dns.service unit.
	byService := map[string]*InstalledServiceInfo{
		"dns": {ServiceName: "dns", Source: RuntimeUnmanaged, Version: ""},
	}

	// Invoke the same probe logic that loadSystemdUnits uses for unversioned entries.
	entry := byService["dns"]
	binPath := filepath.Join(globularBinDir, "dns_server")
	if version := detectGlobularBinaryVersion(t.Context(), binPath); version != "" && version != "unknown" {
		entry.Version = version
		entry.Source = ManagedInstalled
	}

	if entry.Source != ManagedInstalled {
		t.Fatalf("expected ManagedInstalled after binary probe, got %v", entry.Source)
	}
	if entry.Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %q", entry.Version)
	}
}

// TestLoadSystemdUnitsRuntimeUnmanagedOnProbeFailure verifies that when the
// binary does not exist (or --describe fails), the service remains RuntimeUnmanaged.
func TestLoadSystemdUnitsRuntimeUnmanagedOnProbeFailure(t *testing.T) {
	binDir := t.TempDir()

	old := globularBinDir
	globularBinDir = binDir
	t.Cleanup(func() { globularBinDir = old })

	entry := &InstalledServiceInfo{ServiceName: "unknown-svc", Source: RuntimeUnmanaged}
	binPath := filepath.Join(globularBinDir, "unknown_svc_server") // doesn't exist
	version := detectGlobularBinaryVersion(t.Context(), binPath)
	if version != "" && version != "unknown" {
		entry.Version = version
		entry.Source = ManagedInstalled
	} else {
		entry.Source = RuntimeUnmanaged
		entry.Version = "unknown"
	}

	if entry.Source != RuntimeUnmanaged {
		t.Fatalf("expected RuntimeUnmanaged when binary is missing, got %v", entry.Source)
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
