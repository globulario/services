package actions

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

func TestPackageVerify_FileExists(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	os.MkdirAll(binDir, 0o755)
	os.WriteFile(filepath.Join(binDir, "gateway_server"), []byte("fake-binary"), 0o755)

	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name": "gateway",
		"kind": "SERVICE",
	})

	action := packageVerifyAction{}
	msg, err := action.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if msg == "" {
		t.Error("expected non-empty message")
	}
}

func TestPackageVerify_FileNotFound(t *testing.T) {
	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", t.TempDir())

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name": "nonexistent",
		"kind": "SERVICE",
	})

	action := packageVerifyAction{}
	_, err := action.Apply(context.Background(), args)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
}

func TestPackageVerify_ChecksumMatch(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	os.MkdirAll(binDir, 0o755)
	content := []byte("test-binary-content")
	os.WriteFile(filepath.Join(binDir, "test_server"), content, 0o755)

	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)

	// Compute expected checksum.
	expected, _ := fileSHA256(filepath.Join(binDir, "test_server"))

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name":              "test",
		"kind":              "SERVICE",
		"expected_checksum": expected,
	})

	action := packageVerifyAction{}
	_, err := action.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPackageVerify_ChecksumMismatch(t *testing.T) {
	dir := t.TempDir()
	binDir := filepath.Join(dir, "bin")
	os.MkdirAll(binDir, 0o755)
	os.WriteFile(filepath.Join(binDir, "test_server"), []byte("content"), 0o755)

	t.Setenv("GLOBULAR_INSTALL_BIN_DIR", binDir)

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name":              "test",
		"kind":              "SERVICE",
		"expected_checksum": "0000000000000000000000000000000000000000000000000000000000000000",
	})

	action := packageVerifyAction{}
	_, err := action.Apply(context.Background(), args)
	if err == nil {
		t.Fatal("expected checksum mismatch error")
	}
}

func TestApplicationInstall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GLOBULAR_STATE_DIR", dir)

	// Create a test archive with some web content.
	archivePath := filepath.Join(dir, "app.tar.gz")
	createTestAppArchive(t, archivePath)

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name":          "testapp",
		"version":       "1.0.0",
		"artifact_path": archivePath,
	})

	action := applicationInstallAction{}
	msg, err := action.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Log(msg)

	// Verify files were extracted.
	indexPath := filepath.Join(dir, "applications", "testapp", "index.html")
	if _, err := os.Stat(indexPath); err != nil {
		t.Errorf("expected index.html at %s", indexPath)
	}
}

func TestApplicationUninstall(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("GLOBULAR_STATE_DIR", dir)

	// Create app directory.
	appDir := filepath.Join(dir, "applications", "testapp")
	os.MkdirAll(appDir, 0o755)
	os.WriteFile(filepath.Join(appDir, "index.html"), []byte("<html>test</html>"), 0o644)

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name": "testapp",
	})

	action := applicationUninstallAction{}
	msg, err := action.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	t.Log(msg)

	if _, err := os.Stat(appDir); !os.IsNotExist(err) {
		t.Error("expected app directory to be removed")
	}
}

func TestApplicationUninstall_AlreadyRemoved(t *testing.T) {
	t.Setenv("GLOBULAR_STATE_DIR", t.TempDir())

	args, _ := structpb.NewStruct(map[string]interface{}{
		"name": "nonexistent",
	})

	action := applicationUninstallAction{}
	_, err := action.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("expected no error for already-removed app: %v", err)
	}
}

func TestPackageInstall_Validation(t *testing.T) {
	action := packageInstallAction{}

	// Missing name.
	args, _ := structpb.NewStruct(map[string]interface{}{
		"artifact_path": "/tmp/test.tar.gz",
		"kind":          "SERVICE",
	})
	if err := action.Validate(args); err == nil {
		t.Error("expected validation error for missing name")
	}

	// Missing artifact_path.
	args, _ = structpb.NewStruct(map[string]interface{}{
		"name": "test",
		"kind": "SERVICE",
	})
	if err := action.Validate(args); err == nil {
		t.Error("expected validation error for missing artifact_path")
	}

	// Missing kind.
	args, _ = structpb.NewStruct(map[string]interface{}{
		"name":          "test",
		"artifact_path": "/tmp/test.tar.gz",
	})
	if err := action.Validate(args); err == nil {
		t.Error("expected validation error for missing kind")
	}
}

// createTestAppArchive creates a .tar.gz with a simple index.html.
func createTestAppArchive(t *testing.T, path string) {
	t.Helper()
	var buf bytes.Buffer
	gw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gw)

	content := []byte("<html><body>Hello</body></html>")
	tw.WriteHeader(&tar.Header{
		Name: "index.html",
		Size: int64(len(content)),
		Mode: 0o644,
	})
	tw.Write(content)

	cssContent := []byte("body { color: red; }")
	tw.WriteHeader(&tar.Header{
		Name: "css/style.css",
		Size: int64(len(cssContent)),
		Mode: 0o644,
	})
	tw.Write(cssContent)

	tw.Close()
	gw.Close()

	if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
		t.Fatalf("write test archive: %v", err)
	}
}
