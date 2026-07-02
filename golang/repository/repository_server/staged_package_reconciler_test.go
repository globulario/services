package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	repopb "github.com/globulario/services/golang/repository/repositorypb"
	"github.com/globulario/services/golang/workflow"
)

func TestReconcileScyllaFromStagedPackagesPublishesArchive(t *testing.T) {
	srv := newTestServer(t)
	ledger := newFakeLedger()
	srv.scylla = ledger
	srv.listCache = newListCache(ledger)
	srv.workflowRec = workflow.NewRecorder("", "test")

	stageDir := t.TempDir()
	archivePath := filepath.Join(stageDir, "demo_1.2.3-dev.test.1_linux_amd64.tgz")
	archiveBytes := buildStagedTestArchive(t, stagedTestManifest{
		Type:      "service",
		Name:      "demo",
		Version:   "1.2.3-dev.test.1",
		Platform:  "linux_amd64",
		Publisher: "local@test",
		Channel:   "dev",
	})
	if err := os.WriteFile(archivePath, archiveBytes, 0o644); err != nil {
		t.Fatalf("write staged archive: %v", err)
	}
	markStagedArchiveStable(t, archivePath)

	srv.reconcileScyllaFromStagedPackages(context.Background(), []string{stageDir})

	ref := &repopb.ArtifactRef{
		PublisherId: "local@test",
		Name:        "demo",
		Version:     "1.2.3-dev.test.1",
		Platform:    "linux_amd64",
	}
	manifestResp, err := srv.GetArtifactManifest(context.Background(), &repopb.GetArtifactManifestRequest{Ref: ref})
	if err != nil {
		t.Fatalf("GetArtifactManifest: %v", err)
	}
	if manifestResp.GetManifest().GetRef().GetName() != "demo" {
		t.Fatalf("manifest name=%q, want demo", manifestResp.GetManifest().GetRef().GetName())
	}
	if manifestResp.GetManifest().GetPublishState() != repopb.PublishState_PUBLISHED {
		t.Fatalf("publish_state=%s, want PUBLISHED", manifestResp.GetManifest().GetPublishState())
	}

	key := artifactKeyWithBuild(ref, manifestResp.GetManifest().GetBuildNumber())
	if _, err := srv.localStorage.Stat(context.Background(), binaryStorageKey(key)); err != nil {
		t.Fatalf("local CAS blob missing after staged reconcile: %v", err)
	}
	if got := srv.readArtifactState(context.Background(), key); got != PipelinePublished {
		t.Fatalf("artifact_state=%s, want %s", got, PipelinePublished)
	}

	rows, err := ledger.ListManifests(context.Background())
	if err != nil {
		t.Fatalf("ListManifests: %v", err)
	}
	var matched bool
	for _, row := range rows {
		if row.PublisherID == "local@test" &&
			row.Name == "demo" &&
			row.Version == "1.2.3-dev.test.1" &&
			row.Platform == "linux_amd64" &&
			row.PublishState == repopb.PublishState_PUBLISHED.String() {
			matched = true
			break
		}
	}
	if !matched {
		t.Fatalf("expected published Scylla row for staged archive, got %+v", rows)
	}
}

func TestReconcileScyllaFromStagedPackagesSkipsInvalidArchive(t *testing.T) {
	srv := newTestServer(t)
	ledger := newFakeLedger()
	srv.scylla = ledger
	srv.listCache = newListCache(ledger)
	srv.workflowRec = workflow.NewRecorder("", "test")

	stageDir := t.TempDir()
	badArchivePath := filepath.Join(stageDir, "broken.tgz")
	if err := os.WriteFile(badArchivePath, buildBrokenStageArchive(t), 0o644); err != nil {
		t.Fatalf("write broken archive: %v", err)
	}
	markStagedArchiveStable(t, badArchivePath)

	srv.reconcileScyllaFromStagedPackages(context.Background(), []string{stageDir})

	rows, err := ledger.ListManifests(context.Background())
	if err != nil {
		t.Fatalf("ListManifests: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected 0 Scylla rows for invalid archive, got %d", len(rows))
	}
}

func TestReconcileScyllaFromStagedPackagesSkipsUnstableArchive(t *testing.T) {
	srv := newTestServer(t)
	ledger := newFakeLedger()
	srv.scylla = ledger
	srv.listCache = newListCache(ledger)
	srv.workflowRec = workflow.NewRecorder("", "test")

	stageDir := t.TempDir()
	archivePath := filepath.Join(stageDir, "demo_1.2.4-dev.test.1_linux_amd64.tgz")
	archiveBytes := buildStagedTestArchive(t, stagedTestManifest{
		Type:      "service",
		Name:      "demo",
		Version:   "1.2.4-dev.test.1",
		Platform:  "linux_amd64",
		Publisher: "local@test",
		Channel:   "dev",
	})
	if err := os.WriteFile(archivePath, archiveBytes, 0o644); err != nil {
		t.Fatalf("write staged archive: %v", err)
	}

	srv.reconcileScyllaFromStagedPackages(context.Background(), []string{stageDir})

	rows, err := ledger.ListManifests(context.Background())
	if err != nil {
		t.Fatalf("ListManifests: %v", err)
	}
	if len(rows) != 0 {
		t.Fatalf("expected unstable staged archive to be skipped, got %d Scylla rows", len(rows))
	}
}

type stagedTestManifest struct {
	Type               string `json:"type"`
	Name               string `json:"name"`
	Version            string `json:"version"`
	Platform           string `json:"platform"`
	Publisher          string `json:"publisher"`
	Entrypoint         string `json:"entrypoint"`
	EntrypointChecksum string `json:"entrypoint_checksum"`
	Channel            string `json:"channel,omitempty"`
}

func buildStagedTestArchive(t *testing.T, manifest stagedTestManifest) []byte {
	t.Helper()

	entrypointPath := "bin/" + manifest.Name
	entrypointBytes := []byte("#!/bin/sh\necho staged\n")
	sum := sha256.Sum256(entrypointBytes)
	manifest.Entrypoint = entrypointPath
	manifest.EntrypointChecksum = "sha256:" + hex.EncodeToString(sum[:])

	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)

	writeTarFile(t, tw, "package.json", mustJSON(t, manifest), 0o644)
	writeTarFile(t, tw, entrypointPath, entrypointBytes, 0o755)
	writeTarFile(t, tw, "specs/"+manifest.Name+".json", []byte("{}\n"), 0o644)

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func buildBrokenStageArchive(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	writeTarFile(t, tw, "README.txt", []byte("missing package manifest\n"), 0o644)
	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gzw.Close(); err != nil {
		t.Fatalf("close gzip: %v", err)
	}
	return buf.Bytes()
}

func writeTarFile(t *testing.T, tw *tar.Writer, name string, data []byte, mode int64) {
	t.Helper()
	hdr := &tar.Header{
		Name: name,
		Mode: mode,
		Size: int64(len(data)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatalf("write tar header %s: %v", name, err)
	}
	if _, err := tw.Write(data); err != nil {
		t.Fatalf("write tar file %s: %v", name, err)
	}
}

func mustJSON(t *testing.T, v any) []byte {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshal json: %v", err)
	}
	return data
}

func markStagedArchiveStable(t *testing.T, path string) {
	t.Helper()
	stableTime := time.Now().Add(-(stagedPackageStableAge + time.Second))
	if err := os.Chtimes(path, stableTime, stableTime); err != nil {
		t.Fatalf("mark staged archive stable: %v", err)
	}
}
