package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeCanonicalArtifact(t *testing.T, dir, name, spec string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	gz := gzip.NewWriter(f)
	defer gz.Close()
	tw := tar.NewWriter(gz)
	defer tw.Close()

	body := []byte(spec)
	hdr := &tar.Header{
		Name: "specs/test_service.yaml",
		Mode: 0o644,
		Size: int64(len(body)),
	}
	if err := tw.WriteHeader(hdr); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(body); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestRenderCanonicalUnitFromArtifactPath_DeterministicAcrossArtifactPathsAndCWD(t *testing.T) {
	spec := `version: 1
metadata:
  name: test-svc
steps:
  - id: install-test-service
    type: install_services
    units:
      - name: globular-test-svc.service
        content: |
          [Service]
          WorkingDirectory={{.StateDir}}/test_svc
          ExecStart={{.Prefix}}/bin/test_server --node {{.NodeIP}} --data {{.MinioDataDir}} --log {{.LogDir}}
`
	dirA := t.TempDir()
	dirB := t.TempDir()
	artifactA := writeCanonicalArtifact(t, dirA, "a.tgz", spec)
	artifactB := writeCanonicalArtifact(t, dirB, "b.tgz", spec)
	input := CanonicalUnitRenderInput{
		PackageName:  "test-svc",
		Version:      "1.0.0",
		Kind:         "SERVICE",
		PublisherID:  defaultPublisherID,
		Platform:     "linux_amd64",
		StateDir:     "/var/lib/globular",
		Prefix:       "/usr/lib/globular",
		BinDir:       "/usr/lib/globular/bin",
		LogDir:       "/var/log/globular",
		MinioDataDir: "/srv/minio/data",
		NodeIP:       "10.0.0.8",
	}

	origWD, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	if err := os.Chdir(dirA); err != nil {
		t.Fatal(err)
	}
	gotA, err := renderCanonicalUnitFromArtifactPath(artifactA, "globular-test-svc.service", input)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(dirB); err != nil {
		t.Fatal(err)
	}
	gotB, err := renderCanonicalUnitFromArtifactPath(artifactB, "globular-test-svc.service", input)
	if err != nil {
		t.Fatal(err)
	}

	if !canonicalUnitContentEqual(gotA, gotB) {
		t.Fatalf("canonical unit render changed across artifact path/cwd\nA:\n%s\n\nB:\n%s", gotA, gotB)
	}
	if !strings.Contains(string(gotA), "WorkingDirectory=-/var/lib/globular/test-svc") {
		t.Fatalf("expected canonicalized WorkingDirectory in render:\n%s", gotA)
	}
	if strings.Contains(string(gotA), "WorkingDirectory=/var/lib/globular/test_svc") {
		t.Fatalf("fragile/non-canonical WorkingDirectory survived render:\n%s", gotA)
	}
}
