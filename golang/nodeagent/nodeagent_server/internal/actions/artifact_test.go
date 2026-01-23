package actions

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestArtifactFetchResolvesRepoRoot(t *testing.T) {
	repo := t.TempDir()
	service, version, platform := "svc", "1.0.0", "linux_amd64"
	srcPath := filepath.Join(repo, service, version, platform)
	if err := os.MkdirAll(srcPath, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	artifact := filepath.Join(srcPath, "svc.1.0.0.linux_amd64.tgz")
	if err := os.WriteFile(artifact, []byte("data"), 0o644); err != nil {
		t.Fatalf("write artifact: %v", err)
	}
	t.Setenv("GLOBULAR_ARTIFACT_REPO_ROOT", repo)

	dest := filepath.Join(t.TempDir(), "out.tgz")
	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":       service,
		"version":       version,
		"platform":      platform,
		"artifact_path": dest,
	})
	a := artifactFetchAction{}
	if _, err := a.Apply(context.Background(), args); err != nil {
		t.Fatalf("apply: %v", err)
	}
	b, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("read dest: %v", err)
	}
	if string(b) != "data" {
		t.Fatalf("dest content mismatch: %s", string(b))
	}
}

func TestServiceInstallPayloadPromotesFiles(t *testing.T) {
	binDir := filepath.Join(t.TempDir(), "bin")
	systemdDir := filepath.Join(t.TempDir(), "systemd")
	configDir := filepath.Join(t.TempDir(), "config")
	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)
	t.Setenv("GLOBULAR_INSTALL_SYSTEMD_DIR", systemdDir)
	t.Setenv("GLOBULAR_INSTALL_CONFIG_DIR", configDir)
	t.Setenv("GLOBULAR_SKIP_SYSTEMD", "1")

	artifactPath := filepath.Join(t.TempDir(), "svc.tgz")
	createTestArchive(t, artifactPath)

	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":       "svc",
		"version":       "1.0.0",
		"artifact_path": artifactPath,
	})
	a := serviceInstallPayloadAction{}
	if _, err := a.Apply(context.Background(), args); err != nil {
		t.Fatalf("apply install: %v", err)
	}

	checkFile := func(path, want string) {
		b, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		if string(b) != want {
			t.Fatalf("%s content mismatch: %s", path, string(b))
		}
	}

	checkFile(filepath.Join(binDir, "testsvc"), "bin")
	checkFile(filepath.Join(systemdDir, "testsvc.service"), "unit")
	checkFile(filepath.Join(configDir, "svc", "app.yaml"), "cfg")
}

func createTestArchive(t *testing.T, dest string) {
	t.Helper()
	f, err := os.Create(dest)
	if err != nil {
		t.Fatalf("create archive: %v", err)
	}
	gz := gzip.NewWriter(f)
	tw := tar.NewWriter(gz)

	addFile := func(name, content string, mode int64) {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: mode, Size: int64(len(content))}); err != nil {
			t.Fatalf("write hdr: %v", err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatalf("write body: %v", err)
		}
	}

	addFile("bin/testsvc", "bin", 0o755)
	addFile("systemd/testsvc.service", "unit", 0o644)
	addFile("config/app.yaml", "cfg", 0o644)

	if err := tw.Close(); err != nil {
		t.Fatalf("close tar: %v", err)
	}
	if err := gz.Close(); err != nil {
		t.Fatalf("close gz: %v", err)
	}
	if err := f.Close(); err != nil {
		t.Fatalf("close file: %v", err)
	}
}
