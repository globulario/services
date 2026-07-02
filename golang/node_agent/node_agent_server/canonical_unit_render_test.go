package main

import (
	"archive/tar"
	"compress/gzip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	node_agentpb "github.com/globulario/services/golang/node_agent/node_agentpb"
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

func writeCanonicalArtifactWithEntries(t *testing.T, dir, name string, entries map[string]string) string {
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

	for entryName, content := range entries {
		body := []byte(content)
		hdr := &tar.Header{
			Name: entryName,
			Mode: 0o644,
			Size: int64(len(body)),
		}
		if err := tw.WriteHeader(hdr); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write(body); err != nil {
			t.Fatal(err)
		}
	}
	return path
}

func TestRenderCanonicalUnitFromArtifactPath_AcceptsDotSlashTarEntries(t *testing.T) {
	dir := t.TempDir()
	artifact := writeCanonicalArtifactWithEntries(t, dir, "dot-slash.tgz", map[string]string{
		"./systemd/globular-test-svc.service": "[Service]\nExecStart={{.Prefix}}/bin/test_server --node {{.NodeIP}}\n",
		"./specs/test_service.yaml": `version: 1
metadata:
  name: test-svc
`,
	})

	got, err := renderCanonicalUnitFromArtifactPath(artifact, "globular-test-svc.service", CanonicalUnitRenderInput{
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
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(got), "ExecStart=/usr/lib/globular/bin/test_server --node 10.0.0.8") {
		t.Fatalf("expected rendered unit from ./systemd entry, got:\n%s", got)
	}
}

func TestReceiptUnitContent_UsesDiskWhenRenderedDiffers(t *testing.T) {
	rendered := []byte("[Service]\nWorkingDirectory=-/var/lib/globular\nExecStart=/usr/lib/globular/bin/envoy\n")
	disk := []byte("[Service]\nExecStart=/usr/lib/globular/bin/envoy\n")

	got := receiptUnitContent(rendered, disk)
	if string(got) != string(disk) {
		t.Fatalf("receipt must stamp installed disk evidence when rendered unit differs")
	}
}

func TestReceiptUnitContent_UsesRenderedWhenEqual(t *testing.T) {
	rendered := []byte("[Service]\nExecStart=/usr/lib/globular/bin/envoy\n")
	disk := []byte("[Service]\nExecStart=/usr/lib/globular/bin/envoy\n")

	got := receiptUnitContent(rendered, disk)
	if string(got) != string(rendered) {
		t.Fatalf("receipt should keep canonical rendered content when it equals disk")
	}
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

	pkgA := &node_agentpb.InstalledPackage{}
	pkgB := &node_agentpb.InstalledPackage{}
	if err := StampInstallReceipt(pkgA, ReceiptOpts{
		UnitFilePath:        "/etc/systemd/system/globular-test-svc.service",
		UnitFileContent:     gotA,
		UnitRendererVersion: canonicalUnitRendererVersion,
	}); err != nil {
		t.Fatalf("StampInstallReceipt A: %v", err)
	}
	if err := StampInstallReceipt(pkgB, ReceiptOpts{
		UnitFilePath:        "/etc/systemd/system/globular-test-svc.service",
		UnitFileContent:     gotB,
		UnitRendererVersion: canonicalUnitRendererVersion,
	}); err != nil {
		t.Fatalf("StampInstallReceipt B: %v", err)
	}
	if pkgA.Metadata[receiptKeyUnitFileSha256] != pkgB.Metadata[receiptKeyUnitFileSha256] {
		t.Fatalf("canonical unit hash changed across artifact path/cwd: %q vs %q",
			pkgA.Metadata[receiptKeyUnitFileSha256], pkgB.Metadata[receiptKeyUnitFileSha256])
	}
}
