package pkgpack

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"testing"
)

func TestWriteTgzDeterministic(t *testing.T) {
	root := t.TempDir()

	// staging content
	if err := os.MkdirAll(filepath.Join(root, "bin"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "bin", "exec"), []byte("echo hi\n"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "specs"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "specs", "spec.yaml"), []byte("version: 1\n"), 0644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "package.json"), []byte("{}\n"), 0644); err != nil {
		t.Fatal(err)
	}

	out1 := filepath.Join(t.TempDir(), "a.tgz")
	out2 := filepath.Join(t.TempDir(), "b.tgz")

	if err := WriteTgz(out1, root); err != nil {
		t.Fatalf("write tgz 1: %v", err)
	}
	if err := WriteTgz(out2, root); err != nil {
		t.Fatalf("write tgz 2: %v", err)
	}

	hash1, err := fileSHA(out1)
	if err != nil {
		t.Fatal(err)
	}
	hash2, err := fileSHA(out2)
	if err != nil {
		t.Fatal(err)
	}
	if hash1 != hash2 {
		t.Fatalf("expected deterministic archives, got %s vs %s", hash1, hash2)
	}
}

func fileSHA(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(data)
	return fmt.Sprintf("%x", sum[:]), nil
}
