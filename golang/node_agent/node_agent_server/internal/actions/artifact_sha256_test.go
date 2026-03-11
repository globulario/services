package actions

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"google.golang.org/protobuf/types/known/structpb"
)

// PR-D.2: Tests for SHA256 integrity enforcement.

func TestArtifactVerify_MissingSHA256_Rejects(t *testing.T) {
	// Ensure dev bypass is off.
	t.Setenv("GLOBULAR_ALLOW_MISSING_SHA256", "")

	dir := t.TempDir()
	artifact := filepath.Join(dir, "test.tgz")
	if err := os.WriteFile(artifact, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	args, _ := structpb.NewStruct(map[string]interface{}{
		"artifact_path": artifact,
		// expected_sha256 intentionally omitted
	})
	act := artifactVerifyAction{}
	_, err := act.Apply(context.Background(), args)
	if err == nil {
		t.Fatal("expected error when SHA256 is missing, got nil")
	}
	if want := "expected_sha256 is required"; !contains(err.Error(), want) {
		t.Fatalf("unexpected error: %v (want substring %q)", err, want)
	}
}

func TestArtifactVerify_MissingSHA256_DevBypass(t *testing.T) {
	t.Setenv("GLOBULAR_ALLOW_MISSING_SHA256", "true")

	dir := t.TempDir()
	artifact := filepath.Join(dir, "test.tgz")
	if err := os.WriteFile(artifact, []byte("payload"), 0o644); err != nil {
		t.Fatal(err)
	}

	args, _ := structpb.NewStruct(map[string]interface{}{
		"artifact_path": artifact,
	})
	act := artifactVerifyAction{}
	msg, err := act.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("dev bypass should allow missing SHA256, got: %v", err)
	}
	if !contains(msg, "dev bypass") {
		t.Fatalf("expected 'dev bypass' in message, got: %s", msg)
	}
}

func TestArtifactVerify_MismatchedSHA256_Rejects(t *testing.T) {
	t.Setenv("GLOBULAR_ALLOW_MISSING_SHA256", "")

	dir := t.TempDir()
	artifact := filepath.Join(dir, "test.tgz")
	if err := os.WriteFile(artifact, []byte("real content"), 0o644); err != nil {
		t.Fatal(err)
	}

	args, _ := structpb.NewStruct(map[string]interface{}{
		"artifact_path":   artifact,
		"expected_sha256": "0000000000000000000000000000000000000000000000000000000000000000",
	})
	act := artifactVerifyAction{}
	_, err := act.Apply(context.Background(), args)
	if err == nil {
		t.Fatal("expected error on SHA256 mismatch, got nil")
	}
	if !contains(err.Error(), "mismatch") {
		t.Fatalf("expected mismatch error, got: %v", err)
	}
}

func TestArtifactVerify_ValidSHA256_Succeeds(t *testing.T) {
	t.Setenv("GLOBULAR_ALLOW_MISSING_SHA256", "")

	dir := t.TempDir()
	artifact := filepath.Join(dir, "test.tgz")
	content := []byte("verified content")
	if err := os.WriteFile(artifact, content, 0o644); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(content)
	expected := hex.EncodeToString(h[:])

	args, _ := structpb.NewStruct(map[string]interface{}{
		"artifact_path":   artifact,
		"expected_sha256": expected,
	})
	act := artifactVerifyAction{}
	msg, err := act.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("valid SHA256 should succeed, got: %v", err)
	}
	if !contains(msg, expected) {
		t.Fatalf("expected hash in message, got: %s", msg)
	}
}

func TestArtifactFetch_ExistingCorruptFile_ReDownloads(t *testing.T) {
	t.Setenv("GLOBULAR_ALLOW_MISSING_SHA256", "")

	repo := t.TempDir()
	service, version, platform := "svc", "2.0.0", "linux_amd64"
	srcPath := filepath.Join(repo, service, version, platform)
	if err := os.MkdirAll(srcPath, 0o755); err != nil {
		t.Fatal(err)
	}
	correctContent := []byte("correct artifact data")
	artifact := filepath.Join(srcPath, "svc.2.0.0.linux_amd64.tgz")
	if err := os.WriteFile(artifact, correctContent, 0o644); err != nil {
		t.Fatal(err)
	}

	h := sha256.Sum256(correctContent)
	expected := hex.EncodeToString(h[:])

	// Pre-plant a corrupted file at the destination.
	dest := filepath.Join(t.TempDir(), "out.tgz")
	if err := os.WriteFile(dest, []byte("corrupted"), 0o644); err != nil {
		t.Fatal(err)
	}

	t.Setenv("GLOBULAR_ARTIFACT_REPO_ROOT", repo)
	args, _ := structpb.NewStruct(map[string]interface{}{
		"service":         service,
		"version":         version,
		"platform":        platform,
		"artifact_path":   dest,
		"expected_sha256": expected,
	})
	act := artifactFetchAction{}
	_, err := act.Apply(context.Background(), args)
	if err != nil {
		t.Fatalf("should re-fetch after corrupt file: %v", err)
	}

	// Verify correct content.
	data, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != string(correctContent) {
		t.Fatalf("expected correct content after re-fetch, got %q", string(data))
	}
}

// contains() is defined in keepalived_test.go in this package.
