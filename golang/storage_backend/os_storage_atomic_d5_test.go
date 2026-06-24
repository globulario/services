package storage_backend

// os_storage_atomic_d5_test.go — D5a: AtomicWriteFile commits crash-safely and
// the temp-name pattern is recognizable so the orphan sweeper can reap it.

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestAtomicWriteFile_CommitsFullyAndLeavesNoTemp(t *testing.T) {
	root := t.TempDir()
	s := NewOSStorage(root)
	ctx := context.Background()

	if err := s.AtomicWriteFile(ctx, "artifacts/foo.manifest.json", []byte(`{"v":1}`), 0o644); err != nil {
		t.Fatalf("first write: %v", err)
	}
	got, err := os.ReadFile(filepath.Join(root, "artifacts", "foo.manifest.json"))
	if err != nil || string(got) != `{"v":1}` {
		t.Fatalf("read after write: got %q err %v", got, err)
	}

	// Atomic overwrite replaces the bytes fully.
	if err := s.AtomicWriteFile(ctx, "artifacts/foo.manifest.json", []byte(`{"v":2}`), 0o644); err != nil {
		t.Fatalf("second write: %v", err)
	}
	got, _ = os.ReadFile(filepath.Join(root, "artifacts", "foo.manifest.json"))
	if string(got) != `{"v":2}` {
		t.Fatalf("atomic overwrite: got %q", got)
	}

	// No temp file is left behind after a successful write (test #1: no partial sidecar).
	entries, _ := os.ReadDir(filepath.Join(root, "artifacts"))
	if len(entries) != 1 {
		t.Fatalf("expected exactly one committed file, got %d", len(entries))
	}
	if IsAtomicTempName(entries[0].Name()) {
		t.Fatalf("atomic write left a temp file behind: %s", entries[0].Name())
	}
}

func TestIsAtomicTempName(t *testing.T) {
	if !IsAtomicTempName("gateway%1.0.0.bin.tmp.f47ac10b-58cc-4372-a567-0e02b2c3d479") {
		t.Fatal("a .tmp.<uuid> file must be recognized as an atomic temp")
	}
	for _, no := range []string{
		"gateway%1.0.0.bin",           // committed blob
		"gateway%1.0.0.manifest.json", // committed manifest
		"gateway.bin.tmp.not-a-uuid",  // wrong suffix shape
		"gateway.bin.tmp.",            // no uuid
	} {
		if IsAtomicTempName(no) {
			t.Fatalf("%q must NOT be recognized as an atomic temp", no)
		}
	}
}

// Test #2/#5 boundary: while a temp file sits next to the committed file (as it
// does between create and rename, and as a crash leaves it), a reader of the
// committed path still sees the prior valid bytes — never a partial commit.
func TestAtomicWriteFile_ReaderNeverSeesPartialCommit(t *testing.T) {
	root := t.TempDir()
	s := NewOSStorage(root)
	if err := s.AtomicWriteFile(context.Background(), "artifacts/m.json", []byte("OLD-VALID"), 0o644); err != nil {
		t.Fatal(err)
	}
	tmp := filepath.Join(root, "artifacts", "m.json.tmp.f47ac10b-58cc-4372-a567-0e02b2c3d479")
	if err := os.WriteFile(tmp, []byte("HALF"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, _ := os.ReadFile(filepath.Join(root, "artifacts", "m.json"))
	if string(got) != "OLD-VALID" {
		t.Fatalf("committed file must stay the old valid bytes while a temp exists; got %q", got)
	}
}
