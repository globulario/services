package main

// Tests for the Day-1 local CAS seeder (blob_seed.go). The contract under
// test: a PUBLISHED manifest whose blob is missing locally is materialized
// from a staged join package IF AND ONLY IF the staged bytes' sha256 matches
// the manifest checksum — the Scylla manifest is the authority, the staged
// file is only evidence, and filenames never decide identity.

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/globulario/services/golang/storage_backend"
)

func sha256OfBytes(b []byte) string {
	h := sha256.Sum256(b)
	return "sha256:" + hex.EncodeToString(h[:])
}

func newSeedTestServer(t *testing.T) (*server, string) {
	t.Helper()
	casRoot := t.TempDir()
	srv := &server{
		localStorage: storage_backend.NewOSStorage(casRoot),
	}
	return srv, casRoot
}

func writeStaged(t *testing.T, dir, name string, content []byte) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, content, 0o644); err != nil {
		t.Fatalf("write staged %s: %v", p, err)
	}
	return p
}

func seedRow(key, name, version, platform string, content []byte, state string) manifestRow {
	return manifestRow{
		ArtifactKey:  key,
		Name:         name,
		Version:      version,
		Platform:     platform,
		Checksum:     sha256OfBytes(content),
		SizeBytes:    int64(len(content)),
		PublishState: state,
		ManifestJSON: []byte(`{"name":"` + name + `"}`),
	}
}

// A published manifest with a digest-matching staged archive is seeded:
// blob lands in the local CAS (atomic, digest-verified) with the manifest
// sidecar beside it.
func TestBlobSeed_PublishedWithMatchingStagedArchiveIsSeeded(t *testing.T) {
	srv, casRoot := newSeedTestServer(t)
	staged := t.TempDir()
	content := []byte("the package bytes")
	writeStaged(t, staged, "echo_1.0.0_linux_amd64.tgz", content)

	rows := []manifestRow{seedRow("core@globular.io%echo%1.0.0%linux_amd64%1", "echo", "1.0.0", "linux_amd64", content, "published")}
	seeded, present, unmatched := srv.seedManifestRowsFromDirs(context.Background(), rows, []string{staged})

	if seeded != 1 || present != 0 || unmatched != 0 {
		t.Fatalf("counts = (seeded=%d, present=%d, unmatched=%d), want (1,0,0)", seeded, present, unmatched)
	}
	blobPath := filepath.Join(casRoot, "artifacts", "core@globular.io%echo%1.0.0%linux_amd64%1.bin")
	got, err := os.ReadFile(blobPath)
	if err != nil {
		t.Fatalf("seeded blob missing: %v", err)
	}
	if string(got) != string(content) {
		t.Fatalf("seeded blob bytes differ from staged archive")
	}
	sidecar := filepath.Join(casRoot, "artifacts", "core@globular.io%echo%1.0.0%linux_amd64%1.manifest.json")
	if _, err := os.Stat(sidecar); err != nil {
		t.Fatalf("manifest sidecar not written: %v", err)
	}
}

// A staged file whose name matches but whose DIGEST does not must never be
// seeded — filenames are hints, the manifest checksum is the gate.
func TestBlobSeed_DigestMismatchIsNeverSeeded(t *testing.T) {
	srv, casRoot := newSeedTestServer(t)
	staged := t.TempDir()
	writeStaged(t, staged, "echo_1.0.0_linux_amd64.tgz", []byte("tampered or stale bytes"))

	authority := []byte("the bytes the manifest actually promises")
	rows := []manifestRow{seedRow("core@globular.io%echo%1.0.0%linux_amd64%1", "echo", "1.0.0", "linux_amd64", authority, "published")}
	seeded, _, unmatched := srv.seedManifestRowsFromDirs(context.Background(), rows, []string{staged})

	if seeded != 0 || unmatched != 1 {
		t.Fatalf("counts = (seeded=%d, unmatched=%d), want (0,1)", seeded, unmatched)
	}
	if _, err := os.Stat(filepath.Join(casRoot, "artifacts", "core@globular.io%echo%1.0.0%linux_amd64%1.bin")); err == nil {
		t.Fatal("digest-mismatched staged file was materialized — the gate failed")
	}
}

// An already-present blob is left untouched and counted as present.
func TestBlobSeed_ExistingBlobUntouched(t *testing.T) {
	srv, casRoot := newSeedTestServer(t)
	content := []byte("already here")
	blobDir := filepath.Join(casRoot, "artifacts")
	if err := os.MkdirAll(blobDir, 0o755); err != nil {
		t.Fatal(err)
	}
	blobPath := filepath.Join(blobDir, "core@globular.io%echo%1.0.0%linux_amd64%1.bin")
	if err := os.WriteFile(blobPath, content, 0o644); err != nil {
		t.Fatal(err)
	}

	rows := []manifestRow{seedRow("core@globular.io%echo%1.0.0%linux_amd64%1", "echo", "1.0.0", "linux_amd64", content, "published")}
	seeded, present, _ := srv.seedManifestRowsFromDirs(context.Background(), rows, []string{t.TempDir()})
	if seeded != 0 || present != 1 {
		t.Fatalf("counts = (seeded=%d, present=%d), want (0,1)", seeded, present)
	}
}

// Non-published manifests are ignored entirely — seeding must never make a
// non-published artifact look present.
func TestBlobSeed_NonPublishedIgnored(t *testing.T) {
	srv, casRoot := newSeedTestServer(t)
	staged := t.TempDir()
	content := []byte("quarantined bytes")
	writeStaged(t, staged, "echo_1.0.0_linux_amd64.tgz", content)

	rows := []manifestRow{seedRow("core@globular.io%echo%1.0.0%linux_amd64%1", "echo", "1.0.0", "linux_amd64", content, "quarantined")}
	seeded, present, unmatched := srv.seedManifestRowsFromDirs(context.Background(), rows, []string{staged})
	if seeded != 0 || present != 0 || unmatched != 0 {
		t.Fatalf("non-published row was processed: (%d,%d,%d)", seeded, present, unmatched)
	}
	if _, err := os.Stat(filepath.Join(casRoot, "artifacts", "core@globular.io%echo%1.0.0%linux_amd64%1.bin")); err == nil {
		t.Fatal("non-published artifact was seeded")
	}
}

// A manifest with no checksum must never seed — unverified bytes do not enter
// the CAS.
func TestBlobSeed_NoAuthorityChecksumNeverSeeds(t *testing.T) {
	srv, _ := newSeedTestServer(t)
	staged := t.TempDir()
	writeStaged(t, staged, "echo_1.0.0_linux_amd64.tgz", []byte("bytes"))

	row := seedRow("core@globular.io%echo%1.0.0%linux_amd64%1", "echo", "1.0.0", "linux_amd64", []byte("bytes"), "published")
	row.Checksum = ""
	seeded, _, unmatched := srv.seedManifestRowsFromDirs(context.Background(), []manifestRow{row}, []string{staged})
	if seeded != 0 || unmatched != 1 {
		t.Fatalf("checksum-less manifest seeded: (seeded=%d, unmatched=%d)", seeded, unmatched)
	}
}

// The loose filename pattern still seeds when the digest matches (filename is
// only a hint — a renamed archive with correct bytes is acceptable evidence).
func TestBlobSeed_LooseNameMatchStillDigestGated(t *testing.T) {
	srv, casRoot := newSeedTestServer(t)
	staged := t.TempDir()
	content := []byte("correct bytes, loose name")
	writeStaged(t, staged, "echo_9.9.9-rebuild_linux_amd64.tgz", content) // matches name_*_platform glob

	rows := []manifestRow{seedRow("core@globular.io%echo%1.0.0%linux_amd64%1", "echo", "1.0.0", "linux_amd64", content, "published")}
	seeded, _, _ := srv.seedManifestRowsFromDirs(context.Background(), rows, []string{staged})
	if seeded != 1 {
		t.Fatalf("loose-name digest-matching archive should seed, seeded=%d", seeded)
	}
	if _, err := os.Stat(filepath.Join(casRoot, "artifacts", "core@globular.io%echo%1.0.0%linux_amd64%1.bin")); err != nil {
		t.Fatalf("blob not materialized: %v", err)
	}
}
